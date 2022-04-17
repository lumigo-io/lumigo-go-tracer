package lumigotracer

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestTransport(t *testing.T) {
	content := []byte("Hello, world!")

	ctx := context.Background()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		testname  string
		transport http.RoundTripper
	}{
		{
			testname: "http default transport",
			// transport: http.DefaultTransport,
		},
		{
			testname: "http default transport",
			// transport: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testname, func(t *testing.T) {
			tr := NewTransport(http.DefaultTransport)

			c := http.Client{Transport: tr}
			res, err := c.Do(r)
			if err != nil {
				t.Fatal(err)
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(body, content) {
				t.Fatalf("unexpected content: got %d, expected %d", len(body), len(content))
			}
		})
	}
}

const readSize = 42

type readCloser struct {
	readErr, closeErr error
}

func (rc readCloser) Read(p []byte) (n int, err error) {
	return readSize, rc.readErr
}
func (rc readCloser) Close() error {
	return rc.closeErr
}

type span struct {
	trace.Span

	ended       bool
	recordedErr error

	statusCode codes.Code
	statusDesc string
}

func (s *span) End(...trace.SpanEndOption) {
	s.ended = true
}

func (s *span) RecordError(err error, _ ...trace.EventOption) {
	s.recordedErr = err
}

func (s *span) SetStatus(c codes.Code, d string) {
	s.statusCode, s.statusDesc = c, d
}

func TestWrappedBodyRead(t *testing.T) {
	s := new(span)
	wb := &wrappedBody{span: trace.Span(s), body: readCloser{}}
	n, err := wb.Read([]byte{})
	assert.Equal(t, readSize, n, "wrappedBody returned wrong bytes")
	assert.NoError(t, err)
}

func TestWrappedBodyReadEOFError(t *testing.T) {
	s := new(span)
	wb := &wrappedBody{span: trace.Span(s), body: readCloser{readErr: io.EOF}}
	n, err := wb.Read([]byte{})
	assert.Equal(t, readSize, n, "wrappedBody returned wrong bytes")
	assert.Equal(t, io.EOF, err)
}

func TestWrappedBodyReadError(t *testing.T) {
	s := new(span)
	expectedErr := errors.New("test")
	wb := &wrappedBody{span: trace.Span(s), body: readCloser{readErr: expectedErr}}
	n, err := wb.Read([]byte{})
	assert.Equal(t, readSize, n, "wrappedBody returned wrong bytes")
	assert.Equal(t, expectedErr, err)
}

func TestWrappedBodyClose(t *testing.T) {
	s := new(span)
	wb := &wrappedBody{span: trace.Span(s), body: readCloser{}}
	assert.NoError(t, wb.Close())
}

func TestWrappedBodyCloseError(t *testing.T) {
	s := new(span)
	expectedErr := errors.New("test")
	wb := &wrappedBody{span: trace.Span(s), body: readCloser{closeErr: expectedErr}}
	assert.Equal(t, expectedErr, wb.Close())
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
