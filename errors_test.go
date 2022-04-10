package lumigotracer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTakeStackTrace(t *testing.T) {
	stacktrace := takeStacktrace()
	assert.Contains(t, stacktrace, "runtime.goexit")
	assert.Contains(t, stacktrace, "testing.tRunner")
	assert.Contains(t, stacktrace, "go-tracer-beta.TestTakeStackTrace")
}
