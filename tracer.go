package lumigotracer

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type tracer struct {
	provider  *sdktrace.TracerProvider
	logger    logrus.FieldLogger
	span      trace.Span
	eventData []byte
	ctx       context.Context
	traceCtx  context.Context
}

func NewTracer(ctx context.Context, cfg Config, payload json.RawMessage) (retTracer *tracer, err error) {
	defer recoverWithLogs()
	retTracer = &tracer{
		ctx:    ctx,
		logger: logger,
	}

	exporter, err := createExporter(cfg.PrintStdout, ctx, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create otel exporter")
	}

	data, err := json.Marshal(&payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse event payload")
	}
	retTracer.eventData = data

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource(ctx,
			attribute.String("event", string(retTracer.eventData)),
		)),
	)
	retTracer.provider = tracerProvider
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return retTracer, nil
}

// Start tracks the span start data
func (t *tracer) Start() {
	defer recoverWithLogs()

	t.logger.Info("tracer starting")

	traceCtx, span := t.provider.Tracer("lumigo").Start(t.ctx, "LumigoParentSpan")
	span.SetAttributes(attribute.String("event", string(t.eventData)))
	t.span = span
	t.traceCtx = traceCtx
}

// End tracks the span end data after lambda execution
func (t *tracer) End(response []byte, lambdaErr error) {
	defer recoverWithLogs()
	if data, err := json.Marshal(json.RawMessage(response)); err == nil && lambdaErr == nil {
		t.span.SetAttributes(attribute.String("response", string(data)))
	} else {
		t.logger.WithError(err).Error("failed to track response")
	}

	if lambdaErr != nil {
		t.span.SetAttributes(attribute.Bool("has_error", true))
		t.span.SetAttributes(attribute.String("error_type", reflect.TypeOf(lambdaErr).String()))
		t.span.SetAttributes(attribute.String("error_message", lambdaErr.Error()))
		t.span.SetAttributes(attribute.String("error_stacktrace", takeStacktrace()))
	}
	t.span.End()
	t.provider.ForceFlush(t.traceCtx)
	t.provider.Shutdown(t.traceCtx)

	t.logger.Info("tracer ending")
}
