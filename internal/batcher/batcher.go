package batcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/openbuilders/batch-sender/internal/queue"
	"github.com/openbuilders/batch-sender/internal/repository/postgres"
	"github.com/openbuilders/batch-sender/internal/types"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	BatchSize       int
	ParallelBatches int
	BatchInterval   time.Duration
	DBTimeout       time.Duration
}

type Batcher struct {
	Batches   chan types.Batch
	config    *Config
	conn      *amqp.Connection
	repo      Repository
	channel   *amqp.Channel
	log       *slog.Logger
	reconnect bool
}

type Repository interface {
	PersistRawTXBatch([]types.Transaction) error
	PersistTX(context.Context, types.Transaction) ([]types.TXTransfer, error)
}

func New(config *Config, conn *amqp.Connection, repo Repository) *Batcher {
	return &Batcher{
		Batches: make(chan types.Batch, config.ParallelBatches),
		config:  config,
		conn:    conn,
		repo:    repo,
		log:     slog.With("component", "batcher"),
	}
}

func (b *Batcher) Run(ctx context.Context) error {
	b.log.Info("Starting batcher")
	ch, err := queue.EnsureQueueExists(b.conn, queue.QueueSendTON)
	if err != nil {
		return err
	}
	// we'll open a new channel for the consumer anyway
	ch.Close()

	messages, err := b.restartConsumer()
	if err != nil {
		return err
	}

	batch := make([]types.TXTransfer, 0, b.config.BatchSize)

	flush := func() {
		if len(batch) == 0 {
			// b.log.Debug("Emtpy batch, skipping")
			return
		}

		// TODO(carterqw): when allocations become an issue, use sync.Pool
		batchCopy := make([]types.TXTransfer, len(batch))
		copy(batchCopy, batch)

		// go b.processBatch(types.Batch(batchCopy))
		b.Batches <- batchCopy

		batch = batch[:0]
	}

	for {
		if b.reconnect {
			b.log.Debug("Reconnection is needed")

			messages, err = b.restartConsumer()
			if err != nil {
				return err
			}

			b.reconnect = false
		}

		// TODO(carterqw): read new unprocessed transfers from the db and add
		// them to the batch

		select {
		case <-ctx.Done():
			b.log.Info("Stopping batcher...")
			return ctx.Err()
		case msg, ok := <-messages:
			if !ok {
				b.log.Debug("Queue is closed, flushing")
				flush()
				return fmt.Errorf("queue is closed")
			}

			transfers, _ := b.handleMessage(msg)
			for _, transfer := range transfers {
				batch = append(batch, transfer)
				if len(batch) >= b.config.BatchSize {
					b.log.Debug(
						"Reached the max batch size, processing right away",
						"max", b.config.BatchSize,
					)
					flush()
				}
			}

		case <-time.After(b.config.BatchInterval):
			// b.log.Debug("Batch interval tick")
			flush()
		}
	}

	b.log.Info("Batcher stopped")

	return nil
}

func (b *Batcher) restartConsumer() (<-chan amqp.Delivery, error) {
	if b.channel != nil && !b.channel.IsClosed() {
		b.channel.Close()
	}

	ch, err := b.conn.Channel()
	if err != nil {
		return nil, err
	}

	ch.Qos(b.config.BatchSize, 0, false)

	b.channel = ch

	return ch.Consume(
		string(queue.QueueSendTON), // queue
		"batcher",                  // consumer
		false,                      // autoAck
		false,                      // exclusive
		false,                      // noLocal
		false,                      // no wait
		nil,                        // args
	)
}

// handleMessage parses the incoming message and persists in the database,
// extracting transfers, keeping mapping to the original transaction.
func (b *Batcher) handleMessage(message amqp.Delivery) (
	hashes []types.TXTransfer, err error) {

	b.log.Debug("Handling incoming message", "msg", message)

	defer func() {
		if err != nil {
			// if there was an issue with acking a message, or if we returned
			// early, we need to close the channel and restart the consumer,
			// otherwise, unacked messages will stay in the limbo state until
			// the connection is restarted.
			// Unacked messages will accumulate affecting prefetching and
			// eventually when they reach the BatchSize, the consumer will stop
			// receiving messages. To avoid that, we reconnect immediately when
			// we see ack errors.
			b.reconnect = true
		}
	}()

	var tx types.Transaction

	err = json.Unmarshal(message.Body, &tx)
	if err != nil {
		b.log.Error(
			"tx unmarshalling error",
			"body", string(message.Body),
			"error", err,
		)

		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.config.DBTimeout)
	defer cancel()

	transfers, err := b.repo.PersistTX(ctx, tx)
	if err != nil && err != postgres.ErrDuplicateKeyValue {
		return nil, err
	}

	if err == postgres.ErrDuplicateKeyValue {
		b.log.Info("duplicate transfers, skipping", "tx", tx)
	}

	err = message.Ack(false)
	if err != nil {
		b.log.Error(
			"Message ack error",
			"message", string(message.Body),
			"error", err,
		)

		return nil, err
	}

	return transfers, nil
}
