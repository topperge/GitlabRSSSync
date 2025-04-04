package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3BackupConfig holds configuration for S3 backups
type S3BackupConfig struct {
	Enabled    bool
	Endpoint   string
	Region     string
	BucketName string
	KeyPrefix  string
	AccessKey  string
	SecretKey  string
	Frequency  time.Duration
}

// S3BackupManager handles backing up SQLite database to S3
type S3BackupManager struct {
	s3Client  *s3.Client
	store     *SQLiteStore
	config    S3BackupConfig
	ctx       context.Context
	cancelCtx context.CancelFunc
}

// NewS3BackupManager creates a new S3 backup manager
func NewS3BackupManager(store *SQLiteStore, config S3BackupConfig) (*S3BackupManager, error) {
	if !config.Enabled {
		return &S3BackupManager{
			store:  store,
			config: config,
		}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Configure AWS SDK
	var awsConfig aws.Config
	var err error

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if config.Endpoint != "" {
			return aws.Endpoint{
				URL:               config.Endpoint,
				HostnameImmutable: true,
				SigningRegion:     config.Region,
			}, nil
		}
		// Use default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	configOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(config.Region),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	}

	if config.AccessKey != "" && config.SecretKey != "" {
		// Add credentials provider if keys are provided
		configOpts = append(configOpts,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				config.AccessKey,
				config.SecretKey,
				"",
			)),
		)
	}

	// Load AWS config with options
	awsConfig, err = awsconfig.LoadDefaultConfig(ctx, configOpts...)

	if err != nil {
		return nil, fmt.Errorf("failed to configure AWS SDK: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsConfig)

	return &S3BackupManager{
		s3Client:  s3Client,
		store:     store,
		config:    config,
		ctx:       ctx,
		cancelCtx: cancel,
	}, nil
}

// Start begins the backup process based on configuration
func (m *S3BackupManager) Start() {
	if !m.config.Enabled || m.config.Frequency <= 0 {
		log.Println("S3 backup disabled or frequency not set")
		return
	}

	go m.backupLoop()
}

// backupLoop runs periodic backups
func (m *S3BackupManager) backupLoop() {
	ticker := time.NewTicker(m.config.Frequency)
	defer ticker.Stop()

	// Initial backup
	if err := m.Backup(); err != nil {
		log.Printf("Initial S3 backup failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := m.Backup(); err != nil {
				log.Printf("Scheduled S3 backup failed: %v", err)
			}
		case <-m.ctx.Done():
			log.Println("S3 backup manager stopped")
			return
		}
	}
}

// Backup performs a database backup to S3
func (m *S3BackupManager) Backup() error {
	if !m.config.Enabled {
		return nil
	}

	// Create a temporary backup file
	tempDir, err := os.MkdirTemp("", "sqlite-backup-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	backupFile := filepath.Join(tempDir, "backup.db")
	if err := m.store.BackupToFile(backupFile); err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}

	// Upload to S3
	return m.uploadToS3(backupFile)
}

// uploadToS3 uploads a file to S3
func (m *S3BackupManager) uploadToS3(filePath string) error {
	if m.s3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	// Read file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Generate key with timestamp
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	key := fmt.Sprintf("%s/%s.db", m.config.KeyPrefix, timestamp)

	// Upload to S3
	_, err = m.s3Client.PutObject(m.ctx, &s3.PutObjectInput{
		Bucket: aws.String(m.config.BucketName),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	log.Printf("Successfully backed up database to s3://%s/%s", m.config.BucketName, key)
	return nil
}

// Restore attempts to restore the database from S3
func (m *S3BackupManager) Restore() error {
	if !m.config.Enabled || m.s3Client == nil {
		return fmt.Errorf("S3 backup not enabled or client not initialized")
	}

	// List objects to find the latest backup
	prefix := m.config.KeyPrefix + "/"
	resp, err := m.s3Client.ListObjectsV2(m.ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(m.config.BucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return fmt.Errorf("failed to list S3 objects: %w", err)
	}

	if len(resp.Contents) == 0 {
		return fmt.Errorf("no backups found in S3 bucket")
	}

	// Find the latest backup
	var latestKey string
	var latestTime time.Time
	for _, obj := range resp.Contents {
		if obj.LastModified.After(latestTime) {
			latestTime = *obj.LastModified
			latestKey = *obj.Key
		}
	}

	// Download from S3
	objResp, err := m.s3Client.GetObject(m.ctx, &s3.GetObjectInput{
		Bucket: aws.String(m.config.BucketName),
		Key:    aws.String(latestKey),
	})
	if err != nil {
		return fmt.Errorf("failed to download backup from S3: %w", err)
	}
	defer objResp.Body.Close()

	// Read the entire object
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, objResp.Body); err != nil {
		return fmt.Errorf("failed to read S3 object: %w", err)
	}

	// Create a temporary file
	tempDir, err := os.MkdirTemp("", "sqlite-restore-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	restoreFile := filepath.Join(tempDir, "restore.db")
	if err := os.WriteFile(restoreFile, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write restore file: %w", err)
	}

	// Restore from the temporary file
	if err := m.store.RestoreFromFile(restoreFile); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	log.Printf("Successfully restored database from s3://%s/%s", m.config.BucketName, latestKey)
	return nil
}

// Stop stops the backup manager
func (m *S3BackupManager) Stop() {
	if m.cancelCtx != nil {
		m.cancelCtx()
	}
}
