package gamedb

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/*.json
var dataFS embed.FS

type GameEntry struct {
	TitleJA     string
	DescJA      string
	Developer   string
	Publisher   string
	ReleaseDate string
	Genre       string
	Players     string
}

// platform -> titleEN -> GameEntry
var cache map[string]map[string]*GameEntry
var once sync.Once

func load() {
	cache = make(map[string]map[string]*GameEntry)
	entries, err := dataFS.ReadDir("data")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		platform := strings.TrimSuffix(e.Name(), ".json")
		data, err := dataFS.ReadFile("data/" + e.Name())
		if err != nil {
			continue
		}
		var raw map[string]struct {
			TitleJA     string `json:"title_ja"`
			DescJA      string `json:"desc_ja"`
			Developer   string `json:"developer"`
			Publisher   string `json:"publisher"`
			ReleaseDate string `json:"release_date"`
			Genre       string `json:"genre"`
			Players     string `json:"players"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		m := make(map[string]*GameEntry, len(raw))
		for k, v := range raw {
			m[k] = &GameEntry{
				TitleJA:     v.TitleJA,
				DescJA:      v.DescJA,
				Developer:   v.Developer,
				Publisher:   v.Publisher,
				ReleaseDate: v.ReleaseDate,
				Genre:       v.Genre,
				Players:     v.Players,
			}
		}
		cache[strings.ToUpper(platform)] = m
	}
}

func Lookup(platform, titleEN string) *GameEntry {
	once.Do(load)
	m, ok := cache[strings.ToUpper(platform)]
	if !ok {
		return nil
	}
	return m[titleEN]
}

func LookupByHash(platform, crc32, md5, sha1 string) *GameEntry {
	return nil
}
