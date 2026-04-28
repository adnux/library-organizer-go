package main

import (
	"testing"
)

// ─── parseStructure ───────────────────────────────────────────────────────────

func TestParseStructure(t *testing.T) {
	valid := []struct {
		input string
		want  []string
	}{
		{"Year|Genre|Artist|Month", []string{"year", "genre", "artist", "month"}},
		{"Genre|Artist", []string{"genre", "artist"}},
		{"year", []string{"year"}},
		{"Year | Genre", []string{"year", "genre"}}, // whitespace trimmed
	}
	for _, tc := range valid {
		got, err := parseStructure(tc.input)
		if err != nil {
			t.Errorf("parseStructure(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("parseStructure(%q) len=%d; want %d", tc.input, len(got), len(tc.want))
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("parseStructure(%q)[%d] = %q; want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}

	invalid := []string{
		"Invalid",
		"",
		"Year|BadToken|Month",
	}
	for _, input := range invalid {
		_, err := parseStructure(input)
		if err == nil {
			t.Errorf("parseStructure(%q) expected error, got nil", input)
		}
	}
}

// ─── metaField ───────────────────────────────────────────────────────────────

func TestMetaField(t *testing.T) {
	meta := trackMeta{year: "2025", month: "04", genre: "House", artist: "Massano"}
	cases := []struct {
		token string
		want  string
	}{
		{"year", "2025"},
		{"month", "04"},
		{"genre", "House"},
		{"artist", "Massano"},
		{"unknown", "Unknown"},
	}
	for _, tc := range cases {
		got := metaField(meta, tc.token)
		if got != tc.want {
			t.Errorf("metaField(meta, %q) = %q; want %q", tc.token, got, tc.want)
		}
	}
}
