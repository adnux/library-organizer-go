package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// yearIssue describes a file whose DATE tag needs fixing.
type yearIssue struct {
	path       string
	currentVal string // raw current tag value (empty = missing)
	fixedYear  string // 4-digit year to write
	reason     string // human-readable explanation
}

var reFourDigitYear = regexp.MustCompile(`^(19|20)\d{2}$`)
var reYearPrefix = regexp.MustCompile(`^(19|20)\d{2}`)
var reFolderYear = regexp.MustCompile(`^(19|20)\d{2}$`)

// fixYears scans root for files with non-standard DATE tags and either
// prints a report (dry-run) or rewrites the tag via ffmpeg.
func fixYears(root string, execute bool) error {
	mode := "DRY-RUN"
	if execute {
		mode = "EXECUTE"
	}
	fmt.Printf("%s\n  Fix Years — %s\n  Root: %s\n%s\n\n",
		strings.Repeat("=", 80), mode, root, strings.Repeat("=", 80))

	issues, err := scanYearIssues(root)
	if err != nil {
		return fmt.Errorf("scanning files: %w", err)
	}

	if len(issues) == 0 {
		fmt.Println("  All DATE tags are already in YYYY format. Nothing to do.")
		return nil
	}

	// Print plan grouped by reason
	byReason := map[string][]yearIssue{}
	for _, iss := range issues {
		byReason[iss.reason] = append(byReason[iss.reason], iss)
	}
	for reason, items := range byReason {
		fmt.Printf("  ── %s (%d files) ──\n", reason, len(items))
		for _, iss := range items {
			rel, _ := filepath.Rel(root, iss.path)
			current := iss.currentVal
			if current == "" {
				current = "(missing)"
			}
			fmt.Printf("    %-72s  %s  →  %s\n", rel, current, iss.fixedYear)
		}
		fmt.Println()
	}

	fmt.Printf("%s\n  Total files to fix: %d\n", strings.Repeat("─", 80), len(issues))

	if !execute {
		fmt.Println("\n  Run with --fixYears --execute to apply changes.")
		return nil
	}

	// Execute
	fmt.Println("\n  Rewriting tags...")
	fixed, skipped, errs := 0, 0, 0
	for _, iss := range issues {
		if err := rewriteDateTag(iss.path, iss.fixedYear); err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %s: %v\n", filepath.Base(iss.path), err)
			errs++
			continue
		}
		rel, _ := filepath.Rel(root, iss.path)
		fmt.Printf("  FIXED: %s  →  date=%s\n", rel, iss.fixedYear)
		fixed++
		_ = skipped
	}

	fmt.Printf("\n  Done. Fixed: %d  |  Errors: %d\n", fixed, errs)
	return nil
}

// scanYearIssues walks root and returns every file whose DATE tag is not
// already a clean 4-digit year.
func scanYearIssues(root string) ([]yearIssue, error) {
	var issues []yearIssue

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !musicExts[ext] {
			return nil
		}

		tags := getTags(path)
		dateVal := tags["date"]

		switch {
		case reFourDigitYear.MatchString(dateVal):
			// Already correct — skip.
			return nil

		case reYearPrefix.MatchString(dateVal):
			// Starts with a valid year (e.g. "2025-10-10", "2025-01") — truncate.
			year := dateVal[:4]
			issues = append(issues, yearIssue{
				path:       path,
				currentVal: dateVal,
				fixedYear:  year,
				reason:     "truncate full date to year",
			})

		default:
			// Garbage value (URL, random string) or missing — recover from
			// tdor/trda tags first, then fall back to folder year.
			year := recoverYear(path, root, tags)
			if year == "" {
				return nil // cannot determine year, skip
			}
			reason := "replace garbage/missing with folder year"
			if tags["tdor"] != "" || tags["trda"] != "" {
				reason = "replace garbage/missing with tdor/trda year"
			}
			issues = append(issues, yearIssue{
				path:       path,
				currentVal: dateVal,
				fixedYear:  year,
				reason:     reason,
			})
		}

		return nil
	})

	return issues, err
}

// recoverYear finds the best available year for a file with a bad/missing
// DATE tag. Priority: tdor → trda → folder path segment.
func recoverYear(filePath, root string, tags map[string]string) string {
	for _, key := range []string{"tdor", "trda"} {
		if val := tags[key]; val != "" && reYearPrefix.MatchString(val) {
			return val[:4]
		}
	}

	// Fall back to the year-named top-level folder (e.g. .../Electronic/2024/…)
	rel, err := filepath.Rel(root, filePath)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) > 0 && reFolderYear.MatchString(parts[0]) {
		return parts[0]
	}

	return ""
}

// rewriteDateTag uses ffmpeg to overwrite the DATE tag in-place.
// It writes to a temp file then atomically replaces the original.
func rewriteDateTag(filePath, year string) error {
	tmp := filePath + ".tmp" + filepath.Ext(filePath)

	args := []string{
		"-i", filePath,
		"-map_metadata", "0",  // copy all existing tags
		"-metadata", "date=" + year,
		"-codec", "copy",      // no re-encoding
		"-y",                  // overwrite temp if exists
		tmp,
	}

	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %w\n%s", err, string(out))
	}

	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("replacing original: %w", err)
	}

	return nil
}
