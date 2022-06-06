package form

type RegisterForm struct {
	Name     string `validate:"min=2"`
	Email    string `validate:"required,email,min=6"`
	Password string `validate:"required,min=6"`
}
