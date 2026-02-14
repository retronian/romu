package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/retronian/romu/internal/covers"
	"github.com/retronian/romu/internal/dat"
	"github.com/retronian/romu/internal/db"
	"github.com/retronian/romu/internal/gamedb"
	"github.com/retronian/romu/internal/scanner"
	"github.com/retronian/romu/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		cmdScan()
	case "list":
		cmdList()
	case "search":
		cmdSearch()
	case "stats":
		cmdStats()
	case "server":
		cmdServer()
	case "import-dat":
		cmdImportDAT()
	case "import-gamelist":
		cmdImportGameList()
	case "export-gamelist":
		cmdExportGameList()
	case "enrich":
		cmdEnrich()
	case "fetch-covers":
		cmdFetchCovers()
	case "match":
		cmdMatch()
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`romu - ROM collection manager

Usage:
  romu scan <path>              Scan a ROM directory recursively
  romu list                     List registered ROMs
  romu search <query>           Search ROMs by title/filename
                                [--platform XX] to filter by platform
  romu stats                    Show collection statistics
  romu server                   Start web UI server
                                [--port XXXX] (default: 8080)
  romu import-dat <dat-file>    Import a No-Intro DAT file
                                [--platform XX] to override auto-detection
  romu import-gamelist <dir>    Import all gamelist.xml from ROM directory
  romu export-gamelist <dir>    Export gamelist.xml per platform
                                [--platform XX] to export single platform
                                ZIP files use ./zipname.zip as path
                                Empty metadata fields are omitted
  romu enrich                   Apply gamedb metadata to matched games
                                [--platform XX] to filter by platform
  romu fetch-covers             Download cover art from libretro-thumbnails
                                [--platform XX] [--output-dir DIR] [--force]
  romu match                    Match ROMs to games by hash
  romu help                     Show this help`)
}

func cmdSearch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: romu search <query> [--platform XX]")
		os.Exit(1)
	}
	query := os.Args[2]
	platform := ""
	for i := 3; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--platform" {
			platform = os.Args[i+1]
		}
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	files, total, err := database.SearchRoms(query, platform, 1, 100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Printf("No results for %q\n", query)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLATFORM\tFILENAME\tTITLE")
	for _, f := range files {
		title := "-"
		if f.TitleJA != nil {
			title = *f.TitleJA
		} else if f.TitleEN != nil {
			title = *f.TitleEN
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", f.Platform, f.Filename, title)
	}
	w.Flush()
	fmt.Printf("\nFound: %d ROMs\n", total)
}

func cmdStats() {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	stats, err := database.GetStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stats error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLATFORM\tTOTAL\tMATCHED\tUNMATCHED\tTITLE_EN\tTITLE_JA")
	for _, p := range stats.Platforms {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\n", p.Platform, p.Total, p.Matched, p.Unmatched, p.HasTitleEN, p.HasTitleJA)
	}
	fmt.Fprintf(w, "---\t---\t---\t---\t---\t---\n")
	fmt.Fprintf(w, "TOTAL\t%d\t%d\t%d\t\t\n", stats.Total, stats.Matched, stats.Unmatched)
	w.Flush()
}

func cmdServer() {
	port := 8080
	for i := 2; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--port" {
			p, err := strconv.Atoi(os.Args[i+1])
			if err == nil {
				port = p
			}
		}
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	srv := server.New(database, port)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func cmdScan() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: romu scan <path>")
		os.Exit(1)
	}
	path := os.Args[2]

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	fmt.Printf("Scanning %s ...\n", path)
	result, err := scanner.Scan(path, database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone! Scanned: %d, Added: %d, Skipped: %d, Errors: %d\n",
		result.Scanned, result.Added, result.Skipped, result.Errors)
}

func cmdList() {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	files, err := database.ListRomFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "list error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No ROMs registered. Run 'romu scan <path>' first.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLATFORM\tFILENAME\tSIZE\tCRC32\tGAME")
	for _, f := range files {
		game := "-"
		if f.TitleJA != nil {
			game = *f.TitleJA
		} else if f.TitleEN != nil {
			game = *f.TitleEN
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", f.Platform, f.Filename, f.Size, f.HashCRC32, game)
	}
	w.Flush()
	fmt.Printf("\nTotal: %d ROMs\n", len(files))
}

