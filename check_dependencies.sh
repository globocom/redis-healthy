#!/bin/bash
set -e

if ! type "docker-machine" > /dev/null; then
  echo '================================================================='
  echo 'you need to have docker-machine installed'
  echo '================================================================='
  exit 1
fi

if ! docker-machine ls|grep dev ; then
    echo '================================================================='
    echo 'you need to have a docker-machine named dev'
    echo '$ docker-machine create --driver virtualbox dev'
    echo '================================================================='
    exit 1
fi

if ! docker-machine status dev|grep unning ; then
  echo '================================================================='
  echo 'the docker-machine named dev needs to be running'
  echo '$ docker-machine start dev'
  echo '================================================================='
  exit 1
fi
