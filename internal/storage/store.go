package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vitovt/wetterabhaengig/internal/domain"
	"github.com/vitovt/wetterabhaengig/internal/service"
)

type State struct {
	Config       domain.AppConfig `json:"config"`
	LocationLat  float64          `json:"location_lat"`
	LocationLon  float64          `json:"location_lon"`
	SelectedCity int              `json:"selected_city"`
	History      []service.Result `json:"history"`
	Metrics      domain.Metrics   `json:"metrics"`
	LastCheckUTC int64            `json:"last_check_utc"`
	HasChecked   bool             `json:"has_checked"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store {
	return &Store{path: path}
}

func DefaultPath(appName string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, appName, "state.json"), nil
}

func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, os.ErrNotExist
		}
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	return state, nil
}

func (s *Store) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}
