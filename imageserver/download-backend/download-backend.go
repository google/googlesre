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

// Binary download-backend serves photos from blob storage.
package main

import (
	"context"
	"flag"
	"net"

	"app/blobstore"
	"app/blobstore/gcs"
	pb "app/protos"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

var (
	listenPort    = flag.String("listen_port", ":8080", "start server on this port")
	storageBucket = flag.String("storage_bucket", "sre-classroom-image-server_photos", "storage bucket to use for storing photos")
	storageDryRun = flag.Bool("storage_dry_run", false, "disable storage bucket reads")
)

// DownloadPhotoServer implements the DownloadPhoto GRPC service.
type DownloadPhotoServer struct {
	newBlobs blobstore.New
}

// Download method returns the requested photo blob from storage.
func (s *DownloadPhotoServer) Download(ctx context.Context, request *pb.DownloadPhotoRequest) (*pb.DownloadPhotoResponse, error) {
	if *storageDryRun {
		return &pb.DownloadPhotoResponse{ImgBlob: []byte{}}, nil
	}

	blobs, err := s.newBlobs(ctx)
	if err != nil {
		glog.Errorf("blob store failed: %v", err)
		return nil, err
	}
	defer blobs.Close()

	img, err := blobs.Read(ctx, request.ImgName)
	if err != nil {
		glog.Errorf("blob read failed for %s: %v", request.ImgName, err)
		return nil, err
	}

	return &pb.DownloadPhotoResponse{ImgBlob: img}, nil
}

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", *listenPort)
	if err != nil {
		glog.Fatalf("failed to listen on %s: %v", *listenPort, err)
	}

	server := &DownloadPhotoServer{newBlobs: gcs.New(*storageBucket)}

	grpcServer := grpc.NewServer()
	pb.RegisterDownloadPhotoServer(grpcServer, server)

	glog.Infof("starting GRPC server on %s", *listenPort)
	if err := grpcServer.Serve(listener); err != nil {
		glog.Fatalf("failed to serve: %v", err)
	}
}
