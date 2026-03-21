package infrastructure

import (
	"os"
	"testing"
)

func TestRenderBannerForWidth(t *testing.T) {
	if got := renderBannerForWidth(69); got != compactBanner {
		t.Fatalf("width 69 should use compact banner")
	}
	if got := renderBannerForWidth(70); got != banner {
		t.Fatalf("width 70 should use full banner")
	}
}

func TestShouldShowSpinner(t *testing.T) {
	if shouldShowSpinner(59) {
		t.Fatalf("spinner should be hidden for narrow width")
	}
	if !shouldShowSpinner(60) {
		t.Fatalf("spinner should show at width 60")
	}
}

func TestUseDockerInteractiveTTYNonTTYFiles(t *testing.T) {
	a, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	c, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if useDockerInteractiveTTY(a, b, c) {
		t.Fatal("temp files are not terminals")
	}
}

