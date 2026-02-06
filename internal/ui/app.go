package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
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
	"gioui.org/text"
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

	checkNowBtn         widget.Clickable
	homeCheckNowBtn     widget.Clickable
	testPageTestBtn     widget.Clickable
	applySettingsBtn    widget.Clickable
	homeDetailsBtn      widget.Clickable
	settingsNotifSwitch widget.Bool
	settingsTimeSwitch  widget.Bool
	settingsBgSwitch    widget.Bool

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
	setThemeSystemBtn    widget.Clickable
	setThemeLightBtn     widget.Clickable
	setThemeDarkBtn      widget.Clickable
	getGPSBtn            widget.Clickable
	languageButtons      []widget.Clickable
	languageToggleBtn    widget.Clickable
	languageSearchEditor widget.Editor
	languageDropdownOpen bool
	settingsLanguage     string
	settingsThemeMode    string

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
	homeList      layout.List
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
	lastRiskUpdate       time.Time
	homeDetailsExpanded  bool
	systemThemeMode      string
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
		homeList: layout.List{
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
		lastRiskUpdate:   time.Now(),
		systemThemeMode:  detectSystemThemeMode(),
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
	for u.homeCheckNowBtn.Clicked(gtx) {
		u.runCheckNow()
	}
	for u.homeDetailsBtn.Clicked(gtx) {
		u.homeDetailsExpanded = !u.homeDetailsExpanded
		u.saveState()
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
	if u.settingsNotifSwitch.Update(gtx) {
		u.settingsNotificationsEnabled = u.settingsNotifSwitch.Value
	}
	if u.settingsBgSwitch.Update(gtx) {
		u.settingsRunWhenClosed = u.settingsBgSwitch.Value
	}
	if u.settingsTimeSwitch.Update(gtx) {
		if u.settingsTimeSwitch.Value {
			u.settingsTimeFormat = "24h"
		} else {
			u.settingsTimeFormat = "12h"
		}
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
	for u.setThemeSystemBtn.Clicked(gtx) {
		u.settingsThemeMode = "system"
	}
	for u.setThemeLightBtn.Clicked(gtx) {
		u.settingsThemeMode = "light"
	}
	for u.setThemeDarkBtn.Clicked(gtx) {
		u.settingsThemeMode = "dark"
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
	u.lastRiskUpdate = time.Now()
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
			u.trf("status.notification_delivery_error", "Notification delivery error: %v", err),
			true,
		)
		return
	}
}

func (u *UI) recomputeRisk() {
	u.pressureRisk = domain.RiskFromPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Pressure)
	u.kIndexRisk = domain.RiskFromKIndex(u.metrics.KIndex, u.cfg.KIndex)
	u.overallRisk = domain.AggregateRisk(u.pressureRisk, u.kIndexRisk)
}

func (u *UI) layout(gtx layout.Context) layout.Dimensions {
	u.applyThemePalette()
	paint.FillShape(
		gtx.Ops,
		u.theme.Palette.Bg,
		clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op(),
	)

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
			btn := material.Button(u.theme, &u.menuOpenBtn, "☰")
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.H6(u.theme, u.tr("app.title", "Wetterabhaengig"))
			label.Alignment = text.Middle
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
			sub.Color = u.mutedTextColor()
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
	const sections = 4
	return u.homeList.Layout(gtx, sections, func(gtx layout.Context, index int) layout.Dimensions {
		switch index {
		case 0:
			return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, u.layoutTrafficLight)
			})
		case 1:
			return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, u.layoutHomeSummaryCard)
		case 2:
			return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.theme, &u.homeCheckNowBtn, u.tr("buttons.check_now", "Check now"))
				return btn.Layout(gtx)
			})
		default:
			return u.layoutHomeDetails(gtx)
		}
	})
}

