package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Validatable is an optional interface that config structs can implement
// to validate themselves before being swapped in.
type Validatable interface {
	Validate() error
}

// LoadTOML loads a TOML config file into a struct of type T.
// If the file does not exist, it returns the provided defaults.
func LoadTOML[T any](path string, defaults *T) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg := new(T)
	if defaults != nil {
		*cfg = *defaults
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if v, ok := any(cfg).(Validatable); ok {
		if err := v.Validate(); err != nil {
			return nil, fmt.Errorf("validating config %s: %w", path, err)
		}
	}

	return cfg, nil
}
