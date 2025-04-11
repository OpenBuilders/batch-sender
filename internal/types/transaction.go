package types

type Transfer struct {
	Wallet  string  `json:"wallet"`
	Amount  float64 `json:"amount"`
	Comment string  `json:"comment"`
}

type TXTransfer struct {
	Transfer
	TransactionID string `json:"transaction_id"`
}

type Transaction struct {
	ID   string     `json:"transaction_id"`
	Data []Transfer `json:"data"`
}

type Batch []TXTransfer
