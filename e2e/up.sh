#!/usr/bin/env bash
set -euo pipefail

# https://github.com/actions/runner-images/issues/2821
sudo systemctl stop mono-xsp4.service || true
sudo systemctl disable mono-xsp4.service || true
sudo killall -9 mono || true

unzip scenario1.zip >/dev/null
docker-compose up -d || true
docker ps || true
echo "Up done..."
