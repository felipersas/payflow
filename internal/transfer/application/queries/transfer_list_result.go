package queries

type TransferListResult struct {
	Data    []TransferResult `json:"data"`
	Cursor  string           `json:"next_cursor,omitempty"`
	HasMore bool             `json:"has_more"`
}
