package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/vitovt/wetterabhaengig/internal/data"
	"github.com/vitovt/wetterabhaengig/internal/domain"
	"github.com/vitovt/wetterabhaengig/internal/gps"
	"github.com/vitovt/wetterabhaengig/internal/i18n"
	"github.com/vitovt/wetterabhaengig/internal/location"
	"github.com/vitovt/wetterabhaengig/internal/notify"
	"github.com/vitovt/wetterabhaengig/internal/service"
	"github.com/vitovt/wetterabhaengig/internal/storage"
)

type Screen int

const (
	ScreenHome Screen = iota
	ScreenHistory
	ScreenSettings
	ScreenLocation
	ScreenTest
)

type UI struct {
	theme *material.Theme
	cfg   domain.AppConfig
	check *service.Checker
	gps   gps.Provider
	ntf   notify.Notifier
	store *storage.Store
	i18n  *i18n.Bundle

	screen Screen

	navHome      widget.Clickable
	navHistory   widget.Clickable
	navSettings  widget.Clickable
	navLocation  widget.Clickable
	navTest      widget.Clickable
	menuOpenBtn  widget.Clickable
	menuCloseBtn widget.Clickable
	menuScrimBtn widget.Clickable
	menuOpen     bool

	checkNowBtn       widget.Clickable
	settingsTestBtn   widget.Clickable
	testPageTestBtn   widget.Clickable
	toggleNotifBtn    widget.Clickable
	toggleBgChecksBtn widget.Clickable
	applySettingsBtn  widget.Clickable

	setPressureMediumEditor widget.Editor
	setPressureHighEditor   widget.Editor
	setPressureCritEditor   widget.Editor
	setKMediumEditor        widget.Editor
	setKHighEditor          widget.Editor
	setKCritEditor          widget.Editor
	setScheduleEditor       widget.Editor
	setRetentionDaysEditor  widget.Editor

	setUnitHPaBtn        widget.Clickable
	setUnitMMHgBtn       widget.Clickable
	setUnitInHgBtn       widget.Clickable
	setTime24Btn         widget.Clickable
	setTime12Btn         widget.Clickable
	getGPSBtn            widget.Clickable
	languageButtons      []widget.Clickable
	languageToggleBtn    widget.Clickable
	languageSearchEditor widget.Editor
	languageDropdownOpen bool
	settingsLanguage     string

	latEditor         widget.Editor
	lonEditor         widget.Editor
	cities            []location.City
	cityButtons       []widget.Clickable
	cityList          layout.List
	cityToggleBtn     widget.Clickable
	citySearchEditor  widget.Editor
	cityDropdownOpen  bool
	selectedCity      int
	draftSelectedCity int
	locationLat       float64
	locationLon       float64
	applyCoordsBtn    widget.Clickable

	history       []service.Result
	historyList   layout.List
	settingsList  layout.List
	locationList  layout.List
	settingsDirty bool
	locationDirty bool

	settingsNotificationsEnabled bool
	settingsPressureUnit         string
	settingsTimeFormat           string
	settingsRunWhenClosed        bool

	metrics      domain.Metrics
	pressureRisk domain.RiskLevel
	kIndexRisk   domain.RiskLevel
	overallRisk  domain.RiskLevel

	lastCheck            time.Time
	statusMessage        string
	statusMessageError   bool
	notificationID       int
	hasChecked           bool
	autoCheckPending     bool
	nextScheduledCheck   time.Time
	platformLanguageInit bool
	lastScreen           Screen
}

func New() *UI {
	cities := location.DefaultEUCities()
	cityButtons := make([]widget.Clickable, len(cities))
	defaultLocation := cities[0]

	u := &UI{
		theme:       material.NewTheme(),
		cfg:         domain.DefaultConfig(),
		check:       service.NewChecker(data.NewClient(12 * time.Second)),
		gps:         gps.NewProvider(),
		ntf:         notify.New(),
		i18n:        i18n.Load("i18n"),
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
		settingsList: layout.List{
			Axis: layout.Vertical,
		},
		locationList: layout.List{
			Axis: layout.Vertical,
		},
		metrics: domain.Metrics{
			PressureDeltaHPa: 0,
			KIndex:           0,
		},
		statusMessage:    "Ready. Press Check now to load live data.",
		autoCheckPending: true,
		lastScreen:       ScreenHome,
	}

	if storePath, err := storage.DefaultPath("wetterabhaengig"); err == nil {
		u.store = storage.New(storePath)
	}
	u.refreshLanguagesFromBundle()
	u.loadState()
	u.refreshLanguagesFromBundle()
	u.statusMessage = u.tr("status.ready", "Ready. Press Check now to load live data.")

	u.latEditor.SingleLine = true
	u.lonEditor.SingleLine = true
	u.citySearchEditor.SingleLine = true
	u.languageSearchEditor.SingleLine = true
	u.resetLocationDraft()
	u.resetSettingsDraft()
	u.recomputeRisk()
	return u
}

func Run(window *app.Window) error {
	u := New()
	var ops op.Ops
	stopInvalidate := make(chan struct{})
	stoppedInvalidate := false
	stopInvalidator := func() {
		if stoppedInvalidate {
			return
		}
		close(stopInvalidate)
		stoppedInvalidate = true
	}
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				window.Invalidate()
			case <-stopInvalidate:
				return
			}
		}
	}()
	defer stopInvalidator()

	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			stopInvalidator()
			if event.Err == nil && u.shouldRunHeadlessAfterClose() {
				u.runHeadlessChecksLoop()
				return nil
			}
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			u.handleActions(gtx)
			u.layout(gtx)
			event.Frame(gtx.Ops)
		default:
			u.handlePlatformEvent(event)
		}
	}
}

