package lumigotracer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/lambda/messages"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context/ctxhttp"
)

var (
	mockLambdaContext = lambdacontext.LambdaContext{
		AwsRequestID:       "123",
		InvokedFunctionArn: "arn:partition:service:region:account-id:resource-type:resource-id",
		Identity: lambdacontext.CognitoIdentity{
			CognitoIdentityID:     "someId",
			CognitoIdentityPoolID: "somePoolId",
		},
		ClientContext: lambdacontext.ClientContext{},
	}
	mockContext = lambdacontext.NewContext(context.TODO(), &mockLambdaContext)
)

type expected struct {
	val interface{}
	err error
}

type wrapperTestSuite struct {
	suite.Suite
}

func TestSetupWrapperSuite(t *testing.T) {
	suite.Run(t, &wrapperTestSuite{})
}

func (w *wrapperTestSuite) SetupTest() {
	_ = os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "testFunction")
	_ = os.Setenv("AWS_REGION", "us-east-1")
	_ = os.Setenv("AWS_LAMBDA_FUNCTION_VERSION", "$LATEST")
	_ = os.Setenv("AWS_LAMBDA_LOG_STREAM_NAME", "2021/12/06/[$LATEST]2f4f26a6224b421c86bc4570bb7bf84b")
	_ = os.Setenv("AWS_LAMBDA_LOG_GROUP_NAME", "/aws/lambda/helloworld-37")
	_ = os.Setenv("AWS_EXECUTION_ENV", "go")
	_ = os.Setenv("_X_AMZN_TRACE_ID", "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1")
	os.Unsetenv("IS_WARM_START")
}

func (w *wrapperTestSuite) TearDownTest() {
	_ = os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME")
	_ = os.Unsetenv("AWS_REGION")
	_ = os.Unsetenv("AWS_LAMBDA_LOG_STREAM_NAME")
	_ = os.Unsetenv("AWS_LAMBDA_LOG_GROUP_NAME")
	_ = os.Unsetenv("AWS_LAMBDA_FUNCTION_VERSION")
	_ = os.Unsetenv("_X_AMZN_TRACE_ID")
	assert.NoError(w.T(), deleteAllFiles())
}

func (w *wrapperTestSuite) TestLambdaHandlerSignatures() {

	hello := func(s string) string {
		return fmt.Sprintf("Hello %s!", s)
	}

	testCases := []struct {
		name     string
		input    interface{}
		expected expected
		handler  interface{}
	}{
		{
			name:     "input: string, no context",
			input:    "test",
			expected: expected{`"Hello test!"`, nil},
			handler: func(name string) (string, error) {
				return hello(name), nil
			},
		},
		{
			name:     "input: string, with context",
			input:    "test",
			expected: expected{`"Hello test!"`, nil},
			handler: func(ctx context.Context, name string) (string, error) {
				return hello(name), nil
			},
		},
		{
			name:     "input: none, error on return",
			input:    nil,
			expected: expected{"", errors.New("failed")},
			handler: func() (interface{}, error) {
				return nil, errors.New("failed")
			},
		},
		{
			name:     "input: event, error on return",
			input:    "test",
			expected: expected{"", errors.New("failed")},
			handler: func(e interface{}) (interface{}, error) {
				return nil, errors.New("failed")
			},
		},
		{
			name:     "input: context & event, error on return",
			input:    "test",
			expected: expected{"", errors.New("failed")},
			handler: func(ctx context.Context, e interface{}) (interface{}, error) {
				return nil, errors.New("failed")
			},
		},
		{
			name:     "input: event, lambda Invoke error on return",
			input:    "test",
			expected: expected{"", messages.InvokeResponse_Error{Message: "message", Type: "type"}},
			handler: func(e interface{}) (interface{}, error) {
				return nil, messages.InvokeResponse_Error{Message: "message", Type: "type"}
			},
		},
		{
			name:     "input: struct event, response number",
			input:    struct{ Port int }{9090},
			expected: expected{`9090`, nil},
			handler: func(event struct{ Port int }) (int, error) {
				return event.Port, nil
			},
		},
		{
			name:     "input: struct event, response as struct",
			input:    9090,
			expected: expected{`{"Port":9090}`, nil},
			handler: func(event int) (struct{ Port int }, error) {
				return struct{ Port int }{event}, nil
			},
		},
		{
			name:     "input: struct event, with error string",
			input:    9090,
			expected: expected{"", errors.New("failed error")},
			handler: func(event int) (struct{ Port int }, error) {
				return struct{ Port int }{}, errors.New("failed error")
			},
		},
		{
			name:     "input: struct event, with error string",
			input:    9090,
			expected: expected{"", &os.SyscallError{Err: errors.New("fail")}},
			handler: func(event int) (struct{ Port int }, error) {
				return struct{ Port int }{}, &os.SyscallError{Err: errors.New("fail")}
			},
		},
	}
	// setup

	// test invocation via a Handler
	for i, testCase := range testCases {
		testCase := testCase
		w.T().Run(fmt.Sprintf("handlerTestCase[%d] %s", i, testCase.name), func(t *testing.T) {
			inputPayload, _ := json.Marshal(testCase.input)

			lambdaHandler := WrapHandler(testCase.handler, &Config{Token: "token"})

			handler := reflect.ValueOf(lambdaHandler)
			handlerType := handler.Type()
			response := handler.Call([]reflect.Value{reflect.ValueOf(mockContext), reflect.ValueOf(inputPayload)})

			if testCase.expected.err != nil {
				assert.Equal(t, testCase.expected.err, response[handlerType.NumOut()-1].Interface())
			} else {
				assert.Nil(t, response[handlerType.NumOut()-1].Interface())
				responseValMarshalled, _ := json.Marshal(response[0].Interface())
				assert.Equal(t, testCase.expected.val, string(responseValMarshalled))
			}
		})
		assert.NoError(w.T(), deleteAllFiles())
	}
}

