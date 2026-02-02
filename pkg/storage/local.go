package storage

import (
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	Root    string
	BaseURL string
}

func NewLocalStorage(root string, baseUrl string) *LocalStorage {
	if root == "" {
		root = "./uploads"
	}
	// Ensure directory exists
	_ = os.MkdirAll(root, os.ModePerm)

	return &LocalStorage{Root: root, BaseURL: baseUrl}
}

func (s *LocalStorage) GetBaseURL() string {
	return s.BaseURL
}

func (s *LocalStorage) Save(path string, content io.Reader, size int64, contentType string) error {
	fullPath := filepath.Join(s.Root, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, content)
	return err
}

func (s *LocalStorage) Delete(path string) error {
	fullPath := filepath.Join(s.Root, path)
	return os.Remove(fullPath)
}
