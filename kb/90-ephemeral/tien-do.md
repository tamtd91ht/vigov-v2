---
id: tien-do
tier: T5
source: GENERATED
owner: architecture
derived_from_commit: add3bc2
expires: 2026-12-19
owns_facts:
  - "tiến độ từng module: mục nào đã làm, chưa làm, đang treo, và nợ câu hỏi nào"
---

# Tiến độ theo module

**SINH RA — đừng sửa tệp này.** Sửa `kb/90-ephemeral/tien-do/<module>.json` rồi chạy
`make kb`. `hooks/progress_guard.py` chặn mọi lần ghi thẳng vào đây.

Tệp này trả lời đúng một câu: **module nào còn nợ gì.** Vì sao làm thế → `kb/10-decisions/`.
Đã làm gì → `git log`. Cạm bẫy và quyết định đã chốt → `kb/90-ephemeral/ban-giao-phien.md`.

Cập nhật gần nhất **2026-09-20** · hết hạn **2026-12-19**. Hạn đo lần cuối có người cập nhật
một module, không phải lần cuối sinh tệp — quá hạn nghĩa là 90 ngày không ai chạm tới,
tức tin `git log` chứ đừng tin tệp này.

| | |
|---|---|
| ĐANG LÀM | 3 |
| chưa làm | 36 |
| treo | 11 |
| xong | 24 |

## Nợ khách chốt — chặn thật, không tự quyết được

Nội dung câu hỏi ở `kb/00-foundation/open-questions.json`. Đây chỉ là ai đang chờ ai.

| Câu | Trạng thái | Đang chặn |
|---|---|---|
| #1 | OPEN | _chung/sap-nhap-chia-tach-xa |
| #4 | OPEN | service-identity/store-crosstenant · service-reporting/khung-rong-cho-khach-chot |
| #9 | OPEN | service-identity/cap-tai-khoan-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #10 | OPEN | service-identity/tuyen-ghi-danh-ba-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #11 | OPEN | proto/staff-thieu-ho-ten · service-identity/che-so-di-dong-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #12 | OPEN | service-identity/che-so-di-dong-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #13 | OPEN | service-identity/tuyen-ghi-danh-ba-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #14 | OPEN | service-identity/tuyen-ghi-danh-ba-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #15 | OPEN | service-identity/ma-can-bo-va-dien-thoai · web-admin/moi-tuyen-ghi-cho-can-bo |
| #16 | OPEN | service-identity/ma-can-bo-va-dien-thoai · web-admin/moi-tuyen-ghi-cho-can-bo |
| #17 | OPEN | service-identity/cap-tai-khoan-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #18 | OPEN | service-identity/cap-tai-khoan-can-bo · web-admin/moi-tuyen-ghi-cho-can-bo |
| #19 | OPEN | web-admin/man-xac-thuc-loi-khai-cu-tru |
| #20 | OPEN | web-admin/man-xac-thuc-loi-khai-cu-tru |
| #21 | OPEN | service-documents/loai-van-ban-tuyen-ghi · service-finance/hang-muc-tuyen-ghi · service-petitions/danh-muc-nhiem-vu-tuyen-ghi |

## `_chung`

Cập nhật 2026-09-20 · 12 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `chay-migration-that` — Toàn bộ tầng SQL của kho chưa từng chạy một lần nào | chưa làm | — | — | dựng PostgreSQL cục bộ, đặt VIGOV_TEST_DSN, `go test -count=1 ./...` từng module. Phải tự mắt thấy: mọi migration áp được trên CSDL trống · trigger append-only bắn khi gõ THẲNG tên partition · đủ mảnh PARTITION BY HASH · test canh co_tai_khoan chạy thật chứ không skip · sáu bảng kênh công dân áp cùng mười bảy tệp cũ |
| `sap-nhap-chia-tach-xa` — Quy trình sáp nhập / chia tách / đổi tên đơn vị hành chính | chưa làm | — | #1 | chưa làm được gì cho tới khi khách chốt — luật 1 stop condition #3 |
| `ma-hoa-khi-luu` — Ba câu về mã hoá khi lưu chưa có đáp | chưa làm | — | — | hỏi chủ dự án: khoá nằm ở biến môi trường hay nguồn khác · khoá có phải một danh sách xoay vòng được không · có mã hoá ho_ten không — mã hoá thì MẤT khả năng sắp xếp theo tên ở mọi màn hình |
| `jenkins-chay-that` — Mười Jenkinsfile chưa từng chạy trên Jenkins thật | chưa làm | — | — | cần hạ tầng — phạm vi của kho dừng ở Dockerfile + Jenkinsfile, cụm k8s do đội devops phụ trách |
| `ma-hoa-vung-xuyen-xa` — Chính sách mã hoá cho vùng xuyên xã | treo | ADR 0009 là envelope encryption THEO XÃ; dinh_danh_cong_dan không thuộc xã nào nên không có DEK nào bọc nó | — | người dùng đã chốt: ghi thành KHOẢNG HỞ CÓ TÊN, chưa thiết kế gì. Thứ đang bảo vệ nó là luật 3 + phân quyền CSDL |
| `revoke-audit-log` — REVOKE trên audit_log thuộc khâu cấp phát CSDL, không nằm trong tay mã nguồn | treo | — | — | câu SQL đúng giữ trong comment của service-*/migrations/0002. Khâu cấp phát không làm thì lớp quyền vẫn hở dù trigger vẫn đúng |
| `tieu-de-dac-ta-quyen` — Tiêu đề docs/ui-ux/14-cau-hinh.md §4.2 ghi 43 quyền / 11 nhóm, bảng ngay dưới có 33 khoá / 10 nhóm | treo | đếm lại trên chính tệp ấy 2026-09-18 | — | hỏi khách xác nhận TIÊU ĐỀ là chỗ sai. Migration khớp bảng. KHÔNG bịa mười khoá và cũng đừng đi tìm chúng |
| `pii-trong-lich-su-git` — Dữ liệu cá nhân thật vẫn còn trong lịch sử git | treo | cây làm việc đã dọn | — | gỡ khỏi lịch sử là viết lại lịch sử trên main — cần quyết định của chủ dự án, không phải việc agent tự làm |
| `ra-lop-nhan-dien-theo-ten-thu-muc` — Rà cả lớp lỗi "cơ chế nhận diện mã theo tên thư mục" | treo | SÁU ca, không phải năm — bộ đếm cũ dừng ở năm và một bộ đếm thiếu làm người ta tưởng đã rà hết. Ca 5: stop_verify_guard.CODE_DIR thiếu /tools/. Ca 6: tenant_scope_guard.DB_CALL đòi `.Query(` không hậu tố trong khi cả 20 lời gọi CSDL của kho đều là bản *Context — rào chắn mà luật 1 nêu tên làm cơ chế BLOCK không khớp một call site nào, commit babf9bb. Ca 7 cùng lớp nhưng khác trục: grpcx.UnaryServerInterceptor gỡ khỏi chuỗi máy chủ mà 0 ca đỏ, commit e3ed99b — kiểm 2026-09-20 | — | chưa soát CI vì chưa có .github/. Ngày dựng CI thì đây là thứ phải soát lại đầu tiên. Hai ca mới nhất dạy thêm một trục: ca 6 là rào KHÔNG KHỚP GÌ, ca 7 là ca test khẳng định về máy chủ nhưng chỉ chạm client — cả hai đều xanh. Phép kiểm duy nhất đáng tin vẫn là đột biến: gỡ thứ nó đáng lẽ phải chặn rồi xem có đỏ không |
| `ra-hook-hoi-dung-cau` — Rà 16 hook theo trục "thứ nó đang đọc có trả lời đúng câu nó đang hỏi không" | treo | doc_guard đã lộ ra trục này: luật frontmatter là tính chất của TỆP nhưng nó chấm LẦN SỬA | — | hỏng được cả hai chiều: secret_scan mà đi chấm cả tệp thì tố cáo một lần sửa vô can vì bí mật có sẵn từ trước |
| `tang-tien-do-theo-module` — Tầng ghi tiến độ cho agent: ghi theo module, đọc một tệp, có rào chặn | xong | .claude/hooks/progress_guard.py + .claude/commands/progress.md + .claude/agents/progress-reviewer.md + tools/tien_do.py; `python tools/test_hooks.py` 133/133 (6 ca mới), `python tools/check_brain.py` 7/7; nhánh Stop đã thử đột biến: thiếu ghi -> rc=2, có ghi -> rc=0 | — | thứ hook KHÔNG kiểm được: agent nào ghi module nào (payload không mang danh tính agent), và bằng chứng có thật hay không — đó là việc của progress-reviewer, và nó BÁO chứ không sửa |
| `xac-thuc-service-service` — Xác thực giữa các service — món nợ ADR 0012 quyết định 3 để lại | xong | ADR 0025 (kb/10-decisions/0025-xac-thuc-giua-cac-service.md) chốt 2026-09-20; ADR 0012 quyết định 3 đã ghi ĐÃ BỊ THAY THẾ ở :19 và :175, giữ nguyên văn lập luận cũ; mã ở core/grpcx/caller_auth.go + 2 tệp test — kiểm 2026-09-20 | — | CHỦ SỞ HỮU CỦA SỰ THẬT NÀY NAY LÀ `core/xac-thuc-ben-goi-grpc`, kèm ba giới hạn đã biết của cơ chế khoá chung. Mục này giữ lại làm VẾT (đã từng là món nợ, ai trả, bằng gì) — đừng ghi tiếp ở đây, hai chỗ cùng kể một chuyện là hai chỗ sẽ lệch. Dòng `tiep_theo` cũ ở đây từng đọc là 'KHÔNG dựng cơ chế bí mật chia sẻ tạm', tức chỉ thị NGƯỢC HẲN quyết định người dùng vừa chốt |

