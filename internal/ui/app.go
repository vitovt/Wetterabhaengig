package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/vitovt/wetterabhaengig/internal/data"
	"github.com/vitovt/wetterabhaengig/internal/domain"
	"github.com/vitovt/wetterabhaengig/internal/location"
	"github.com/vitovt/wetterabhaengig/internal/notify"
	"github.com/vitovt/wetterabhaengig/internal/service"
)

type Screen int

const (
	ScreenHome Screen = iota
	ScreenHistory
	ScreenSettings
	ScreenTest
)

type UI struct {
	theme *material.Theme
	cfg   domain.AppConfig
	check *service.Checker
	ntf   *notify.Notifier

	screen Screen

	navHome     widget.Clickable
	navHistory  widget.Clickable
	navSettings widget.Clickable
	navTest     widget.Clickable

	checkNowBtn     widget.Clickable
	settingsTestBtn widget.Clickable
	testPageTestBtn widget.Clickable
	toggleNotifBtn  widget.Clickable

	latEditor      widget.Editor
	lonEditor      widget.Editor
	cities         []location.City
	cityButtons    []widget.Clickable
	cityList       layout.List
	selectedCity   int
	locationLat    float64
	locationLon    float64
	applyCoordsBtn widget.Clickable

	history     []service.Result
	historyList layout.List

	metrics      domain.Metrics
	pressureRisk domain.RiskLevel
	kIndexRisk   domain.RiskLevel
	overallRisk  domain.RiskLevel

	lastCheck          time.Time
	statusMessage      string
	statusMessageError bool
	notificationID     int
	hasChecked         bool
	autoCheckPending   bool
}

func New() *UI {
	cities := location.DefaultEUCities()
	cityButtons := make([]widget.Clickable, len(cities))
	defaultLocation := cities[0]

	u := &UI{
		theme:       material.NewTheme(),
		cfg:         domain.DefaultConfig(),
		check:       service.NewChecker(data.NewClient(12 * time.Second)),
		ntf:         notify.New(),
		screen:      ScreenHome,
		cities:      cities,
		cityButtons: cityButtons,
		cityList: layout.List{
			Axis: layout.Vertical,
		},
		selectedCity: defaultLocationIndex(defaultLocation, cities),
		locationLat:  defaultLocation.Lat,
		locationLon:  defaultLocation.Lon,
		historyList: layout.List{
			Axis: layout.Vertical,
		},
		metrics: domain.Metrics{
			PressureDeltaHPa: 0,
			KIndex:           0,
		},
		statusMessage:    "Ready. Press Check now to load live data.",
		autoCheckPending: true,
	}
	u.latEditor.SingleLine = true
	u.lonEditor.SingleLine = true
	u.latEditor.SetText(fmt.Sprintf("%.4f", u.locationLat))
	u.lonEditor.SetText(fmt.Sprintf("%.4f", u.locationLon))
	u.recomputeRisk()
	return u
}

