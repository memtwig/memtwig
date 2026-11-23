package index

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/embed"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

type Store struct {
	DB   *sql.DB
	Path string
}

type Stats struct {
	Vaults  int
	Indexed int
	Skipped int
	Removed int
	Chunks  int
	Embeds  int
}

type Chunk struct {
	ID        int64
	VaultID   string
	RelPath   string
	Title     string
	Heading   string
	Content   string
	Embedding []float32
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS index_meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	vault_id TEXT NOT NULL,
	rel_path TEXT NOT NULL,
	sha256 TEXT NOT NULL,
	title TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT '',
	owner TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT '',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	UNIQUE(vault_id, rel_path)
);

CREATE TABLE IF NOT EXISTS chunks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	doc_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
	vault_id TEXT NOT NULL,
	rel_path TEXT NOT NULL,
	title TEXT NOT NULL,
	heading TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL,
	embedding BLOB
);

CREATE INDEX IF NOT EXISTS idx_chunks_vault ON chunks(vault_id);
CREATE INDEX IF NOT EXISTS idx_chunks_doc ON chunks(doc_id);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
	title,
	heading,
	content,
	content='chunks',
	content_rowid='id',
	tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
	INSERT INTO chunks_fts(rowid, title, heading, content)
	VALUES (new.id, new.title, new.heading, new.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, title, heading, content)
	VALUES('delete', old.id, old.title, old.heading, old.content);
