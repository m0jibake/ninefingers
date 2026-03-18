package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Summary struct {
	ID          string    `json:"id"`
	VideoURL    string    `json:"video_url"`
	VideoTitle  string    `json:"video_title"`
	Model       string    `json:"model"`
	Language    string    `json:"language"`
	Prompt      string    `json:"prompt"`
	SummaryText string    `json:"summary_text"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

func New() (*Store, error) {
	dataDir, err := dataPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "ninefingers.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func dataPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ninefingers"), nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS summaries (
			id          TEXT PRIMARY KEY,
			video_url   TEXT NOT NULL,
			video_title TEXT NOT NULL DEFAULT '',
			model       TEXT NOT NULL,
			language    TEXT NOT NULL DEFAULT 'en',
			prompt      TEXT NOT NULL DEFAULT '',
			summary_text TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		)
	`)
	return err
}

func (s *Store) SaveSummary(summary *Summary) error {
	_, err := s.db.Exec(`
		INSERT INTO summaries (id, video_url, video_title, model, language, prompt, summary_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, summary.ID, summary.VideoURL, summary.VideoTitle, summary.Model, summary.Language, summary.Prompt, summary.SummaryText, summary.CreatedAt)
	return err
}

// UpdateSummaryText appends or replaces the summary text (used during streaming).
func (s *Store) UpdateSummaryText(id, text string) error {
	_, err := s.db.Exec(`UPDATE summaries SET summary_text = ? WHERE id = ?`, text, id)
	return err
}

func (s *Store) ListSummaries() ([]Summary, error) {
	rows, err := s.db.Query(`
		SELECT id, video_url, video_title, model, language, prompt, summary_text, created_at
		FROM summaries ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var sum Summary
		if err := rows.Scan(&sum.ID, &sum.VideoURL, &sum.VideoTitle, &sum.Model, &sum.Language, &sum.Prompt, &sum.SummaryText, &sum.CreatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, sum)
	}
	return summaries, rows.Err()
}

func (s *Store) GetSummary(id string) (*Summary, error) {
	var sum Summary
	err := s.db.QueryRow(`
		SELECT id, video_url, video_title, model, language, prompt, summary_text, created_at
		FROM summaries WHERE id = ?
	`, id).Scan(&sum.ID, &sum.VideoURL, &sum.VideoTitle, &sum.Model, &sum.Language, &sum.Prompt, &sum.SummaryText, &sum.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

func (s *Store) DeleteSummary(id string) error {
	_, err := s.db.Exec(`DELETE FROM summaries WHERE id = ?`, id)
	return err
}
