---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 5617c800
expires: 2027-01-01
owns_facts:
  - "quyết định đã chốt với chủ dự án và nơi ghi, cạm bẫy đã gặp, phiên song song đang giữ đường dẫn nào — tại 03/10/2026"
---

# Bàn giao phiên — cập nhật 2026-10-03

**Đọc SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.** Tệp này chỉ trả lời:
*đã quyết gì và ghi ở đâu, ai đang giữ đường dẫn nào, cạm bẫy nào đã tốn thời gian.*

Viết bằng `/handover`: **một tệp, ghi đè trọn vẹn**, ngày nằm bên trong. Hết hạn **2027-01-01**;
sau ngày đó tin `git log`, đừng tin tệp này.

**Cách kiểm:** mọi dòng giữ từ bản `af3ffcd9` (01/10) đã mở tệp kiểm lại trên đĩa ngày 03/10; dòng
không kiểm được đã bỏ, hoặc ghi rõ *chưa kiểm*. Dòng mới 03/10 viết từ ADR và mã đã mở.

---

## 1. Đã làm — chỉ những gì `git log` không nói

### Quyết định 03/10 — ảnh trong thân bài, tin kiểu báo

**Để làm gì:** tin của xã trên app riêng đọc như một bài báo — ảnh xen giữa đoạn, chú thích, trích
dẫn, dòng tác giả/nguồn, sapo là ô `Tóm tắt` sẵn có. Tệp sở hữu: **ADR 0067 §"Sửa đổi 03/10/2026"**
(H1–H10, K9–K11) và §L (còn mở). Đừng chép lại ở đây.

Hai chỗ dễ đọc sai trong cùng ADR 0067:

| Điểm | Vì sao dễ sai |
|---|---|
| Link ảnh cán bộ dán vào do **MÁY CHỦ** tải về (H5), chỉ giữ bản mã hoá lại, không giữ bản gốc (K9) | Giữ bản gốc là câu L4 còn mở, cần thao tác mới trong `core/storage` (ADR 0052) — không tự thêm |
| Trần 30 lần/giờ/cán bộ ở tuyến lấy ảnh từ link, **đếm cả lần hỏng, Redis hỏng thì ĐÓNG** (K10) | Ngược chiều với trần 120/phút/IP của tin công khai cùng ADR (Redis hỏng thì **cho qua**). Hai trần, hai chiều hỏng, đều có chủ ý |

### Quyết định 01–02/10 — do các phiên khác chốt với chủ dự án

Mỗi dòng chỉ là con trỏ; nội dung ở tệp sở hữu. Đã kiểm tệp có trên đĩa, **không** kiểm từng chữ.

| Chủ đề | Ghi ở |
|---|---|
| Đăng nhập app riêng của xã chuyển vào `service-identity`; `vihat-miniapp` chỉ còn app chung/demo | ADR 0066 |
| Nội dung Mini App phần A (rich text lọc ở máy chủ, đồng bộ Cổng, danh mục, truyền thanh, banner) | ADR 0067 (01–02/10) |
| Làm mới giao diện web-admin; phần chưa dựng là control vô hiệu dấu "?" | ADR 0068 §11–§15 |
| Logo + banner web-admin do xã tự tải | ADR 0069 |
| Đổi/gỡ App ID Mini App từ platform-admin; khoá bí mật đi qua platform rồi identity gRPC | ADR 0070 |
| Giao diện app riêng theo prototype khách, `--demo` làm lại, lượt xem tin, ảnh hiện trường, ảnh nghiệm thu | ADR 0047 §6 (các dòng 30/09–02/10) |
| Nhập hộ phiếu, 9 câu trạng thái nguyên văn, quá hạn đo bằng giờ làm việc ở identity | ADR 0028 · 0027 · 0007 |
| Triển khai khu vận hành `admin.vigov.vn`; một nguồn tên miền công khai `deploy/hosts.yaml` | ADR 0048 · 0046 (sửa đổi 01/10) |

### Đã phân tích, CHƯA quyết — đừng ghi thành quyết định

Đổi tên miền xã từ `<xa>-<tinh>.vigov.vn` sang `<xa>.<tinh>.vigov.vn`: biên Go tra **nguyên chuỗi Host**
(`service-platform/internal/store/directory.go:99-103`) nên chạy được bằng cách thêm dòng tên miền qua
tuyến vận hành; luật tên miền dành riêng **không** chặn `admin.<tinh>.vigov.vn` / `<tinh>.vigov.vn` (hở,
chủ dự án chưa quyết); đã khuyên **không** tách `tenant_domain.host` thành hai cột. Nền: ADR 0046.

