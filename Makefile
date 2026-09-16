# ViGov v2 — verification gate and knowledge generation
#
# `make check` is what stop_verify_guard looks for in the session transcript. The agent must
# not report "done" before it has run.

.PHONY: check brain hooks lint build test web kb proto tidy

check: brain hooks lint build test web   ## Full verification — run before saying it is done

brain:                          ## 7 structural invariants of the brain — anti-drift
	python tools/check_brain.py

hooks:                          ## Hook self-test: one block case + one pass case each
	python tools/test_hooks.py

lint:
	@# gofmt -l PRINTS unformatted files and still EXITS 0, so for as long as this target
	@# simply called it, unformatted code walked straight through the gate while the file
	@# names scrolled past. A gate that reports and passes is not a gate.
	@# gen/ is excluded because it is produced by `make proto`, not written here.
	@out=$$(gofmt -l . | grep -v '^gen/' || true); \
	if [ -n "$$out" ]; then \
		echo "gofmt: chưa định dạng, chạy gofmt -w:"; echo "$$out"; exit 1; fi
	go vet ./...
	-golangci-lint run ./...
	@# buf lint is NO LONGER prefixed with `-`. It was ignored while it was failing; it has
	@# been clean since the contract fixes, so ignoring it now only hides the next regression.
	buf lint

build:
	go build ./...

test:
	@# -race is not optional here. One process serves 200+ communes, so a data race is a race
	@# BETWEEN COMMUNES, and that is the class of defect that stays invisible until two
	@# communes are live. It needs a C toolchain; if this fails to start, fix the toolchain
	@# rather than dropping the flag.
	go test -race -count=1 ./...

web:                            ## Typecheck the Next.js apps — skips LOUDLY when deps are absent
	@# Deliberately not `next build`: src/app/ holds only .gitkeep today, so a build would fail
	@# for having no pages — a red gate that says nothing about code quality.
	@#
	@# And deliberately not a silent skip. A gate that quietly does nothing is the failure this
	@# project keeps finding elsewhere; this one says out loud that it checked nothing and why.
	@for app in apps/*/; do \
		if [ -f "$$app/package.json" ]; then \
			if [ -d "$$app/node_modules" ]; then \
				echo "typecheck $$app"; \
				(cd "$$app" && npx --no-install tsc --noEmit) || exit 1; \
			else \
				echo "BỎ QUA $$app — chưa có node_modules. Mã TypeScript KHÔNG được kiểm."; \
				echo "         chạy: (cd $$app && npm install)"; \
			fi; \
		fi; \
	done

kb:                             ## Regenerate the GENERATED tiers of kb/ from source
	go run ./tools/kb

proto:                          ## Regenerate Go code from .proto (requires buf)
	buf generate

tidy:
	go mod tidy
