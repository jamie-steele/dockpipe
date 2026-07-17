package mcpbridge

import (
	"net/http"
	"testing"
)

func TestExpectedAPIKeyFromRequest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		hdr    http.Header
		expect string
	}{
		{
			name:   "x-api-key",
			hdr:    http.Header{"X-Api-Key": []string{"  sekret  "}},
			expect: "sekret",
		},
		{
			name:   "bearer",
			hdr:    http.Header{"Authorization": []string{"Bearer mytoken"}},
			expect: "mytoken",
		},
		{
			name:   "apikey scheme",
			hdr:    http.Header{"Authorization": []string{"ApiKey other"}},
			expect: "other",
		},
		{
			name: "x-api-key wins over auth",
			hdr: http.Header{
				"X-Api-Key":     []string{"first"},
				"Authorization": []string{"Bearer second"},
			},
			expect: "first",
		},
		{
			name:   "empty",
			hdr:    http.Header{},
			expect: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &http.Request{Header: tc.hdr}
			if got := ExpectedAPIKeyFromRequest(r); got != tc.expect {
				t.Fatalf("got %q want %q", got, tc.expect)
			}
		})
	}
}

func TestConstantTimeEqualString(t *testing.T) {
	t.Parallel()
	if !ConstantTimeEqualString("a", "a") {
		t.Fatal("equal strings")
	}
	if ConstantTimeEqualString("a", "b") {
		t.Fatal("different strings")
	}
	if ConstantTimeEqualString("ab", "a") {
		t.Fatal("length mismatch")
	}
}
