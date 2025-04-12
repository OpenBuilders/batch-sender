package sender

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

type LightClientSenderConfig struct {
	Mnemonic string
}

type LightClientSender struct {
	config *LightClientSenderConfig
	client ton.APIClientWrapped
	wallet *wallet.Wallet
	log    *slog.Logger
}

func NewLightClientSender(config *LightClientSenderConfig,
	client ton.APIClientWrapped) *LightClientSender {
	return &LightClientSender{
		config: config,
		client: client,
		log:    slog.With("component", "lightclient-sender"),
	}
}

// TODO(carterqw):
// - [ ] initialize the wallet from the last query ID
// - [ ] store query id and tx hash per batch
// - [ ] balance checks
func (s *LightClientSender) Send(ctx context.Context, txs []types.Transaction) (string, error) {
	words := strings.Split(s.config.Mnemonic, " ")
	s.log.Debug("Words are", "words", words)

	q, err := FromQueryID(2)
	if err != nil {
		s.log.Error("couldn't restore query id generator", "error", err)
		return "", fmt.Errorf("query id generator error: %w", err)
	}

	// initialize high-load wallet
	w, err := wallet.FromSeed(s.client, words, wallet.ConfigHighloadV3{
		MessageTTL: 60 * 5,
		MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
			if !q.HasNext() {
				return 0, 0, fmt.Errorf("reached the limit of query id")
			}

			next, err := q.GetNext()
			if err != nil {
				return 0, 0, err
			}

			// Due to specific of externals emulation on liteserver,
			// we need to take something less than or equals to block time, as message creation time,
			// otherwise external message will be rejected, because time will be > than emulation time
			// hope it will be fixed in the next LS versions
			createdAt = time.Now().Unix() - 30

			return uint32(next.GetQueryID()), createdAt, nil
		},
	})

	if err != nil {
		s.log.Error("FromSeed error", "error", err)
		return "", fmt.Errorf("couldn't create wallet from seed")
	}

	s.log.Info("wallet address", "addr", w.WalletAddress())

	block, err := s.client.CurrentMasterchainInfo(context.Background())
	if err != nil {
		s.log.Error("CurrentMasterchainInfo error", "error", err)
		return "", fmt.Errorf("couldn't fetch master chain info: %w", err)
	}

	s.log.Debug("block", "data", block)

	balance, err := w.GetBalance(context.Background(), block)
	if err != nil {
		s.log.Error("GetBalance error", "error", err)
		return "", fmt.Errorf("GetBalance error: %w", err)
	}

	s.log.Debug("Balance", "value", balance)

	// TODO(carterqw): calculate the required balance
	if balance.Nano().Uint64() >= 3000000 {
		var messages []*wallet.Message
		for _, tx := range txs {
			// create comment cell to send in body of each message
			comment, err := wallet.CreateCommentCell(tx.Comment)
			if err != nil {
				s.log.Error("CreateComment error", "error", err)
				return "", fmt.Errorf("couldn't create comment: %w", err)
			}

			addr := address.MustParseAddr(tx.Wallet)

			messages = append(messages, &wallet.Message{
				Mode: wallet.PayGasSeparately + wallet.IgnoreErrors, // pay fee separately, ignore action errors
				InternalMessage: &tlb.InternalMessage{
					IHRDisabled: true, // disable hyper routing (currently not works in ton)
					Bounce:      addr.IsBounceable(),
					DstAddr:     addr,
					Amount:      tlb.MustFromTON(fmt.Sprintf("%f", tx.Amount)),
					Body:        comment,
				},
			})
		}

		s.log.Debug("sending transaction and waiting for confirmation...")

		// send transaction that contains all our messages, and wait for confirmation
		txHash, err := w.SendManyWaitTxHash(context.Background(), messages)
		if err != nil {
			s.log.Error("Transfer error", "error", err)
			return "", fmt.Errorf("transfer error: %w", err)
		}

		s.log.Debug(
			"transaction sent",
			"hash", base64.StdEncoding.EncodeToString(txHash),
		)
		s.log.Debug(
			"explorer link",
			"link", "https://testnet.tonscan.org/tx/"+
				base64.URLEncoding.EncodeToString(txHash),
		)
		return string(txHash), nil
	}

	return "", fmt.Errorf("not enough balance")
}
