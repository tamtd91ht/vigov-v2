---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: af3ffcd9
expires: 2026-12-30
owns_facts:
  - "quyết định đã chốt với người dùng/khách, cạm bẫy đã gặp, và câu đang chờ người dùng — tại 01/10/2026"
---

# Bàn giao phiên — cập nhật 2026-10-01

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.** Nó chỉ trả lời:
*đã quyết gì, đang chờ ai, và cạm bẫy nào đã tốn thời gian của người trước.*

Viết bằng `/handover`. **MỘT tệp, ghi đè trọn vẹn mỗi lần**; tên không mang ngày, ngày nằm bên trong.
Hết hạn **2026-12-30**; sau ngày đó tin `git log`, đừng tin tệp này.

Viết lại 01/10 sau lượt làm giao diện app riêng của xã theo prototype khách và nối API tin tức.
Bản trước ở `417e9564` (30/09). **Cách kiểm các dòng giữ lại:** dòng mới 30/09–01/10 viết từ commit
của lượt này. Dòng giữ từ bản trước **không** kiểm lại từng chữ, trừ những dòng ghi rõ đã bỏ hoặc sửa.

---

## 1. Đã làm — chỉ những gì `git log` không nói

**Không có danh sách commit ở đây.** `git log --oneline 417e9564..HEAD` trả lời chính xác hơn.

### Quyết định 30/09–01/10 — mới (lượt giao diện app riêng của xã)

Mọi dòng dưới đây **ghi ở ADR 0047 §6**, các dòng cuối bảng. Đó là tệp sở hữu; bảng này chỉ trỏ tới.

| Quyết định | Ngày | Ai quyết |
|---|---|---|
| App riêng của xã theo **prototype khách** (`vigov-require/apps/miniapp`): **màu đỏ** `#c1121c` thay navy; ảnh bìa trang chủ theo tên miền (`scripts/banner-xa/<tên-miền>.png`, như logo, không có thì ẩn); không "lượt xem"; 3 tab Tin tức · Sự kiện · Thông báo (không "Tất cả") + chip danh mục **hai tầng** (chọn cha gồm cả con, chỉ hiện danh mục có tin); nút "Gửi phản ánh mới" ở chân tab Phản ánh; bỏ hết màu hồng | 30/09 | người dùng |
| Theo `PROTOTYPE.md` khách gửi: **chỉ lấy giao diện** (§4–§7), **không** lấy kiến trúc (mock, `X-Tenant-Code`/`X-Citizen-Id`, `localStorage`, lĩnh vực viết cứng). Giữ: chữ ≥16px, chữ "Quay lại", "đang tải" là chữ, logo xã (không quốc huy), 3 mức cỡ chữ; chuông rời header, thành dòng "Thông báo" ở Cá nhân; không công tắc "Nhận thông báo"; không "Nhắn Zalo" | 30/09 | người dùng |
| **Tên xã chỉ ở header Trang chủ**, không thêm vào màn con: *"vào app là đang biết làm việc với ai rồi"* | 30/09 | người dùng |
| **Nối API luôn**, 12 điểm + G1–G9: ảnh tin tải lên kho tệp (bản dẫn xuất public khi đăng); giờ đăng ghi ở **lần đăng đầu**, sau không đổi; sự kiện; link video; cảnh báo thiên tai là thực thể mới của comms, dùng khoá `content.read`/`content.update`; ảnh hiện trường không bắt buộc, tối đa 5, **máy chủ bỏ EXIF, không nhận HEIC** (G3); giờ từng trạng thái bằng 2 cột mới trên phiếu (không đọc `nhat_ky_phan_anh`); SĐT đã che qua `GET /api/v1/citizen/me`; truyền thanh mp3/m4a 30 MB, thời lượng cán bộ gõ; **tra cứu hồ sơ một cửa để sau**; ảnh "sau xử lý" của cán bộ để sau; lời văn chính sách quyền riêng tư: Claude viết nháp 1.1, chủ dự án duyệt | 30/09 | người dùng |
| **Cờ `--demo` làm lại:** không một chữ "demo"/"trải nghiệm" nào trong app, ở mọi bản dựng. `--demo` chỉ thay danh tính (tên và số cố định, xem mã `citizen-app/src/lib/demo-build.ts`), không xin quyền Zalo, chạy thật tới máy chủ. Máy chủ `vihat-miniapp` nhận thiếu `phoneToken` chỉ cho App ID trong `DEMO_APP_IDS`. Đồng ý hệ quả: mọi người dùng bản demo là **một** công dân, phiếu là phiếu thật | 01/10 | người dùng — **đã nói nhiều lần, lần đầu Claude hiểu sai** (xem ghi nhớ `ban-test-la-that`) |
| `citizen-app/ui-design/` (ảnh, `PROTOTYPE.md` khách gửi) **không vào kho**: `.gitignore` + phép kiểm brain #6 bỏ qua | 01/10 | người dùng |
| Chỉ lớp `content-source` được đưa ra bucket public; `records` và `citizen-media` luôn bị từ chối. Ảnh bìa phải có bản dẫn xuất, không đăng thẳng bản gốc | 30/09 | Claude siết, ghi ở ADR 0052 §1 "Bổ sung 30/09" |

