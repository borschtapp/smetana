package store

import (
	"context"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinIO *minio.Client

func Setup() (err error) {
	MinIO, err = minio.New(os.Getenv("S3_HOST"), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""),
		Secure: true,
	})
	return
}

func PutObject(bucket, objectName string, reader io.Reader, objectSize int64, contentType string) (minio.UploadInfo, error) {
	ctx := context.Background()
	return MinIO.PutObject(ctx, bucket, objectName, reader, objectSize, minio.PutObjectOptions{ContentType: contentType})
}

func DirectUrl(bucket string, path string) string {
	return MinIO.EndpointURL().String() + "/" + bucket + "/" + path
}
