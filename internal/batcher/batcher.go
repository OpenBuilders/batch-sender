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

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	BatchSize       int
	ParallelBatches int
	BatchTimeout    time.Duration
	BatchDelay      time.Duration
	DBTimeout       time.Duration
}

type Batcher struct {
	Batches   chan uuid.UUID
	config    *Config
	queue     *queue.Queue
	repo      Repository
	channel   *amqp.Channel
	log       *slog.Logger
	reconnect bool
}

type Repository interface {
	PersistMessage(context.Context, types.SendTONMessage) (int64, error)
	NextBatch(context.Context, int) (uuid.UUID, error)
}

func New(config *Config, rabbit *queue.Queue, repo Repository) *Batcher {
	return &Batcher{
		Batches: make(chan uuid.UUID, config.ParallelBatches),
		config:  config,
		queue:   rabbit,
		repo:    repo,
		log:     slog.With("component", "batcher"),
	}
}

func (b *Batcher) Run(ctx context.Context) {
	b.log.Info("Starting batcher")

	b.queue.RegisterWorker(func(wrkCtx context.Context, conn *amqp.Connection) error {
		b.log.Debug("started batcher worker")
		for {
			select {
			case <-wrkCtx.Done():
				b.log.Debug("worker shutting down due to manager reconnect")
				return nil
			default:
			}

			ch, err := conn.Channel()
			if err != nil {
				b.log.Error("channel open failed", "error", err)
				time.Sleep(time.Second)
				continue
			}

			closeChan := ch.NotifyClose(make(chan *amqp.Error, 1))
			ch.NotifyClose(closeChan)

			q, err := ch.QueueDeclare(
				string(queue.QueueTONTransfer),
				true,
				false,
				false,
				false,
				nil,
			)
			if err != nil {
				b.log.Error("queue declaration failed", "error", err)
				return err
			}

			messages, err := ch.Consume(q.Name, "batcher", false, false, false, false, nil)
			if err != nil {
				b.log.Error("message consume failed", "error", err)
				return err
			}
			go b.consumeMessages(wrkCtx, messages)

			select {
			case <-wrkCtx.Done():
				b.log.Debug("context cancelled during consumption")
				return nil
			case err := <-closeChan:
				b.log.Debug("channel is closed, reconnecting", "error", err)
			}
		}
		return nil
	})
}

func (b *Batcher) consumeMessages(ctx context.Context,
	messages <-chan amqp.Delivery) error {
	var unbatched int64
	updateInterval := b.config.BatchTimeout

	for {
		select {
		case <-ctx.Done():
			b.log.Info("Stopping consumer...")
			return ctx.Err()
		case msg, ok := <-messages:
			if !ok {
				b.log.Debug("channel is closed")
				return fmt.Errorf("channel is closed")
			}

			count, _ := b.handleMessage(msg)
			unbatched += count

			if unbatched >= int64(b.config.BatchSize) {
				b.log.Debug(
					"Reached the max batch size, processing right away",
					"max", b.config.BatchSize,
				)
				created := b.createBatch()
				if created {
					updateInterval = b.config.BatchDelay
				} else {
					updateInterval = b.config.BatchTimeout
				}
				unbatched = 0
			}

		case <-time.After(updateInterval):
			// b.log.Debug("Batch interval tick")
			created := b.createBatch()
			if created {
				updateInterval = b.config.BatchDelay
			} else {
				updateInterval = b.config.BatchTimeout
			}
			unbatched = 0
		}
	}
}

// handleMessage parses the incoming message and persists in the database,
// extracting transfers, keeping mapping to the original transaction.
func (b *Batcher) handleMessage(message amqp.Delivery) (
	msgCount int64, err error) {

	b.log.Debug("Handling incoming message", "msg", message)

	var msg types.SendTONMessage

	err = json.Unmarshal(message.Body, &msg)
	if err != nil {
		b.log.Error(
			"tx unmarshalling error",
			"body", string(message.Body),
			"error", err,
		)

		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.config.DBTimeout)
	defer cancel()

	count, err := b.repo.PersistMessage(ctx, msg)
	if err != nil && err != postgres.ErrDuplicateKeyValue {
		b.log.Error("message persistence error", "msg", msg, "error", err)
		return 0, err
	}

	if err == postgres.ErrDuplicateKeyValue {
		b.log.Info("duplicate transfers, skipping", "msg", msg)
	}

	b.log.Debug("acking")
	err = message.Ack(false)
	if err != nil {
		b.log.Error(
			"Message ack error",
			"message", string(message.Body),
			"error", err,
		)

		return 0, err
	}

	return count, nil
}

func (b *Batcher) createBatch() bool {
	ctx, cancel := context.WithTimeout(context.Background(), b.config.DBTimeout)
	defer cancel()

	batchUUID, err := b.repo.NextBatch(ctx, b.config.BatchSize)
	if err != nil {
		b.log.Error("next batch error", "error", err)
		return false
	}

	if batchUUID == uuid.Nil {
		return false
	}

	b.Batches <- batchUUID

	return true
}
