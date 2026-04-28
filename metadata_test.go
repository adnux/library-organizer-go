package main

import (
	"testing"
)

// ─── normalizeGenre ───────────────────────────────────────────────────────────

func TestNormalizeGenre(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"", ""},
		{"Melodic House & Techno", "Melodic House & Techno"},
		{"Melodic H&T", "Melodic House & Techno"},
		{"melodic house and techno", "Melodic House & Techno"},
		{"melodic house & techno", "Melodic House & Techno"},
		{"Melodic Techno", "Techno"}, // "Melodic Techno" has no "house" — matches Techno rule
		{"Indie Dance", "Indie Dance"},
		{"Techno (Peak Time / Driving)", "Techno"},
		{"Driving Techno", "Techno"},
		{"Techno", "Techno"},
		{"Drum & Bass", "Drum & Bass"},
		{"DnB", "Drum & Bass"},
		{"Trance", "Trance"},
		{"Deep House", "House"},
		{"House", "House"},
		{"Electronic", "Electronic"},
		{"EDM", "Electronic"},
		{"Electronica", "Electronic"},
		{"Dance", "Dance"},
		{"Electro", "Electro"},
		{"Pop", "Pop"},
		{"Jazz", "Jazz"}, // unknown genre — passed through as-is
		// Rules iterate first, then segments — first rule that matches ANY segment wins
		{"Techno; House", "Techno"},  // Techno rule before House rule
		{"House, Techno", "Techno"},  // Techno rule fires on "Techno" segment before House rule fires
	}
	for _, tc := range cases {
		got := normalizeGenre(tc.raw)
		if got != tc.want {
			t.Errorf("normalizeGenre(%q) = %q; want %q", tc.raw, got, tc.want)
		}
	}
}

// ─── normalizeArtist ─────────────────────────────────────────────────────────

func TestNormalizeArtist(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"", "Various Artists"},
		{"VA", "Various Artists"},
		{"va", "Various Artists"},
		{"Various Artists", "Various Artists"},
		{"Various", "Various Artists"},
		{"various artists", "Various Artists"},
		{"Massano", "Massano"},
		{"Bicep", "Bicep"},
		{"  Bicep  ", "Bicep"}, // trimmed
		{"Agoria, Mooglie", "Various Artists"},
		{"Artist1, Artist2, Artist3", "Various Artists"},
	}
	for _, tc := range cases {
		got := normalizeArtist(tc.raw)
		if got != tc.want {
			t.Errorf("normalizeArtist(%q) = %q; want %q", tc.raw, got, tc.want)
		}
	}
}

// ─── parseDateTag ─────────────────────────────────────────────────────────────

func TestParseDateTag(t *testing.T) {
	cases := []struct {
		tags      map[string]string
		wantYear  int
		wantMonth int
	}{
		{map[string]string{"date": "2025"}, 2025, 0},
		{map[string]string{"date": "2025-04"}, 2025, 4},
		{map[string]string{"date": "2025-04-15"}, 2025, 4},
		{map[string]string{"tdor": "2023-07"}, 2023, 7},
		{map[string]string{"trda": "2022-12"}, 2022, 12},
		// tdor takes priority over date
		{map[string]string{"tdor": "2023-07", "date": "2024"}, 2023, 7},
		// year tag as fallback
		{map[string]string{"year": "2020"}, 2020, 0},
		// empty / missing
		{map[string]string{}, 0, 0},
		{map[string]string{"date": ""}, 0, 0},
		// garbage value
		{map[string]string{"date": "https://djsoundtop.com"}, 0, 0},
	}
	for _, tc := range cases {
		gotYear, gotMonth := parseDateTag(tc.tags)
		if gotYear != tc.wantYear || gotMonth != tc.wantMonth {
			t.Errorf("parseDateTag(%v) = (%d,%d); want (%d,%d)",
				tc.tags, gotYear, gotMonth, tc.wantYear, tc.wantMonth)
		}
	}
}

// ─── weekToMonth ─────────────────────────────────────────────────────────────
// Reference: Jan 4, 2024 = Thursday → ISO week 1 starts Jan 1, 2024.

