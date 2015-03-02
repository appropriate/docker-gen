#!/bin/bash

set -e

: ${REPO:=md5/docker-gen}
: ${APP:=${REPO#*/}}

if [ $# -eq 0 ] ; then
	echo 2>&1 "Usage: ./update.sh <$REPO tag or branch>"
	exit 1
fi

VERSION="$1"

# cd to the current directory so the script can be run from anywhere.
cd `dirname "$0"`

# Update the certificates.
echo "Updating certificates..."
./certs/update.sh

echo "Fetching and building $APP $VERSION..."

# Create a temporary directory.
TEMP=`mktemp -d "/tmp/$APP.XXXXXX"`

git clone -b "$VERSION" https://github.com/$REPO.git $TEMP
docker build -t $APP-builder $TEMP

# Create a dummy container so we can run a cp against it.
ID=$(docker create $APP-builder)

# Update the local binary.
docker cp $ID:/go/bin/$APP .

# Cleanup.
docker rm -f $ID
docker rmi $APP-builder

echo "Done."
