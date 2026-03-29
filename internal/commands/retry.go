package commands

// offerRecovery presents the user with recovery options after an error.
func offerRecovery(ws *WorkspaceSession, contents, errMsg string) (string, error) {
	ws.Runtime.UI.Notify("Error", errMsg)

	choice, err := ws.Runtime.UI.Select("What now?", []string{
		"Re-edit",
		"Copy to clipboard and abort",
		"Abort",
	})
	if err != nil {
		return "", err
	}

	switch choice {
	case 0:
		return ws.Runtime.UI.EditDocument(contents, "ihj_")
	case 1:
		if clipErr := ws.Runtime.UI.CopyToClipboard(contents); clipErr != nil {
			ws.Runtime.UI.Notify("Warning", "Could not copy to clipboard")
		} else {
			ws.Runtime.UI.Notify("Rescue", "Buffer copied to clipboard.")
		}
		return "", nil
	default:
		return "", nil
	}
}
