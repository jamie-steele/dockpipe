package infrastructure

import "testing"

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

