//go:build !android

package background

import "github.com/vitovt/wetterabhaengig/internal/domain"

func syncConfig(cfg domain.AppConfig, lat, lon float64) error {
	_ = cfg
	_ = lat
	_ = lon
	return nil
}

func triggerNow() error {
	return nil
}