func Run(window *app.Window) error {
	u := New()
	var ops op.Ops

	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			u.handleActions(gtx)
			u.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func (u *UI) handleActions(gtx layout.Context) {
	if u.autoCheckPending {
		u.autoCheckPending = false
		u.runCheckNow()
	}

	for u.navHome.Clicked(gtx) {
		u.screen = ScreenHome
	}
	for u.navHistory.Clicked(gtx) {
		u.screen = ScreenHistory
	}
	for u.navSettings.Clicked(gtx) {
		u.screen = ScreenSettings
	}
	for u.navTest.Clicked(gtx) {
		u.screen = ScreenTest
	}
	for u.checkNowBtn.Clicked(gtx) {
		u.runCheckNow()
	}
	for u.applyCoordsBtn.Clicked(gtx) {
		if err := u.syncLocationFromEditors(); err != nil {
			u.setStatus(err.Error(), true)
		} else {
			u.setStatus(fmt.Sprintf("Location updated: %.4f, %.4f", u.locationLat, u.locationLon), false)
		}
	}
	for u.toggleNotifBtn.Clicked(gtx) {
		u.cfg.Notifications.Enabled = !u.cfg.Notifications.Enabled
		if u.cfg.Notifications.Enabled {
			u.setStatus("Notifications enabled.", false)
		} else {
			u.setStatus("Notifications disabled.", false)
		}
	}
	for u.settingsTestBtn.Clicked(gtx) {
		u.triggerTestNotification()
	}
	for u.testPageTestBtn.Clicked(gtx) {
		u.triggerTestNotification()
	}
	for idx := range u.cityButtons {
		for u.cityButtons[idx].Clicked(gtx) {
			u.selectCity(idx)
		}
	}
}

func (u *UI) runCheckNow() {
	if err := u.syncLocationFromEditors(); err != nil {
		u.setStatus(err.Error(), true)
		return
	}

	oldRisk := u.overallRisk
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := u.check.Evaluate(
		ctx,
		u.cfg,
		u.locationLat,
		u.locationLon,
		u.metrics,
	)

	u.metrics.PressureDeltaHPa = result.PressureDeltaHPa
	u.metrics.KIndex = result.KIndex
	u.pressureRisk = result.PressureRisk
	u.kIndexRisk = result.KIndexRisk
	u.overallRisk = result.OverallRisk
	u.lastCheck = result.CheckedAt
	u.hasChecked = true
	u.history = append(u.history, result)

	if err != nil {
		u.setStatus(
			fmt.Sprintf("Check completed with fallback data: %v", err),
			true,
		)
	} else {
		u.setStatus(
			fmt.Sprintf("Check completed at %s. Risk=%s.", result.CheckedAt.Format("2006-01-02 15:04:05"), result.OverallRisk.String()),
			false,
		)
	}

	if u.cfg.Notifications.Enabled && oldRisk != u.overallRisk && len(u.history) > 1 {
		u.pushNotification(
			fmt.Sprintf(
				"State changed: %s -> %s",
				oldRisk.String(),
				u.overallRisk.String(),
			),
		)
	}
}

func (u *UI) triggerTestNotification() {
	u.pushNotification(u.currentNotificationText())
}

func (u *UI) pushNotification(message string) {
	u.notificationID++
	if err := u.ntf.Send("Wetterabhaengig", message); err != nil {
		u.setStatus(
			fmt.Sprintf(
				"Notification #%d (in-app only): %s | local delivery error: %v",
				u.notificationID,
				message,
				err,
			),
			true,
		)
		return
	}
	u.setStatus(fmt.Sprintf("Notification #%d sent: %s", u.notificationID, message), false)
}

func (u *UI) recomputeRisk() {
	u.pressureRisk = domain.RiskFromPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Pressure)
	u.kIndexRisk = domain.RiskFromKIndex(u.metrics.KIndex, u.cfg.KIndex)
	u.overallRisk = domain.AggregateRisk(u.pressureRisk, u.kIndexRisk)
}

func (u *UI) layout(gtx layout.Context) layout.Dimensions {
	compact := gtx.Constraints.Max.X < gtx.Dp(unit.Dp(760)) || gtx.Constraints.Max.Y > gtx.Constraints.Max.X

	inset := layout.UniformInset(unit.Dp(12))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if compact {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(u.layoutHeader),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(u.layoutTopNavCompact),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Flexed(1, u.layoutCurrentScreen),
			)
		}

		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				width := gtx.Dp(unit.Dp(220))
				gtx.Constraints.Max.X = width
				gtx.Constraints.Min.X = width
				return u.layoutSidebar(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(u.layoutHeader),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Flexed(1, u.layoutCurrentScreen),
				)
			}),
		)
	})
}

func (u *UI) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H5(u.theme, "Wetterabhaengig")
			return title.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			sub := material.Body2(
				u.theme,
				"Weather risk monitoring with traffic light status and numeric context.",
			)
			sub.Color = color.NRGBA{A: 255, R: 70, G: 70, B: 70}
			return sub.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.statusMessage == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				text := material.Body2(u.theme, u.statusMessage)
				if u.statusMessageError {
					text.Color = color.NRGBA{A: 255, R: 155, G: 30, B: 30}
				} else {
					text.Color = color.NRGBA{A: 255, R: 25, G: 100, B: 25}
				}
				return text.Layout(gtx)
			})
		}),
	)
}

func (u *UI) layoutSidebar(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.layoutNavRow(&u.navHome, ScreenHome, "Home")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navHistory, ScreenHistory, "History")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navSettings, ScreenSettings, "Settings")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navTest, ScreenTest, "Test")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.checkNowBtn, "Check now")
			return btn.Layout(gtx)
		}),
	)
}

func (u *UI) layoutTopNavCompact(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, u.layoutNavRow(&u.navHome, ScreenHome, "Home")),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, u.layoutNavRow(&u.navHistory, ScreenHistory, "History")),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, u.layoutNavRow(&u.navSettings, ScreenSettings, "Settings")),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, u.layoutNavRow(&u.navTest, ScreenTest, "Test")),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.checkNowBtn, "Check now")
			return btn.Layout(gtx)
		}),
	)
}

func (u *UI) layoutNavRow(clickable *widget.Clickable, screen Screen, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if u.screen == screen {
			label = "• " + label
		}
		btn := material.Button(u.theme, clickable, label)
		return btn.Layout(gtx)
	}
}

func (u *UI) layoutCurrentScreen(gtx layout.Context) layout.Dimensions {
	switch u.screen {
	case ScreenHome:
		return u.layoutHome(gtx)
	case ScreenHistory:
		return u.layoutHistory(gtx)
	case ScreenSettings:
		return u.layoutSettings(gtx)
	case ScreenTest:
		return u.layoutTest(gtx)
	default:
		return layout.Dimensions{}
	}
}

