# Wetterabhaengig - Agreed Requirements

This file captures only the requirements that have been agreed so far.

## App Identity
- App name: Wetterabhaengig.
- Application ID: com.vitovt.wetterabhaengig.

## Goal
- Cross-platform app to track weather-related changes and show warnings.
- Purpose: provide a quick, understandable risk signal for the selected location, plus the numeric context behind it.
- Primary output: a traffic light indicator (red/yellow/green) plus numeric details.
- All available numeric values should be shown under the traffic light.

## Tech Stack & Platform
- Language: Go.
- UI framework: Gio (gioui.org).
- Supported platforms: Linux, Android, Windows, macOS.
- Android minimum version: 11 (API 30).
- Target SDK must be Android 14 or newer.
- Code comments must be in English.
- README must be in English.

## Build & Tooling
- Create a `Makefile` that handles preparation, dependencies, and builds.
- Required targets:
  - `make help` (default behavior) to show available commands.
  - `make linux`
  - `make android`
  - `make windows`
  - `make mac`

## Data Sources (from the Python prototype)
- Open-Meteo API: hourly surface pressure, forecast_days=2, timezone=auto.
  - Use the first 24 hours of pressure values.
  - Compute delta = max - min.
  - Default risk thresholds (editable in settings):
    - delta > 12 hPa => CRITICAL
    - delta > 8 hPa => HIGH
    - delta > 5 hPa => MEDIUM
    - else => LOW
- NOAA planetary K-index 1m JSON:
  - Use the latest value.
  - Default risk thresholds (editable in settings):
    - K >= 6 => CRITICAL
    - K >= 5 => HIGH
    - K >= 4 => MEDIUM
    - else => LOW
- Overall risk aggregation:
  - If any source is CRITICAL => CRITICAL
  - Else if any HIGH => HIGH
  - Else if any MEDIUM => MEDIUM
  - Else LOW

## Location Input
- Manual coordinate entry.
- GPS (only during active session, enabled from settings).
- City list selection (20-30 EU cities; easily editable list).
- Selecting a city should populate the coordinate fields.
- Single active location profile. Changing coordinates affects all future checks.

## Updates & Notifications
- Update on app start.
- Manual "Check now" button.
- Scheduled checks with configurable period:
  - Default: 60 minutes.
  - Minimum: 15 minutes (lower is not allowed).
- Optional Android background checks when app window is closed:
  - Available only as best-effort mode.
  - Controlled by settings switch.
  - Dedicated OS-level Android background scheduler is not part of `main`.
  - Disabled by default.
- Local notifications when the traffic light state changes (any direction).
- Notifications can be enabled/disabled (default: enabled).
- Notification backends are implemented for Android, Linux, macOS, and Windows.
- Settings page does not include `Test notification` (moved out for cleaner settings UX).
- Test page includes a `Test notification` action.
- `Test notification` must immediately show a local notification using current notification data.

## History & Storage
- Store a record for every check (based on the configured schedule).
- Default retention: one calendar month.
- Retention is configurable up to a maximum of 50 years.

## Charts
- History view with two lines on a single chart:
  - Pressure line.
  - Magnetic K-index line.
- Lines use different default colors.
- Two independent Y axes (not normalized).
- Segments are highlighted using the current alert thresholds (LOW/MEDIUM/HIGH/CRITICAL).
  - If thresholds change in settings, chart highlighting updates accordingly.

## Units
- Multiple unit systems, including US.
- Pressure units: hPa, mmHg, inHg.
- Time format: 24h or 12h.
- K-index remains unitless.

## Localization
- Default language: English.
- Also include German and Ukrainian from the start.
- Language setting: system, English, German, Ukrainian.
- App should automatically include other languages when translation files exist.
- Translation format: TOML.

## Layout & Responsiveness
- Use one consistent layout concept across all platforms.
- Layout should adapt to screen size and window size while keeping visual consistency.
- In vertical orientation or narrow resized windows, switch to a compact layout variant.

## Traffic Light Mapping
- LOW => Green.
- MEDIUM => Yellow.
- HIGH => Red.
- CRITICAL => Red blinking.
- Show a short explanatory label under the traffic light.
- Also show which criterion is out of range.

## Repository Rules
- Use Git during the full development process.
- Commit in small logical blocks for every change.
- One file per commit when possible.
- The existing file `weather_check.py` should not be committed.

## Open Questions / TBD
- Exact content/wording of the explanatory label under the traffic light.
