#!/usr/bin/env bash
set -euo pipefail

docker-compose down || true
docker ps
rm -rf docker-compose.yml volumes export.json || true
