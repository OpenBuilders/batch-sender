package types

import (
	"math"

	"github.com/google/uuid"
)

type BatchStatus string

const (
	BatchStatusNew     BatchStatus = "new"
	BatchStatusPending BatchStatus = "pending"
	BatchStatusSuccess BatchStatus = "success"
	BatchStatusError   BatchStatus = "error"
)

type TransferStatus string

const (
	TransferStatusNew     TransferStatus = "new"
	TransferStatusBatched TransferStatus = "batched"
	TransferStatusSuccess TransferStatus = "success"
	TransferStatusError   TransferStatus = "error"
)

type Transfer struct {
	ID      int64   `db:"id"`
	OrderID string  `db:"order_id"`
	Wallet  string  `db:"wallet"`
	Amount  float64 `db:"amount"`
	Comment string  `db:"comment"`
}

type BatchResult struct {
	BatchUUID string      `db:"batch_uuid"`
	OrderID   string      `db:"order_id"`
	TxHash    string      `db:"tx_hash"`
	Status    BatchStatus `db:"status"`
}

type DataItem struct {
	Wallet  string  `json:"wallet"`
	Amount  float64 `json:"amount"`
	Comment string  `json:"comment"`
}

type SendTONMessage struct {
	Pattern string `json:"pattern"`
	Data    struct {
		TransactionID string     `json:"transaction_id"`
		Data          []DataItem `json:"data"`
	} `json:"data"`
}

type Batch struct {
	UUID      uuid.UUID
	Transfers []Transfer
}

func (b *Batch) GetTotalNano() uint64 {
	total := uint64(0)

	for _, tr := range b.Transfers {
		total += uint64(tr.Amount * math.Pow(10, 9))
	}

	return total
}
