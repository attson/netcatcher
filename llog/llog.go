// Package llog provides leveled logging helpers that produce a uniform
// "[level] component: message" format across the codebase. The backing
// writer (see logbuffer) parses the [level] prefix to classify each entry,
// so these helpers are the canonical way to emit logs.
package llog

import (
	"fmt"
	"log"
)

const (
	levelDebug = "debug"
	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

func Debugf(component, format string, args ...any) { emit(levelDebug, component, format, args...) }
func Infof(component, format string, args ...any)  { emit(levelInfo, component, format, args...) }
func Warnf(component, format string, args ...any)  { emit(levelWarn, component, format, args...) }
func Errorf(component, format string, args ...any) { emit(levelError, component, format, args...) }

func emit(level, component, format string, args ...any) {
	log.Printf("[%s] %s: %s", level, component, fmt.Sprintf(format, args...))
}
