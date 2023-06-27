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

// Binary search-frontend serves tag search results to HTTP clients.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	pb "app/protos"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

var (
	backendAddress = flag.String("backend_address", "search-backend-service:8080", "backend server address")
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

// SearchRequestHandler returns keyword search results from the backend GRPC service.
func SearchRequestHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := grpc.Dial(*backendAddress, grpc.WithInsecure())
	if err != nil {
		glog.Errorf("grpc dial failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *backendTimeout)
	defer cancel()

	keyword := r.FormValue("keyword")
	request := &pb.GetThumbnailImagesRequest{SearchKeyword: keyword}

	client := pb.NewGetThumbnailClient(conn)
	response, err := client.GetThumbnail(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		glog.Errorf("search request failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	urls, err := json.Marshal(response.StorageUrl)
	if err != nil {
		glog.Errorf("json marshal failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(urls)
}

func main() {
	flag.Parse()

	http.HandleFunc("/", HealthRequestHandler)
	http.HandleFunc("/search", SearchRequestHandler)

	glog.Infof("starting HTTP server on %s", *listenPort)
	glog.Fatal(http.ListenAndServe(*listenPort, nil))
}
