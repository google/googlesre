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

// Package redisdb implements metadatastore.Store on top of Redis. Each
// collection is stored as a Redis hash: the hash key is the collection name,
// each hash field is a document ID, and each field value is the document's
// data, JSON-encoded.
package redisdb

import (
	"context"
	"encoding/json"
	"fmt"

	"app/metadatastore"
	"github.com/go-redis/redis/v8"
)

type store struct {
	client *redis.Client
}

// New returns a metadatastore.New that opens a Redis instance at addr
// (host:port).
func New(addr string) metadatastore.New {
	return func(ctx context.Context) (metadatastore.Store, error) {
		client := redis.NewClient(&redis.Options{Addr: addr})

		if err := client.Ping(ctx).Err(); err != nil {
			client.Close()
			return nil, fmt.Errorf("redis connect failed: %v", err)
		}

		return &store{client: client}, nil
	}
}

func (s *store) Put(ctx context.Context, collection, id string, data map[string]interface{}) error {
	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis marshal failed for %s/%s: %v", collection, id, err)
	}

	if err := s.client.HSet(ctx, collection, id, value).Err(); err != nil {
		return fmt.Errorf("redis hset failed for %s/%s: %v", collection, id, err)
	}

	return nil
}

func (s *store) List(ctx context.Context, collection string, limit int) ([]metadatastore.Item, error) {
	var items []metadatastore.Item
	var cursor uint64

	for len(items) < limit {
		var fields []string
		var err error
		fields, cursor, err = s.client.HScan(ctx, collection, cursor, "", int64(limit)).Result()
		if err != nil {
			return nil, fmt.Errorf("redis hscan failed for %s: %v", collection, err)
		}

		for i := 0; i+1 < len(fields) && len(items) < limit; i += 2 {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(fields[i+1]), &data); err != nil {
				return nil, fmt.Errorf("redis unmarshal failed for %s/%s: %v", collection, fields[i], err)
			}
			items = append(items, metadatastore.Item{ID: fields[i], Data: data})
		}

		if cursor == 0 {
			break
		}
	}

	return items, nil
}

func (s *store) Close() error {
	return s.client.Close()
}
