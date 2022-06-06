package form

type UpdateUserForm struct {
	Name  string `validate:"min=2"`
	Email string `validate:"required,email,min=6"`
}
