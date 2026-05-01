package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const workerCount = 5

var (
	musicExts = map[string]bool{
		".flac": true, ".mp3": true, ".aiff": true,
		".aif": true, ".wav": true, ".m4a": true,
	}

	validTokens = map[string]bool{
		"year": true, "month": true, "genre": true, "artist": true,
	}

	defaultStructure = []string{"year", "genre", "artist", "month"}
)

func parseStructure(s string) ([]string, error) {
	parts := strings.Split(s, "|")
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.ToLower(strings.TrimSpace(p))
		if t == "" {
			continue
		}
		if !validTokens[t] {
			return nil, fmt.Errorf("unknown structure token %q (valid: Year, Month, Genre, Artist)", p)
		}
		tokens = append(tokens, t)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("structure must have at least one token")
	}
	return tokens, nil
}

func metaField(meta trackMeta, token string) string {
	switch token {
	case "year":
		return meta.year
	case "month":
		return meta.month
	case "genre":
		return meta.genre
	case "artist":
		return meta.artist
	}
	return "Unknown"
}

type move struct {
	src string
	dst string
}

func organize(root string, execute bool, structure []string, onlyRoot bool) error {
	mode := "DRY-RUN"
	if execute {
		mode = "EXECUTE"
	}
	structureStr := make([]string, len(structure))
	for i, t := range structure {
		structureStr[i] = strings.Title(t)
	}
	fmt.Printf("%s\n  Music Organizer — %s\n  Root:      %s\n  Structure: %s\n%s\n\n",
		strings.Repeat("=", 80), mode, root,
		strings.Join(structureStr, " / "),
		strings.Repeat("=", 80))

	moves, err := buildPlan(root, structure, onlyRoot)
	if err != nil {
		return fmt.Errorf("building plan: %w", err)
	}

	printPlan(moves, root, structure, execute)

	if !execute {
		fmt.Println("\n  Run with --execute to apply changes.")
		return nil
	}

	return executePlan(moves, root)
}

func collectMusicFiles(root string, onlyRoot bool) ([]string, error) {
	if onlyRoot {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if musicExts[strings.ToLower(filepath.Ext(e.Name()))] {
				files = append(files, filepath.Join(root, e.Name()))
			}
		}
		return files, nil
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if musicExts[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func buildPlan(root string, structure []string, onlyRoot bool) ([]move, error) {
	files, err := collectMusicFiles(root, onlyRoot)
	if err != nil {
		return nil, fmt.Errorf("collecting files: %w", err)
	}
	fmt.Printf("  Found %d music files. Resolving metadata with %d workers...\n\n",
		len(files), workerCount)

	type result struct {
		mv move
		ok bool
	}

	jobs    := make(chan string, len(files))
	results := make(chan result, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				meta := resolveMetadata(path)

				parts := make([]string, 0, len(structure)+2)
				parts = append(parts, root)
				for _, token := range structure {
					parts = append(parts, metaField(meta, token))
				}
				parts = append(parts, filepath.Base(path))
				dst := filepath.Join(parts...)

				if path != dst {
					results <- result{mv: move{src: path, dst: dst}, ok: true}
				} else {
					results <- result{ok: false}
				}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var moves []move
	for r := range results {
		if r.ok {
			moves = append(moves, r.mv)
		}
	}

	sort.Slice(moves, func(i, j int) bool {
		return moves[i].src < moves[j].src
	})

	return moves, nil
}

func printPlan(moves []move, root string, structure []string, execute bool) {
	verb := "WOULD MOVE"
	if execute {
		verb = "MOVE"
	}

	for _, m := range moves {
		srcRel, _ := filepath.Rel(root, m.src)
		dstRel, _ := filepath.Rel(root, m.dst)
		fmt.Printf("  %s: %s\n         → %s\n", verb, srcRel, dstRel)
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 80))
	fmt.Printf("  Total files to move: %d\n", len(moves))

	for i, token := range structure {
		counts := map[string]int{}
		for _, m := range moves {
			rel, _ := filepath.Rel(root, m.dst)
			parts := strings.SplitN(rel, string(filepath.Separator), len(structure)+2)
			if i < len(parts) {
				counts[parts[i]]++
			}
		}
		fmt.Printf("  By %-8s: %s\n", strings.Title(token), formatCounter(counts))
	}
}

func executePlan(moves []move, root string) error {
	fmt.Println("\n  Moving files...")
	moved, skipped := 0, 0

	for _, m := range moves {
		if _, err := os.Stat(m.dst); err == nil {
			fmt.Printf("  SKIP (exists): %s\n", filepath.Base(m.dst))
			skipped++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(m.dst), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", filepath.Dir(m.dst), err)
		}

		if err := os.Rename(m.src, m.dst); err != nil {
			// Rename fails across filesystems; fall back to copy+delete.
			if err2 := copyFile(m.src, m.dst); err2 != nil {
				return fmt.Errorf("moving %s: %w", m.src, err2)
			}
			os.Remove(m.src)
		}
		moved++
	}

	fmt.Printf("\n  Done. Moved: %d  |  Skipped: %d\n", moved, skipped)

	fmt.Println("  Cleaning up empty source folders...")
	removed := removeEmptyDirs(root)
	fmt.Printf("  Removed %d empty directories.\n", removed)

	return nil
}

func removeEmptyDirs(root string) int {
	removed := 0

	var dirs []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})

	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))

	for _, dir := range dirs {
		rel, _ := filepath.Rel(root, dir)
		if err := os.Remove(dir); err == nil { // only succeeds if empty
			fmt.Printf("  Removed empty dir: %s\n", rel)
			removed++
		}
	}

	return removed
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func formatCounter(m map[string]int) string {
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].v > pairs[j].v
	})
	var parts []string
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("%s(%d)", p.k, p.v))
	}
	return strings.Join(parts, "  ")
}
