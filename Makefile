# ViGov v2 — verification gate and knowledge generation
#
# `make check` is what stop_verify_guard looks for in the session transcript. The agent must
# not report "done" before it has run.

.PHONY: check brain hooks quyen vet-actor khoaduynhat envmap buildfiles lint build standalone test web kb proto tidy

# Danh sách module, HỎI CHÍNH GO — không gõ tay, và không bóc tách văn bản go.work.
#
# Từ khi mỗi đơn vị triển khai là một module riêng, `go build ./...` ở gốc kho KHÔNG còn bao
# được cả kho: gốc không thuộc module nào. Go từ chối TO tiếng ("directory prefix . does not
# contain modules listed in go.work"), nên chuyện đó không âm thầm. Nhưng nếu ai "sửa" bằng
# cách liệt kê tay vài module thì module thứ mười một sẽ không được kiểm — và CHUYỆN ĐÓ mới
# âm thầm. `go list -m` luôn trả đúng những gì go.work đang nạp.
#
# `tr '\134' '/'` ĐỔI DẤU GẠCH NGƯỢC THÀNH GẠCH XUÔI, và nó không thừa. Trên Windows `go list`
# trả `D:\works\...`; make chuyển chuỗi ấy cho `sh`, và `sh` nuốt mọi dấu `\` như ký tự thoát —
# `cd "D:\works\..."` thành `cd "D:worksvihat..."`. Mục `standalone` đổ với một thông báo trỏ
# vào một đường dẫn không ai gõ bao giờ, còn `go vet`/`go build` thì nhận đường dẫn hỏng.
# Trên Linux chuỗi không có dấu gạch ngược nào nên đây là phép biến đổi rỗng.
MOD_DIRS := $(shell go list -m -f '{{.Dir}}' | tr '\134' '/')
MODULES  := $(addsuffix /...,$(MOD_DIRS))

# Trình thông dịch cho mọi `tools/*.py` — CẦN PYTHON 3.8+ (`tools/check_env_map.py` dùng `:=`).
#
# Mặc định `python` vì trên Windows `python3` thường là lối tắt Microsoft Store, không chạy
# được. Trên máy build Linux thì ngược lại: CentOS/RHEL 7 để `python` là Python 2.7, và ngày
# 24/09/2026 cổng kiểm đổ ở mục đầu tiên với "SyntaxError: Non-ASCII character" — trông như
# tệp hỏng mã hoá chứ không như gọi nhầm trình thông dịch. Shebang `python3` trong tệp không
# cứu được: gọi `python <tệp>` là bỏ qua shebang. Jenkinsfile truyền `PYTHON=python3`.
PYTHON ?= python


check: brain hooks quyen vet-actor khoaduynhat envmap buildfiles lint build standalone test web   ## Full verification — run before saying it is done

brain:                          ## 7 structural invariants of the brain — anti-drift
	$(PYTHON) tools/check_brain.py

hooks:                          ## Hook self-test: one block case + one pass case each
	$(PYTHON) tools/test_hooks.py

quyen:                          ## Mọi khoá quyền trong mã Go có thật trong bảng `quyen` chưa
	@# LỚP LỖI IM LẶNG NHẤT CỦA LUẬT 5. Một chuỗi trao cho `authz.RequirePermission` mà bảng
	@# `quyen` không có là một quyền KHÔNG QUẢN TRỊ VIÊN NÀO CẤP ĐƯỢC: màn Phân quyền không có
	@# ô ấy, nên tuyến trả 403 với mọi tài khoản, mãi mãi — trong khi bộ đồ thử giả cấp bất kỳ
	@# chuỗi nào nên phép kiểm vẫn xanh. Ngày 21/09/2026 kho này có BA chuỗi như thế cùng lúc.
	@#
	@# `.claude/hooks/quyen_key_guard.py` chặn cùng lớp lỗi lúc GHI, nhưng nó chỉ thấy MỘT tệp
	@# và không bao giờ đọc lại thứ đã nằm sẵn trên đĩa — đúng chỗ ba khoá ấy đã sống. Chỉ lần
	@# quét toàn kho này trả lời được câu "hôm nay cả kho còn khoá bịa nào không".
	$(PYTHON) tools/check_quyen.py

