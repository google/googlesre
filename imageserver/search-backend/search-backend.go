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
	"regexp"

	"app/metadatastore"
	"app/metadatastore/firestoredb"
	pb "app/protos"
	"github.com/golang/glog"
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

// hashtagPrefix namespaces hashtag-index collections so that /search cannot
// be used to enumerate unrelated top-level collections (per-user indexes,
// "latest", etc.). Must match the prefix used by upload-backend.
const hashtagPrefix = "tag:"

var keywordRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{0,64}$`)

// GetThumbnailServer implements the GetThumbnail GRPC service.
type GetThumbnailServer struct {
	newMeta metadatastore.New
}

// GetThumbnail method returns the thumbnails for requested keyword.
func (s *GetThumbnailServer) GetThumbnail(ctx context.Context, request *pb.GetThumbnailImagesRequest) (*pb.GetThumbnailImagesResponse, error) {
	var urls []string

	if !keywordRE.MatchString(request.SearchKeyword) {
		glog.Errorf("rejecting invalid search keyword: %q", request.SearchKeyword)
		return &pb.GetThumbnailImagesResponse{StorageUrl: urls}, nil
	}

	if *firestoreDryRun {
		return &pb.GetThumbnailImagesResponse{StorageUrl: urls}, nil
	}

	meta, err := s.newMeta(ctx)
	if err != nil {
		glog.Errorf("metadata store failed: %v", err)
		return nil, err
	}
	defer meta.Close()

	var collection string
	var f getFunc
	if request.SearchKeyword == "" {
		collection = *firestoreLatest
		f = getData
	} else {
		collection = hashtagPrefix + request.SearchKeyword
		f = getID
	}

	items, err := meta.List(ctx, collection, *thumbnailCount)
	if err != nil {
		glog.Errorf("metadata list failed: %v", err)
		return nil, err
	}

	for _, item := range items {
		urls = append(urls, *thumbnailPrefix+f(item))
	}

	return &pb.GetThumbnailImagesResponse{StorageUrl: urls}, nil
}

// getFunc type represents a function mapping from an item to a thumbnail name.
type getFunc func(item metadatastore.Item) string

// getData reads the thumbnail name from the item contents.
func getData(item metadatastore.Item) string {
	return item.Data["obj_name"].(string)
}

// getID reads the thumbnail name from the item identifier.
func getID(item metadatastore.Item) string {
	return item.ID
}

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", *listenPort)
	if err != nil {
		glog.Fatalf("failed to listen on %s: %v", *listenPort, err)
	}

	server := &GetThumbnailServer{newMeta: firestoredb.New(*firestoreProject)}

	grpcServer := grpc.NewServer()
	pb.RegisterGetThumbnailServer(grpcServer, server)

	glog.Infof("starting GRPC server on %s", *listenPort)
	if err := grpcServer.Serve(listener); err != nil {
		glog.Fatalf("failed to serve: %v", err)
	}
}