func (w *wrapperTestSuite) TestLambdaHandlerE2ELocal() {
	hello := func(s string) string {
		return fmt.Sprintf("Hello %s!", s)
	}
	content := []byte("Hello, world!")
	ts := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		if _, err := wr.Write(content); err != nil {
			w.T().Fatal(err)
		}
	}))
	testCases := []struct {
		name       string
		lambdaType string
		isHttp     bool
		input      interface{}
		expected   expected
		handler    interface{}
	}{
		{
			name:     "input: string, with context",
			input:    "test",
			expected: expected{`"Hello test!"`, nil},
			handler: func(ctx context.Context, name string) (string, error) {
				return hello(name), nil
			},
		},
		{
			name:     "input: struct event, response as struct",
			input:    9090,
			expected: expected{`{"Port":9090}`, nil},
			handler: func(event int) (struct{ Port int }, error) {
				return struct{ Port int }{event}, nil
			},
		},
		{
			name:     "input: struct event, return error",
			input:    9090,
			expected: expected{"", errors.New("failed error")},
			handler: func(event int) (*struct{ Port int }, error) {
				return nil, errors.New("failed error")
			},
		},
		{
			name:     "ctxhttp transport",
			input:    "test",
			isHttp:   true,
			expected: expected{`"Hello test!"`, nil},
			handler: func(ctx context.Context, name string) (string, error) {
				postBody, _ := json.Marshal(map[string]string{
					"name": strings.Repeat("test", 512),
				})
				r, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL, bytes.NewBuffer(postBody))
				if err != nil {
					w.T().Fatal(err)
				}
				r.Header.Set("Agent", "test")
				c := &http.Client{Transport: NewTransport(http.DefaultTransport)}

				res, err := ctxhttp.Do(ctx, c, r)
				if err != nil {
					w.T().Fatal(err)
				}

				_, err = ioutil.ReadAll(res.Body)
				if err != nil {
					w.T().Fatal(err)
				}

				return hello(name), nil
			},
		},
	}
	testContext := lambdacontext.NewContext(mockContext, &mockLambdaContext)
	for i, testCase := range testCases {
		w.T().Run(fmt.Sprintf("handlerTestCase[%d] %s", i, testCase.name), func(t *testing.T) {

			inputPayload, _ := json.Marshal(testCase.input)
			lambdaHandler := WrapHandler(testCase.handler, &Config{Token: "token", debug: true})

			handler := reflect.ValueOf(lambdaHandler)
			_ = handler.Call([]reflect.Value{reflect.ValueOf(testContext), reflect.ValueOf(inputPayload)})

			spans, err := readSpansFromFile()
			assert.NoError(w.T(), err)

			startFuncSpan := spans.startFileSpans[0]
			assert.Equal(w.T(), "account-id", startFuncSpan.Account)
			assert.Equal(w.T(), "token", startFuncSpan.Token)
			assert.Equal(w.T(), os.Getenv("AWS_LAMBDA_FUNCTION_NAME"), startFuncSpan.LambdaName)
			assert.Equal(w.T(), "go", startFuncSpan.Runtime)
			assert.Equal(w.T(), os.Getenv("AWS_LAMBDA_LOG_STREAM_NAME"), startFuncSpan.SpanInfo.LogStreamName)
			assert.Equal(w.T(), os.Getenv("AWS_LAMBDA_LOG_GROUP_NAME"), startFuncSpan.SpanInfo.LogGroupName)
			assert.Equal(w.T(), "1-5759e988-bd862e3fe1be46a994272793", startFuncSpan.SpanInfo.TraceID.Root)
			assert.Equal(w.T(), os.Getenv("AWS_REGION"), startFuncSpan.Region)
			assert.Equal(w.T(), "bd862e3fe1be46a994272793", startFuncSpan.TransactionID)
			assert.Equal(w.T(), string(inputPayload), startFuncSpan.Event)
			assert.Equal(w.T(), version, startFuncSpan.SpanInfo.TracerVersion.Version)
			if i > 0 { // check if second test
				assert.Equal(w.T(), "warm", startFuncSpan.LambdaReadiness)
			} else {
				assert.Equal(w.T(), "cold", startFuncSpan.LambdaReadiness)
			}

			if len(spans.endFileSpans) > 1 {
				httpSpan := spans.endFileSpans[0]
				if httpSpan.SpanType == "http" {
					assert.NotNil(w.T(), httpSpan.SpanInfo.HttpInfo)
					assert.Equal(w.T(), ts.URL, "http://"+httpSpan.SpanInfo.HttpInfo.Host)
					assert.Equal(w.T(), ts.URL, "http://"+*httpSpan.SpanInfo.HttpInfo.Request.URI)
					assert.Equal(w.T(), "POST", *httpSpan.SpanInfo.HttpInfo.Request.Method)
					assert.Equal(w.T(), 2048, len(httpSpan.SpanInfo.HttpInfo.Request.Body))
					assert.Contains(w.T(), httpSpan.SpanInfo.HttpInfo.Request.Body, `{"name":"test`)
					assert.Contains(w.T(), httpSpan.SpanInfo.HttpInfo.Request.Headers, `"Agent":"test"`)
				}
			}
			// endFuncSpan.

			endFuncSpan := spans.endFileSpans[len(spans.endFileSpans)-1]
			assert.Equal(w.T(), "account-id", endFuncSpan.Account)
			assert.Equal(w.T(), "token", endFuncSpan.Token)
			assert.Equal(w.T(), os.Getenv("AWS_LAMBDA_FUNCTION_NAME"), endFuncSpan.LambdaName)
			assert.Equal(w.T(), "go", endFuncSpan.Runtime)
			assert.Equal(w.T(), os.Getenv("AWS_LAMBDA_LOG_STREAM_NAME"), endFuncSpan.SpanInfo.LogStreamName)
			assert.Equal(w.T(), os.Getenv("AWS_LAMBDA_LOG_GROUP_NAME"), endFuncSpan.SpanInfo.LogGroupName)
			assert.Equal(w.T(), "1-5759e988-bd862e3fe1be46a994272793", endFuncSpan.SpanInfo.TraceID.Root)
			assert.Equal(w.T(), os.Getenv("AWS_REGION"), endFuncSpan.Region)
			assert.Equal(w.T(), "bd862e3fe1be46a994272793", endFuncSpan.TransactionID)
			assert.Equal(w.T(), string(inputPayload), endFuncSpan.Event)
			if i > 0 { // check if second test
				assert.Equal(w.T(), "warm", endFuncSpan.LambdaReadiness)
			} else {
				assert.Equal(w.T(), "cold", endFuncSpan.LambdaReadiness)
			}

			assert.Equal(w.T(), version, startFuncSpan.SpanInfo.TracerVersion.Version)

			if startFuncSpan.SpanType == "http" {
				assert.Equal(w.T(), 200, startFuncSpan.SpanInfo.HttpInfo.Response.StatusCode)
				assert.Equal(w.T(), `Hello, world!`, startFuncSpan.SpanInfo.HttpInfo.Response.Body)
				assert.Contains(w.T(), `"Content-Length": "13"`, startFuncSpan.SpanInfo.HttpInfo.Response.Headers)
			}

			if testCase.expected.err != nil {
				assert.NotNil(w.T(), endFuncSpan.SpanError)
				assert.Equal(w.T(), testCase.expected.err.Error(), endFuncSpan.SpanError.Message)
				assert.Equal(w.T(), reflect.TypeOf(testCase.expected.err).String(), endFuncSpan.SpanError.Type)

				assert.Contains(t, endFuncSpan.SpanError.Stacktrace, "lumigo-go-tracer.WrapHandler.func1")
				assert.Contains(t, endFuncSpan.SpanError.Stacktrace, "lumigo-go-tracer.(*wrapperTestSuite).TestLambdaHandlerE2ELocal.func7")
			} else {
				assert.NotNil(w.T(), endFuncSpan.LambdaResponse)
				assert.Equal(w.T(), testCase.expected.val, *endFuncSpan.LambdaResponse)
			}
		})

		assert.NoError(w.T(), deleteAllFiles())
	}
}

