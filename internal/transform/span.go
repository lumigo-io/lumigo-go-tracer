package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/google/uuid"
	lumigoctx "github.com/lumigo-io/lumigo-go-tracer/internal/context"
	"github.com/lumigo-io/lumigo-go-tracer/internal/telemetry"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
)

type mapper struct {
	ctx          context.Context
	span         sdktrace.ReadOnlySpan
	logger       logrus.FieldLogger
	maxEntrySize int
}

func NewMapper(ctx context.Context, span sdktrace.ReadOnlySpan, logger logrus.FieldLogger, maxEntrySize int) *mapper {
	return &mapper{
		ctx:          ctx,
		span:         span,
		logger:       logger,
		maxEntrySize: maxEntrySize,
	}
}

func (m *mapper) Transform(invocationStartedTimestamp int64) telemetry.Span {
	numAttrs := len(m.span.Attributes()) + m.span.Resource().Len() + 2

	if m.span.SpanKind() != apitrace.SpanKindUnspecified {
		numAttrs++
	}

	attrs := make(map[string]interface{}, numAttrs)

	for iter := m.span.Resource().Iter(); iter.Next(); {
		kv := iter.Label()
		attrs[string(kv.Key)] = kv.Value.AsInterface()
	}
	for _, kv := range m.span.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsInterface()
		m.logger.WithField(string(kv.Key), kv.Value.AsInterface()).Info()
	}

	m.logger.WithFields(attrs).Info("span attributes")

	if m.span.SpanKind() != apitrace.SpanKindUnspecified {
		attrs["m.span.kind"] = strings.ToLower(m.span.SpanKind().String())
	}

	startTime := m.span.StartTime().UnixMilli()
	if telemetry.IsEndSpan(m.span) {
		startTime = invocationStartedTimestamp
	}
	lumigoSpan := telemetry.Span{
		StartedTimestamp: startTime,
		EndedTimestamp:   m.span.EndTime().UnixMilli(),
	}

	isStartSpan := telemetry.IsStartSpan(m.span)
	isEndSpan := telemetry.IsEndSpan(m.span)
	lumigoSpan.Region = os.Getenv("AWS_REGION")

	awsRoot := getAmazonTraceID()
	if awsRoot == "" {
		m.logger.Error("unable to fetch Amazon Trace ID")
	}
	lumigoSpan.SpanInfo = telemetry.SpanInfo{
		LogStreamName: os.Getenv("AWS_LAMBDA_LOG_STREAM_NAME"),
		LogGroupName:  os.Getenv("AWS_LAMBDA_LOG_GROUP_NAME"),
		TraceID: telemetry.SpanTraceRoot{
			Root: awsRoot,
		},
	}
	lambdaName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	lambdaType := "function"
	if m.span.Name() != lambdaName && m.span.Name() != "LumigoParentSpan" {
		lambdaType = "http"
		lumigoSpan.SpanInfo.HttpInfo = m.getHTTPInfo(attrs)
	} else {
		lumigoSpan.LambdaName = lambdaName
		lumigoSpan.MemoryAllocated = os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE")
		lumigoSpan.Runtime = os.Getenv("AWS_EXECUTION_ENV")
		if event, ok := attrs["event"]; ok {
			lumigoSpan.Event = fmt.Sprint(event)
		} else {
			m.logger.Error("unable to fetch event")
		}

		isWarmStart := os.Getenv("IS_WARM_START")
		if isWarmStart == "" && !isProvisionConcurrencyInitialization() {
			lumigoSpan.LambdaReadiness = "cold"
			os.Setenv("IS_WARM_START", "true")
		} else {
			lumigoSpan.LambdaReadiness = "warm"
		}
	}
	lumigoSpan.LambdaType = lambdaType
	if isStartSpan {
		lumigoSpan.LambdaEnvVars = m.getEnvVars()
	}
	lambdaCtx, lambdaOk := lambdacontext.FromContext(m.ctx)
	if lambdaOk {
		containerID, _ := uuid.NewUUID()
		lumigoSpan.LambdaContainerID = containerID.String()

		if lambdaType == "http" {
			spanID, _ := uuid.NewUUID()
			lumigoSpan.ID = spanID.String()
			lumigoSpan.ParentID = lambdaCtx.AwsRequestID
		} else {
			lumigoSpan.ID = lambdaCtx.AwsRequestID
		}

		if isStartSpan {
			lumigoSpan.ID = fmt.Sprintf("%s_started", lumigoSpan.ID)
			deadline, _ := m.ctx.Deadline()
			lumigoSpan.MaxFinishTime = time.Now().UnixMilli() - deadline.UnixMilli()
		}

		accountID, err := getAccountID(lambdaCtx)
		if err != nil {
			m.logger.WithError(err).Error()
		}
		lumigoSpan.Account = accountID

	} else {
		m.logger.Error("unable to fetch from LambdaContext")
	}

	if token, ok := attrs["lumigo_token"]; ok {
		lumigoSpan.Token = fmt.Sprint(token)
	} else {
		m.logger.Error("unable to fetch lumigo token from span")
	}
	if isEndSpan {
		lumigoSpan.SpanError = m.getSpanError(attrs)
		lambdaResp := m.getAttrAndLimit(attrs, "response")
		if lambdaResp != "" {
			lumigoSpan.LambdaResponse = aws.String(lambdaResp)
		}
	}
	if transactionID := getTransactionID(awsRoot); transactionID != "" {
		lumigoSpan.TransactionID = transactionID
	} else {
		m.logger.Error("unable to fetch transaction ID")
	}

	lumigoCtx, lumigoOk := lumigoctx.FromContext(m.ctx)
	if lumigoOk {
		lumigoSpan.SpanInfo.TracerVersion = telemetry.TracerVersion{
			Version: lumigoCtx.TracerVersion,
		}
	} else {
		m.logger.Error("unable to fetch from LumigoContext")
	}
	return lumigoSpan
}

