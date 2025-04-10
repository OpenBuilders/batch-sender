package types

type Transfer struct {
	Wallet  string  `json:"wallet"`
	Amount  float64 `json:"amount"`
	Comment string  `json:"comment"`
}

type Transaction struct {
	TransactionID string     `json:"transaction_id"`
	Data          []Transfer `json:"data"`
}
