#!/bin/bash
#
# This script builds our Go Lambda functions. It also runs tidy's modules
# and runs the tests (unless NOGOTESTS is set).

set -e

STAGE=$1
FUNCTIONS=(
  app
  "cognitotriggers/custommessage"
  "cognitotriggers/postconfirmation"
)
ROOT_DIR="`pwd`"
BIN_DIR="${ROOT_DIR}/bin/"
ZIP_DIR="${ROOT_DIR}/dist/"

echo "Building functions for ${STAGE}..."

mkdir -p ${BIN_DIR} ${ZIP_DIR}

echo "Generating Templ templates..."
cd app; go tool templ generate; cd ..

for func in "${FUNCTIONS[@]}"
do
  pushd $func > /dev/null

  if [ -z "${NOGOTESTS}" ]; then
    echo "Go mod tidy ${func}..."
    go mod tidy

    if [ "${STAGE}" != "local" ];
    then
      echo "Testing ${func}..."
      go test
    fi
  fi

  echo "Compiling ${func} Go code"
  OUTPUT_DIR="${BIN_DIR}/${func}"
  mkdir -p $OUTPUT_DIR
  GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags="-s -w -X main.LambdaStage=${STAGE}" -o "${OUTPUT_DIR}/bootstrap"
  popd > /dev/null

  zipfile=${ZIP_DIR}${func/\//}.zip
  rm -f ${zipfile}
  zip -j ${zipfile} ${OUTPUT_DIR}/bootstrap
  echo "Created artifact: ${zipfile}"
done

echo "Build complete"
