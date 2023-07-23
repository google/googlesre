#!/bin/bash

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

CLUSTER_NAME=${CLUSTER_NAME:-cluster-1}
LOCATION=${LOCATION:-nam5}
REGION=${REGION:-us-central}
ZONE=${ZONE:-us-central1-c}
MACHINE_TYPE=${MACHINE_TYPE:-e2-standard-2}
NUM_NODES=${NUM_NODES:-5}

DOWNLOAD="download-backend download-frontend"
SEARCH="search-backend search-frontend"
UPLOAD="upload-backend upload-frontend"
UI="ui-frontend"
CONTAINERS="${CONTAINERS:-$DOWNLOAD $SEARCH $UPLOAD $UI}"

log() {
  printf "[$(date +'%Y-%m-%d %H:%M:%S%z')] \033[0;32m$*\033[0m\n"
}

err() {
  printf "[$(date +'%Y-%m-%d %H:%M:%S%z')] \033[0;31mERROR $*\033[0m\n" >&2
}

check_command() {
  local command="$1"

  if ! type "$command" &>/dev/null; then
    err "Command not found: $command"
    exit 1
  fi
}

check_config() {
  check_command gcloud

  cd $(dirname $0)

  if [[ -z "$PROJECT_ID" ]]; then
    PROJECT_ID="$(gcloud config get-value project 2>/dev/null)"
  fi
  if [[ -z "$PROJECT_ID" ]]; then
    err "Project name not found"
    exit 1
  fi

  PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" \
    | sed -ne '/^projectNumber:/s/[^0-9]//gp')
  log "Using project: $PROJECT_ID ($PROJECT_NUMBER)"
}

create_cluster() {
  enable_api container.googleapis.com

  if gcloud container clusters list | grep -q ${CLUSTER_NAME?}; then
    log "Using existing cluster: ${CLUSTER_NAME?}"
  else
    log "Creating new cluster: ${CLUSTER_NAME?}"
    gcloud container clusters create ${CLUSTER_NAME?} \
      --zone ${ZONE?} \
      --machine-type ${MACHINE_TYPE?} \
      --num-nodes ${NUM_NODES?} \
      --release-channel regular \
      --enable-ip-alias \
      --scopes "https://www.googleapis.com/auth/cloud-platform"
  fi

  if ! gcloud compute networks subnets list | grep -q proxy-only-subnet; then
    log "Creating proxy subnet: proxy-only-subnet"
    gcloud compute networks subnets create proxy-only-subnet \
      --purpose INTERNAL_HTTPS_LOAD_BALANCER \
      --role ACTIVE \
      --region ${ZONE%-*} \
      --network default \
      --range 10.0.0.0/23
  fi
}

enable_api() {
  local api=$1
  if [[ -z "$api" ]]; then
    err "Invalid API name"
    exit 1
  fi

  if ! gcloud services list | grep -qF "$api"; then
    log "Enabling API: $api"
    gcloud services enable "$api"
  fi
}

enable_iam() {
  local member=$1
  local role=$2

  if [[ -z "$member" ]]; then
    err "Invalid member name"
    exit 1
  fi

  if [[ -z "$role" ]]; then
    err "Invalid role name"
    exit 1
  fi

  if ! gcloud projects get-iam-policy "$PROJECT_ID" \
     --flatten="bindings[].members" \
     --format="table(bindings.members)" \
     --filter="bindings.role:$role" \
     | grep -qF "$member"
  then
    log "Adding $member to $role"
    gcloud projects add-iam-policy-binding "$PROJECT_ID" \
      --member="$member" --role="$role" --quiet
  fi
}

build_containers() {
  local n

  enable_api cloudbuild.googleapis.com
  enable_iam "serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com" \
    "roles/container.developer"

  if [[ -z "$TAG" ]]; then
    TAG="$(git rev-parse HEAD)"
  fi

  for container in $CONTAINERS; do
    subst="_CONTAINER_NAME=$container"
    subst="$subst,_TAG=${TAG?}"
    subst="$subst,_GKE_CLUSTER=${CLUSTER_NAME?}"
    subst="$subst,_GKE_LOCATION=${ZONE?}"

    log "Starting build for container: $container"
    gcloud builds submit \
      --substitutions "$subst" \
      --timeout 20m \
      --async &>/dev/null
  done

  while true; do
    n=$(gcloud builds list | grep -cE 'WORKING|QUEUED')
    if [[ "$n" -eq 0 ]]; then
      break
    elif [[ "$n" -eq 1 ]]; then
      log "Waiting for 1 build to finish..."
      sleep 10
    else
      log "Waiting for $n builds to finish..."
      sleep 30
    fi
  done
}

create_bucket() {
  check_command gsutil

  local bucket="gs://${PROJECT_ID}_photos"
  if gsutil ls | grep -qF "$bucket"; then
    log "Using existing bucket: $bucket"
  else
    log "Creating new bucket: $bucket"
    gsutil mb "$bucket"
  fi
}

create_firestore() {
  enable_api appengine.googleapis.com
  if ! gcloud app describe 2>/dev/null | grep -q name:; then
    log "Creating App Engine application"
    gcloud app create --region="${REGION}"
  fi

  if ! gcloud firestore databases list 2>/dev/null | grep -q name:; then
    log "Creating firestore database"
    gcloud firestore databases create --location="${LOCATION}"
  fi
  enable_api firestore.googleapis.com
}

main() {
  check_config
  create_bucket
  create_firestore
  create_cluster
  build_containers
}

main
