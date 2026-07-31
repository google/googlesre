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

// Package firestoredb implements metadatastore.Store on top of Firestore.
package firestoredb

import (
	"context"
	"fmt"
	"path"

	"app/metadatastore"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type store struct {
	client *firestore.Client
}

// New returns a metadatastore.New that opens Firestore in the given project.
func New(projectID string) metadatastore.New {
	return func(ctx context.Context) (metadatastore.Store, error) {
		client, err := firestore.NewClient(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("firestore client failed: %v", err)
		}
		return &store{client: client}, nil
	}
}

func (s *store) Put(ctx context.Context, collection, id string, data map[string]interface{}) error {
	docPath := path.Join(collection, id)

	if _, err := s.client.Doc(docPath).Set(ctx, data); err != nil {
		return fmt.Errorf("firestore create failed for %s: %v", docPath, err)
	}

	return nil
}

func (s *store) List(ctx context.Context, collection string, limit int) ([]metadatastore.Item, error) {
	iter := s.client.Collection(collection).Documents(ctx)
	defer iter.Stop()

	var items []metadatastore.Item
	for i := 0; i < limit; i++ {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		items = append(items, metadatastore.Item{ID: doc.Ref.ID, Data: doc.Data()})
	}

	return items, nil
}

func (s *store) Close() error {
	return s.client.Close()
}
