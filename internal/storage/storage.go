package storage

import (
	"encoding/json"
	"io"
	"strings"

	"borscht.app/smetana/internal/utils"
)

// FileStorage is the interface for persisting and retrieving binary files.
type FileStorage interface {
	Save(path string, content io.Reader, size int64, contentType string) error
	Delete(path string) error
	GetBaseURL() string
}

var Default FileStorage

func SetDefault(s FileStorage) {
	Default = s
}

func AbsoluteUrl(path string) string {
	if strings.HasPrefix(path, "http") || Default == nil {
		return path
	}
	// Ensure we handle trailing/leading slashes correctly if needed
	// For now keeping it simple as per original logic
	return utils.EnsureSuffix(Default.GetBaseURL(), "/") + strings.TrimPrefix(path, "/")
}

func RelativeUrl(path string) string {
	if strings.HasPrefix(path, "http") && Default != nil {
		return strings.TrimPrefix(path, utils.EnsureSuffix(Default.GetBaseURL(), "/"))
	}
	return path
}

type Path string

func (p Path) MarshalJSON() ([]byte, error) {
	return json.Marshal(AbsoluteUrl(string(p)))
}

func (p *Path) UnmarshalJSON(b []byte) error {
	var path string
	err := json.Unmarshal(b, &path)
	if err == nil {
		*p = Path(RelativeUrl(path))
	}
	return err
}