vet-actor:                      ## Chủ thể mọi dòng vết là MÃ CÁN BỘ, không phải id nội bộ
	@# LỚP LỖI CÙNG HÌNH DẠNG VỚI `quyen` Ở TRÊN, khác cột. `audit_log.actor_id` trả lời câu
	@# "AI làm việc này" trên sổ có giá trị pháp lý. Chính sách đã viết ở dang_nhap.go:151 từ
	@# lâu — trong một CHÚ THÍCH, nên không gì đọc được nó, và ngày 22/09/2026 sáu chỗ ghi vết
	@# mới nạp ULID nội bộ vào đúng cột ấy với mọi ca kiểm vẫn xanh.
	@#
	@# `.claude/hooks/audit_actor_guard.py` chặn cùng lớp lỗi lúc GHI, nhưng chỉ thấy MỘT tệp.
	@# Năm trong sáu khiếm khuyết ấy đã nằm sẵn trên đĩa trước khi có rào nào.
	$(PYTHON) tools/check_audit_actor.py

khoaduynhat:                    ## Khoá duy nhất hợp thành với `tenant_id`, và không tính sót dòng đã xoá mềm
	@# HAI LUẬT ĐANG ĐÚNG MÀ KHÔNG GÌ KIỂM — luật 1 bất biến 6 và luật 7 bất biến 3. Cả hai
	@# hỏng im lặng: không bài test nào đỏ, không lần chạy nào hỏng, và chỉ lộ ra ở một XÃ THẬT.
	@#
	@# `UNIQUE (…) WHERE deleted_at IS NULL` cho phép xoá mềm rồi thêm lại CÙNG MỘT MÃ — tức
	@# một mã đã in ra giấy, đã đóng dấu, đã gửi đi thì được cấp lại. Kho anh em
	@# `../vigov-require` đã trả giá cho đúng bẫy này.
	@#
	@# `UNIQUE (ma)` thiếu `tenant_id` thì xã thứ hai không onboard được — và nó chỉ đỏ vào
	@# đúng ngày có xã thứ hai, khi sửa đã là migration trên dữ liệu đang chạy.
	@#
	@# Cổng này dựng lúc cả 49 khai báo đều ĐÚNG, tức nó GIỮ một tính chất chứ không dọn một
	@# đống đã hỏng — đúng lúc rẻ nhất.
	$(PYTHON) tools/check_khoa_duy_nhat.py

envmap:                         ## Bảng map biến môi trường ở deploy/README.md còn khớp mã không
	@# `deploy/README.md` mục 5 là nơi DUY NHẤT trả lời "biến này do ConfigMap hay Secret cấp".
	@# `core/config` chỉ biết đọc, `.env.example` chỉ giữ chỗ, và `env_contract_guard` nói thẳng
	@# rằng nó KHÔNG kiểm chỗ ràng buộc. Một fact viết tay cạnh một danh sách mọc từ mã là đúng
	@# hình dạng sẽ trôi — và lúc trôi, người vận hành đọc bảng rồi tin là đã khai đủ.
	$(PYTHON) tools/check_env_map.py

buildfiles:                     ## Dockerfile + Jenkinsfile của từng dịch vụ, và phần chung không ai đánh rơi
	@# Mỗi dịch vụ tự dựng và tự đóng gói: nó quyết build cái gì, khi nào, ra ảnh nào.
	@# Cái giá là chín bản sao sẽ trôi — và phần trôi trước tiên luôn là phần KHÔNG gây lỗi
	@# ngay: chạy bằng root, thiếu zoneinfo, hoặc đánh rơi `core/**` khỏi đường kích hoạt để
	@# rồi một bản vá trong mã dùng chung không kích hoạt dịch vụ nào cả.
	$(PYTHON) tools/check_build.py