func cmdImportGameList() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: romu import-gamelist <roms-dir>")
		fmt.Fprintln(os.Stderr, "  Scans for gamelist.xml in platform subdirectories")
		os.Exit(1)
	}
	romsDir := os.Args[2]

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Walk romsDir for gamelist.xml files
	totalCreated, totalMatched := 0, 0
	err = filepath.Walk(romsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "gamelist.xml" {
			return nil
		}

		// Detect platform from parent directory name
		parentDir := strings.ToLower(filepath.Base(filepath.Dir(path)))
		platform := scanner.DetectPlatformFromFolder(parentDir)
		if platform == "" {
			fmt.Printf("  skip %s (unknown platform: %s)\n", path, parentDir)
			return nil
		}

		entries, err := dat.ParseGameList(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error %s: %v\n", path, err)
			return nil
		}

		// Convert to db entries
		dbEntries := make([]db.GameListEntry, len(entries))
		for i, e := range entries {
			dbEntries[i] = db.GameListEntry{
				Filename:    e.Filename,
				Name:        e.Name,
				Desc:        e.Desc,
				ReleaseDate: e.ReleaseDate,
				Developer:   e.Developer,
				Publisher:   e.Publisher,
				Genre:       e.Genre,
				Players:     e.Players,
				Rating:      e.Rating,
			}
		}

		created, matched, err := database.MatchByGameList(dbEntries, platform)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error %s: %v\n", path, err)
			return nil
		}

		fmt.Printf("  [%s] %s: %d games created, %d ROMs matched\n", platform, parentDir, created, matched)
		totalCreated += created
		totalMatched += matched
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nTotal: %d games created, %d ROMs matched\n", totalCreated, totalMatched)
}

func cmdEnrich() {
	platform := ""
	showSkipped := false
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--platform" && i+1 < len(os.Args) {
			platform = os.Args[i+1]
			i++
		}
		if os.Args[i] == "--show-skipped" {
			showSkipped = true
		}
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	roms, noMatch, err := database.GetEnrichableRoms(platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if noMatch > 0 {
		fmt.Printf("Note: %d ROM(s) have no game match. Run 'romu match' with DAT files first.\n\n", noMatch)
	}

	enriched, skipped := 0, 0
	// platform -> list of skipped titles
	skippedByPlatform := make(map[string][]string)
	for _, r := range roms {
		entry := gamedb.Lookup(r.Platform, r.TitleEN)
		if entry == nil {
			skipped++
			skippedByPlatform[r.Platform] = append(skippedByPlatform[r.Platform], r.TitleEN)
			continue
		}
		err := database.UpdateGameMetadata(r.GameID, entry.TitleJA, entry.DescJA, entry.Developer, entry.Publisher, entry.ReleaseDate, entry.Genre, entry.Players)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error updating game %d: %v\n", r.GameID, err)
			continue
		}
		enriched++
	}

	// Also try to enrich unmatched ROMs by filename
	unmatchedRoms, err := database.GetUnmatchedRoms(platform)
	filenameEnriched := 0
	filenameSkipped := 0
	if err == nil {
		for _, ur := range unmatchedRoms {
			// Extract title from filename (may be "archive.zip/romname.ext")
			title := ur.Filename
			if idx := strings.LastIndex(title, "/"); idx >= 0 {
				title = title[idx+1:]
			}
			// Strip ROM extension
			for _, ext := range []string{".zip", ".7z", ".nes", ".sfc", ".smc", ".gb", ".gbc", ".gba", ".md", ".bin", ".pce", ".ws", ".wsc", ".n64", ".z64", ".v64", ".nds"} {
				title = strings.TrimSuffix(title, ext)
			}
			// Also try the zip name (before /) as fallback
			zipTitle := ur.Filename
			if idx := strings.Index(zipTitle, "/"); idx >= 0 {
				zipTitle = zipTitle[:idx]
			}
			zipTitle = strings.TrimSuffix(zipTitle, ".zip")
			zipTitle = strings.TrimSuffix(zipTitle, ".7z")
			entry := gamedb.Lookup(ur.Platform, title)
			lookupTitle := title
			if entry == nil {
				entry = gamedb.Lookup(ur.Platform, zipTitle)
				lookupTitle = zipTitle
			}
			if entry == nil {
				filenameSkipped++
				skippedByPlatform[ur.Platform] = append(skippedByPlatform[ur.Platform], title)
				continue
			}
			err := database.CreateGameAndLink(ur.ID, lookupTitle, ur.Platform, entry.TitleJA, entry.DescJA, entry.Developer, entry.Publisher, entry.ReleaseDate, entry.Genre, entry.Players)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error creating game for %s: %v\n", title, err)
				continue
			}
			filenameEnriched++
		}
	}

	fmt.Printf("Enriched %d games (%d skipped - no gamedb entry)\n", enriched, skipped)
	if filenameEnriched > 0 || filenameSkipped > 0 {
		fmt.Printf("Enriched %d unmatched ROMs by filename (%d skipped)\n", filenameEnriched, filenameSkipped)
	}

	if showSkipped && (skipped > 0 || filenameSkipped > 0) {
		fmt.Printf("\n--- Skipped titles by platform ---\n")
		// Sort platforms for consistent output
		platforms := make([]string, 0, len(skippedByPlatform))
		for p := range skippedByPlatform {
			platforms = append(platforms, p)
		}
		sort.Strings(platforms)
		for _, p := range platforms {
			titles := skippedByPlatform[p]
			fmt.Printf("\n[%s] (%d skipped)\n", p, len(titles))
			sort.Strings(titles)
			for _, t := range titles {
				fmt.Printf("  - %s\n", t)
			}
		}
	}
}

