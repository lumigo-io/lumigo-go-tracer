package lumigotracer

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ErrInvalidToken an error about a missing token
var ErrInvalidToken = errors.New("invalid Token. Go to Lumigo Settings to get a valid token")

// defaultStackLength specifies the default maximum size of a stack trace.
const defaultStackLength = 64

func recoverWithLogs() {
	if err := recover(); err != nil {
		logger.WithFields(logrus.Fields{
			"stacktrace": takeStacktrace(),
			"error":      err,
		}).Error("an exception occurred in lumigo's code")
	}
}

func takeStacktrace() string {
	var builder strings.Builder
	pcs := make([]uintptr, defaultStackLength)

	// +2 to exclude runtime.Callers and takeStacktrace
	numFrames := runtime.Callers(2+int(0), pcs)
	if numFrames == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:numFrames])
	for i := 0; ; i++ {
		frame, more := frames.Next()
		if i != 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(frame.Function)
		builder.WriteByte('\n')
		builder.WriteByte('\t')
		builder.WriteString(frame.File)
		builder.WriteByte(':')
		builder.WriteString(strconv.Itoa(frame.Line))
		if !more {
			break
		}
	}
	return builder.String()
}
