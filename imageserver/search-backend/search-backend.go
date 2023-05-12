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

// Binary search-backend returns thumbnail names from index.
package main

import (
	"context"
	"flag"
	"net"

	pb "app/protos"
	"cloud.google.com/go/firestore"
	"github.com/golang/glog"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
)

var (
	firestoreLatest  = flag.String("firestore_latest", "latest", "firestore collection storing latest photos")
	firestoreProject = flag.String("firestore_project", "sre-classroom-image-server", "firestore project to use for storing tags")
	firestoreDryRun  = flag.Bool("firestore_dry_run", false, "disable firestore reads")
	listenPort       = flag.String("listen_port", ":8080", "start server on this port")
	thumbnailCount   = flag.Int("thumbnail_count", 10, "number of thumbnails to return")
	thumbnailPrefix  = flag.String("thumbnail_prefix", "/download/thumbnail_", "URL prefix for returned thumbnails")
)

// GetThumbnailServer implements the GetThumbnail GRPC service.
type GetThumbnailServer struct{}

// GetThumbnail method returns the thumbnails for requested keyword.
func (s *GetThumbnailServer) GetThumbnail(ctx context.Context, request *pb.GetThumbnailImagesRequest) (*pb.GetThumbnailImagesResponse, error) {
	var urls []string

	if *firestoreDryRun {
		return &pb.GetThumbnailImagesResponse{StorageUrl: urls}, nil
	}

	client, err := firestore.NewClient(ctx, *firestoreProject)
	if err != nil {
		glog.Errorf("firestore client failed: %v", err)
		return nil, err
	}
	defer client.Close()

	var f getFunc
	var q *firestore.CollectionRef
	if request.SearchKeyword == "" {
		q = client.Collection(*firestoreLatest)
		f = getData
	} else {
		q = client.Collection(request.SearchKeyword)
		f = getID
	}

	if q != nil {
		iter := q.Documents(ctx)
		defer iter.Stop()

		urls, err = getThumbnails(iter, f)
		if err != nil {
			glog.Errorf("firestore iterator failed: %v", err)
			return nil, err
		}
	}

	return &pb.GetThumbnailImagesResponse{StorageUrl: urls}, nil
}

// getThumbnails function returns a list of thumbnail urls from query iterator.
func getThumbnails(iter *firestore.DocumentIterator, f getFunc) ([]string, error) {
	var urls []string

	for i := 0; i < *thumbnailCount; i++ {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		urls = append(urls, *thumbnailPrefix+f(doc))
	}

	return urls, nil
}

// getFunc type represents a function mapping from a document to a thumbnail name.
type getFunc func(doc *firestore.DocumentSnapshot) string

// getData reads the thumbnail name from document contents.
func getData(doc *firestore.DocumentSnapshot) string {
	data := doc.Data()
	return data["obj_name"].(string)
}

// getID reads the thumbnail name from document identifier.
func getID(doc *firestore.DocumentSnapshot) string {
	return doc.Ref.ID
}

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", *listenPort)
	if err != nil {
		glog.Fatalf("failed to listen on %s: %v", *listenPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterGetThumbnailServer(grpcServer, &GetThumbnailServer{})

	glog.Infof("starting GRPC server on %s", *listenPort)
	if err := grpcServer.Serve(listener); err != nil {
		glog.Fatalf("failed to serve: %v", err)
	}
}
