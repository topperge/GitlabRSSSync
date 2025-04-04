package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteStore(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "sqlite-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	backupPath := filepath.Join(tempDir, "test.db.bak")

	// Create a new store
	store, err := NewSQLiteStore(dbPath, backupPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Verify the store can ping
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Verify the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Database file was not created at %s", dbPath)
	}
}

func TestSQLiteStore_IsMember(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "sqlite-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	backupPath := filepath.Join(tempDir, "test.db.bak")

	// Create a new store
	store, err := NewSQLiteStore(dbPath, backupPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	feedID := "test-feed"
	guid := "test-guid"

	// Initially, the guid should not exist
	exists, err := store.IsMember(ctx, feedID, guid)
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if exists {
		t.Errorf("Guid %s should not exist in feed %s", guid, feedID)
	}

	// Add the guid to the feed
	err = store.Add(ctx, feedID, guid)
	if err != nil {
		t.Fatalf("Failed to add guid: %v", err)
	}

	// Now the guid should exist
	exists, err = store.IsMember(ctx, feedID, guid)
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if !exists {
		t.Errorf("Guid %s should exist in feed %s", guid, feedID)
	}
}

func TestSQLiteStore_BackupAndRestore(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "sqlite-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	backupPath := filepath.Join(tempDir, "test.db.bak")

	// Create a new store
	store, err := NewSQLiteStore(dbPath, backupPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}

	ctx := context.Background()
	feedID := "test-feed"
	guid := "test-guid"

	// Add a guid to the feed
	err = store.Add(ctx, feedID, guid)
	if err != nil {
		t.Fatalf("Failed to add guid: %v", err)
	}

	// Backup the database
	err = store.BackupToFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to backup database: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatalf("Backup file was not created at %s", backupPath)
	}

	// Close the store
	store.Close()

	// Create a new store and restore from backup
	newStore, err := NewSQLiteStore(dbPath+".new", backupPath)
	if err != nil {
		t.Fatalf("Failed to create new SQLite store: %v", err)
	}
	defer newStore.Close()

	// Restore from backup
	err = newStore.RestoreFromFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to restore database: %v", err)
	}

	// Verify the guid exists in the restored database
	exists, err := newStore.IsMember(ctx, feedID, guid)
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if !exists {
		t.Errorf("Guid %s should exist in feed %s after restore", guid, feedID)
	}
}

func TestRedisAdapter(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "sqlite-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	backupPath := filepath.Join(tempDir, "test.db.bak")

	// Create a new store
	store, err := NewSQLiteStore(dbPath, backupPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Create a Redis adapter
	adapter := NewRedisAdapter(store)

	ctx := context.Background()
	feedID := "test-feed"
	guid := "test-guid"

	// Test SIsMember
	result := adapter.SIsMember(ctx, feedID, guid)
	exists, err := result.Result()
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if exists {
		t.Errorf("Guid %s should not exist in feed %s", guid, feedID)
	}

	// Test SAdd
	result2 := adapter.SAdd(ctx, feedID, guid)
	count, err := result2.Result()
	if err != nil {
		t.Fatalf("Failed to add guid: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count to be 1, got %d", count)
	}

	// Test Ping
	pingResult := adapter.Ping(ctx)
	pong, err := pingResult.Result()
	if err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
	if pong != "PONG" {
		t.Errorf("Expected ping result to be PONG, got %s", pong)
	}

	// Test SIsMember again
	result = adapter.SIsMember(ctx, feedID, guid)
	exists, err = result.Result()
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if !exists {
		t.Errorf("Guid %s should exist in feed %s", guid, feedID)
	}
}

func TestRedisStore(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "sqlite-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create dummy S3 backup config with backup disabled
	s3Config := S3BackupConfig{
		Enabled:   false,
		Frequency: 1 * time.Hour,
	}

	// Create a new Redis store
	redisStore, err := NewRedisStore(dbPath, s3Config)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer redisStore.Close()

	// Get Redis client
	client := redisStore.GetClient()

	ctx := context.Background()
	feedID := "test-feed"
	guid := "test-guid"

	// Test SIsMember
	result := client.SIsMember(ctx, feedID, guid)
	exists, err := result.Result()
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if exists {
		t.Errorf("Guid %s should not exist in feed %s", guid, feedID)
	}

	// Test SAdd
	result2 := client.SAdd(ctx, feedID, guid)
	count, err := result2.Result()
	if err != nil {
		t.Fatalf("Failed to add guid: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count to be 1, got %d", count)
	}

	// Test SIsMember again
	result = client.SIsMember(ctx, feedID, guid)
	exists, err = result.Result()
	if err != nil {
		t.Fatalf("Failed to check if guid exists: %v", err)
	}
	if !exists {
		t.Errorf("Guid %s should exist in feed %s", guid, feedID)
	}

	// Test forced backup (should not fail even though S3 is disabled)
	err = redisStore.ForceBackup()
	if err == nil {
		t.Error("Expected an error for ForceBackup when S3 is disabled")
	}
}
