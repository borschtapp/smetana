package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	_ = os.MkdirAll(root, 0750)

	return &LocalStorage{Root: root, BaseURL: baseUrl}
}

func (s *LocalStorage) GetBaseURL() string {
	return s.BaseURL
}

func (s *LocalStorage) Save(path string, content io.Reader, size int64, contentType string) error {
	// Clean and validate path to prevent directory traversal
	fullPath := filepath.Join(s.Root, filepath.Clean(path))
	relPath, err := filepath.Rel(s.Root, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return errors.New("invalid path: directory traversal attempt")
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	file, err := os.Create(fullPath) // #nosec G304 - path is validated above
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, content)
	return err
}

func (s *LocalStorage) Delete(path string) error {
	// Clean and validate path to prevent directory traversal
	fullPath := filepath.Join(s.Root, filepath.Clean(path))
	relPath, err := filepath.Rel(s.Root, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return errors.New("invalid path: directory traversal attempt")
	}

	return os.Remove(fullPath)
}
