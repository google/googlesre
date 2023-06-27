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

// Binary upload-frontend stores photos from HTTP clients.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	pb "app/protos"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var (
	backendAddress = flag.String("backend_address", "upload-backend-service:8080", "backend server address")
	backendTimeout = flag.Duration("backend_timeout", 10*time.Second, "backend request timeout")
	listenPort     = flag.String("listen_port", ":8080", "start server on this port")
)

// HealthRequestHandler returns response for the / URL as required for LB health checks.
func HealthRequestHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	fmt.Fprintf(w, "Alive")
}

// UploadRequestHandler sends the photo and tags to backend GRPC service.
func UploadRequestHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	if username == "" {
		glog.Errorf("getting username failed")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tags, err := getTags(r)
	if err != nil {
		glog.Errorf("getting tags failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	blob, filename, err := getPhoto(r)
	if err != nil {
		glog.Errorf("getting file failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := grpc.Dial(*backendAddress, grpc.WithInsecure())
	if err != nil {
		glog.Errorf("grpc dial failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	client := pb.NewUploadPhotoClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), *backendTimeout)
	defer cancel()

	objName := uuid.New().String() + path.Ext(filename)
	errors := make(chan error)

	go func() {
		uploadRequest := &pb.UploadPhotoRequest{ObjName: objName, Image: blob}
		_, err = client.Upload(ctx, uploadRequest, grpc.WaitForReady(true))
		if err != nil {
			err = fmt.Errorf("photo upload failed: %v", err)
		}
		errors <- err
	}()

	go func() {
		metadataRequest := &pb.CreateMetadataRequest{
			ObjName: objName, User: username, Hashtags: tags}
		_, err = client.CreateMetadata(ctx, metadataRequest, grpc.WaitForReady(true))
		if err != nil {
			err = fmt.Errorf("metadata create failed: %v", err)
		}
		errors <- err
	}()

	status := http.StatusOK
	for i := 0; i < 2; i++ {
		err = <-errors
		if err != nil {
			glog.Error(err)
			status = http.StatusInternalServerError
		}
	}
	w.WriteHeader(status)
}

// getPhoto function tries to read and decode photo data from the request.
func getPhoto(r *http.Request) ([]byte, string, error) {
	image, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file: %v", err)
	}
	defer image.Close()

	var b bytes.Buffer
	_, err = io.Copy(&b, image)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %v", err)
	}

	return b.Bytes(), header.Filename, nil
}

// getTags function tries to read and decode photo tags from the request.
func getTags(r *http.Request) ([]string, error) {
	var tags []string

	data := r.FormValue("hashtags")
	if len(data) == 0 {
		return tags, nil
	}

	err := json.Unmarshal([]byte(data), &tags)
	if err != nil {
		return nil, err
	}

	return tags, nil
}

func main() {
	flag.Parse()

	http.HandleFunc("/", HealthRequestHandler)
	http.HandleFunc("/upload", UploadRequestHandler)

	glog.Infof("starting HTTP server on %s", *listenPort)
	glog.Fatal(http.ListenAndServe(*listenPort, nil))
}
