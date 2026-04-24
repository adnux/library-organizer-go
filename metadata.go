package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Genre normalization
// ---------------------------------------------------------------------------

type genreRule struct {
	pattern *regexp.Regexp
	name    string
}

// Ordered most-specific first — first match wins.
var genreRules = []genreRule{
	{regexp.MustCompile(`(?i)melodic.h.?&.?t|melodic.house.and.techno|melodic.house.&.techno|melodic.*house.*techno|techno.*melodic`), "Melodic House & Techno"},
	{regexp.MustCompile(`(?i)indie.dance`), "Indie Dance"},
	{regexp.MustCompile(`(?i)techno.peak.time|peak.time.*driv|driving.*techno|techno.*driving`), "Techno"},
	{regexp.MustCompile(`(?i)\btechno\b`), "Techno"},
	{regexp.MustCompile(`(?i)drum.&.bass|dnb`), "Drum & Bass"},
	{regexp.MustCompile(`(?i)\btrance\b`), "Trance"},
	{regexp.MustCompile(`(?i)\bhouse\b`), "House"},
	{regexp.MustCompile(`(?i)\belectronic\b`), "Electronic"},
	{regexp.MustCompile(`(?i)\bedm\b`), "Electronic"},
	{regexp.MustCompile(`(?i)electronica`), "Electronic"},
	{regexp.MustCompile(`(?i)\bdance\b`), "Dance"},
	{regexp.MustCompile(`(?i)electro`), "Electro"},
	{regexp.MustCompile(`(?i)\bpop\b`), "Pop"},
}

// normalizeGenre picks the most specific known genre from a raw (possibly
// compound) tag value. Compound tags like "edm;edm:techno:festival:melodic"
// are split on ; : , before matching.
func normalizeGenre(raw string) string {
	if raw == "" {
		return ""
	}
	segments := regexp.MustCompile(`[;:,]`).Split(raw, -1)
	for _, rule := range genreRules {
		for _, seg := range segments {
			if rule.pattern.MatchString(strings.TrimSpace(seg)) {
				return rule.name
			}
		}
	}
	return strings.TrimSpace(raw)
}

func normalizeArtist(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "Various Artists"
	}
	if strings.EqualFold(s, "va") {
		return "Various Artists"
	}
	if strings.HasPrefix(strings.ToLower(s), "various") {
		return "Various Artists"
	}
	// Multiple artists separated by commas → treat as compilation
	if strings.Count(s, ",") >= 1 {
		return "Various Artists"
	}
	return s
}

// ---------------------------------------------------------------------------
// ffprobe tag extraction
// ---------------------------------------------------------------------------

type ffprobeOutput struct {
	Format struct {
		Tags map[string]string `json:"tags"`
	} `json:"format"`
}

// getTags reads embedded audio tags from filepath via ffprobe.
// Returns a lowercased key map; empty map on any error.
func getTags(filepath string) map[string]string {
	out, err := exec.Command(
		"ffprobe", "-v", "quiet",
		"-print_format", "json",
		"-show_format", filepath,
	).Output()
	if err != nil {
		return map[string]string{}
	}

	var result ffprobeOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return map[string]string{}
	}

	lower := make(map[string]string, len(result.Format.Tags))
	for k, v := range result.Format.Tags {
		lower[strings.ToLower(k)] = v
	}
	return lower
}

// parseDateTag extracts (year, month) from embedded tags.
// month is 0 if only a year is found.
func parseDateTag(tags map[string]string) (year, month int) {
	dateRe := regexp.MustCompile(`(\d{4})(?:-(\d{2}))?`)
	for _, key := range []string{"tdor", "trda", "date", "year"} {
		val := tags[key]
		if val == "" {
			continue
		}
		m := dateRe.FindStringSubmatch(val)
		if m == nil {
			continue
		}
		y, _ := strconv.Atoi(m[1])
		mo := 0
		if m[2] != "" {
			mo, _ = strconv.Atoi(m[2])
		}
		return y, mo
	}
	return 0, 0
}

// ---------------------------------------------------------------------------
// Folder-name heuristics
// ---------------------------------------------------------------------------

var (
	monthNames = map[string]int{
		"january": 1, "february": 2, "march": 3, "april": 4,
		"may": 5, "june": 6, "july": 7, "august": 8,
		"september": 9, "october": 10, "november": 11, "december": 12,
		"jan": 1, "feb": 2, "mar": 3, "apr": 4,
		"jun": 6, "jul": 7, "aug": 8,
		"sep": 9, "sept": 9, "oct": 10, "nov": 11, "dec": 12,
	}

	reISOWeek      = regexp.MustCompile(`(?i)\bw(?:ee)?k\s*0*(\d{1,2})\b`)
	reExplicitDate = regexp.MustCompile(`(20\d{2})[-.](\d{2})`)
	reYear         = regexp.MustCompile(`\b(20\d{2}|19\d{2})\b`)
	reYearBracketed = regexp.MustCompile(`[\[\(](20\d{2}|19\d{2})[\]\)]`)

	// Artist extraction patterns
	reBracketedYear  = regexp.MustCompile(`^\[\d{4}\]\s*(.+?)\s*-\s*.+`)
	reArtistDashYear = regexp.MustCompile(`^(.+?)\s*-\s*20\d{2}\s*-\s*.+`)
	reYearDashArtist = regexp.MustCompile(`^20\d{2}\s*-\s*(.+?)\s*-\s*.+`)

	reCompilationPrefix = regexp.MustCompile(
		`(?i)^(beatport|va|various|serious|global|state|defected|drumcode|armada|nervous|above|group)`,
	)
	reVA = regexp.MustCompile(`(?i)^(VA|Various)`)
)

