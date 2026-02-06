package domain

import "fmt"

type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "LOW"
	case RiskMedium:
		return "MEDIUM"
	case RiskHigh:
		return "HIGH"
	case RiskCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

type PressureThresholds struct {
	Medium float64
	High   float64
	Crit   float64
}

type KIndexThresholds struct {
	Medium float64
	High   float64
	Crit   float64
}

type ScheduleSettings struct {
	PeriodMinutes int
	MinMinutes    int
}

type RetentionSettings struct {
	DefaultDays int
	MaxYears    int
}

type NotificationSettings struct {
	Enabled bool
}

type UnitSettings struct {
	PressureUnit string
	TimeFormat   string
}

type AppConfig struct {
	Pressure      PressureThresholds
	KIndex        KIndexThresholds
	Schedule      ScheduleSettings
	Retention     RetentionSettings
	Notifications NotificationSettings
	Units         UnitSettings
	Languages     []string
	Language      string
}

type Metrics struct {
	PressureDeltaHPa float64
	KIndex           float64
}

func DefaultConfig() AppConfig {
	return AppConfig{
		Pressure: PressureThresholds{
			Medium: 5,
			High:   8,
			Crit:   12,
		},
		KIndex: KIndexThresholds{
			Medium: 4,
			High:   5,
			Crit:   6,
		},
		Schedule: ScheduleSettings{
			PeriodMinutes: 60,
			MinMinutes:    15,
		},
		Retention: RetentionSettings{
			DefaultDays: 30,
			MaxYears:    50,
		},
		Notifications: NotificationSettings{
			Enabled: true,
		},
		Units: UnitSettings{
			PressureUnit: "hPa",
			TimeFormat:   "24h",
		},
		Languages: []string{"system", "en", "de", "uk"},
		Language:  "system",
	}
}

func RiskFromPressureDelta(delta float64, t PressureThresholds) RiskLevel {
	switch {
	case delta > t.Crit:
		return RiskCritical
	case delta > t.High:
		return RiskHigh
	case delta > t.Medium:
		return RiskMedium
	default:
		return RiskLow
	}
}

func RiskFromKIndex(k float64, t KIndexThresholds) RiskLevel {
	switch {
	case k >= t.Crit:
		return RiskCritical
	case k >= t.High:
		return RiskHigh
	case k >= t.Medium:
		return RiskMedium
	default:
		return RiskLow
	}
}

func AggregateRisk(levels ...RiskLevel) RiskLevel {
	result := RiskLow
	for _, level := range levels {
		if level > result {
			result = level
		}
	}
	return result
}

func PressureDeltaMMHg(hPa float64) float64 {
	return hPa * 0.750061683
}

func PressureDeltaInHg(hPa float64) float64 {
	return hPa * 0.0295299831
}

func ConvertPressureDelta(deltaHPa float64, unitName string) (float64, error) {
	switch unitName {
	case "hPa":
		return deltaHPa, nil
	case "mmHg":
		return PressureDeltaMMHg(deltaHPa), nil
	case "inHg":
		return PressureDeltaInHg(deltaHPa), nil
	default:
		return 0, fmt.Errorf("unsupported pressure unit: %s", unitName)
	}
}

func ValidateConfig(cfg AppConfig) error {
	if cfg.Pressure.Medium <= 0 || cfg.Pressure.High <= 0 || cfg.Pressure.Crit <= 0 {
		return fmt.Errorf("pressure thresholds must be > 0")
	}
	if !(cfg.Pressure.Medium < cfg.Pressure.High && cfg.Pressure.High < cfg.Pressure.Crit) {
		return fmt.Errorf("pressure thresholds must satisfy medium < high < critical")
	}
	if cfg.KIndex.Medium < 0 || cfg.KIndex.High < 0 || cfg.KIndex.Crit < 0 {
		return fmt.Errorf("k-index thresholds must be >= 0")
	}
	if !(cfg.KIndex.Medium < cfg.KIndex.High && cfg.KIndex.High < cfg.KIndex.Crit) {
		return fmt.Errorf("k-index thresholds must satisfy medium < high < critical")
	}
	if cfg.Schedule.MinMinutes < 1 {
		return fmt.Errorf("minimum schedule period must be >= 1")
	}
	if cfg.Schedule.PeriodMinutes < cfg.Schedule.MinMinutes {
		return fmt.Errorf("schedule period must be >= %d", cfg.Schedule.MinMinutes)
	}
	if cfg.Retention.MaxYears < 1 {
		return fmt.Errorf("retention max years must be >= 1")
	}
	maxDays := cfg.Retention.MaxYears * 365
	if cfg.Retention.DefaultDays < 1 || cfg.Retention.DefaultDays > maxDays {
		return fmt.Errorf("retention days must be in [1, %d]", maxDays)
	}
	switch cfg.Units.PressureUnit {
	case "hPa", "mmHg", "inHg":
	default:
		return fmt.Errorf("unsupported pressure unit: %s", cfg.Units.PressureUnit)
	}
	switch cfg.Units.TimeFormat {
	case "24h", "12h":
	default:
		return fmt.Errorf("unsupported time format: %s", cfg.Units.TimeFormat)
	}
	if cfg.Language == "" {
		return fmt.Errorf("language must not be empty")
	}
	return nil
}
