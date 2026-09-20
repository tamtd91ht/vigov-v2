---
id: tien-do
tier: T5
source: GENERATED
owner: architecture
derived_from_commit: 1d64ec6
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
| chưa làm | 32 |
| treo | 11 |
| xong | 12 |

## Nợ khách chốt — chặn thật, không tự quyết được

Nội dung câu hỏi ở `kb/00-foundation/open-questions.json`. Đây chỉ là ai đang chờ ai.

| Câu | Trạng thái | Đang chặn |
|---|---|---|
| #1 | OPEN | _chung/sap-nhap-chia-tach-xa |
| #4 | OPEN | service-identity/store-crosstenant |
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
| `ra-lop-nhan-dien-theo-ten-thu-muc` — Rà cả lớp lỗi "cơ chế nhận diện mã theo tên thư mục" | treo | đã soát .claude/hooks/ và tools/, tìm được ca thứ năm (stop_verify_guard.CODE_DIR thiếu /tools/), đã vá + thêm ca test | — | chưa soát CI vì chưa có .github/. Ngày dựng CI thì đây là thứ phải soát lại đầu tiên |
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

Cập nhật 2026-09-20 · 3 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `kho-phien-cong-dan-dem` — Đệm TTL ngắn cho đường tra cứu phiên công dân (ADR 0022 đòi) | chưa làm | core/httpx/citizen.go:72-74 khai đây là đường nóng và đòi đệm có TTL ngắn, vô hiệu khi thu hồi; service-identity/internal/store/phien_cong_dan.go ghi rõ đã HOÃN và vì sao — kiểm 2026-09-20 | — | Hoãn có lý do, không phải bỏ quên: identity chạy nhiều bản sao và ADR 0010 chốt chỉ có PostgreSQL, nên không có kênh nào để một lần thu hồi ở bản sao A với tới bản sao B. Cửa sổ lệch sẽ đúng bằng TTL, và ca hỏng là nút 'đăng xuất màn hình này' ở quầy một cửa. Đo trước, rồi mới đệm, kèm kênh vô hiệu hoá thật |
| `backfill-theo-xa` — Backfill dữ liệu theo từng xã | treo | core/migrate chỉ lo DDL — kiểm 2026-09-20 | — | luật 7 bất biến 5 (migration chạy per-commune, resumable, ghi tiến độ) mới đạt một nửa. Ngưỡng cần cơ chế thật là khi thời gian giữ khoá thành đáng kể — ADR 0013, mục Giới hạn |
| `xac-thuc-ben-goi-grpc` — Xác thực bên gọi trên cổng gRPC — một cặp header, giá trị từ secret k8s | xong | core/grpcx/caller_auth.go + caller_auth_test.go + caller_auth_exempt_test.go; hằng MetadataCallerKey trong core/grpcx/grpcx.go; GRPCCallerKey trong core/config/config.go; service-platform/cmd/server/main_test.go chạy bufconn trên hàm dựng thật. `make check` rc=0 — kiểm 2026-09-20. ADR 0025 | — | Ba giới hạn ĐÃ BIẾT, không phải thiếu sót: khoá chung không nói service nào gọi nên vết kiểm không quy được trách nhiệm; ai trong cụm cầm khoá đều gọi được mọi thứ, lớp mạng là thứ chặn bán kính; xoay khoá phải đổi đồng loạt. Đường ra cho cả ba là mTLS/mesh, và phải SỬA ADR 0025 chứ không lặng lẽ thêm header thứ hai |

## `proto`

Cập nhật 2026-09-20 · 1 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `staff-thieu-ho-ten` — Message Staff không có trường họ tên nên BatchGetStaff chưa phục vụ được mục đích nó tự khai | treo | — | #11 | thêm trường là SỬA HỢP ĐỒNG — cùng lúc phải trả lời câu che/không che số di động cán bộ |

