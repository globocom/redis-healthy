.DEFAULT_GOAL := build

pre_check:
	./check_dependencies.sh

test: pre_check
	./run_tests.sh

build: pre_check
	go build

path = "/go/src/github.com/globocom/redis-healthy"
build-docker:
	docker run --rm -it -v $(shell pwd):$(path) -w $(path) golang:1.7 go build

deploy: build-docker
	tsuru app-deploy Procfile redis-healthy -a redis-healthy