func (u *UI) handleActions(gtx layout.Context) {
	compact := u.isCompactLayout(gtx)
	u.updateDirtyState()

	if u.autoCheckPending {
		u.autoCheckPending = false
		u.runCheck(false, "startup")
	}
	if u.shouldRunScheduledCheck(time.Now()) {
		u.runCheck(false, "scheduled")
	}

	for u.navHome.Clicked(gtx) {
		u.setScreen(ScreenHome, compact)
	}
	for u.navHistory.Clicked(gtx) {
		u.setScreen(ScreenHistory, compact)
	}
	for u.navSettings.Clicked(gtx) {
		u.setScreen(ScreenSettings, compact)
	}
	for u.navLocation.Clicked(gtx) {
		u.setScreen(ScreenLocation, compact)
	}
	for u.navTest.Clicked(gtx) {
		u.setScreen(ScreenTest, compact)
	}
	for u.menuOpenBtn.Clicked(gtx) {
		u.menuOpen = true
	}
	for u.menuCloseBtn.Clicked(gtx) {
		u.menuOpen = false
	}
	for u.menuScrimBtn.Clicked(gtx) {
		u.menuOpen = false
	}
	for u.checkNowBtn.Clicked(gtx) {
		u.runCheckNow()
		if compact {
			u.menuOpen = false
		}
	}
	for u.applyCoordsBtn.Clicked(gtx) {
		if !u.locationDirty {
			continue
		}
		if err := u.syncLocationFromEditors(); err != nil {
			u.setStatus(err.Error(), true)
		} else {
			u.setStatus(u.trf("status.location_updated", "Location updated: %.4f, %.4f", u.locationLat, u.locationLon), false)
			u.saveState()
			u.resetLocationDraft()
		}
	}
	for u.toggleNotifBtn.Clicked(gtx) {
		u.settingsNotificationsEnabled = !u.settingsNotificationsEnabled
	}
	for u.toggleBgChecksBtn.Clicked(gtx) {
		u.settingsRunWhenClosed = !u.settingsRunWhenClosed
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
	for u.cityToggleBtn.Clicked(gtx) {
		u.cityDropdownOpen = !u.cityDropdownOpen
	}
	for u.languageToggleBtn.Clicked(gtx) {
		u.languageDropdownOpen = !u.languageDropdownOpen
	}
	for idx := range u.languageButtons {
		if idx >= len(u.cfg.Languages) {
			break
		}
		for u.languageButtons[idx].Clicked(gtx) {
			u.selectLanguageDraft(u.cfg.Languages[idx])
		}
	}
	for u.applySettingsBtn.Clicked(gtx) {
		if !u.settingsDirty {
			continue
		}
		if err := u.applySettingsFromEditors(); err != nil {
			u.setStatus(err.Error(), true)
		} else {
			u.setStatus(u.tr("status.settings_saved", "Settings saved."), false)
			u.resetSettingsDraft()
		}
	}
	for u.setUnitHPaBtn.Clicked(gtx) {
		u.settingsPressureUnit = "hPa"
	}
	for u.setUnitMMHgBtn.Clicked(gtx) {
		u.settingsPressureUnit = "mmHg"
	}
	for u.setUnitInHgBtn.Clicked(gtx) {
		u.settingsPressureUnit = "inHg"
	}
	for u.setTime24Btn.Clicked(gtx) {
		u.settingsTimeFormat = "24h"
	}
	for u.setTime12Btn.Clicked(gtx) {
		u.settingsTimeFormat = "12h"
	}
	for u.getGPSBtn.Clicked(gtx) {
		u.getCurrentLocationViaGPS()
	}
	u.updateDirtyState()
}

func (u *UI) runCheckNow() {
	u.runCheck(false, "manual")
}

func (u *UI) shouldRunHeadlessAfterClose() bool {
	return runtime.GOOS == "android" && u.cfg.Schedule.RunWhenClosed
}

func (u *UI) runHeadlessChecksLoop() {
	if !u.shouldRunHeadlessAfterClose() {
		return
	}
	if !u.hasChecked {
		u.runCheck(false, "startup")
	}
	if u.shouldRunScheduledCheck(time.Now()) {
		u.runCheck(false, "scheduled")
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		if u.shouldRunScheduledCheck(time.Now()) {
			u.runCheck(false, "scheduled")
		}
	}
}

func (u *UI) runCheck(applyEditorLocation bool, reason string) {
	if applyEditorLocation {
		if err := u.syncLocationFromEditors(); err != nil {
			u.setStatus(err.Error(), true)
			return
		}
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
	u.pruneHistory()
	u.scheduleNextCheck(result.CheckedAt)
	u.saveState()

	if err != nil {
		u.setStatus(
			u.trf("status.check_fallback", "Check (%s) completed with fallback data: %v", u.checkReasonLabel(reason), err),
			true,
		)
	} else {
		u.setStatus(
			u.trf(
				"status.check_success",
				"Check (%s) completed at %s. Risk=%s.",
				u.checkReasonLabel(reason),
				u.formatTime(result.CheckedAt),
				u.riskLabel(result.OverallRisk),
			),
			false,
		)
	}

	if u.cfg.Notifications.Enabled && oldRisk != u.overallRisk && len(u.history) > 1 {
		u.pushNotification(
			u.trf(
				"notification.state_changed",
				"State changed: %s -> %s",
				u.riskLabel(oldRisk),
				u.riskLabel(u.overallRisk),
			),
		)
	}
}

func (u *UI) triggerTestNotification() {
	u.pushNotification(u.currentNotificationText())
}

func (u *UI) pushNotification(message string) {
	u.notificationID++
	if err := u.ntf.Send(u.tr("app.title", "Wetterabhaengig"), message); err != nil {
		u.setStatus(
			u.trf(
				"status.notification_local_error",
				"Notification #%d (in-app only): %s | local delivery error: %v",
				u.notificationID,
				message,
				err,
			),
			true,
		)
		return
	}
	u.setStatus(u.trf("status.notification_sent", "Notification #%d sent: %s", u.notificationID, message), false)
}

func (u *UI) recomputeRisk() {
	u.pressureRisk = domain.RiskFromPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Pressure)
	u.kIndexRisk = domain.RiskFromKIndex(u.metrics.KIndex, u.cfg.KIndex)
	u.overallRisk = domain.AggregateRisk(u.pressureRisk, u.kIndexRisk)
}

func (u *UI) layout(gtx layout.Context) layout.Dimensions {
	compact := u.isCompactLayout(gtx)

	inset := layout.UniformInset(unit.Dp(12))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if compact {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(u.layoutCompactTopBar),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(u.layoutHeader),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Flexed(1, u.layoutCurrentScreen),
					)
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if !u.menuOpen {
						return layout.Dimensions{}
					}
					return u.layoutMenuOverlay(gtx)
				}),
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

func (u *UI) isCompactLayout(gtx layout.Context) bool {
	return gtx.Constraints.Max.X < gtx.Dp(unit.Dp(760)) || gtx.Constraints.Max.Y > gtx.Constraints.Max.X
}

func (u *UI) layoutCompactTopBar(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.menuOpenBtn, u.tr("menu.menu", "Menu"))
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(u.theme, u.trf("menu.screen", "Screen: %s", u.screenName()))
			label.Color = color.NRGBA{A: 255, R: 90, G: 90, B: 90}
			return label.Layout(gtx)
		}),
	)
}

