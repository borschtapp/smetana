package book

import (
	"borscht.app/smetana/model"
)

// Service is an interface from which our api module can access our repository of all our models
type Service interface {
	FetchBooks() (*[]model.Book, error)
	FindBook(ID uint) (*model.Book, error)
	CreateBook(book *model.Book) (*model.Book, error)
	UpdateBook(book *model.Book) (*model.Book, error)
	DeleteBook(ID uint) error
}

type service struct {
	repository Repository
}

// NewService is used to create a single instance of the service
func NewService(r Repository) Service {
	return &service{
		repository: r,
	}
}

// FetchBooks is a service layer that helps fetch all books in BookShop
func (s *service) FetchBooks() (*[]model.Book, error) {
	return s.repository.FetchAll()
}

func (s *service) FindBook(ID uint) (*model.Book, error) {
	return s.repository.Find(ID)
}

// InsertBook is a service layer that helps insert book in BookShop
func (s *service) CreateBook(book *model.Book) (*model.Book, error) {
	return s.repository.Create(book)
}

// UpdateBook is a service layer that helps update books in BookShop
func (s *service) UpdateBook(book *model.Book) (*model.Book, error) {
	return s.repository.Update(book)
}

// RemoveBook is a service layer that helps remove books from BookShop
func (s *service) DeleteBook(ID uint) error {
	return s.repository.Delete(ID)
}
