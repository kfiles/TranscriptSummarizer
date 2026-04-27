.PHONY: test test-verbose test-coverage test-integration

test:
	go test ./pkg/...

test-verbose:
	go test -v ./pkg/...

test-coverage:
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

# Requires MONGODB_URI to be set and a reachable MongoDB instance.
test-integration:
	go test -tags integration ./pkg/...