## `citizen-app`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `nop-zalo-duyet` — Nộp Mini App cho Zalo duyệt | chưa làm | — | — | hai thứ còn thiếu đều KHÔNG phải mã: (1) logo/icon, ảnh chụp màn hình, mô tả store — Zalo bắt buộc, kho chưa có tệp ảnh nào; (2) citizen-app/app-config.json phải đối chiếu Developer Console — tên khoá viết từ NGUỒN THỨ CẤP, thư mục build đang là dist/ (mặc định Vite) trong khi zmp-cli thường dùng www/. Sai khoá là hồ sơ bị trả về |
| `ba-cau-hoi-zalo` — Ba giả định về Zalo chưa ai xác nhận | chưa làm | — | — | hỏi CÙNG LÚC lúc nộp: ràng buộc 1 Mini App ↔ 1 OA (cả ADR 0018 đứng trên nguồn thứ cấp) · app duyệt dạng hồ sơ doanh nghiệp sau này gắn dịch vụ công có phải xác thực lại không · tham số deep link có tới app khi app đang chạy nền không |
| `giai-doan-2-man-hinh-cong-dan` — Màn hình kênh công dân giai đoạn 2 | chưa làm | citizen-app/src/features/ chưa có tính năng kênh công dân nào — kiểm 2026-09-20 | — | chờ kho đọc + route ở service-identity (xem tien-do/service-identity.json, mục kho-doc-kenh-cong-dan) |
| `giai-doan-1-gioi-thieu` — Mini App giai đoạn 1 — giới thiệu ViHAT Software | xong | citizen-app/src/features/{company-intro,diagnostics,kham-pha,tinh-nang} + phase1-collects-nothing.test.ts, bundle-for-zalo.test.ts, accessibility.test.ts — kiểm 2026-09-20 | — | thứ DUY NHẤT trong kho sẵn sàng giao ra ngoài |

