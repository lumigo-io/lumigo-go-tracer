package lumigotracer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type provider struct {
	s trace.Span
}

func (p *provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return &myTracer{s: p.s}
}

type myTracer struct {
	s trace.Span
}

func (t *myTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return context.Background(), t.s
}

type mySpan struct {
	attrs     string
	endCalled bool
	p         trace.TracerProvider
}

func (s *mySpan) End(options ...trace.SpanEndOption) {
	s.endCalled = true
}

func (s *mySpan) AddEvent(name string, options ...trace.EventOption)  {}
func (s *mySpan) IsRecording() bool                                   { return true }
func (s *mySpan) RecordError(err error, options ...trace.EventOption) {}
func (s *mySpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}
func (s *mySpan) SetStatus(code codes.Code, description string) {}
func (s *mySpan) SetName(name string)                           {}
func (s *mySpan) SetAttributes(kv ...attribute.KeyValue) {
	for _, kvInner := range kv {
		s.attrs += fmt.Sprintf("%s:%s;", kvInner.Key, kvInner.Value.AsString()) + "\n"
	}
}
func (s *mySpan) TracerProvider() trace.TracerProvider {
	return s.p
}

type readCloser struct {
	readErr, closeErr error
	readSize          int
}

func (rc readCloser) Read(p []byte) (n int, err error) {
	return rc.readSize, rc.readErr
}

func (rc readCloser) Close() error {
	return rc.closeErr
}
func cleanDates(str string) string {
	m1 := regexp.MustCompile(`"Date":"\w\w\w, ((\d\d)|(\d)) \w\w\w \d\d\d\d \d\d:\d\d:\d\d \w\w\w"`)
	return m1.ReplaceAllString(str, `"Date":"Fri, 07 Dec 1979 19:00:18 GMT"`)
}

func TestTransport(t *testing.T) {
	err := loadConfig(Config{Token: "test"})
	assert.NoError(t, err)

	content := []byte("Hello, world!")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}))

	defer ts.Close()

	testcases := []struct {
		testname string
		readFunc func(body io.ReadCloser) ([]byte, error)
		expected []byte
	}{
		{
			testname: "read all",
			readFunc: func(body io.ReadCloser) ([]byte, error) { return io.ReadAll(body) },
			expected: []byte("Hello, world!"),
		},
		{
			testname: "partial read",
			readFunc: func(body io.ReadCloser) ([]byte, error) { return io.ReadAll(io.LimitReader(body, 5)) },
			expected: []byte("Hello"),
		},
		{
			testname: "no read",
			readFunc: func(body io.ReadCloser) ([]byte, error) { return []byte{}, nil },
			expected: []byte{},
		},
	}

	for _, tc := range testcases {
		defer func() { getTracerProvider = otel.GetTracerProvider }()
		t.Run(tc.testname, func(t *testing.T) {
			spanMock := &mySpan{}
			lTrans := NewTransport(http.DefaultTransport)
			getTracerProvider = func() trace.TracerProvider { return &provider{s: spanMock} }
			c := http.Client{Transport: lTrans}
			res, err := c.Post(ts.URL, "application/json", bytes.NewReader([]byte("post body")))
			if err != nil {
				assert.FailNow(t, err.Error())
			}
			body, err := tc.readFunc(res.Body)
			assert.NoError(t, err)
			res.Body.Close()
			assert.Equal(t, tc.expected, body)
			assert.Equal(t, true, spanMock.endCalled)
			assert.Equal(t, fmt.Sprintf(`http.method:POST;
http.url:%s;
http.request_content_length:;
http.scheme:http;
http.host:%s;
http.flavor:1.1;
http.target:;
http.host:%s;
http.request_body:post body;
http.request_headers:{"Content-Type":"application/json"};
http.status_code:;
http.response_body:Hello, world!;
http.response_headers:{"Content-Length":"13","Content-Type":"text/plain; charset=utf-8","Date":"Fri, 07 Dec 1979 19:00:18 GMT"};
`, ts.URL, ts.URL[7:], ts.URL[7:]), cleanDates(spanMock.attrs))

		})
	}
}

func TestGetFirstNCharsFromReader(t *testing.T) {
	rc := io.NopCloser(bytes.NewReader([]byte("Hello, world!")))
	first5Char, origReader, err := getFirstNCharsFromReadCloser(rc, 5)
	assert.NoError(t, err)
	assert.Equal(t, "Hello", first5Char)
	allChars, err := io.ReadAll(origReader)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), allChars)
}

func TestGetFirstNCharsFromReaderWithErr(t *testing.T) {
	rc := &readCloser{readErr: errors.New("test")}
	first5Char, _, err := getFirstNCharsFromReadCloser(rc, 5)
	assert.Error(t, err)
	assert.Equal(t, "", first5Char)
}

func TestTransportBodyReadError(t *testing.T) {
	err := loadConfig(Config{Token: "test"})
	assert.NoError(t, err)
	spanMock := &mySpan{}
	lTrans := NewTransport(http.DefaultTransport)
	defer func() { getTracerProvider = otel.GetTracerProvider }()
	getTracerProvider = func() trace.TracerProvider { return &provider{s: spanMock} }
	c := http.Client{Transport: lTrans}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Hello, world!")); err != nil {
			t.Fatal(err)
		}
	}))
	r, _ := http.NewRequest("POST", ts.URL, bytes.NewReader([]byte("post body")))
	r.Header.Set("Content-Type", "application/json")
	_, err = c.Post(ts.URL, "application/json", readCloser{readErr: errors.New("test")})
	assert.Error(t, err)
	assert.Equal(t, true, spanMock.endCalled)
	assert.Equal(t, fmt.Sprintf(`http.method:POST;
http.url:%s;
http.scheme:http;
http.host:%s;
http.flavor:1.1;
http.target:;
http.host:%s;
http.request_body:;
http.request_headers:{"Content-Type":"application/json"};
`, ts.URL, ts.URL[7:], ts.URL[7:]), cleanDates(spanMock.attrs))
}
