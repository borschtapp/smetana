package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3/log"
)

type LocalStorage struct {
	Root    string
	BaseURL string
}

func NewLocalStorage(root string, baseUrl string) *LocalStorage {
	if root == "" {
		root = "./uploads"
	}
	if err := os.MkdirAll(root, 0750); err != nil {
		log.Fatalw("failed to create upload directory", "path", root, "error", err)
	}

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
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Warnw("failed to close file", "path", fullPath, "error", err)
		}
	}(file)

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
