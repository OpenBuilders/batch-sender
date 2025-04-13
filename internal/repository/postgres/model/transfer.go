package model

import (
	"time"

	"github.com/openbuilders/batch-sender/internal/types"
)

type Batch struct {
	UUID           string            `db:"uuid"`
	TransactionIDs []int64           `db:"transaction_ids"`
	Status         types.BatchStatus `db:"status"`
	CreatedAt      time.Time         `db:"created_at"`
	UpdatedAt      time.Time         `db:"updated_at"`
}

type Transaction struct {
	ID        int64     `db:"id"`
	OrderID   string    `db:"order_id"`
	Wallet    string    `db:"wallet"`
	Amount    float64   `db:"amount"`
	Comment   string    `db:"comment"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
