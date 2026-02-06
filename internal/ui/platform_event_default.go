//go:build !android

package ui

func (u *UI) handlePlatformEvent(event any) {
	_ = event
}
