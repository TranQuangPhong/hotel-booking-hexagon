.PHONY: buf-gen-go
buf-gen-go:
	buf format -w
	buf lint
	buf generate
	go mod tidy
