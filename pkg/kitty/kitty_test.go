package kitty

import (
	"image"
	"strings"
	"testing"
)

func TestKittyClearGraphics(t *testing.T) {
	got := KittyClearGraphics()
	if !strings.Contains(got, "\x1b_Ga=d,d=A") {
		t.Errorf("unexpected clear graphics sequence: %q", got)
	}
}

func TestEncodeKittyGraphics(t *testing.T) {
	if got := EncodeKittyGraphics(nil, 10, 10, 100); got != "" {
		t.Errorf("expected empty string for nil data, got %q", got)
	}

	data := []byte("fake png data")
	got := EncodeKittyGraphics(data, 20, 20, 100)
	if !strings.HasPrefix(got, "\x1b_Ga=T,f=100,c=20,r=20;") {
		t.Errorf("unexpected encode header: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b\\") {
		t.Errorf("expected escape termination, got %q", got)
	}
}

func TestWrapTitleLines(t *testing.T) {
	short := WrapTitleLines("Short")
	if len(short) != 1 || short[0] != "Short" {
		t.Errorf("unexpected wrap for short title: %v", short)
	}

	empty := WrapTitleLines("")
	if len(empty) != 1 || empty[0] != "Podcast" {
		t.Errorf("unexpected wrap for empty title: %v", empty)
	}

	long := WrapTitleLines("A Very Long Podcast Title That Needs To Be Split Across Several Lines Cleanly")
	if len(long) < 2 {
		t.Errorf("expected multiple lines for long title, got %d", len(long))
	}
}

func TestGenerateGenericCover(t *testing.T) {
	img := GenerateGenericCover("Tech News Daily")
	if img == nil {
		t.Fatalf("expected non-nil image")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 256 || bounds.Dy() != 256 {
		t.Errorf("expected 256x256 dimensions, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestScaleImageThumbnail(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	scaled := ScaleImageThumbnail(src, 50, 50)
	if scaled == nil {
		t.Fatalf("expected scaled image")
	}
	if scaled.Bounds().Dx() != 50 || scaled.Bounds().Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", scaled.Bounds().Dx(), scaled.Bounds().Dy())
	}
}

func TestClearImageMemoryCache(t *testing.T) {
	ClearImageMemoryCache()
}
