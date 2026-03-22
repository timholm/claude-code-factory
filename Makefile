BINARY := factory
CMD     := ./cmd/factory

.PHONY: build test run-gather run-analyze run-build run-mirror clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./...

run-gather: build
	./$(BINARY) gather

run-analyze: build
	./$(BINARY) analyze

run-build: build
	./$(BINARY) build

run-mirror: build
	./$(BINARY) mirror

clean:
	rm -f $(BINARY)
