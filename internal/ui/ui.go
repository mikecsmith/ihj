// Package ui defines the interfaces for user interaction.
package ui

// UI abstracts all user interaction so commands never touch stdin/stdout
// directly. Bubble Tea implements this for the real TUI. Tests provide
// a mock. Headless/CI provides a flag-driven stub.
type UI interface {
	// Select presents a list of options and returns the chosen index.
	// Returns -1 if the user cancels.
	Select(title string, options []string) (int, error)

	// Confirm asks a yes/no question. Returns true for yes.
	Confirm(prompt string) (bool, error)

	// EditText opens the user's editor with initial content and returns
	// the edited result. prefix is used for the temp file name.
	EditText(initial, prefix string, cursorLine int, searchPattern string) (string, error)

	// Notify displays a message to the user (toast, inline, etc).
	Notify(title, message string)

	// CopyToClipboard copies text to the system clipboard.
	CopyToClipboard(text string) error

	// PromptText asks for a single line of text input.
	PromptText(prompt string) (string, error)

	// Status shows a transient progress message (spinner in TUI, stderr in terminal).
	Status(message string)
}
