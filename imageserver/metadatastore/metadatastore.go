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

// Package metadatastore defines a backend-agnostic interface for the photo
// index: named collections (user, hashtag, latest) of documents keyed by ID.
package metadatastore

import "context"

// Item is a single document returned by List, keyed by ID within a collection.
type Item struct {
	ID   string
	Data map[string]interface{}
}

// Store indexes documents grouped into named collections.
type Store interface {
	// Put creates or overwrites the document identified by collection/id.
	Put(ctx context.Context, collection, id string, data map[string]interface{}) error
	// List returns up to limit documents from collection.
	List(ctx context.Context, collection string, limit int) ([]Item, error)
	// Close releases resources held by the store.
	Close() error
}

// New opens a Store, e.g. by connecting to a backend service.
type New func(ctx context.Context) (Store, error)