END;
`

func Open(dbPath string) (*Store, error) {
	if dbPath == "" {
		var err error
		dbPath, err = config.DefaultDBPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{DB: db, Path: dbPath}, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// RebuildOrUpdate indexes all vaults in cfg incrementally (or full when full=true or embed signature changed).
func (s *Store) RebuildOrUpdate(ctx context.Context, cfg *config.Config, full bool) (Stats, error) {
	var stats Stats
	embedSig := fmt.Sprintf("%s|%s|%d", cfg.Embeddings.Provider, cfg.Embeddings.Model, cfg.Embeddings.Dimensions)
	var prevSig string
	_ = s.DB.QueryRow(`SELECT value FROM index_meta WHERE key = 'embed_signature'`).Scan(&prevSig)
	if prevSig != "" && prevSig != embedSig {
		full = true
	}
	if full {
		if _, err := s.DB.Exec(`DELETE FROM chunks; DELETE FROM documents;`); err != nil {
			return stats, err
		}
	}
	_, _ = s.DB.Exec(`INSERT INTO index_meta(key, value) VALUES('embed_signature', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, embedSig)
	prov := embed.New(cfg.Embeddings)
	useEmbed := prov.Name() != "none"

	for name, vc := range cfg.Vaults {
		root := config.ExpandHome(vc.Path)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		stats.Vaults++
		notes, err := vault.WalkNotes(name, root)
		if err != nil {
			return stats, fmt.Errorf("walk vault %s: %w", name, err)
		}

		// Load existing hashes for this vault
		existing := map[string]struct {
			id  int64
			sha string
		}{}
		rows, err := s.DB.Query(`SELECT id, rel_path, sha256 FROM documents WHERE vault_id = ?`, name)
		if err != nil {
			return stats, err
		}
		for rows.Next() {
			var id int64
			var rp, sha string
			if err := rows.Scan(&id, &rp, &sha); err == nil {
				existing[rp] = struct {
					id  int64
					sha string
				}{id: id, sha: sha}
			}
		}
		rows.Close()

		seen := map[string]bool{}
		for _, n := range notes {
			seen[n.RelPath] = true
			if prev, ok := existing[n.RelPath]; ok && prev.sha == n.SHA256 && !full {
				stats.Skipped++
				continue
			}

			parts := ChunkMarkdown(n.Title, n.Body, 1200)
			var vecs [][]float32
			if useEmbed && len(parts) > 0 {
				texts := make([]string, len(parts))
				for i, p := range parts {
					texts[i] = strings.TrimSpace(p.Heading + "\n" + p.Content)
				}
				if v, err := prov.Embed(ctx, texts); err == nil && len(v) == len(parts) {
					vecs = v
					stats.Embeds += len(v)
				} else {
					// Graceful degradation: disable embeddings for rest of run if provider fails.
					useEmbed = false
				}
			}

			tx, err := s.DB.BeginTx(ctx, nil)
			if err != nil {
				return stats, err
			}
			if prev, ok := existing[n.RelPath]; ok {
				if _, err := tx.Exec(`DELETE FROM documents WHERE id = ?`, prev.id); err != nil {
					tx.Rollback()
					return stats, err
				}
			}
			res, err := tx.Exec(
				`INSERT INTO documents(vault_id, rel_path, sha256, title, status, owner, updated_at, size_bytes)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				name, n.RelPath, n.SHA256, n.Title, n.Status, n.Owner, n.Updated, n.SizeBytes,
			)
			if err != nil {
				tx.Rollback()
				return stats, err
			}
			docID, _ := res.LastInsertId()

			for i, p := range parts {
				var blob []byte
				if i < len(vecs) {
					blob = EncodeFloat32s(vecs[i])
				}
				if _, err := tx.Exec(
					`INSERT INTO chunks(doc_id, vault_id, rel_path, title, heading, content, embedding)
					 VALUES (?, ?, ?, ?, ?, ?, ?)`,
					docID, name, n.RelPath, n.Title, p.Heading, p.Content, blob,
				); err != nil {
					tx.Rollback()
					return stats, err
				}
				stats.Chunks++
			}
			if err := tx.Commit(); err != nil {
				return stats, err
			}
			stats.Indexed++
		}

		// Delete documents no longer on disk
		for rp, prev := range existing {
			if !seen[rp] {
				if _, err := s.DB.Exec(`DELETE FROM documents WHERE id = ?`, prev.id); err == nil {
					stats.Removed++
				}
			}
		}
	}
	return stats, nil
}

// EnsureFresh performs a fast incremental update so searches and hooks always see recent note edits.
// Files whose SHA256 matches the SQLite table are skipped without re-chunking or re-embedding.
func (s *Store) EnsureFresh(ctx context.Context, cfg *config.Config) (Stats, error) {
	return s.RebuildOrUpdate(ctx, cfg, false)
}

// Watch periodically checks configured vaults for modified or deleted Markdown notes and refreshes the index.
func (s *Store) Watch(ctx context.Context, cfg *config.Config, interval time.Duration, onChange func(Stats)) error {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	if stats, err := s.EnsureFresh(ctx, cfg); err == nil && (stats.Indexed > 0 || stats.Removed > 0) && onChange != nil {
		onChange(stats)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			stats, err := s.EnsureFresh(ctx, cfg)
			if err == nil && (stats.Indexed > 0 || stats.Removed > 0) && onChange != nil {
				onChange(stats)
			}
		}
	}
}

type mdSection struct {
	Heading string
	Content string
}

// ChunkMarkdown splits markdown text by headings (## / ###) and wraps overly long sections.
func ChunkMarkdown(title, body string, maxChars int) []mdSection {
	if maxChars <= 0 {
		maxChars = 1200
	}
	lines := strings.Split(body, "\n")
	var rawSections []mdSection
	curHeading := title
	var buf []string

	flush := func() {
		txt := strings.TrimSpace(strings.Join(buf, "\n"))
		if txt != "" {
			rawSections = append(rawSections, mdSection{Heading: curHeading, Content: txt})
		}
		buf = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			flush()
			curHeading = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			continue
		}
		buf = append(buf, line)
	}
	flush()

	if len(rawSections) == 0 && strings.TrimSpace(body) != "" {
		rawSections = append(rawSections, mdSection{Heading: title, Content: strings.TrimSpace(body)})
	}

	var out []mdSection
	for _, s := range rawSections {
		if len(s.Content) <= maxChars {
			out = append(out, s)
			continue
		}
		// Split long section on line boundaries
		sLines := strings.Split(s.Content, "\n")
		var part strings.Builder
		for _, l := range sLines {
			if part.Len()+len(l)+1 > maxChars && part.Len() > 0 {
				out = append(out, mdSection{Heading: s.Heading, Content: strings.TrimSpace(part.String())})
				part.Reset()
			}
			part.WriteString(l)
			part.WriteByte('\n')
		}
		if strings.TrimSpace(part.String()) != "" {
			out = append(out, mdSection{Heading: s.Heading, Content: strings.TrimSpace(part.String())})
		}
	}
	return out
}

func EncodeFloat32s(v []float32) []byte {
	if len(v) == 0 {
		return nil
	}
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func DecodeFloat32s(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}