## `core`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `kho-phien-cong-dan-dem` — Đệm TTL ngắn cho đường tra cứu phiên công dân (ADR 0022 đòi) | chưa làm | core/httpx/citizen.go:72-74 khai đây là đường nóng và đòi đệm có TTL ngắn, vô hiệu khi thu hồi; service-identity/internal/store/phien_cong_dan.go ghi rõ đã HOÃN và vì sao — kiểm 2026-09-20 | — | Hoãn có lý do, không phải bỏ quên: identity chạy nhiều bản sao và ADR 0010 chốt chỉ có PostgreSQL, nên không có kênh nào để một lần thu hồi ở bản sao A với tới bản sao B. Cửa sổ lệch sẽ đúng bằng TTL, và ca hỏng là nút 'đăng xuất màn hình này' ở quầy một cửa. Đo trước, rồi mới đệm, kèm kênh vô hiệu hoá thật |
| `backfill-theo-xa` — Backfill dữ liệu theo từng xã | treo | core/migrate chỉ lo DDL — kiểm 2026-09-20 | — | luật 7 bất biến 5 (migration chạy per-commune, resumable, ghi tiến độ) mới đạt một nửa. Ngưỡng cần cơ chế thật là khi thời gian giữ khoá thành đáng kể — ADR 0013, mục Giới hạn |
| `xac-thuc-ben-goi-grpc` — Xác thực bên gọi trên cổng gRPC — một cặp header, giá trị từ secret k8s | xong | core/grpcx/caller_auth.go + caller_auth_test.go + caller_auth_exempt_test.go; MetadataCallerKey ở core/grpcx/grpcx.go:110; GRPCCallerKey ở core/config/config.go:128. Phép kiểm đáng tin là ĐỘT BIẾN chứ không phải `make check` xanh: gỡ UnaryServerCallerAuth -> 2 ca đỏ. Mốc e3ed99b — kiểm 2026-09-20. ADR 0025 | — | Ba giới hạn ĐÃ BIẾT, không phải thiếu sót: khoá chung không nói service nào gọi nên vết kiểm không quy được trách nhiệm; ai trong cụm cầm khoá đều gọi được mọi thứ, lớp mạng là thứ chặn bán kính; xoay khoá phải đổi đồng loạt. Đường ra cho cả ba là mTLS/mesh, và phải SỬA ADR 0025 chứ không lặng lẽ thêm header thứ hai. Bằng chứng cũ của mục này từng viện `make check` rc=0 — e3ed99b đo được lượt xanh ấy xanh vì lý do sai, nên bằng chứng nay là phép đột biến |
| `staffauth-va-identityclient` — core/staffauth + core/identityclient — một cài đặt xác thực cán bộ dùng chung cho bốn service khung | xong | core/staffauth/staffauth.go 306 dòng + test, core/identityclient/identityclient.go 226 dòng + test; bốn service-{comms,documents,finance,petitions}/cmd/server/main.go đều import và mắc vào chuỗi. Commit ebc3b0b — kiểm 2026-09-20 | — | Đây là thứ gỡ 401 cho sáu tuyến (19c6008). Vì một cài đặt phục vụ bốn service nên mọi thay đổi ở đây là thay đổi cho cả bốn: đổi hành vi thì phải chạy main_test.go của cả bốn, không phải của service đang sửa |

## `deploy`

Cập nhật 2026-09-20 · 3 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `thieu-manifest-tam-don-vi` — Tám đơn vị triển khai chưa có manifest nào | chưa làm | deploy/base/ chỉ có identity, mang, platform, web-admin — kiểm 2026-09-20 | — | Thiếu: service-comms · service-documents · service-dossiers · service-finance · service-petitions · service-reporting · citizen-app · platform-admin. Bốn service khung nay đã phục vụ được thật (commit 19c6008) nên khoảng cách giữa 'chạy được cục bộ' và 'triển khai được' đang rộng ra, không hẹp lại |
| `cum-chua-chay-that` — Toàn bộ tầng này chưa từng chạm một cụm thật | chưa làm | — | — | Đi cùng `_chung/jenkins-chay-that` nhưng KHÁC nó: mục kia nói mười Jenkinsfile chưa chạy, mục này nói manifest chưa ai `kubectl apply`. Một tệp YAML chưa bao giờ được máy chủ đọc là một tệp chưa ai biết có đúng không — phạm vi kho dừng ở đây, cụm do đội devops phụ trách |
| `manifest-k8s-ba-don-vi` — Manifest Kubernetes cho platform, identity, web-admin, và lớp mạng | xong | deploy/base/{identity,platform,web-admin,mang}/, deploy/cluster/{namespace,rbac-jenkins}.yaml, deploy/overlays/{prod,staging}/{ingress,kustomization}.yaml, deploy/Jenkinsfile. Commit 0e54c51 — 32 tệp, 1524 dòng thêm; cũng vá mốc dựng lại của 9 pipeline — kiểm 2026-09-20 | — | Bố cục có chủ ý: `base/` KHÔNG mang namespace và KHÔNG mang thẻ ảnh; namespace, cấu hình môi trường và thẻ ảnh chỉ nằm ở `overlays/<mt>/`. `deploy/Jenkinsfile` là NƠI DUY NHẤT gọi kubectl — chín job đóng ảnh dừng ở Harbor, không chạm cụm. Thành quả này trước 2026-09-20 không chỗ nào trong sổ ghi là đã có, tức nó là loại kết quả biến mất giữa hai phiên |

## `platform-admin`

Cập nhật 2026-09-20 · 2 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `ts-chua-tung-duoc-kiem` — Mã TypeScript của platform-admin chưa từng đi qua cổng kiểm | chưa làm | `make check` in `BỎ QUA platform-admin/ — chưa có node_modules. Mã TypeScript KHÔNG được kiểm.` rồi tiếp tục và vẫn trả rc=0 — đọc trong log lượt chạy đầy đủ 2026-09-20 | — | Đây là LỖ HỔNG CỦA CỔNG, không phải của app: cổng xanh trong khi một đơn vị triển khai không được kiểm dòng nào. Cùng lớp với `golangci-lint` vắng mặt mà mục `lint` nuốt lỗi bằng tiền tố `-`. Gỡ bằng `(cd platform-admin && npm install)`, nhưng câu đáng hỏi trước là: một đơn vị chưa có mã thì nên BỎ QUA hay nên làm đỏ cổng |
| `chua-dung-man-hinh-nao` — Console quản trị nền tảng chưa có màn hình nào | chưa làm | platform-admin/src/{app,components,features}/ đều RỖNG; chỉ có src/lib/api.ts. Commit cuối chạm vào module: 6d1d333 (lượt đưa mỗi đơn vị lên cấp một) — kiểm 2026-09-20 | — | ĐỌC ADR 0003 TRƯỚC khi dựng màn đầu tiên: platform chỉ giữ SIÊU DỮ LIỆU, nên console này không được có đường nào đọc dữ liệu nghiệp vụ của một xã. Đó cũng là stop condition #5 của luật 1 — quản trị viên nhà cung cấp chạm dữ liệu nghiệp vụ của xã là câu phải hỏi người dùng, không tự quyết |

## `proto`

