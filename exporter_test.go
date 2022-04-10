package lumigotracer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/lumigo-io/lumigo-go-tracer/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type exporterTestSuite struct {
	suite.Suite
}

func TestSetupExporterSuite(t *testing.T) {
	suite.Run(t, &exporterTestSuite{})
}

func (e *exporterTestSuite) TearDownTest() {
	assert.NoError(e.T(), deleteAllFiles())
}

func (e *exporterTestSuite) TestNilExporter() {
	span := &tracetest.SpanStub{}
	var exporter *Exporter

	exporter.ExportSpans(context.Background(), []trace.ReadOnlySpan{span.Snapshot()}) //nolint
}

func (e *exporterTestSuite) TestExportSpans() {
	logger.Out = ioutil.Discard
	os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test")
	os.Setenv("AWS_REGION", "us-east-1")
	spanID, _ := oteltrace.SpanIDFromHex("83887e5d7da921ba")
	traceID, _ := oteltrace.TraceIDFromHex("83887e5d7da921ba")

	spanCtx := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		SpanID:  spanID,
		TraceID: traceID,
	})
	startSpan := &tracetest.SpanStub{
		Name:        "test",
		StartTime:   time.Now(),
		EndTime:     time.Now(),
		SpanContext: spanCtx,
		Attributes: []attribute.KeyValue{
			attribute.String("faas.execution", "3f12bdd4-651f-4610-a469-a797721cd438"),
			attribute.String("cloud.account.id", "123"),
		},
		Resource: resource.NewWithAttributes(semconv.SchemaURL,
			attribute.KeyValue{
				Key:   "cloud.provider",
				Value: attribute.StringValue("aws"),
			},
			attribute.KeyValue{
				Key:   "faas.name",
				Value: attribute.StringValue("test"),
			},
			attribute.KeyValue{
				Key:   "faas.name",
				Value: attribute.StringValue("test"),
			},
			attribute.KeyValue{
				Key:   "lumigo_token",
				Value: attribute.StringValue("test"),
			},
			attribute.KeyValue{
				Key:   "cloud.region",
				Value: attribute.StringValue("us-east-1"),
			},
		),
	}
	endSpan := &tracetest.SpanStub{
		Name:        "LumigoParentSpan",
		StartTime:   time.Now(),
		EndTime:     time.Now(),
		SpanContext: spanCtx,
		Attributes: []attribute.KeyValue{
			attribute.String("event", `{"key1":"value1","key2":"value2","key3":"value3"}`),
			attribute.String("response", "Hello"),
		},
		Resource: resource.NewWithAttributes(semconv.SchemaURL,
			attribute.KeyValue{
				Key:   "cloud.region",
				Value: attribute.StringValue("us-east-1"),
			},
			attribute.KeyValue{
				Key:   "lumigo_token",
				Value: attribute.StringValue("test"),
			},
		),
	}

	testContext := lambdacontext.NewContext(context.Background(), &mockLambdaContext)
	exp, err := createExporter(false, testContext, logger)
	assert.NoError(e.T(), err)

	err = exp.ExportSpans(context.Background(), []trace.ReadOnlySpan{
		startSpan.Snapshot(),
		endSpan.Snapshot(),
	})
	assert.NoError(e.T(), err)

	container, err := readSpansFromFile()
	assert.NoError(e.T(), err)

	lumigoStart := container.startSpan[0]
	assert.Equal(e.T(), mockLambdaContext.AwsRequestID+"_started", lumigoStart.ID)
	assert.Equal(e.T(), "account-id", lumigoStart.Account)

	lumigoEnd := container.endSpan[0]
	event := fmt.Sprint(endSpan.Attributes[0].Value.AsString())
	response := fmt.Sprint(endSpan.Attributes[1].Value.AsString())
	assert.Equal(e.T(), mockLambdaContext.AwsRequestID, lumigoEnd.ID)
	assert.Equal(e.T(), event, lumigoEnd.Event)
	assert.Equal(e.T(), aws.String(response), lumigoEnd.LambdaResponse)
	assert.Equal(e.T(), endSpan.Resource.Attributes()[0].Value.AsString(), lumigoEnd.Region)
	assert.Equal(e.T(), endSpan.Resource.Attributes()[1].Value.AsString(), lumigoEnd.Token)

}

type spanContainer struct {
	startSpan []telemetry.Span
	endSpan   []telemetry.Span
}

func readSpansFromFile() (spanContainer, error) {
	files, err := ioutil.ReadDir(SPANS_DIR)
	if err != nil {
		return spanContainer{}, err
	}

	var container spanContainer
	for _, file := range files {
		var spans []telemetry.Span
		content, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", SPANS_DIR, file.Name()))
		if err != nil {
			return spanContainer{}, err
		}
		err = json.Unmarshal(content, &spans)
		if err != nil {
			return spanContainer{}, err
		}
		if strings.Contains(file.Name(), "_span") {
			container.startSpan = spans
			continue
		}
		container.endSpan = spans
	}
	return container, nil
}

func deleteAllFiles() error {
	files, err := ioutil.ReadDir(SPANS_DIR)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := os.Remove(fmt.Sprintf("%s/%s", SPANS_DIR, file.Name())); err != nil {
			return err
		}
	}
	return nil
}
