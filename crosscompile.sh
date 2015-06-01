#!/bin/bash

OSARCH="linux/amd64 darwin/amd64 windows/amd64"
OUTDIR="bin"
OUTPUT="$OUTDIR/{{.Dir}}_{{.OS}}_{{.Arch}}"

function run_in_docker {
  docker run --rm -v "$(pwd)":/usr/src/chisel -w /usr/src/chisel \
      tcnksm/gox:1.4.2 \
      /bin/bash -c \
      "go get -d ./... && gox -osarch=\"$OSARCH\" -output=\"$OUTPUT\""
}

function run_local {
  if ! which gox > /dev/null; then
    go get "github.com/mitchell/gox"
    gox -build-toolchain
  fi
  gox -osarch="$OSARCH" -output="$OUTPUT"
}

echo "compiling chisel for platform $OSARCH into directory $OUTDIR"
#run_in_docker
run_local
