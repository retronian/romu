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
	Path        string `xml:"path"`
	Name        string `xml:"name"`
	Desc        string `xml:"desc"`
	ReleaseDate string `xml:"releasedate"`
	Developer   string `xml:"developer"`
	Publisher   string `xml:"publisher"`
	Genre       string `xml:"genre"`
	Players     string `xml:"players"`
	Rating      string `xml:"rating"`
	Thumbnail   string `xml:"thumbnail"`
	Image       string `xml:"image"`
	Marquee     string `xml:"marquee"`
}

// GameListEntry holds a parsed gamelist.xml entry
type GameListEntry struct {
	Filename    string
	Name        string
	Desc        string
	ReleaseDate string
	Developer   string
	Publisher   string
	Genre       string
	Players     string
	Rating      string
	Thumbnail   string
	Image       string
	Marquee     string
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
				Filename:    filename,
				Name:        name,
				Desc:        strings.TrimSpace(g.Desc),
				ReleaseDate: strings.TrimSpace(g.ReleaseDate),
				Developer:   strings.TrimSpace(g.Developer),
				Publisher:   strings.TrimSpace(g.Publisher),
				Genre:       strings.TrimSpace(g.Genre),
				Players:     strings.TrimSpace(g.Players),
				Rating:      strings.TrimSpace(g.Rating),
				Thumbnail:   strings.TrimSpace(g.Thumbnail),
				Image:       strings.TrimSpace(g.Image),
				Marquee:     strings.TrimSpace(g.Marquee),
			})
		}
	}
	return entries, nil
}
