package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/openbuilders/batch-sender/internal/api"
	"github.com/openbuilders/batch-sender/internal/batcher"
	"github.com/openbuilders/batch-sender/internal/env"
	"github.com/openbuilders/batch-sender/internal/log"
)

func main() {
	logLevel := env.GetString("LOG_LEVEL", "INFO")
	log.Setup(logLevel)

	slog.Debug("Foo")

	listenPort := env.GetInt("LISTEN_PORT", 8090)
	probesPort := env.GetInt("PROBES_PORT", 8081)
	metricsPort := env.GetInt("METRICS_PORT", 9091)

	// create the context and register signals that could cause its cancellation
	// and gracefull shutdown
	ctx, _ := signal.NotifyContext(
		context.Background(),
		os.Interrupt, os.Kill,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)

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
		BatchSize: 200,
		BatchTimeout: 10 * time.Second,
	})

	server := api.NewServer(&config, batcher)

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	// when the app is interrupted, the signal will be sent to the stop channel
	go func() {
		defer wg.Done()
		waitForShutdown(ctx, stop)
	}()

	go func() {
		defer wg.Done()
		server.Start(ctx, stop)
	}()

	wg.Wait()
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
