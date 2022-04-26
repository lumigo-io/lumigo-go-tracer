package transform

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/lumigo-io/lumigo-go-tracer/internal/telemetry"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var (
	traceID, _        = trace.TraceIDFromHex("000000")
	spanID, _         = trace.SpanIDFromHex("000000")
	mockLambdaContext = lambdacontext.LambdaContext{
		AwsRequestID:       "123",
		InvokedFunctionArn: "arn:partition:service:region:account-id:resource-type:resource-id",
		Identity: lambdacontext.CognitoIdentity{
			CognitoIdentityID:     "someId",
			CognitoIdentityPoolID: "somePoolId",
		},
	}
)

func TestTransform(t *testing.T) {
	now := time.Now()
	ctx := lambdacontext.NewContext(context.Background(), &mockLambdaContext)
	testcases := []struct {
		testname string
		input    *tracetest.SpanStub
		expect   telemetry.Span
		before   func()
		after    func()
	}{
		{
			testname: "simplest span",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "test",
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "cold",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID + "_started",
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
			},
		},
		{
			testname: "span with runtime and info",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "test",
			},
			expect: telemetry.Span{
				LambdaName:      "test",
				LambdaType:      "function",
				LambdaReadiness: "cold",
				Runtime:         "go",
				Account:         "account-id",
				SpanInfo: telemetry.SpanInfo{
					LogStreamName: "2021/12/06/[$LATEST]2f4f26a6224b421c86bc4570bb7bf84b",
					LogGroupName:  "/aws/lambda/helloworld-37",
					TraceID:       telemetry.SpanTraceRoot{},
				},
				ID:               mockLambdaContext.AwsRequestID + "_started",
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("AWS_EXECUTION_ENV", "go")
				os.Setenv("AWS_LAMBDA_LOG_STREAM_NAME", "2021/12/06/[$LATEST]2f4f26a6224b421c86bc4570bb7bf84b")
				os.Setenv("AWS_LAMBDA_LOG_GROUP_NAME", "/aws/lambda/helloworld-37")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("AWS_EXECUTION_ENV")
				os.Unsetenv("AWS_LAMBDA_LOG_STREAM_NAME")
				os.Unsetenv("AWS_LAMBDA_LOG_GROUP_NAME")
			},
		},
		{
			testname: "span lambda readiness warm",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "test",
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "warm",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID + "_started",
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
		{
			testname: "span with event",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "test",
				Attributes: []attribute.KeyValue{
					attribute.String("event", "test"),
				},
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "warm",
				Event:            "test",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID + "_started",
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
		{
			testname: "span with event and response",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "LumigoParentSpan",
				Attributes: []attribute.KeyValue{
					attribute.String("event", "test"),
					attribute.String("response", "test2"),
				},
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "warm",
				LambdaResponse:   aws.String("test2"),
				Event:            "test",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID,
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
		{
			testname: "span with LumigoParentSpan is function",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "LumigoParentSpan",
				Attributes: []attribute.KeyValue{
					attribute.String("event", "test"),
					attribute.String("response", "test2"),
				},
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "warm",
				LambdaResponse:   aws.String("test2"),
				Event:            "test",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID,
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
		{
			testname: "span with error",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "LumigoParentSpan",
				Attributes: []attribute.KeyValue{
					attribute.String("event", "test"),
					attribute.Bool("has_error", true),
					attribute.String("error_type", "TestError"),
					attribute.String("error_message", "failed error"),
					attribute.String("error_stacktrace", "failed error"),
				},
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "warm",
				LambdaResponse:   nil,
				Event:            "test",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID,
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
				SpanError: &telemetry.SpanError{
					Type:       "TestError",
					Message:    "failed error",
					Stacktrace: "failed error",
				},
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
		{
			testname: "span http success",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "HttpSpan",
				Attributes: []attribute.KeyValue{
					attribute.String("http.host", "s3.aws.com"),
					attribute.String("http.target", "/"),
					attribute.String("http.method", "POST"),
					attribute.Int64("http.status_code", 200),
					attribute.String("http.request_headers", `{"Agent": "test"}`),
					attribute.String("http.request_body", `{"name": "test"}`),
					attribute.String("http.response_headers", `{"Agent": "test"}`),
					attribute.String("http.response_body", `{"response": "test"}`),
					attribute.String("event", "test"),
				},
			},
			expect: telemetry.Span{
				LambdaType:       "http",
				LambdaResponse:   nil,
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID,
				ParentID:         mockLambdaContext.AwsRequestID,
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
				SpanInfo: telemetry.SpanInfo{
					HttpInfo: &telemetry.SpanHttpInfo{
						Host: "s3.aws.com",
						Request: telemetry.SpanHttpCommon{
							URI:     aws.String("s3.aws.com/"),
							Method:  aws.String("POST"),
							Headers: `{"Agent": "test"}`,
							Body:    `{"name": "test"}`,
						},
						Response: telemetry.SpanHttpCommon{
							StatusCode: aws.Int64(200),
							Headers:    `{"Agent": "test"}`,
							Body:       `{"response": "test"}`,
						},
					},
				},
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
		{
			testname: "end span check limits",
			input: &tracetest.SpanStub{
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Name:      "LumigoParentSpan",
				Attributes: []attribute.KeyValue{
					attribute.String("event", strings.Repeat("even", 512)+"to cut"),
					attribute.String("response", strings.Repeat("resp", 512)+"to cut"),
				},
			},
			expect: telemetry.Span{
				LambdaName:       "test",
				LambdaType:       "function",
				LambdaReadiness:  "warm",
				Account:          "account-id",
				ID:               mockLambdaContext.AwsRequestID,
				StartedTimestamp: now.UnixMilli(),
				EndedTimestamp:   now.Add(1 * time.Second).UnixMilli(),
				LambdaResponse:   aws.String(strings.Repeat("resp", 512)),
				Event:            strings.Repeat("even", 512),
			},
			before: func() {
				os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
				os.Setenv("IS_WARM_START", "true")
			},
			after: func() {
				os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
				os.Unsetenv("IS_WARM_START")
			},
		},
	}

	for _, tc := range testcases {
		tc.before()
		mapper := NewMapper(ctx, tc.input.Snapshot(), logrus.New(), 2048)
		invocationStartedTimestamp := now.UnixMilli()
		if tc.expect.LambdaType == "function" && strings.HasSuffix(tc.expect.ID, "_started") {
			invocationStartedTimestamp = 0
		}
		lumigoSpan := mapper.Transform(invocationStartedTimestamp)
		// intentionally ignore CI and Local envs
		lumigoSpan.LambdaEnvVars = ""
		// intentionally ignore generated LambdaContainerID
		lumigoSpan.LambdaContainerID = ""
		// intentionally ignore MaxFinishTime, cannot be matched
		lumigoSpan.MaxFinishTime = 0
		if lumigoSpan.LambdaType == "http" {
			lumigoSpan.ID = mockLambdaContext.AwsRequestID
		}
		// assert.Equal(t, tc.expect, lumigoSpan)
		if diff := cmp.Diff(tc.expect, lumigoSpan); diff != "" {
			t.Errorf("%s mismatch (-want +got):\n%s", tc.testname, diff)
		}

		tc.after()
	}
}

func TestTransformCheckEnvsCut(t *testing.T) {
	span := &tracetest.SpanStub{}
	os.Setenv("REALLY_LONG_ENV", strings.Repeat("envs", 512))
	ctx := lambdacontext.NewContext(context.Background(), &mockLambdaContext)
	mapper := NewMapper(ctx, span.Snapshot(), logrus.New(), 2048)
	lumigoSpan := mapper.Transform(0)
	if len(lumigoSpan.LambdaEnvVars) != 2048 {
		t.Errorf("LambdaEnvVars should be of size 2048, got %d", len(lumigoSpan.LambdaEnvVars))
	}
	os.Unsetenv("REALLY_LONG_ENV")
}
