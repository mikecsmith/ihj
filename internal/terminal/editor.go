package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CalculateCursor returns the editor cursor line and search pattern
// for a frontmatter document. If summary is empty, the cursor targets
// the summary field; otherwise it positions after the closing ---.
func CalculateCursor(doc, summary string) (int, string) {
	if summary == "" {
		return 0, "^summary:"
	}
	dashes := 0
	for i, line := range strings.Split(doc, "\n") {
		if strings.TrimSpace(line) == "---" {
			dashes++
			if dashes == 2 {
				return i + 2, ""
			}
		}
	}
	return 0, ""
}

// SplitShellCommand splits a command string into arguments, respecting quotes.
func SplitShellCommand(cmd string) []string {
	if cmd == "" {
		return nil
	}
	var args []string
	var cur strings.Builder
	inQuote := false
	qChar := byte(0)
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if inQuote {
			if c == qChar {
				inQuote = false
			} else {
				cur.WriteByte(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			qChar = c
		} else if c == ' ' || c == '\t' {
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

// IsVimLike returns true if the editor name is vim, nvim, or vi.
func IsVimLike(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "vim") || strings.Contains(l, "nvim") || l == "vi"
}

// PrepareEditor creates a temp file with the given content and returns
// an exec.Cmd ready to launch the editor, plus the temp file path.
// The caller is responsible for reading and cleaning up the temp file.
func PrepareEditor(editorCmd, initial, prefix string, cursorLine int, searchPattern string) (*exec.Cmd, string, error) {
	cmd := SplitShellCommand(editorCmd)
	if len(cmd) == 0 {
		cmd = []string{"vim"}
	}

	base := filepath.Base(cmd[0])
	if IsVimLike(base) {
		if searchPattern != "" {
			cmd = append(cmd, "-c", "/"+searchPattern, "-c", "normal! $", "-c", "startinsert")
		} else if cursorLine > 0 {
			cmd = append(cmd, fmt.Sprintf("+%d", cursorLine), "-c", "startinsert")
		} else {
			cmd = append(cmd, "-c", "startinsert")
		}
	}

	tmpFile, err := os.CreateTemp("", prefix+"*.md")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(initial); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return nil, "", err
	}
	_ = tmpFile.Close()

	cmd = append(cmd, tmpPath)
	proc := exec.Command(cmd[0], cmd[1:]...)
	return proc, tmpPath, nil
}

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "linux":
		candidates = [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		}
	}
	for _, cmd := range candidates {
		if _, err := exec.LookPath(cmd[0]); err == nil {
			c := exec.Command(cmd[0], cmd[1:]...)
			c.Stdin = strings.NewReader(text)
			if err := c.Run(); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("no clipboard utility found")
}
