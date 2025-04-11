package model

import (
	"time"
)

type Batch struct {
	UUID           string    `db:"uuid"`
	TransactionIDs []int64   `db:"transaction_ids"`
	Status         string    `db:"status"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type Transaction struct {
	OrderID   string    `db:"order_id"`
	Wallet    string    `db:"wallet"`
	Amount    float64   `db:"amount"`
	Comment   string    `db:"comment"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
