package queue

import (
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueName string

const (
	QueueTONTransfer QueueName = "ton_transfer"
)

type Publisher struct {
	queueName QueueName
	conn      *amqp.Connection
	channel   *amqp.Channel
	log       *slog.Logger
}

func NewPublisher(conn *amqp.Connection, queueName QueueName) *Publisher {
	return &Publisher{
		queueName: queueName,
		conn:      conn,
		log:       slog.With("component", "queue-publisher", "queue", queueName),
	}
}

func (p *Publisher) Publish(message []byte) error {
	ch, err := EnsureQueueExists(p.conn, p.queueName)
	if err != nil {
		return err
	}

	p.channel = ch

	p.log.Debug("publishing a message", "message", message)

	err = p.channel.Publish(
		"",                  // exchange — empty means default (direct to queue)
		string(p.queueName), // routing key = queue name
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        message,
		},
	)
	if err != nil {
		p.log.Error("Failed to publish", "message", message, "error", err)
		return err
	}

	return nil
}

func EnsureQueueExists(conn *amqp.Connection, queueName QueueName) (
	*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Attempt passive declare (won't create the queue)
	_, err = ch.QueueDeclarePassive(
		string(queueName), // name
		true,              // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // args
	)

	if err != nil {
		// because of the error the channel we opened before got closed
		ch, err := conn.Channel()
		if err != nil {
			conn.Close()
			return nil, err
		}

		_, err = ch.QueueDeclare(
			string(queueName),
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			return nil, err
		}
	}

	return ch, nil
}
