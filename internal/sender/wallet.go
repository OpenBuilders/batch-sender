package sender

import (
	"context"
	"fmt"
	"log/slog"
	// "math/rand"
	"strings"
	"sync"
	"time"

	"github.com/openbuilders/batch-sender/internal/helpers"
	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

type Wallet struct {
	client     ton.APIClientWrapped
	mnemonic   string
	isTestnet  bool
	messageTTL time.Duration
	queryID    *HighloadQueryID
	wallet     *wallet.Wallet
	mu         sync.Mutex
	log        *slog.Logger
}

func NewWallet(client ton.APIClientWrapped, mnemonic string, isTestnet bool,
	messageTTL time.Duration, queryID *HighloadQueryID) (*Wallet, error) {
	wallet := &Wallet{
		client:     client,
		mnemonic:   mnemonic,
		isTestnet:  isTestnet,
		messageTTL: messageTTL,
		queryID:    queryID,
		log:        slog.With("component", "wallet"),
	}

	err := wallet.init()
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// PrepareMessage generates an external message containing all the transfers
// from the passed batch.
func (w *Wallet) PrepareMessage(ctx context.Context, batch *types.Batch) (
	*tlb.ExternalMessage, error) {
	var messages []*wallet.Message

	for _, transfer := range batch.Transfers {
		addr := address.MustParseAddr(transfer.Wallet)
		inMsg, err := w.wallet.BuildTransfer(
			addr,
			tlb.MustFromTON(fmt.Sprintf("%f", transfer.Amount)),
			addr.IsBounceable(),
			transfer.Comment,
		)
		if err != nil {
			return nil, fmt.Errorf("build internal message error: %w", err)
		}

		w.log.Debug("Transfer internal message", "msg", *inMsg, "inMsg", *inMsg.InternalMessage)
		messages = append(messages, inMsg)
	}

	return w.wallet.BuildExternalMessageForMany(ctx, messages)
}

// SendMessage broadcasts the message created by the PrepareMessage method.
func (w *Wallet) SendMessage(ctx context.Context, extMsg *tlb.ExternalMessage) error {
	/*return fmt.Errorf("No sending today")
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// 50% chance
	if rand.Intn(2) != 0 {
		return fmt.Errorf("out of luck, not sending")
	}*/

	block, err := w.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return fmt.Errorf("master chain info error: %w", err)
	}

	balance, err := w.wallet.GetBalance(ctx, block)
	if err != nil {
		return fmt.Errorf("GetBalance error: %w", err)
	}

	w.log.Debug("Balance", "value", balance)

	if balance.Nano().Uint64() < 3_000_000 {
		return fmt.Errorf("not enough balance")
	}

	w.log.Debug("sending transaction...")

	err = w.client.SendExternalMessage(ctx, extMsg)
	if err != nil {
		return fmt.Errorf("send ext message error: %w", err)
	}

	return nil
}

// Hash returns a 6-character hash of the wallet based on its' mnemonic.
func (w *Wallet) Hash() string {
	return helpers.TinyHash(w.mnemonic)
}

// GetAddress returns the wallet address.
func (w *Wallet) GetAddress() *address.Address {
	return w.wallet.WalletAddress()
}

// Init initializes the wallet from the mnemonic and restores the state of the
// query ID.
func (w *Wallet) init() error {
	words := strings.Split(w.mnemonic, " ")

	// initialize high-load wallet
	newWallet, err := wallet.FromSeed(w.client, words, wallet.ConfigHighloadV3{
		MessageTTL: uint32(w.messageTTL.Seconds()),
		MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
			// Due to specific of externals emulation on liteserver,
			// we need to take something less than or equals to block time, as message creation time,
			// otherwise external message will be rejected, because time will be > than emulation time
			// hope it will be fixed in the next LS versions
			createdAt = time.Now().Unix() - 30

			w.mu.Lock()
			defer w.mu.Unlock()

			// send transaction that contains all our messages, and wait for confirmation
			if !w.queryID.HasNext() {
				return 0, 0, fmt.Errorf("reached the limit of query id")
			}

			next, err := w.queryID.GetNext()
			if err != nil {
				return 0, 0, err
			}

			w.queryID = next

			return uint32(w.queryID.GetQueryID()), createdAt, nil
		},
	})
	if err != nil {
		return fmt.Errorf("couldn't create wallet from seed: %w", err)
	}

	w.wallet = newWallet

	return nil
}
