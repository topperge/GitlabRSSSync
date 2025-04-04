package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
)

var (
	// ErrClosed indicates the client is closed
	ErrClosed = errors.New("client is closed")
)

// RedisCmd represents the result of a Redis command
type RedisCmd struct {
	val interface{}
	err error
}

// Result returns the result of a Redis command
func (c *RedisCmd) Result() (interface{}, error) {
	return c.val, c.err
}

// Err returns the error of a Redis command
func (c *RedisCmd) Err() error {
	return c.err
}

// BoolCmd represents the result of a Redis command that returns bool
type BoolCmd struct {
	val bool
	err error
}

// Result returns the bool result and error
func (c *BoolCmd) Result() (bool, error) {
	return c.val, c.err
}

// Err returns only the error
func (c *BoolCmd) Err() error {
	return c.err
}

// IntCmd represents the result of a Redis command that returns int64
type IntCmd struct {
	val int64
	err error
}

// Result returns the int64 result and error
func (c *IntCmd) Result() (int64, error) {
	return c.val, c.err
}

// Err returns only the error
func (c *IntCmd) Err() error {
	return c.err
}

// StatusCmd represents the result of a Redis command that returns string
type StatusCmd struct {
	val string
	err error
}

// Result returns the string result and error
func (c *StatusCmd) Result() (string, error) {
	return c.val, c.err
}

// Err returns only the error
func (c *StatusCmd) Err() error {
	return c.err
}

// RedisInterface defines a minimal Redis-like interface needed by the application
type RedisInterface interface {
	SIsMember(ctx context.Context, key string, member interface{}) *BoolCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *IntCmd
	Ping(ctx context.Context) *StatusCmd
	Close() error
}

// NewRedisAdapter creates a new Redis adapter using SQLite store
func NewRedisAdapter(store *SQLiteStore) *RedisAdapter {
	return &RedisAdapter{
		store: store,
	}
}

// RedisAdapter implements a Redis-like interface using SQLite
type RedisAdapter struct {
	store *SQLiteStore
}

// SIsMember checks if a member exists in a set
func (r *RedisAdapter) SIsMember(ctx context.Context, key string, member interface{}) *BoolCmd {
	memberStr, ok := member.(string)
	if !ok {
		memberStr = fmt.Sprintf("%v", member)
	}

	exists, err := r.store.IsMember(ctx, key, memberStr)
	if err != nil {
		log.Printf("Error in SIsMember: %v", err)
		return &BoolCmd{val: false, err: err}
	}

	return &BoolCmd{val: exists, err: nil}
}

// SAdd adds a member to a set
func (r *RedisAdapter) SAdd(ctx context.Context, key string, members ...interface{}) *IntCmd {
	var count int64
	var lastErr error

	for _, member := range members {
		memberStr, ok := member.(string)
		if !ok {
			memberStr = fmt.Sprintf("%v", member)
		}

		err := r.store.Add(ctx, key, memberStr)
		if err != nil {
			log.Printf("Error in SAdd with member %s: %v", memberStr, err)
			lastErr = err
			continue
		}
		count++
	}

	return &IntCmd{val: count, err: lastErr}
}

// Ping checks connection to the database
func (r *RedisAdapter) Ping(ctx context.Context) *StatusCmd {
	err := r.store.Ping(ctx)
	if err != nil {
		return &StatusCmd{val: "", err: err}
	}
	return &StatusCmd{val: "PONG", err: nil}
}

// Close closes the database connection
func (r *RedisAdapter) Close() error {
	return r.store.Close()
}

// RedisStore implements a Redis-like store using SQLite
// This is a convenience type that combines SQLiteStore and S3BackupManager
type RedisStore struct {
	adapter *RedisAdapter
	store   *SQLiteStore
	backup  *S3BackupManager
}

// NewRedisStore creates a new Redis-like store with SQLite backend
func NewRedisStore(dbPath string, backupConfig S3BackupConfig) (*RedisStore, error) {
	// Create SQLite store
	store, err := NewSQLiteStore(dbPath, dbPath+".bak")
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite store: %w", err)
	}

	// Create Redis adapter
	adapter := NewRedisAdapter(store)

	// Create S3 backup manager if enabled
	var backup *S3BackupManager
	if backupConfig.Enabled {
		backup, err = NewS3BackupManager(store, backupConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 backup manager: %w", err)
		}
		backup.Start()
	}

	return &RedisStore{
		adapter: adapter,
		store:   store,
		backup:  backup,
	}, nil
}

// GetClient returns a Redis-compatible client
func (r *RedisStore) GetClient() RedisInterface {
	return r.adapter
}

// Close closes the store and stops any background processes
func (r *RedisStore) Close() error {
	if r.backup != nil {
		r.backup.Stop()
	}
	return r.store.Close()
}

// ForceBackup triggers an immediate backup if S3 is configured
func (r *RedisStore) ForceBackup() error {
	if r.backup == nil {
		return errors.New("backup not configured")
	}
	return r.backup.Backup()
}

// RestoreFromBackup restores the database from S3 backup
func (r *RedisStore) RestoreFromBackup() error {
	if r.backup == nil {
		return errors.New("backup not configured")
	}
	return r.backup.Restore()
}
