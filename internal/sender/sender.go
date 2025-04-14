package sender

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/openbuilders/batch-sender/internal/helpers"
	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/google/uuid"
	"github.com/xssnick/tonutils-go/ton"

	"golang.org/x/sync/errgroup"
)

const (
	ErrorCountTolerated = 5
)

type Config struct {
	NumWorkers              int64
	DBTimeout               time.Duration
	MessageTTL              time.Duration
	ExpirationCheckInterval time.Duration
}

type MessageBuilderFunc func(context.Context, uint32) (uint32, int64, error)

type TransactionSender interface {
	Send(context.Context, []types.Transfer) (string, error)
}

type Sender struct {
	batches   chan uuid.UUID
	client    ton.APIClientWrapped
	mnemonic  string
	isTestnet bool
	config    *Config
	wallet    *Wallet
	scanner   *TxScanner
	repo      Repository
	log       *slog.Logger
}

type Repository interface {
	GetNewBatches(context.Context) ([]uuid.UUID, error)
	ResubmitExpiredBatches(context.Context, time.Duration) ([]uuid.UUID, error)
	GetTransfersBatch(context.Context, uuid.UUID) (*types.Batch, error)
	UpdateBatchStatus(context.Context, uuid.UUID, types.BatchStatus) error
	GetLastQueryID(context.Context, string) (uint64, error)
	PersistWallet(context.Context, string, string, string) error
	MarkBatchAsProcessing(context.Context, string, uuid.UUID, *types.ExtMsgInfo) error
	GetLastWalletLt(context.Context, string) (uint64, error)
	UpdateLastWalletLt(context.Context, string, uint64) error
	PersistTransaction(context.Context, []byte, *types.ExtMsgInfo) error
}

func New(config *Config, client ton.APIClientWrapped, mnemonic string,
	isTestnet bool, repo Repository, batches chan uuid.UUID) *Sender {
	return &Sender{
		batches:   batches,
		client:    client,
		mnemonic:  mnemonic,
		isTestnet: isTestnet,
		config:    config,
		repo:      repo,
		log:       slog.With("component", "sender"),
	}
}

