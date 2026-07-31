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

// Package gcs implements blobstore.Store on top of Google Cloud Storage.
package gcs

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"app/blobstore"
	"cloud.google.com/go/storage"
)

type store struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

// New returns a blobstore.New that opens the GCS bucket named bucketName.
func New(bucketName string) blobstore.New {
	return func(ctx context.Context) (blobstore.Store, error) {
		client, err := storage.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage client failed: %v", err)
		}
		return &store{client: client, bucket: client.Bucket(bucketName)}, nil
	}
}

func (s *store) Write(ctx context.Context, name string, data []byte) error {
	w := s.bucket.Object(name).NewWriter(ctx)

	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("storage copy failed: %v", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("storage close failed: %v", err)
	}

	return nil
}

func (s *store) Read(ctx context.Context, name string) ([]byte, error) {
	r, err := s.bucket.Object(name).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage reader failed for %s: %v", name, err)
	}
	defer r.Close()

	var b bytes.Buffer
	if _, err := b.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("storage reading failed for %s: %v", name, err)
	}

	return b.Bytes(), nil
}

func (s *store) Close() error {
	return s.client.Close()
}
