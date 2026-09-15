# ViGov v2 — verification gate and knowledge generation
#
# `make check` is what stop_verify_guard looks for in the session transcript. The agent must
# not report "done" before it has run.

.PHONY: check brain hooks lint build test kb proto tidy

check: brain hooks lint build test   ## Full verification — run before saying it is done

brain:                          ## 7 structural invariants of the brain — anti-drift
	python tools/check_brain.py

hooks:                          ## Hook self-test: one block case + one pass case each
	python tools/test_hooks.py

lint:
	gofmt -l .
	go vet ./...
	-golangci-lint run ./...
	-buf lint

build:
	go build ./...

test:
	go test ./...

kb:                             ## Regenerate the GENERATED tiers of kb/ from source
	go run ./tools/kb

proto:                          ## Regenerate Go code from .proto (requires buf)
	buf generate

tidy:
	go mod tidy