func (u *UI) layoutMenuOverlay(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.menuScrimBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(
					gtx.Ops,
					color.NRGBA{A: 130, R: 15, G: 18, B: 22},
					clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op(),
				)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			drawerWidth := gtx.Dp(unit.Dp(280))
			if drawerWidth > gtx.Constraints.Max.X {
				drawerWidth = gtx.Constraints.Max.X
			}
			gtx.Constraints.Min.X = drawerWidth
			gtx.Constraints.Max.X = drawerWidth

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					panelRect := image.Rectangle{Max: gtx.Constraints.Max}
					paint.FillShape(
						gtx.Ops,
						color.NRGBA{A: 255, R: 246, G: 249, B: 252},
						clip.Rect(panelRect).Op(),
					)
					return layout.Inset{Top: unit.Dp(10), Left: unit.Dp(10), Right: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										title := material.H6(u.theme, u.tr("menu.navigation", "Navigation"))
										return title.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										btn := material.Button(u.theme, &u.menuCloseBtn, u.tr("menu.close", "Close"))
										return btn.Layout(gtx)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
							layout.Rigid(u.layoutSidebar),
						)
					})
				}),
			)
		}),
	)
}

func (u *UI) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H5(u.theme, u.tr("app.title", "Wetterabhaengig"))
			return title.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			sub := material.Body2(
				u.theme,
				u.tr("app.subtitle", "Weather risk monitoring with traffic light status and numeric context."),
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
		layout.Rigid(u.layoutNavRow(&u.navHome, ScreenHome, u.tr("nav.home", "Home"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navHistory, ScreenHistory, u.tr("nav.history", "History"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navSettings, ScreenSettings, u.tr("nav.settings", "Settings"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navLocation, ScreenLocation, u.tr("nav.location", "Location"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutNavRow(&u.navTest, ScreenTest, u.tr("nav.test", "Test"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.checkNowBtn, u.tr("buttons.check_now", "Check now"))
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

func (u *UI) screenName() string {
	switch u.screen {
	case ScreenHome:
		return u.tr("nav.home", "Home")
	case ScreenHistory:
		return u.tr("nav.history", "History")
	case ScreenSettings:
		return u.tr("nav.settings", "Settings")
	case ScreenLocation:
		return u.tr("nav.location", "Location")
	case ScreenTest:
		return u.tr("nav.test", "Test")
	default:
		return u.tr("common.unknown", "Unknown")
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
	case ScreenLocation:
		return u.layoutLocation(gtx)
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
			label := material.H6(u.theme, u.trf("home.overall_risk", "Overall risk: %s", u.riskLabel(u.overallRisk)))
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
				u.trf("home.location", "Location: %.4f, %.4f", u.locationLat, u.locationLon),
			)
			return locationLine.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			primaryPressure, _ := domain.ConvertPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Units.PressureUnit)
			value := material.Body1(
				u.theme,
				u.trf("home.pressure_delta", "Pressure delta: %.2f %s | %.2f hPa | %.2f mmHg | %.2f inHg",
					primaryPressure,
					u.cfg.Units.PressureUnit,
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
				u.trf("home.k_index", "K-index: %.1f", u.metrics.KIndex),
			)
			return value.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := material.Body1(
				u.theme,
				u.trf(
					"home.source_risks",
					"Source risks: pressure=%s, k-index=%s | Time format: %s",
					u.riskLabel(u.pressureRisk),
					u.riskLabel(u.kIndexRisk),
					u.cfg.Units.TimeFormat,
				),
			)
			return value.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !u.hasChecked {
				return material.Body2(u.theme, u.tr("home.last_check_none", "Last check: not yet completed")).Layout(gtx)
			}
			value := material.Body2(
				u.theme,
				u.trf("home.last_check", "Last check: %s", u.formatTime(u.lastCheck)),
			)
			return value.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.nextScheduledCheck.IsZero() {
				return layout.Dimensions{}
			}
			value := material.Body2(
				u.theme,
				u.trf("home.next_scheduled", "Next scheduled check: %s", u.formatTime(u.nextScheduledCheck)),
			)
			return value.Layout(gtx)
		}),
	)
}

func (u *UI) layoutHistory(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H6(u.theme, u.tr("nav.history", "History"))
			return title.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := material.Body2(
				u.theme,
				u.tr(
					"history.description",
					"Chart implementation comes next. History rows below already track pressure delta and K-index per check.",
				),
			)
			return text.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(u.layoutHistoryChart),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(u.history) == 0 {
				return material.Body1(u.theme, u.tr("history.none", "No history yet. Run Check now.")).Layout(gtx)
			}
			return u.historyList.Layout(gtx, len(u.history), func(gtx layout.Context, index int) layout.Dimensions {
				item := u.history[len(u.history)-1-index]
				line := u.trf(
					"history.row",
					"%s | risk=%s | delta=%.2f hPa | K=%.1f",
					u.formatTime(item.CheckedAt),
					u.riskLabel(item.OverallRisk),
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

func (u *UI) layoutHistoryChart(gtx layout.Context) layout.Dimensions {
	width := gtx.Constraints.Max.X
	height := gtx.Dp(unit.Dp(220))
	if width <= 0 {
		width = gtx.Dp(unit.Dp(300))
	}
	if len(u.history) < 2 {
		box := image.Rect(0, 0, width, height)
		paint.FillShape(gtx.Ops, color.NRGBA{A: 255, R: 238, G: 242, B: 245}, clip.Rect(box).Op())
		return layout.Dimensions{Size: image.Pt(width, height)}
	}

	padding := 28.0
	chartW := float32(width) - float32(padding*2)
	chartH := float32(height) - float32(padding*2)
	if chartW <= 1 || chartH <= 1 {
		return layout.Dimensions{Size: image.Pt(width, height)}
	}

	// Chart background.
	box := image.Rect(0, 0, width, height)
	paint.FillShape(gtx.Ops, color.NRGBA{A: 255, R: 246, G: 249, B: 252}, clip.Rect(box).Op())

	minPressure := u.history[0].PressureDeltaHPa
	maxPressure := u.history[0].PressureDeltaHPa
	minK := u.history[0].KIndex
	maxK := u.history[0].KIndex
	for i := range u.history {
		if u.history[i].PressureDeltaHPa < minPressure {
			minPressure = u.history[i].PressureDeltaHPa
		}
		if u.history[i].PressureDeltaHPa > maxPressure {
			maxPressure = u.history[i].PressureDeltaHPa
		}
		if u.history[i].KIndex < minK {
			minK = u.history[i].KIndex
		}
		if u.history[i].KIndex > maxK {
			maxK = u.history[i].KIndex
		}
	}
	if maxPressure-minPressure < 0.1 {
		maxPressure += 0.05
		minPressure -= 0.05
	}
	if maxK-minK < 0.1 {
		maxK += 0.05
		minK -= 0.05
	}

	toX := func(i int, n int) float32 {
		step := chartW / float32(n-1)
		return float32(padding) + float32(i)*step
	}
	toYPressure := func(v float64) float32 {
		k := (v - minPressure) / (maxPressure - minPressure)
		return float32(padding) + chartH - float32(k)*chartH
	}
	toYK := func(v float64) float32 {
		k := (v - minK) / (maxK - minK)
		return float32(padding) + chartH - float32(k)*chartH
	}

	// Axes.
	axisColor := color.NRGBA{A: 255, R: 140, G: 148, B: 160}
	drawLine(gtx, float32(padding), float32(padding), float32(padding), float32(padding)+chartH, 1.5, axisColor)
	drawLine(gtx, float32(width)-float32(padding), float32(padding), float32(width)-float32(padding), float32(padding)+chartH, 1.5, axisColor)
	drawLine(gtx, float32(padding), float32(padding)+chartH, float32(padding)+chartW, float32(padding)+chartH, 1.5, axisColor)

	// Pressure line (left Y axis).
	for i := 1; i < len(u.history); i++ {
		x1 := toX(i-1, len(u.history))
		y1 := toYPressure(u.history[i-1].PressureDeltaHPa)
		x2 := toX(i, len(u.history))
		y2 := toYPressure(u.history[i].PressureDeltaHPa)
		level := domain.RiskFromPressureDelta(u.history[i].PressureDeltaHPa, u.cfg.Pressure)
		drawLine(gtx, x1, y1, x2, y2, 2.4, riskColor(level))
	}

	// K-index line (right Y axis).
	for i := 1; i < len(u.history); i++ {
		x1 := toX(i-1, len(u.history))
		y1 := toYK(u.history[i-1].KIndex)
		x2 := toX(i, len(u.history))
		y2 := toYK(u.history[i].KIndex)
		level := domain.RiskFromKIndex(u.history[i].KIndex, u.cfg.KIndex)
		drawLine(gtx, x1, y1, x2, y2, 2.0, riskColor(level))
	}

	return layout.Dimensions{Size: image.Pt(width, height)}
}

func drawLine(gtx layout.Context, x1, y1, x2, y2, width float32, col color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(x1, y1))
	p.LineTo(f32.Pt(x2, y2))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: width}.Op())
}

func (u *UI) layoutSettings(gtx layout.Context) layout.Dimensions {
	notificationState := u.tr("common.on", "ON")
	if !u.settingsNotificationsEnabled {
		notificationState = u.tr("common.off", "OFF")
	}
	backgroundState := u.tr("common.on", "ON")
	if !u.settingsRunWhenClosed {
		backgroundState = u.tr("common.off", "OFF")
	}

	return u.settingsList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		content := gtx
		content.Constraints.Min.Y = 0
		content.Constraints.Max.Y = 1_000_000
		return layout.Inset{Bottom: unit.Dp(12)}.Layout(content, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H6(u.theme, u.tr("nav.settings", "Settings"))
					return title.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(u.theme, u.tr("settings.risk_thresholds", "Risk thresholds (editable)"))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setPressureMediumEditor, u.tr("settings.pressure_medium", "Pressure medium"))
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setPressureHighEditor, u.tr("settings.pressure_high", "Pressure high"))
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setPressureCritEditor, u.tr("settings.pressure_critical", "Pressure critical"))
							return ed.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setKMediumEditor, u.tr("settings.k_medium", "K medium"))
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setKHighEditor, u.tr("settings.k_high", "K high"))
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setKCritEditor, u.tr("settings.k_critical", "K critical"))
							return ed.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(
						u.theme,
						u.trf(
							"settings.current_pressure",
							"Current pressure: medium>%.1f, high>%.1f, critical>%.1f",
							u.cfg.Pressure.Medium,
							u.cfg.Pressure.High,
							u.cfg.Pressure.Crit,
						),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(
						u.theme,
						u.trf(
							"settings.current_kindex",
							"Current K-index: medium>=%.1f, high>=%.1f, critical>=%.1f",
							u.cfg.KIndex.Medium,
							u.cfg.KIndex.High,
							u.cfg.KIndex.Crit,
						),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(
						u.theme,
						u.trf(
							"settings.schedule_info",
							"Schedule period min (>= %d) and retention days (max %d years)",
							u.cfg.Schedule.MinMinutes,
							u.cfg.Retention.MaxYears,
						),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setScheduleEditor, u.tr("settings.schedule_minutes", "Schedule minutes"))
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.setRetentionDaysEditor, u.tr("settings.retention_days", "Retention days"))
							return ed.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(u.theme, u.tr("settings.language", "Language"))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(
						u.theme,
						fmt.Sprintf(
							"%s: %s",
							u.tr("settings.current_language", "Current language"),
							u.languageDisplayName(u.settingsLanguage),
						),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutLanguageDropdown(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutPrimaryButton(gtx, &u.applySettingsBtn, u.tr("buttons.save_settings", "Save settings"), u.settingsDirty)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(u.theme, u.trf("settings.pressure_unit", "Pressure unit: %s", u.settingsPressureUnit))
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setUnitHPaBtn, selectedLabel(u.settingsPressureUnit == "hPa", "hPa"))
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setUnitMMHgBtn, selectedLabel(u.settingsPressureUnit == "mmHg", "mmHg"))
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setUnitInHgBtn, selectedLabel(u.settingsPressureUnit == "inHg", "inHg"))
							return btn.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(u.theme, u.trf("settings.time_format", "Time format: %s", u.settingsTimeFormat))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setTime24Btn, selectedLabel(u.settingsTimeFormat == "24h", "24h"))
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setTime12Btn, selectedLabel(u.settingsTimeFormat == "12h", "12h"))
							return btn.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(
						u.theme,
						fmt.Sprintf(
							"%s (%s)",
							u.tr("settings.background_checks_closed", "Run checks in background when app is closed (Android)"),
							backgroundState,
						),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(u.theme, &u.toggleBgChecksBtn, u.tr("buttons.toggle_background_checks", "Toggle background checks when app is closed"))
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(
						u.theme,
						u.tr("settings.background_checks_note", "Disabled by default. This mode is Android-only and depends on system battery/background limits."),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(u.theme, &u.toggleNotifBtn, fmt.Sprintf("%s (%s)", u.tr("buttons.toggle_notifications", "Toggle notifications"), notificationState))
					return btn.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(u.theme, &u.settingsTestBtn, u.tr("buttons.test_notification", "Test notification"))
					return btn.Layout(gtx)
				}),
			)
		})
	})
}

