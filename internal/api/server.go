package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/openbuilders/batch-sender/internal/batcher"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// APIHandler is a custom handler type that returns data or an error
type APIHandler func(w http.ResponseWriter, r *http.Request) (interface{}, error)

type Server struct {
	config     *Config
	batcher    *batcher.Batcher
	httpServer *http.Server
	ctx        context.Context
	log        *slog.Logger
}

type Config struct {
	ListenAddr   string
	ListenPort   int
	MetricsPort  int
	ProbesPort   int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	ID           string
}

func NewServer(config *Config, batcher *batcher.Batcher) *Server {
	return &Server{
		config: config,
		batcher: batcher,
		log:    slog.With("pod", config.ID, "component", "web-server"),
		httpServer: &http.Server{
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			IdleTimeout:  config.IdleTimeout,
		},
	}
}

func (s *Server) StartProbesAndMetrics() {
	// Expose Prometheus metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		slog.Info("Serving metrics", "port", s.config.MetricsPort)

		addr := fmt.Sprintf(":%d", s.config.MetricsPort)
		slog.Error("Prometheus HTTP listener failed", "error",
			http.ListenAndServe(addr, nil))
	}()

	// Expose health probes
	go func() {
		http.Handle("/health", WithMethod(
			WithJSONResponse(s.HealthHandler),
			http.MethodGet,
		))

		http.Handle("/ready", WithMethod(
			WithJSONResponse(s.ReadinessHandler),
			http.MethodGet,
		))

		slog.Info("Serving health probes", "port", s.config.ProbesPort)

		addr := fmt.Sprintf(":%d", s.config.ProbesPort)
		slog.Error("Health checks HTTP listener failed", "error",
			http.ListenAndServe(addr, nil))
	}()
}

func (s *Server) Start(ctx context.Context, stop <-chan os.Signal) {
	s.StartProbesAndMetrics()

	mux := http.NewServeMux()

	// The order of middleware calls is up to bottom, first WithCORS is called,
	// then WithMethod and so on.
	mux.HandleFunc("/job", WithMethod(
		WithJSONResponse(s.JobHandler),
		http.MethodPost,
	))

	s.httpServer.Handler = http.TimeoutHandler(mux, s.config.WriteTimeout, "Timeout")

	go s.run(ctx)

	<-stop

	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exiting")
}

func (s *Server) run(ctx context.Context) {
	s.ctx = ctx

	slog.Info("Starting server", "port", s.config.ListenPort)

	// Use ListenConfig to create a listener with context support
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", s.config.ListenAddr, s.config.ListenPort))
	if err != nil {
		slog.Error("Error creating listener", "error", err)
	}
	defer listener.Close()

	if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		slog.Error("Could not start server", "error", err.Error())
	}
}