### Giữ từ bản trước — đã kiểm lại 03/10

| Quyết định | Kiểm ở |
|---|---|
| **Gỡ toàn bộ chiến dịch đổi tên tiếng Anh** (ADR 0061). Tên MỚI tiếng Anh, tên CŨ không đổi. Đừng đề xuất đổi tên hàng loạt lại | commit `090d6b0c` · luật 12 |
| **Bản test là bản thật:** không chữ "demo"/"trải nghiệm" trong app; `--demo` chỉ thay danh tính, chạy thật tới máy chủ | ADR 0047 §6 dòng "Cờ `--demo` làm lại" · ghi nhớ `ban-test-la-that` |
| Mã QR bản thử v30 trong App ID do ViHAT Group đứng tên là **prototype của `vigov-require`**, không phải `citizen-app` — App ID ấy ghim cứng trong `vigov-require/apps/miniapp/zmp-cli.json` | ADR 0031 · tệp đã nêu |
| `citizen-app/ui-design/` (tài liệu khách gửi) không vào kho | `citizen-app/.gitignore` · `tools/check_brain.py` |
| Trần `always_load` 28000 token — **đừng tự nâng** | `kb/INDEX.yaml` `budget` |
| Cụm thật PostgreSQL 16 | ghi nhớ `pg16-miniapp-chua-len-zalo` |
| Chưa có **dữ liệu thật** trên môi trường nào (người dùng nói 30/09) | **chưa kiểm** — không có gì trên đĩa chứng minh được |

`kb/00-foundation/open-questions.json` (kiểm 03/10): **39 câu, cả 39 DECIDED.** Nhiều câu là **đề
xuất của nhà cung cấp**, không phải trả lời của khách: đọc §"Hệ quả — cái gì đỏ nếu một mục ở đây
bị phủ quyết" cuối ADR 0035 trước khi dựa vào.

---

## 2. Việc kế tiếp

**Không nằm ở đây.** `kb/90-ephemeral/tien-do.md`, một mục theo module. Lượt ảnh thân bài:
`python tools/tien_do.py --menu "noi-dung-mini-app"` (mục `service-comms/anh-trong-than-bai`,
`citizen-app/than-bai-anh-trich-dan-sapo`, `web-admin/noi-dung-anh-trich-dan-tac-gia`).

---

## 3. Đang bị chặn

**Bảng "Nợ khách chốt" hiện KHÔNG có trong `tien-do.md`** — và đó không phải tin tốt. Bộ sinh
(`tools/tien_do.py`) chỉ in bảng khi một mục có `no_confirm`, và `no_confirm` chỉ nhận số hiệu câu trong
`open-questions.json`; cả 39 câu đã DECIDED nên bảng rỗng. Câu đang chờ chủ dự án/vận hành vì thế nằm
ở hai chỗ khác, không gom về đâu:

- mục **"Còn mở"** của từng ADR — ví dụ ADR 0067 §L (L1–L4), ADR 0052 §Còn mở #3:
  `grep -n "Còn mở" kb/10-decisions/*.md`;
- trường `tiep_theo` (văn xuôi) của từng mục sổ.

Ví dụ hở đo được 03/10: L1 và L2 có trong `tiep_theo` của `citizen-app/than-bai-anh-trich-dan-sapo`;
**L4 không có trong sổ nào**.

---

## 4. Phiên song song

Lúc viết (03/10), cây làm việc chung có việc **chưa commit** của phiên khác. Đừng stage, đừng checkout:

| Đường dẫn | Việc |
|---|---|
| `core/storage/key.go`, `proto/vigov/platform/v1/platform.proto` | mục đích tải lên `staff-avatar` (ảnh cán bộ, identity, menu `danh-ba-can-bo`). Làm đỏ `TestUploadPolicyPurposeCheckMatchesStorage` (`service-platform/internal/store`) tới khi migration platform định nghĩa lại CHECK được commit |
| `tasks/web/open/` — 6 thẻ chưa theo dõi (`staff-counts`, `external-contacts`, `staff/{id}/account`; màn `12-danh-ba-can-bo`) | cùng lượt danh bạ cán bộ, sinh từ một lần `make kb` |