func (u *UI) layoutLocation(gtx layout.Context) layout.Dimensions {
	return u.locationList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		content := gtx
		content.Constraints.Min.Y = 0
		content.Constraints.Max.Y = 1_000_000
		return layout.Inset{Bottom: unit.Dp(12)}.Layout(content, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H6(u.theme, u.tr("nav.location", "Location"))
					return title.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(
						u.theme,
						u.trf("home.location", "Location: %.4f, %.4f", u.locationLat, u.locationLon),
					)
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(u.theme, &u.getGPSBtn, u.tr("buttons.get_gps_location", "Get my current location via GPS"))
					return btn.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.latEditor, u.tr("settings.latitude", "Latitude"))
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.lonEditor, u.tr("settings.longitude", "Longitude"))
							return ed.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutPrimaryButton(gtx, &u.applyCoordsBtn, u.tr("buttons.save_location", "Save location"), u.locationDirty)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutCityDropdown(gtx)
				}),
			)
		})
	})
}

func (u *UI) layoutPrimaryButton(gtx layout.Context, clickable *widget.Clickable, text string, enabled bool) layout.Dimensions {
	btn := material.Button(u.theme, clickable, text)
	if !enabled {
		btn.Background = color.NRGBA{A: 255, R: 185, G: 185, B: 185}
		btn.Color = color.NRGBA{A: 255, R: 95, G: 95, B: 95}
	}
	return btn.Layout(gtx)
}

