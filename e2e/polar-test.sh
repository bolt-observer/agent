#!/usr/bin/env bash

export SERVER API_KEY || true
# Invoke scenarios
echo "Invoking scenarios..."
./scenario1.sh || exit 1

echo "Invoking scenarios... done"
