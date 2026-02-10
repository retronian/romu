package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/retronian/romu/internal/dat"
	"github.com/retronian/romu/internal/db"
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
			dbEntries[i] = db.GameListEntry{Filename: e.Filename, Name: e.Name}
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
