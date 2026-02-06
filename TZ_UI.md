# Wetterabhaengig — Home Screen UI Technical Specification (Gio / Android)

Version: 1.0
Scope: Home screen (traffic-light status + summary) + Collapsible Details section + Localization strings (EN → UKR/DE)

## 1. Objectives

* Make the Home screen immediately understandable in 2–3 seconds.
* Present the current risk state clearly without relying only on color.
* Show the *two drivers* (pressure change + geomagnetic activity) with clean formatting (no “one-line mush”).
* Move technical/debug context (coordinates, internal “out of range”, scheduling) into a structured, collapsible Details section.
* Keep UI consistent across EN/DE/UKR.

## 2. Risk model (UI-facing)

### 2.1 Levels

* LOW
* MEDIUM
* HIGH
* CRITICAL

### 2.2 Visual mapping

* LOW: Green (static)
* MEDIUM: Amber/Yellow (static)
* HIGH: Red (static)
* CRITICAL: Red (pulse/blink) *(already implemented)*

### 2.3 Text mapping (must always be shown)

* LOW → `Low`
* MEDIUM → `Medium`
* HIGH → `High`
* CRITICAL → `Critical`

## 3. Home screen information architecture

### 3.1 Screen layout (top to bottom)

1. Top App Bar
2. Traffic Light Indicator (large)
3. Status Summary Card (primary text under the light)
4. Quick action: Check now
5. Collapsible Details section (smaller text, below everything)

### 3.2 Top App Bar

* Left: Menu / navigation icon (hamburger)
* Center: App name (short, stable).
* Remove debug-only elements like “Screen: Home” from release UI.

## 4. Traffic Light Indicator component

### 4.1 Purpose

* Primary visual state cue (green/amber/red + pulsing red for critical).
* Must be accompanied by text (accessibility and clarity).

### 4.2 Behavior

* After refresh (Check now or scheduled) color should be animated smoothly (fade/scale) from gray making visual feedback that data was updated
* CRITICAL animation:
  * Prefer a **slow pulse** (~1–1.5s cycle) over fast blinking.

## 5. Status Summary Card (the “explanatory label” under the light)

### 5.1 Component goals

* Answer, in order:

  * What is the overall risk?
  * What does it mean in plain language?
  * What are the two drivers (numbers + human labels), each on its own line.

### 5.2 Exact final copy and line structure (EN)

The summary must render as **3–4 lines** with forced line breaks (no joining metrics with `•` on Home).

**Line 1 (primary, largest):**

* `Overall risk: {Low|Medium|High|Critical}`

**Line 2 (secondary, short interpretation):**

* LOW: `Conditions are stable.`
* MEDIUM: `Noticeable change detected.`
* HIGH: `Strong change detected.`
* CRITICAL: `Rapid change detected.`

**Line 3 (metric 1, dedicated line):**

* SmallHeader: `Atmosphere pressure`
* `ΔPressure ({window}): {pressureDelta} {pressureUnit} — {pressureBand}`

**Line 4 (metric 2, dedicated line):**

* SmallHeader: `Solar activity`
* `K-index: {kIndex} — {kBand}`

### 5.3 Examples (EN)

LOW:


* Overall risk: Low
* Conditions are stable.
Atmosphere pressure:
* ΔPressure (24h): 2.5 hPa — Normal
Solar activity:
* K-index: 1 — Quiet

MEDIUM:

* Overall risk: Medium
* Noticeable change detected.
Atmosphere pressure:
* ΔPressure (24h): 7.6 hPa — High
Solar activity:
* K-index: 1 — Quiet

HIGH:

* Overall risk: High
* Strong change detected.
Atmosphere pressure:
* ΔPressure (24h): 10.2 hPa — Very high
Solar activity:
* K-index: 3 — Unsettled

CRITICAL:

* Overall risk: Critical
* Rapid change detected.
Atmosphere pressure:
* ΔPressure (24h): 14.5 hPa — Extreme
Solar activity:
* K-index: 6 — Storm

