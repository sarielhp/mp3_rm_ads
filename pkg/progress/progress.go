// Package progress carries human-readable progress out of library code.
//
// Library packages describe what they are doing; they do not decide whether,
// where, or how loudly it appears. A caller that wants no output passes
// nothing and gets Discard, so there is no quiet flag to thread through a
// call chain and no fmt.Print to corrupt a full-screen UI.
package progress

import (
	"fmt"
	"io"
	"strings"
)

// Reporter receives progress from a library operation. Detailf is for output
// a caller shows only when asked for detail; Warnf is for conditions that did
// not stop the operation.
type Reporter interface {
	Infof(format string, args ...any)
	Detailf(format string, args ...any)
	Warnf(format string, args ...any)
}

type discard struct{}

func (discard) Infof(string, ...any)   {}
func (discard) Detailf(string, ...any) {}
func (discard) Warnf(string, ...any)   {}

// Discard drops everything reported to it.
var Discard Reporter = discard{}

// Or substitutes Discard for a nil Reporter, so library code can report
// unconditionally without nil-checking at every call.
func Or(r Reporter) Reporter {
	if r == nil {
		return Discard
	}
	return r
}

type writer struct {
	out     io.Writer
	err     io.Writer
	verbose bool
}

// Writer reports to out, sending Warnf to errOut and dropping Detailf unless
// verbose. A nil writer discards that stream.
func Writer(out, errOut io.Writer, verbose bool) Reporter {
	return &writer{out: out, err: errOut, verbose: verbose}
}

func (w *writer) Infof(format string, args ...any) { emit(w.out, format, args...) }

func (w *writer) Detailf(format string, args ...any) {
	if w.verbose {
		emit(w.out, format, args...)
	}
}

func (w *writer) Warnf(format string, args ...any) { emit(w.err, format, args...) }

func emit(dst io.Writer, format string, args ...any) {
	if dst == nil {
		return
	}
	s := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	fmt.Fprint(dst, s)
}

// Lines collects reported text instead of displaying it, for callers that
// render it themselves later — a full-screen UI, or a test.
type Lines struct {
	Info   []string
	Detail []string
	Warn   []string
}

func (l *Lines) Infof(format string, args ...any) {
	l.Info = append(l.Info, fmt.Sprintf(format, args...))
}

func (l *Lines) Detailf(format string, args ...any) {
	l.Detail = append(l.Detail, fmt.Sprintf(format, args...))
}

func (l *Lines) Warnf(format string, args ...any) {
	l.Warn = append(l.Warn, fmt.Sprintf(format, args...))
}

// All returns every line collected, in Info, Detail, Warn order.
func (l *Lines) All() []string {
	out := make([]string, 0, len(l.Info)+len(l.Detail)+len(l.Warn))
	out = append(out, l.Info...)
	out = append(out, l.Detail...)
	return append(out, l.Warn...)
}
