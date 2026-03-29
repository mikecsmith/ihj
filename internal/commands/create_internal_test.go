package commands

import "testing"

func TestFirst(t *testing.T) {
	if got := first("", "", "c"); got != "c" {
		t.Errorf("first(\"\", \"\", \"c\") = %q; want \"c\"", got)
	}
	if got := first("a", "b"); got != "a" {
		t.Errorf("first(\"a\", \"b\") = %q; want \"a\"", got)
	}
	if got := first(""); got != "" {
		t.Errorf("first(\"\") = %q; want \"\"", got)
	}
}
