package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vitovt/wetterabhaengig/internal/data"
	"github.com/vitovt/wetterabhaengig/internal/domain"
)

type Result struct {
	CheckedAt         time.Time
	PressureDeltaHPa  float64
	KIndex            float64
	PressureRisk      domain.RiskLevel
	KIndexRisk        domain.RiskLevel
	OverallRisk       domain.RiskLevel
	UsedFallbackK     bool
	UsedFallbackPress bool
}

type Checker struct {
	client *data.Client
}

func NewChecker(client *data.Client) *Checker {
	return &Checker{client: client}
}

func (c *Checker) Evaluate(
	ctx context.Context,
	cfg domain.AppConfig,
	lat,
	lon float64,
	prev domain.Metrics,
) (Result, error) {
	type pressureOut struct {
		value float64
		err   error
	}
	type kOut struct {
		value float64
		err   error
	}

	pressCh := make(chan pressureOut, 1)
	kCh := make(chan kOut, 1)

	go func() {
		value, err := c.client.FetchPressureDelta(ctx, lat, lon)
		pressCh <- pressureOut{value: value, err: err}
	}()
	go func() {
		value, err := c.client.FetchLatestKIndex(ctx)
		kCh <- kOut{value: value, err: err}
	}()

	pressureResult := <-pressCh
	kResult := <-kCh

	result := Result{
		CheckedAt: time.Now(),
	}
	var errs []error

	if pressureResult.err != nil {
		result.PressureDeltaHPa = prev.PressureDeltaHPa
		result.UsedFallbackPress = true
		errs = append(errs, fmt.Errorf("pressure fetch failed: %w", pressureResult.err))
	} else {
		result.PressureDeltaHPa = pressureResult.value
	}

	if kResult.err != nil {
		result.KIndex = prev.KIndex
		result.UsedFallbackK = true
		errs = append(errs, fmt.Errorf("k-index fetch failed: %w", kResult.err))
	} else {
		result.KIndex = kResult.value
	}

	result.PressureRisk = domain.RiskFromPressureDelta(result.PressureDeltaHPa, cfg.Pressure)
	result.KIndexRisk = domain.RiskFromKIndex(result.KIndex, cfg.KIndex)
	result.OverallRisk = domain.AggregateRisk(result.PressureRisk, result.KIndexRisk)

	if len(errs) > 0 {
		return result, errors.Join(errs...)
	}
	return result, nil
}