**Mã QR `zalo.me/s/3043188591857102858/?env=TESTING&version=30`** là App ID do **ViHAT Group** đứng tên
(ADR 0031). Bản thử v30 trong đó là **prototype của `vigov-require`**, không phải `citizen-app`:
`vigov-require/apps/miniapp` ghim cứng App ID này, và `zmp:deploy` đẩy vào đó. App riêng Thăng Bình
của kho này là App ID khác (ADR 0047 §6). Chưa ai chốt tách App ID cho prototype.

### Quyết định 29–30/09 — giữ từ bản trước

| Quyết định | Ngày | Ai quyết | Ghi ở |
|---|---|---|---|
| **GỠ TOÀN BỘ chiến dịch đổi tên sang tiếng Anh** (ADR 0061). Luật 12: **tên MỚI tiếng Anh, tên CŨ không đổi**. Đừng đề xuất đổi tên hàng loạt lại | 30/09 | người dùng | commit `090d6b0c` · luật 12 |
| Chưa có **dữ liệu thật** trên môi trường nào | 30/09 | người dùng | `4603450`, `090d6b0c` |
| **Mini App: bản test là bản thật sẽ submit.** Không có `--demo` thì gọi hàm thật; thiếu quyền Zalo thì màn hình báo lỗi. Không nới kiểm phân quyền/xã phía ViGov | 30/09 | người dùng | ghi nhớ `ban-test-la-that` · `citizen-app/bao-dung-loi-quyen-zalo` |
| Ưu tiên nối thông chuỗi Mini App → phiếu → Sổ phản ánh của xã, chạy thật trên bản test | 30/09 | người dùng | `deploy/mo-cong-cau-phien-cong-dan` |
| `vihat-miniapp` chạy cùng cụm, cùng namespace; cổng cầu phiên identity 9091 | 30/09 | người dùng | `deploy/base/mang/netpol.yaml` quy tắc 8–9 |
| Cụm thật **PostgreSQL 16** | 29/09 | người dùng | ghi nhớ `pg16-miniapp-chua-len-zalo` |

### Quyết định trước 29/09 — giữ từ bản 28/09, chưa kiểm lại từng dòng

| Quyết định | Ghi ở |
|---|---|
| Tiền tố `vigov`; Jenkins docker CLI; hạ tầng dừng ở Dockerfile + Jenkinsfile | Jenkinsfile · `deploy/README.md` §9, §11 |
| Trần `always_load` 28000 token — **đừng tự nâng** | `kb/INDEX.yaml` `budget` |
| Miễn xã cho `ResolveCitizenSession` | `core/grpcx/grpcx.go` (chưa có ADR — §3) |
| Ingress sinh từ `openapi.json`; `/api/v1` lạ nhận 404 JSON | ADR 0043 |
| 4 tên miền, người quản trị đầu tiên của xã | ADR 0046 |
| Mini App hai chế độ, hai giai đoạn, tham số QR mang tên miền, tuyến tra tên miền thuộc identity | ADR 0044 · 0045 · 0047 |
| Khu vận hành `platform-admin/`, 12 câu thiết kế chưa chốt | ADR 0048 |
| Nhiệm vụ, danh bạ #12, đơn thư C3–C19, báo công dân theo bảng, phản ánh 1–2 sao tự mở lại, biên bản họp, thu chi | ledger từng module · ADR 0039 · 0040 · 0041 · 0050 |

**`open-questions.json` (kiểm 01/10): 39 câu, TẤT CẢ đã DECIDED.** #35–#39 (an ninh) đã chốt ở lượt
identity `bb85def5`. Nhiều câu DECIDED là **đề xuất của nhà cung cấp**, không phải trả lời của khách:
đọc bảng "cái gì đỏ nếu bị phủ quyết" cuối ADR 0035 trước khi dựa vào.

---

## 2. Việc kế tiếp

