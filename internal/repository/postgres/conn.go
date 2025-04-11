package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pg          *pgxpool.Pool
	pingTimeout time.Duration
	log         *slog.Logger
}

func New(pool *pgxpool.Pool, pingTimeout time.Duration) *Postgres {
	return &Postgres{
		pg:          pool,
		pingTimeout: pingTimeout,
		log:         slog.With("component", "db"),
	}
}

func (p *Postgres) Ping(ctx context.Context) error {
	timeout := 5 * time.Second

	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	var err error
	// Ping 3 times with a specified time interval.
	for i := 1; i <= 3; i++ {
		// Maximum allowed context lifetime for ping, slightly less than the duration of the test cycle.
		// If the ping process is not successful, it hangs indefinitely, so it's important to limit the context with a timeout.
		pingCtx, cancel := context.WithTimeout(ctx, timeout-time.Millisecond*10)
		if err = p.pg.Ping(pingCtx); err == nil {
			cancel()

			return nil
		} else {
			slog.Info("ping attempt was not successful", "attempt", i)
		}
		cancel()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return err
}
