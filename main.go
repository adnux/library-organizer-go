package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadEnvRoot() string {
	exe, _ := os.Executable()
	candidates := []string{
		filepath.Join(filepath.Dir(exe), ".env"),
		filepath.Join(".", ".env"),
	}

	for _, envPath := range candidates {
		f, err := os.Open(envPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			key, val, _ := strings.Cut(line, "=")
			if strings.TrimSpace(key) == "MUSIC_ROOT" {
				val = strings.TrimSpace(val)
				val = strings.Trim(val, `"'`)
				// Expand ~ manually
				if strings.HasPrefix(val, "~/") {
					home, _ := os.UserHomeDir()
					val = filepath.Join(home, val[2:])
				}
				f.Close()
				abs, err := filepath.Abs(val)
				if err == nil {
					return abs
				}
				return val
			}
		}
		f.Close()
	}

	cwd, _ := os.Getwd()
	return cwd
}

func main() {
	defaultRoot := loadEnvRoot()
	defaultStructureStr := strings.Join(defaultStructure, "|")

	root          := flag.String("root", defaultRoot, "")
	execute       := flag.Bool("execute", false, "")
	fixYearsFlag  := flag.Bool("fixYears", false, "")
	flattenFlag   := flag.Bool("flatten", false, "")
	structureFlag := flag.String("structure", defaultStructureStr, "")
	onlyRoot      := flag.Bool("only-root", false, "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `organize — Reorganize a music folder into a configurable folder structure.

USAGE
  organize [flags]

FLAGS
  -root PATH
        Root folder to operate on.
        Default: resolved from .env MUSIC_ROOT, or current directory.
        Currently: %s

  -execute
        Apply changes. Without this flag the tool runs in dry-run mode
        and only prints what it would do.

  -structure TOKENS
        Pipe-separated folder hierarchy (case-insensitive tokens).
        Default: Year|Genre|Artist|Month
        Valid tokens: Year, Month, Genre, Artist

  -only-root
        Only reorganize files directly in <root> — subfolders are not
        touched. Without this flag all files in all subfolders are
        processed (default behaviour).

  -fixYears
        Scan all files and rewrite malformed DATE tags (e.g. full dates
        like "2025-10-10", URLs) to a clean 4-digit year. Uses ffmpeg —
        no audio is re-encoded. Cannot be combined with -structure or
        -flatten.

  -flatten
        Move all music files directly into <root>, removing all
        subfolders. Useful for starting a fresh reorganization.

ROOT FOLDER RESOLUTION  (highest priority first)
  1. -root flag
  2. MUSIC_ROOT in a .env file (binary directory, then current directory)
  3. Current working directory

  Example .env:
    MUSIC_ROOT=~/Music/Electronic

STRUCTURE TOKENS
  Year    Release year (4-digit).           Unknown → "Unknown"
  Month   Release month (zero-padded 01–12). Unknown → "00"
  Genre   Normalized genre name.            Unknown → "Unknown"
  Artist  Track artist.            Compilations → "Various Artists"

EXAMPLES
  organize
      Dry-run with default structure: Year|Genre|Artist|Month

  organize -execute
      Move files for real.

  organize -only-root
      Dry-run, but only files directly in <root> (skip subfolders).

  organize -only-root -execute
      Move only root-level files for real.

  organize -structure "Genre|Year|Artist" -execute
      Reorganize as  <root>/Melodic House & Techno/2025/Massano/track.flac

  organize -structure "Artist|Year"
      Dry-run as  <root>/Massano/2025/track.flac

  organize -fixYears
      Preview DATE tag fixes (e.g. "https://djsoundtop.com" → "2024").

  organize -fixYears -execute
      Apply DATE tag fixes.

  organize -flatten
      Preview flattening all files into root.

  organize -flatten -execute
      Move all files to root and delete empty subfolders.

  organize -root /Volumes/MyDrive/Electronic -execute
      Operate on a different root folder.

CONVENTIONS
  Unknown month  → 00
  Unknown year   → Unknown
  Unknown genre  → Unknown
  VA / Various   → Various Artists

  Non-music files (.nfo, .sfv, .m3u, .jpg, .crdownload, etc.) are
  ignored and left in place.
`, defaultRoot)
	}

	flag.Parse()

	rootPath, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid root path: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: root folder does not exist: %s\n", rootPath)
		os.Exit(1)
	}

	if *fixYearsFlag {
		if err := fixYears(rootPath, *execute); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *flattenFlag {
		if err := flatten(rootPath, *execute); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	structure, err := parseStructure(*structureFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid --structure: %v\n", err)
		os.Exit(1)
	}

	if err := organize(rootPath, *execute, structure, *onlyRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

