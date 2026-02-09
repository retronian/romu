package scanner

import (
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

// Platform mapping: folder name -> platform, and valid extensions
var platformFolders = map[string]string{
	"fc":  "FC",
	"nes": "FC",
	"sfc": "SFC",
	"snes": "SFC",
	"gb":  "GB",
	"gbc": "GBC",
	"gba": "GBA",
	"md":  "MD",
	"genesis": "MD",
	"ps1": "PS1",
	"psx": "PS1",
	"n64": "N64",
	"nds": "NDS",
	"pce": "PCE",
	"msx": "MSX",
	"gg":  "GG",
	"sms": "SMS",
	"ws":  "WS",
	"wsc": "WSC",
	"ngp": "NGP",
}

var platformExtensions = map[string][]string{
	"FC":  {".nes"},
	"SFC": {".sfc", ".smc"},
	"GB":  {".gb"},
	"GBC": {".gbc"},
	"GBA": {".gba"},
	"MD":  {".md", ".bin", ".gen"},
	"PS1": {".bin", ".cue", ".img", ".iso"},
	"N64": {".n64", ".z64", ".v64"},
	"NDS": {".nds"},
	"PCE": {".pce"},
	"MSX": {".rom"},
	"GG":  {".gg"},
	"SMS": {".sms"},
	"WS":  {".ws"},
	"WSC": {".wsc"},
	"NGP": {".ngp"},
}

type Result struct {
	Scanned  int
	Added    int
	Skipped  int
	Errors   int
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

func detectPlatform(root, path string) string {
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