func (u *UI) layoutTest(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H6(u.theme, u.tr("nav.test", "Test"))
			return title.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := material.Body1(u.theme, u.tr("test.manual_tools", "Manual test tools for current notification payload."))
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
				u.tr("test.action_note", "This action must immediately create a local notification once platform notification backends are added."),
			)
			return txt.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.testPageTestBtn, u.tr("buttons.test_notification", "Test notification"))
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

func riskColor(level domain.RiskLevel) color.NRGBA {
	switch level {
	case domain.RiskLow:
		return color.NRGBA{A: 255, R: 45, G: 165, B: 70}
	case domain.RiskMedium:
		return color.NRGBA{A: 255, R: 230, G: 175, B: 25}
	case domain.RiskHigh:
		return color.NRGBA{A: 255, R: 208, G: 52, B: 42}
	case domain.RiskCritical:
		return color.NRGBA{A: 255, R: 165, G: 20, B: 20}
	default:
		return color.NRGBA{A: 255, R: 120, G: 120, B: 120}
	}
}

func (u *UI) trafficLightColor() color.NRGBA {
	switch u.overallRisk {
	case domain.RiskLow:
		return riskColor(domain.RiskLow)
	case domain.RiskMedium:
		return riskColor(domain.RiskMedium)
	case domain.RiskHigh:
		return riskColor(domain.RiskHigh)
	case domain.RiskCritical:
		if time.Now().Unix()%2 == 0 {
			return color.NRGBA{A: 255, R: 245, G: 25, B: 25}
		}
		return riskColor(domain.RiskCritical)
	default:
		return color.NRGBA{A: 255, R: 120, G: 120, B: 120}
	}
}

func (u *UI) currentNotificationText() string {
	outOfRange := u.tr("notification.out_none", "none")
	if u.pressureRisk >= u.kIndexRisk && u.pressureRisk != domain.RiskLow {
		outOfRange = u.trf(
			"notification.out_pressure",
			"pressure delta %.2f hPa (>%.1f)",
			u.metrics.PressureDeltaHPa,
			thresholdLabel(u.pressureRisk, u.cfg.Pressure),
		)
	} else if u.kIndexRisk != domain.RiskLow {
		outOfRange = u.trf(
			"notification.out_kindex",
			"k-index %.1f (>=%.1f)",
			u.metrics.KIndex,
			thresholdLabelK(u.kIndexRisk, u.cfg.KIndex),
		)
	}

	return u.trf(
		"notification.payload",
		"Risk: %s. Pressure delta %.2f hPa. K-index %.1f. Out of range: %s. Location: %.4f, %.4f.",
		u.riskLabel(u.overallRisk),
		u.metrics.PressureDeltaHPa,
		u.metrics.KIndex,
		outOfRange,
		u.locationLat,
		u.locationLon,
	)
}

func (u *UI) layoutCityDropdown(gtx layout.Context) layout.Dimensions {
	filtered := u.filteredCityIndices()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := u.trf("settings.city", "City: %s", u.currentDraftCityName())
			if u.cityDropdownOpen {
				label = u.trf("settings.city", "City: %s", u.currentDraftCityName()) + " ▲"
			} else {
				label = u.trf("settings.city", "City: %s", u.currentDraftCityName()) + " ▼"
			}
			btn := material.Button(u.theme, &u.cityToggleBtn, label)
			return btn.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !u.cityDropdownOpen {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(u.theme, &u.citySearchEditor, u.tr("settings.search_city", "Search city (case-insensitive)"))
						return ed.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(filtered) == 0 {
							return material.Body2(u.theme, u.tr("settings.no_city_match", "No matching cities")).Layout(gtx)
						}
						// Render all filtered options and delegate scrolling to the Settings page list.
						children := make([]layout.FlexChild, 0, len(filtered)*2)
						for _, cityIndex := range filtered {
							idx := cityIndex
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								enabledLabel := u.cities[idx].Name
								if idx == u.draftSelectedCity {
									enabledLabel = "• " + enabledLabel
								}
								btn := material.Button(u.theme, &u.cityButtons[idx], enabledLabel)
								return btn.Layout(gtx)
							}))
							children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					}),
				)
			})
		}),
	)
}

func (u *UI) layoutLanguageDropdown(gtx layout.Context) layout.Dimensions {
	u.syncLanguageButtons()
	if len(u.cfg.Languages) == 0 {
		return material.Body2(u.theme, u.tr("settings.no_languages", "No languages found")).Layout(gtx)
	}
	filtered := u.filteredLanguageIndices()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := fmt.Sprintf("%s: %s", u.tr("settings.language", "Language"), u.languageDisplayName(u.settingsLanguage))
			if u.languageDropdownOpen {
				label += " ▲"
			} else {
				label += " ▼"
			}
			btn := material.Button(u.theme, &u.languageToggleBtn, label)
			return btn.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !u.languageDropdownOpen {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(u.theme, &u.languageSearchEditor, u.tr("settings.search_language", "Search language"))
						return ed.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(filtered) == 0 {
							return material.Body2(u.theme, u.tr("settings.no_language_match", "No matching languages")).Layout(gtx)
						}
						children := make([]layout.FlexChild, 0, len(filtered)*2)
						for i, langIndex := range filtered {
							idx := langIndex
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := u.languageDisplayName(u.cfg.Languages[idx])
								if u.cfg.Languages[idx] == u.settingsLanguage {
									label = "• " + label
								}
								btn := material.Button(u.theme, &u.languageButtons[idx], label)
								return btn.Layout(gtx)
							}))
							if i < len(filtered)-1 {
								children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
							}
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					}),
				)
			})
		}),
	)
}

