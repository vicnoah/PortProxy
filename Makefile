# 常用命令，git hooks（lefthook.yml）也用它们
LINT := go tool -modfile=golangci-lint.mod golangci-lint

.PHONY: dev build test lint format format-check hooks

dev:
	go tool mygo dev

build:
	go tool mygo build

test:
	go test ./...

lint:
	$(LINT) run
	go tool mygo vet

format:
	$(LINT) fmt

format-check:
	$(LINT) fmt --diff

# clone 之后执行一次
hooks:
	go tool lefthook install