### 5.4 Formatting rules (anti “one-line mush”)

* Each metric must be a separate line on Home.
* Never concatenate metrics with separators (`•`) on Home.
* Prevent splitting numeric + unit across lines:

  * Use non-breaking space between number and unit (e.g., `14.5 hPa`), or
  * Use a layout that keeps value+unit together.
* Keep parentheses minimal on Home; use Details for multi-unit expansions.

### 5.5 Typography and spacing (guidelines)

* Line 1: largest in the card (headline style).
* Line 2: body text, slightly muted.
* Lines 3–4: body-small or body with muted labels; values remain readable.
* Vertical spacing between lines: ~8–12dp.
* Card padding: ~16dp.
* Do not align this text under the circle’s left edge; make the card full-width with consistent margins.

## 6. State change messaging (Home)

* Do not show a permanent debug line like `Notification #6 sent: State changed ...` in the main area. Only in the test section.
* Recommended behavior:

  * On state change: show a brief toast/snackbar: `Risk changed: Medium → High`
  * Log full history in a “Notifications log” screen or inside Details (optional).

## 7. Collapsible Details section (below everything)

### 7.1 Goals

* Provide transparency and exact numbers without cluttering the Home summary.
* Replace debug text with structured, human-readable rows.
* Collapsed/expanded state must persist.

### 7.2 Placement & style

* Located below all main content.
* Text smaller than Status Summary (one step down).
* Compact spacing, high readability.
* Container: subtle card/divider separation.

### 7.3 Collapsible header row (always visible)

**Header row content:**

* Left: `Details`
* Optional right-side snippet (recommended): `Last check: {timeShort}` or `{relativeTime}`
* Right: chevron `▾` (collapsed) / `▴` (expanded)

**Interaction:**

* Tap anywhere on header toggles collapse/expand.
* Small, subtle animation for chevron/height.

### 7.4 Collapsed preview line (recommended)

When collapsed, show a single compact preview line under the header (still inside the Details container):

* `ΔPressure: {pressureDelta} {unit} • K-index: {kIndex} • Next: {timeShort}`
  This preview is allowed to use `•` because it is secondary and small.

### 7.5 Expanded content blocks (grouped)

When expanded, show groups in this order:

#### A) Checks & schedule

Rows:

* `Last check` → `{timestampLocal}`
* `Next scheduled check` → `{timestampLocal}`
* `Check interval` → `{intervalHuman}` (e.g., `60 min`)
  Optional rows (only if user-configurable/exposed):
* `Time format` → `24h`
* `Notifications` → `On (state changes)` / `Off`

#### B) Trigger / explanation (replace “Out of range” debug output)

Purpose: explain why the current state is what it is.

Rows:

* `Primary driver` → `Pressure change` or `Geomagnetic activity` (choose the highest contributing driver)
* `Drivers` → `Pressure: {level} • K-index: {level}`
* `Out of range`:

  * If none: `None`
  * If present: `ΔPressure {value} {unit} (threshold {thresholdValue} {unit})` and/or `K-index {k} (threshold {kThreshold})`
* `Thresholds` (optional but useful): show the configured cutoffs in a readable form:

  * `ΔPressure thresholds` → `Medium ≥ 5.0 hPa, High ≥ 9.0 hPa, Critical ≥ 12.0 hPa` *(example; match your config)*
  * `K-index thresholds` → `...` *(if used)*

Emoji usage (optional, keep minimal):

* Group title may be `🎯 Trigger` or plain `Trigger`.

#### C) Measurements

Rows:

* `ΔPressure ({window})` → `{pressureDelta} {unit}`
* `K-index` → `{kIndex} ({kBand})`
  Optional multi-units (Details only):
* For pressure: `({mmHg} mmHg • {inHg} inHg)` after the primary value.
  Optional:
* `Data source` → provider name if stable and user-relevant.

#### D) Location

Rows:

