//go:build !android

package gps

import (
	"context"
	"errors"
)

type stubProvider struct{}

func newProvider() Provider {
	return &stubProvider{}
}

func (s *stubProvider) CurrentLocation(ctx context.Context) (float64, float64, error) {
	return 0, 0, errors.New("GPS location is available only on Android builds")
}
