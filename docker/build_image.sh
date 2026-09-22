#!/bin/bash

########################################################
# Script for building the docker image
#
# Usage:
#   ./build_image.sh <image tag>
#
# Description:
#   This script is used to build the docker image for fru-lab.
#   The image tag is the name of the image to be built, default is latest.
########################################################

LATEST_TAG="latest"
IMAGE_NAME="alonza0314/fru-lab"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# the Dockerfile COPYs web/backend and web/frontend, so the build context has
# to be the repo root (one level up from this script), not this docker/ dir.
REPO_ROOT="$(dirname "${SCRIPT_DIR}")"

build_docker_image() {
    if ! docker build -f ${SCRIPT_DIR}/Dockerfile -t $IMAGE_NAME:$image_tag ${REPO_ROOT}; then
        echo "Failed to build the docker image"
        return 1
    fi
}

main() {
    local image_tag=${1:-$LATEST_TAG}

    if ! build_docker_image $image_tag; then
        return 1
    fi
}

main "$@"
