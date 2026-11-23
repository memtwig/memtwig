.PHONY: build test clean

BINARY=memtwig

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) ./cmd/memtwig

test:
	CGO_ENABLED=0 go test -v ./...

clean:
	rm -rf bin/
