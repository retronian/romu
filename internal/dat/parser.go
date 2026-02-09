package dat

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/retronian/romu/internal/db"
)

// No-Intro DAT XML structure
type Datafile struct {
	XMLName xml.Name `xml:"datafile"`
	Header  Header   `xml:"header"`
	Games   []XMLGame `xml:"game"`
}

type Header struct {
	Name        string `xml:"name"`
	Description string `xml:"description"`
}

type XMLGame struct {
	Name string   `xml:"name,attr"`
	ROMs []XMLRom `xml:"rom"`
}

type XMLRom struct {
	Name  string `xml:"name,attr"`
	Size  string `xml:"size,attr"`
	CRC   string `xml:"crc,attr"`
	MD5   string `xml:"md5,attr"`
	SHA1  string `xml:"sha1,attr"`
}

// ParseDAT parses a No-Intro DAT XML file and returns DATRom entries
func ParseDAT(path string, platform string) ([]db.DATRom, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open DAT: %w", err)
	}
	defer f.Close()

	var datafile Datafile
	dec := xml.NewDecoder(f)
	if err := dec.Decode(&datafile); err != nil {
		return nil, "", fmt.Errorf("parse DAT XML: %w", err)
	}

	// Auto-detect platform from header if not specified
	if platform == "" {
		platform = detectPlatformFromHeader(datafile.Header.Name)
	}
	if platform == "" {
		return nil, "", fmt.Errorf("cannot detect platform from DAT header %q, use --platform flag", datafile.Header.Name)
	}

	var roms []db.DATRom
	for _, g := range datafile.Games {
		for _, r := range g.ROMs {
			size, _ := strconv.ParseInt(r.Size, 10, 64)
			roms = append(roms, db.DATRom{
				GameTitle: g.Name,
				Platform:  platform,
				CRC32:     strings.ToUpper(r.CRC),
				MD5:       strings.ToUpper(r.MD5),
				SHA1:      strings.ToUpper(r.SHA1),
				Size:      size,
			})
		}
	}

	return roms, datafile.Header.Name, nil
}

func detectPlatformFromHeader(name string) string {
	lower := strings.ToLower(name)
	patterns := map[string]string{
		"nintendo - nes":                    "FC",
		"nintendo - nintendo entertainment system": "FC",
		"nintendo - famicom":                "FC",
		"nintendo - super nintendo":         "SFC",
		"nintendo - super famicom":          "SFC",
		"nintendo - game boy -":             "GB",
		"nintendo - game boy color":         "GBC",
		"nintendo - game boy advance":       "GBA",
		"sega - mega drive":                 "MD",
		"sega - genesis":                    "MD",
		"sony - playstation":                "PS1",
		"nintendo - nintendo 64":            "N64",
		"nintendo - nintendo ds":            "NDS",
		"nec - pc engine":                   "PCE",
		"sega - game gear":                  "GG",
		"sega - master system":              "SMS",
		"bandai - wonderswan -":             "WS",
		"bandai - wonderswan color":         "WSC",
		"snk - neo geo pocket":              "NGP",
	}
	for pattern, platform := range patterns {
		if strings.Contains(lower, pattern) {
			return platform
		}
	}
	return ""
}
