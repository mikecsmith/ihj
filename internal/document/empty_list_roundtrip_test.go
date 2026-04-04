package document_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/document"
)

func TestEmptyListItem_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		md     string
		expect string
	}{
		{
			"empty item only",
			"# Acceptance Criteria\n\n- \n",
			"# Acceptance Criteria\n\n- \n",
		},
		{
			"empty item with content after",
			"# Acceptance Criteria\n\n-\n\nSome text after.\n",
			"# Acceptance Criteria\n\n- \n\nSome text after.\n",
		},
		{
			"empty item mid-list",
			"- first\n- \n- third\n",
			"- first\n- \n- third\n",
		},
		{
			"multiple empty items stacked",
			"- \n- \n- \n",
			"- \n- \n- \n",
		},
		{
			"dash only no space",
			"-\n",
			"- \n",
		},
		{
			"non-empty preserved",
			"- first\n- second\n",
			"- first\n- second\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := document.ParseMarkdownString(tt.md)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			out := document.RenderMarkdown(node)
			if out != tt.expect {
				t.Errorf("render:\n  got:  %q\n  want: %q", out, tt.expect)
			}

			// Round-trip again to check stability — the output must not
			// change on subsequent parse/render cycles.
			node2, err := document.ParseMarkdownString(out)
			if err != nil {
				t.Fatalf("re-parse error: %v", err)
			}
			out2 := document.RenderMarkdown(node2)
			if out != out2 {
				t.Errorf("round-trip unstable:\n  first:  %q\n  second: %q", out, out2)
			}
		})
	}
}