Cập nhật 2026-09-20 · 2 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `staff-thieu-ho-ten` — Message Staff không có trường họ tên nên BatchGetStaff chưa phục vụ được mục đích nó tự khai | treo | — | #11 | thêm trường là SỬA HỢP ĐỒNG — cùng lúc phải trả lời câu che/không che số di động cán bộ. Thêm một chỗ chặn chưa ai nói: trong 33 khoá quyền đã seed KHÔNG có khoá nào nghĩa là 'xem chi tiết đầy đủ cán bộ', nên chọn một khoá cho tuyến ấy là thiết kế hộ khách mô hình phân quyền |
| `resolve-staff-principal` — RPC biến một chứng thực của cán bộ thành một principal, cho service không phải identity | xong | proto/vigov/identity/v1/identity.proto — ResolveStaffPrincipal + ba message; buf lint, buf breaking, buf generate đều sạch; mục đầu tiên có thật trong kb/30-indexes/transaction-boundaries.json. Commit ae65ac7 — kiểm 2026-09-20 | — | HAI THỨ HỢP ĐỒNG NÀY CỐ Ý KHÔNG MANG, và cả hai sẽ bị đòi thêm: (1) `ho_ten`/`chuc_vu` — không cổng gác nào đọc tên, và câu mở #11 chặn; (2) `ma` nghiệp vụ — vết kiểm của bốn service sẽ cần 'ai' theo luật 6 bất biến 2, và lúc ấy phải chọn: thêm `ma` vào StaffPrincipal (dữ liệu nhận dạng cán bộ đi qua biên), hay để bốn service ghi id nội bộ — tức HAI vết kiểm gọi một người bằng hai tên. Chưa quyết, liên quan ADR 0025 mục còn mở #4. MỘT SỰ THẬT ĐÃ ĐO: `[debug_redact = true]` VÔ TÁC DỤNG trong protobuf-go v1.36.12 — `%+v` in token nguyên văn, nên không gì trong mã sinh bảo vệ được; dấu ấy đã gỡ thay vì để lại |

## `service-comms`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `sql-chua-chay` — Chạy 0003_danh_muc_loai_tai_nguyen_ban_do.sql trên PostgreSQL thật | chưa làm | 7 ca service-comms/internal/store/loai_tai_nguyen_ban_do_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat — đọc information_schema để đối chiếu danh sách cột trong mã với lược đồ thật. Đó là phép kiểm duy nhất bắt được một migration đổi tên cột dưới chân store; ba service kia đang thêm bản tương ứng |
| `dac-ta-ban-do-tu-mau-thuan` — Đặc tả bản đồ lệch với chính nó — số nhóm và bộ mã | treo | docs/ui-ux/10-ban-do-kinh-te-so.md:37 nói 11 nhóm, :53 nói 8 nhóm; một chỗ dùng mã `enterprise`, chỗ kia `doanh-nghiep`. Ghi sẵn trong service-comms/migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql — kiểm 2026-09-20 | — | Bảng ship RỖNG có chủ đích cho tới khi đặc tả tự khớp. KHÔNG chọn hộ một trong hai bản — hỏi người viết đặc tả |
| `loai-tai-nguyen-ban-do-tuyen-doc` — GET /api/v1/map-asset-types — tuyến ĐỌC danh mục loại tài nguyên bản đồ | xong | service-comms/internal/http/routes.go — tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store service-comms/internal/store/loai_tai_nguyen_ban_do.go. `go test -count=1 ./...` xanh — kiểm 2026-09-20 | — | Tên tài nguyên URL ĐÃ chốt và ô trong ubiquitous-language.md ĐÃ điền (commit 9388eaf) — `grep -c "(chưa chốt)"` trả 0. Dòng này từng giao việc ấy cho knowledge-keeper: việc đã xong trước khi ai đọc tới, tức sổ đang đặt hàng một việc thừa. KHÔNG ghi số ca test ở đây: bản cũ chốt một con số, rồi 19c6008 thêm main_test.go cho cả bốn service và mọi con số thành thấp hơn thực tế — sai theo kiểu trông y hệt số đúng. |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | xong | cmd/server/main.go dựng chuỗi rìa thật qua `dungBien`, gắn core/staffauth.Middleware; cmd/server/main_test.go có ca principal của xã A gọi ở host xã A TỚI ĐƯỢC handler và nhận 200 — lần đầu một tuyến của service này phục vụ được một yêu cầu. Bốn đột biến đều đỏ, gồm biến thể đệm hẹp theo từng middleware. `make check` rc=0 — kiểm 2026-09-20, commit 19c6008 | — | MỘT CHỖ ĐỘT BIẾN KHÔNG BẮT ĐƯỢC, ghi ra thay vì im: main_test.go chứng minh `dungBien` dựng đúng chuỗi, KHÔNG chứng minh `run()` có gọi `dungBien` — xoá lời gọi ấy rồi phục vụ `mux` trần thì không gì đỏ. Bịt được cần một phép kiểm khởi động tiến trình thật, một lớp test khác |

## `service-documents`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `loai-van-ban-tuyen-ghi` — Tuyến GHI danh mục loại văn bản (thêm/sửa/tắt) | chưa làm | không có tuyến POST/PUT/PATCH/DELETE nào trong service-documents/internal/http/routes.go — kiểm 2026-09-20 | #21 | KHÔNG viết trước khi khách chốt: xã sửa được DANH SÁCH MÃ hay chỉ NHÃN và THỨ TỰ. Một tuyến ghi viết trước là quyết hộ khách |
| `sql-chua-chay` — Chạy 0003_danh_muc_loai_van_ban.sql trên PostgreSQL thật | chưa làm | 5 ca service-documents/internal/store/loai_van_ban_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN, gói vẫn in `ok` — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat — đọc information_schema để đối chiếu danh sách cột trong mã với lược đồ thật, gồm cả tenant_id/deleted_at/thu_tu vốn không nằm trong SELECT. Nó TỒN TẠI nhưng CHƯA CHẠY: driver giả không bắt được tên cột sai, và ca này chỉ bắt được từ lần chạy đầu tiên có DSN |
| `loai-van-ban-tuyen-doc` — GET /api/v1/document-types — tuyến ĐỌC danh mục loại văn bản | xong | service-documents/internal/http/routes.go — tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store service-documents/internal/store/loai_van_ban.go. `go test -count=1 ./...` xanh — kiểm 2026-09-20 | — | Tên tài nguyên URL ĐÃ chốt và ô trong ubiquitous-language.md ĐÃ điền (commit 9388eaf). Dòng này từng giao việc ấy cho knowledge-keeper, và việc đã xong trước khi ai đọc tới. KHÔNG ghi số ca test ở đây: bản cũ chốt một con số, rồi 19c6008 thêm main_test.go cho cả bốn service và mọi con số thành thấp hơn thực tế — sai theo kiểu trông y hệt số đúng. |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | xong | BA BƯỚC ĐÃ XONG CẢ BA: hợp đồng ResolveStaffPrincipal (ae65ac7) → core/staffauth + core/identityclient (ebc3b0b) → server gRPC của identity (5a56a81) → đấu rìa (19c6008). cmd/server/main_test.go có ca principal của xã A gọi ở host xã A nhận 200. `make check` rc=0 — kiểm 2026-09-20 | — | HAI ĐIỀU PHẢI GIỮ, cả hai đều là thứ người sau sẽ 'tối ưu': (1) tập khoá quyền chỉ sống trong ĐÚNG MỘT yêu cầu, nằm trong context và không đâu khác — sống lâu hơn là thành vai trò nhúng trong token, phá luật 5 bất biến 4; có hai ca test bắt cả biến thể đệm hẹp. (2) mọi lỗi vận chuyển là 503, KHÔNG BAO GIỜ là 'không có principal' — dịch sai là bảo mọi cán bộ đăng nhập lại qua chính service đang sập. Khoảng hở còn lại: main_test.go chứng minh `dungBien` dựng đúng chuỗi, không chứng minh `run()` gọi nó |

