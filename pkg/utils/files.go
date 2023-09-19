package utils

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"strings"

	"borscht.app/smetana/pkg/store"
)

func detectContentTypeFromResponse(r *http.Response) string {
	contentType := detectContentTypeFromHeader(r.Header)
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

func detectContentTypeFromHeader(header http.Header) string {
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

func extensionByType(typ string) string {
	if typ == "image/jpeg" {
		return ".jpg"
	} else if typ == "image/png" {
		return ".png"
	}

	extensions, err := mime.ExtensionsByType(typ)
	if err != nil || len(extensions) == 0 {
		return ""
	}

	return extensions[0]
}

func DownloadAndPutObject(url string, bucket string, path string) (string, error) {
	var err error
	var resp *http.Response
	if resp, err = http.Get(url); err != nil { //nolint
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New("unable to download image")
	}

	contentType := detectContentTypeFromResponse(resp)
	extension := extensionByType(contentType)
	if info, err := store.PutObject(bucket, path+extension, resp.Body, resp.ContentLength, contentType); err != nil {
		return "", err
	} else {
		if info.Location != "" {
			return info.Location, nil
		}
		return store.DirectUrl(bucket, path+extension), nil
	}
}

type UploadedImage struct {
	Path   string
	Width  int
	Height int
}

func DownloadAndPutImage(imageUrl string, bucket string, path string) (*UploadedImage, error) {
	var err error
	var resp *http.Response

	client := &http.Client{}

	req, err := http.NewRequest("GET", imageUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36")
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("unable to download image")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("unable to read response body")
	}

	contentType := detectContentTypeFromHeader(resp.Header)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	img, err := decodeImage(contentType, bytes.NewReader(data))
	if err != nil || img == nil {
		return nil, fmt.Errorf("unable to decode %s", contentType)
	}

	if contentType != "image/jpeg" {
		// convert to jpeg
		buf := new(bytes.Buffer)
		if err := jpeg.Encode(buf, img, nil); err != nil {
			return nil, errors.New("unable to encode jpg")
		}
		data = buf.Bytes()
	}

	if info, err := store.PutObject(bucket, path, bytes.NewBuffer(data), int64(len(data)), "image/jpeg"); err != nil {
		return nil, err
	} else {
		if info.Location == "" {
			info.Location = store.DirectUrl(info.Bucket, info.Key)
		}

		return &UploadedImage{
			Path:   info.Location,
			Width:  img.Bounds().Dx(),
			Height: img.Bounds().Dy(),
		}, nil
	}
}

func decodeImage(contentType string, r io.Reader) (img image.Image, err error) {
	switch contentType {
	case "image/jpeg":
		img, err = jpeg.Decode(r)
	case "image/png":
		img, err = png.Decode(r)
	case "image/gif":
		img, err = gif.Decode(r)
	}

	if err != nil || img == nil {
		return nil, errors.New("decode error")
	}
	return img, nil
}
