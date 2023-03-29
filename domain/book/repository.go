package book

import (
	"gorm.io/gorm"

	"borscht.app/smetana/model"
)

// Repository interface allows us to access the CRUD Operations in mongo here.
type Repository interface {
	Find(ID uint) (*model.Book, error)
	FetchAll() (*[]model.Book, error)
	Create(book *model.Book) (*model.Book, error)
	Update(book *model.Book) (*model.Book, error)
	Delete(ID uint) error
}

type repository struct {
	db *gorm.DB
}

// NewRepo is the single instance repo that is being created.
func NewRepo(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Find(ID uint) (*model.Book, error) {
	var book model.Book

	if result := r.db.Take(&book, ID); result.Error != nil {
		return nil, result.Error
	}

	return &book, nil
}

func (r *repository) FetchAll() (*[]model.Book, error) {
	var books []model.Book

	if result := r.db.Find(&books); result.Error != nil {
		return nil, result.Error
	}

	return &books, nil
}

func (r *repository) Create(book *model.Book) (*model.Book, error) {
	if result := r.db.Create(&book); result.Error != nil {
		return nil, result.Error
	}

	return book, nil
}

func (r *repository) Update(book *model.Book) (*model.Book, error) {
	if result := r.db.Save(&book); result.Error != nil {
		return nil, result.Error
	}

	return book, nil
}

func (r *repository) Delete(ID uint) error {
	if result := r.db.Delete(&model.Book{}, ID); result.Error != nil {
		return result.Error
	}

	return nil
}