## `service-dossiers`

Cập nhật 2026-09-20 · 1 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `khung-rong-chua-khoi-cong` — Hồ sơ một cửa — mới có khung service, chưa một dòng nghiệp vụ | chưa làm | service-dossiers/internal/ chỉ có doc.go ở app, domain, event, store và một routes.go; migrations/ chỉ có 0001_init.sql + 0002_audit_log_append_only.sql. Commit cuối chạm vào phần mã: c199eec (lượt tách module Go) — kiểm 2026-09-20 | — | Chưa khởi công là ĐÚNG THỨ TỰ, không phải bỏ quên: hồ sơ một cửa là loại bản ghi lưu trữ nặng nhất kho, và nó cần trước hai thứ chưa có — danh mục thủ tục, và lịch làm việc theo xã để đếm hạn (ADR 0007). Dựng trước hai thứ ấy là dựng một quy trình đếm hạn sai mà không ai thấy cho tới lúc báo cáo lên trên |

## `service-finance`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `hang-muc-tuyen-ghi` — Tuyến GHI danh mục hạng mục kế hoạch vốn | chưa làm | không có tuyến POST/PUT/PATCH/DELETE nào trong service-finance/internal/http/routes.go — kiểm 2026-09-20 | #21 | KHÔNG viết trước khi khách chốt: xã sửa được DANH SÁCH MÃ hay chỉ NHÃN và THỨ TỰ |
| `sql-chua-chay` — Chạy 0003_danh_muc_hang_muc_ke_hoach_von.sql trên PostgreSQL thật | chưa làm | 6 ca service-finance/internal/store/hang_muc_ke_hoach_von_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN, gói vẫn in `ok` — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat (đọc information_schema) nhưng nó CHƯA CHẠY. Một lượt đối chiếu TĨNH với DDL của migration cho kết quả thiếu 0 cột — đó là đọc hai tệp trong kho, KHÔNG phải kiểm lược đồ đã triển khai. Driver giả cũng không kiểm được định tuyến phân mảnh, UNIQUE một mặc định, hay trigger ba tầng |
| `hang-muc-ke-hoach-von-tuyen-doc` — GET /api/v1/capital-plan-categories — tuyến ĐỌC danh mục hạng mục kế hoạch vốn | xong | service-finance/internal/http/routes.go — tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store service-finance/internal/store/hang_muc_ke_hoach_von.go. `go test -count=1 ./...` xanh — kiểm 2026-09-20 | — | Tên tài nguyên URL ĐÃ chốt và ô trong ubiquitous-language.md ĐÃ điền (commit 9388eaf). Dòng này từng giao việc ấy cho knowledge-keeper, và việc đã xong trước khi ai đọc tới. KHÔNG ghi số ca test ở đây: bản cũ chốt một con số, rồi 19c6008 thêm main_test.go cho cả bốn service và mọi con số thành thấp hơn thực tế — sai theo kiểu trông y hệt số đúng. |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | xong | cmd/server/main.go dựng chuỗi rìa thật qua `dungBien`, gắn core/staffauth.Middleware; cmd/server/main_test.go có ca principal của xã A gọi ở host xã A nhận 200, và ca identity chết thì trả 503 CHỨ KHÔNG 401. `make check` rc=0 — kiểm 2026-09-20, commit 19c6008 | — | Cùng khoảng hở với ba service kia: main_test.go chứng minh `dungBien` dựng đúng chuỗi, không chứng minh `run()` gọi nó — xem service-comms.json |

## `service-identity`