* Prefer place name if available:

  * `Location` → `{placeName}`
  * `Coordinates` → `{lat}, {lon}`
    If no place name:
* `Coordinates` → `{lat}, {lon}`

Emoji optional:

* `📍 Location` as group title.

#### E) Shortcuts (optional, if these screens exist)

Rows (tapable):

* `📈 View history chart`
* `🧾 View notifications log`
* `⚙️ Configure thresholds`

### 7.6 Key–value row layout rules

* Left column: label (muted).
* Right column: value (normal).
* Row padding: ~8–10dp vertical.
* Labels preferably single-line; values may wrap.
* Use consistent number formatting across rows.

### 7.7 Persistence (collapsed/expanded remembers)

* Persist a boolean flag, e.g., `pref.home.detailsExpanded`.
* Restore on Home screen init and app restart.
* Per-screen persistence is sufficient (Home only).

## 8. Data formatting rules (global)

* Pressure delta:

* K-index: integer.
* Window formatting: `24h` or `{n}h` exactly, consistent everywhere.
* Use locale-aware decimal separator if desired, but keep stable across the app (most apps keep dot in scientific-style values; choose one and be consistent).
* Always show a human band label for both signals:

  * Pressure band: `Normal / High / Very high / Extreme` (or your final set)
  * K band: `Quiet / Unsettled / Active / Storm`

## 9. Accessibility & UX requirements

* Never rely only on color: always show the textual level in Line 1.
* Ensure sufficient contrast for text over light background.
* Support dark/licht mode. Add to settings: Dark/Light/System Default
* CRITICAL animation must not be rapid flashing.
* Touch targets:

  * Details header row tap area ≥ 44dp height.
  * Shortcut rows ≥ 44dp height.

---

# 10. Localization: Strings (EN → Ukrainian, Deutsch)

Use these as localization keys and translations. Placeholders use `{...}`.

## 10.1 Core labels

| Key              | English (en)          | Ukrainian (uk)           | Deutsch (de)          |
| ---------------- | --------------------- | ------------------------ | --------------------- |
| home.overallRisk | Overall risk: {level} | Загальний ризик: {level} | Gesamtrisiko: {level} |
| risk.low         | Low                   | Низький                  | Niedrig               |
| risk.medium      | Medium                | Середній                 | Mittel                |
| risk.high        | High                  | Високий                  | Hoch                  |
| risk.critical    | Critical              | Критичний                | Kritisch              |

## 10.2 Summary interpretation sentences

| Key              | English (en)                | Ukrainian (uk)          | Deutsch (de)                              |
| ---------------- | --------------------------- | ----------------------- | ----------------------------------------- |
| summary.low      | Conditions are stable.      | Умови стабільні.        | Die Bedingungen sind stabil.              |
| summary.medium   | Noticeable change detected. | Виявлено помітні зміни. | Eine spürbare Veränderung wurde erkannt.  |
| summary.high     | Strong change detected.     | Виявлено значні зміни.  | Eine deutliche Veränderung wurde erkannt. |
| summary.critical | Rapid change detected.      | Виявлено різкі зміни.   | Eine schnelle Veränderung wurde erkannt.  |

## 10.3 Metric lines (Home)

| Key                  | English (en)                                  | Ukrainian (uk)                                  | Deutsch (de)                                      |
| -------------------- | --------------------------------------------- | ----------------------------------------------- | ------------------------------------------------- |
| metric.pressureDelta | ΔPressure ({window}): {value} {unit} — {band} | Зміна тиску ({window}): {value} {unit} — {band} | Druckänderung ({window}): {value} {unit} — {band} |
| metric.kIndex        | K-index: {value} — {band}                     | K-індекс: {value} — {band}                      | K-Index: {value} — {band}                         |

## 10.4 Band labels (Pressure)

