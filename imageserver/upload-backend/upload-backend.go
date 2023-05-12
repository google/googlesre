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

// Binary upload-backend saves photos and tags to backend storage.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"net"
	"path"
	"strconv"
	"sync/atomic"
	"time"

	pb "app/protos"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"github.com/nfnt/resize"
	"google.golang.org/grpc"
)

var (
	firestoreLatestPath   = flag.String("firestore_latest_path", "latest", "path for storing latest images")
	firestoreLatestPhotos = flag.Uint("firestore_latest_photos", 10, "number of latest images to store")
	firestoreProject      = flag.String("firestore_project", "sre-classroom-image-server", "firestore project to use for storing tags")
	firestoreDryRun       = flag.Bool("firestore_dry_run", false, "disable firestore writes")
	listenPort            = flag.String("listen_port", ":8080", "start server on this port")
	storageBucket         = flag.String("storage_bucket", "sre-classroom-image-server_photos", "storage bucket to use for storing photos")
	storageDryRun         = flag.Bool("storage_dry_run", false, "disable storage bucket writes")
	thumbnailHeight       = flag.Uint("thumbnail_height", 180, "height of the generated photo thumbnail")
	thumbnailPrefix       = flag.String("thumbnail_prefix", "thumbnail_", "name prefix to use for storing thumbnails")
	thumbnailWidth        = flag.Uint("thumbnail_width", 320, "width of the generated photo thumbnail")
	latestIdxFirestore    = rand.Uint32()
)

// UploadPhotoServer implements the UploadPhoto GRPC service.
type UploadPhotoServer struct{}

// Upload method stores the photo and generated thumbnail to blob storage.
func (s *UploadPhotoServer) Upload(ctx context.Context, request *pb.UploadPhotoRequest) (*pb.UploadPhotoResponse, error) {
	thumb, err := makeThumbnail(request.Image, *thumbnailWidth, *thumbnailHeight)
	if err != nil {
		glog.Errorf("photo resize failed: %v", err)
		return nil, err
	}

	if *storageDryRun {
		return &pb.UploadPhotoResponse{}, nil
	}

	start := time.Now()
	client, err := storage.NewClient(ctx)
	if err != nil {
		glog.Errorf("storage client failed: %v", err)
		return nil, err
	}
	defer client.Close()
	delta := time.Since(start)
	if delta > 150*time.Millisecond {
		glog.Error("storage client took: ", delta)
	}

	bucket := client.Bucket(*storageBucket)
	errors := make(chan error)

	go func() {
		start := time.Now()
		err = writeBlob(ctx, bucket, request.ObjName, request.Image)
		if err != nil {
			err = fmt.Errorf("photo write failed: %v", err)
		}
		delta := time.Since(start)
		if delta > 150*time.Millisecond {
			glog.Error("photo write took: ", delta)
		}
		errors <- err
	}()

	go func() {
		start := time.Now()
		err = writeBlob(ctx, bucket, *thumbnailPrefix+request.ObjName, thumb)
		if err != nil {
			err = fmt.Errorf("thumbnail write failed: %v", err)
		}
		delta := time.Since(start)
		if delta > 150*time.Millisecond {
			glog.Error("thumbnail write took: ", delta)
		}
		errors <- err
	}()

	var failure error
	for i := 0; i < 2; i++ {
		err = <-errors
		if err != nil {
			glog.Error(err)
			if failure == nil {
				failure = err
			}
		}
	}

	if failure != nil {
		return nil, failure
	} else {
		return &pb.UploadPhotoResponse{}, nil
	}
}