func (u *UI) layoutHomeSummaryCard(gtx layout.Context) layout.Dimensions {
	return widget.Border{
		Color:        u.subtleBorderColor(),
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H6(u.theme, u.trf("home.overall_risk", "Overall risk: %s", u.riskLabel(u.overallRisk)))
					return title.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					text := material.Body1(u.theme, u.riskMeaningText())
					text.Color = u.mutedTextColor()
					return text.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					h := material.Body2(u.theme, u.tr("home.metric_pressure_header", "Atmosphere pressure"))
					h.Color = u.mutedTextColor()
					return h.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					primaryPressure, _ := domain.ConvertPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Units.PressureUnit)
					line := u.trf(
						"home.metric_pressure_line",
						"Delta Pressure (%s): %s -- %s",
						u.pressureWindowLabel(),
						u.valueWithUnit(primaryPressure, u.cfg.Units.PressureUnit),
						u.pressureBandLabel(),
					)
					return material.Body1(u.theme, line).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					h := material.Body2(u.theme, u.tr("home.metric_k_header", "Solar activity"))
					h.Color = u.mutedTextColor()
					return h.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					line := u.trf(
						"home.metric_k_line",
						"K-index: %d -- %s",
						int(math.Round(u.metrics.KIndex)),
						u.kBandLabel(),
					)
					return material.Body1(u.theme, line).Layout(gtx)
				}),
			)
		})
	})
}

func (u *UI) layoutHomeDetails(gtx layout.Context) layout.Dimensions {
	lastCheckShort := "-"
	if !u.lastCheck.IsZero() {
		lastCheckShort = u.formatShortTime(u.lastCheck)
	}

	headerLabel := u.tr("details.title", "Details")
	chevron := "▾"
	if u.homeDetailsExpanded {
		chevron = "▴"
	}

	return widget.Border{
		Color:        u.subtleBorderColor(),
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					minH := gtx.Dp(unit.Dp(44))
					if gtx.Constraints.Min.Y < minH {
						gtx.Constraints.Min.Y = minH
					}
					return u.homeDetailsBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								title := material.Body1(u.theme, headerLabel)
								return title.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								snippet := material.Body2(u.theme, u.trf("details.header_snippet", "Last check: %s", lastCheckShort))
								snippet.Color = u.mutedTextColor()
								return snippet.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								s := material.Body1(u.theme, chevron)
								s.Color = u.mutedTextColor()
								return s.Layout(gtx)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !u.homeDetailsExpanded {
						primaryPressure, _ := domain.ConvertPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Units.PressureUnit)
						preview := u.trf(
							"details.preview",
							"Delta Pressure: %.1f %s • K-index: %d • Next: %s",
							primaryPressure,
							u.cfg.Units.PressureUnit,
							int(math.Round(u.metrics.KIndex)),
							u.nextCheckDisplayShort(),
						)
						txt := material.Body2(u.theme, preview)
						txt.Color = u.mutedTextColor()
						return txt.Layout(gtx)
					}

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(u.layoutDetailsTriggerGroup),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(u.layoutDetailsChecksGroup),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(u.layoutDetailsMeasurementsGroup),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(u.layoutDetailsLocationGroup),
					)
				}),
			)
		})
	})
}

func (u *UI) layoutDetailsChecksGroup(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutGroupTitle(gtx, u.tr("group.checks", "Checks & schedule"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lastCheck := u.tr("details.none", "None")
			if !u.lastCheck.IsZero() {
				lastCheck = u.formatTime(u.lastCheck)
			}
			return u.layoutDetailRow(gtx, u.tr("details.lastCheck", "Last check"), lastCheck)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.nextCheck", "Next scheduled check"), u.nextCheckDisplay())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.interval", "Check interval"), u.trf("details.interval_minutes", "%d min", u.cfg.Schedule.PeriodMinutes))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.timeFormat", "Time format"), u.cfg.Units.TimeFormat)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			state := u.tr("common.off", "OFF")
			if u.cfg.Notifications.Enabled {
				state = u.tr("common.on", "ON")
			}
			return u.layoutDetailRow(gtx, u.tr("details.notifications", "Notifications"), state)
		}),
	)
}

