.PHONY: build test web

build: web
	go build -o inftask ./cmd/inftask

test: web
	node scripts/test-delegation-hook.mjs
	npm --prefix web run lint
	npm --prefix web run test
	npm --prefix desktop test
	go test ./...

web:
	npm --prefix web run build

