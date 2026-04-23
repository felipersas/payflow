package commands

// RegisterCommand representa a intenção de registrar um novo usuário.
type RegisterCommand struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
