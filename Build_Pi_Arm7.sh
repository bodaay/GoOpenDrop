#!/bin/bash

mkdir -p out/arm64/keys
mkdir -p out/arm/keys
mkdir -p out/arm64/certs
mkdir -p out/arm/certs
cp keys/* out/arm/keys/
cp keys/* out/arm64/keys/
cp certs/* out/arm64/certs/
cp certs/* out/arm/certs/
cp config.json fixed_thumbnail.jp2 out/arm64/
cp config.json fixed_thumbnail.jp2 out/arm/


GOARM=7 GOOS=linux GOARCH=arm64  go build -o out/arm64//goopendrop .
GOARM=7 GOOS=linux GOARCH=arm  go build -o out/arm/goopendrop .
