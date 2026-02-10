package dat

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/retronian/romu/internal/db"
)

// No-Intro DAT XML structure
type Datafile struct {
	XMLName xml.Name  `xml:"datafile"`
	Header  Header    `xml:"header"`
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
	Name string `xml:"name,attr"`
	Size string `xml:"size,attr"`
	CRC  string `xml:"crc,attr"`
	MD5  string `xml:"md5,attr"`
	SHA1 string `xml:"sha1,attr"`
}

// ParseDAT parses a No-Intro DAT file (XML or ClrMamePro format)
func ParseDAT(path string, platform string) ([]db.DATRom, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open DAT: %w", err)
	}
	defer f.Close()

	// Peek at first line to detect format
	scanner := bufio.NewScanner(f)
	scanner.Scan()
	firstLine := strings.TrimSpace(scanner.Text())
	f.Seek(0, 0)

	if strings.HasPrefix(firstLine, "clrmamepro") || strings.HasPrefix(firstLine, "clrmamepro (") {
		return parseClrMamePro(f, platform)
	}
	return parseXML(f, platform)
}

func parseXML(f *os.File, platform string) ([]db.DATRom, string, error) {
	var datafile Datafile
	dec := xml.NewDecoder(f)
	if err := dec.Decode(&datafile); err != nil {
		return nil, "", fmt.Errorf("parse DAT XML: %w", err)
	}

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

// ClrMamePro format parser
var clrRomLineRe = regexp.MustCompile(`rom\s*\(\s*name\s+"([^"]+)"\s+size\s+(\d+)\s+crc\s+(\w+)\s+md5\s+(\w+)\s+sha1\s+(\w+)(?:\s+[^)]*?)?\s*\)`)

func parseClrMamePro(f *os.File, platform string) ([]db.DATRom, string, error) {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	headerName := ""
	var roms []db.DATRom
	currentGame := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Header name
		if strings.HasPrefix(line, `name "`) {
			val := extractQuoted(line, "name")
			if headerName == "" {
				headerName = val
			}
		}

		// Game block start
		if strings.HasPrefix(line, "game (") || line == "game (" {
			currentGame = ""
		}

		// Game name inside block
		if currentGame == "" && strings.HasPrefix(line, `name "`) {
			currentGame = extractQuoted(line, "name")
		}

		// ROM line (can be inline with game or separate)
		if strings.Contains(line, "rom (") || strings.HasPrefix(line, "rom (") {
			m := clrRomLineRe.FindStringSubmatch(line)
			if m != nil {
				gameName := currentGame
				if gameName == "" {
					// Try to extract from rom filename
					gameName = m[1]
				}
				size, _ := strconv.ParseInt(m[2], 10, 64)
				roms = append(roms, db.DATRom{
					GameTitle: gameName,
					Platform:  "", // set below
					CRC32:     strings.ToUpper(m[3]),
					MD5:       strings.ToUpper(m[4]),
					SHA1:      strings.ToUpper(m[5]),
					Size:      size,
				})
			}
		}
	}

	if platform == "" {
		platform = detectPlatformFromHeader(headerName)
	}
	if platform == "" {
		return nil, "", fmt.Errorf("cannot detect platform from DAT header %q, use --platform flag", headerName)
	}

	// Set platform on all roms
	for i := range roms {
		roms[i].Platform = platform
	}

	return roms, headerName, nil
}

func extractQuoted(line, key string) string {
	prefix := key + ` "`
	idx := strings.Index(line, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(line[start:], `"`)
	if end < 0 {
		return line[start:]
	}
	return line[start : start+end]
}

func detectPlatformFromHeader(name string) string {
	lower := strings.ToLower(name)
	patterns := map[string]string{
		"nintendo entertainment system":     "FC",
		"famicom":                           "FC",
		"super nintendo":                    "SFC",
		"super famicom":                     "SFC",
		"game boy advance":                  "GBA",
		"game boy color":                    "GBC",
		"game boy":                          "GB",
		"mega drive":                        "MD",
		"genesis":                           "MD",
		"playstation":                       "PS1",
		"nintendo 64":                       "N64",
		"nintendo ds":                       "NDS",
		"pc engine":                         "PCE",
		"turbografx":                        "PCE",
		"game gear":                         "GG",
		"master system":                     "SMS",
		"wonderswan color":                  "WSC",
		"wonderswan":                        "WS",
		"neo geo pocket":                    "NGP",
	}
	// Check longer patterns first to avoid false matches
	order := []string{
		"game boy advance", "game boy color", "game boy",
		"wonderswan color", "wonderswan",
		"super nintendo", "super famicom",
		"nintendo entertainment system", "famicom",
		"mega drive", "genesis",
		"nintendo 64", "nintendo ds",
		"pc engine", "turbografx",
		"game gear", "master system",
		"neo geo pocket", "playstation",
	}
	for _, pattern := range order {
		if strings.Contains(lower, pattern) {
			return patterns[pattern]
		}
	}
	return ""
}