func TestWeekToMonth(t *testing.T) {
	cases := []struct {
		year, week int
		want       int
	}{
		{2024, 1, 1},  // Jan 1
		{2024, 5, 1},  // Jan 29
		{2024, 6, 2},  // Feb 5
		{2024, 14, 4}, // Apr 1
		{2024, 26, 6}, // Jun 24
		{2024, 52, 12}, // Dec 23
	}
	for _, tc := range cases {
		got := weekToMonth(tc.year, tc.week)
		if got != tc.want {
			t.Errorf("weekToMonth(%d, %d) = %d; want %d", tc.year, tc.week, got, tc.want)
		}
	}
}

// ─── parseFolderName ─────────────────────────────────────────────────────────

func TestParseFolderName(t *testing.T) {
	cases := []struct {
		name       string
		wantYear   int
		wantMonth  int
		wantGenre  string
		wantArtist string
	}{
		// Explicit YYYY-MM date
		{"Beatport Top 100 2025-04", 2025, 4, "", ""},
		// Month name + year
		{"Beatport Best New Tracks April 2025", 2025, 4, "", ""},
		{"New Releases March 2024", 2024, 3, "", ""},
		// Bracketed year + artist
		{"[2024] Massano - Every Day", 2024, 0, "", "Massano"},
		// Artist - Year - Album pattern
		{"Massano - 2024 - Every Day", 2024, 0, "", "Massano"},
		// VA prefix → no artist extracted
		{"VA - 2025 - Compilation", 2025, 0, "", ""},
		// Genre from name
		{"Beatport Best New Melodic House & Techno April 2025", 2025, 4, "Melodic House & Techno", ""},
		{"Beatport Techno Top 100 2024", 2024, 0, "Techno", ""},
		// Plain year only
		{"Singles 2023", 2023, 0, "", ""},
		// No recognizable metadata
		{"Random Folder", 0, 0, "", ""},
	}
	for _, tc := range cases {
		got := parseFolderName(tc.name)
		if got.year != tc.wantYear {
			t.Errorf("parseFolderName(%q).year = %d; want %d", tc.name, got.year, tc.wantYear)
		}
		if got.month != tc.wantMonth {
			t.Errorf("parseFolderName(%q).month = %d; want %d", tc.name, got.month, tc.wantMonth)
		}
		if got.genre != tc.wantGenre {
			t.Errorf("parseFolderName(%q).genre = %q; want %q", tc.name, got.genre, tc.wantGenre)
		}
		if got.artist != tc.wantArtist {
			t.Errorf("parseFolderName(%q).artist = %q; want %q", tc.name, got.artist, tc.wantArtist)
		}
	}
}

// ─── recoverYear ─────────────────────────────────────────────────────────────

func TestRecoverYear(t *testing.T) {
	cases := []struct {
		filePath, root string
		tags           map[string]string
		want           string
	}{
		// From tdor tag
		{"/root/Unknown/Artist/file.flac", "/root", map[string]string{"tdor": "2022-05"}, "2022"},
		// From trda tag
		{"/root/Unknown/Artist/file.flac", "/root", map[string]string{"trda": "2021"}, "2021"},
		// tdor takes priority over folder year
		{"/root/2024/Artist/file.flac", "/root", map[string]string{"tdor": "2023-01"}, "2023"},
		// From year-named top-level folder
		{"/root/2024/Artist/file.flac", "/root", map[string]string{}, "2024"},
		// Non-year top-level folder → no result
		{"/root/Unknown/Artist/file.flac", "/root", map[string]string{}, ""},
	}
	for _, tc := range cases {
		got := recoverYear(tc.filePath, tc.root, tc.tags)
		if got != tc.want {
			t.Errorf("recoverYear(%q, %q, %v) = %q; want %q",
				tc.filePath, tc.root, tc.tags, got, tc.want)
		}
	}
}

// ─── firstOf ─────────────────────────────────────────────────────────────────

func TestFirstOf(t *testing.T) {
	cases := []struct {
		vals []string
		want string
	}{
		{[]string{"a", "b", "c"}, "a"},
		{[]string{"", "b", "c"}, "b"},
		{[]string{"", "", "c"}, "c"},
		{[]string{"", "", ""}, ""},
		{[]string{}, ""},
	}
	for _, tc := range cases {
		got := firstOf(tc.vals...)
		if got != tc.want {
			t.Errorf("firstOf(%v) = %q; want %q", tc.vals, got, tc.want)
		}
	}
}
