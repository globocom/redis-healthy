eval "$(docker-machine env dev)" &&
  docker rmi $(docker images -q -f dangling=true) || true &&
  docker-compose stop test &&
  docker-compose rm --all -f &&
  docker-compose build test &&
  docker-compose run test
