//go:build android

package ui

import (
	"gioui.org/app"

	"github.com/vitovt/wetterabhaengig/internal/gps"
)

func (u *UI) handlePlatformEvent(event any) {
	viewEvent, ok := event.(app.AndroidViewEvent)
	if !ok {
		return
	}
	if binder, ok := u.gps.(gps.AndroidViewBinder); ok {
		binder.SetAndroidView(viewEvent.View)
	}
}
