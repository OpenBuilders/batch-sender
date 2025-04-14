package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openbuilders/batch-sender/internal/api"
	"github.com/openbuilders/batch-sender/internal/batcher"
	"github.com/openbuilders/batch-sender/internal/env"
	"github.com/openbuilders/batch-sender/internal/log"
	"github.com/openbuilders/batch-sender/internal/queue"
	"github.com/openbuilders/batch-sender/internal/repository/postgres"
	"github.com/openbuilders/batch-sender/internal/sender"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"golang.org/x/sync/errgroup"
)

func main() {
	logLevel := env.GetString("LOG_LEVEL", "INFO")
	log.Setup(logLevel)

	listenPort := env.GetInt("LISTEN_PORT", 8090)
	probesPort := env.GetInt("PROBES_PORT", 8081)
	metricsPort := env.GetInt("METRICS_PORT", 9091)
	rabbitURL := env.GetString("RABBIT_URL",
		"amqp://guest:guest@rabbitmq:5672/")
	postgresURL := env.GetString("POSTGRES_URL",
		"postgres://postgres:dev@db:5432/postgres?connect_timeout=1")
	lightClientConfig := env.GetString("LIGHTCLIENT_CONFIG",
		"https://ton.org/testnet-global.config.json")
	mnemonic := env.GetString("MNEMONIC", "")
	isTestnet := env.GetBool("IS_TESTNET", true)

	slog.Info("Connecting to RabbitMQ...")

	rabbitConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		slog.Error("connect to RabbitMQ", "error", err)
		return
	}
	defer rabbitConn.Close()

	// create the context and register signals that could cause its cancellation
	// and gracefull shutdown
	ctx, _ := signal.NotifyContext(
		context.Background(),
		os.Interrupt, os.Kill,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)

	slog.Info("Connecting to Postgres...")

	pg, err := pgxpool.New(ctx, postgresURL)
	if err != nil {
		slog.Error("connect to Postgres", "error", err)
		return
	}

	pgClient := postgres.New(pg, 1*time.Second)

	err = pgClient.Ping(ctx)
	if err != nil {
		slog.Error("check Postgres connection", "error", err)
		return
	}

	publisher := queue.NewPublisher(rabbitConn, queue.QueueSendTON)

	instanceID := getInstanceID()

	//healthChecker := health.NewChecker(redisClient, db, &health.Config{
	//	ID:                 instanceID,
	//})
	config := api.Config{
		ListenAddr:   "",
		ListenPort:   listenPort,
		MetricsPort:  metricsPort,
		ProbesPort:   probesPort,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
		ID:           instanceID,
	}

	batcher := batcher.New(&batcher.Config{
		BatchSize:       200,
		BatchTimeout:    1 * time.Second,
		BatchDelay:      10 * time.Millisecond,
		DBTimeout:       3 * time.Second,
		ParallelBatches: 5,
	}, rabbitConn, pgClient)

	client := liteclient.NewConnectionPool()

	// connect to testnet lite server
	err = client.AddConnectionsFromConfigUrl(context.Background(), lightClientConfig)
	if err != nil {
		slog.Error("couldn't add connection to lite client", "error", err)
		return
	}

	lightclientAPI := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()

	sender := sender.New(&sender.Config{
		NumWorkers:              5,
		DBTimeout:               3 * time.Second,
		MessageTTL:              300 * time.Second,
		ExpirationCheckInterval: 30 * time.Second,
	}, lightclientAPI, mnemonic, isTestnet, pgClient, batcher.Batches)

	server := api.NewServer(&config, publisher)

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)

	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		// when the app is interrupted, the signal will be sent to the stop channel
		waitForShutdown(ctx, stop)
		return nil
	})

	errGroup.Go(func() error {
		server.Start(ctx, stop)
		return nil
	})

	errGroup.Go(func() error {
		err := batcher.Run(ctx)
		if err != nil {
			slog.Error("Batcher exited with an error", "error", err)
			return err
		}

		return nil
	})

	errGroup.Go(func() error {
		err := sender.Run(ctx)
		if err != nil {
			slog.Error("Sender exited with an error", "error", err)
			return err
		}

		return nil
	})

	if err := errGroup.Wait(); err != nil {
		slog.Error("batch sender exited with an error", "error", err)
	}
}

func waitForShutdown(ctx context.Context, stop chan<- os.Signal) {
	for {
		select {
		case <-ctx.Done():
			slog.Debug("Received a graceful shutdown request")
			stop <- os.Kill
			return
		}
	}
}

func getInstanceID() string {
	instanceID := env.GetString("POD_NAME", "")

	if instanceID == "" {
		rand.Seed(time.Now().UnixNano())
		instanceID = fmt.Sprint(rand.Intn(math.MaxUint32))
	}

	return instanceID
}
