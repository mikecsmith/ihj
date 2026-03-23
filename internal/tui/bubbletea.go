package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// BubbleTeaUI implements the ui.UI interface. It is the sole UI
// implementation — handling both interactive TUI mode (when program != nil)
// and headless CLI mode (when program == nil, e.g. `ihj assign FOO-1`).
type BubbleTeaUI struct {
	EditorCmd string
	program   *tea.Program // Set when TUI is running, nil otherwise.
}

// SetProgram attaches the running Bubble Tea program for suspend/resume.
func (b *BubbleTeaUI) SetProgram(p *tea.Program) {
	b.program = p
}

// --- ui.UI implementation ---

func (b *BubbleTeaUI) Select(title string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, nil
	}
	m := selectModel{title: title, options: options, cursor: 0, chosen: -1}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return -1, err
	}
	return result.(selectModel).chosen, nil
}

func (b *BubbleTeaUI) Confirm(prompt string) (bool, error) {
	m := confirmModel{prompt: prompt}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return false, err
	}
	return result.(confirmModel).yes, nil
}

func (b *BubbleTeaUI) EditText(initial, prefix string, cursorLine int, searchPattern string) (string, error) {
	cmd := splitShellCommand(b.EditorCmd)
	if len(cmd) == 0 {
		cmd = []string{"vim"}
	}

	base := filepath.Base(cmd[0])
	if isVimLike(base) {
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
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }() // Use a deferred func to ignore error cleanly

	if _, err := tmpFile.WriteString(initial); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	_ = tmpFile.Close()

	cmd = append(cmd, tmpPath)

	proc := exec.Command(cmd[0], cmd[1:]...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	if err := proc.Run(); err != nil {
		return "", fmt.Errorf("editor error: %w", err)
	}

	result, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("reading editor output: %w", err)
	}
	return string(result), nil
}

// PrepareEditor creates a temp file with the given content and returns
// an exec.Cmd ready to launch the editor, plus the temp file path.
// The caller is responsible for reading and cleaning up the temp file.
func (b *BubbleTeaUI) PrepareEditor(initial, prefix string, cursorLine int, searchPattern string) (*exec.Cmd, string, error) {
	cmd := splitShellCommand(b.EditorCmd)
	if len(cmd) == 0 {
		cmd = []string{"vim"}
	}

	base := filepath.Base(cmd[0])
	if isVimLike(base) {
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

func (b *BubbleTeaUI) Notify(title, message string) {
	if b.program != nil {
		b.program.Send(notifyMsg{title: title, message: message})
		return
	}
	// Headless mode: print to stderr.
	fmt.Fprintf(os.Stderr, "  %s: %s\n", title, message)
}

func (b *BubbleTeaUI) CopyToClipboard(text string) error {
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

func (b *BubbleTeaUI) PromptText(prompt string) (string, error) {
	m := promptModel{prompt: prompt}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	rm := result.(promptModel)
	if rm.canceled {
		return "", nil
	}
	return rm.value, nil
}

func (b *BubbleTeaUI) Status(message string) {
	if b.program != nil {
		b.program.Send(statusMsg(message))
		return
	}
	fmt.Fprintf(os.Stderr, "  %s\n", message)
}

// --- Messages for Bubble Tea program communication ---

type notifyMsg struct {
	title   string
	message string
}

type statusMsg string

// --- Helpers ---

func isVimLike(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "vim") || strings.Contains(l, "nvim") || l == "vi"
}

func splitShellCommand(cmd string) []string {
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
