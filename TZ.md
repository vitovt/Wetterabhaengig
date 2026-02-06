# Wetterabhaengig - Agreed Requirements

This file captures only the requirements that have been agreed so far.

## App Identity
- App name: Wetterabhaengig.
- Application ID: com.vitovt.wetterabhaengig.

## Goal
- Android app to track weather-related changes and show warnings.
- Primary output: a traffic light indicator (red/yellow/green) plus numeric details.
- All available numeric values should be shown under the traffic light.

## Tech Stack & Platform
- Language: Go.
- UI framework: Gio (gioui.org).
- Android minimum version: 11 (API 30).
- Target SDK must be Android 14 or newer.
- Code comments must be in English.
- README must be in English.

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
- Local notifications when the traffic light state changes (any direction).
- Notifications can be enabled/disabled (default: enabled).

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

## Traffic Light Mapping
- LOW => Green.
- MEDIUM => Yellow.
- HIGH => Red.
- CRITICAL => Red blinking.
- Show a short explanatory label under the traffic light.
- Also show which criterion is out of range.

## Repository Rules
- Initialize Git in this folder.
- Small commits only: one logical change, one file per commit when possible.
- The existing file `weather_check.py` should not be committed.

## Open Questions / TBD
- Exact content/wording of the explanatory label under the traffic light.
