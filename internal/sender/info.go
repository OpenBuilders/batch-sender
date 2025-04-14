package sender

import (
	"fmt"
	"time"

	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/google/uuid"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

func GetHighLoadWalletMsgInfo(extMsg *tlb.ExternalMessage) (*types.ExtMsgInfo, error) {
	body := extMsg.Payload()
	if body == nil {
		return nil, fmt.Errorf("nil body for external message")
	}

	hash := body.Hash()
	u, err := uuid.FromBytes(hash[:16])
	if err != nil {
		return nil, err
	}

	var msg struct {
		Sign    []byte     `tlb:"bits 512"`
		Payload *cell.Cell `tlb:"^"` // 1 referenced cell
	}
	err = tlb.LoadFromCell(&msg, body.BeginParse())
	if err != nil {
		return nil, err
	}
	var payload struct {
		SubwalletID uint32     `tlb:"## 32"`
		Msg         *cell.Cell `tlb:"^"` // this is your MustStoreRef
		Mode        uint8      `tlb:"## 8"`
		QueryID     uint64     `tlb:"## 23"`
		CreatedAt   uint64     `tlb:"## 64"`
		TTL         uint64     `tlb:"## 22"`
	}
	err = tlb.LoadFromCell(&payload, msg.Payload.BeginParse())
	if err != nil {
		return nil, err
	}

	return &types.ExtMsgInfo{
		UUID:      u,
		QueryID:   payload.QueryID,
		CreatedAt: time.Unix(int64(payload.CreatedAt), 0),
		ExpiredAt: time.Unix(int64(payload.CreatedAt+payload.TTL), 0),
		TTL:       payload.TTL,
	}, nil
}
