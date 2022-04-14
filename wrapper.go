package lumigotracer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	lumigoctx "github.com/lumigo-io/lumigo-go-tracer/internal/context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	lambdadetector "go.opentelemetry.io/contrib/detectors/aws/lambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const SPANS_DIR = "/tmp/lumigo-spans"

var logger *log.Logger

const (
	version = "0.1.0"
)

func init() {
	logger = log.New()
	logger.Out = os.Stdout
	logger.Formatter = &LogFormatter{}
}

//Log custom format
type LogFormatter struct{}

//Format details
func (s *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("#LUMIGO# - %s - %s - %s ", timestamp, strings.ToUpper(entry.Level.String()), entry.Message)
	if entry.Data != nil {
		jsonBytes, err := json.Marshal(entry.Data)
		if err != nil {
			msg += fmt.Sprintf("failed to extract data from logger err: %+v", err)

		} else {
			msg += "structured data: " + string(jsonBytes)
		}
	}
	msg += "\n"
	return []byte(msg), nil
}

// WrapHandler wraps the lambda handler
func WrapHandler(handler interface{}, conf *Config) interface{} {
	if err := loadConfig(*conf); err != nil {
		logger.WithError(err).Error("failed validation error")
		return handler
	}
	if !cfg.debug {
		logger.Out = io.Discard
	}
	return func(ctx context.Context, payload json.RawMessage) (interface{}, error) {
		ctx = lumigoctx.NewContext(ctx, &lumigoctx.LumigoContext{
			TracerVersion: version,
		})
		tracer, err := NewTracer(ctx, cfg, payload)
		// catch all errors and exceptions
		if tracer == nil || err != nil {
			response, err := lambda.NewHandler(handler).Invoke(ctx, payload)
			return json.RawMessage(response), err
		}
		tracer.Start()

		response, lambdaErr := otellambda.WrapHandler(lambda.NewHandler(handler),
			otellambda.WithTracerProvider(tracer.provider),
			otellambda.WithFlusher(tracer.provider)).Invoke(tracer.traceCtx, payload)

		tracer.End(response, lambdaErr)

		return json.RawMessage(response), lambdaErr
	}
}

// newResource returns a resource describing this application.
func newResource(ctx context.Context, extraAttrs ...attribute.KeyValue) *resource.Resource {
	attrs := []attribute.KeyValue{
		attribute.String("lumigo_token", cfg.Token),
	}
	attrs = append(attrs, extraAttrs...)
	detector := lambdadetector.NewResourceDetector()
	res, err := detector.Detect(ctx)
	if err != nil {
		logger.WithError(err).Warn("failed to detect AWS lambda resources")
		return resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}
	r, _ := resource.Merge(
		res,
		resource.NewWithAttributes(res.SchemaURL(), attrs...),
	)
	return r
}

// createExporter returns a console exporter.
func createExporter(printStdout bool, ctx context.Context, logger log.FieldLogger) (trace.SpanExporter, error) {
	if printStdout {
		return stdouttrace.New()
	}
	if _, err := os.Stat(SPANS_DIR); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(SPANS_DIR, os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "failed to create dir: %s", SPANS_DIR)
		}
	} else if err != nil {
		logger.WithError(err).Error()
	}
	return newExporter(ctx, logger)
}
