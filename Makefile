pre_check:
	./check_dependencies.sh

test: pre_check
	./run_tests.sh