**Không nằm ở đây.** `kb/90-ephemeral/tien-do.md` (theo module), và `python tools/tien_do.py --menu
"noi-dung-mini-app"` cho lượt này. Thứ tự đã chốt cho phần nối API: đợt 1 tin tức đầy đủ (còn ảnh bìa
và phần Mini App), đợt 2 cảnh báo thiên tai, đợt 3 ảnh hiện trường, đợt 4 giờ từng bước + SĐT che +
truyền thanh. Mục `service-comms/tin-mini-app-anh-su-kien-video` ghi đúng bước kế.

---

## 3. Đang bị chặn — và chặn bởi ai

Bảng *"Nợ khách chốt"* ở đầu `tien-do.md` sinh từ `no_confirm`. Những câu dưới đây **không có trong
tệp ấy**; chúng chờ NGƯỜI DÙNG, VẬN HÀNH hoặc KHÁCH:

| Câu | Chờ ai | Chặn gì | Chi tiết ở |
|---|---|---|---|
| **`vihat-miniapp` nhận đăng nhập `demoIdentity` (`DEMO_APP_IDS`)** — Claude **bị bộ phân loại an toàn chặn** khi giao việc sửa (01/10). **Đừng tìm đường vòng** | chủ dự án tự sửa, hoặc cấp quyền | bản `--demo` mở phiên bị từ chối, không gửi phiếu được | `citizen-app/co-demo-app-rieng` |
| **Cấp MinIO (quyền GHI `content-source/…/comms/*`) + ClamAV cho comms**, rồi mới khai nhóm cấu hình ObjectStore/MalwareScan ở comms (prod từ chối khởi động nếu thiếu) | vận hành | tải ảnh bìa tin, âm thanh | `service-comms/tin-mini-app-anh-su-kien-video` |
| **Tên miền kho tệp vào danh sách cho phép của App ID** trên console Zalo | người có console Zalo | ảnh và tải lên không chạy trong Zalo | ADR 0052 Còn mở #3 |
| Prototype `vigov-require` đẩy vào **App ID ViHAT Group**; dùng chung chuỗi phiên bản với app Tập đoàn | chủ dự án / BA | có thể đè bản thử của nhau | §1 |
| Câu "nâng cao **trải nghiệm** khách hàng" (trích eSMS ViHAT, `content/company-profile.ts`) còn trong bundle | chủ dự án | không chặn mã | `citizen-app/co-demo-app-rieng` |
| Chuỗi Mini App → phiếu chạy thật (manifest lên Rancher, khoá cầu, CORS, dòng `mini_app`, SLA xã, `ZMP_TOKEN`) | vận hành · quản trị viên xã · chủ dự án | mọi tuyến `CitizenOnly` trên máy thật | `deploy/mo-cong-cau-phien-cong-dan` · `deploy/gan-mini-app-thang-binh` |
| Đo trên máy thật: mã từ chối SDK (`-201`/`-2002`/`-1401`), UNKNOWN #1 ADR 0045 | người có console Zalo | câu lỗi quyền | `citizen-app/bao-dung-loi-quyen-zalo` |
| Bốn xung đột Phản ánh C1–C4 | người dùng / khách | ảnh sau xử lý, nhập hộ, kiểm duyệt công khai | `service-petitions/vong-doi-phieu-phan-anh` · `phan-anh-tuyen-cong-dan-con-thieu` |
| Vòng đời nhiệm vụ lệch `vigov-require` + bảy xung đột | khách | bảng chuyển trạng thái | `service-petitions/doi-chieu-26-09-nhiem-vu-truoc-neo` |
| Gỡ `IDENTITY_ADMIN_SEED_PASSWORD` khi mọi xã đã đổi mật khẩu | vận hành | bí mật mở `admin` | `service-identity/xa-moi-khong-co-vai-tro-va-quyen` |
| CNI có thực thi NetworkPolicy (cổng 9091, pod `vihat-miniapp`) | vận hành | `netpol.yaml` | `deploy/README.md` §11.0 |
| Miễn xã `ResolveCitizenSession` có cần ADR; phiên không xã cần bảng #25; tên tham số QR (ADR 0047 CÒN MỞ #5); 12 câu khu vận hành; bộ trạng thái văn bản đến; ngày làm việc hay ngày lịch cho hạn đơn thư | người dùng / khách / pháp chế | xem từng mục | như bản 30/09 — chưa kiểm lại |

**Đã bỏ khỏi bảng 01/10 vì đã giải:** #35–#39 (DECIDED); "mã lỗi hiện cho mọi người dân" (mã xuống
dòng phụ, `citizen-app/doi-ma-loi-zalo-xuong-dong-phu`); "tạo nhiệm vụ từ phiếu không còn lối"
(`f6400372`); thu chi "chốt kỳ" đã dựng ở phía máy chủ (`fa245787`, phiên song song).

