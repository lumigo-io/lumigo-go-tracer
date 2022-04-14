package lumigotracer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type Transport struct {
	rt         http.RoundTripper
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
}

func NewTransport(transport http.RoundTripper) *Transport {
	return &Transport{
		rt:         transport,
		provider:   otel.GetTracerProvider(),
		propagator: otel.GetTextMapPropagator(),
	}
}

func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	traceCtx, span := t.provider.Tracer("lumigo").Start(req.Context(), "HttpSpan")

	req = req.WithContext(traceCtx)
	span.SetAttributes(semconv.HTTPClientAttributesFromHTTPRequest(req)...)
	span.SetAttributes(semconv.HTTPTargetKey.String(req.URL.Path))
	span.SetAttributes(semconv.HTTPHostKey.String(req.URL.Host))
	t.propagator.Inject(traceCtx, propagation.HeaderCarrier(req.Header))
	if req.Body != nil {
		bodyBytes, bodyErr := io.ReadAll(req.Body)
		if bodyErr != nil {
			logger.WithError(bodyErr).Error("failed to parse request body")
		}

		if len(bodyBytes) > cfg.MaxEntrySize {
			span.SetAttributes(attribute.String("http.request_body", string(bodyBytes[:cfg.MaxEntrySize])))
		} else {
			span.SetAttributes(attribute.String("http.request_body", string(bodyBytes)))
		}
		// restore body
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	reqHeaders := make(map[string]string)
	for k, values := range req.Header {
		for _, value := range values {
			reqHeaders[k] = value
		}
	}
	headersJson, err := json.Marshal(reqHeaders)
	if err != nil {
		logger.WithError(err).Error("failed to fetch request headers")
	}
	reqHeaderString := string(headersJson)
	if len(reqHeaderString) > cfg.MaxEntrySize {
		span.SetAttributes(attribute.String("http.request_headers", string(reqHeaderString[:cfg.MaxEntrySize])))
	} else {
		span.SetAttributes(attribute.String("http.request_headers", string(reqHeaderString)))
	}

	resp, err = t.rt.RoundTrip(req)
	if resp == nil {
		return nil, err
	}
	// response
	span.SetAttributes(semconv.HTTPAttributesFromHTTPStatusCode(resp.StatusCode)...)
	span.SetStatus(semconv.SpanStatusFromHTTPStatusCode(resp.StatusCode))

	responseHeaders := make(map[string]string)
	for k, values := range resp.Header {
		for _, value := range values {
			responseHeaders[k] = value
		}
	}
	headersJson, jsonErr := json.Marshal(responseHeaders)
	if jsonErr != nil {
		logger.WithError(err).Error("failed to fetch response headers")
	}
	if len(headersJson) > cfg.MaxEntrySize {
		span.SetAttributes(attribute.String("http.response_headers", string(headersJson[:cfg.MaxEntrySize])))
	} else {
		span.SetAttributes(attribute.String("http.response_headers", string(headersJson)))
	}

	if resp.Body != nil {
		bodyBytes, bodyErr := io.ReadAll(resp.Body)
		if bodyErr != nil {
			logger.WithError(bodyErr).Error("failed to parse response body")
		}
		if len(bodyBytes) > cfg.MaxEntrySize {
			span.SetAttributes(attribute.String("http.response_body", string(bodyBytes[:cfg.MaxEntrySize])))
		} else {
			span.SetAttributes(attribute.String("http.response_body", string(bodyBytes)))
		}

		resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	resp.Body = &wrappedBody{ctx: traceCtx, span: span, body: resp.Body}
	return resp, err
}

type wrappedBody struct {
	ctx  context.Context
	span trace.Span
	body io.ReadCloser
}

var _ io.ReadCloser = &wrappedBody{}

func (wb *wrappedBody) Read(b []byte) (int, error) {
	n, err := wb.body.Read(b)

	switch err {
	case nil:
		// nothing to do here but fall through to the return
	case io.EOF:
		wb.span.End()
	default:
		wb.span.RecordError(err)
		wb.span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

func (wb *wrappedBody) Close() error {
	wb.span.End()
	return wb.body.Close()
}
