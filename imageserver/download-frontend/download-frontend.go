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

// Binary download-frontend serves photos to HTTP clients.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"path"
	"time"

	pb "app/protos"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

var (
	backendAddress = flag.String("backend_address", "download-backend-service:8080", "backend server address")
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

// DownloadRequestHandler returns photo from the backend GRPC service.
func DownloadRequestHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := grpc.Dial(*backendAddress, grpc.WithInsecure())
	if err != nil {
		glog.Errorf("grpc dial failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *backendTimeout)
	defer cancel()

	objName := path.Base(r.URL.Path)
	request := &pb.DownloadPhotoRequest{ImgName: objName}

	client := pb.NewDownloadPhotoClient(conn)
	response, err := client.Download(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		glog.Errorf("downloading image failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Write(response.ImgBlob)
}

func main() {
	flag.Parse()

	http.HandleFunc("/", HealthRequestHandler)
	http.HandleFunc("/download/", DownloadRequestHandler)

	glog.Infof("starting HTTP server on %s", *listenPort)
	glog.Fatal(http.ListenAndServe(*listenPort, nil))
}
