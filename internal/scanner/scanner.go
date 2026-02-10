package scanner

import (
	"archive/zip"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/retronian/romu/internal/db"
)

// Platform mapping: folder name -> platform
var platformFolders = map[string]string{
	"fc":              "FC",
	"nes":             "FC",
	"sfc":             "SFC",
	"snes":            "SFC",
	"gb":              "GB",
	"gbc":             "GBC",
	"gba":             "GBA",
	"md":              "MD",
	"genesis":         "MD",
	"megadrive":       "MD",
	"ps1":             "PS1",
	"psx":             "PS1",
	"n64":             "N64",
	"nds":             "NDS",
	"pce":             "PCE",
	"pcengine":        "PCE",
	"pcenginecd":      "PCE",
	"msx":             "MSX",
	"gg":              "GG",
	"sms":             "SMS",
	"ws":              "WS",
	"wonderswan":      "WS",
	"wsc":             "WSC",
	"wonderswancolor": "WSC",
	"ngp":             "NGP",
	"pcfx":            "PCFX",
	"neogeo":          "NEOGEO",
	"pico8":           "PICO8",
	"ps2":             "PS2",
	"segasaturn":      "SS",
	"arcade":          "ARCADE",
}

var platformExtensions = map[string][]string{
	"FC":     {".nes"},
	"SFC":    {".sfc", ".smc"},
	"GB":     {".gb"},
	"GBC":    {".gbc"},
	"GBA":    {".gba"},
	"MD":     {".md", ".bin", ".gen"},
	"PS1":    {".bin", ".cue", ".img", ".iso"},
	"N64":    {".n64", ".z64", ".v64"},
	"NDS":    {".nds"},
	"PCE":    {".pce"},
	"MSX":    {".rom"},
	"GG":     {".gg"},
	"SMS":    {".sms"},
	"WS":     {".ws"},
	"WSC":    {".wsc"},
	"NGP":    {".ngp"},
	"PCFX":   {".iso", ".bin", ".cue"},
	"NEOGEO": {".zip"},
	"PICO8":  {".p8", ".png"},
	"PS2":    {".iso", ".bin", ".cue"},
	"SS":     {".iso", ".bin", ".cue"},
	"ARCADE": {".zip"},
}

// Platforms where .zip file itself IS the ROM (don't look inside)
var zipIsRomPlatforms = map[string]bool{
	"NEOGEO": true,
	"ARCADE": true,
}

type Result struct {
	Scanned int
	Added   int
	Skipped int
	Errors  int
}

func Scan(root string, database *db.DB) (*Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	result := &Result{}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors++
			return nil
		}
		if info.IsDir() {
			return nil
		}

		platform := detectPlatform(root, path)
		if platform == "" {
			result.Skipped++
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Handle ZIP files
		if ext == ".zip" {
			if zipIsRomPlatforms[platform] {
				// ZIP itself is the ROM — hash the zip file
				if !isValidExtension(platform, ".zip") {
					result.Skipped++
					return nil
				}
				result.Scanned++
				crc, md5h, sha1h, err := hashFile(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "hash error %s: %v\n", path, err)
					result.Errors++
					return nil
				}
				err = database.UpsertRomFile(path, filepath.Base(path), info.Size(), crc, md5h, sha1h, platform)
				if err != nil {
					fmt.Fprintf(os.Stderr, "db error %s: %v\n", path, err)
					result.Errors++
					return nil
				}
				result.Added++
				fmt.Printf("  [%s] %s (CRC32: %s)\n", platform, filepath.Base(path), crc)
			} else {
				// Look inside ZIP for ROM files
				scanned := scanZipContents(path, platform, info.Size(), database, result)
				if !scanned {
					result.Skipped++
				}
			}
			return nil
		}

		// Regular file
		if !isValidExtension(platform, ext) {
			result.Skipped++
			return nil
		}

		result.Scanned++

		crc, md5h, sha1h, err := hashFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hash error %s: %v\n", path, err)
			result.Errors++
			return nil
		}

		err = database.UpsertRomFile(path, filepath.Base(path), info.Size(), crc, md5h, sha1h, platform)
		if err != nil {
			fmt.Fprintf(os.Stderr, "db error %s: %v\n", path, err)
			result.Errors++
			return nil
		}

		result.Added++
		fmt.Printf("  [%s] %s (CRC32: %s)\n", platform, filepath.Base(path), crc)
		return nil
	})

	return result, err
}

// scanZipContents opens a ZIP and hashes ROM files inside it.
// Returns true if at least one ROM file was found and processed.
func scanZipContents(zipPath, platform string, zipSize int64, database *db.DB, result *Result) bool {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zip open error %s: %v\n", zipPath, err)
		result.Errors++
		return false
	}
	defer r.Close()

	found := false
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if !isValidExtension(platform, ext) {
			continue
		}

		found = true
		result.Scanned++

		crc, md5h, sha1h, err := hashZipEntry(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hash error %s!%s: %v\n", zipPath, f.Name, err)
			result.Errors++
			continue
		}

		// Store path as zipPath, display name as inner file name
		displayName := filepath.Base(zipPath) + "/" + f.Name
		err = database.UpsertRomFile(zipPath, displayName, int64(f.UncompressedSize64), crc, md5h, sha1h, platform)
		if err != nil {
			fmt.Fprintf(os.Stderr, "db error %s!%s: %v\n", zipPath, f.Name, err)
			result.Errors++
			continue
		}

		result.Added++
		fmt.Printf("  [%s] %s (CRC32: %s)\n", platform, displayName, crc)
	}
	return found
}

func hashZipEntry(f *zip.File) (string, string, string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", "", "", err
	}
	defer rc.Close()

	crcH := crc32.NewIEEE()
	md5H := md5.New()
	sha1H := sha1.New()

	w := io.MultiWriter(crcH, md5H, sha1H)
	if _, err := io.Copy(w, rc); err != nil {
		return "", "", "", err
	}

	return fmt.Sprintf("%08X", crcH.Sum32()),
		strings.ToUpper(hex.EncodeToString(md5H.Sum(nil))),
		strings.ToUpper(hex.EncodeToString(sha1H.Sum(nil))),
		nil
}

func detectPlatform(root, path string) string {
	// First check if root itself is a platform folder
	rootBase := strings.ToLower(filepath.Base(root))
	if p, ok := platformFolders[rootBase]; ok {
		return p
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	// Check each directory component from top
	for _, part := range parts {
		lower := strings.ToLower(part)
		if p, ok := platformFolders[lower]; ok {
			return p
		}
	}
	return ""
}

func isValidExtension(platform, ext string) bool {
	exts, ok := platformExtensions[platform]
	if !ok {
		return true // unknown platform, accept all
	}
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func hashFile(path string) (string, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", "", err
	}
	defer f.Close()

	crcH := crc32.NewIEEE()
	md5H := md5.New()
	sha1H := sha1.New()

	w := io.MultiWriter(crcH, md5H, sha1H)
	if _, err := io.Copy(w, f); err != nil {
		return "", "", "", err
	}

	return fmt.Sprintf("%08X", crcH.Sum32()),
		strings.ToUpper(hex.EncodeToString(md5H.Sum(nil))),
		strings.ToUpper(hex.EncodeToString(sha1H.Sum(nil))),
		nil
}
