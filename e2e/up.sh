#!/usr/bin/env bash

unzip scenario0.zip
docker-compose -d up
docker ps
