package sender

import (
	"context"
	"log/slog"

	"github.com/openbuilders/batch-sender/internal/types"
)

type Sender struct {
	batches <-chan types.Batch
	log     *slog.Logger
}

func New(batches <-chan types.Batch) *Sender {
	return &Sender{
		batches: batches,
		log:     slog.With("component", "sender"),
	}
}

func (s *Sender) Run(ctx context.Context) error {
	s.log.Info("Starting sender")

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Stopping sender...")
			return ctx.Err()
		case batch, ok := <-s.batches:
			if !ok {
				s.log.Debug("Batches channel is closed")
				return nil
			}
			s.log.Info("Received a new batch", "batch", batch)
		}
	}
}
