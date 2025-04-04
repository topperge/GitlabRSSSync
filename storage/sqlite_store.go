package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store defines the interface for item storage
type Store interface {
	IsMember(ctx context.Context, feedID string, guid string) (bool, error)
	Add(ctx context.Context, feedID string, guid string) error
	Ping(ctx context.Context) error
	Close() error
}

// SQLiteStore implements the Store interface using SQLite
type SQLiteStore struct {
	db         *sql.DB
	dbPath     string
	backupPath string
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(dbPath string, backupPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create store
	store := &SQLiteStore{
		db:         db,
		dbPath:     dbPath,
		backupPath: backupPath,
	}

	// Initialize database
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates necessary tables if they don't exist
func (s *SQLiteStore) initialize() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS feed_items (
			feed_id TEXT NOT NULL,
			guid TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (feed_id, guid)
		);
		CREATE INDEX IF NOT EXISTS idx_feed_items_feed_id ON feed_items(feed_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

// IsMember checks if a GUID exists for a feed
func (s *SQLiteStore) IsMember(ctx context.Context, feedID string, guid string) (bool, error) {
	var exists bool
	row := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM feed_items WHERE feed_id = ? AND guid = ?)",
		feedID, guid)

	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check if guid exists: %w", err)
	}

	return exists, nil
}

// Add adds a GUID to a feed
func (s *SQLiteStore) Add(ctx context.Context, feedID string, guid string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO feed_items (feed_id, guid, created_at) VALUES (?, ?, ?)",
		feedID, guid, time.Now())

	if err != nil {
		return fmt.Errorf("failed to add guid: %w", err)
	}

	return nil
}

// Ping checks if the database is accessible
func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// BackupToFile backs up the database to a local file
func (s *SQLiteStore) BackupToFile(backupPath string) error {
	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// First, make sure all writes are flushed
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
		log.Printf("Warning: Failed to checkpoint database: %v", err)
	}

	// Copy the database file to the backup location
	srcFile, err := os.Open(s.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	return nil
}

// RestoreFromFile restores the database from a local file
func (s *SQLiteStore) RestoreFromFile(backupPath string) error {
	// Close the current database connection
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	// Copy the backup file to the database location
	srcFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(s.dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return fmt.Errorf("failed to copy backup: %w", err)
	}

	// Reopen the database connection
	db, err := sql.Open("sqlite3", s.dbPath)
	if err != nil {
		return fmt.Errorf("failed to reopen database: %w", err)
	}

	s.db = db
	return nil
}
