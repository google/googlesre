// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package s3 implements blobstore.Store on top of any S3-compatible object
// storage service (AWS S3, MinIO, Ceph RGW, etc.) via the minio-go client.
package s3

import (
	"bytes"
	"context"
	"fmt"

	"app/blobstore"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type store struct {
	client *minio.Client
	bucket string
}

// New returns a blobstore.New that opens bucketName on the S3-compatible
// endpoint (host:port, no scheme), authenticating with the given static
// access/secret key pair. useSSL selects https vs http for the endpoint.
func New(endpoint, bucketName, accessKey, secretKey string, useSSL bool) blobstore.New {
	return func(ctx context.Context) (blobstore.Store, error) {
		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			return nil, fmt.Errorf("s3 client failed: %v", err)
		}
		return &store{client: client, bucket: bucketName}, nil
	}
}

func (s *store) Write(ctx context.Context, name string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, name, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("s3 put failed for %s: %v", name, err)
	}

	return nil
}

func (s *store) Read(ctx context.Context, name string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 get failed for %s: %v", name, err)
	}
	defer obj.Close()

	var b bytes.Buffer
	if _, err := b.ReadFrom(obj); err != nil {
		return nil, fmt.Errorf("s3 read failed for %s: %v", name, err)
	}

	return b.Bytes(), nil
}

// Close is a no-op: the minio client holds no state that needs releasing.
func (s *store) Close() error {
	return nil
}
