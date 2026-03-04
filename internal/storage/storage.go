package storage

import (
	"encoding/json"
	"io"
	"strings"
)

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
	return Default.GetBaseURL() + "/" + path
}

func RelativeUrl(path string) string {
	if strings.HasPrefix(path, "http") && Default != nil {
		return strings.TrimPrefix(path, Default.GetBaseURL()+"/")
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
