package health

import (
	"context"
	"log/slog"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gitlab.com/tonstarter/stickers/internal/database"
)

type Config struct {
	RedisCheckInterval time.Duration
	DBCheckInterval    time.Duration
	ID                 string
}

type Component string

const (
	ComponentRedis = "redis"
	ComponentDB    = "db"
)

type CheckResult struct {
	Timestamp time.Time `json:"timestamp"`
	Result    bool      `json:"result"`
}

type HealthChecks map[Component]CheckResult

type HealthStatus struct {
	Healthy bool         `json:"healthy"`
	Checks  HealthChecks `json:"checks"`
}

type Checker struct {
	config *Config
	redis  *redis.Client
	db     *database.DB
	log    *slog.Logger
	checks HealthChecks
}

func NewChecker(redis *redis.Client, db *database.DB, config *Config) *Checker {
	return &Checker{
		config: config,
		redis:  redis,
		db:     db,
		log:    slog.With("pod", config.ID, "component", "health"),
		checks: HealthChecks{
			// if this code gets executed, we assume that there was an initial
			// check
			ComponentDB:    CheckResult{Timestamp: time.Now(), Result: true},
			ComponentRedis: CheckResult{Timestamp: time.Now(), Result: true},
		},
	}
}

func (c *Checker) Run(ctx context.Context) {
	c.log.Debug("Starting the health checker...")

	redisTicker := time.NewTicker(c.config.RedisCheckInterval)
	dbTicker := time.NewTicker(c.config.DBCheckInterval)

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("Stopping health checker ...")
			return
		case <-redisTicker.C:
			c.checkRedis(ctx)

		case <-dbTicker.C:
			c.checkDB(ctx)
		}
	}
}

func (c *Checker) checkRedis(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	_, err := c.redis.Ping(checkCtx).Result()

	c.checks[ComponentRedis] = CheckResult{
		Timestamp: time.Now(),
		Result:    err == nil,
	}
}

func (c *Checker) checkDB(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := c.db.IsUpAndRunning(checkCtx)

	c.checks[ComponentDB] = CheckResult{
		Timestamp: time.Now(),
		Result:    err == nil,
	}
}

func (c *Checker) GetHealthStatus() HealthStatus {
	healthy := true

	for component, check := range c.checks {
		if !check.Result {
			healthy = false
			c.log.Error("Component health check failed", "component", component)
		}
	}

	return HealthStatus{
		Healthy: healthy,
		Checks:  c.checks,
	}
}