`tasks/web/claimed/` giữ 5 thẻ nhận ngày 30/09–01/10: `9604db83ba23`, `dc972835a13f`, `df8ab1449c0b`
(chốt kỳ ngân sách, finance) · `ae174d9a3f26` (tạo nhiệm vụ từ phiếu, petitions) · `b1e4e0acb2a2`
(công khai cán bộ hàng loạt, identity). web-admin đã có mã gọi cả năm tuyến — nhiều khả năng là thẻ
quên chuyển sang `done/`. **Chưa kiểm** phiên nào nhận.

Rác chưa theo dõi, không rõ chủ: `img.png`, `img_1.png`, `img_2.png` (gốc kho), `service-comms/grep.exe.stackdump`,
`tools/apidoc/zz_tmp_probe_test.go` — tệp cuối là một ca test **chạy thật trong `make check`**. Đề xuất
xoá, chờ người dùng xác nhận.

Worktree còn sót (`git worktree list`): `D:/wt-kb` và `D:/works/vihat/wt-kb` có `node_modules` là
**junction trỏ vào kho chính** — xem §5.

---

## 5. Cạm bẫy đã gặp

Một nửa bảng có chung một dạng: **phép kiểm xanh hoặc đỏ vì lý do sai**. Gặp cái tiếp theo cùng dạng
thì hỏi cả lớp ấy còn ở đâu.

### Lượt 03/10 — mới, đã gặp thật

| Triệu chứng | Sự thật |
|---|---|
| `make check` đỏ ở một test mình không chạm | Tệp chưa commit của phiên song song trong cây chung (§4). Chứng minh HEAD xanh: `git worktree add --detach <thư mục> HEAD`, chạy `buf generate` trong đó (`core/gen` bị gitignore nên worktree mới không build), xong `git worktree remove --force` |
| Gỡ worktree có `node_modules` junction | `worktree remove --force` xoá **xuyên** junction, mất `node_modules` thật của kho chính (02/10). Xoá junction trước, hoặc đừng tạo. Hai worktree ở §4 đang ở đúng trạng thái này |
| `make kb` sinh `openapi.json`/Ingress mang tuyến của phiên khác | Bộ sinh đọc cả cây. Xem `git diff kb/20-contracts/openapi.json` theo đường dẫn tuyến trước khi commit |
| Commit `schema.gen.ts` mới sinh, web-admin đỏ ở commit sau | Trường phản hồi **bắt buộc** mới làm vỡ fixture test. Thứ tự: `npm run gen:api` → `npm run typecheck` → commit. `make kb` **không** sinh tệp này |
| Phiên khác commit cả tệp sổ `tien-do/<module>.json`, cuốn theo chỗ mình đang sửa | Mọi phiên dùng chung một index và một cây. Commit sổ ngay sau thẻ; đừng coi tệp sổ đang sửa là của riêng mình. `git add` ngay trước `git commit` trong cùng một lệnh |
| Ngày ghi vào ADR/migration | Lấy từ `date` của máy. Đầu migration và lý do kiểm toán bị checksum khoá khi đã áp — ngày sai không sửa được |
| `data_safety_guard` chặn migration chỉ vì chú thích | Nó bắt cả `UPDATE … SET` trong **văn xuôi**. Viết cách đảo ngược như mục REVERSAL của `service-platform/migrations/0016_upload_policy_tenant_logo_banner.sql` |
| IDE/LSP báo "undefined" ở tệp phiên khác vừa tạo | `go build` là sự thật, không phải LSP |
| Lệnh Bash ghép nhiều lệnh bị từ chối | Lớp cấp quyền của phiên (kể cả khi nhắc tên kho anh em). Tách lệnh, đọc bằng Read/Grep |

### Giữ từ bản trước — đã kiểm lại 03/10