func cmdExportGameList() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: romu export-gamelist <output-dir> [--platform XX]")
		os.Exit(1)
	}
	outDir := os.Args[2]
	platform := ""
	for i := 3; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--platform" {
			platform = os.Args[i+1]
		}
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	var platforms []string
	if platform != "" {
		platforms = []string{platform}
	} else {
		platforms, err = database.GetPlatforms()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	for _, p := range platforms {
		entries, err := database.ExportGameList(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error [%s]: %v\n", p, err)
			continue
		}
		if len(entries) == 0 {
			continue
		}

		dir := filepath.Join(outDir, p)
		os.MkdirAll(dir, 0755)
		outPath := filepath.Join(dir, "gamelist.xml")

		f, err := os.Create(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error creating %s: %v\n", outPath, err)
			continue
		}
		f.WriteString("<?xml version=\"1.0\"?>\n<gameList>\n")
		for _, e := range entries {
			f.WriteString("  <game>\n")
			writeXMLField(f, "path", e.Path)
			writeXMLField(f, "name", e.Name)
			writeXMLField(f, "desc", e.Desc)
			writeXMLField(f, "releasedate", e.ReleaseDate)
			writeXMLField(f, "developer", e.Developer)
			writeXMLField(f, "publisher", e.Publisher)
			writeXMLField(f, "genre", e.Genre)
			writeXMLField(f, "players", e.Players)
			writeXMLField(f, "rating", e.Rating)
			f.WriteString("  </game>\n")
		}
		f.WriteString("</gameList>\n")
		f.Close()

		fmt.Printf("  [%s] %d games → %s\n", p, len(entries), outPath)
	}
}

func cmdImportDAT() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: romu import-dat <dat-file> [--platform XX]")
		os.Exit(1)
	}
	datPath := os.Args[2]
	platform := ""
	for i := 3; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--platform" {
			platform = os.Args[i+1]
		}
	}

	roms, headerName, err := dat.ParseDAT(datPath, platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	count, err := database.ImportDATGames(roms)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Imported DAT: %s\n", headerName)
	fmt.Printf("Games added: %d (from %d ROM entries)\n", count, len(roms))
}

func cmdMatch() {
	// For matching, we need DAT files to have been imported first.
	// We re-read all DAT info from the games table and match by hash.
	// However, since we don't store ROM hashes in games table,
	// we need a different approach: store DAT ROM info separately or
	// re-parse DAT files. For simplicity, we'll ask user to provide DAT files again.

	fmt.Println("Matching ROMs to games by hash...")
	fmt.Println("Note: You need to provide DAT files for matching.")
	fmt.Println()

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: romu match <dat-file> [--platform XX]")
		fmt.Fprintln(os.Stderr, "  Provide the same DAT file(s) used with import-dat")
		os.Exit(1)
	}

	datPath := os.Args[2]
	platform := ""
	for i := 3; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--platform" {
			platform = os.Args[i+1]
		}
	}

	roms, _, err := dat.ParseDAT(datPath, platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	matched, err := database.MatchROMs(roms)
	if err != nil {
		fmt.Fprintf(os.Stderr, "match error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Matched %d ROM(s) to games.\n", matched)
}

func cmdFetchCovers() {
	platform := ""
	outputDir := ""
	force := false
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--platform":
			if i+1 < len(os.Args) {
				platform = os.Args[i+1]
				i++
			}
		case "--output-dir":
			if i+1 < len(os.Args) {
				outputDir = os.Args[i+1]
				i++
			}
		case "--force":
			force = true
		}
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := covers.FetchCovers(database, platform, outputDir, force); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func writeXMLField(f *os.File, tag, value string) {
	if value == "" {
		return
	}
	escaped := xmlEscape(value)
	fmt.Fprintf(f, "    <%s>%s</%s>\n", tag, escaped, tag)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
