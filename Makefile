.PHONY: test

test:
	go test -race -cover -timeout 60s ./...