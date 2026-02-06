package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vitovt/wetterabhaengig/internal/data"
	"github.com/vitovt/wetterabhaengig/internal/domain"
)

type checkerRoundTrip func(req *http.Request) (*http.Response, error)

func (f checkerRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func checkerHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newCheckerForTest(rt checkerRoundTrip) *Checker {
	client := data.NewClientWithHTTPClient(&http.Client{
		Timeout:   2 * time.Second,
		Transport: rt,
	})
	return NewChecker(client)
}

func TestCheckerEvaluate_UsesFallbackForFailedPressureFetch(t *testing.T) {
	t.Parallel()

	checker := newCheckerForTest(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.open-meteo.com":
			return checkerHTTPResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
		case "services.swpc.noaa.gov":
			return checkerHTTPResponse(http.StatusOK, `[{"k_index":6}]`), nil
		default:
			return nil, fmt.Errorf("unexpected host: %s", req.URL.Host)
		}
	})

	prev := domain.Metrics{
		PressureDeltaHPa: 3.2,
		KIndex:           1.0,
	}
	result, err := checker.Evaluate(context.Background(), domain.DefaultConfig(), 48.1, 11.6, prev)
	if err == nil {
		t.Fatalf("expected combined error with fallback, got nil")
	}
	if !strings.Contains(err.Error(), "pressure fetch failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.UsedFallbackPress {
		t.Fatalf("expected pressure fallback flag")
	}
	if result.UsedFallbackK {
		t.Fatalf("did not expect k-index fallback")
	}
	if result.PressureDeltaHPa != prev.PressureDeltaHPa {
		t.Fatalf("pressure fallback value=%v, want %v", result.PressureDeltaHPa, prev.PressureDeltaHPa)
	}
	if result.KIndex != 6 {
		t.Fatalf("k-index value=%v, want 6", result.KIndex)
	}
	if result.OverallRisk != domain.RiskCritical {
		t.Fatalf("overall risk=%v, want %v", result.OverallRisk, domain.RiskCritical)
	}
}

func TestCheckerEvaluate_UsesFallbackForBothSources(t *testing.T) {
	t.Parallel()

	checker := newCheckerForTest(func(req *http.Request) (*http.Response, error) {
		return checkerHTTPResponse(http.StatusServiceUnavailable, `unavailable`), nil
	})

	prev := domain.Metrics{
		PressureDeltaHPa: 9,
		KIndex:           5,
	}
	result, err := checker.Evaluate(context.Background(), domain.DefaultConfig(), 48.1, 11.6, prev)
	if err == nil {
		t.Fatalf("expected joined fallback error, got nil")
	}
	if !result.UsedFallbackPress || !result.UsedFallbackK {
		t.Fatalf("expected both fallback flags, got press=%v k=%v", result.UsedFallbackPress, result.UsedFallbackK)
	}
	if result.PressureDeltaHPa != prev.PressureDeltaHPa {
		t.Fatalf("pressure fallback value=%v, want %v", result.PressureDeltaHPa, prev.PressureDeltaHPa)
	}
	if result.KIndex != prev.KIndex {
		t.Fatalf("k-index fallback value=%v, want %v", result.KIndex, prev.KIndex)
	}
}