lint:
	@# gofmt -l PRINTS unformatted files and still EXITS 0, so for as long as this target
	@# simply called it, unformatted code walked straight through the gate while the file
	@# names scrolled past. A gate that reports and passes is not a gate.
	@# core/gen/ is excluded because it is produced by `make proto`, not written here.
	@out=$$(gofmt -l . | grep -v '^core/gen/' || true); \
	if [ -n "$$out" ]; then \
		echo "gofmt: chưa định dạng, chạy gofmt -w:"; echo "$$out"; exit 1; fi
	go vet $(MODULES)
	-golangci-lint run $(MODULES)
	@# buf lint is NO LONGER prefixed with `-`. It was ignored while it was failing; it has
	@# been clean since the contract fixes, so ignoring it now only hides the next regression.
	buf lint
	@# `buf.yaml` KHAI HẠNG `FILE` — một lời hứa tương thích ở mức MÃ NGUỒN, không chỉ mức dây.
	@# Cho tới hôm nay không cổng nào kiểm lời hứa ấy: `buf breaking` chưa từng chạy trong
	@# `make check`, nên một lần đổi tên RPC đi qua hoàn toàn im lặng và cổng vẫn xanh. Đó đúng
	@# hình dạng "biện pháp không phải biện pháp" mà tuần này đã dọn năm lần.
	@#
	@# SO VỚI COMMIT LIỀN TRƯỚC, không phải một mốc cố định. Mốc cố định trả lời câu "đã trôi
	@# bao xa từ một ngày nào đó" — câu ấy càng ngày càng ít ai đọc, rồi thành một cổng luôn đỏ
	@# vì chuyện cũ và bị gỡ ra. `HEAD` hỏi đúng câu đáng hỏi, "thay đổi ĐANG LÀM có phá hợp
	@# đồng không", vào đúng lúc sửa còn rẻ, và không có con số nào phải nhớ cập nhật.
	@#
	@# KHÔNG có tiền tố `-`. Nuốt lỗi ở đây là dựng lại đúng cái vừa gỡ. Ngày cần phá vỡ có chủ
	@# ý thì cổng đỏ CHÍNH LÀ tính năng: nó buộc người làm nói ra lý do thay vì đi qua im lặng.
	buf breaking --against '.git#ref=HEAD'

build:
	go build $(MODULES)

standalone:                     ## Mỗi module build được KHI KHÔNG CÓ go.work — đúng điều Docker làm
	@# go.work làm mọi module thấy nhau qua thư mục, nên nó CHE mất một `require` bị thiếu.
	@# Ảnh Docker không có go.work: nó chép core/ + module của dịch vụ và dựa vào `replace`
	@# trong go.mod của dịch vụ ấy. Không có mục này, một go.mod thiếu require vẫn xanh suốt
	@# ở máy trạm rồi đổ ở CI lúc đóng ảnh — xa nhất có thể khỏi chỗ gây ra lỗi.
	@for m in $(MOD_DIRS); do \
		( cd "$$m" && GOWORK=off go build ./... ) \
			|| { echo "ĐỎ: $$m không build được khi thiếu go.work"; exit 1; }; \
		echo "  OK   $$m"; \
	done

test:
	@# -race is not optional here. One process serves 200+ communes, so a data race is a race
	@# BETWEEN COMMUNES, and that is the class of defect that stays invisible until two
	@# communes are live. It needs a C toolchain; if this fails to start, fix the toolchain
	@# rather than dropping the flag.
	go test -race -count=1 $(MODULES)

