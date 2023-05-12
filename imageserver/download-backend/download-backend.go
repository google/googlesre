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
	"bytes"
	"context"
	"flag"
	"net"

	pb "app/protos"
	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

var (
	listenPort    = flag.String("listen_port", ":8080", "start server on this port")
	storageBucket = flag.String("storage_bucket", "sre-classroom-image-server_photos", "storage bucket to use for storing photos")
	storageDryRun = flag.Bool("storage_dry_run", false, "disable storage bucket reads")
)

// DownloadPhotoServer implements the DownloadPhoto GRPC service.
type DownloadPhotoServer struct{}

// Download method returns the requested photo blob from storage.
func (s *DownloadPhotoServer) Download(ctx context.Context, request *pb.DownloadPhotoRequest) (*pb.DownloadPhotoResponse, error) {
	if *storageDryRun {
		return &pb.DownloadPhotoResponse{ImgBlob: []byte{}}, nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		glog.Errorf("storage client failed: %v", err)
		return nil, err
	}

	bucket := client.Bucket(*storageBucket)
	blob := bucket.Object(request.ImgName)

	r, err := blob.NewReader(ctx)
	if err != nil {
		glog.Errorf("storage reader failed for %s: %v", request.ImgName, err)
		return nil, err
	}
	defer r.Close()

	var b bytes.Buffer
	_, err = b.ReadFrom(r)
	if err != nil {
		glog.Errorf("storage reading failed for %s: %v", request.ImgName, err)
		return nil, err
	}

	return &pb.DownloadPhotoResponse{ImgBlob: b.Bytes()}, nil
}

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", *listenPort)
	if err != nil {
		glog.Fatalf("failed to listen on %s: %v", *listenPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDownloadPhotoServer(grpcServer, &DownloadPhotoServer{})

	glog.Infof("starting GRPC server on %s", *listenPort)
	if err := grpcServer.Serve(listener); err != nil {
		glog.Fatalf("failed to serve: %v", err)
	}
}
