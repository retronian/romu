package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

type RomFile struct {
	ID        int64
	Path      string
	Filename  string
	Size      int64
	HashCRC32 string
	HashMD5   string
	HashSHA1  string
	Platform  string
	GameID    *int64
	TitleEN   *string // joined from games
	TitleJA   *string // joined from games
}

type Game struct {
	ID          int64
	TitleEN     string
	Platform    string
	Developer   string
	Publisher   string
	ReleaseDate string
}

func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".romu")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "romu.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY,
		title_en TEXT,
		title_ja TEXT,
		description_ja TEXT,
		platform TEXT NOT NULL,
		developer TEXT,
		publisher TEXT,
		release_date TEXT,
		genre TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS rom_files (
		id INTEGER PRIMARY KEY,
		path TEXT NOT NULL UNIQUE,
		filename TEXT NOT NULL,
		size INTEGER,
		hash_crc32 TEXT,
		hash_md5 TEXT,
		hash_sha1 TEXT,
		platform TEXT NOT NULL,
		game_id INTEGER REFERENCES games(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS cover_arts (
		id INTEGER PRIMARY KEY,
		game_id INTEGER REFERENCES games(id),
		image_type TEXT NOT NULL,
		file_path TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_rom_files_crc32 ON rom_files(hash_crc32);
	CREATE INDEX IF NOT EXISTS idx_rom_files_md5 ON rom_files(hash_md5);
	CREATE INDEX IF NOT EXISTS idx_rom_files_sha1 ON rom_files(hash_sha1);
	CREATE INDEX IF NOT EXISTS idx_games_platform ON games(platform);
	`
	_, err := db.Exec(schema)
	return err
}

func (d *DB) UpsertRomFile(path, filename string, size int64, crc32, md5, sha1, platform string) error {
	_, err := d.Exec(`
		INSERT INTO rom_files (path, filename, size, hash_crc32, hash_md5, hash_sha1, platform, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			filename=excluded.filename, size=excluded.size,
			hash_crc32=excluded.hash_crc32, hash_md5=excluded.hash_md5, hash_sha1=excluded.hash_sha1,
			platform=excluded.platform, updated_at=CURRENT_TIMESTAMP
	`, path, filename, size, crc32, md5, sha1, platform)
	return err
}

func (d *DB) ListRomFiles() ([]RomFile, error) {
	rows, err := d.Query(`
		SELECT r.id, r.path, r.filename, r.size, r.hash_crc32, r.hash_md5, r.hash_sha1, r.platform, r.game_id, g.title_en, g.title_ja
		FROM rom_files r LEFT JOIN games g ON r.game_id = g.id
		ORDER BY r.platform, r.filename
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []RomFile
	for rows.Next() {
		var f RomFile
		if err := rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.HashCRC32, &f.HashMD5, &f.HashSHA1, &f.Platform, &f.GameID, &f.TitleEN, &f.TitleJA); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

func (d *DB) InsertGame(titleEN, platform, crc32, md5, sha1 string, size int64) (int64, error) {
	res, err := d.Exec(`
		INSERT INTO games (title_en, platform) VALUES (?, ?)
	`, titleEN, platform)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpsertGameFromDAT(titleEN, platform, crc32, md5, sha1 string, size int64) error {
	// Check if game already exists with same title and platform
	var id int64
	err := d.QueryRow(`SELECT id FROM games WHERE title_en = ? AND platform = ?`, titleEN, platform).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = d.Exec(`INSERT INTO games (title_en, platform) VALUES (?, ?)`, titleEN, platform)
	}
	return err
}

// ImportDATGame stores a game from DAT along with its ROM hash info for later matching
type DATRom struct {
	GameTitle string
	Platform  string
	CRC32     string
	MD5       string
	SHA1      string
	Size      int64
}

func (d *DB) ImportDATGames(roms []DATRom) (int, error) {
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	count := 0
	for _, r := range roms {
		// Insert game if not exists
		var gameID int64
		err := tx.QueryRow(`SELECT id FROM games WHERE title_en = ? AND platform = ?`, r.GameTitle, r.Platform).Scan(&gameID)
		if err == sql.ErrNoRows {
			res, err := tx.Exec(`INSERT INTO games (title_en, platform) VALUES (?, ?)`, r.GameTitle, r.Platform)
			if err != nil {
				return 0, fmt.Errorf("insert game %q: %w", r.GameTitle, err)
			}
			gameID, _ = res.LastInsertId()
			count++
		} else if err != nil {
			return 0, err
		}
	}

	return count, tx.Commit()
}

// MatchByGameList matches rom_files to games using filename from gamelist.xml
// It creates games with title_ja and links them to rom_files by filename match.
func (d *DB) MatchByGameList(entries []GameListEntry, platform string) (created int, matched int, err error) {
	tx, err := d.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	for _, e := range entries {
		// Find rom_files matching this filename and platform
		// Match exact filename, or "zipname/inner" pattern, or path containing the zip name
		rows, err := tx.Query(
			`SELECT id FROM rom_files WHERE platform = ? AND (filename = ? OR filename LIKE ? OR filename LIKE ?)`,
			platform, e.Filename, "%/"+e.Filename, e.Filename+"/%",
		)
		if err != nil {
			return 0, 0, err
		}
		var romIDs []int64
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			romIDs = append(romIDs, id)
		}
		rows.Close()

		if len(romIDs) == 0 {
			continue
		}

		// Find or create game
		var gameID int64
		err = tx.QueryRow(`SELECT id FROM games WHERE title_ja = ? AND platform = ?`, e.Name, platform).Scan(&gameID)
		if err != nil {
			res, err := tx.Exec(`INSERT INTO games (title_ja, platform) VALUES (?, ?)`, e.Name, platform)
			if err != nil {
				return 0, 0, fmt.Errorf("insert game %q: %w", e.Name, err)
			}
			gameID, _ = res.LastInsertId()
			created++
		}

		// Link rom_files to game
		for _, rid := range romIDs {
			_, err = tx.Exec(`UPDATE rom_files SET game_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, gameID, rid)
			if err != nil {
				return 0, 0, err
			}
			matched++
		}
	}

	return created, matched, tx.Commit()
}

// GameListEntry for import
type GameListEntry struct {
	Filename string
	Name     string
}

// SearchResult holds a ROM search result
type SearchResult struct {
	Platform string
	Filename string
	Title    string
}

// SearchRoms searches ROMs by title/filename with optional platform filter
func (d *DB) SearchRoms(query, platform string, page, perPage int) ([]RomFile, int, error) {
	if perPage <= 0 {
		perPage = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage
	q := "%" + query + "%"

	baseWhere := `FROM rom_files r LEFT JOIN games g ON r.game_id = g.id
		WHERE (r.filename LIKE ? OR g.title_ja LIKE ? OR g.title_en LIKE ?)`
	args := []interface{}{q, q, q}
	if platform != "" {
		baseWhere += ` AND r.platform = ?`
		args = append(args, platform)
	}

	var total int
	err := d.QueryRow("SELECT COUNT(*) "+baseWhere, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectArgs := append(args, perPage, offset)
	rows, err := d.Query(`SELECT r.id, r.path, r.filename, r.size, r.hash_crc32, r.hash_md5, r.hash_sha1, r.platform, r.game_id, g.title_en, g.title_ja `+baseWhere+` ORDER BY r.platform, r.filename LIMIT ? OFFSET ?`, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var files []RomFile
	for rows.Next() {
		var f RomFile
		if err := rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.HashCRC32, &f.HashMD5, &f.HashSHA1, &f.Platform, &f.GameID, &f.TitleEN, &f.TitleJA); err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	return files, total, rows.Err()
}

// PlatformStats holds stats for one platform
type PlatformStats struct {
	Platform  string `json:"platform"`
	Total     int    `json:"total"`
	Matched   int    `json:"matched"`
	Unmatched int    `json:"unmatched"`
	HasTitleEN int   `json:"has_title_en"`
	HasTitleJA int   `json:"has_title_ja"`
}

// Stats holds overall collection stats
type Stats struct {
	Platforms []PlatformStats `json:"platforms"`
	Total     int             `json:"total"`
	Matched   int             `json:"matched"`
	Unmatched int             `json:"unmatched"`
}

// GetStats returns collection statistics
func (d *DB) GetStats() (*Stats, error) {
	rows, err := d.Query(`
		SELECT r.platform,
			COUNT(*) as total,
			COUNT(r.game_id) as matched,
			COUNT(*) - COUNT(r.game_id) as unmatched,
			COUNT(g.title_en) as has_en,
			COUNT(g.title_ja) as has_ja
		FROM rom_files r LEFT JOIN games g ON r.game_id = g.id
		GROUP BY r.platform ORDER BY r.platform
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	s := &Stats{}
	for rows.Next() {
		var p PlatformStats
		if err := rows.Scan(&p.Platform, &p.Total, &p.Matched, &p.Unmatched, &p.HasTitleEN, &p.HasTitleJA); err != nil {
			return nil, err
		}
		s.Total += p.Total
		s.Matched += p.Matched
		s.Unmatched += p.Unmatched
		s.Platforms = append(s.Platforms, p)
	}
	return s, rows.Err()
}

// GetPlatforms returns list of distinct platforms
func (d *DB) GetPlatforms() ([]string, error) {
	rows, err := d.Query(`SELECT DISTINCT platform FROM rom_files ORDER BY platform`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var platforms []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		platforms = append(platforms, p)
	}
	return platforms, rows.Err()
}

// MatchByHash matches rom_files to games using DAT ROM info
func (d *DB) MatchROMs(datRoms []DATRom) (int, error) {
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	matched := 0
	for _, dr := range datRoms {
		// Find rom_files by hash (SHA1 > MD5 > CRC32)
		var query string
		var hashVal string
		if dr.SHA1 != "" {
			query = `SELECT id, game_id FROM rom_files WHERE hash_sha1 = ?`
			hashVal = dr.SHA1
		} else if dr.MD5 != "" {
			query = `SELECT id, game_id FROM rom_files WHERE hash_md5 = ?`
			hashVal = dr.MD5
		} else if dr.CRC32 != "" {
			query = `SELECT id, game_id FROM rom_files WHERE hash_crc32 = ?`
			hashVal = dr.CRC32
		} else {
			continue
		}

		rows, err := tx.Query(query, hashVal)
		if err != nil {
			continue
		}
		type romMatch struct {
			id     int64
			gameID *int64
		}
		var matches []romMatch
		for rows.Next() {
			var rm romMatch
			rows.Scan(&rm.id, &rm.gameID)
			matches = append(matches, rm)
		}
		rows.Close()

		if len(matches) == 0 {
			continue
		}

		for _, rm := range matches {
			if rm.gameID != nil {
				// ROM already linked to a game — update that game's title_en
				tx.Exec(`UPDATE games SET title_en = ? WHERE id = ? AND (title_en IS NULL OR title_en = '')`,
					dr.GameTitle, *rm.gameID)
				matched++
			} else {
				// ROM not linked — find or create a game with this title_en
				var gameID int64
				err := tx.QueryRow(`SELECT id FROM games WHERE title_en = ? AND platform = ?`, dr.GameTitle, dr.Platform).Scan(&gameID)
				if err != nil {
					res, err := tx.Exec(`INSERT INTO games (title_en, platform) VALUES (?, ?)`, dr.GameTitle, dr.Platform)
					if err != nil {
						continue
					}
					gameID, _ = res.LastInsertId()
				}
				tx.Exec(`UPDATE rom_files SET game_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, gameID, rm.id)
				matched++
			}
		}
	}
	return matched, tx.Commit()
}
