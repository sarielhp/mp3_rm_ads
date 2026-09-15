package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestOrSubstitutesDiscardForNil(t *testing.T) {
	r := Or(nil)
	if r == nil {
		t.Fatal("Or(nil) returned nil")
	}
	r.Infof("must not panic %d", 1)
	r.Detailf("must not panic")
	r.Warnf("must not panic")
}

func TestOrPassesThroughNonNil(t *testing.T) {
	l := &Lines{}
	if got := Or(l); got != Reporter(l) {
		t.Errorf("Or replaced a non-nil Reporter")
	}
}

func TestWriterRoutesStreamsAndAppendsNewline(t *testing.T) {
	var out, errOut bytes.Buffer
	r := Writer(&out, &errOut, false)
	r.Infof("hello %s", "world")
	r.Warnf("uh oh")

	if out.String() != "hello world\n" {
		t.Errorf("stdout = %q", out.String())
	}
	if errOut.String() != "uh oh\n" {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestWriterDoesNotDoubleNewline(t *testing.T) {
	var out bytes.Buffer
	Writer(&out, nil, false).Infof("already\n")
	if out.String() != "already\n" {
		t.Errorf("got %q, want one trailing newline", out.String())
	}
}

func TestWriterDetailRequiresVerbose(t *testing.T) {
	var quiet, loud bytes.Buffer
	Writer(&quiet, nil, false).Detailf("detail")
	Writer(&loud, nil, true).Detailf("detail")

	if quiet.String() != "" {
		t.Errorf("non-verbose writer emitted detail: %q", quiet.String())
	}
	if !strings.Contains(loud.String(), "detail") {
		t.Errorf("verbose writer dropped detail: %q", loud.String())
	}
}

func TestWriterNilStreamIsSilent(t *testing.T) {
	r := Writer(nil, nil, true)
	r.Infof("dropped")
	r.Detailf("dropped")
	r.Warnf("dropped")
}

func TestLinesCollectsByLevel(t *testing.T) {
	l := &Lines{}
	l.Infof("i%d", 1)
	l.Detailf("d%d", 2)
	l.Warnf("w%d", 3)

	if len(l.Info) != 1 || l.Info[0] != "i1" {
		t.Errorf("Info = %v", l.Info)
	}
	if len(l.Detail) != 1 || l.Detail[0] != "d2" {
		t.Errorf("Detail = %v", l.Detail)
	}
	if len(l.Warn) != 1 || l.Warn[0] != "w3" {
		t.Errorf("Warn = %v", l.Warn)
	}
	if got := strings.Join(l.All(), ","); got != "i1,d2,w3" {
		t.Errorf("All() = %q", got)
	}
}

func TestDiscardIsUsable(t *testing.T) {
	Discard.Infof("x")
	Discard.Detailf("x")
	Discard.Warnf("x")
}
