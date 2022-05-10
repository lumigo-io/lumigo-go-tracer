package telemetry

import (
	"os"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanTraceRoot the amazon X-Trace-ID
type SpanTraceRoot struct {
	Root string `json:"Root"`
}

// TracerVersion the version info for the tracer
// which captured the spans
type TracerVersion struct {
	Version string `json:"version"`
}

// SpanInfo extra info for span
type SpanInfo struct {
	LogStreamName string        `json:"logStreamName"`
	LogGroupName  string        `json:"logGroupName"`
	TraceID       SpanTraceRoot `json:"traceId"`
	TracerVersion TracerVersion `json:"tracer"`
	HttpInfo      *SpanHttpInfo `json:"httpInfo,omitempty"`
}

// SpanHttpInfo extra info for HTTP reuquests
type SpanHttpInfo struct {
	Host     string         `json:"host"`
	Request  SpanHttpCommon `json:"request"`
	Response SpanHttpCommon `json:"response"`
}

// SpanHttpRequest the span for the HTTP request
type SpanHttpCommon struct {
	URI        *string `json:"uri,omitempty"`
	Method     *string `json:"method,omitempty"`
	StatusCode *int64  `json:"statusCode,omitempty"`
	InstanceID *string `json:"instance_id,omitempty"`
	Body       string  `json:"body,omitempty"`
	Headers    string  `json:"headers,omitempty"`
}

// SpanError the extra info if lambda returned
// an error
type SpanError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Stacktrace string `json:"stacktrace"`
}

func (s SpanError) IsEmpty() bool {
	if s.Type == "" && s.Message == "" && s.Stacktrace == "" {
		return true
	}
	return false
}

// Span is a distributed tracing span.
type Span struct {
	// Required Fields:
	//
	// ID is a unique identifier for this span.
	ID string `json:"id"`

	// ParentID is the span id of the previous caller of this span.  This
	// can be empty if this is the first span.
	ParentID string `json:"parentId"`

	// TransactionID is the ID generated for this span transaction
	TransactionID string `json:"transactionId"`

	// Runtime the runtime which lambda runs on
	Runtime string `json:"runtime"`

	// Region the region which lambda runs
	Region string `json:"region"`

	// Event is the lambda event triggered the lambda
	Event string `json:"event"`

	// Token is the lumigo token needed to send the spans later
	// from file extensions
	Token string `json:"token"`

	// MemoryAllocated the requested memory for this lambda
	MemoryAllocated string `json:"memoryAllocated"`

	// Account represents the AWS Account ID
	Account string `json:"account"`

	// Envs the environments variables of lambda
	LambdaEnvVars string `json:"envs"`

	// LambdaType the type of the lambda function etc.
	SpanType string `json:"type"`

	// LambdaName the name of the lambda
	LambdaName string `json:"name"`

	// LambdaReadiness is if lambda is cold or warmed already
	LambdaReadiness string `json:"readiness"`

	// LambdaResponse the response of Lambda
	LambdaResponse *string `json:"return_value"`

	// LambdaContainerID the id of the lambda container
	LambdaContainerID string `json:"lambda_container_id"`

	// SpanInfo extra info for span
	SpanInfo SpanInfo `json:"info"`

	// StartedTimestamp when this span started
	StartedTimestamp int64 `json:"started"`

	// EndedTimestamp when this span ended
	EndedTimestamp int64 `json:"ended"`

	// MaxFinishTime the max finish tiem of lambda
	MaxFinishTime int64 `json:"maxFinishTime"`

	// SpanError error details
	SpanError *SpanError `json:"error"`
}

func IsStartSpan(span sdktrace.ReadOnlySpan) bool {
	return span.Name() == os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
}

func IsEndSpan(span sdktrace.ReadOnlySpan) bool {
	return span.Name() == "LumigoParentSpan"
}
