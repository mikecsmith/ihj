package document_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/document"
)

// TestParseADF_NilInput tests that renderers handle nil nodes gracefully.
// Despite its name, this test exercises RenderMarkdown and RenderANSI, not ParseADF.
func TestParseADF_NilInput(t *testing.T) {
	if md := document.RenderMarkdown(nil); md != "" {
		t.Errorf("RenderMarkdown(nil) = %q; want empty", md)
	}
	if out := document.RenderANSI(nil, document.ANSIConfig{}); out != "" {
		t.Errorf("RenderANSI(nil) = %q; want empty", out)
	}
}
