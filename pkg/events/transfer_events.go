package events

type TransferOcurred struct {
	Event         BaseEvent
	TransferID    string
	FromAccountID string
	ToAccountID   string
	Amount        int64
	Currency      string
	Status        string
}