func (u *UI) selectCity(index int) {
	if index < 0 || index >= len(u.cities) {
		return
	}
	u.cityDropdownOpen = false
	u.citySearchEditor.SetText("")
	u.draftSelectedCity = index
	u.latEditor.SetText(fmt.Sprintf("%.4f", u.cities[index].Lat))
	u.lonEditor.SetText(fmt.Sprintf("%.4f", u.cities[index].Lon))
	u.setStatus(
		u.trf("status.city_selected", "City selected: %s (%.4f, %.4f)", u.cities[index].Name, u.cities[index].Lat, u.cities[index].Lon),
		false,
	)
}

func (u *UI) selectLanguageDraft(language string) {
	if language == "" {
		language = "system"
	}
	if !containsString(u.cfg.Languages, language) {
		u.setStatus(u.trf("status.unknown_language", "Unknown language: %s", language), true)
		return
	}
	if u.settingsLanguage == language {
		return
	}
	u.settingsLanguage = language
	u.languageDropdownOpen = false
	u.languageSearchEditor.SetText("")
}

func (u *UI) getCurrentLocationViaGPS() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lat, lon, err := u.gps.CurrentLocation(ctx)
	if err != nil {
		u.setStatus(u.trf("status.gps_unavailable", "GPS location unavailable: %v", err), true)
		return
	}

	u.draftSelectedCity = -1
	u.latEditor.SetText(fmt.Sprintf("%.4f", lat))
	u.lonEditor.SetText(fmt.Sprintf("%.4f", lon))
	u.setStatus(
		u.trf("status.gps_applied", "GPS location applied: %.4f, %.4f", lat, lon),
		false,
	)
}

func (u *UI) syncLocationFromEditors() error {
	latText := strings.TrimSpace(u.latEditor.Text())
	lonText := strings.TrimSpace(u.lonEditor.Text())
	lat, err := strconv.ParseFloat(latText, 64)
	if err != nil {
		return fmt.Errorf(u.tr("error.invalid_latitude", "invalid latitude: %q"), latText)
	}
	lon, err := strconv.ParseFloat(lonText, 64)
	if err != nil {
		return fmt.Errorf(u.tr("error.invalid_longitude", "invalid longitude: %q"), lonText)
	}
	if lat < -90 || lat > 90 {
		return errors.New(u.tr("error.latitude_range", "latitude must be in [-90, 90]"))
	}
	if lon < -180 || lon > 180 {
		return errors.New(u.tr("error.longitude_range", "longitude must be in [-180, 180]"))
	}

	u.locationLat = lat
	u.locationLon = lon
	u.selectedCity = u.cityIndexForCoordinates(lat, lon)
	return nil
}

func (u *UI) currentDraftCityName() string {
	if u.draftSelectedCity < 0 || u.draftSelectedCity >= len(u.cities) {
		return u.tr("settings.custom_city", "custom")
	}
	return u.cities[u.draftSelectedCity].Name
}

func (u *UI) filteredLanguageIndices() []int {
	query := strings.ToLower(strings.TrimSpace(u.languageSearchEditor.Text()))
	out := make([]int, 0, len(u.cfg.Languages))
	for idx, code := range u.cfg.Languages {
		label := strings.ToLower(u.languageDisplayName(code))
		if query == "" || strings.Contains(label, query) || strings.Contains(strings.ToLower(code), query) {
			out = append(out, idx)
		}
	}
	return out
}

func (u *UI) filteredCityIndices() []int {
	query := strings.ToLower(strings.TrimSpace(u.citySearchEditor.Text()))
	out := make([]int, 0, len(u.cities))
	for idx := range u.cities {
		if query == "" || strings.Contains(strings.ToLower(u.cities[idx].Name), query) {
			out = append(out, idx)
		}
	}
	return out
}

func (u *UI) setStatus(message string, isError bool) {
	u.statusMessage = message
	u.statusMessageError = isError
}

func (u *UI) refreshLanguagesFromBundle() {
	if u.i18n == nil {
		u.syncLanguageButtons()
		return
	}
	u.cfg.Languages = u.i18n.AvailableLanguages()
	lang := strings.TrimSpace(u.cfg.Language)
	if lang == "" {
		u.cfg.Language = "system"
		u.syncLanguageButtons()
		return
	}
	if lang == "system" {
		u.syncLanguageButtons()
		return
	}
	for _, candidate := range u.cfg.Languages {
		if candidate == lang {
			u.syncLanguageButtons()
			return
		}
	}
	u.cfg.Language = "system"
	u.syncLanguageButtons()
}

func (u *UI) tr(key, fallback string) string {
	if u.i18n == nil {
		return fallback
	}
	return u.i18n.Text(u.cfg.Language, key, fallback)
}

func (u *UI) trf(key, fallback string, args ...any) string {
	return fmt.Sprintf(u.tr(key, fallback), args...)
}

func (u *UI) resetSettingsDraft() {
	editors := []*widget.Editor{
		&u.setPressureMediumEditor,
		&u.setPressureHighEditor,
		&u.setPressureCritEditor,
		&u.setKMediumEditor,
		&u.setKHighEditor,
		&u.setKCritEditor,
		&u.setScheduleEditor,
		&u.setRetentionDaysEditor,
	}
	for _, ed := range editors {
		ed.SingleLine = true
	}
	u.setPressureMediumEditor.SetText(fmt.Sprintf("%.1f", u.cfg.Pressure.Medium))
	u.setPressureHighEditor.SetText(fmt.Sprintf("%.1f", u.cfg.Pressure.High))
	u.setPressureCritEditor.SetText(fmt.Sprintf("%.1f", u.cfg.Pressure.Crit))
	u.setKMediumEditor.SetText(fmt.Sprintf("%.1f", u.cfg.KIndex.Medium))
	u.setKHighEditor.SetText(fmt.Sprintf("%.1f", u.cfg.KIndex.High))
	u.setKCritEditor.SetText(fmt.Sprintf("%.1f", u.cfg.KIndex.Crit))
	u.setScheduleEditor.SetText(fmt.Sprintf("%d", u.cfg.Schedule.PeriodMinutes))
	u.setRetentionDaysEditor.SetText(fmt.Sprintf("%d", u.cfg.Retention.DefaultDays))
	u.settingsPressureUnit = u.cfg.Units.PressureUnit
	u.settingsTimeFormat = u.cfg.Units.TimeFormat
	u.settingsNotificationsEnabled = u.cfg.Notifications.Enabled
	u.settingsRunWhenClosed = u.cfg.Schedule.RunWhenClosed
	u.settingsLanguage = strings.TrimSpace(u.cfg.Language)
	if u.settingsLanguage == "" {
		u.settingsLanguage = "system"
	}
	u.languageDropdownOpen = false
	u.languageSearchEditor.SetText("")
	u.syncLanguageButtons()
	u.updateDirtyState()
}

