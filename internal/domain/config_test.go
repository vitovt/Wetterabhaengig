package domain

import (
	"strings"
	"testing"
)

func TestValidateConfig_DefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("default config must be valid, got error: %v", err)
	}
}

func TestValidateConfig_InvalidCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(*AppConfig)
		wantErrPart string
	}{
		{
			name: "pressure thresholds must be positive",
			mutate: func(cfg *AppConfig) {
				cfg.Pressure.Medium = 0
			},
			wantErrPart: "pressure thresholds must be > 0",
		},
		{
			name: "pressure thresholds order",
			mutate: func(cfg *AppConfig) {
				cfg.Pressure.Medium = 10
				cfg.Pressure.High = 9
			},
			wantErrPart: "pressure thresholds must satisfy",
		},
		{
			name: "kindex thresholds nonnegative",
			mutate: func(cfg *AppConfig) {
				cfg.KIndex.Medium = -1
			},
			wantErrPart: "k-index thresholds must be >= 0",
		},
		{
			name: "kindex thresholds order",
			mutate: func(cfg *AppConfig) {
				cfg.KIndex.High = cfg.KIndex.Medium
			},
			wantErrPart: "k-index thresholds must satisfy",
		},
		{
			name: "schedule min minutes",
			mutate: func(cfg *AppConfig) {
				cfg.Schedule.MinMinutes = 0
			},
			wantErrPart: "minimum schedule period must be >= 1",
		},
		{
			name: "schedule period less than min",
			mutate: func(cfg *AppConfig) {
				cfg.Schedule.PeriodMinutes = 10
				cfg.Schedule.MinMinutes = 15
			},
			wantErrPart: "schedule period must be >=",
		},
		{
			name: "retention max years",
			mutate: func(cfg *AppConfig) {
				cfg.Retention.MaxYears = 0
			},
			wantErrPart: "retention max years must be >= 1",
		},
		{
			name: "retention days out of range",
			mutate: func(cfg *AppConfig) {
				cfg.Retention.DefaultDays = 0
			},
			wantErrPart: "retention days must be in",
		},
		{
			name: "pressure unit",
			mutate: func(cfg *AppConfig) {
				cfg.Units.PressureUnit = "psi"
			},
			wantErrPart: "unsupported pressure unit",
		},
		{
			name: "time format",
			mutate: func(cfg *AppConfig) {
				cfg.Units.TimeFormat = "iso"
			},
			wantErrPart: "unsupported time format",
		},
		{
			name: "theme mode",
			mutate: func(cfg *AppConfig) {
				cfg.ThemeMode = "amoled"
			},
			wantErrPart: "unsupported theme mode",
		},
		{
			name: "language required",
			mutate: func(cfg *AppConfig) {
				cfg.Language = ""
			},
			wantErrPart: "language must not be empty",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.mutate(&cfg)
			err := ValidateConfig(cfg)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Fatalf("unexpected error: %v; expected to contain: %q", err, tc.wantErrPart)
			}
		})
	}
}
