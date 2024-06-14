#!/bin/sh

set -e

timeout 15 sh -c "until docker info &>/dev/null; do true; sleep 1; done"
kind create cluster --config /kind_config.yaml --verbosity 9