func (u *UI) layoutDetailsTriggerGroup(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutGroupTitle(gtx, u.tr("group.trigger", "Trigger"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.primaryDriver", "Primary driver"), u.primaryDriverText())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			drivers := u.trf(
				"details.drivers_value",
				"Pressure: %s • K-index: %s",
				u.riskLabel(u.pressureRisk),
				u.riskLabel(u.kIndexRisk),
			)
			return u.layoutDetailRow(gtx, u.tr("details.drivers", "Drivers"), drivers)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.outOfRange", "Out of range"), u.outOfRangeDetailsText())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			medium, _ := domain.ConvertPressureDelta(u.cfg.Pressure.Medium, u.cfg.Units.PressureUnit)
			high, _ := domain.ConvertPressureDelta(u.cfg.Pressure.High, u.cfg.Units.PressureUnit)
			critical, _ := domain.ConvertPressureDelta(u.cfg.Pressure.Crit, u.cfg.Units.PressureUnit)
			value := u.trf(
				"details.pressureThresholdsValue",
				"Medium > %.1f %s, High > %.1f %s, Critical > %.1f %s",
				medium, u.cfg.Units.PressureUnit,
				high, u.cfg.Units.PressureUnit,
				critical, u.cfg.Units.PressureUnit,
			)
			return u.layoutDetailRow(gtx, u.tr("details.pressureThresholds", "Delta Pressure thresholds"), value)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := u.trf(
				"details.kThresholdsValue",
				"Medium >= %.1f, High >= %.1f, Critical >= %.1f",
				u.cfg.KIndex.Medium,
				u.cfg.KIndex.High,
				u.cfg.KIndex.Crit,
			)
			return u.layoutDetailRow(gtx, u.tr("details.kThresholds", "K-index thresholds"), value)
		}),
	)
}

func (u *UI) layoutDetailsMeasurementsGroup(gtx layout.Context) layout.Dimensions {
	primaryPressure, _ := domain.ConvertPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Units.PressureUnit)
	measurementValue := u.trf(
		"details.measure_pressure",
		"%s (%s): %s (%s mmHg • %s inHg)",
		u.tr("details.pressure_delta_label", "Delta Pressure"),
		u.pressureWindowLabel(),
		u.valueWithUnit(primaryPressure, u.cfg.Units.PressureUnit),
		u.valueWithUnit(domain.PressureDeltaMMHg(u.metrics.PressureDeltaHPa), "mmHg"),
		u.valueWithUnit(domain.PressureDeltaInHg(u.metrics.PressureDeltaHPa), "inHg"),
	)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutGroupTitle(gtx, u.tr("group.measurements", "Measurements"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.measurements", "Measurements"), measurementValue)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			kValue := u.trf(
				"details.measure_k",
				"K-index: %d (%s)",
				int(math.Round(u.metrics.KIndex)),
				u.kBandLabel(),
			)
			return u.layoutDetailRow(gtx, u.tr("details.kIndex", "K-index"), kValue)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.dataSource", "Data source"), u.tr("details.dataSourceValue", "Open-Meteo + NOAA SWPC"))
		}),
	)
}

func (u *UI) layoutDetailsLocationGroup(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutGroupTitle(gtx, u.tr("group.location", "Location"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			locationName := u.currentCityName()
			return u.layoutDetailRow(gtx, u.tr("group.location", "Location"), locationName)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutDetailRow(gtx, u.tr("details.coordinates", "Coordinates"), u.trf("details.coordinates_value", "%.4f, %.4f", u.locationLat, u.locationLon))
		}),
	)
}

func (u *UI) layoutGroupTitle(gtx layout.Context, text string) layout.Dimensions {
	label := material.Body1(u.theme, text)
	label.Color = u.mutedTextColor()
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, label.Layout)
}

