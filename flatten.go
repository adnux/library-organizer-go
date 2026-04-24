package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// flatten moves all music files directly into root, removing the folder hierarchy.
func flatten(root string, execute bool) error {
	mode := "DRY-RUN"
	if execute {
		mode = "EXECUTE"
	}
	fmt.Printf("%s\n  Music Organizer — FLATTEN (%s)\n  Root: %s\n%s\n\n",
		strings.Repeat("=", 80), mode, root, strings.Repeat("=", 80))

	type flatMove struct {
		src      string
		dst      string
		conflict bool
	}

	var moves []flatMove

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Skip files already at the root level
		if filepath.Dir(path) == root {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !musicExts[ext] {
			return nil
		}

		dst := filepath.Join(root, filepath.Base(path))
		_, statErr := os.Stat(dst)
		moves = append(moves, flatMove{
			src:      path,
			dst:      dst,
			conflict: statErr == nil, // target already exists
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking root: %w", err)
	}

	verb := "WOULD MOVE"
	if execute {
		verb = "MOVE"
	}

	conflicts := 0
	for _, m := range moves {
		srcRel, _ := filepath.Rel(root, m.src)
		if m.conflict {
			conflicts++
			fmt.Printf("  CONFLICT (skip): %s → %s\n", srcRel, filepath.Base(m.dst))
		} else {
			fmt.Printf("  %s: %s\n         → %s\n", verb, srcRel, filepath.Base(m.dst))
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 80))
	fmt.Printf("  Files to move:  %d\n", len(moves)-conflicts)
	fmt.Printf("  Conflicts skip: %d\n", conflicts)

	if !execute {
		fmt.Println("\n  Run with --flatten --execute to apply changes.")
		return nil
	}

	fmt.Println("\n  Moving files...")
	moved, skipped := 0, 0
	for _, m := range moves {
		if m.conflict {
			skipped++
			continue
		}
		// Re-check in case a duplicate filename appeared mid-run
		if _, err := os.Stat(m.dst); err == nil {
			fmt.Printf("  SKIP (exists): %s\n", filepath.Base(m.dst))
			skipped++
			continue
		}
		if err := os.Rename(m.src, m.dst); err != nil {
			if err2 := copyFile(m.src, m.dst); err2 != nil {
				return fmt.Errorf("moving %s: %w", m.src, err2)
			}
			os.Remove(m.src)
		}
		moved++
	}

	fmt.Printf("\n  Done. Moved: %d  |  Skipped: %d\n", moved, skipped)

	fmt.Println("  Cleaning up empty directories...")
	removed := removeEmptyDirs(root)
	fmt.Printf("  Removed %d empty directories.\n", removed)

	return nil
}

