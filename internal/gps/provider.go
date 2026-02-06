package gps

import "context"

type Provider interface {
	CurrentLocation(ctx context.Context) (float64, float64, error)
}

func NewProvider() Provider {
	return newProvider()
}
