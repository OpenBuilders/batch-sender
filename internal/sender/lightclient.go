package sender

import (
	"fmt"

	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/xssnick/tonutils-go/ton"
)

type LightClientSenderConfig struct {
}

type LightClientSender struct {
	config *LightClientSenderConfig
	client *ton.APIClient
}

func NewLightClientSender(config *LightClientSenderConfig,
	client *ton.APIClient) *LightClientSender {
	return &LightClientSender{
		config: config,
		client: client,
	}
}

func (s *LightClientSender) Send(txs []types.Transaction) (string, error) {
	return "", fmt.Errorf("Not implemented")
}