func (u *UI) layoutHome(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.layoutTrafficLight),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.H6(u.theme, fmt.Sprintf("Overall risk: %s", u.overallRisk.String()))
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := material.Body1(u.theme, u.currentNotificationText())
			return text.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			locationLine := material.Body1(
				u.theme,
				fmt.Sprintf("Location: %.4f, %.4f", u.locationLat, u.locationLon),
			)
			return locationLine.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := material.Body1(
				u.theme,
				fmt.Sprintf("Pressure delta: %.2f hPa | %.2f mmHg | %.2f inHg",
					u.metrics.PressureDeltaHPa,
					domain.PressureDeltaMMHg(u.metrics.PressureDeltaHPa),
					domain.PressureDeltaInHg(u.metrics.PressureDeltaHPa),
				),
			)
			return value.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := material.Body1(
				u.theme,
				fmt.Sprintf("K-index: %.1f", u.metrics.KIndex),
			)
			return value.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := material.Body1(
				u.theme,
				fmt.Sprintf("Source risks: pressure=%s, k-index=%s", u.pressureRisk.String(), u.kIndexRisk.String()),
			)
			return value.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !u.hasChecked {
				return material.Body2(u.theme, "Last check: not yet completed").Layout(gtx)
			}
			value := material.Body2(
				u.theme,
				fmt.Sprintf("Last check: %s", u.lastCheck.Format("2006-01-02 15:04:05")),
			)
			return value.Layout(gtx)
		}),
	)
}

func (u *UI) layoutHistory(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H6(u.theme, "History")
			return title.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := material.Body2(
				u.theme,
				"Chart implementation comes next. History rows below already track pressure delta and K-index per check.",
			)
			return text.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(u.history) == 0 {
				return material.Body1(u.theme, "No history yet. Run Check now.").Layout(gtx)
			}
			return u.historyList.Layout(gtx, len(u.history), func(gtx layout.Context, index int) layout.Dimensions {
				item := u.history[len(u.history)-1-index]
				line := fmt.Sprintf(
					"%s | risk=%s | delta=%.2f hPa | K=%.1f",
					item.CheckedAt.Format("2006-01-02 15:04:05"),
					item.OverallRisk.String(),
					item.PressureDeltaHPa,
					item.KIndex,
				)
				return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Body1(u.theme, line).Layout(gtx)
				})
			})
		}),
	)
}

func (u *UI) layoutSettings(gtx layout.Context) layout.Dimensions {
	notificationState := "ON"
	if !u.cfg.Notifications.Enabled {
		notificationState = "OFF"
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H6(u.theme, "Settings")
			return title.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, fmt.Sprintf("Schedule period: %d min (minimum %d min)", u.cfg.Schedule.PeriodMinutes, u.cfg.Schedule.MinMinutes))
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, fmt.Sprintf("History retention default: %d days, max: %d years", u.cfg.Retention.DefaultDays, u.cfg.Retention.MaxYears))
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, fmt.Sprintf("Pressure thresholds: medium>%.1f, high>%.1f, critical>%.1f", u.cfg.Pressure.Medium, u.cfg.Pressure.High, u.cfg.Pressure.Crit))
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, fmt.Sprintf("K-index thresholds: medium>=%.1f, high>=%.1f, critical>=%.1f", u.cfg.KIndex.Medium, u.cfg.KIndex.High, u.cfg.KIndex.Crit))
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, fmt.Sprintf("Language options: %v", u.cfg.Languages))
			return txt.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, "Location coordinates")
			return txt.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(u.theme, &u.latEditor, "Latitude")
					return ed.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(u.theme, &u.lonEditor, "Longitude")
					return ed.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.applyCoordsBtn, "Apply coordinates")
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body2(
				u.theme,
				fmt.Sprintf("Selected city: %s", u.currentCityName()),
			)
			return txt.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutCitySelector(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.toggleNotifBtn, fmt.Sprintf("Toggle notifications (%s)", notificationState))
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.settingsTestBtn, "Test notification")
			return btn.Layout(gtx)
		}),
	)
}

func (u *UI) layoutTest(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H6(u.theme, "Test")
			return title.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, "Manual test tools for current notification payload.")
			return txt.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, u.currentNotificationText())
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body2(
				u.theme,
				"This action must immediately create a local notification once platform notification backends are added.",
			)
			return txt.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.testPageTestBtn, "Test notification")
			return btn.Layout(gtx)
		}),
	)
}

func (u *UI) layoutTrafficLight(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(unit.Dp(126))
	dims := image.Pt(size, size)
	shape := clip.UniformRRect(image.Rectangle{Max: dims}, size/2)
	paint.FillShape(gtx.Ops, u.trafficLightColor(), shape.Op(gtx.Ops))
	return layout.Dimensions{Size: dims}
}