func (u *UI) resetLocationDraft() {
	u.latEditor.SingleLine = true
	u.lonEditor.SingleLine = true
	u.citySearchEditor.SingleLine = true
	u.latEditor.SetText(fmt.Sprintf("%.4f", u.locationLat))
	u.lonEditor.SetText(fmt.Sprintf("%.4f", u.locationLon))
	u.draftSelectedCity = u.selectedCity
	u.cityDropdownOpen = false
	u.citySearchEditor.SetText("")
	u.updateDirtyState()
}

func (u *UI) updateDirtyState() {
	u.settingsDirty = u.isSettingsDirty()
	u.locationDirty = u.isLocationDirty()
}

func (u *UI) isSettingsDirty() bool {
	if strings.TrimSpace(u.setPressureMediumEditor.Text()) != fmt.Sprintf("%.1f", u.cfg.Pressure.Medium) {
		return true
	}
	if strings.TrimSpace(u.setPressureHighEditor.Text()) != fmt.Sprintf("%.1f", u.cfg.Pressure.High) {
		return true
	}
	if strings.TrimSpace(u.setPressureCritEditor.Text()) != fmt.Sprintf("%.1f", u.cfg.Pressure.Crit) {
		return true
	}
	if strings.TrimSpace(u.setKMediumEditor.Text()) != fmt.Sprintf("%.1f", u.cfg.KIndex.Medium) {
		return true
	}
	if strings.TrimSpace(u.setKHighEditor.Text()) != fmt.Sprintf("%.1f", u.cfg.KIndex.High) {
		return true
	}
	if strings.TrimSpace(u.setKCritEditor.Text()) != fmt.Sprintf("%.1f", u.cfg.KIndex.Crit) {
		return true
	}
	if strings.TrimSpace(u.setScheduleEditor.Text()) != fmt.Sprintf("%d", u.cfg.Schedule.PeriodMinutes) {
		return true
	}
	if strings.TrimSpace(u.setRetentionDaysEditor.Text()) != fmt.Sprintf("%d", u.cfg.Retention.DefaultDays) {
		return true
	}
	if u.settingsPressureUnit != u.cfg.Units.PressureUnit {
		return true
	}
	if u.settingsTimeFormat != u.cfg.Units.TimeFormat {
		return true
	}
	if u.settingsNotificationsEnabled != u.cfg.Notifications.Enabled {
		return true
	}
	if u.settingsRunWhenClosed != u.cfg.Schedule.RunWhenClosed {
		return true
	}
	if strings.TrimSpace(u.settingsLanguage) != strings.TrimSpace(u.cfg.Language) {
		return true
	}
	return false
}

func (u *UI) isLocationDirty() bool {
	if strings.TrimSpace(u.latEditor.Text()) != fmt.Sprintf("%.4f", u.locationLat) {
		return true
	}
	if strings.TrimSpace(u.lonEditor.Text()) != fmt.Sprintf("%.4f", u.locationLon) {
		return true
	}
	return u.draftSelectedCity != u.selectedCity
}

func (u *UI) setScreen(next Screen, compact bool) {
	if u.screen != next {
		u.onScreenLeave(u.screen)
		u.lastScreen = u.screen
		u.screen = next
	}
	if compact {
		u.menuOpen = false
	}
}

func (u *UI) onScreenLeave(from Screen) {
	switch from {
	case ScreenSettings:
		u.resetSettingsDraft()
	case ScreenLocation:
		u.resetLocationDraft()
	}
}

func (u *UI) cityIndexForCoordinates(lat, lon float64) int {
	for i := range u.cities {
		if almostEqual(u.cities[i].Lat, lat) && almostEqual(u.cities[i].Lon, lon) {
			return i
		}
	}
	return -1
}

func (u *UI) applySettingsFromEditors() error {
	cfg := u.cfg

	pressureMedium, err := u.parseFloatEditor(&u.setPressureMediumEditor, u.tr("settings.pressure_medium", "Pressure medium"))
	if err != nil {
		return err
	}
	pressureHigh, err := u.parseFloatEditor(&u.setPressureHighEditor, u.tr("settings.pressure_high", "Pressure high"))
	if err != nil {
		return err
	}
	pressureCrit, err := u.parseFloatEditor(&u.setPressureCritEditor, u.tr("settings.pressure_critical", "Pressure critical"))
	if err != nil {
		return err
	}
	kMedium, err := u.parseFloatEditor(&u.setKMediumEditor, u.tr("settings.k_medium", "K medium"))
	if err != nil {
		return err
	}
	kHigh, err := u.parseFloatEditor(&u.setKHighEditor, u.tr("settings.k_high", "K high"))
	if err != nil {
		return err
	}
	kCrit, err := u.parseFloatEditor(&u.setKCritEditor, u.tr("settings.k_critical", "K critical"))
	if err != nil {
		return err
	}
	scheduleMinutes, err := u.parseIntEditor(&u.setScheduleEditor, u.tr("settings.schedule_minutes", "Schedule minutes"))
	if err != nil {
		return err
	}
	retentionDays, err := u.parseIntEditor(&u.setRetentionDaysEditor, u.tr("settings.retention_days", "Retention days"))
	if err != nil {
		return err
	}
	cfg.Pressure = domain.PressureThresholds{
		Medium: pressureMedium,
		High:   pressureHigh,
		Crit:   pressureCrit,
	}
	cfg.KIndex = domain.KIndexThresholds{
		Medium: kMedium,
		High:   kHigh,
		Crit:   kCrit,
	}
	cfg.Schedule.PeriodMinutes = scheduleMinutes
	cfg.Schedule.RunWhenClosed = u.settingsRunWhenClosed
	cfg.Retention.DefaultDays = retentionDays
	cfg.Units.PressureUnit = strings.TrimSpace(u.settingsPressureUnit)
	if cfg.Units.PressureUnit == "" {
		cfg.Units.PressureUnit = "hPa"
	}
	cfg.Units.TimeFormat = strings.TrimSpace(u.settingsTimeFormat)
	if cfg.Units.TimeFormat == "" {
		cfg.Units.TimeFormat = "24h"
	}
	cfg.Notifications.Enabled = u.settingsNotificationsEnabled
	cfg.Language = strings.TrimSpace(u.settingsLanguage)
	if cfg.Language == "" {
		cfg.Language = "system"
	}
	if cfg.Language != "system" && !containsString(cfg.Languages, cfg.Language) {
		return fmt.Errorf(u.tr("error.unknown_language_code", "unknown language code: %s"), cfg.Language)
	}

	if err := domain.ValidateConfig(cfg); err != nil {
		return err
	}

	u.cfg = cfg
	u.recomputeRisk()
	u.pruneHistory()
	u.scheduleNextCheck(time.Now())
	u.saveState()
	return nil
}

