package domain

type Error struct {
	Status  int                `json:"status"`
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Fields  *map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

var ErrRecordNotFound = &Error{Status: 404, Code: "not-found", Message: "The requested entity does not exist"}
var ErrForbidden = &Error{Status: 403, Code: "forbidden", Message: "Access denied"}
