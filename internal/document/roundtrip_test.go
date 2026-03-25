package document

import "testing"

// TestParseADF_NilInput tests that renderers handle nil nodes gracefully.
// Despite its name, this test exercises RenderMarkdown and RenderANSI, not ParseADF.
func TestParseADF_NilInput(t *testing.T) {
	if md := RenderMarkdown(nil); md != "" {
		t.Errorf("RenderMarkdown(nil) = %q; want empty", md)
	}
	if out := RenderANSI(nil, ANSIConfig{}); out != "" {
		t.Errorf("RenderANSI(nil) = %q; want empty", out)
	}
}
