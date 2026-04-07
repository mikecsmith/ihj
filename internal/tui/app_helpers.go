package tui

import "time"

func (m *AppModel) setNotify(msg string) {
	m.notify = msg
	m.notifyAt = time.Now()
	m.ui.Emit(EventNotify, "message", msg)
}