## `service-comms`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | chưa làm | XacThuc chỉ tồn tại ở service-identity/internal/http/middleware.go:71 — luật 2 cấm #1 cấm import. Hệ quả đã kiểm: GET /api/v1/map-asset-types trả 401 cho MỌI người gọi — kiểm 2026-09-20 | — | TIỀN ĐỀ ĐÃ XONG (ADR 0025, chốt 2026-09-20, thay thế ADR 0012 quyết định 3). Ba bước còn lại chung cho cả bốn service khung — xem cùng mục ở service-documents.json |
| `sql-chua-chay` — Chạy 0003_danh_muc_loai_tai_nguyen_ban_do.sql trên PostgreSQL thật | chưa làm | 7 ca service-comms/internal/store/loai_tai_nguyen_ban_do_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat — đọc information_schema để đối chiếu danh sách cột trong mã với lược đồ thật. Đó là phép kiểm duy nhất bắt được một migration đổi tên cột dưới chân store; ba service kia đang thêm bản tương ứng |
| `dac-ta-ban-do-tu-mau-thuan` — Đặc tả bản đồ lệch với chính nó — số nhóm và bộ mã | treo | docs/ui-ux/10-ban-do-kinh-te-so.md:37 nói 11 nhóm, :53 nói 8 nhóm; một chỗ dùng mã `enterprise`, chỗ kia `doanh-nghiep`. Ghi sẵn trong service-comms/migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql — kiểm 2026-09-20 | — | Bảng ship RỖNG có chủ đích cho tới khi đặc tả tự khớp. KHÔNG chọn hộ một trong hai bản — hỏi người viết đặc tả |
| `loai-tai-nguyen-ban-do-tuyen-doc` — GET /api/v1/map-asset-types — tuyến ĐỌC danh mục loại tài nguyên bản đồ | xong | service-comms/internal/http/routes.go — tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store service-comms/internal/store/loai_tai_nguyen_ban_do.go. `go test -count=1 ./...` xanh, 21 ca chạy thật (12 tuyến + 9 kho trên driver database/sql giả), 0 skip ngoài bộ pg — kiểm 2026-09-20 | — | tên tài nguyên URL đã chốt với người dùng 2026-09-20 nhưng ubiquitous-language.md:156 vẫn ghi (chưa chốt) — knowledge-keeper phải điền ô đó |

## `service-documents`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `loai-van-ban-tuyen-ghi` — Tuyến GHI danh mục loại văn bản (thêm/sửa/tắt) | chưa làm | không có tuyến POST/PUT/PATCH/DELETE nào trong service-documents/internal/http/routes.go — kiểm 2026-09-20 | #21 | KHÔNG viết trước khi khách chốt: xã sửa được DANH SÁCH MÃ hay chỉ NHÃN và THỨ TỰ. Một tuyến ghi viết trước là quyết hộ khách |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | chưa làm | XacThuc chỉ tồn tại ở service-identity/internal/http/middleware.go:71, nằm trong internal/ của identity nên luật 2 cấm #1 cấm import. Hệ quả đã kiểm: GET /api/v1/document-types trả 401 cho MỌI người gọi — kiểm 2026-09-20 | — | TIỀN ĐỀ ĐÃ XONG: người dùng chốt 2026-09-20 xác thực giữa service bằng MỘT cặp header, giá trị đọc env từ secret k8s — ADR 0025, thay thế ADR 0012 quyết định 3. Còn thiếu ba bước, theo thứ tự: (1) server gRPC của identity, CHƯA tồn tại; (2) một RPC xác minh phiên trong identity.proto — sửa hợp đồng, việc của contract-designer; (3) middleware dựng Principal dùng chung trong core, gọi RPC ấy. Đường token tự kiểm KHÔNG dùng: thu hồi phiên mất tác dụng, vỡ luật 5 bất biến 4 |
| `sql-chua-chay` — Chạy 0003_danh_muc_loai_van_ban.sql trên PostgreSQL thật | chưa làm | 5 ca service-documents/internal/store/loai_van_ban_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN, gói vẫn in `ok` — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat — đọc information_schema để đối chiếu danh sách cột trong mã với lược đồ thật, gồm cả tenant_id/deleted_at/thu_tu vốn không nằm trong SELECT. Nó TỒN TẠI nhưng CHƯA CHẠY: driver giả không bắt được tên cột sai, và ca này chỉ bắt được từ lần chạy đầu tiên có DSN |
| `loai-van-ban-tuyen-doc` — GET /api/v1/document-types — tuyến ĐỌC danh mục loại văn bản | xong | service-documents/internal/http/routes.go — tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store service-documents/internal/store/loai_van_ban.go. `go test -count=1 ./...` xanh, 21 ca chạy thật (11 tuyến + 10 kho trên driver database/sql giả) — kiểm 2026-09-20 | — | tên tài nguyên URL đã chốt với người dùng 2026-09-20 nhưng ubiquitous-language.md:158 vẫn ghi (chưa chốt) — knowledge-keeper phải điền ô đó |

