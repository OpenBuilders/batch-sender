package types

type BatchStatus string

const (
	StatusNew     BatchStatus = "new"
	StatusPending BatchStatus = "pending"
	StatusSuccess BatchStatus = "success"
	StatusError   BatchStatus = "error"
)

type Transaction struct {
	ID      int64   `db:"id"`
	OrderID string  `db:"order_id"`
	Wallet  string  `db:"wallet"`
	Amount  float64 `db:"amount"`
	Comment string  `db:"comment"`
}

type DataItem struct {
	Wallet  string  `json:"wallet"`
	Amount  float64 `json:"amount"`
	Comment string  `json:"comment"`
}

type SendTONMessage struct {
	TransactionID string     `json:"transaction_id"`
	Data          []DataItem `json:"data"`
}

type Batch []Transaction