func (w *wrapperTestSuite) TestCheckFailWriteSpanHandler() {
	// verify that the spans dir is empty
	dirEntries, err := os.ReadDir(SPANS_DIR)
	assert.NoError(w.T(), err)
	assert.Equal(w.T(), 0, len(dirEntries))

	recoverAndCheckFailWriteSpan()
	dirEntries, err = os.ReadDir(SPANS_DIR)
	assert.NoError(w.T(), err)
	assert.Equal(w.T(), 1, len(dirEntries))
	assert.NoError(w.T(), deleteAllFiles())

}
func (w *wrapperTestSuite) TestCheckFailWriteSpanHandler_HandlerSuccess() {
	handlerToWrap := func(s string) string {
		return fmt.Sprintf("Hello %s!", s)
	}
	inputPayload, _ := json.Marshal("test")
	lambdaHandler := WrapHandler(handlerToWrap, &Config{Token: "token", debug: true})

	handler := reflect.ValueOf(lambdaHandler)
	_ = handler.Call([]reflect.Value{reflect.ValueOf(context.Background()), reflect.ValueOf(inputPayload)})

	dirEntries, err := os.ReadDir(SPANS_DIR)
	assert.NoError(w.T(), err)
	assert.Equal(w.T(), 2, len(dirEntries))
	var startSpanFilename, endSpanFilename string
	for _, dirEntry := range dirEntries {
		if strings.Contains(dirEntry.Name(), "span") {
			startSpanFilename = dirEntry.Name()
		} else if strings.Contains(dirEntry.Name(), "end") {
			endSpanFilename = dirEntry.Name()
		}
		assert.NotEqual(w.T(), "balagan_stop", dirEntry.Name())
	}
	assert.NotEmpty(w.T(), startSpanFilename)
	assert.NotEmpty(w.T(), endSpanFilename)

	fileBytes, err := ioutil.ReadFile(path.Join(SPANS_DIR, startSpanFilename))
	assert.NoError(w.T(), err)
	assert.NotEmpty(w.T(), fileBytes)
	assert.NoError(w.T(), deleteAllFiles())
}

func (w *wrapperTestSuite) TestCheckFailWriteSpanHandler_FailLoadConfig() {
	handlerToWrap := func(s string) string {
		return fmt.Sprintf("Hello %s!", s)
	}
	lambdaHandler := WrapHandler(handlerToWrap, &Config{Token: "", debug: true})

	handler := reflect.ValueOf(lambdaHandler)
	_ = handler.Call([]reflect.Value{reflect.ValueOf("test")})

	dirEntries, err := os.ReadDir(SPANS_DIR)
	assert.NoError(w.T(), err)
	assert.Equal(w.T(), 1, len(dirEntries))

	assert.Equal(w.T(), "balagan_stop", dirEntries[0].Name())
}
