package dat

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmulationStation gamelist.xml structures
type GameList struct {
	XMLName xml.Name       `xml:"gameList"`
	Games   []GameListGame `xml:"game"`
}

type GameListGame struct {
	Path string `xml:"path"`
	Name string `xml:"name"`
}

// GameListEntry holds a parsed gamelist.xml entry
type GameListEntry struct {
	Filename string // e.g. "1944j.zip"
	Name     string // e.g. "1944 ザ・ループマスター"
}

// ParseGameList parses an EmulationStation gamelist.xml file
func ParseGameList(path string) ([]GameListEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open gamelist: %w", err)
	}
	defer f.Close()

	var gl GameList
	if err := xml.NewDecoder(f).Decode(&gl); err != nil {
		return nil, fmt.Errorf("parse gamelist XML: %w", err)
	}

	var entries []GameListEntry
	for _, g := range gl.Games {
		filename := filepath.Base(g.Path)
		name := strings.TrimSpace(g.Name)
		if filename != "" && name != "" {
			entries = append(entries, GameListEntry{
				Filename: filename,
				Name:     name,
			})
		}
	}
	return entries, nil
}