Cập nhật 2026-09-20 · 11 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `store-crosstenant` — Gói đọc chéo xã — nơi duy nhất được phép đọc qua ranh giới xã | ĐANG LÀM | service-identity/internal/store/crosstenant/{dinh_danh_cong_dan.go,doc.go} + 2 tệp test, đã vào git ở commit 274b73a — kiểm 2026-09-20. ban-giao-phien.md §2.3 ghi "chưa tồn tại" là ĐÃ LỖI THỜI. Dòng này từng ghi "CHƯA COMMIT", tức dạy phiên sau rằng vùng ấy chưa vào git và có thể viết đè | #4 | migration nêu tên ba truy vấn đọc chéo; mới có dinh_danh_cong_dan. Mỗi truy vấn mang `// @cross-tenant: <lý do>` (luật 1 cấm #6) — đó là cách duy nhất khiến đọc chéo thành danh sách ĐẾM ĐƯỢC |
| `kho-doc-kenh-cong-dan` — Kho đọc cho sáu bảng kênh công dân — giai đoạn 2 mới xong một phần | ĐANG LÀM | hợp đồng + rìa core/httpx/citizen.go + sáu bảng migration đã có; store/crosstenant/dinh_danh_cong_dan.go đã có — kiểm 2026-09-20 | — | chưa có route nào. Kho đọc nằm ở service-identity/internal/store/ — vùng khác với vùng đã viết migration, đó là đường nối phải bắc |
| `sql-chua-chay` — Chạy migration 0005 + mười bộ pg_test của identity trên PostgreSQL thật | chưa làm | service-identity/migrations/0005_don_vi_dan_cu_va_danh_muc.sql 501 dòng; mười tệp *_pg_test.go trong internal/store/ (kể cả crosstenant/dinh_danh_cong_dan_pg_test.go) đều phụ thuộc VIGOV_TEST_DSN, thiếu biến thì SKIP mà gói vẫn in `ok` — kiểm 2026-09-20 | — | Bốn service khung đều đã có mục này, identity thì KHÔNG — tức sổ đang im lặng về việc chưa ai làm, hướng nguy hiểm hơn hẳn nói sai. Đi cùng _chung/chay-migration-that: cùng một cái thiếu, nhưng đây là phần của identity và nó nặng nhất vì 0005 là tệp dài nhất kho |
| `cap-tai-khoan-can-bo` — Toàn bộ luồng cấp tài khoản cán bộ | chưa làm | — | #9 #17 #18 | mật khẩu đầu tiên của cán bộ mới · tự đặt lại mật khẩu · Ghi nhớ đăng nhập — cả ba chờ khách |
| `tuyen-ghi-danh-ba-can-bo` — Toàn bộ tuyến GHI của danh bạ cán bộ | chưa làm | — | #10 #13 #14 | khoá hay xoá cán bộ · chặn mất quản trị viên cuối cùng · tự thao tác lên chính mình |
| `ma-can-bo-va-dien-thoai` — Mã cán bộ do ai đặt, và dien_thoai/di_dong là một trường hay hai | chưa làm | — | #15 #16 | CHẶN SCHEMA nên đắt hơn các câu khác — hỏi trước khi viết migration tiếp |
| `che-so-di-dong-can-bo` — Che hay không che số di động cán bộ, và ai quyết việc công khai lên Mini App | chưa làm | — | #11 #12 | chặn cả cột hiển thị lẫn khoá quyền |
| `ban-giao-viec-khi-khoa-tai-khoan` — Bàn giao việc đang xử lý khi khoá tài khoản | treo | — | — | cố ý CHƯA ghi thành câu hỏi mở: chưa có bảng giao việc nào tồn tại để nói "việc đang giữ" nghĩa là gì. Hỏi khi dựng bảng nghiệp vụ đầu tiên có người phụ trách — câu trả lời nhiều khả năng là "tuỳ xã" |
| `grpc-server` — Server gRPC của identity | xong | service-identity/internal/grpc/server.go — ResolveStaffPrincipal + BatchGetStaff, cả hai interceptor trên chuỗi mà cmd/server/dungGRPCServer dựng; server_test.go 14 ca + cmd/server/main_test.go bufconn trên hàm dựng THẬT. Năm đột biến đều đỏ, gồm: gỡ interceptor khoá caller (bên gọi trần trụi nhận OK trên cả hai RPC) và đảo phép đối chiếu xã xuống sau lượt đọc sổ phiên. `make check` rc=0 — kiểm 2026-09-20, commit 5a56a81 | — | Đã mở khoá sáu tuyến 401 (commit 19c6008). HAI THỨ CÒN LẠI TRÊN CỔNG NÀY, cả hai đã ghi trong tài liệu gói: (1) không có interceptor bắt panic — grpc-go KHÔNG tự phục hồi panic của handler, nên một `tenant.MustFrom` lọt qua sẽ hạ cả tiến trình đang phục vụ 200+ xã; chỗ đúng của nó là core/grpcx. (2) `vanTay` nay có hai bản sao (internal/http và internal/grpc) và PHẢI giữ y hệt nhau — người vận hành đối chiếu cảnh báo tập trung với cảnh báo của XacThuc bằng chính chuỗi ấy, hai bản lệch nhau đẻ ra hai dấu vân tay cho một sid, đọc ra là hai phiên. ListCitizenCommunes vẫn `Unimplemented`, hai chỗ chặn độc lập — xem mục store-crosstenant |
| `danh-muc-dan-cu-tuyen-doc` — Ba tuyến ĐỌC danh mục dân cư: residential-units · residential-unit-types · task-blocs | xong | service-identity/internal/http/routes.go:157,167,177 khai ba kho; :251,253,255 panic ngay lúc dựng nếu thiếu kho — hỏng to tiếng chứ không phục vụ nửa vời. Commit cdc5b5f, tự khai là ba tuyến DUY NHẤT phục vụ được thật lúc ấy; fcea7bd sửa `is_active`->`active` trên dây — kiểm 2026-09-20 | — | Tuyến GHI chưa có, và chặn bởi cùng câu #21 như ba service kia — xem `danh-muc-nhiem-vu-tuyen-ghi` ở sổ service-petitions |
| `kho-phien-cong-dan` — Kho phiên công dân — đường nối rìa kênh công dân chờ từ đầu | xong | service-identity/internal/store/phien_cong_dan.go 482 dòng + phien_cong_dan_test.go + phien_cong_dan_pg_test.go. Commit 274b73a — kiểm 2026-09-20 | — | Mục này SỞ HỮU tệp ấy. `core/kho-phien-cong-dan-dem` chỉ TRỎ tới nó để nói về phần ĐỆM còn thiếu — hai việc khác nhau, đừng gộp |

## `service-petitions`

Cập nhật 2026-09-20 · 6 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `vong-doi-phieu-phan-anh` — Nghiệp vụ phản ánh — tiếp nhận, phân loại, phân công, nghiệm thu, đóng phiếu | chưa làm | — | — | CHỈ bắt đầu sau khi có lich_lam_viec + ngay_nghi_le theo xã (ADR 0007). Đếm hạn bằng giờ hành chính mà thiếu lịch của xã thì mọi con số hạn đều sai, và sai theo hướng không ai thấy cho tới lúc báo cáo lên trên |
| `lich-lam-viec-theo-xa` — Cấu hình lich_lam_viec + ngay_nghi_le theo từng xã | chưa làm | — | — | tiền đề của mọi phép đếm hạn — ADR 0007, luật 10 bất biến 4 |
| `danh-muc-nhiem-vu-tuyen-ghi` — Tuyến GHI hai danh mục nhiệm vụ | chưa làm | không có tuyến POST/PUT/PATCH/DELETE nào trong service-petitions/internal/http/routes.go — kiểm 2026-09-20 | #21 | KHÔNG viết trước khi khách chốt: xã sửa được DANH SÁCH MÃ hay chỉ NHÃN và THỨ TỰ |
| `sql-danh-muc-chua-chay` — Chạy 0003_danh_muc_nhiem_vu.sql trên PostgreSQL thật | chưa làm | 5 ca service-petitions/internal/store/danh_muc_nhiem_vu_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN, gói vẫn in `ok` — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat (đọc information_schema, kiểm cả thu_tu — với muc_uu_tien_nhiem_vu thì thu_tu CHÍNH LÀ thang, đổi tên nó là phá thang trong im lặng). Nó TỒN TẠI nhưng CHƯA CHẠY. Vẫn chưa gì kiểm được định tuyến phân mảnh, partial index, và sáu phép từ chối của trigger danh_muc_ba_tang |
| `danh-muc-nhiem-vu-tuyen-doc` — GET /api/v1/task-types + /api/v1/task-priorities — hai tuyến ĐỌC danh mục nhiệm vụ | xong | service-petitions/internal/http/routes.go — hai tuyến, mỗi tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store internal/store/{loai_nhiem_vu,muc_uu_tien_nhiem_vu}.go. `go test -count=1 ./...` xanh — kiểm 2026-09-20 | — | 0003_danh_muc_nhiem_vu.sql KHÔNG tạo bảng trạng thái nhiệm vụ nào — đã kiểm. Tên tài nguyên URL ĐÃ chốt và ô ĐÃ điền (commit 9388eaf); dòng cũ còn trỏ ubiquitous-language.md:162-163, mà hai dòng ấy nay là ResidentialUnitType và TaskBloc — hai danh mục của identity, tức trỏ SAI THỰC THỂ chứ không chỉ lệch dòng. KHÔNG ghi số ca test ở đây: bản cũ chốt một con số, rồi 19c6008 thêm main_test.go cho cả bốn service và mọi con số thành thấp hơn thực tế — sai theo kiểu trông y hệt số đúng. |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | xong | cmd/server/main.go dựng chuỗi rìa thật qua `dungBien`, gắn core/staffauth.Middleware; cmd/server/main_test.go có ca principal của xã A gọi ở host xã A nhận 200 trên cả hai tuyến. `make check` rc=0 — kiểm 2026-09-20, commit 19c6008 | — | Hai điều phải giữ và một khoảng hở còn lại — xem cùng mục ở service-documents.json, không chép lại ở đây |