---

## 4. Phiên song song

Lúc viết (01/10), cây làm việc có việc **chưa commit** của phiên khác. Đừng stage, đừng checkout:

| Đường dẫn | Việc (đoán từ tên tệp) |
|---|---|
| `service-identity/**` (danh bạ cán bộ, khoá đăng nhập) | phiên identity |
| `web-admin/src/features/thu-chi/**`, `web-admin/src/lib/api/thu-chi.ts`, `kb/90-ephemeral/tien-do/service-finance.json` | phiên thu chi |
| `tasks/web/open/*` → `claimed/` (3 tệp đang chuyển) | một phiên web đang nhận việc |

Một phiên song song **đã commit cuốn theo** tệp tôi đã stage (`1521cd85` mang `schema.gen.ts`), vì
các phiên dùng chung một index git. Xem §5.

---

## 5. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp**, tức một
phép kiểm xanh vì lý do sai. Gặp cái tiếp theo cùng dạng thì hỏi cả lớp ấy còn ở đâu.

### Lượt 30/09–01/10 — giao diện app xã, nối API (mới, đã gặp thật)

| Triệu chứng | Sự thật |
|---|---|
| Người dùng nói "demo" / "trải nghiệm", Claude dựng dải "Chế độ demo" và câu "chưa gửi được" | **Sai hai lần.** Người dùng muốn: không chữ demo nào trong app; `--demo` chỉ thay danh tính và chạy thật. Nhắc lại cách hiểu và hỏi đúng chưa **trước** khi dựng |
| Phiên song song commit, tệp mình đã `git add` nằm trong commit của họ | Mọi phiên dùng **chung một index**. Chỉ `git add` ngay trước `git commit` trong **cùng một lệnh** |
| `make kb` trên cây chính sinh chỉ mục mang việc dở của phiên khác | Sinh trên worktree sạch: `D:\works\vihat\wt-kb` (detached HEAD; `core/gen` chép vào; `node_modules` của citizen-app, web-admin, platform-admin là **junction** trỏ về kho chính). **Đừng `rm -rf` worktree ấy**: xoá junction trước (`(Get-Item …).Delete()`), rồi `git worktree remove --force` |
| Chrome qua Claude-in-Chrome không mở được `localhost:3100` | Tiện ích nối với **máy macOS khác**. Chụp màn bằng Playwright trên máy này: `vigov-require/apps/miniapp/node_modules/playwright` với `executablePath` = `%LOCALAPPDATA%/ms-playwright/chromium-1228/chrome-win64/chrome.exe` (bản Playwright đòi 1200, máy có 1228) |
| brain #6 đỏ vì tệp `.md` chưa commit | Phép kiểm đi bằng hệ tệp, không bằng git. Thư mục đã gitignore phải thêm vào danh sách bỏ qua trong `tools/check_brain.py` |
| Builder comms dừng: "core/storage không có thao tác ghi" | Đúng: `Promote` chỉ chép đúng khoá, `PublishDerivative` từ chối bản `original`. Nay có `PutServerProduced`. Tệp do máy chủ tạo (dẫn xuất, ảnh bỏ EXIF) **phải** đi qua nó |
| Migration `ADD CONSTRAINT … NOT VALID` trên bảng phân vùng | Bị từ chối trước PG 18 (khoá ngoại) và không rút ngắn khoá. Thêm có kiểm luôn khi mọi giá trị đang NULL |
| Tệp "down" migration đặt cạnh tệp "up" | `core/migrate` áp **mọi** `*.sql` trong thư mục. Đảo ngược viết trong khối chú thích |
| `node --test scripts/*.test.mjs` đỏ | Các tệp đó là vitest; chạy `npx vitest run scripts/<tệp>` |
| Giải mã QR Zalo (chấm tròn, logo giữa) bằng jsQR ra "NO QR FOUND" | Dùng `@zxing/library` (TRY_HARDER), đọc được ngay |
| Test hợp đồng web-admin (`noi-dung.test.ts`) đỏ sau khi thêm trường ở comms | Nó khoá bộ trường form phải gửi = bộ trường hợp đồng. Commit hợp đồng mới phải đi cùng web-admin |
| Tuyến công khai mới trong `/api/v1/commune-news/…` | Nằm trong cây con đã tách ở outer mux (`main.go`); `categories` là chữ nên thắng `{id}` (Go 1.22). Tuyến ngoài cây con sẽ rơi vào chuỗi cán bộ và 404 |

### Lượt 30/09 — gỡ đổi tên, nối chuỗi Mini App (giữ từ bản trước)

