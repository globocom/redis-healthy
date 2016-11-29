.DEFAULT_GOAL := build

pre_check:
	./check_dependencies.sh

test: pre_check
	./run_tests.sh

build: pre_check
	go build
