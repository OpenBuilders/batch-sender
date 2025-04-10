package batcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/openbuilders/batch-sender/internal/types"
)

type Config struct {
	BatchSize    int
	BatchTimeout time.Duration
}

type Batcher struct {
	log *slog.Logger
}

func New(config *Config) *Batcher {
	return &Batcher{
		log: slog.With("component", "batcher"),
	}
}

func (b *Batcher) Enqueue(ctx context.Context, tx *types.Transaction) error {
	return nil
}
