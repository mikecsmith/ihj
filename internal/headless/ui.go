// Package headless implements commands.UI for CLI (non-TUI) commands.
// Interactive methods use Huh forms; ReviewDiff uses a custom Bubble Tea model.
package headless

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// Compile-time check that HeadlessUI satisfies commands.UI.
var _ commands.UI = (*HeadlessUI)(nil)

// HeadlessUI implements commands.UI for headless CLI usage.
// Interactive methods use Huh forms; ReviewDiff uses a custom Bubble Tea model.
type HeadlessUI struct {
	EditorCmd string
}

// NewHeadlessUI creates a new HeadlessUI instance.
func NewHeadlessUI() *HeadlessUI {
	return &HeadlessUI{}
}

func (h *HeadlessUI) Select(title string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, nil
	}

	huhOpts := make([]huh.Option[string], len(options))
	for i, opt := range options {
		huhOpts[i] = huh.NewOption(opt, opt)
	}

	var selected string
	err := huh.NewSelect[string]().
		Title(title).
		Options(huhOpts...).
		Value(&selected).
		Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return -1, nil
		}
		return -1, err
	}

	for i, opt := range options {
		if opt == selected {
			return i, nil
		}
	}
	return -1, nil
}

func (h *HeadlessUI) Confirm(prompt string) (bool, error) {
	var yes bool
	err := huh.NewConfirm().
		Title(prompt).
		Value(&yes).
		Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return false, nil
		}
		return false, err
	}
	return yes, nil
}

func (h *HeadlessUI) InputText(prompt, initial string) (string, error) {
	value := initial
	err := huh.NewText().
		Title(prompt).
		Value(&value).
		CharLimit(4000).
		Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (h *HeadlessUI) EditDocument(initial, prefix string) (string, error) {
	// Compute cursor position from document content.
	summary := ""
	if fm, _, parseErr := core.ParseFrontmatter(initial); parseErr == nil {
		summary = fm["summary"]
	}
	cursorLine, searchPattern := commands.CalculateCursor(initial, summary)

	proc, tmpPath, err := terminal.PrepareEditor(h.EditorCmd, initial, prefix, cursorLine, searchPattern)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpPath) }()

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

func (h *HeadlessUI) Notify(title, message string) {
	fmt.Fprintf(os.Stderr, "  %s: %s\n", title, message)
}

func (h *HeadlessUI) CopyToClipboard(text string) error {
	return terminal.CopyToClipboard(text)
}

func (h *HeadlessUI) PromptText(prompt string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(prompt).
		Value(&value).
		Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (h *HeadlessUI) ReviewDiff(title string, changes []commands.FieldDiff, options []string) (int, error) {
	if len(options) == 0 {
		return -1, nil
	}
	m := diffModel{title: title, changes: changes, options: options, cursor: 0, chosen: -1, keys: terminal.DefaultKeyMap()}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return -1, err
	}

	if dm, ok := result.(diffModel); ok {
		return dm.chosen, nil
	}
	return -1, fmt.Errorf("unexpected model type returned: %T", result)
}

func (h *HeadlessUI) Status(message string) {
	fmt.Fprintf(os.Stderr, "  %s\n", message)
}