func (u *UI) trafficLightColor() color.NRGBA {
	switch u.overallRisk {
	case domain.RiskLow:
		return color.NRGBA{A: 255, R: 34, G: 139, B: 34}
	case domain.RiskMedium:
		return color.NRGBA{A: 255, R: 255, G: 199, B: 0}
	case domain.RiskHigh:
		return color.NRGBA{A: 255, R: 200, G: 25, B: 25}
	case domain.RiskCritical:
		if time.Now().Unix()%2 == 0 {
			return color.NRGBA{A: 255, R: 235, G: 0, B: 0}
		}
		return color.NRGBA{A: 255, R: 110, G: 0, B: 0}
	default:
		return color.NRGBA{A: 255, R: 120, G: 120, B: 120}
	}
}

func (u *UI) currentNotificationText() string {
	outOfRange := "none"
	if u.pressureRisk >= u.kIndexRisk && u.pressureRisk != domain.RiskLow {
		outOfRange = fmt.Sprintf(
			"pressure delta %.2f hPa (>%.1f)",
			u.metrics.PressureDeltaHPa,
			thresholdLabel(u.pressureRisk, u.cfg.Pressure),
		)
	} else if u.kIndexRisk != domain.RiskLow {
		outOfRange = fmt.Sprintf(
			"k-index %.1f (>=%.1f)",
			u.metrics.KIndex,
			thresholdLabelK(u.kIndexRisk, u.cfg.KIndex),
		)
	}

	return fmt.Sprintf(
		"Risk: %s. Pressure delta %.2f hPa. K-index %.1f. Out of range: %s. Location: %.4f, %.4f.",
		u.overallRisk.String(),
		u.metrics.PressureDeltaHPa,
		u.metrics.KIndex,
		outOfRange,
		u.locationLat,
		u.locationLon,
	)
}

func (u *UI) layoutCitySelector(gtx layout.Context) layout.Dimensions {
	maxHeight := gtx.Dp(unit.Dp(200))
	gtx.Constraints.Max.Y = maxHeight
	return u.cityList.Layout(gtx, len(u.cityButtons), func(gtx layout.Context, index int) layout.Dimensions {
		enabledLabel := u.cities[index].Name
		if index == u.selectedCity {
			enabledLabel = "• " + enabledLabel
		}
		return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.cityButtons[index], enabledLabel)
			return btn.Layout(gtx)
		})
	})
}

func (u *UI) selectCity(index int) {
	if index < 0 || index >= len(u.cities) {
		return
	}
	u.selectedCity = index
	u.locationLat = u.cities[index].Lat
	u.locationLon = u.cities[index].Lon
	u.latEditor.SetText(fmt.Sprintf("%.4f", u.locationLat))
	u.lonEditor.SetText(fmt.Sprintf("%.4f", u.locationLon))
	u.setStatus(
		fmt.Sprintf("City selected: %s (%.4f, %.4f)", u.cities[index].Name, u.locationLat, u.locationLon),
		false,
	)
}

func (u *UI) syncLocationFromEditors() error {
	latText := strings.TrimSpace(u.latEditor.Text())
	lonText := strings.TrimSpace(u.lonEditor.Text())
	lat, err := strconv.ParseFloat(latText, 64)
	if err != nil {
		return fmt.Errorf("invalid latitude: %q", latText)
	}
	lon, err := strconv.ParseFloat(lonText, 64)
	if err != nil {
		return fmt.Errorf("invalid longitude: %q", lonText)
	}
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude must be in [-90, 90]")
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("longitude must be in [-180, 180]")
	}

	u.locationLat = lat
	u.locationLon = lon
	return nil
}

func (u *UI) currentCityName() string {
	if u.selectedCity < 0 || u.selectedCity >= len(u.cities) {
		return "custom"
	}
	return u.cities[u.selectedCity].Name
}

func (u *UI) setStatus(message string, isError bool) {
	u.statusMessage = message
	u.statusMessageError = isError
}

func defaultLocationIndex(selected location.City, cities []location.City) int {
	for i := range cities {
		if cities[i].Name == selected.Name {
			return i
		}
	}
	return 0
}

func thresholdLabel(level domain.RiskLevel, t domain.PressureThresholds) float64 {
	switch level {
	case domain.RiskCritical:
		return t.Crit
	case domain.RiskHigh:
		return t.High
	case domain.RiskMedium:
		return t.Medium
	default:
		return 0
	}
}

func thresholdLabelK(level domain.RiskLevel, t domain.KIndexThresholds) float64 {
	switch level {
	case domain.RiskCritical:
		return t.Crit
	case domain.RiskHigh:
		return t.High
	case domain.RiskMedium:
		return t.Medium
	default:
		return 0
	}
}
