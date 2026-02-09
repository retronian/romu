# romu

ROM collection manager. Scan, catalog, and identify your ROM files using No-Intro DAT databases.

## Install

```bash
go install github.com/retronian/romu/cmd/romu@latest
```

Or build from source:

```bash
git clone https://github.com/retronian/romu.git
cd romu
go build -o romu ./cmd/romu/
```

## Usage

### Scan ROMs

Scan a directory for ROM files. ROMs are detected by folder name (`fc/`, `sfc/`, `gb/`, `gba/`, `md/`, `ps1/`, etc.) and file extension.

```bash
romu scan /path/to/roms
```

Expected directory structure:
```
roms/
├── fc/          # Famicom / NES (.nes)
├── sfc/         # Super Famicom / SNES (.sfc, .smc)
├── gb/          # Game Boy (.gb)
├── gbc/         # Game Boy Color (.gbc)
├── gba/         # Game Boy Advance (.gba)
├── md/          # Mega Drive / Genesis (.md, .bin)
├── ps1/         # PlayStation (.bin, .cue, .img, .iso)
├── n64/         # Nintendo 64 (.n64, .z64, .v64)
├── nds/         # Nintendo DS (.nds)
└── pce/         # PC Engine (.pce)
```

### List ROMs

```bash
romu list
```

### Import No-Intro DAT

Import a No-Intro DAT file (XML format) to register game metadata:

```bash
romu import-dat "Nintendo - Game Boy Advance (20240101-000000).dat"
```

Platform is auto-detected from the DAT header. Override with `--platform`:

```bash
romu import-dat mydat.dat --platform GBA
```

### Match ROMs to Games

After scanning ROMs and importing DAT files, match them by hash (SHA1 > MD5 > CRC32):

```bash
romu match "Nintendo - Game Boy Advance (20240101-000000).dat"
```

## Data

Database is stored at `~/.romu/romu.db` (SQLite).

## Supported Platforms

| Code | Platform |
|------|----------|
| FC   | Famicom / NES |
| SFC  | Super Famicom / SNES |
| GB   | Game Boy |
| GBC  | Game Boy Color |
| GBA  | Game Boy Advance |
| MD   | Mega Drive / Genesis |
| PS1  | PlayStation |
| N64  | Nintendo 64 |
| NDS  | Nintendo DS |
| PCE  | PC Engine |
| GG   | Game Gear |
| SMS  | Master System |
| WS   | WonderSwan |
| WSC  | WonderSwan Color |
| NGP  | Neo Geo Pocket |
| MSX  | MSX |

## License

MIT
