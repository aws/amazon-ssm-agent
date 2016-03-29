#!/usr/bin/env bash

echo "Install misspell, goreportcard, gometalinter and godep"
go get github.com/client9/misspell/cmd/misspell
go get github.com/gojp/goreportcard
go get github.com/alecthomas/gometalinter
go get github.com/tools/godep

echo "Setup Path"
BIN_FOLDER=${BGO_SPACE}/vendor/bin
LOCAL_REPORT_CARD_SRC_OVERRIDE=${BGO_SPACE}/Tools/src/goreportcard/checks.go
SOURCE_FOLDER=${BGO_SPACE}/vendor/src/github.com/gojp/goreportcard
REPOS_PACKAGE=${SOURCE_FOLDER}/repos/src/github.com/aws/amazon-ssm-agent
PATH=${BIN_FOLDER}:${PATH}

echo "Replace gojp/goreportcard/handlers/checks.go with local copy"
cp ${LOCAL_REPORT_CARD_SRC_OVERRIDE} ${SOURCE_FOLDER}/handlers/checks.go

echo "Install gometalinter dependency"
${BIN_FOLDER}/gometalinter --install --update
${BIN_FOLDER}/godep save

echo "Copy amazon-ssm-agent package to the gocardreport repos directory"
rm -rf ${REPOS_PACKAGE}
mkdir -p ${REPOS_PACKAGE}
cp -R ${BGO_SPACE}/agent ${REPOS_PACKAGE}/agent/

echo "Start up goreportcard"
cd ${SOURCE_FOLDER}
${BIN_FOLDER}/godep go run main.go