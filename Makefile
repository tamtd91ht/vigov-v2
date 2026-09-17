# ViGov v2 — verification gate and knowledge generation
#
# `make check` is what stop_verify_guard looks for in the session transcript. The agent must
# not report "done" before it has run.

.PHONY: check brain hooks buildfiles lint build test web kb proto tidy

check: brain hooks buildfiles lint build test web   ## Full verification — run before saying it is done

brain:                          ## 7 structural invariants of the brain — anti-drift
	python tools/check_brain.py

hooks:                          ## Hook self-test: one block case + one pass case each
	python tools/test_hooks.py

buildfiles:                     ## Dockerfile + Jenkinsfile của từng dịch vụ, và phần chung không ai đánh rơi
	@# Mỗi dịch vụ tự dựng và tự đóng gói: nó quyết build cái gì, khi nào, ra ảnh nào.
	@# Cái giá là chín bản sao sẽ trôi — và phần trôi trước tiên luôn là phần KHÔNG gây lỗi
	@# ngay: chạy bằng root, thiếu zoneinfo, hoặc đánh rơi `core/**` khỏi đường kích hoạt để
	@# rồi một bản vá trong mã dùng chung không kích hoạt dịch vụ nào cả.
	python tools/check_build.py

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

web:                            ## Typecheck + test the Next.js apps — skips LOUDLY when deps are absent
	@# Deliberately not `next build`: a production build is the CI image's job (build/web.Dockerfile),
	@# and doing it here would make the local gate several times slower for no extra signal.
	@#
	@# And deliberately not a silent skip. A gate that quietly does nothing is the failure this
	@# project keeps finding elsewhere; this one says out loud that it checked nothing and why.
	@#
	@# check:api is part of THIS target on purpose. It re-derives src/lib/api/schema.gen.ts from
	@# kb/20-contracts/openapi.json and fails when they differ — the one check that catches a
	@# REST contract change the web has not been regenerated for. Listing it only in Jenkinsfile
	@# would give the project two definitions of "verified", and the looser one always wins.
	@for app in */; do \
		if [ -f "$$app/package.json" ]; then \
			if [ -d "$$app/node_modules" ]; then \
				echo "typecheck $$app"; \
				(cd "$$app" && npx --no-install tsc --noEmit) || exit 1; \
				echo "test $$app"; \
				(cd "$$app" && npm test --silent) || exit 1; \
				echo "check:api $$app"; \
				(cd "$$app" && npm run --silent check:api) || exit 1; \
			else \
				echo "BỎ QUA $$app — chưa có node_modules. Mã TypeScript KHÔNG được kiểm."; \
				echo "         chạy: (cd $$app && npm install)"; \
			fi; \
		fi; \
	done

kb:                             ## Regenerate the GENERATED tiers of kb/ from source
	go run ./tools/kb
	@# ONE command regenerates everything derived from the code. Two targets would mean two
	@# things to remember, and the one nobody remembers is the one that goes stale — which for
	@# kb/20-contracts/openapi.json means the web builds a screen against a shape the server
	@# stopped sending.
	go run ./tools/apidoc

proto:                          ## Regenerate Go code from .proto (requires buf)
	buf generate

tidy:
	go mod tidy
