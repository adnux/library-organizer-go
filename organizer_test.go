package main

import (
"os"
"path/filepath"
"testing"
)

// ─── parseStructure ──────────────────────────────────────────────────────────

func TestParseStructure(t *testing.T) {
valid := []struct {
input string
want  []string
}{
{"Year|Genre|Artist|Month", []string{"year", "genre", "artist", "month"}},
{"Genre|Artist", []string{"genre", "artist"}},
{"year", []string{"year"}},
{"Year | Genre", []string{"year", "genre"}},
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

invalid := []string{"Invalid", "", "Year|BadToken|Month"}
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

// ─── collectMusicFiles ───────────────────────────────────────────────────────

func TestCollectMusicFiles(t *testing.T) {
root := t.TempDir()

rootFlac := filepath.Join(root, "root_track.flac")
rootMp3  := filepath.Join(root, "root_track.mp3")
sub      := filepath.Join(root, "subdir")
if err := os.Mkdir(sub, 0o755); err != nil {
t.Fatal(err)
}
subFlac := filepath.Join(sub, "sub_track.flac")
txtFile := filepath.Join(root, "info.nfo")

for _, f := range []string{rootFlac, rootMp3, subFlac, txtFile} {
if err := os.WriteFile(f, []byte{}, 0o644); err != nil {
t.Fatal(err)
}
}

t.Run("onlyRoot=false collects all music files recursively", func(t *testing.T) {
files, err := collectMusicFiles(root, false)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(files) != 3 {
t.Errorf("got %d files; want 3", len(files))
}
})

t.Run("onlyRoot=true collects only root-level music files", func(t *testing.T) {
files, err := collectMusicFiles(root, true)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(files) != 2 {
t.Errorf("got %d files; want 2", len(files))
}
for _, f := range files {
if filepath.Dir(f) != root {
t.Errorf("file %q is not directly in root %q", f, root)
}
}
})

t.Run("onlyRoot=true excludes non-music files", func(t *testing.T) {
files, err := collectMusicFiles(root, true)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
for _, f := range files {
if filepath.Ext(f) == ".nfo" {
t.Errorf("non-music file %q should not be included", f)
}
}
})
}
