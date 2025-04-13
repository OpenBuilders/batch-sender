package sender

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/openbuilders/batch-sender/internal/helpers"
	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/google/uuid"
	"github.com/xssnick/tonutils-go/ton"
)

type Config struct {
	NumWorkers  int64
	NumRetries  int64
	SendTimeout time.Duration
	DBTimeout   time.Duration
	MessageTTL  time.Duration
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
	repo      Repository
	log       *slog.Logger
}

type Repository interface {
	GetNewBatches(context.Context) ([]uuid.UUID, error)
	GetTransfersBatch(context.Context, uuid.UUID) (*types.Batch, error)
	UpdateBatchStatus(context.Context, uuid.UUID, types.BatchStatus) error
	GetLastQueryID(context.Context, string) (uint64, error)
	PersistWallet(context.Context, string, string, string) error
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

	go s.backfillFromDB()

	var wg sync.WaitGroup
	for i := 0; i < int(s.config.NumWorkers); i++ {
		wg.Add(1)
		go s.worker(ctx, i, s.batches, &wg)
	}

	wg.Wait()
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
	wallet := NewWallet(s.client, s.mnemonic, s.isTestnet, highloadQueryID)
	err = wallet.Init()
	if err != nil {
		return fmt.Errorf("couldn't create wallet: %w", err)
	}

	address, err := wallet.GetAddress()
	if err != nil {
		return fmt.Errorf("get address error: %w", err)
	}

	s.log.Debug("Wallet address", "address", address)

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

func (s *Sender) worker(ctx context.Context, id int, batches <-chan uuid.UUID,
	wg *sync.WaitGroup) {
	defer wg.Done()

	s.log.Info("Starting sender worker", "id", id)
	for {
		select {
		case <-ctx.Done():
			s.log.Info("Stopping sender worker...", "id", id)
			return
		case batchUUID, ok := <-s.batches:
			if !ok {
				s.log.Debug("Batches channel is closed")
				return
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

			result, err := s.wallet.Send(ctx, batch, s.config.NumRetries, s.config.SendTimeout)
			if err != nil {
				s.log.Error(
					"sending tx failed",
					"batch", batch,
					"error", err,
				)
				//TODO(carterqw): handle errors
				continue
			}

			s.log.Debug("Got tx result", "result", result)

			ctxWithTimeout, cancel = context.WithTimeout(ctx, s.config.DBTimeout)
			defer cancel()

			//err = s.repo.PersistTransaction(ctxWithTimeout, result)
			//if err

			/* err = s.repo.UpdateBatchStatus(context.Background(), batchUUID, types.StatusPending)
			if err != nil {
				s.log.Error("couldn't update batch")
				continue
			}*/
		}
	}
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