## `service-platform`

Cập nhật 2026-09-20 · 5 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `dang-viet-ten-tinh-thanh` — 34 tên tỉnh/thành ở dạng viết chính thức | ĐANG LÀM | service-platform/migrations/0005_seed_tinh_thanh.sql đã seed, và tự khai ngay đầu tệp: DỮ LIỆU CHƯA ĐƯỢC ĐỐI CHIẾU VỚI VĂN BẢN GỐC — kiểm 2026-09-20 | — | đối chiếu với Nghị quyết 202/2025/QH15. Cột này in thẳng ra màn hình công dân nên DẠNG VIẾT là nội dung (`Đà Nẵng` hay `Thành phố Đà Nẵng`). Đã thử ba nguồn chính phủ, cả ba render phía client. Sửa một tên là THÊM một migration, không đụng tệp đã chạy |
| `loi-he-thong` — Bảng loi_he_thong | chưa làm | `grep -rl loi_he_thong --include=*.sql --include=*.go .` không có kết quả — kiểm 2026-09-20 | — | không bị chặn bởi câu hỏi nào |
| `danh-muc-tham-chieu` — Tám bảng danh mục tham chiếu | xong | ADR 0024 chuyển quyền sở hữu danh mục SANG DỊCH VỤ SỞ HỮU, không nằm ở platform: service-{comms,documents,finance,petitions}/migrations/0003_danh_muc_*.sql + service-identity/migrations/0005 — kiểm 2026-09-20. ban-giao-phien.md §2.5 ghi "platform: danh_muc chưa có" là ĐÃ LỖI THỜI | — | KHÔNG seed dòng nào, có chủ ý (commit fa10cf1) |
| `webhook-zalo-mini-app` — Endpoint nhận webhook Zalo Mini App — nhận được, chưa xử lý gì | xong | service-platform/internal/http/webhook_zalo.go 66 dòng + webhook_zalo_test.go 93 dòng, nối vào cmd/server/main.go. Commit 1a8ce11 — kiểm 2026-09-20 | — | LUÔN TRẢ 200 là chủ ý, không phải chưa làm xong: Zalo thử lại khi nhận mã lỗi, nên một lỗi phía ta sẽ thành bão yêu cầu. Nhưng hệ quả là sự kiện nhận được hiện KHÔNG đi tới đâu — chưa có hàng đợi, chưa có consumer, chưa ai quyết sự kiện nào đáng giữ. Đó là việc còn nợ của mục này |
| `ca-kiem-interceptor-xa-da-chet` — Ca kiểm interceptor xã trên cổng gRPC xanh vì lý do sai | xong | ĐÃ ĐO: gỡ grpcx.UnaryServerInterceptor khỏi chuỗi dungGRPCServer dựng -> 0 ca đỏ; gỡ UnaryServerCallerAuth -> 2 ca đỏ. Nguyên nhân: mọi ca đều quay số CÓ gắn interceptor client nên x-tenant-id luôn trên dây. Ca mới TestMayChuTuChoiRpcKhongMangXaDuDaCoKhoa quay số có khoá và KHÔNG có interceptor xã phía client; đột biến lại -> đúng nó đỏ, một mình. Commit e3ed99b — kiểm 2026-09-20 | — | PHẠM VI THẬT, đừng thổi lên: hai RPC của platform hiện không đọc xã từ context nên thiếu interceptor hôm nay KHÔNG rò gì. Thứ mất là lời KHAI rằng một RPC không được miễn thì phải mang xã (luật 1 cấm #1). Phát hiện ra nó là agent viết server identity khi so khuôn ca test của hai service — không cổng kiểm nào bắt được. Bài học cùng lớp với ba ca khác trong phiên: một rào chắn xanh không nói lên gì tới khi có người thử làm nó đỏ |

## `service-reporting`

Cập nhật 2026-09-20 · 1 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `khung-rong-cho-khach-chot` — Báo cáo tổng hợp — mới có khung service, và câu chặn nằm ở phía khách | chưa làm | service-reporting/internal/ chỉ có doc.go ở app, domain, event, store và một routes.go; migrations/ chỉ có 0001_init.sql + 0002_audit_log_append_only.sql. Commit cuối chạm vào phần mã: c199eec — kiểm 2026-09-20 | #4 | Đây là service sẽ ĐỌC NHIỀU XÃ, nên nó không được viết một dòng nào trước khi khách chốt câu #4 — mức chi tiết cấp huyện/tỉnh được xem. `drift_guard` đang cảnh báo #4 bị lặng lẽ quyết vì đã có 13 chỗ mang đường đọc chéo. Cùng câu ấy chặn `service-identity/store-crosstenant`. Khi khởi công: mọi truy vấn chéo xã phải mang `// @cross-tenant: <lý do>` (luật 1 cấm #6) và bản thân lượt đọc chéo phải để lại vết kiểm (luật 6 bất biến 7) |

## `tools`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `bo-sinh-doc-dau-entity` — Bộ sinh đọc dấu @entity để điền data-ownership.json | chưa làm | kb/30-indexes/data-ownership.json vẫn là chỗ giữ chỗ rỗng; 8 thực thể danh mục đã có dấu @entity trong migration nhưng không có gì đọc chúng — kiểm 2026-09-20 | — | CLAUDE.md bước 4 dạy phiên sau hỏi 'ai sở hữu X' thì mở data-ownership.json. Hôm nay họ mở ra và không thấy gì cho cả tám, rồi có thể kết luận chúng vô chủ. Bịt bằng bộ sinh trong tools/, KHÔNG bịt bằng cách hand-write một dòng sẽ mục nát ngay khi bộ sinh ra đời (luật 9) |
| `bo-sinh-tien-do` — tools/tien_do.py — sinh tệp đọc tien-do.md từ các tệp ghi theo module | xong | tools/tien_do.py:1; `python tools/tien_do.py` chạy thật, sinh ra kb/90-ephemeral/tien-do.md; đã nối vào mục `kb` của Makefile:125. KHÔNG ghi số module/mục ở đây — con số nào chép vào cũng sai trong vài ngày | — | hạn của tệp sinh tính từ `cap_nhat` MỚI NHẤT của các module, không phải ngày sinh — quá hạn nghĩa là 90 ngày không ai cập nhật tiến độ |
| `check-brain-bo-qua-tmp` — check_brain bất biến 6 đỏ vì một tệp .md nháp trong tmp/ (đã nằm trong .gitignore) | xong | tools/check_brain.py:277 thêm `tmp` vào danh sách loại trừ khi duyệt cây; `python tools/check_brain.py` 7/7 — số dòng kiểm lại 2026-09-20 (bản cũ ghi :272, đã lệch) | — | phép kiểm đi bằng hệ tệp chứ không đi bằng git. Nếu còn thư mục nào khác trong .gitignore mà sinh .md thì sẽ đỏ lại theo đúng cách này |
| `apidoc-mau-route-hang-chuoi` — apidoc từ chối mẫu route là hằng chuỗi — bộ sinh hợp đồng REST hỏng từ 1a8ce11 | xong | tools/apidoc/route.go — mauRoute + hangChuoiTrongTep; 3 ca mới trong route_test.go, đã đột biến (bỏ phần tra hằng -> 2 ca đỏ). `make kb` chạy lại được: hợp đồng đi từ 9 lên 17 tuyến — kiểm 2026-09-20 | — | Chỉ tra hằng khai trong CÙNG tệp, có chủ ý; mẫu dựng lúc chạy vẫn là lỗi và có ca test giữ. Bài học ghi trong mã: bộ sinh từng phạt đúng khuôn mã luật 9 đòi (một đường dẫn, một nguồn) và đẩy người viết đi chép đường dẫn ra hai chỗ |

## `web-admin`

Cập nhật 2026-09-20 · 7 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `tam-viec-web-tu-hop-dong` — Tám màn hình danh mục đã mở việc trong tasks/web/open/, chưa ai nhận | chưa làm | tasks/web/open/ có 8 tệp (task-types · map-asset-types · task-priorities · task-blocs · capital-plan-categories · residential-unit-types · document-types · residential-units); claimed 0 · done 10 · stale 1. web-admin/src/lib/api/schema.gen.ts 545 dòng, sinh từ hợp đồng 17 tuyến ở commit b899f4c — kiểm 2026-09-20 | — | Nhận việc là ĐỔI TÊN tệp sang tasks/web/claimed/ (ROUTING §3) — os.Rename là khoá duy nhất, đừng thêm cờ nào. Mục này tồn tại vì hàng đợi ấy BLOCKS NOTHING (ROUTING.md:137): tám việc có thể nằm đó vô hạn mà không cổng nào kêu, nên chỗ duy nhất làm chúng hiện ra là sổ này |
| `muoi-mot-phan-he-chua-dung` — Mười một phân hệ trong đặc tả chưa có màn hình nào | chưa làm | docs/ui-ux/ có 16 chương; web-admin/src/app/ mới có cau-hinh, dang-nhap, page.tsx, layout.tsx, not-found.tsx, globals.css — kiểm 2026-09-20 | — | Chưa dựng: nhiệm vụ (02) · biên bản họp (04) · văn bản đơn thư (05) · giải ngân (06) · thu chi ngân sách (07) · thông báo (08) · phản ánh (09) · bản đồ kinh tế số (10) · nội dung Mini App (11) · danh bạ cán bộ (12) · báo cáo (13). GIỮ MỘT MỤC chứ không tách mười một: tách ra là mười một dòng cùng nói một câu `chưa bắt đầu`, và sổ phình mà không thêm thông tin. Tách khi một phân hệ thật sự khởi công |
| `man-xac-thuc-loi-khai-cu-tru` — Màn hình cán bộ xác thực lời khai cư trú của công dân | chưa làm | web-admin/src/app/ mới có cau-hinh, dang-nhap, page.tsx, not-found.tsx — kiểm 2026-09-20 | #19 #20 | ADR 0023 đã chốt nghiệp vụ. Hai điều phải đúng NGAY BẢN ĐẦU vì sửa sau là sửa chữ trên màn hình một cơ quan nhà nước: (1) nhãn nói "xác nhận LỜI KHAI", không phải xác nhận nhân thân — nút "Xác nhận thường trú" đứng trơ sẽ được hiểu là đang cấp một xác nhận hành chính; (2) `bị từ chối` là trạng thái riêng, giữ nguyên lời khai và giữ LÝ DO như trường nghiệp vụ BẮT BUỘC — một textarea tuỳ chọn thì thực tế sẽ rỗng, và công dân nhận về một "bị từ chối" không lý do, đúng cái im lặng luật 10 cấm |
| `moi-tuyen-ghi-cho-can-bo` — Mọi tuyến GHI cho cán bộ | chưa làm | — | #9 #10 #11 #12 #13 #14 #15 #16 #17 #18 | cố ý không dựng scaffolding: một tuyến ghi viết dở trông y hệt một quyết định ai đó đã ra |
| `tong-quan-va-so-tay` — Trang /tong-quan và /nhiem-vu/so-tay | chưa làm | — | — | cần API thống kê chưa tồn tại. Một bảng điều khiển với số bịa ra là thứ lãnh đạo đọc rồi báo cáo lên trên |
| `goc-api-noi-bo` — Biến môi trường gốc API nội bộ cho web-admin | chưa làm | tiến trình Next.js gọi https://<Host>/api/v1/communes/current bằng TÊN MIỀN CÔNG KHAI — đã chứng minh, không còn là suy đoán | — | chờ devops xác nhận cụm có split-horizon DNS / chặn egress không; nếu chặn thì MỌI yêu cầu 500. web-admin hiện không có tệp mẫu env nào để thêm vào |
| `can-bo-khong-co-vai-tro` — Cán bộ không có vai trò nào thì vào `/` thấy gì | treo | đặc tả docs/ui-ux §1 chỉ chia "Lãnh đạo" / "vai trò khác" | — | chưa ghi thành câu hỏi mở vì chưa biết trạng thái ấy có tồn tại thật trên dữ liệu xã hay không |
