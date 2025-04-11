package types

type Transaction struct {
	OrderID string
	Wallet  string
	Amount  float64
	Comment string
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
