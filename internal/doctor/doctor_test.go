package doctor_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/doctor"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

func TestDoctorCleanVault(t *testing.T) {
	tmp := t.TempDir()
	vDir := filepath.Join(tmp, "vault")
	if err := vault.Init(vDir); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(tmp, "config.toml")
	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{Path: vDir, Kind: "personal", DefaultWrite: "direct"}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	checks := doctor.Run(context.Background(), cfgPath, filepath.Join(tmp, "index.db"))
	for _, c := range checks {
		if c.Status == "fail" {
			t.Fatalf("unexpected failure in doctor: %+v", c)
		}
	}
}