## `service-finance`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `hang-muc-tuyen-ghi` — Tuyến GHI danh mục hạng mục kế hoạch vốn | chưa làm | không có tuyến POST/PUT/PATCH/DELETE nào trong service-finance/internal/http/routes.go — kiểm 2026-09-20 | #21 | KHÔNG viết trước khi khách chốt: xã sửa được DANH SÁCH MÃ hay chỉ NHÃN và THỨ TỰ |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | chưa làm | XacThuc chỉ tồn tại ở service-identity/internal/http/middleware.go:71 — luật 2 cấm #1 cấm import. Hệ quả đã kiểm: GET /api/v1/capital-plan-categories trả 401 cho MỌI người gọi — kiểm 2026-09-20 | — | TIỀN ĐỀ ĐÃ XONG (ADR 0025, chốt 2026-09-20, thay thế ADR 0012 quyết định 3). Ba bước còn lại chung cho cả bốn service khung — xem cùng mục ở service-documents.json |
| `sql-chua-chay` — Chạy 0003_danh_muc_hang_muc_ke_hoach_von.sql trên PostgreSQL thật | chưa làm | 6 ca service-finance/internal/store/hang_muc_ke_hoach_von_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN, gói vẫn in `ok` — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat (đọc information_schema) nhưng nó CHƯA CHẠY. Một lượt đối chiếu TĨNH với DDL của migration cho kết quả thiếu 0 cột — đó là đọc hai tệp trong kho, KHÔNG phải kiểm lược đồ đã triển khai. Driver giả cũng không kiểm được định tuyến phân mảnh, UNIQUE một mặc định, hay trigger ba tầng |
| `hang-muc-ke-hoach-von-tuyen-doc` — GET /api/v1/capital-plan-categories — tuyến ĐỌC danh mục hạng mục kế hoạch vốn | xong | service-finance/internal/http/routes.go — tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store service-finance/internal/store/hang_muc_ke_hoach_von.go. `go test -count=1 ./...` xanh, 22 ca chạy thật (13 tuyến + 9 kho trên driver database/sql giả) — kiểm 2026-09-20 | — | tên tài nguyên URL đã chốt với người dùng 2026-09-20 nhưng ubiquitous-language.md:157 vẫn ghi (chưa chốt) — knowledge-keeper phải điền ô đó |

## `service-identity`

