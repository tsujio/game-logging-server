package storages

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	gcs "cloud.google.com/go/storage"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

type Storage interface {
	GetGameSecret(ctx context.Context, gameName string) (string, error)
	InsertLog(ctx context.Context, gameName string, timestamp time.Time, content interface{}) error
	RegisterScore(ctx context.Context, score *GameScore) error
	GetScoreList(ctx context.Context, gameName string) ([]GameScore, error)
	Close() error
}

type GameScore struct {
	ID        string    `firestore:"id"`
	GameName  string    `firestore:"gameName"`
	Timestamp time.Time `firestore:"timestamp"`
	PlayerID  string    `firestore:"playerId"`
	PlayID    string    `firestore:"playId"`
	Score     int       `firestore:"score"`
}

type storage struct {
	gcsClient       *gcs.Client
	firestoreClient *firestore.Client
	bucket          string
}

func New(projectID string, bucket string) (Storage, error) {
	ctx := context.Background()

	gcsClient, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	// Check if bucket exists
	if _, err := gcsClient.Bucket(bucket).Attrs(ctx); err != nil {
		return nil, fmt.Errorf("Failed to get bucket attrs: %w", err)
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &storage{
		gcsClient:       gcsClient,
		firestoreClient: firestoreClient,
		bucket:          bucket,
	}, nil
}

func (s *storage) GetGameSecret(ctx context.Context, gameName string) (string, error) {
	docs, err := s.firestoreClient.Collection("games").
		Where("gameName", "==", gameName).
		Documents(ctx).
		GetAll()

	if err != nil {
		return "", err
	}

	if len(docs) == 0 {
		return "", fmt.Errorf("Game not found: %s", gameName)
	}

	data := docs[0].Data()

	secretValue, exists := data["secret"]
	if !exists {
		return "", nil
	}
	secret, ok := secretValue.(string)
	if !ok {
		return "", nil
	}

	return secret, nil
}

func (s *storage) InsertLog(ctx context.Context, gameName string, timestamp time.Time, content interface{}) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("Failed to generate uuid: %w", err)
	}
	key := fmt.Sprintf(
		"logs/game=%s/dt=%s/%s.json",
		gameName,
		timestamp.Format("2006-01-02"),
		id.String(),
	)

	bucket := s.gcsClient.Bucket(s.bucket)
	obj := bucket.Object(key)
	w := obj.NewWriter(ctx)
	defer w.Close()

	if err := json.NewEncoder(w).Encode(content); err != nil {
		return fmt.Errorf("Failed to encode as json: %w", err)
	}

	return nil
}

func (s *storage) RegisterScore(ctx context.Context, score *GameScore) error {
	if score.ID == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("Failed to generate uuid: %w", err)
		}
		score.ID = id.String()
	}

	_, err := s.firestoreClient.Collection("scores").Doc(score.ID).Set(ctx, score)

	return err
}

func (s *storage) GetScoreList(ctx context.Context, gameName string) ([]GameScore, error) {
	iter := s.firestoreClient.Collection("scores").
		Where("gameName", "==", gameName).
		OrderBy("score", firestore.Desc).
		OrderBy("timestamp", firestore.Desc).
		Limit(10).
		Documents(ctx)

	var scores []GameScore
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var score GameScore
		if err := doc.DataTo(&score); err != nil {
			return nil, err
		}

		scores = append(scores, score)
	}

	return scores, nil
}

func (s *storage) Close() error {
	if err := s.gcsClient.Close(); err != nil {
		return err
	}
	if err := s.firestoreClient.Close(); err != nil {
		return err
	}
	return nil
}
