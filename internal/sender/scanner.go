package sender

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"golang.org/x/sync/errgroup"
)

type TxScannerConfig struct {
	DBTimeout time.Duration
}

type TxScanner struct {
	config *TxScannerConfig
	client ton.APIClientWrapped
	repo   Repository
	wallet *Wallet
	log    *slog.Logger
}

func NewTxScanner(config *TxScannerConfig, client ton.APIClientWrapped,
	repo Repository, wallet *Wallet) *TxScanner {
	return &TxScanner{
		config: config,
		client: client,
		repo:   repo,
		wallet: wallet,
		log:    slog.With("component", "scanner"),
	}
}

func (s *TxScanner) Start(ctx context.Context) error {
	s.log.Debug("Starting tx scanner...")

	g, ctx := errgroup.WithContext(ctx)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.DBTimeout)
	defer cancel()

	lt, err := s.repo.GetLastWalletLt(ctxWithTimeout, s.wallet.Hash())
	if err != nil {
		return fmt.Errorf("last wallet lt error: %w", err)
	}

	s.log.Debug("wallet last lt", "wallet", s.wallet.Hash(), "lt", lt)

	txs := make(chan *tlb.Transaction)
	g.Go(func() error {
		s.client.SubscribeOnTransactions(ctx, s.wallet.GetAddress(), lt,
			txs)
		return fmt.Errorf("transaction subscriber exited")
	})

	g.Go(func() error {
		err := s.processNewTransactions(ctx, txs)
		if err != nil {
			return fmt.Errorf("transaction processor exited with error: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("tx scanner exited with error: %w", err)
	}

	return nil
}

func (s *TxScanner) processNewTransactions(ctx context.Context,
	txs <-chan *tlb.Transaction) error {
	walletHash := s.wallet.Hash()

	for tx := range txs {
		// process only external in messages
		if tx.IO.In.MsgType != tlb.MsgTypeExternalIn {
			continue
		}

		msg := tx.IO.In.AsExternalIn()

		s.log.Debug("New tx", "tx", tx)

		info, err := GetHighLoadWalletMsgInfo(msg)
		if err != nil {
			s.log.Error("parsing info error", "error", err)
			continue
		}

		// TODO(carterqw): add more checks on bounced etc
		s.log.Debug("parsed info", "info", info)

		success := false
		if desc, ok := tx.Description.(*tlb.TransactionDescriptionOrdinary); ok {
			success = !desc.Aborted
		}

		err = s.repo.PersistTransaction(ctx, tx.Hash, success, info)
		if err != nil {
			log.Fatalf("can't save tx confirmation: %v", err)
		}

		err = s.repo.UpdateLastWalletLt(ctx, walletHash, tx.LT)
		if err != nil {
			log.Fatalf("can't save tx confirmation: %v", err)
		}
	}

	return fmt.Errorf("transaction channel got closed")
}

func isEqualAddresses(address1, address2 *address.Address) bool {
	if address1.Workchain() == address2.Workchain() && bytes.Equal(address1.Data(), address2.Data()) {
		return true
	}
	return false
}
