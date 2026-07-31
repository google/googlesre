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

// Package blobstore defines a backend-agnostic interface for storing and
// retrieving photo and thumbnail bytes by name.
package blobstore

import "context"

// Store reads and writes blobs identified by name.
type Store interface {
	// Write stores data under name, overwriting any existing blob.
	Write(ctx context.Context, name string, data []byte) error
	// Read returns the blob stored under name.
	Read(ctx context.Context, name string) ([]byte, error)
	// Close releases resources held by the store.
	Close() error
}

// New opens a Store, e.g. by connecting to a backend service.
type New func(ctx context.Context) (Store, error)
