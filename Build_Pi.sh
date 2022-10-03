#!/bin/bash

mkdir -p out/keys
mkdir -p out/certs
cp keys/* out/keys/
cp certs/* out/certs/
cp config.json fixed_thumbnail.jp2 out/
# sudo apt install gcc-aarch64-linux-gnu #64-bit arm compiler
# sudo apt install gcc-arm-linux-gnueabihf #32-bit arm compiler
#CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc GOARM=7 GOOS=linux GOARCH=arm64  go build -o out/arm64//goopendrop .
#CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc GOARM=7 GOOS=linux GOARCH=arm  go build -o out/arm/goopendrop .

GOARM=7 GOOS=linux GOARCH=arm64  go build -o out/arm64//goopendrop .
GOARM=7 GOOS=linux GOARCH=arm  go build -o out/arm/goopendrop .