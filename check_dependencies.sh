#!/bin/bash
set -e

if ! type "docker-compose" > /dev/null; then
  echo '================================================================='
  echo 'you need to have docker-compose installed'
  echo '================================================================='
  exit 1
fi
