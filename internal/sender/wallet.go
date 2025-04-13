package sender

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/openbuilders/batch-sender/internal/helpers"
	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/google/uuid"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

type Wallet struct {
	client    ton.APIClientWrapped
	mnemonic  string
	isTestnet bool
	queryID   *HighloadQueryID
	wallet    *wallet.Wallet
	mu        sync.Mutex
	log       *slog.Logger
}

type TransactionResult struct {
	Success    bool
	BatchUUID  uuid.UUID
	Attempt    int64
	WalletHash string
	Hash       string
	QueryID    uint64
	Error      string
}

func NewWallet(client ton.APIClientWrapped, mnemonic string, isTestnet bool,
	queryID *HighloadQueryID) *Wallet {
	return &Wallet{
		client:    client,
		mnemonic:  mnemonic,
		isTestnet: isTestnet,
		queryID:   queryID,
		log:       slog.With("component", "wallet"),
	}
}

func (w *Wallet) Hash() string {
	return helpers.TinyHash(w.mnemonic)
}

func (w *Wallet) Init() error {
	words := strings.Split(w.mnemonic, " ")

	// initialize high-load wallet
	newWallet, err := wallet.FromSeed(w.client, words, wallet.ConfigHighloadV3{
		MessageTTL: 60 * 5,
		MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
			// Due to specific of externals emulation on liteserver,
			// we need to take something less than or equals to block time, as message creation time,
			// otherwise external message will be rejected, because time will be > than emulation time
			// hope it will be fixed in the next LS versions
			createdAt = time.Now().Unix() - 30

			return uint32(w.queryID.GetQueryID()), createdAt, nil
		},
	})
	if err != nil {
		return fmt.Errorf("couldn't create wallet from seed: %w", err)
	}

	w.wallet = newWallet
	return nil
}

func (w *Wallet) GetAddress() (*address.Address, error) {
	if w.wallet == nil {
		return nil, fmt.Errorf("wallet is not initialized")
	}

	return w.wallet.WalletAddress().Testnet(w.isTestnet), nil
}

func (w *Wallet) Send(ctx context.Context, batch *types.Batch, retries int64, timeout time.Duration) (*TransactionResult, error) {
	block, err := w.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		w.log.Error("CurrentMasterchainInfo error", "error", err)
		return nil, fmt.Errorf("couldn't fetch master chain info: %w", err)
	}

	balance, err := w.wallet.GetBalance(ctx, block)
	if err != nil {
		w.log.Error("GetBalance error", "error", err)
		return nil, fmt.Errorf("GetBalance error: %w", err)
	}

	w.log.Debug("Balance", "value", balance)

	if balance.Nano().Uint64() >= 3000000+batch.GetTotalNano() {
		var messages []*wallet.Message
		for _, transfer := range batch.Transfers {
			// create comment cell to send in body of each message
			comment, err := wallet.CreateCommentCell(transfer.Comment)
			if err != nil {
				w.log.Error("CreateComment error", "error", err)
				return nil, fmt.Errorf("couldn't create comment: %w", err)
			}

			addr := address.MustParseAddr(transfer.Wallet)

			messages = append(messages, &wallet.Message{
				Mode: wallet.PayGasSeparately + wallet.IgnoreErrors, // pay fee separately, ignore action errors
				InternalMessage: &tlb.InternalMessage{
					IHRDisabled: true, // disable hyper routing (currently not working in ton)
					Bounce:      addr.IsBounceable(),
					DstAddr:     addr,
					Amount:      tlb.MustFromTON(fmt.Sprintf("%f", transfer.Amount)),
					Body:        comment,
				},
			})
		}

		w.log.Debug("sending transaction and waiting for confirmation...")

		w.mu.Lock()

		queryID := w.queryID.GetQueryID()

		// send transaction that contains all our messages, and wait for confirmation
		if !w.queryID.HasNext() {
			return nil, fmt.Errorf("reached the limit of query id")
		}

		next, err := w.queryID.GetNext()
		if err != nil {
			return nil, err
		}

		w.queryID = next

		w.mu.Unlock()

		var attempt int64
		var tx *tlb.Transaction

		for attempt = 1; attempt < retries; attempt++ {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			tx, _, err = w.wallet.SendManyWaitTransaction(ctxWithTimeout, messages)
			if err == nil {
				break
			}

			w.log.Error(
				"transaction error",
				"attempt", attempt,
				"tx", tx,
				"error", err,
			)
		}

		if err != nil {
			return &TransactionResult{
				Success:    false,
				WalletHash: w.Hash(),
				Attempt:    attempt,
				QueryID:    queryID,
				Error:      err.Error(),
			}, nil
		}

		w.log.Debug(
			"transaction sent",
			"transaction", tx,
			"hash", base64.StdEncoding.EncodeToString(tx.Hash),
		)
		w.log.Debug(
			"explorer link",
			"link", "https://testnet.tonscan.org/tx/"+
				base64.URLEncoding.EncodeToString(tx.Hash),
		)
		return &TransactionResult{
			Success:    true,
			WalletHash: w.Hash(),
			Hash:       string(tx.Hash),
			QueryID:    queryID,
		}, nil
	}

	return nil, fmt.Errorf("not enough balance")
}