| Key                   | English (en) | Ukrainian (uk)     | Deutsch (de) |
| --------------------- | ------------ | ------------------ | ------------ |
| pressureBand.normal   | Normal       | Норма              | Normal       |
| pressureBand.high     | High         | Високо             | Hoch         |
| pressureBand.veryHigh | Very high    | Дуже високо        | Sehr hoch    |
| pressureBand.extreme  | Extreme      | Надзвичайно високо | Extrem       |

## 10.5 Band labels (K-index)

| Key             | English (en) | Ukrainian (uk) | Deutsch (de) |
| --------------- | ------------ | -------------- | ------------ |
| kBand.quiet     | Quiet        | Спокійно       | Ruhig        |
| kBand.unsettled | Unsettled    | Нестійко       | Unbeständig  |
| kBand.active    | Active       | Активно        | Aktiv        |
| kBand.storm     | Storm        | Шторм          | Sturm        |

## 10.6 Details section (header + groups)

| Key                | English (en)                                        | Ukrainian (uk)                                         | Deutsch (de)                                               |
| ------------------ | --------------------------------------------------- | ------------------------------------------------------ | ---------------------------------------------------------- |
| details.title      | Details                                             | Деталі                                                 | Details                                                    |
| details.preview    | ΔPressure: {p} {unit} • K-index: {k} • Next: {time} | Зміна тиску: {p} {unit} • K-індекс: {k} • Далі: {time} | Druckänderung: {p} {unit} • K-Index: {k} • Nächste: {time} |
| group.checks       | Checks & schedule                                   | Перевірки й розклад                                    | Prüfungen & Zeitplan                                       |
| group.trigger      | Trigger                                             | Причина                                                | Auslöser                                                   |
| group.measurements | Measurements                                        | Вимірювання                                            | Messwerte                                                  |
| group.location     | Location                                            | Локація                                                | Standort                                                   |
| group.shortcuts    | Shortcuts                                           | Швидкі дії                                             | Schnellzugriffe                                            |

## 10.7 Details rows (labels)

| Key                        | English (en)         | Ukrainian (uk)     | Deutsch (de)                |
| -------------------------- | -------------------- | ------------------ | --------------------------- |
| details.lastCheck          | Last check           | Остання перевірка  | Letzte Prüfung              |
| details.nextCheck          | Next scheduled check | Наступна перевірка | Nächste geplante Prüfung    |
| details.interval           | Check interval       | Інтервал перевірки | Prüfintervall               |
| details.timeFormat         | Time format          | Формат часу        | Zeitformat                  |
| details.notifications      | Notifications        | Сповіщення         | Benachrichtigungen          |
| details.primaryDriver      | Primary driver       | Основний чинник    | Hauptfaktor                 |
| details.drivers            | Drivers              | Чинники            | Faktoren                    |
| details.outOfRange         | Out of range         | Поза межами        | Außerhalb des Bereichs      |
| details.none               | None                 | Немає              | Keine                       |
| details.thresholds         | Thresholds           | Пороги             | Schwellenwerte              |
| details.pressureThresholds | ΔPressure thresholds | Пороги зміни тиску | Schwellen für Druckänderung |
| details.kThresholds        | K-index thresholds   | Пороги K-індексу   | Schwellen für K-Index       |
| details.dataSource         | Data source          | Джерело даних      | Datenquelle                 |
| details.coordinates        | Coordinates          | Координати         | Koordinaten                 |

## 10.8 Shortcuts (optional)

| Key                       | English (en)              | Ukrainian (uk)                  | Deutsch (de)                          |
| ------------------------- | ------------------------- | ------------------------------- | ------------------------------------- |
| shortcut.history          | 📈 View history chart     | 📈 Переглянути графік історії   | 📈 Verlaufdiagramm ansehen            |
| shortcut.notificationsLog | 🧾 View notifications log | 🧾 Переглянути журнал сповіщень | 🧾 Benachrichtigungsprotokoll ansehen |
| shortcut.settings         | ⚙️ Configure thresholds   | ⚙️ Налаштувати пороги           | ⚙️ Schwellenwerte einstellen          |

