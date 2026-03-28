package commands

import "os/exec"

// OpenInBrowser opens a URL in the system browser. Used by both CLI and TUI.
func OpenInBrowser(url string) error {
	candidates := []string{"open", "xdg-open"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return exec.Command(path, url).Start()
		}
	}
	return nil
}
