package cli

import (
	"strings"
	"testing"
)

func TestPrintBanner(t *testing.T) {
	tests := []struct {
		name   string
		hidden bool
		want   bool // whether any output is expected
	}{
		{name: "shown", hidden: false, want: true},
		{name: "hidden", hidden: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			printBanner(&buf, tt.hidden)

			got := buf.Len() > 0
			if got != tt.want {
				t.Fatalf("printBanner(hidden=%v) produced output=%v, want %v", tt.hidden, got, tt.want)
			}

			if tt.want {
				out := buf.String()
				if !strings.Contains(out, "self-hosted GitHub Actions runner") {
					t.Errorf("banner output missing tagline:\n%s", out)
				}
			}
		})
	}
}
