package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/retronian/romu/internal/dat"
	"github.com/retronian/romu/internal/db"
	"github.com/retronian/romu/internal/scanner"
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
	case "import-dat":
		cmdImportDAT()
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
  romu import-dat <dat-file>    Import a No-Intro DAT file
                                [--platform XX] to override auto-detection
  romu match                    Match ROMs to games by hash
  romu help                     Show this help`)
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
		if f.TitleEN != nil {
			game = *f.TitleEN
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", f.Platform, f.Filename, f.Size, f.HashCRC32, game)
	}
	w.Flush()
	fmt.Printf("\nTotal: %d ROMs\n", len(files))
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
