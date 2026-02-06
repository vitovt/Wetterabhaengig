package main

import (
	"log"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vitovt/wetterabhaengig/internal/ui"
)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("Wetterabhaengig"),
			app.Size(unit.Dp(980), unit.Dp(680)),
		)
		if err := ui.Run(window); err != nil {
			log.Fatal(err)
		}
	}()

	app.Main()
}
