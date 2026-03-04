package utils

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strings"
)

func DetectContentTypeFromResponse(r *http.Response) string {
	contentType := DetectContentTypeFromHeader(r.Header)
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}

	// if we cannot detect type from headers, try to detect from body content
	bodyBytes, _ := io.ReadAll(r.Body) // we need to read it fully, so we can restore it later
	_ = r.Body.Close()                 // must close manually

	detectedType := http.DetectContentType(bodyBytes)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // create new buffer with body
	return detectedType
}

func DetectContentTypeFromHeader(header http.Header) string {
	contentType := header.Get("Content-type")
	if contentType != "" {
		for _, v := range strings.Split(contentType, ",") {
			if t, _, err := mime.ParseMediaType(v); err == nil {
				return t
			}
		}
	}

	return ""
}

func ExtensionByType(typ string) string {
	extensions, err := mime.ExtensionsByType(typ)
	if err != nil || len(extensions) == 0 {
		return ""
	}

	return extensions[len(extensions)-1]
}