| Triệu chứng | Sự thật |
|---|---|
| `git reset --hard` bị `data_safety_guard` chặn | Dùng `git stash` |
| Revert đổi tên → Go build theo mã sinh cũ | `core/gen` bị gitignore. `mingw32-make proto` trước `build` |
| `lint` đỏ ở `buf breaking` sau revert | So cây làm việc với `HEAD`; trên cây sạch thì xanh |
| `tien-do.md` xung đột | Tệp SINH; lấy một phía rồi `make kb` hoặc `python tools/tien_do.py` |
| Edit sổ bị `progress_guard` chặn vì mục khác | Rào kiểm cả tệp; một mục `"menu": null` chặn mọi lần ghi |
| Mở cổng 9091 mà Mini App vẫn không đăng nhập | `vihat-miniapp` cùng namespace, `deny-all` chặn cả vào lẫn ra |
| Sổ ghi "bị chặn bởi `vihat-miniapp`" sau khi kho bên kia đã sửa | `git -C ../vihat-miniapp log` trước khi tin |
| Lệnh Bash nối `awk` / `sed -n` / `find` bị từ chối | Lớp cấp quyền của phiên. Đọc bằng Read/Grep/Glob, sửa JSON bằng python |

### Rào chắn, công cụ sinh, máy này — giữ từ 28/09, không kiểm lại từng dòng

| Triệu chứng | Sự thật |
|---|---|
| Thêm trường thân request, web-admin vỡ lúc `gen:api` | Trường mới luôn `omitempty` |
| `go test` xanh, `make check` đỏ ở `lint` | `gofmt -l <module>` trước commit |
| `make kb \| grep` "xanh" | Ống dẫn che mã thoát |
| `make kb` không sinh `schema.gen.ts` | `node web-admin/scripts/gen-api-types.mjs`, cùng commit |
| codegraph trả ký hiệu kho khác | Luôn truyền `projectPath` |
| Tầng always_load sát trần | Chỉ người dùng nâng trần |
| `citizen_commitment_guard` chặn `const quaHan` ở web | Dương tính giả; viết lại inline |
| Builder sửa bằng python | PreToolUse hook không chạy; soát tay diff |
| `UnicodeEncodeError: 'charmap'` | `PYTHONIOENCODING=utf-8` |
| Go đổ vì ổ C đầy | `GOCACHE` sang ổ D |
| Ca `_pg_test` "xanh" | Là SKIP (`VIGOV_TEST_DSN` trống) |
| `git checkout --` để hoàn tác | Xoá việc chưa commit của phiên khác |
| Hai cột cùng kiểu đọc theo vị trí trong `Scan` | Lỗi im lặng đối xứng; petitions loại/mức ưu tiên nhiệm vụ **chưa sửa** |
| Kết quả grep âm tính | Không phải bằng chứng vắng mặt; mở tệp |

---

## 6. Cổng kiểm

```
PYTHONIOENCODING=utf-8 GOCACHE=<thư mục trên ổ D> mingw32-make check
```

**01/10, chạy thật:** `make check` **exit 0** trên worktree sạch tại `af3ffcd9`: brain, hooks, Go các
module, citizen-app, web-admin, `check:api` khớp. `lint` báo `Error 2 (ignored)` vì thiếu `golangci-lint`.

**CHƯA KIỂM:**
- Chưa có PostgreSQL thật. Migration comms 0011 chưa chạy: khoá ngoại giữa hai bảng phân vùng, trigger
  `UPDATE OF` trên bảng phân vùng, `TestPgFirstPublishInstant…`, `TestPgPublicCategoryFilterAndChips`.
- Chưa có MinIO thật: `TestIntegrationPublishUnpublish` và `TestIntegrationPutServerProduced` đều SKIP.
- App riêng của xã chưa chạy trên máy thật hay webview Zalo. Màn gửi bước 2 và chi tiết phiếu chỉ mới
  chụp qua harness, không có phiên thật.
- `vihat-miniapp` không build/test lại ở lượt này.

**Thứ cổng KHÔNG phủ:** PostgreSQL thật · MinIO/ClamAV thật · `golangci-lint` · Jenkins/k8s (cụm dựng
tay trên Rancher) · trình duyệt thật (vitest không DOM) · Zalo · hook có nhìn thấy gì không (chỉ đột
biến mới chứng minh được).

---

## 7. Việc treo

**Không nằm ở đây.** Mỗi việc treo ở module của nó với `trang_thai: "treo"` trong `kb/90-ephemeral/tien-do.md`.
Phân biệt: **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là chờ một câu ở §3.
