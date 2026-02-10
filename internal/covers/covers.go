package covers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/retronian/romu/internal/db"
)

var LibretroSystems = map[string]string{
	"FC":     "Nintendo_-_Nintendo_Entertainment_System",
	"SFC":    "Nintendo_-_Super_Nintendo_Entertainment_System",
	"GB":     "Nintendo_-_Game_Boy",
	"GBC":    "Nintendo_-_Game_Boy_Color",
	"GBA":    "Nintendo_-_Game_Boy_Advance",
	"MD":     "Sega_-_Mega_Drive_-_Genesis",
	"N64":    "Nintendo_-_Nintendo_64",
	"NDS":    "Nintendo_-_Nintendo_DS",
	"PCE":    "NEC_-_PC_Engine_-_TurboGrafx_16",
	"GG":     "Sega_-_Game_Gear",
	"SMS":    "Sega_-_Master_System_-_Mark_III",
	"WS":     "Bandai_-_WonderSwan",
	"WSC":    "Bandai_-_WonderSwan_Color",
	"NGP":    "SNK_-_Neo_Geo_Pocket",
	"NEOGEO": "SNK_-_Neo_Geo_Pocket",
}

func FetchCovers(database *db.DB, platform, outputDir string, force bool) error {
	home, _ := os.UserHomeDir()
	if outputDir == "" {
		outputDir = filepath.Join(home, ".romu", "covers")
	}

	// Get platforms to process
	var platforms []string
	if platform != "" {
		platforms = []string{platform}
	} else {
		var err error
		platforms, err = database.GetPlatforms()
		if err != nil {
			return err
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, plat := range platforms {
		sys, ok := LibretroSystems[plat]
		if !ok {
			fmt.Printf("[%s] No libretro system mapping, skipping\n", plat)
			continue
		}

		roms, _, err := database.GetEnrichableRoms(plat)
		if err != nil {
			return fmt.Errorf("[%s] db error: %w", plat, err)
		}
		if len(roms) == 0 {
			fmt.Printf("[%s] No matched games\n", plat)
			continue
		}

		dir := filepath.Join(outputDir, plat)
		os.MkdirAll(dir, 0755)

		fetched, notFound, skipped := 0, 0, 0
		total := len(roms)

		for i, rom := range roms {
			// Sanitize filename: libretro uses the game name directly
			safeName := sanitizeForFilename(rom.TitleEN)
			outPath := filepath.Join(dir, safeName+".png")

			if !force {
				if _, err := os.Stat(outPath); err == nil {
					skipped++
					fetched++
					continue
				}
			}

			// Build URL
			encodedName := url.PathEscape(strings.ReplaceAll(rom.TitleEN, "&", "_"))
			imgURL := fmt.Sprintf("https://raw.githubusercontent.com/libretro-thumbnails/%s/master/Named_Boxarts/%s.png", sys, encodedName)

			resp, err := client.Get(imgURL)
			if err != nil {
				notFound++
				if (i+1)%100 == 0 || i+1 == total {
					fmt.Printf("\r[%s] %d/%d fetched (%d not found)", plat, fetched, total, notFound)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if resp.StatusCode == 404 {
				resp.Body.Close()
				notFound++
			} else if resp.StatusCode == 200 {
				data, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				os.WriteFile(outPath, data, 0644)
				fetched++
			} else {
				resp.Body.Close()
				notFound++
			}

			if (i+1)%10 == 0 || i+1 == total {
				fmt.Printf("\r[%s] %d/%d fetched (%d not found)    ", plat, fetched, total, notFound)
			}

			time.Sleep(100 * time.Millisecond)
		}
		fmt.Printf("\r[%s] %d/%d fetched (%d not found, %d cached)\n", plat, fetched, total, notFound, skipped)
	}
	return nil
}

func sanitizeForFilename(name string) string {
	// Replace characters not allowed in filenames
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}
