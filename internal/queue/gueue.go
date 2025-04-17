package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueName string

const (
	QueueTONTransfer QueueName = "ton_transfer"
	QueueMainService QueueName = "main-service"
)

type Publisher struct {
	queueName QueueName
	conn      *amqp.Connection
	channel   *amqp.Channel
	log       *slog.Logger
}

type WorkerFunc func(context.Context, *amqp.Connection) error

type Config struct {
	URL               string
	ReconnectInterval time.Duration
	ConnectTimeout    time.Duration
}

type Queue struct {
	config  *Config
	conn    *amqp.Connection
	workers []WorkerFunc
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	log     *slog.Logger
}

func New(config *Config) *Queue {
	return &Queue{
		config: config,
		log:    slog.With("component", "queue"),
	}
}

func (q *Queue) Start(ctx context.Context) error {
	q.log.Info("Starting the queue manager.")
	defer q.log.Info("Stopping the queue manager.")

	return q.reconnectLoop(ctx)
}

// RegisterWorker stores a worker that will be invoked every time a channel is (re)created.
func (q *Queue) RegisterWorker(w WorkerFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.workers = append(q.workers, w)
}

func (q *Queue) reconnectLoop(ctx context.Context) error {
	q.log.Debug("started reconnect loop.")
	defer q.log.Debug("reconnect loop exited.")

	for {
		select {
		case <-ctx.Done():
			q.log.Debug("closing reconnect loop...")
			return ctx.Err()
		default:
		}

		q.log.Info("connecting to Rabbit MQ...")
		if err := q.connect(ctx); err != nil {
			q.log.Error("connection to Rabbit MQ failed", "error", err)
			time.Sleep(q.config.ReconnectInterval)
			continue
		}

		q.log.Info("connected to Rabbit MQ...")

		connErrors := make(chan *amqp.Error, 1)
		q.conn.NotifyClose(connErrors)

		select {
		case <-ctx.Done():
			q.log.Debug("closing reconnect loop...")
			return ctx.Err()
		case err := <-connErrors:
			q.log.Error("rabbit mq connection closed", "error", err)
		}

		// q.cleanup()
		// cancel context of all the workers
		// q.cancel()

		time.Sleep(q.config.ReconnectInterval)
	}
}

func (q *Queue) connect(ctx context.Context) error {
	conn, err := amqp.DialConfig(q.config.URL, amqp.Config{
		Dial: amqp.DefaultDial(q.config.ConnectTimeout),
	})
	if err != nil {
		return err
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	q.ctx = ctxWithCancel
	q.cancel = cancel

	q.mu.Lock()
	q.conn = conn
	workers := append([]WorkerFunc{}, q.workers...)
	q.mu.Unlock()

	for _, w := range workers {
		go w(ctxWithCancel, q.conn)
	}

	return nil
}

func (q *Queue) cleanup() {
	q.log.Debug("In cleanup")

	// cancel context of all the workers
	q.cancel()

	/*q.mu.Lock()
	defer q.mu.Unlock()
	if q.conn != nil {
		q.log.Debug("closing the connection")
		_ = q.conn.Close()
	}*/

	q.conn = nil
}

func (q *Queue) Publish(queueName QueueName, message []byte) error {
	if q.conn == nil {
		return fmt.Errorf("connection is not open yet")
	}

	ch, err := q.conn.Channel()
	if err != nil {
		return fmt.Errorf("couldn't open channel", "error", err)
	}
	defer ch.Close()

	err = ch.Publish(
		"",                // exchange — empty means default (direct to queue)
		string(queueName), // routing key = queue name
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        message,
		},
	)
	if err != nil {
		q.log.Error("Failed to publish", "message", message, "error", err)
		return err
	}

	return nil
}