| Triệu chứng | Sự thật |
|---|---|
| `git reset --hard` bị chặn | `data_safety_guard` chặn; dùng `git stash`. `git checkout --` xoá việc chưa commit của phiên khác |
| Go build theo mã sinh cũ | `core/gen/` bị gitignore; `mingw32-make proto` trước `build` |
| `lint` đỏ ở `buf breaking` | So cây làm việc với `HEAD` — một `.proto` chưa commit của phiên khác cũng tính |
| `go test` xanh, `make check` đỏ ở `lint` | `gofmt -l` chạy trong `lint` và làm đỏ; `gofmt -w <module>` trước commit |
| `tien-do.md` xung đột | Tệp SINH; lấy một phía rồi `make kb` |
| Mở cổng 9091 mà Mini App vẫn không vào | `vihat-miniapp` cùng namespace; `deny-all` chặn cả vào lẫn ra (`deploy/base/mang/netpol.yaml`) |
| Tệp "down" migration đặt cạnh "up" | `core/migrate` áp **mọi** `*.sql` trong thư mục. Đảo ngược viết trong chú thích |
| `ADD CONSTRAINT … NOT VALID` trên bảng phân vùng | Khoá ngoại NOT VALID trên bảng phân vùng bị từ chối trước PG 18; cụm thật là PG 16 |
| `node --test citizen-app/scripts/*.test.mjs` đỏ | Các tệp đó là vitest: `npx vitest run scripts/<tệp>` |
| `web-admin/src/lib/api/noi-dung.test.ts` đỏ sau khi thêm trường ở comms | Nó khoá bộ trường form = bộ trường hợp đồng. Hợp đồng mới đi cùng web-admin |
| Tuyến công khai mới của comms trả 404 | Phải nằm trong cây con `/api/v1/commune-news/` đã tách ở `service-comms/cmd/server/main.go`; ngoài cây con sẽ rơi vào chuỗi cán bộ |
| Sửa tệp bằng `python -c` qua Bash | Hook PreToolUse gắn vào Edit/Write không chạy (`tools/tien_do.py` nêu lý do). Soát tay diff |
| `UnicodeEncodeError: 'charmap'` | `PYTHONIOENCODING=utf-8` |
| Ca `_pg_test` "xanh" | Là SKIP khi `VIGOV_TEST_DSN` trống |
| `make kb \| grep` "xanh" | Ống dẫn che mã thoát |
| Kết quả grep âm tính | Không phải bằng chứng vắng mặt; mở tệp |

---

## 6. Cổng kiểm

```
GOCACHE=/d/gocache PYTHONIOENCODING=utf-8 mingw32-make check
```

**03/10, chạy thật trên cây làm việc chung tại `5617c800`: ĐỎ (exit 2)** ở mục `test`, đúng một ca:
`TestUploadPolicyPurposeCheckMatchesStorage` — do `staff-avatar` chưa commit của phiên khác (§4).
brain, hooks, quyen, vet-actor, khoaduynhat, envmap, buildfiles, security, `buf lint`, `buf breaking`,
build, standalone: xanh. **Mục `web` (typecheck, test, `check:api` của ba app) KHÔNG chạy** vì make dừng
ở `test`. Phiên chính đã chạy lại **trên worktree sạch ở `5617c800`** (sau `buf generate`): `go test`
`service-platform/internal/store`, `core/storage`, `core/ratelimit` **xanh** — ca đỏ chỉ do tệp chưa commit.
Mục `web` được chạy riêng trong phiên (web-admin typecheck + lint + test + `check:api`; citizen-app
typecheck + test): xanh tại `71d084c0` / `ceeddf6d`. **Chưa** chạy lại toàn bộ `make check` trên worktree sạch.

**Thứ cổng KHÔNG phủ trên máy này:**
- `golangci-lint` không cài: `lint` báo `Error 2 (ignored)`.
- Mọi ca PostgreSQL SKIP (không có `VIGOV_TEST_DSN`): `service-platform/migrations/0018_upload_policy_content_body_image.sql`
  và SQL `stored_file`/việc nền gỡ ảnh nháp của comms **chưa từng chạy trên PostgreSQL**.
- `make vuln` không nằm trong `check`. Phiên chính đo 03/10: **đỏ** ở `braces` của citizen-app
  (GHSA-vfj7-8cjw-p6xm, high) — có từ trước, chặn cổng Jenkins tới khi nâng bản hoặc thêm ngoại lệ có
  hạn vào `tools/vuln_exceptions.json`. Lượt viết này không chạy lại.
- `service-comms` chưa có request id: `traceID` trả rỗng (`service-comms/cmd/server/main.go:646`).
- MinIO/ClamAV thật, Jenkins/k8s (cụm dựng tay trên Rancher), trình duyệt thật, webview Zalo.

---

## 7. Việc treo

**Không nằm ở đây.** Mỗi việc treo nằm ở module của nó với `trang_thai: "treo"` trong
`kb/90-ephemeral/tien-do.md`. **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là chờ một
câu ở §3.
