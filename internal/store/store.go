// Package store persists indexed documents in a workspace: a pure-Go SQLite
// catalog (for lazy listing and corpus-level routing) plus one inspectable JSON
// blob per document. This replaces PageIndex's single growing _pindex.json
// manifest, which does not scale to a large corpus.
package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)

	"github.com/jjfantini/pindex/internal/tree"
)

// Store is a document workspace backed by SQLite + per-doc JSON files.
type Store struct {
	dir string
	db  *sql.DB
}

// CatalogRow is one lightweight catalog entry.
type CatalogRow struct {
	ID          string
	DocName     string
	Type        string
	Path        string
	Description string
	PageCount   int
	LineCount   int
	IndexedAt   string
}

// DocID is a stable id for a document path (sha256 of its absolute path).
func DocID(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:])[:16]
}

// Open creates/opens a workspace at dir (made if absent) and ensures the schema.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "catalog.db"))
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}
	db.SetMaxOpenConns(1) // sqlite is single-writer; avoid "database is locked"
	const schema = `CREATE TABLE IF NOT EXISTS catalog (
		id TEXT PRIMARY KEY, doc_name TEXT, type TEXT, path TEXT,
		description TEXT, page_count INTEGER, line_count INTEGER, indexed_at TEXT);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	return &Store{dir: dir, db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) docPath(id string) string { return filepath.Join(s.dir, "docs", id+".json") }

// Has reports whether a document with this id is cataloged.
func (s *Store) Has(id string) bool {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM catalog WHERE id = ?`, id).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// Save writes the document's JSON blob (atomically) and upserts its catalog row.
func (s *Store) Save(doc tree.Document) error {
	if doc.ID == "" {
		return fmt.Errorf("store: document has no id")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.docPath(doc.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("store: write blob: %w", err)
	}
	if err := os.Rename(tmp, s.docPath(doc.ID)); err != nil {
		return fmt.Errorf("store: commit blob: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO catalog
		 (id, doc_name, type, path, description, page_count, line_count, indexed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.DocName, string(doc.Type), doc.Path, doc.DocDescription,
		doc.PageCount, doc.LineCount, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store: upsert catalog: %w", err)
	}
	return nil
}

// Load reads a document's full record by id.
func (s *Store) Load(id string) (tree.Document, error) {
	data, err := os.ReadFile(s.docPath(id))
	if err != nil {
		return tree.Document{}, fmt.Errorf("store: load %s: %w", id, err)
	}
	var doc tree.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return tree.Document{}, fmt.Errorf("store: parse %s: %w", id, err)
	}
	return doc, nil
}

// List returns all catalog rows, most-recently-indexed first.
func (s *Store) List() ([]CatalogRow, error) {
	rows, err := s.db.Query(`SELECT id, doc_name, type, path, description, page_count, line_count, indexed_at
		FROM catalog ORDER BY indexed_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CatalogRow
	for rows.Next() {
		var r CatalogRow
		if err := rows.Scan(&r.ID, &r.DocName, &r.Type, &r.Path, &r.Description,
			&r.PageCount, &r.LineCount, &r.IndexedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