Cập nhật 2026-09-20 · 8 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `store-crosstenant` — Gói đọc chéo xã — nơi duy nhất được phép đọc qua ranh giới xã | ĐANG LÀM | service-identity/internal/store/crosstenant/{dinh_danh_cong_dan.go,doc.go} + 2 tệp test — kiểm 2026-09-20, CHƯA COMMIT (một phiên song song đang viết vùng này lúc kiểm). ban-giao-phien.md §2.3 ghi "chưa tồn tại" là ĐÃ LỖI THỜI | #4 | migration nêu tên ba truy vấn đọc chéo; mới có dinh_danh_cong_dan. Mỗi truy vấn mang `// @cross-tenant: <lý do>` (luật 1 cấm #6) — đó là cách duy nhất khiến đọc chéo thành danh sách ĐẾM ĐƯỢC |
| `kho-doc-kenh-cong-dan` — Kho đọc cho sáu bảng kênh công dân — giai đoạn 2 mới xong một phần | ĐANG LÀM | hợp đồng + rìa core/httpx/citizen.go + sáu bảng migration đã có; store/crosstenant/dinh_danh_cong_dan.go đã có — kiểm 2026-09-20 | — | chưa có route nào. Kho đọc nằm ở service-identity/internal/store/ — vùng khác với vùng đã viết migration, đó là đường nối phải bắc |
| `grpc-server` — Server gRPC của identity | chưa làm | không có service-identity/internal/grpc — kiểm 2026-09-20 | — | VIỆC KẾ TIẾP ĐƯỢC NGƯỜI DÙNG CHỐT 2026-09-20: dựng cái này TRƯỚC, để mở khoá sáu tuyến đọc danh mục đang trả 401 cho mọi người gọi (xem mục `xac-thuc-can-bo` trong sổ của service-{comms,documents,finance,petitions}). Lý do chọn đường này thay vì hai đường nhanh hơn: `XacThuc` tra SỔ ĐĂNG KÝ PHIÊN mỗi request, và bỏ phép tra ấy là token không thu hồi được nữa — phá luật 5 bất biến 4. Dùng lại interceptor hai đầu ở core/grpcx, nay đã có xác thực bên gọi (ADR 0025). ĐỌC ADR 0012 quyết định 1, 2, 4 trước khi thêm RPC — quyết định 3 ĐÃ BỊ THAY THẾ, đừng đọc theo bản cũ |
| `cap-tai-khoan-can-bo` — Toàn bộ luồng cấp tài khoản cán bộ | chưa làm | — | #9 #17 #18 | mật khẩu đầu tiên của cán bộ mới · tự đặt lại mật khẩu · Ghi nhớ đăng nhập — cả ba chờ khách |
| `tuyen-ghi-danh-ba-can-bo` — Toàn bộ tuyến GHI của danh bạ cán bộ | chưa làm | — | #10 #13 #14 | khoá hay xoá cán bộ · chặn mất quản trị viên cuối cùng · tự thao tác lên chính mình |
| `ma-can-bo-va-dien-thoai` — Mã cán bộ do ai đặt, và dien_thoai/di_dong là một trường hay hai | chưa làm | — | #15 #16 | CHẶN SCHEMA nên đắt hơn các câu khác — hỏi trước khi viết migration tiếp |
| `che-so-di-dong-can-bo` — Che hay không che số di động cán bộ, và ai quyết việc công khai lên Mini App | chưa làm | — | #11 #12 | chặn cả cột hiển thị lẫn khoá quyền |
| `ban-giao-viec-khi-khoa-tai-khoan` — Bàn giao việc đang xử lý khi khoá tài khoản | treo | — | — | cố ý CHƯA ghi thành câu hỏi mở: chưa có bảng giao việc nào tồn tại để nói "việc đang giữ" nghĩa là gì. Hỏi khi dựng bảng nghiệp vụ đầu tiên có người phụ trách — câu trả lời nhiều khả năng là "tuỳ xã" |

## `service-petitions`

Cập nhật 2026-09-20 · 6 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `vong-doi-phieu-phan-anh` — Nghiệp vụ phản ánh — tiếp nhận, phân loại, phân công, nghiệm thu, đóng phiếu | chưa làm | — | — | CHỈ bắt đầu sau khi có lich_lam_viec + ngay_nghi_le theo xã (ADR 0007). Đếm hạn bằng giờ hành chính mà thiếu lịch của xã thì mọi con số hạn đều sai, và sai theo hướng không ai thấy cho tới lúc báo cáo lên trên |
| `lich-lam-viec-theo-xa` — Cấu hình lich_lam_viec + ngay_nghi_le theo từng xã | chưa làm | — | — | tiền đề của mọi phép đếm hạn — ADR 0007, luật 10 bất biến 4 |
| `danh-muc-nhiem-vu-tuyen-ghi` — Tuyến GHI hai danh mục nhiệm vụ | chưa làm | không có tuyến POST/PUT/PATCH/DELETE nào trong service-petitions/internal/http/routes.go — kiểm 2026-09-20 | #21 | KHÔNG viết trước khi khách chốt: xã sửa được DANH SÁCH MÃ hay chỉ NHÃN và THỨ TỰ |
| `xac-thuc-can-bo` — Dựng authz.Principal cho yêu cầu của cán bộ | chưa làm | XacThuc chỉ tồn tại ở service-identity/internal/http/middleware.go:71 — luật 2 cấm #1 cấm import. Hệ quả đã kiểm: hai tuyến trên trả 401 cho MỌI người gọi — kiểm 2026-09-20 | — | TIỀN ĐỀ ĐÃ XONG (ADR 0025, chốt 2026-09-20, thay thế ADR 0012 quyết định 3). Ba bước còn lại chung cho cả bốn service khung — xem cùng mục ở service-documents.json |
| `sql-danh-muc-chua-chay` — Chạy 0003_danh_muc_nhiem_vu.sql trên PostgreSQL thật | chưa làm | 5 ca service-petitions/internal/store/danh_muc_nhiem_vu_pg_test.go đều SKIP vì thiếu VIGOV_TEST_DSN, gói vẫn in `ok` — kiểm 2026-09-20 | — | Bộ này có TestPgCotTrongMaKhopVoiLuocDoThat (đọc information_schema, kiểm cả thu_tu — với muc_uu_tien_nhiem_vu thì thu_tu CHÍNH LÀ thang, đổi tên nó là phá thang trong im lặng). Nó TỒN TẠI nhưng CHƯA CHẠY. Vẫn chưa gì kiểm được định tuyến phân mảnh, partial index, và sáu phép từ chối của trigger danh_muc_ba_tang |
| `danh-muc-nhiem-vu-tuyen-doc` — GET /api/v1/task-types + /api/v1/task-priorities — hai tuyến ĐỌC danh mục nhiệm vụ | xong | service-petitions/internal/http/routes.go — hai tuyến, mỗi tuyến khai authz.AnyAuthenticated trong CÙNG câu lệnh; store internal/store/{loai_nhiem_vu,muc_uu_tien_nhiem_vu}.go. `go test -count=1 ./...` xanh, 36 ca chạy thật (20 tuyến + 16 kho trên driver database/sql giả) — kiểm 2026-09-20 | — | 0003_danh_muc_nhiem_vu.sql KHÔNG tạo bảng trạng thái nhiệm vụ nào — đã kiểm. Tên tài nguyên URL chốt với người dùng 2026-09-20 nhưng ubiquitous-language.md:162-163 vẫn ghi (chưa chốt) |

