# Build the Go CLI into bin/dockpipe.bin (bin/dockpipe launcher invokes it).
.PHONY: build test
build:
	go build -trimpath -ldflags "-s -w" -o bin/dockpipe.bin ./cmd/dockpipe
	@echo "Built bin/dockpipe.bin — run via bin/dockpipe"

test:
	go test ./...
