package lumigotracer

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
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
		t.Run(tc.testname, func(t *testing.T) {
			c := http.Client{Transport: NewTransport(http.DefaultTransport)}
			res, err := c.Do(r)
			assert.NoError(t, err)

			body, err := tc.readFunc(res.Body)
			assert.NoError(t, err)
			defer res.Body.Close()
			assert.Equal(t, tc.expected, body)
			// span := getSpan(t)

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
