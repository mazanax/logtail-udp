#!/usr/bin/env sh

cp ./version/version.go ./version/version.go.back
CURRENT_VERSION=$(git rev-parse --verify HEAD)
sed -i "s#<INJECT VERSION HERE>#$CURRENT_VERSION#g" ./version/version.go
CGO_ENABLED=0 go build -o logtail-proxy .
mv -f ./version/version.go.back ./version/version.go