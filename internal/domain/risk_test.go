package domain

import "testing"

func TestAggregateRisk_PicksHighestLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		levels []RiskLevel
		want   RiskLevel
	}{
		{name: "empty defaults low", levels: nil, want: RiskLow},
		{name: "single low", levels: []RiskLevel{RiskLow}, want: RiskLow},
		{name: "medium present", levels: []RiskLevel{RiskLow, RiskMedium}, want: RiskMedium},
		{name: "high present", levels: []RiskLevel{RiskMedium, RiskHigh, RiskLow}, want: RiskHigh},
		{name: "critical present", levels: []RiskLevel{RiskLow, RiskCritical, RiskHigh}, want: RiskCritical},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := AggregateRisk(tc.levels...)
			if got != tc.want {
				t.Fatalf("AggregateRisk(%v) = %v, want %v", tc.levels, got, tc.want)
			}
		})
	}
}

func TestRiskFromPressureDelta_Boundaries(t *testing.T) {
	t.Parallel()

	thr := PressureThresholds{Medium: 5, High: 8, Crit: 12}
	tests := []struct {
		delta float64
		want  RiskLevel
	}{
		{delta: 5, want: RiskLow},
		{delta: 5.001, want: RiskMedium},
		{delta: 8, want: RiskMedium},
		{delta: 8.001, want: RiskHigh},
		{delta: 12, want: RiskHigh},
		{delta: 12.001, want: RiskCritical},
	}

	for _, tc := range tests {
		got := RiskFromPressureDelta(tc.delta, thr)
		if got != tc.want {
			t.Fatalf("RiskFromPressureDelta(%v) = %v, want %v", tc.delta, got, tc.want)
		}
	}
}

func TestRiskFromKIndex_Boundaries(t *testing.T) {
	t.Parallel()

	thr := KIndexThresholds{Medium: 4, High: 5, Crit: 6}
	tests := []struct {
		k    float64
		want RiskLevel
	}{
		{k: 3.9, want: RiskLow},
		{k: 4, want: RiskMedium},
		{k: 4.9, want: RiskMedium},
		{k: 5, want: RiskHigh},
		{k: 5.9, want: RiskHigh},
		{k: 6, want: RiskCritical},
	}

	for _, tc := range tests {
		got := RiskFromKIndex(tc.k, thr)
		if got != tc.want {
			t.Fatalf("RiskFromKIndex(%v) = %v, want %v", tc.k, got, tc.want)
		}
	}
}
