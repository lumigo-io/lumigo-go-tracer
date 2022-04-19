package lumigotracer

import (
	"bytes"
	"encoding/json"
	"io"
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
	req, span = addRequestDataToSpanAndWrap(req, span)
	resp, err = t.rt.RoundTrip(req)
	if resp == nil {
		return nil, err
	}
	span.SetAttributes(semconv.HTTPAttributesFromHTTPStatusCode(resp.StatusCode)...)
	span.SetStatus(semconv.SpanStatusFromHTTPStatusCode(resp.StatusCode))
	resp, span = addResponseDataToSpanAndWrap(resp, span)
	span.End()
	return resp, err
}

func addRequestDataToSpanAndWrap(req *http.Request, span trace.Span) (*http.Request, trace.Span) {
	req.Body, span = addBodyToSpan(req.Body, span, "http.request_body")
	span = addHeaderToSpan(req.Header, span, "http.request_headers")
	return req, span
}

func addResponseDataToSpanAndWrap(resp *http.Response, span trace.Span) (*http.Response, trace.Span) {
	resp.Body, span = addBodyToSpan(resp.Body, span, "http.response_body")
	span = addHeaderToSpan(resp.Header, span, "http.response_headers")
	return resp, span
}

func addBodyToSpan(body io.ReadCloser, span trace.Span, attributeKey string) (io.ReadCloser, trace.Span) {
	if body != nil {
		bodyStr, bodyReadCloser, bodyErr := getFirstNCharsFromReadCloser(body, cfg.MaxEntrySize)
		if bodyErr != nil {
			logger.WithError(bodyErr).Error("failed to parse response body")
			span.RecordError(bodyErr)
			span.SetStatus(codes.Error, bodyErr.Error())
			span.SetAttributes(attribute.String(attributeKey, ""))
		} else {
			span.SetAttributes(attribute.String(attributeKey, bodyStr))
			body = bodyReadCloser
		}
	}
	return body, span
}

func addHeaderToSpan(srcHeaders http.Header, span trace.Span, attributeKey string) trace.Span {
	headers := make(map[string]string)
	for k, values := range srcHeaders {
		for _, value := range values {
			headers[k] = value
		}
	}
	headersJson, jsonErr := json.Marshal(headers)
	if jsonErr != nil {
		logger.WithError(jsonErr).Error("failed to fetch request headers")
	}
	if len(headersJson) > cfg.MaxEntrySize {
		span.SetAttributes(attribute.String(attributeKey, string(headersJson[:cfg.MaxEntrySize])))
	} else {
		span.SetAttributes(attribute.String(attributeKey, string(headersJson)))
	}
	return span
}

func getFirstNCharsFromReadCloser(rc io.ReadCloser, n int) (string, io.ReadCloser, error) {
	buf := make([]byte, n)
	readBytes, err := rc.Read(buf)
	if err == io.EOF || readBytes < n {
		rc.Close()
		return string(buf[:readBytes]), io.NopCloser(bytes.NewReader(buf[:readBytes])), nil
	} else if err != nil {
		return "", rc, err
	}
	return string(buf), newMultiReadCloser(bytes.NewReader(buf), rc), nil
}

func newMultiReadCloser(tail io.Reader, rc io.ReadCloser) io.ReadCloser {
	return &multiReadCloser{
		reader: io.MultiReader(tail, rc),
		closer: rc,
	}
}

type multiReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (mrc *multiReadCloser) Read(b []byte) (int, error) {
	return mrc.reader.Read(b)
}

func (mrc *multiReadCloser) Close() error {
	return mrc.closer.Close()
}
