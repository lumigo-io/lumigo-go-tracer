package lumigotracer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/lumigo-io/go-tracer-beta/internal/telemetry"
	"github.com/lumigo-io/go-tracer-beta/internal/transform"
	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Exporter exports OpenTelemetry data to Lumigo.
type Exporter struct {
	context   context.Context
	logger    logrus.FieldLogger
	encoderMu sync.Mutex

	stoppedMu sync.RWMutex
	stopped   bool
}

// newExporter creates an Exporter with the passed options.
func newExporter(ctx context.Context, logger logrus.FieldLogger) (*Exporter, error) {
	return &Exporter{
		logger:  logger,
		context: ctx,
	}, nil
}

// ExportSpans writes spans in json format to file.
func (e *Exporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if e == nil {
		return nil
	}
	e.stoppedMu.RLock()
	stopped := e.stopped
	e.stoppedMu.RUnlock()
	if stopped {
		return nil
	}

	if len(spans) == 0 {
		return nil
	}

	var lumigoSpans []telemetry.Span
	e.encoderMu.Lock()
	defer e.encoderMu.Unlock()
	for _, span := range spans {
		mapper := transform.NewMapper(e.context, span, logger)
		lumigoSpan := mapper.Transform()
		if telemetry.IsStartSpan(span) {
			e.logger.Info("writing start span")
			if err := writeSpan([]telemetry.Span{lumigoSpan}, true); err != nil {
				return errors.Wrap(err, "failed to store startSpan")
			}
			continue
		}
		lumigoSpans = append(lumigoSpans, lumigoSpan)
	}

	if len(lumigoSpans) == 0 {
		return nil
	}
	e.logger.Info("writing end span")
	if err := writeSpan(lumigoSpans, false); err != nil {
		return errors.Wrap(err, "failed to store endSpan")
	}
	return nil
}

// Shutdown is called to stop the exporter, it preforms no action.
func (e *Exporter) Shutdown(ctx context.Context) error {
	e.stoppedMu.Lock()
	e.stopped = true
	e.stoppedMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	e.logger.Info("finished writing spans files")
	return nil
}

func writeSpan(spans []telemetry.Span, isStart bool) error {
	var file string
	if isStart {
		file = fmt.Sprintf("/tmp/lumigo-spans/%s_span", ksuid.New())
	} else {
		file = fmt.Sprintf("/tmp/lumigo-spans/%s_end", ksuid.New())
	}
	writer, err := os.Create(file)
	if err != nil {
		return errors.Wrapf(err, "failed to create span data store: %s", file)
	}
	enc := json.NewEncoder(writer)
	if err := enc.Encode(spans); err != nil {
		return errors.Wrapf(err, "failed to write span in data store: %s", file)
	}
	return nil
}
