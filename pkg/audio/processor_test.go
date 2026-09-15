package audio

import (
	"context"
	"testing"
)

type mockProcessor struct {
	dur float64
	err error
}

func (m *mockProcessor) Duration(ctx context.Context, audioPath string) (float64, error) {
	return m.dur, m.err
}
func (m *mockProcessor) Truncate(ctx context.Context, in, out string, durSec float64) error {
	return m.err
}
func (m *mockProcessor) Cut(ctx context.Context, in string, segs [][2]float64, out string) error {
	return m.err
}
func (m *mockProcessor) ExtractTags(ctx context.Context, audioPath string) (map[string]string, error) {
	return map[string]string{"title": "Test Title"}, m.err
}
func (m *mockProcessor) PreserveMetadata(ctx context.Context, src, dst string) error {
	return m.err
}

func TestMockAudioProcessor(t *testing.T) {
	t.Parallel()
	var proc AudioProcessor = &mockProcessor{dur: 120.5}
	dur, err := proc.Duration(context.Background(), "test.mp3")
	if err != nil || dur != 120.5 {
		t.Fatalf("Duration() = (%v, %v); want (120.5, nil)", dur, err)
	}

	tags, err := proc.ExtractTags(context.Background(), "test.mp3")
	if err != nil || tags["title"] != "Test Title" {
		t.Fatalf("ExtractTags() = (%v, %v); want title Test Title", tags, err)
	}
}

func TestDefaultProcessorInstance(t *testing.T) {
	t.Parallel()
	if DefaultProcessor == nil {
		t.Fatal("DefaultProcessor should not be nil")
	}
}