## `service-platform`

Cập nhật 2026-09-20 · 3 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `dang-viet-ten-tinh-thanh` — 34 tên tỉnh/thành ở dạng viết chính thức | ĐANG LÀM | service-platform/migrations/0005_seed_tinh_thanh.sql đã seed, và tự khai ngay đầu tệp: DỮ LIỆU CHƯA ĐƯỢC ĐỐI CHIẾU VỚI VĂN BẢN GỐC — kiểm 2026-09-20 | — | đối chiếu với Nghị quyết 202/2025/QH15. Cột này in thẳng ra màn hình công dân nên DẠNG VIẾT là nội dung (`Đà Nẵng` hay `Thành phố Đà Nẵng`). Đã thử ba nguồn chính phủ, cả ba render phía client. Sửa một tên là THÊM một migration, không đụng tệp đã chạy |
| `loi-he-thong` — Bảng loi_he_thong | chưa làm | `grep -rl loi_he_thong --include=*.sql --include=*.go .` không có kết quả — kiểm 2026-09-20 | — | không bị chặn bởi câu hỏi nào |
| `danh-muc-tham-chieu` — Tám bảng danh mục tham chiếu | xong | ADR 0024 chuyển quyền sở hữu danh mục SANG DỊCH VỤ SỞ HỮU, không nằm ở platform: service-{comms,documents,finance,petitions}/migrations/0003_danh_muc_*.sql + service-identity/migrations/0005 — kiểm 2026-09-20. ban-giao-phien.md §2.5 ghi "platform: danh_muc chưa có" là ĐÃ LỖI THỜI | — | KHÔNG seed dòng nào, có chủ ý (commit fa10cf1) |

## `tools`