web:                            ## Typecheck + test the Next.js apps — ĐỎ khi một app thiếu deps
	@# Deliberately not `next build`: a production build is the CI image's job (web-admin/Dockerfile),
	@# and doing it here would make the local gate several times slower for no extra signal.
	@#
	@# check:api is part of THIS target on purpose. It re-derives src/lib/api/schema.gen.ts from
	@# kb/20-contracts/openapi.json and fails when they differ — the one check that catches a
	@# REST contract change the web has not been regenerated for. Listing it only in Jenkinsfile
	@# would give the project two definitions of "verified", and the looser one always wins.
	@# ĐỎ, KHÔNG PHẢI BỎ QUA — đổi 22/09/2026, và đây là câu mà sổ tiến độ để ngỏ:
	@# *"một đơn vị chưa có mã thì nên BỎ QUA hay nên làm đỏ cổng"*. Trả lời: đỏ khi nó CÓ mã.
	@#
	@# Bản cũ in ra một câu rất rõ rằng mã TypeScript không được kiểm, rồi trả rc=0. Câu ấy đi
	@# vào log CI giữa hàng trăm dòng khác và không ai đọc; thứ người ta đọc là màu của cổng, và
	@# màu ấy nói "đã kiểm". `platform-admin` sống như thế từ 20/09 tới 22/09/2026 — một đơn vị
	@# triển khai được, có mã TypeScript, chưa từng đi qua một phép kiểm nào.
	@#
	@# `npm test` và `check:api` chạy CÓ ĐIỀU KIỆN vì không phải app nào cũng khai chúng, và
	@# `npm test` trên một app không có script ấy sẽ đỏ vì lý do sai — "chưa viết test nào" không
	@# phải một khiếm khuyết cùng loại với "mã không biên dịch được". `tsc` thì KHÔNG có điều
	@# kiện nào: mọi app đều phải biên dịch được.
	@for app in */; do \
		if [ -f "$$app/package.json" ]; then \
			if [ ! -d "$$app/node_modules" ]; then \
				echo "[FAIL] $$app có package.json nhưng chưa có node_modules — mã TypeScript của nó KHÔNG được kiểm dòng nào."; \
				echo "       Một cổng xanh trong khi một đơn vị triển khai không được kiểm là một cổng nói dối."; \
				echo "       chạy: (cd $$app && npm install)"; \
				exit 1; \
			fi; \
			echo "typecheck $$app"; \
			(cd "$$app" && npx --no-install tsc --noEmit) || exit 1; \
			if (cd "$$app" && npm run 2>/dev/null | grep -q '^  test$$'); then \
				echo "test $$app"; \
				(cd "$$app" && npm test --silent) || exit 1; \
			else \
				echo "  (bỏ qua test $$app — package.json không khai script 'test')"; \
			fi; \
			if (cd "$$app" && npm run 2>/dev/null | grep -q '^  check:api$$'); then \
				echo "check:api $$app"; \
				(cd "$$app" && npm run --silent check:api) || exit 1; \
			else \
				echo "  (bỏ qua check:api $$app — package.json không khai script 'check:api')"; \
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
	@# SAU `apidoc`, và thứ tự ấy bắt buộc: bảng định tuyến Ingress sinh TỪ openapi.json, nên
	@# chạy trước thì nó sinh ra từ bản hợp đồng cũ. Đây là mắt thứ hai của cùng một chuỗi —
	@# route trong Go → openapi.json → deploy/base/mang/ingress.yaml — và nó nằm ở đây vì lý
	@# do đã viết ngay trên: một lệnh sinh lại mọi thứ dẫn xuất từ mã. Ingress lệch hợp đồng
	@# nghĩa là tuyến của dịch vụ không phải identity trả 404 dù pod xanh và probe xanh.
	go run ./tools/ingress
	@# Tầng tiến độ: tệp ĐỌC sinh từ các tệp GHI theo module. Nằm cùng mục `kb` vì lý do đã
	@# viết ngay trên: thứ phải nhớ chạy riêng là thứ sẽ có ngày không ai chạy, và một
	@# `tien-do.md` cũ hơn các tệp module là tệp nói dối về việc gì đã xong.
	VIGOV_COMMIT=$$(git rev-parse --short HEAD) $(PYTHON) tools/tien_do.py
	@# Bản estimate gửi khách sinh từ bản nội bộ. Hai tệp estimate chép tay là hai tệp sẽ lệch,
	@# và bản lệch là bản có người gửi ra ngoài.
	$(PYTHON) tools/estimate_khach.py

proto:                        ## Regenerate Go code from .proto (requires buf)
	buf generate

tidy:
	@# Không còn `go mod tidy` ở gốc: gốc kho không thuộc module nào. Mỗi module tự tidy,
	@# và GOWORK=off là cố ý — tidy trong chế độ workspace không ghi lại require cho
	@# những gì go.work đang che, nên nó để lại một go.mod chỉ build được khi có go.work.
	@for m in $(MOD_DIRS); do \
		echo "tidy $$m"; ( cd "$$m" && GOWORK=off go mod tidy ) || exit 1; \
	done
