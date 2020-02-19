#!/bin/sh
set GOOS=linux
go build -o main main.go
~/Go/bin/build-lambda-zip.exe -output main.zip main