func writeBlob(ctx context.Context, bucket *storage.BucketHandle, name string, data []byte) error {
	obj := bucket.Object(name)
	w := obj.NewWriter(ctx)
	r := bytes.NewReader(data)

	_, err := io.Copy(w, r)
	if err != nil {
		return fmt.Errorf("storage copy failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("storage close failed: %v", err)
	}

	return nil
}

func makeThumbnail(photo []byte, width, height uint) ([]byte, error) {
	r := bytes.NewReader(photo)
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("photo decode failed: %v", err)
	}

	thumb := resize.Thumbnail(width, height, img, resize.NearestNeighbor)

	var b bytes.Buffer
	if format == "jpeg" {
		err = jpeg.Encode(&b, thumb, nil)
	} else if format == "png" {
		err = png.Encode(&b, thumb)
	} else if format == "gif" {
		err = gif.Encode(&b, thumb, nil)
	} else {
		err = fmt.Errorf("unsuported image format: %s", format)
	}
	if err != nil {
		return nil, fmt.Errorf("thumbnail encode failed: %v", err)
	}

	return b.Bytes(), nil
}

// CreateMetadata method stores image user and tags into Firestore.
func (s *UploadPhotoServer) CreateMetadata(ctx context.Context, request *pb.CreateMetadataRequest) (*pb.CreateMetadataResponse, error) {
	if *firestoreDryRun {
		return &pb.CreateMetadataResponse{}, nil
	}

	start := time.Now()
	client, err := firestore.NewClient(ctx, *firestoreProject)
	if err != nil {
		glog.Errorf("firestore client failed: %v", err)
		return nil, err
	}
	defer client.Close()
	delta := time.Since(start)
	if delta > 150*time.Millisecond {
		glog.Error("firestore client took: ", delta)
	}

	timestamp := time.Now().Unix()
	errors := make(chan error)

	// Store photo name under user.
	go func() {
		start := time.Now()
		id := path.Join(request.User, request.ObjName)
		_, err := client.Doc(id).Set(ctx, map[string]interface{}{
			"uploaded_time": timestamp,
		})
		if err != nil {
			err = fmt.Errorf("firestore create failed for %s: %v", id, err)
		}
		delta := time.Since(start)
		if delta > 150*time.Millisecond {
			glog.Error("user write took: ", delta)
		}
		errors <- err
	}()

	// Store photo name under tags.
	go func() {
		var err error
		start := time.Now()
		for _, tag := range request.Hashtags {
			id := path.Join(tag, request.ObjName)
			_, err = client.Doc(id).Set(ctx, map[string]interface{}{
				"uploaded_time": timestamp,
				"user":          request.User,
			})
			if err != nil {
				err = fmt.Errorf("firestore create failed for %s: %v", id, err)
				break
			}
		}
		delta := time.Since(start)
		if delta > 150*time.Millisecond {
			glog.Error("tag write took: ", delta)
		}
		errors <- err
	}()

	// Store photo name under latest.
	go func() {
		index := atomic.AddUint32(&latestIdxFirestore, 1) % uint32(*firestoreLatestPhotos)
		id := path.Join(*firestoreLatestPath, strconv.Itoa(int(index)))
		start := time.Now()
		_, err := client.Doc(id).Set(ctx, map[string]interface{}{
			"obj_name": request.ObjName,
		})
		if err != nil {
			err = fmt.Errorf("firestore create failed for %s: %v", id, err)
		}
		delta := time.Since(start)
		if delta > 150*time.Millisecond {
			glog.Error("latest write took: ", delta)
		}
		errors <- err
	}()

	var failure error
	for i := 0; i < 3; i++ {
		err = <-errors
		if err != nil {
			glog.Error(err)
			if failure == nil {
				failure = err
			}
		}
	}

	if failure != nil {
		return nil, failure
	} else {
		return &pb.CreateMetadataResponse{}, nil
	}
}

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", *listenPort)
	if err != nil {
		glog.Fatalf("failed to listen on %s: %v", *listenPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterUploadPhotoServer(grpcServer, &UploadPhotoServer{})

	glog.Infof("starting GRPC server on %s", *listenPort)
	if err := grpcServer.Serve(listener); err != nil {
		glog.Fatalf("failed to serve: %v", err)
	}
}
