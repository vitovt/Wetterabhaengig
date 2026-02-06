# Wetterabhaengig

Wetterabhaengig is a Go + Gio weather risk monitoring app for Linux, Android, Windows, and macOS.

## Purpose

The app provides a quick traffic-light risk signal for a selected location and shows the numeric values that explain the current state.

## Current Development Stage

- App skeleton with responsive layout.
- Screens: Home, History, Settings, Test.
- Live check flow using Open-Meteo and NOAA endpoints.
- Default risk rules for pressure delta and planetary K-index.
- Coordinate editing plus editable EU city list selection.
- Test-notification actions on Settings and Test screens.
- Auto-generated placeholder icons, including Android `appicon.png`.

## Build

The project uses `Makefile` targets:

- `make help` (default)
- `make prepare`
- `make deps`
- `make linux`
- `make windows`
- `make mac`
- `make android`
- `make clean`

## Android Icon

Android builds expect `cmd/wetterabhaengig/appicon.png`.

## Notes

- Code comments are written in English.
- `weather_check.py` must not be committed.
- Local desktop notification delivery is best-effort (`notify-send` on Linux, `osascript` on macOS). Android and Windows backend wiring is still pending.
