package sender

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/google/uuid"
)

type Config struct {
	NumWorkers int64
	DBTimeout  time.Duration
}

type TransactionSender interface {
	Send(context.Context, []types.Transaction) (string, error)
}

type Sender struct {
	batches chan uuid.UUID
	senders []TransactionSender
	config  *Config
	repo    Repository
	log     *slog.Logger
}

type Repository interface {
	GetNewBatches(context.Context) ([]uuid.UUID, error)
	GetBatchTransactions(context.Context, uuid.UUID) ([]types.Transaction, error)
	UpdateBatchStatus(context.Context, uuid.UUID, types.BatchStatus) error
}

func New(config *Config, senders []TransactionSender, repo Repository,
	batches chan uuid.UUID) *Sender {
	return &Sender{
		batches: batches,
		senders: senders,
		config:  config,
		repo:    repo,
		log:     slog.With("component", "sender"),
	}
}

func (s *Sender) Run(ctx context.Context) error {
	s.log.Info("Starting sender")

	go s.backfillFromDB()

	var wg sync.WaitGroup
	for i := 0; i < int(s.config.NumWorkers); i++ {
		wg.Add(1)
		go s.worker(ctx, i, s.batches, &wg)
	}

	wg.Wait()
	s.log.Info("Stopped sender")

	return nil
}

func (s *Sender) worker(ctx context.Context, id int, batches <-chan uuid.UUID,
	wg *sync.WaitGroup) {
	defer wg.Done()

	s.log.Info("Starting sender worker", "id", id)
	for {
		select {
		case <-ctx.Done():
			s.log.Info("Stopping sender worker...", "id", id)
			return
		case batchUUID, ok := <-s.batches:
			if !ok {
				s.log.Debug("Batches channel is closed")
				return
			}

			s.log.Debug("Received a new batch", "uuid", batchUUID)

			ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.DBTimeout)
			defer cancel()

			txs, err := s.repo.GetBatchTransactions(ctxWithTimeout, batchUUID)
			if err != nil {
				s.log.Error(
					"couldn't get batch txs",
					"uuid", batchUUID,
					"error", err,
				)
				continue
			}

			s.log.Debug("Got batch txs", "txs", txs)
			// TODO(carterqw): emulates sending, remove
			sender := s.senders[0]
			txHash, err := sender.Send(ctxWithTimeout, txs)
			if err != nil {
				s.log.Error("sending txs failed", "txs", txs, "error", err)
				//TODO(carterqw): handle errors
				continue
			}

			s.log.Debug("Got tx hash", "hash", txHash)

			err = s.repo.UpdateBatchStatus(context.Background(), batchUUID, types.StatusPending)
			if err != nil {
				s.log.Error("couldn't update batch")
				continue
			}
		}
	}
}

// backfillFromDB reads all batches in the 'new' status and writes them to the
// same channel for processing
func (s *Sender) backfillFromDB() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), s.config.DBTimeout)
		defer cancel()

		uuids, err := s.repo.GetNewBatches(ctx)
		if err != nil {
			s.log.Error(
				"couldn't get new batches",
				"error", err,
			)
		}

		s.log.Debug("Got batch uuids", "uuids", uuids)
		for _, uuid := range uuids {
			s.batches <- uuid
		}

		s.log.Debug("Backfilling completed", "count", len(uuids))
		return
	}
}
