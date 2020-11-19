#!/usr/bin/env bash

# Fail on error and print executions
set -ex

# Build the build base image (if it's not already)
docker build -t cray/reds-base --target base .

# Run the tests.
docker build -t cray/reds-unit-testing -f Dockerfile.testing --no-cache .
