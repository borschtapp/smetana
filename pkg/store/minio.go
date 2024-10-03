package store

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinIO *minio.Client
var bucket string

func Setup() (err error) {
	if bucket = os.Getenv("S3_BUCKET"); bucket == "" {
		return errors.New("S3_BUCKET is not set")
	}

	MinIO, err = minio.New(os.Getenv("S3_HOST"), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""),
		Secure: true,
	})
	return
}

func PutObject(objectName string, reader io.Reader, objectSize int64, contentType string) (minio.UploadInfo, error) {
	ctx := context.Background()
	return MinIO.PutObject(ctx, bucket, objectName, reader, objectSize, minio.PutObjectOptions{ContentType: contentType})
}

func AbsoluteUrl(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return MinIO.EndpointURL().String() + "/" + bucket + "/" + path
}

func RelativeUrl(path string) string {
	if strings.HasPrefix(path, "http") {
		return strings.TrimPrefix(path, MinIO.EndpointURL().String()+"/"+bucket+"/")
	}
	return path
}

type StoragePath string

func (p StoragePath) MarshalJSON() ([]byte, error) {
	return json.Marshal(AbsoluteUrl(string(p)))
}

func (p *StoragePath) UnmarshalJSON(b []byte) error {
	var path string
	err := json.Unmarshal(b, &path)
	if err == nil {
		*p = StoragePath(RelativeUrl(path))
	}
	return err
}
