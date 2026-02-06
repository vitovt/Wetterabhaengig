package gps

import "context"

type Provider interface {
	CurrentLocation(ctx context.Context) (float64, float64, error)
}

type AndroidViewBinder interface {
	SetAndroidView(view uintptr)
}

func NewProvider() Provider {
	return newProvider()
}
