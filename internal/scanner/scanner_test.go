package scanner

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/retronian/romu/internal/db"
)

func TestScan(t *testing.T) {
	tmp := t.TempDir()
	fcDir := filepath.Join(tmp, "fc")
	os.MkdirAll(fcDir, 0755)
	os.WriteFile(filepath.Join(fcDir, "test.nes"), []byte("fake NES ROM data"), 0644)

	gbDir := filepath.Join(tmp, "gb")
	os.MkdirAll(gbDir, 0755)
	os.WriteFile(filepath.Join(gbDir, "test.gb"), []byte("fake GB ROM data"), 0644)

	// Skip file with wrong extension
	os.WriteFile(filepath.Join(fcDir, "readme.txt"), []byte("not a rom"), 0644)

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

func TestScanZipContainingRom(t *testing.T) {
	tmp := t.TempDir()
	fcDir := filepath.Join(tmp, "fc")
	os.MkdirAll(fcDir, 0755)

	// Create a zip containing a .nes file
	zipPath := filepath.Join(fcDir, "game.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	fw, _ := zw.Create("game.nes")
	fw.Write([]byte("fake NES ROM in ZIP"))
	zw.Close()
	zf.Close()

	os.Setenv("HOME", tmp)
	database, _ := db.Open()
	defer database.Close()

	result, err := Scan(tmp, database)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
}

func TestScanZipIsRom(t *testing.T) {
	tmp := t.TempDir()
	neogeoDir := filepath.Join(tmp, "neogeo")
	os.MkdirAll(neogeoDir, 0755)

	// Create a zip file that IS the ROM
	zipPath := filepath.Join(neogeoDir, "kof98.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	fw, _ := zw.Create("rom.bin")
	fw.Write([]byte("neogeo rom data"))
	zw.Close()
	zf.Close()

	os.Setenv("HOME", tmp)
	database, _ := db.Open()
	defer database.Close()

	result, err := Scan(tmp, database)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
}

func TestScanSubfolderRoms(t *testing.T) {
	tmp := t.TempDir()
	// Simulate ~/roms/Roms/gb/game.gb
	gbDir := filepath.Join(tmp, "Roms", "gb")
	os.MkdirAll(gbDir, 0755)
	os.WriteFile(filepath.Join(gbDir, "test.gb"), []byte("fake GB ROM data"), 0644)

	os.Setenv("HOME", tmp)
	database, _ := db.Open()
	defer database.Close()

	result, err := Scan(tmp, database)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
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
		{"/roms", "/roms/megadrive/game.md", "MD"},
		{"/roms", "/roms/pcengine/game.pce", "PCE"},
		{"/roms", "/roms/neogeo/kof.zip", "NEOGEO"},
		{"/roms", "/roms/arcade/sf2.zip", "ARCADE"},
		{"/roms", "/roms/Roms/gb/game.gb", "GB"},
		{"/roms", "/roms/segasaturn/game.iso", "SS"},
		{"/roms", "/roms/wonderswan/game.ws", "WS"},
		{"/roms", "/roms/wonderswancolor/game.wsc", "WSC"},
	}
	for _, tt := range tests {
		got := detectPlatform(tt.root, tt.path)
		if got != tt.want {
			t.Errorf("detectPlatform(%q, %q) = %q, want %q", tt.root, tt.path, got, tt.want)
		}
	}
}
