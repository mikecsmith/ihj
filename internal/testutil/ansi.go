package testutil

import "strings"

// StripANSI removes ANSI escape sequences for stable text comparison.
// Handles CSI (ESC [ ...), OSC (ESC ] ...), and other ESC sequences.
func StripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip CSI sequences: ESC [ ... final byte (0x40–0x7E)
			if i+1 < len(s) && s[i+1] == '[' {
				j := i + 2
				for j < len(s) && s[j] < 0x40 {
					j++
				}
				if j < len(s) {
					j++ // skip final byte
				}
				i = j
				continue
			}
			// Skip OSC sequences: ESC ] ... ST (ESC \ or BEL)
			if i+1 < len(s) && s[i+1] == ']' {
				j := i + 2
				for j < len(s) {
					if s[j] == '\x07' { // BEL
						j++
						break
					}
					if s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			}
			// Skip other ESC sequences (ESC + one byte)
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
