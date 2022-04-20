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
	defer span.End()

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
	resp = addResponseDataToSpanAndWrap(resp, span)
	return resp, err
}

func addRequestDataToSpanAndWrap(req *http.Request, span trace.Span) (*http.Request, trace.Span) {
	req.Body, span = addBodyToSpan(req.Body, span, "http.request_body")
	addHeaderToSpan(req.Header, span, "http.request_headers")
	return req, span
}

func addResponseDataToSpanAndWrap(resp *http.Response, span trace.Span) *http.Response {
	resp.Body, span = addBodyToSpan(resp.Body, span, "http.response_body")
	addHeaderToSpan(resp.Header, span, "http.response_headers")
	return resp
}

func addBodyToSpan(body io.ReadCloser, span trace.Span, attributeKey string) (io.ReadCloser, trace.Span) {
	if body != nil {
		logger.Info("adding body to span")
		bodyStr, bodyReadCloser, bodyErr := getFirstNCharsFromReadCloser(body, cfg.MaxEntrySize)
		if bodyErr != nil {
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

func addHeaderToSpan(srcHeaders http.Header, span trace.Span, attributeKey string) {
	headers := make(map[string]string)
	for k, values := range srcHeaders {
		for _, value := range values {
			headers[k] = value
		}
	}
	headersJson, jsonErr := json.Marshal(headers)
	if jsonErr != nil {
		logger.WithError(jsonErr).Error("failed to fetch request headers")
		span.RecordError(jsonErr)
		span.SetStatus(codes.Error, jsonErr.Error())
	}
	if len(headersJson) > cfg.MaxEntrySize {
		span.SetAttributes(attribute.String(attributeKey, string(headersJson[:cfg.MaxEntrySize])))
	} else {
		span.SetAttributes(attribute.String(attributeKey, string(headersJson)))
	}
}

func getFirstNCharsFromReadCloser(rc io.ReadCloser, n int) (string, io.ReadCloser, error) {
	buf := make([]byte, n)
	readBytes, err := rc.Read(buf)
	if err != nil && err != io.EOF {
		logger.WithError(err).Errorf("failed to read from readCloser %+v", rc)
		return "", rc, err
	}
	if err == io.EOF || readBytes < n {
		wrapedReadCloser := readCloserContainer{reader: bytes.NewReader(buf[:readBytes]), closer: rc}
		return string(buf[:readBytes]), &wrapedReadCloser, nil
	}
	return string(buf), newMultiReadCloser(bytes.NewReader(buf), rc), nil
}

func newMultiReadCloser(tail io.Reader, rc io.ReadCloser) io.ReadCloser {
	return &readCloserContainer{
		reader: io.MultiReader(tail, rc),
		closer: rc,
	}
}

type readCloserContainer struct {
	reader io.Reader
	closer io.Closer
}

func (mrc *readCloserContainer) Read(b []byte) (int, error) {
	return mrc.reader.Read(b)
}

func (mrc *readCloserContainer) Close() error {
	return mrc.closer.Close()
}
