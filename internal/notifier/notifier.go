package notifier

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/openbuilders/batch-sender/internal/queue"
	"github.com/openbuilders/batch-sender/internal/types"
)

type Config struct {
	BatchSize    int64
	PollInterval time.Duration
	DBTimeout    time.Duration
}

type Notifier struct {
	config *Config
	queue  *queue.Queue
	repo   Repository
	log    *slog.Logger
}

const (
	PatternPaymentStatus = "payment-status"
)

type BatchResultData struct {
	TransactionID string            `json:"transaction_id"`
	TxHash        string            `json:"tx_hash"`
	Status        types.BatchStatus `json:"status"`
}

type BatchResultNotification struct {
	Pattern string          `json:"payment-status"`
	Data    BatchResultData `json:"data"`
}

type Repository interface {
	GetTransfersBatchResults(context.Context, int64) ([]types.BatchResult, error)
	PersistProcessedBatchResults(context.Context, []string) error
}

func New(config *Config, rabbit *queue.Queue, repo Repository) *Notifier {
	return &Notifier{
		config: config,
		queue:  rabbit,
		repo:   repo,
		log:    slog.With("component", "notifier"),
	}
}

func (n *Notifier) Start(ctx context.Context) error {
	n.log.Info("Starting notifier...")

	pollInterval := time.Duration(0)

main:
	for {
		select {
		case <-ctx.Done():
			n.log.Info("Stopping notifier.")
			return nil

		case <-time.After(pollInterval):
			pollInterval = n.config.PollInterval

			ctxWithTimeout, cancel := context.WithTimeout(ctx, n.config.DBTimeout)
			defer cancel()

			results, err := n.repo.GetTransfersBatchResults(ctxWithTimeout,
				n.config.BatchSize,
			)
			if err != nil {
				n.log.Error("couldn't get batch results", "error", err)
				continue
			}

			batchUUIDs := make([]string, len(results))

			for i, result := range results {
				payload := BatchResultNotification{
					Pattern: PatternPaymentStatus,
					Data: BatchResultData{
						TransactionID: result.OrderID,
						TxHash:        result.TxHash,
						Status:        result.Status,
					},
				}

				jsonData, err := json.Marshal(payload)
				if err != nil {
					n.log.Error(
						"error marshaling JSON",
						"payload", payload,
						"error", err,
					)

					continue main
				}

				batchUUIDs[i] = result.BatchUUID

				n.log.Debug("Sending notification", "payload", jsonData)

				err = n.queue.Publish(queue.QueueMainService, jsonData)
				if err != nil {
					n.log.Error(
						"couldn't enqueue message",
						"message", jsonData,
						"error", err,
					)

					continue main
				}
			}

			if len(batchUUIDs) > 0 {
				ctxWithTimeout, cancel := context.WithTimeout(ctx, n.config.DBTimeout)
				defer cancel()

				err = n.repo.PersistProcessedBatchResults(ctxWithTimeout, batchUUIDs)
				if err != nil {
					n.log.Error(
						"couldn't persist notification results",
						"batches", batchUUIDs,
						"error", err,
					)
				}

				n.log.Debug("Processed a batch of transfers", "uuids", batchUUIDs)
			}
		}
	}

	return nil
}