func (m *mapper) getSpanError(attrs map[string]interface{}) *telemetry.SpanError {
	if _, ok := attrs["has_error"]; !ok {
		return nil
	}
	var spanError telemetry.SpanError
	if errType, ok := attrs["error_type"]; ok {
		spanError.Type = fmt.Sprint(errType)
	} else {
		m.logger.Error("unable to fetch lambda error type from span")
	}

	if errMessage, ok := attrs["error_message"]; ok {
		spanError.Message = fmt.Sprint(errMessage)
	} else {
		m.logger.Error("unable to fetch lambda error message from span")
	}

	if errStacktrace, ok := attrs["error_stacktrace"]; ok {
		spanError.Stacktrace = fmt.Sprint(errStacktrace)
	} else {
		m.logger.Error("unable to fetch lambda error stacktrace from span")
	}
	if spanError.IsEmpty() {
		return nil
	}
	return &spanError
}

func (m *mapper) getEnvVars() string {
	envs := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		envs[pair[0]] = pair[1]
	}
	envsBytes, err := json.Marshal(envs)
	if err != nil {
		m.logger.Error("unable to fetch lambda environment vars")
	}
	if len(envsBytes) > m.maxEntrySize {
		envsBytes = envsBytes[:m.maxEntrySize]
	}
	return string(envsBytes)
}

func (m *mapper) getHTTPInfo(attrs map[string]interface{}) *telemetry.SpanHttpInfo {
	var spanHttpInfo telemetry.SpanHttpInfo
	if host, ok := attrs["http.host"]; ok {
		spanHttpInfo.Host = fmt.Sprint(host)
	} else {
		m.logger.Error("unable to fetch HTTP host")
	}

	if method, ok := attrs["http.method"]; ok {
		spanHttpInfo.Request.Method = aws.String(fmt.Sprint(method))
	} else {
		m.logger.Error("unable to fetch HTTP method")
	}

	if target, ok := attrs["http.target"]; ok {
		uri := fmt.Sprintf("%s%s", spanHttpInfo.Host, target)
		spanHttpInfo.Request.URI = aws.String(uri)
	} else {
		m.logger.Error("unable to fetch HTTP target")
	}

	if headers, ok := attrs["http.request_headers"]; ok {
		spanHttpInfo.Request.Headers = fmt.Sprint(headers)
	} else {
		m.logger.Error("unable to fetch HTTP request headers")
	}

	if reqBody, ok := attrs["http.request_body"]; ok {
		spanHttpInfo.Request.Body = fmt.Sprint(reqBody)
	}

	if headers, ok := attrs["http.response_headers"]; ok {
		spanHttpInfo.Response.Headers = fmt.Sprint(headers)
	} else {
		m.logger.Error("unable to fetch HTTP response headers")
	}

	// response
	if respBody, ok := attrs["http.response_body"]; ok {
		spanHttpInfo.Response.Body = fmt.Sprint(respBody)
	} else {
		m.logger.Error("unable to fetch HTTP response body")
	}

	if code, ok := attrs["http.status_code"]; ok {
		spanHttpInfo.Response.StatusCode = aws.Int64(code.(int64))
	} else {
		m.logger.Error("unable to fetch HTTP status code")
	}

	return &spanHttpInfo
}

func isProvisionConcurrencyInitialization() bool {
	return os.Getenv("AWS_LAMBDA_INITIALIZATION_TYPE") == "provisioned-concurrency"
}

func getAccountID(ctx *lambdacontext.LambdaContext) (string, error) {
	functionARN, err := arn.Parse(ctx.InvokedFunctionArn)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse ARN")
	}
	return functionARN.AccountID, nil
}

func getAmazonTraceID() string {
	awsTraceItems := strings.SplitN(os.Getenv("_X_AMZN_TRACE_ID"), ";", 2)
	if len(awsTraceItems) > 1 {
		root := strings.SplitN(awsTraceItems[0], "=", 2)
		return root[1]
	}
	return ""
}

func getTransactionID(root string) string {
	items := strings.SplitN(root, "-", 3)
	if len(items) > 1 {
		return items[2]
	}
	return ""
}

func (m *mapper) getAttrAndLimit(attrs map[string]interface{}, key string) string {
	if value, ok := attrs[key]; ok {
		valueStr := fmt.Sprint(value)
		if len(valueStr) > m.maxEntrySize {
			valueStr = valueStr[:m.maxEntrySize]
		}
		return valueStr
	} else {
		m.logger.Errorf("unable to fetch lambda %s from span", key)
	}
	return ""
}