func (u *UI) layoutDetailRow(gtx layout.Context, label, value string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(u.theme, label)
				lbl.Color = u.mutedTextColor()
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
				return material.Body2(u.theme, value).Layout(gtx)
			}),
		)
	})
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
	u.settingsNotifSwitch.Value = u.settingsNotificationsEnabled
	u.settingsBgSwitch.Value = u.settingsRunWhenClosed
	u.settingsTimeSwitch.Value = u.settingsTimeFormat == "24h"
	stateOn := u.tr("common.on", "ON")
	stateOff := u.tr("common.off", "OFF")
	notificationState := stateOff
	if u.settingsNotificationsEnabled {
		notificationState = stateOn
	}
	backgroundState := stateOff
	if u.settingsRunWhenClosed {
		backgroundState = stateOn
	}
	timeState := u.tr("settings.time_12h", "12h")
	if u.settingsTimeSwitch.Value {
		timeState = u.tr("settings.time_24h", "24h")
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
					txt := material.Body1(u.theme, u.tr("settings.theme_mode", "Theme mode"))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setThemeSystemBtn, selectedLabel(u.settingsThemeMode == "system", u.tr("settings.theme_system", "System")))
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setThemeLightBtn, selectedLabel(u.settingsThemeMode == "light", u.tr("settings.theme_light", "Light")))
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(u.theme, &u.setThemeDarkBtn, selectedLabel(u.settingsThemeMode == "dark", u.tr("settings.theme_dark", "Dark")))
							return btn.Layout(gtx)
						}),
					)
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
					txt := material.Body1(u.theme, u.tr("settings.time_format_title", "Time format"))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(u.theme, u.trf("settings.current_state", "Current: %s", timeState))
					txt.Color = u.mutedTextColor()
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sw := material.Switch(u.theme, &u.settingsTimeSwitch, u.tr("settings.time_format_switch", "Use 24-hour format"))
					return sw.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(u.theme, u.tr("settings.time_format_help", "ON means 24-hour clock. OFF means 12-hour AM/PM clock."))
					txt.Color = u.mutedTextColor()
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body1(u.theme, u.tr("settings.background_checks_title", "Background checks when app is closed"))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(u.theme, u.trf("settings.current_state", "Current: %s", backgroundState))
					txt.Color = u.mutedTextColor()
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sw := material.Switch(u.theme, &u.settingsBgSwitch, u.tr("settings.background_checks_switch", "Run scheduled checks while app is closed (Android)"))
					return sw.Layout(gtx)
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
					txt := material.Body1(u.theme, u.tr("settings.notifications_title", "Notifications"))
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(u.theme, u.trf("settings.current_state", "Current: %s", notificationState))
					txt.Color = u.mutedTextColor()
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sw := material.Switch(u.theme, &u.settingsNotifSwitch, u.tr("settings.notifications_switch", "Enable local notifications"))
					return sw.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					txt := material.Body2(u.theme, u.tr("settings.notifications_help", "When enabled, the app sends a local notification when risk level changes."))
					txt.Color = u.mutedTextColor()
					return txt.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutPrimaryButton(gtx, &u.applySettingsBtn, u.tr("buttons.save_settings", "Save settings"), u.settingsDirty)
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
	baseSize := float32(gtx.Dp(unit.Dp(126)))
	scale := float32(1.0)
	if !u.lastRiskUpdate.IsZero() {
		age := time.Since(u.lastRiskUpdate)
		if age < 500*time.Millisecond {
			t := float32(age) / float32(500*time.Millisecond)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			scale *= 0.9 + 0.1*t
			gtx.Execute(op.InvalidateCmd{At: time.Now().Add(16 * time.Millisecond)})
		}
	}
	if u.overallRisk == domain.RiskCritical {
		phase := float64(time.Now().UnixNano()%int64(1300*time.Millisecond)) / float64(1300*time.Millisecond)
		pulse := 0.95 + 0.05*float32((math.Sin(2*math.Pi*phase)+1)/2)
		scale *= pulse
		gtx.Execute(op.InvalidateCmd{At: time.Now().Add(16 * time.Millisecond)})
	}

	size := int(baseSize * scale)
	if size < gtx.Dp(unit.Dp(96)) {
		size = gtx.Dp(unit.Dp(96))
	}
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
	target := riskColor(u.overallRisk)
	if u.overallRisk == domain.RiskLow || u.overallRisk == domain.RiskMedium || u.overallRisk == domain.RiskHigh || u.overallRisk == domain.RiskCritical {
		target = riskColor(u.overallRisk)
	} else {
		target = color.NRGBA{A: 255, R: 120, G: 120, B: 120}
	}
	if u.lastRiskUpdate.IsZero() {
		return target
	}

	age := time.Since(u.lastRiskUpdate)
	if age >= 550*time.Millisecond {
		return target
	}

	t := float64(age) / float64(550*time.Millisecond)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	start := color.NRGBA{A: 255, R: 150, G: 150, B: 150}
	return blendColor(start, target, float32(t))
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
			"k-index %d (>=%.1f)",
			int(math.Round(u.metrics.KIndex)),
			thresholdLabelK(u.kIndexRisk, u.cfg.KIndex),
		)
	}

	return u.trf(
		"notification.payload",
		"Risk: %s. Pressure delta %.2f hPa. K-index %d. Out of range: %s. Location: %.4f, %.4f.",
		u.riskLabel(u.overallRisk),
		u.metrics.PressureDeltaHPa,
		int(math.Round(u.metrics.KIndex)),
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
	u.settingsNotifSwitch.Value = u.settingsNotificationsEnabled
	u.settingsRunWhenClosed = u.cfg.Schedule.RunWhenClosed
	u.settingsBgSwitch.Value = u.settingsRunWhenClosed
	u.settingsTimeSwitch.Value = u.settingsTimeFormat == "24h"
	u.settingsThemeMode = strings.TrimSpace(u.cfg.ThemeMode)
	if u.settingsThemeMode == "" {
		u.settingsThemeMode = "system"
	}
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
	if strings.TrimSpace(u.settingsThemeMode) != strings.TrimSpace(u.cfg.ThemeMode) {
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
	cfg.ThemeMode = strings.TrimSpace(u.settingsThemeMode)
	if cfg.ThemeMode == "" {
		cfg.ThemeMode = "system"
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
	if u.cfg.ThemeMode == "" {
		u.cfg.ThemeMode = "system"
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
	u.homeDetailsExpanded = state.HomeDetailsExpanded
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
		Config:              u.cfg,
		LocationLat:         u.locationLat,
		LocationLon:         u.locationLon,
		SelectedCity:        u.selectedCity,
		History:             u.history,
		Metrics:             u.metrics,
		LastCheckUTC:        lastCheckUTC,
		HasChecked:          u.hasChecked,
		HomeDetailsExpanded: u.homeDetailsExpanded,
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

func (u *UI) applyThemePalette() {
	switch u.effectiveThemeMode() {
	case "dark":
		u.theme.Palette = material.Palette{
			Bg:         color.NRGBA{A: 255, R: 22, G: 24, B: 28},
			Fg:         color.NRGBA{A: 255, R: 230, G: 233, B: 237},
			ContrastBg: color.NRGBA{A: 255, R: 62, G: 110, B: 195},
			ContrastFg: color.NRGBA{A: 255, R: 255, G: 255, B: 255},
		}
	default:
		u.theme.Palette = material.Palette{
			Bg:         color.NRGBA{A: 255, R: 248, G: 250, B: 252},
			Fg:         color.NRGBA{A: 255, R: 28, G: 33, B: 40},
			ContrastBg: color.NRGBA{A: 255, R: 56, G: 104, B: 188},
			ContrastFg: color.NRGBA{A: 255, R: 255, G: 255, B: 255},
		}
	}
}

func (u *UI) effectiveThemeMode() string {
	mode := strings.ToLower(strings.TrimSpace(u.cfg.ThemeMode))
	switch mode {
	case "dark", "light":
		return mode
	default:
		if u.systemThemeMode == "dark" {
			return "dark"
		}
		return "light"
	}
}

func (u *UI) mutedTextColor() color.NRGBA {
	if u.effectiveThemeMode() == "dark" {
		return color.NRGBA{A: 255, R: 165, G: 173, B: 182}
	}
	return color.NRGBA{A: 255, R: 97, G: 106, B: 117}
}

func (u *UI) subtleBorderColor() color.NRGBA {
	if u.effectiveThemeMode() == "dark" {
		return color.NRGBA{A: 255, R: 66, G: 72, B: 80}
	}
	return color.NRGBA{A: 255, R: 210, G: 218, B: 228}
}

func (u *UI) riskMeaningText() string {
	switch u.overallRisk {
	case domain.RiskLow:
		return u.tr("summary.low", "Conditions are stable.")
	case domain.RiskMedium:
		return u.tr("summary.medium", "Noticeable change detected.")
	case domain.RiskHigh:
		return u.tr("summary.high", "Strong change detected.")
	case domain.RiskCritical:
		return u.tr("summary.critical", "Rapid change detected.")
	default:
		return u.tr("summary.low", "Conditions are stable.")
	}
}

func (u *UI) pressureBandLabel() string {
	switch u.pressureRisk {
	case domain.RiskCritical:
		return u.tr("pressureBand.extreme", "Extreme")
	case domain.RiskHigh:
		return u.tr("pressureBand.veryHigh", "Very high")
	case domain.RiskMedium:
		return u.tr("pressureBand.high", "High")
	default:
		return u.tr("pressureBand.normal", "Normal")
	}
}

func (u *UI) kBandLabel() string {
	switch u.kIndexRisk {
	case domain.RiskCritical:
		return u.tr("kBand.storm", "Storm")
	case domain.RiskHigh:
		return u.tr("kBand.active", "Active")
	case domain.RiskMedium:
		return u.tr("kBand.unsettled", "Unsettled")
	default:
		return u.tr("kBand.quiet", "Quiet")
	}
}

func (u *UI) pressureWindowLabel() string {
	return u.tr("common.window_24h", "24h")
}

func (u *UI) valueWithUnit(value float64, unitName string) string {
	return fmt.Sprintf("%.1f\u00A0%s", value, unitName)
}

func (u *UI) nextCheckDisplay() string {
	if u.nextScheduledCheck.IsZero() {
		return u.tr("details.none", "None")
	}
	return u.formatTime(u.nextScheduledCheck)
}

func (u *UI) nextCheckDisplayShort() string {
	if u.nextScheduledCheck.IsZero() {
		return "-"
	}
	return u.formatShortTime(u.nextScheduledCheck)
}

func (u *UI) formatShortTime(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	if u.cfg.Units.TimeFormat == "12h" {
		return ts.Local().Format("03:04 PM")
	}
	return ts.Local().Format("15:04")
}

func (u *UI) primaryDriverText() string {
	if u.kIndexRisk > u.pressureRisk {
		return u.tr("details.driver_k", "Geomagnetic activity")
	}
	return u.tr("details.driver_pressure", "Pressure change")
}

func (u *UI) outOfRangeDetailsText() string {
	parts := make([]string, 0, 2)
	primaryPressure, _ := domain.ConvertPressureDelta(u.metrics.PressureDeltaHPa, u.cfg.Units.PressureUnit)
	if u.pressureRisk != domain.RiskLow {
		thresholdPrimary, _ := domain.ConvertPressureDelta(thresholdLabel(u.pressureRisk, u.cfg.Pressure), u.cfg.Units.PressureUnit)
		parts = append(parts, u.trf(
			"details.out_pressure",
			"Delta Pressure %s (threshold %s)",
			u.valueWithUnit(primaryPressure, u.cfg.Units.PressureUnit),
			u.valueWithUnit(thresholdPrimary, u.cfg.Units.PressureUnit),
		))
	}
	if u.kIndexRisk != domain.RiskLow {
		parts = append(parts, u.trf(
			"details.out_k",
			"K-index %d (threshold %.1f)",
			int(math.Round(u.metrics.KIndex)),
			thresholdLabelK(u.kIndexRisk, u.cfg.KIndex),
		))
	}
	if len(parts) == 0 {
		return u.tr("details.none", "None")
	}
	return strings.Join(parts, " | ")
}

func (u *UI) currentCityName() string {
	if u.selectedCity < 0 || u.selectedCity >= len(u.cities) {
		return u.tr("settings.custom_city", "custom")
	}
	return u.cities[u.selectedCity].Name
}

func blendColor(a, b color.NRGBA, t float32) color.NRGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	lerp := func(from, to uint8) uint8 {
		return uint8(float32(from) + (float32(to)-float32(from))*t)
	}
	return color.NRGBA{
		R: lerp(a.R, b.R),
		G: lerp(a.G, b.G),
		B: lerp(a.B, b.B),
		A: lerp(a.A, b.A),
	}
}

func detectSystemThemeMode() string {
	raw := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		os.Getenv("GTK_THEME"),
		os.Getenv("KDE_COLOR_SCHEME"),
		os.Getenv("QT_STYLE_OVERRIDE"),
		os.Getenv("SYSTEM_THEME"),
	}, " ")))
	if strings.Contains(raw, "dark") {
		return "dark"
	}
	return "light"
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
			return u.tr("settings.system_default", "System")
		}
		resolved := u.i18n.ResolveLanguage("system")
		return fmt.Sprintf("%s (%s)", u.tr("settings.system_default", "System"), strings.ToUpper(resolved))
	case "en":
		return u.tr("settings.lang_en", "English")
	case "de":
		return u.tr("settings.lang_de", "Deutsch")
	case "uk":
		return u.tr("settings.lang_uk", "Ukrainian")
	default:
		if code == "" {
			return u.tr("settings.system_default", "System")
		}
		return strings.ToUpper(code)
	}
}

func (u *UI) riskLabel(level domain.RiskLevel) string {
	switch level {
	case domain.RiskLow:
		return u.tr("risk.low", "Low")
	case domain.RiskMedium:
		return u.tr("risk.medium", "Medium")
	case domain.RiskHigh:
		return u.tr("risk.high", "High")
	case domain.RiskCritical:
		return u.tr("risk.critical", "Critical")
	default:
		return u.tr("risk.unknown", "Unknown")
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
