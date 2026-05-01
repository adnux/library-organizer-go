# music-organizer (Go)

A compiled CLI tool that reorganizes a music folder into a configurable hierarchical structure based on embedded audio tags.

## Requirements

- Go 1.22+ (to build from source)
- `ffprobe` + `ffmpeg` — used to read/write embedded audio tags

### Install Go

**macOS**
```bash
brew install go
```

**Linux (Debian / Ubuntu)**
```bash
sudo apt update && sudo apt install -y golang-go
# or download from: https://go.dev/dl/
```

**Windows**
```powershell
winget install GoLang.Go
# or download from: https://go.dev/dl/
```

### Install ffmpeg (includes ffprobe)

**macOS**
```bash
brew install ffmpeg
```

**Linux (Debian / Ubuntu)**
```bash
sudo apt update && sudo apt install -y ffmpeg
```

**Windows**
```powershell
winget install ffmpeg
# or: choco install ffmpeg
```

## Build

**macOS / Linux**
```bash
cd organizer-go
go build -o organize .
```

**Windows**
```powershell
cd organizer-go
go build -o organize.exe .
```

The resulting binary is self-contained and requires no runtime dependencies other than `ffprobe`/`ffmpeg` being on `PATH`.

## Configuration

The root folder is resolved in this priority order:

1. `-root` flag (explicit override)
2. `MUSIC_ROOT` in a `.env` file (binary directory, then current directory)
3. Current working directory

**macOS / Linux**
```bash
cp .env.example .env
# Edit .env:
# MUSIC_ROOT=~/Music/Electronic
```

**Windows**
```powershell
copy .env.example .env
# Edit .env:
# MUSIC_ROOT=C:\Users\YourName\Music\Electronic
```

## Usage

**macOS / Linux**
```bash
# Dry-run — prints what would happen, no files touched (default)
./organize

# Execute — actually move the files
./organize -execute

# Custom root folder
./organize -root /Volumes/MyDrive/Electronic -execute

# Custom folder structure (pipe-separated tokens)
./organize -structure "Genre|Year|Artist"
./organize -structure "Artist|Year" -execute

# Only reorganize files directly in root (skip subfolders)
./organize -only-root
./organize -only-root -execute

# Real-world example: sort Electronic library by Genre → Artist → Year
./music-organizer -root ~/Music/Electronic -structure "Genre|Artist|Year" -execute

# Full example: organize your music library by genre, then artist, then year
./music-organizer -root ~/Music/Electronic -structure "Genre|Artist|Year" -execute

# Fix malformed DATE tags (normalize to 4-digit year)
./organize -fixYears            # preview
./organize -fixYears -execute   # apply

# Flatten — move all files directly into root, removing subfolders
./organize -flatten             # preview
./organize -flatten -execute    # apply
```

**Windows**
```powershell
# Dry-run
.\organize.exe

# Execute
.\organize.exe -execute

# Custom root folder
.\organize.exe -root "D:\Music\Electronic" -execute

# Only reorganize files in root (skip subfolders)
.\organize.exe -only-root
.\organize.exe -only-root -execute

# Custom folder structure
.\organize.exe -structure "Genre|Year|Artist"

# Fix malformed DATE tags
.\organize.exe -fixYears
.\organize.exe -fixYears -execute

# Flatten
.\organize.exe -flatten
.\organize.exe -flatten -execute
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-root PATH` | `.env` `MUSIC_ROOT` or CWD | Root folder to operate on |
| `-execute` | dry-run | Apply changes (without this flag nothing is written) |
| `-structure TOKENS` | `Year\|Genre\|Artist\|Month` | Pipe-separated folder hierarchy |
| `-only-root` | — | Only process files directly in root; skip subdirectories |
| `-fixYears` | — | Normalize DATE tags to 4-digit year instead of reorganizing |
| `-flatten` | — | Move all music files to root, removing all subfolders |

### `-structure` tokens

Tokens are case-insensitive and can be combined in any order:

| Token | Description |
|---|---|
| `Year` | Release year (4-digit) |
| `Month` | Release month (zero-padded, `01`–`12`) |
| `Genre` | Normalized genre name |
| `Artist` | Track artist (or `Various Artists` for compilations) |

Examples:
```
Year|Genre|Artist|Month   → 2025/Melodic House & Techno/Massano/04/
Genre|Year|Artist         → Melodic House & Techno/2025/Massano/
Artist|Year               → Massano/2025/
```

## Source files

| File | Purpose |
|---|---|
| `main.go` | CLI entry point; flag parsing and dispatch |
| `metadata.go` | `ffprobe` tag reading, genre/artist normalization, folder-name heuristics |
| `organizer.go` | File walker, move planner, executor, `--structure` parsing |
| `fix_years.go` | DATE tag scanner and rewriter (`--fixYears`) |
| `flatten.go` | Flatten logic (`--flatten`) |

## How it works

1. **Reads embedded tags** via `ffprobe` (`artist`, `album_artist`, `genre`, `date`, `TDOR`/`TRDA`)
2. **Falls back to folder-name heuristics** for files with missing/incomplete tags:
   - `[YYYY] Artist - Album` → year + artist
   - `Beatport Top 100 YYYY-MM Month YYYY` → year + month
   - `Beatport Best New <Genre> [Month YYYY]` → genre + date
   - `Singles week NN YYYY` / `WK05 2026` → ISO week converted to month
   - Scene format `VA-Release_Name-WEB-YYYY-GROUP` → year
3. **Normalizes genres** — compound Beatport tags like `edm;edm:techno:festival:melodic` resolve to the most specific known genre (`Melodic House & Techno`)
4. **No overwrites** — existing files at the target path are always skipped
5. **Cross-filesystem moves** — falls back to copy+delete if `os.Rename` fails
6. **Cleans up** empty directories after moving

## Conventions

| Value | Folder name |
|---|---|
| Unknown month | `00` |
| Unknown year | `Unknown` |
| Unknown genre | `Unknown` |
| VA / Various Artists | `Various Artists` |

Non-music files (`.nfo`, `.sfv`, `.m3u`, `.jpg`, `.crdownload`, etc.) are ignored and left in place.