// weekToMonth converts an ISO year+week to the month of that week's Monday.
func weekToMonth(year, week int) int {
	// Jan 4 is always in ISO week 1.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	dow := int(jan4.Weekday())
	if dow == 0 {
		dow = 7 // Sunday → 7 in ISO
	}
	mondayW1 := jan4.AddDate(0, 0, 1-dow)
	target := mondayW1.AddDate(0, 0, (week-1)*7)
	return int(target.Month())
}

type folderInfo struct {
	year, month int
	genre       string
	artist      string
}

// parseFolderName infers release metadata from a single folder name segment.
func parseFolderName(name string) folderInfo {
	info := folderInfo{}

	// ISO week
	if m := reISOWeek.FindStringSubmatch(name); m != nil {
		if ym := reYear.FindStringSubmatch(name); ym != nil {
			y, _ := strconv.Atoi(ym[1])
			w, _ := strconv.Atoi(m[1])
			info.year = y
			info.month = weekToMonth(y, w)
		}
	}

	// Explicit YYYY-MM
	if info.month == 0 {
		if m := reExplicitDate.FindStringSubmatch(name); m != nil {
			info.year, _ = strconv.Atoi(m[1])
			info.month, _ = strconv.Atoi(m[2])
		}
	}

	// Month name
	if info.month == 0 {
		lower := strings.ToLower(name)
		// Try longest names first to avoid "mar" matching "march"
		best := ""
		for mn := range monthNames {
			re := regexp.MustCompile(`(?i)\b` + mn + `\b`)
			if re.MatchString(lower) && len(mn) > len(best) {
				best = mn
			}
		}
		if best != "" {
			info.month = monthNames[best]
			if ym := reYear.FindStringSubmatch(name); ym != nil && info.year == 0 {
				info.year, _ = strconv.Atoi(ym[1])
			}
		}
	}

	// Year only
	if info.year == 0 {
		if m := reYearBracketed.FindStringSubmatch(name); m != nil {
			info.year, _ = strconv.Atoi(m[1])
		} else if m := reYear.FindStringSubmatch(name); m != nil {
			info.year, _ = strconv.Atoi(m[1])
		}
	}

	// Genre
	for _, rule := range genreRules {
		if rule.pattern.MatchString(name) {
			info.genre = rule.name
			break
		}
	}

	// Artist
	if m := reBracketedYear.FindStringSubmatch(name); m != nil {
		info.artist = strings.TrimSpace(m[1])
	}
	if info.artist == "" {
		if m := reArtistDashYear.FindStringSubmatch(name); m != nil {
			if !reVA.MatchString(m[1]) {
				info.artist = strings.TrimSpace(m[1])
			}
		}
	}
	if info.artist == "" {
		if m := reYearDashArtist.FindStringSubmatch(name); m != nil {
			if !reCompilationPrefix.MatchString(m[1]) {
				info.artist = strings.TrimSpace(m[1])
			}
		}
	}

	return info
}

// ---------------------------------------------------------------------------
// Metadata resolver
// ---------------------------------------------------------------------------

type trackMeta struct {
	year, month, genre, artist string
}

// resolveMetadata determines the target year/month/genre/artist for a file.
// It reads embedded tags first, then walks up the folder tree for fallbacks.
func resolveMetadata(filePath string) trackMeta {
	tags := getTags(filePath)

	year, month := parseDateTag(tags)
	artist := normalizeArtist(firstOf(tags["album_artist"], tags["artist"]))
	genre := normalizeGenre(tags["genre"])

	// Walk up folder segments (closest parent first)
	parts := strings.Split(filePath, "/")
	for i := len(parts) - 2; i >= 0; i-- {
		if year != 0 && month != 0 && genre != "" && artist != "Various Artists" {
			break
		}
		fi := parseFolderName(parts[i])
		if year == 0 && fi.year != 0 {
			year = fi.year
		}
		if month == 0 && fi.month != 0 {
			month = fi.month
		}
		if genre == "" && fi.genre != "" {
			genre = fi.genre
		}
		if artist == "Various Artists" && fi.artist != "" {
			artist = normalizeArtist(fi.artist)
		}
	}

	yearStr := "Unknown"
	if year != 0 {
		yearStr = strconv.Itoa(year)
	}
	monthStr := "00"
	if month != 0 {
		monthStr = fmt.Sprintf("%02d", month)
	}
	if genre == "" {
		genre = "Unknown"
	}
	if artist == "" {
		artist = "Various Artists"
	}

	return trackMeta{year: yearStr, month: monthStr, genre: genre, artist: artist}
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
