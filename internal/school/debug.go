package school

import (
	"fmt"
	"time"
)

var debugf func(format string, args ...any)

func SetDebugLogger(fn func(format string, args ...any)) {
	debugf = fn
}

func debugLog(format string, args ...any) {
	if debugf != nil {
		debugf(format, args...)
	}
}

func debugStep(name string) func() {
	if debugf == nil {
		return func() {}
	}
	start := time.Now()
	debugLog("start %s", name)
	return func() {
		debugLog("done %s (%s)", name, time.Since(start).Round(time.Millisecond))
	}
}

func debugStepf(format string, args ...any) func() {
	return debugStep(fmt.Sprintf(format, args...))
}

func withSchoolDebugStep(name string, fn func() error) error {
	done := debugStep(name)
	defer done()
	return fn()
}
