package config

import (
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	Name    string `toml:"name"`
	Verbose bool   `toml:"verbose"`
}

func TestStoreGetSwap(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}
	store := NewStore(defaults)

	// Verify Get returns defaults
	got := store.Get()
	if got.Name != "default" || got.Verbose != false {
		t.Errorf("Get() = %+v, want %+v", got, defaults)
	}

	// Swap with new value
	newVal := &testConfig{Name: "swapped", Verbose: true}
	store.Swap(newVal)

	// Verify Get returns new value
	got = store.Get()
	if got.Name != "swapped" || got.Verbose != true {
		t.Errorf("after Swap, Get() = %+v, want %+v", got, newVal)
	}
}

func TestStoreOnChange(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}
	store := NewStore(defaults)

	var oldVal, newVal *testConfig
	store.OnChange(func(old, new_ *testConfig) {
		oldVal = old
		newVal = new_
	})

	newCfg := &testConfig{Name: "changed", Verbose: true}
	store.Swap(newCfg)

	if oldVal == nil || oldVal.Name != "default" {
		t.Errorf("callback old = %+v, want Name=default", oldVal)
	}
	if newVal == nil || newVal.Name != "changed" || !newVal.Verbose {
		t.Errorf("callback new = %+v, want Name=changed Verbose=true", newVal)
	}
}

func TestStoreMultipleOnChange(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}
	store := NewStore(defaults)

	var order []int
	store.OnChange(func(old, new_ *testConfig) {
		order = append(order, 1)
	})
	store.OnChange(func(old, new_ *testConfig) {
		order = append(order, 2)
	})
	store.OnChange(func(old, new_ *testConfig) {
		order = append(order, 3)
	})

	store.Swap(&testConfig{Name: "new", Verbose: true})

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("callbacks fired in order %v, want [1 2 3]", order)
	}
}

func TestLoadTOML(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}

	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(path, []byte(`name = "test-app"
verbose = true`), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadTOML[testConfig](path, defaults)
		if err != nil {
			t.Fatalf("LoadTOML: %v", err)
		}
		if cfg.Name != "test-app" || !cfg.Verbose {
			t.Errorf("LoadTOML: got %+v, want Name=test-app Verbose=true", cfg)
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent.toml")

		cfg, err := LoadTOML[testConfig](path, defaults)
		if err != nil {
			t.Fatalf("LoadTOML with missing file: unexpected error %v", err)
		}
		if cfg.Name != "default" || cfg.Verbose != false {
			t.Errorf("LoadTOML with missing file: got %+v, want defaults", cfg)
		}
	})

	t.Run("invalid_toml", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "config-*.toml")
		if err != nil {
			t.Fatal(err)
		}
		path := f.Name()
		f.WriteString("name = [ invalid")
		f.Close()

		_, err = LoadTOML[testConfig](path, defaults)
		if err == nil {
			t.Error("LoadTOML with invalid TOML: expected error, got nil")
		}
	})
}

func TestWorkspaceBridge(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}
	store := NewStore(defaults)

	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.toml")
	if err := os.WriteFile(path, []byte(`name = "bridge-test"
verbose = true`), 0644); err != nil {
		t.Fatal(err)
	}

	bridge := NewWorkspaceBridge(store, path, defaults)

	if err := bridge.HandleChange(); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}

	got := store.Get()
	if got.Name != "bridge-test" || !got.Verbose {
		t.Errorf("store after HandleChange: got %+v, want Name=bridge-test Verbose=true", got)
	}
}

func TestWorkspaceBridge_MissingFile(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}
	store := NewStore(defaults)

	bridge := NewWorkspaceBridge(store, "/nonexistent/path/config.toml", defaults)
	if err := bridge.HandleChange(); err != nil {
		t.Fatalf("HandleChange with missing file should not error: %v", err)
	}

	got := store.Get()
	if got.Name != "default" {
		t.Errorf("expected defaults after missing file, got %+v", got)
	}
}

func TestStoreSwap_NoChange(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: false}
	store := NewStore(defaults)

	callCount := 0
	store.OnChange(func(old, new_ *testConfig) {
		callCount++
	})

	// Swap fires callback even when values are same (pointer comparison)
	store.Swap(&testConfig{Name: "default", Verbose: false})
	if callCount != 1 {
		t.Errorf("expected 1 callback, got %d", callCount)
	}
}

func TestLoadTOML_PartialOverride(t *testing.T) {
	defaults := &testConfig{Name: "default", Verbose: true}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`name = "partial"`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadTOML[testConfig](path, defaults)
	if err != nil {
		t.Fatalf("LoadTOML: %v", err)
	}
	if cfg.Name != "partial" {
		t.Errorf("Name = %q, want 'partial'", cfg.Name)
	}
}