Cập nhật 2026-09-20 · 4 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `bo-sinh-doc-dau-entity` — Bộ sinh đọc dấu @entity để điền data-ownership.json | chưa làm | kb/30-indexes/data-ownership.json vẫn là chỗ giữ chỗ rỗng; 8 thực thể danh mục đã có dấu @entity trong migration nhưng không có gì đọc chúng — kiểm 2026-09-20 | — | CLAUDE.md bước 4 dạy phiên sau hỏi 'ai sở hữu X' thì mở data-ownership.json. Hôm nay họ mở ra và không thấy gì cho cả tám, rồi có thể kết luận chúng vô chủ. Bịt bằng bộ sinh trong tools/, KHÔNG bịt bằng cách hand-write một dòng sẽ mục nát ngay khi bộ sinh ra đời (luật 9) |
| `bo-sinh-tien-do` — tools/tien_do.py — sinh tệp đọc tien-do.md từ các tệp ghi theo module | xong | tools/tien_do.py:1; `python tools/tien_do.py` chạy thật, sinh ra kb/90-ephemeral/tien-do.md; đã nối vào mục `kb` của Makefile:122. KHÔNG ghi số module/mục ở đây — con số nào chép vào cũng sai trong vài ngày | — | hạn của tệp sinh tính từ `cap_nhat` MỚI NHẤT của các module, không phải ngày sinh — quá hạn nghĩa là 90 ngày không ai cập nhật tiến độ |
| `check-brain-bo-qua-tmp` — check_brain bất biến 6 đỏ vì một tệp .md nháp trong tmp/ (đã nằm trong .gitignore) | xong | tools/check_brain.py:272 thêm `tmp` vào danh sách loại trừ khi duyệt cây; `python tools/check_brain.py` 7/7 | — | phép kiểm đi bằng hệ tệp chứ không đi bằng git. Nếu còn thư mục nào khác trong .gitignore mà sinh .md thì sẽ đỏ lại theo đúng cách này |
| `apidoc-mau-route-hang-chuoi` — apidoc từ chối mẫu route là hằng chuỗi — bộ sinh hợp đồng REST hỏng từ 1a8ce11 | xong | tools/apidoc/route.go — mauRoute + hangChuoiTrongTep; 3 ca mới trong route_test.go, đã đột biến (bỏ phần tra hằng -> 2 ca đỏ). `make kb` chạy lại được: hợp đồng đi từ 9 lên 17 tuyến — kiểm 2026-09-20 | — | Chỉ tra hằng khai trong CÙNG tệp, có chủ ý; mẫu dựng lúc chạy vẫn là lỗi và có ca test giữ. Bài học ghi trong mã: bộ sinh từng phạt đúng khuôn mã luật 9 đòi (một đường dẫn, một nguồn) và đẩy người viết đi chép đường dẫn ra hai chỗ |

## `web-admin`

Cập nhật 2026-09-20 · 5 mục

| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |
|---|---|---|---|---|
| `man-xac-thuc-loi-khai-cu-tru` — Màn hình cán bộ xác thực lời khai cư trú của công dân | chưa làm | web-admin/src/app/ mới có cau-hinh, dang-nhap, page.tsx, not-found.tsx — kiểm 2026-09-20 | #19 #20 | ADR 0023 đã chốt nghiệp vụ. Hai điều phải đúng NGAY BẢN ĐẦU vì sửa sau là sửa chữ trên màn hình một cơ quan nhà nước: (1) nhãn nói "xác nhận LỜI KHAI", không phải xác nhận nhân thân — nút "Xác nhận thường trú" đứng trơ sẽ được hiểu là đang cấp một xác nhận hành chính; (2) `bị từ chối` là trạng thái riêng, giữ nguyên lời khai và giữ LÝ DO như trường nghiệp vụ BẮT BUỘC — một textarea tuỳ chọn thì thực tế sẽ rỗng, và công dân nhận về một "bị từ chối" không lý do, đúng cái im lặng luật 10 cấm |
| `moi-tuyen-ghi-cho-can-bo` — Mọi tuyến GHI cho cán bộ | chưa làm | — | #9 #10 #11 #12 #13 #14 #15 #16 #17 #18 | cố ý không dựng scaffolding: một tuyến ghi viết dở trông y hệt một quyết định ai đó đã ra |
| `tong-quan-va-so-tay` — Trang /tong-quan và /nhiem-vu/so-tay | chưa làm | — | — | cần API thống kê chưa tồn tại. Một bảng điều khiển với số bịa ra là thứ lãnh đạo đọc rồi báo cáo lên trên |
| `goc-api-noi-bo` — Biến môi trường gốc API nội bộ cho web-admin | chưa làm | tiến trình Next.js gọi https://<Host>/api/v1/communes/current bằng TÊN MIỀN CÔNG KHAI — đã chứng minh, không còn là suy đoán | — | chờ devops xác nhận cụm có split-horizon DNS / chặn egress không; nếu chặn thì MỌI yêu cầu 500. web-admin hiện không có tệp mẫu env nào để thêm vào |
| `can-bo-khong-co-vai-tro` — Cán bộ không có vai trò nào thì vào `/` thấy gì | treo | đặc tả docs/ui-ux §1 chỉ chia "Lãnh đạo" / "vai trò khác" | — | chưa ghi thành câu hỏi mở vì chưa biết trạng thái ấy có tồn tại thật trên dữ liệu xã hay không |
