#!/bin/bash
#
# version 0.1
#

retries=${3:-20}
protocol=${4:-"tcp"}

if [ -z "$1" ]; then
  echo "host not specified"
  exit 1
fi

if [ -z "$2" ]; then
  echo "port not specified"
  exit 1
fi

try=0

while ! echo 1 2>/dev/null > /dev/$protocol/$1/$2;
do
  if [ $try -ge $retries ]; then
    echo "max retries (${retries}) exceeded, aborting"
    break
  fi
  echo "waiting for ${1}:${2} to be ready..."; sleep 3;
  ((try++))
done