func (s *Sender) Run(ctx context.Context) error {
	s.log.Info("Starting sender")

	err := s.initWallet(ctx)
	if err != nil {
		s.log.Error("couldn't initialize wallet", "error", err)
		return err
	}

	txScanner := NewTxScanner(&TxScannerConfig{
		DBTimeout: s.config.DBTimeout,
	}, s.client, s.repo, s.wallet)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := txScanner.Start(ctx)
		if err != nil {
			return fmt.Errorf("tx scanner start error: %w", err)
		}

		return nil
	})

	go s.backfillFromDB()

	g.Go(func() error {
		err := s.rebatchExpired(ctx)
		if err != nil {
			return fmt.Errorf("expired rebatcher error: %w", err)
		}

		return nil
	})

	for i := 0; i < int(s.config.NumWorkers); i++ {
		g.Go(func() error {
			err := s.worker(ctx, i, s.batches)
			if err != nil {
				return fmt.Errorf(
					"sender worker #%d exited with error: %w", i, err)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("sender exited with error: %w", err)
	}

	s.log.Info("Stopped sender")

	return nil
}

func (s *Sender) initWallet(ctx context.Context) error {
	hash := helpers.TinyHash(s.mnemonic)
	s.log.Debug("Mnemonic hash", "hash", hash)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.DBTimeout)
	defer cancel()

	lastQueryID, err := s.repo.GetLastQueryID(ctxWithTimeout, hash)
	if err != nil {
		s.log.Error(
			"couldn't fetch last query ID",
			"wallet", hash,
			"error", err,
		)
		return err
	}

	s.log.Debug("last query ID", "wallet", hash, "lastQueryID", lastQueryID)

	highloadQueryID, err := FromQueryID(lastQueryID)
	if err != nil {
		return fmt.Errorf("couldn't create highload query ID: %w", err)
	}

	// initialize high-load wallet
	wallet, err := NewWallet(s.client, s.mnemonic, s.isTestnet,
		s.config.MessageTTL, highloadQueryID)
	if err != nil {
		return fmt.Errorf("couldn't create wallet: %w", err)
	}

	address := wallet.GetAddress()
	s.log.Debug("Wallet address", "address", address.Testnet(s.isTestnet))

	ctxWithTimeout, cancel = context.WithTimeout(ctx, s.config.DBTimeout)
	defer cancel()

	err = s.repo.PersistWallet(
		ctxWithTimeout, hash,
		address.Testnet(false).String(),
		address.Testnet(true).String(),
	)
	if err != nil {
		return fmt.Errorf("wallet persistence error: %w", err)
	}

	s.wallet = wallet

	return nil
}

func (s *Sender) worker(ctx context.Context, id int, batches <-chan uuid.UUID) error {
	s.log.Info("Starting sender worker", "id", id)

	walletHash := s.wallet.Hash()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Stopping sender worker...", "id", id)
			return nil
		case batchUUID, ok := <-s.batches:
			if !ok {
				s.log.Debug("Batches channel is closed")
				return fmt.Errorf("batches channel is closed")
			}

			s.log.Debug("Received a new batch", "uuid", batchUUID)

			ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.DBTimeout)
			defer cancel()

			batch, err := s.repo.GetTransfersBatch(ctxWithTimeout, batchUUID)
			if err != nil {
				s.log.Error(
					"couldn't get batch txs",
					"uuid", batchUUID,
					"error", err,
				)
				continue
			}

			s.log.Debug("Got transfers batch", "batch", batch)

			msg, err := s.wallet.PrepareMessage(ctx, batch)
			if err != nil {
				s.log.Error(
					"preparing message failed",
					"batch", batch,
					"error", err,
				)
				continue
			}

			info, err := GetHighLoadWalletMsgInfo(msg)
			if err != nil {
				s.log.Error(
					"get external message ttl error",
					"message", msg,
					"error", err,
				)
			}

			s.log.Debug("Info", "info", info)

			ctxWithTimeout, cancel = context.WithTimeout(ctx, s.config.DBTimeout)
			defer cancel()

			err = s.repo.MarkBatchAsProcessing(ctxWithTimeout, walletHash,
				batch.UUID, info)
			if err != nil {
				s.log.Error(
					"couldn't persist external message",
					"wallet", walletHash,
					"batch", batch.UUID,
					"info", info,
				)

				continue
			}

			err = s.wallet.SendMessage(ctx, msg)
			if err != nil {
				s.log.Error(
					"sending message failed",
					"batch", batch,
					"msg", msg,
					"error", err,
				)
				continue
			}
		}
	}

	return nil
}

// backfillFromDB reads all batches in the 'new' status and writes them to the
// same channel for processing
func (s *Sender) backfillFromDB() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), s.config.DBTimeout)
		defer cancel()

		uuids, err := s.repo.GetNewBatches(ctx)
		if err != nil {
			s.log.Error(
				"couldn't get new batches",
				"error", err,
			)
		}

		s.log.Debug("Got batch uuids", "uuids", uuids)
		for _, uuid := range uuids {
			s.batches <- uuid
		}

		s.log.Debug("Backfilling completed", "count", len(uuids))
		return
	}
}

// rebatchExpired periodically checks the database for batches which external
// messages have expired already and still haven't been confirmed.
func (s *Sender) rebatchExpired(ctx context.Context) error {
	s.log.Debug("Starting expiration checker")

	checkInterval := time.Duration(0)

	errorCount := 0

	for {
		select {
		case <-ctx.Done():
			s.log.Debug("Shutting down expiration checker")
			return nil
		case <-time.After(checkInterval):
			s.log.Debug("Checking expired batches")
			checkInterval = s.config.ExpirationCheckInterval
		}

		ctxWithTimeout, cancel := context.WithTimeout(context.Background(),
			s.config.DBTimeout)
		defer cancel()

		uuids, err := s.repo.ResubmitExpiredBatches(ctxWithTimeout,
			s.config.MessageTTL)
		if err != nil {
			s.log.Error(
				"couldn't resubmit expired batches",
				"error", err,
			)

			errorCount += 1
			if errorCount >= ErrorCountTolerated {
				s.log.Error(
					"reached max error count, exiting",
					"count", errorCount,
				)
				return fmt.Errorf("max error count reached")
			}

			continue
		}

		s.log.Info("Expired batches UUIDs", "uuids", uuids)
		for _, uuid := range uuids {
			s.batches <- uuid
		}
		errorCount = 0
	}

	return nil
}
