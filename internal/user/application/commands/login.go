package commands

// LoginCommand representa a intenção de autenticar um usuário.
type LoginCommand struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
