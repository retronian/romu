package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/retronian/romu/internal/db"
)

func TestScan(t *testing.T) {
	// Create temp ROM directory structure
	tmp := t.TempDir()
	fcDir := filepath.Join(tmp, "fc")
	os.MkdirAll(fcDir, 0755)
	os.WriteFile(filepath.Join(fcDir, "test.nes"), []byte("fake NES ROM data"), 0644)

	gbDir := filepath.Join(tmp, "gb")
	os.MkdirAll(gbDir, 0755)
	os.WriteFile(filepath.Join(gbDir, "test.gb"), []byte("fake GB ROM data"), 0644)

	// Skip file with wrong extension
	os.WriteFile(filepath.Join(fcDir, "readme.txt"), []byte("not a rom"), 0644)

	// Override DB path for testing
	os.Setenv("HOME", tmp)
	database, err := db.Open()
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer database.Close()

	result, err := Scan(tmp, database)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if result.Added != 2 {
		t.Errorf("expected 2 added, got %d", result.Added)
	}

	files, err := database.ListRomFiles()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files in db, got %d", len(files))
	}
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		root, path string
		want       string
	}{
		{"/roms", "/roms/fc/game.nes", "FC"},
		{"/roms", "/roms/sfc/game.sfc", "SFC"},
		{"/roms", "/roms/gba/game.gba", "GBA"},
		{"/roms", "/roms/unknown/game.bin", ""},
	}
	for _, tt := range tests {
		got := detectPlatform(tt.root, tt.path)
		if got != tt.want {
			t.Errorf("detectPlatform(%q, %q) = %q, want %q", tt.root, tt.path, got, tt.want)
		}
	}
}
