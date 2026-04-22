package commands

// CreateAccountCommand representa a intenção de criar uma nova conta.
// Commands representam "o que o usuário quer fazer" (escrita).
type CreateAccountCommand struct {
	UserID   string `json:"user_id"`
	Currency string `json:"currency"`
}
