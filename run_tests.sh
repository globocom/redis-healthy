docker-compose stop test &&
docker-compose rm --all -f &&
docker-compose build test &&
docker-compose run --rm test
