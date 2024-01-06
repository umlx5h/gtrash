.PHONY: test itest

gtrash: main.go
	go build

clean:
	rm -f gtrash
	rm -rf coverage itest/coverage
	rm -f coverage.txt coverage.html

test-all: clean test itest report-coverage

test:
	mkdir -p coverage
	go test -cover -v ./internal/... -args -test.gocoverdir="$$PWD/coverage"

itest:
	mkdir -p itest/coverage
	go build -cover
	docker compose run itest

report-coverage:
	go tool covdata percent -i=./coverage,./itest/coverage
	go tool covdata textfmt -i=./coverage,./itest/coverage -o coverage.txt
	go tool cover -html=coverage.txt -o coverage.html
