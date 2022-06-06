package form

type LoginForm struct {
	Email    string `validate:"required,email,min=6"`
	Password string `validate:"required,min=6"`
}
