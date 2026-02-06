package domain

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
