package document

import "testing"

func TestVisibleLength(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"\033[1m" + "bold" + "\033[0m", 4},
		{"\033]8;;https://x.com\ahello\033]8;;\a", 5},
		{"", 0},
	}
	for _, tc := range cases {
		got := visibleLength(tc.input)
		if got != tc.want {
			t.Errorf("visibleLength(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
