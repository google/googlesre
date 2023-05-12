# SRE Classroom Image Server

This repository contains a simple Image Server web application. It is a sample
implementation of the [SRE Classroom: Distributed
ImageServer](https://sre.google/classroom/imageserver/) using GCP services. The
application allows users to upload photos in a few different formats, search
photos by tags and then download them. It is meant to be used as a reference
system for exploring the behavior of microservices running inside a [Kubernetes
cluster](https://cloud.google.com/kubernetes-engine).

## Disclaimer

This is not an officially supported Google product.

## Overview

The application consists of the following components:

* Load balancer (provided by GCP)
* UI frontend (Angular application)
* Upload frontend (Go application)
* Upload backend (Go application)
* Search frontend (Go application)
* Search backend (Go application)
* Download frontend (Go application)
* Download backend (Go application)
* Firestore database (provided by GCP)
* Cloud storage (provided by GCP)

User browser communicates with the frontend components by sending
[JSON](https://www.json.org/) requests.  Frontend components communicate with
the backends components using [Protocol
Buffers](https://developers.google.com/protocol-buffers) and
[gRPC](https://grpc.io/). Backend components use the
[Firestore](https://cloud.google.com/firestore) and [Cloud
storage](https://cloud.google.com/storage) for storing and retrieving image
data.

## Installation

New instance of an Image Server application can be created by following these
steps:

1. Open the [GCP dashboard](https://console.cloud.google.com/)
1. Create a new GCP project
1. Create a new [source
   repository](https://cloud.google.com/source-repositories) inside the GCP
   project
1. Import the Image Server code into the source repository
1. Start a [Cloud Shell](https://cloud.google.com/shell) for the GCP project
1. In the Cloud Shell clone the source repository
1. In the Cloud Shell run the `deploy.sh` script
1. Access the web application after the Ingress IP address becomes available

## Scenario 1

### Problem

Try logging into the application (no password is required) and uploading a new
image. It seems like there is no response from the application (upload is
failing). Can you find the source of the problem and fix it?

### Solution

Upload frontend is unable to contact the upload backend due to a
misconfiguration. Fix the Kubernetes configuration for the upload backend and
deploy the application again. Check if the problem is resolved.
