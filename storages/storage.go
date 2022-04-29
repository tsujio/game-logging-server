package storages

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gcs "cloud.google.com/go/storage"
	"github.com/google/uuid"
)

type Storage interface {
	InsertLog(ctx context.Context, gameName string, timestamp time.Time, content interface{}) error
	Close() error
}

func New(bucket string) (Storage, error) {
	ctx := context.Background()

	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	// Check if bucket exists
	if _, err := client.Bucket(bucket).Attrs(ctx); err != nil {
		return nil, fmt.Errorf("Failed to get bucket attrs: %w", err)
	}

	return &storage{
		client: client,
		bucket: bucket,
	}, nil
}

type storage struct {
	client *gcs.Client
	bucket string
}

func (s *storage) InsertLog(ctx context.Context, gameName string, timestamp time.Time, content interface{}) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("Failed to generate uuid: %w", err)
	}
	key := fmt.Sprintf(
		"game=%s/dt=%s/%s.json",
		gameName,
		timestamp.Format("2006-01-02"),
		id.String(),
	)

	bucket := s.client.Bucket(s.bucket)
	obj := bucket.Object(key)
	w := obj.NewWriter(ctx)
	defer w.Close()

	if err := json.NewEncoder(w).Encode(content); err != nil {
		return fmt.Errorf("Failed to encode as json: %w", err)
	}

	return nil
}

func (s *storage) Close() error {
	return s.client.Close()
}
