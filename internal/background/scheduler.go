package background

import "github.com/vitovt/wetterabhaengig/internal/domain"

func SyncConfig(cfg domain.AppConfig, lat, lon float64) error {
	return syncConfig(cfg, lat, lon)
}

func TriggerNow() error {
	return triggerNow()
}