func (u *UI) shouldRunScheduledCheck(now time.Time) bool {
	if !u.hasChecked {
		return false
	}
	if u.cfg.Schedule.PeriodMinutes < u.cfg.Schedule.MinMinutes {
		return false
	}
	if u.nextScheduledCheck.IsZero() {
		u.scheduleNextCheck(now)
		return false
	}
	return !u.nextScheduledCheck.After(now)
}

func (u *UI) scheduleNextCheck(from time.Time) {
	period := time.Duration(u.cfg.Schedule.PeriodMinutes) * time.Minute
	if period <= 0 {
		u.nextScheduledCheck = time.Time{}
		return
	}
	u.nextScheduledCheck = from.Add(period)
}

func (u *UI) formatTime(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	if u.cfg.Units.TimeFormat == "12h" {
		return ts.Local().Format("2006-01-02 03:04:05 PM")
	}
	return ts.Local().Format("2006-01-02 15:04:05")
}

func (u *UI) pruneHistory() {
	if u.cfg.Retention.DefaultDays <= 0 {
		return
	}

	cutoff := time.Now().Add(-time.Duration(u.cfg.Retention.DefaultDays) * 24 * time.Hour)
	filtered := make([]service.Result, 0, len(u.history))
	for i := range u.history {
		if u.history[i].CheckedAt.After(cutoff) {
			filtered = append(filtered, u.history[i])
		}
	}
	u.history = filtered
}

func (u *UI) loadState() {
	if u.store == nil {
		return
	}
	state, err := u.store.Load()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			u.setStatus(u.trf("status.state_load_error", "State load error: %v", err), true)
		}
		return
	}

	u.cfg = state.Config
	if u.cfg.Schedule.MinMinutes < 15 {
		u.cfg.Schedule.MinMinutes = 15
	}
	if u.cfg.Retention.MaxYears < 1 {
		u.cfg.Retention.MaxYears = 50
	}
	if u.cfg.Units.PressureUnit == "" {
		u.cfg.Units.PressureUnit = "hPa"
	}
	if u.cfg.Units.TimeFormat == "" {
		u.cfg.Units.TimeFormat = "24h"
	}
	if u.cfg.Language == "" {
		u.cfg.Language = "system"
	}
	if len(u.cfg.Languages) == 0 {
		u.cfg.Languages = []string{"system", "en", "de", "uk"}
	}
	u.syncLanguageButtons()
	u.locationLat = state.LocationLat
	u.locationLon = state.LocationLon
	u.selectedCity = state.SelectedCity
	if u.selectedCity < -1 || u.selectedCity >= len(u.cities) {
		u.selectedCity = 0
	}
	if u.locationLat == 0 && u.locationLon == 0 {
		if u.selectedCity >= 0 && u.selectedCity < len(u.cities) {
			u.locationLat = u.cities[u.selectedCity].Lat
			u.locationLon = u.cities[u.selectedCity].Lon
		} else if len(u.cities) > 0 {
			u.selectedCity = 0
			u.locationLat = u.cities[0].Lat
			u.locationLon = u.cities[0].Lon
		}
	}
	u.metrics = state.Metrics
	u.history = state.History
	u.hasChecked = state.HasChecked
	if state.LastCheckUTC > 0 {
		u.lastCheck = time.Unix(state.LastCheckUTC, 0).UTC()
		u.scheduleNextCheck(u.lastCheck)
	}
	u.resetSettingsDraft()
	u.resetLocationDraft()
	u.recomputeRisk()
}

func (u *UI) saveState() {
	if u.store == nil {
		return
	}

	lastCheckUTC := int64(0)
	if !u.lastCheck.IsZero() {
		lastCheckUTC = u.lastCheck.Unix()
	}

	_ = u.store.Save(storage.State{
		Config:       u.cfg,
		LocationLat:  u.locationLat,
		LocationLon:  u.locationLon,
		SelectedCity: u.selectedCity,
		History:      u.history,
		Metrics:      u.metrics,
		LastCheckUTC: lastCheckUTC,
		HasChecked:   u.hasChecked,
	})
}

func defaultLocationIndex(selected location.City, cities []location.City) int {
	for i := range cities {
		if cities[i].Name == selected.Name {
			return i
		}
	}
	return 0
}

func (u *UI) parseFloatEditor(editor *widget.Editor, label string) (float64, error) {
	raw := strings.TrimSpace(editor.Text())
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf(u.tr("error.invalid_value", "invalid %s: %q"), label, raw)
	}
	return value, nil
}

func (u *UI) parseIntEditor(editor *widget.Editor, label string) (int, error) {
	raw := strings.TrimSpace(editor.Text())
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf(u.tr("error.invalid_value", "invalid %s: %q"), label, raw)
	}
	return value, nil
}

func selectedLabel(selected bool, text string) string {
	if selected {
		return "• " + text
	}
	return text
}

func almostEqual(a, b float64) bool {
	delta := a - b
	if delta < 0 {
		delta = -delta
	}
	return delta < 0.00005
}

func containsString(items []string, needle string) bool {
	for i := range items {
		if items[i] == needle {
			return true
		}
	}
	return false
}

func (u *UI) syncLanguageButtons() {
	if len(u.cfg.Languages) == 0 {
		u.cfg.Languages = []string{"system"}
	}
	if len(u.languageButtons) == len(u.cfg.Languages) {
		return
	}
	u.languageButtons = make([]widget.Clickable, len(u.cfg.Languages))
}

func (u *UI) languageDisplayName(code string) string {
	switch code {
	case "system":
		if u.i18n == nil {
			return u.tr("settings.system_default", "System default")
		}
		resolved := u.i18n.ResolveLanguage("system")
		return fmt.Sprintf("%s (%s)", u.tr("settings.system_default", "System default"), strings.ToUpper(resolved))
	case "en":
		return u.tr("settings.lang_en", "English")
	case "de":
		return u.tr("settings.lang_de", "Deutsch")
	case "uk":
		return u.tr("settings.lang_uk", "Ukrainian")
	default:
		if code == "" {
			return u.tr("settings.system_default", "System default")
		}
		return strings.ToUpper(code)
	}
}

func (u *UI) riskLabel(level domain.RiskLevel) string {
	switch level {
	case domain.RiskLow:
		return u.tr("risk.low", "LOW")
	case domain.RiskMedium:
		return u.tr("risk.medium", "MEDIUM")
	case domain.RiskHigh:
		return u.tr("risk.high", "HIGH")
	case domain.RiskCritical:
		return u.tr("risk.critical", "CRITICAL")
	default:
		return u.tr("risk.unknown", "UNKNOWN")
	}
}

func (u *UI) checkReasonLabel(reason string) string {
	switch reason {
	case "startup":
		return u.tr("check_reason.startup", "startup")
	case "scheduled":
		return u.tr("check_reason.scheduled", "scheduled")
	case "manual":
		return u.tr("check_reason.manual", "manual")
	default:
		return reason
	}
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
