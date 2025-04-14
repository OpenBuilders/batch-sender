package sender

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	// "github.com/xssnick/tonutils-go/tvm/cell"
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

func (s *TxScanner) Start(ctx context.Context) {
	s.log.Debug("Starting tx scanner...")

	ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.DBTimeout)
	defer cancel()

	lt, err := s.repo.GetLastWalletLt(ctxWithTimeout, s.wallet.Hash())
	if err != nil {
		s.log.Error("last wallet lt error", "error", err)
	}

	txs := make(chan *tlb.Transaction)
	go s.client.SubscribeOnTransactions(ctx, s.wallet.GetAddress(), lt, txs)
	go s.processNewTransactions(ctx, txs)

	s.log.Debug("Last processed LT", "wallet", s.wallet.Hash(), "lt", lt)
}

func (s *TxScanner) processNewTransactions(ctx context.Context,
	txs <-chan *tlb.Transaction) {

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

		err = s.repo.PersistTransaction(ctx, tx.Hash, info)
		if err != nil {
			log.Fatalf("can't save tx confirmation: %v", err)
		}

		err = s.repo.UpdateLastWalletLt(ctx, walletHash, tx.LT)
		if err != nil {
			log.Fatalf("can't save tx confirmation: %v", err)
		}
	}
}

func isEqualAddresses(address1, address2 *address.Address) bool {
	if address1.Workchain() == address2.Workchain() && bytes.Equal(address1.Data(), address2.Data()) {
		return true
	}
	return false
}
