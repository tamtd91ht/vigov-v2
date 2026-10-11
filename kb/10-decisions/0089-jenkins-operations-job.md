---
id: 0089-jenkins-operations-job
tier: T1
source: CURATED
owner: architecture
derived_from_commit: bbc29c25
expires: null
owns_facts:
  - "vì sao có job vận hành Jenkins MỚI deploy/van-hanh/Jenkinsfile chạy song song job cũ deploy/Jenkinsfile, dựng trên plugin Active Choices, form bốn vùng (ngữ cảnh đọc từ cụm · nhóm → việc · tham số theo việc · an toàn) (chủ dự án, 10/10/2026)"
  - "vì sao job vận hành mặc định chạy thử (dry-run) in kế hoạch cũ → mới, chỉ khởi động lại dịch vụ dùng khoá vừa đổi, và đòi gõ namespace + số phiếu cho mọi lần ghi prod (chủ dự án, 10/10/2026)"
  - "lần dùng đầu của job vận hành: đổi cấu hình tài nguyên prod và chuyển prod sang một CSDL PostgreSQL MỚI (một máy chủ, mỗi dịch vụ một CSDL), bảng rỗng do migration của từng dịch vụ tạo, CSDL cũ không bị đụng, DSN cũ giữ được để quay lui, KHÔNG chuyển dữ liệu ở đợt này (mặc định chủ dự án 10/10/2026, có thể đổi)"
  - "vì sao job vận hành được dựng lại chỉ với MỘT việc — đổi hạ tầng PostgreSQL/MinIO/Redis/ClamAV, form theo hạ tầng, ô trống = giữ, mật khẩu qua bảy tham số password, kiểm từ trong cụm trước khi ghi — và bỏ mọi việc cũ (chủ dự án, 11/10/2026)"
  - "rủi ro được chấp nhận: Jenkins lưu tham số của lượt build — tham số kiểu password được mã hoá nhưng quản trị viên Jenkins đọc được (10/10/2026)"
---

# 0089. Job vận hành Jenkins mới, form đọc từ cụm (Active Choices)

**Trạng thái:** đã chốt hướng, chưa dựng · **Ngày:** 2026-10-10 · **Người quyết:** chủ dự án,
10/10/2026, truyền qua phiên chính (không có phiếu hỏi trong kho để đối chiếu) · **Dựng:** chưa có
commit nào.

Cách dựng job trên Jenkins, quy trình đưa ảnh lên cụm và bảng biến → ConfigMap/Secret **không**
thuộc ADR này: chúng thuộc `deploy/README.md` mục 2–3 và `deploy/cau-hinh/README.md` mục 1–4. ADR
này chỉ giữ **vì sao** có job mới và những ràng buộc nó phải giữ.

## Bối cảnh

| Hiện trạng | Hệ quả | Nguồn |
|---|---|---|
| Job hạ tầng `vigov-deploy` có **một** danh sách `HANH_DONG` 16 việc, mô tả dồn vào một chuỗi dài; tham số `XAC_NHAN`, `TENANT_ID`, `APP_ID`, `TICKET`, `APP_SECRET`… hiện cho **mọi** việc dù mỗi cái chỉ dùng cho vài việc | Người bấm phải đọc chuỗi mô tả để biết ô nào có nghĩa với việc mình chọn | `deploy/Jenkinsfile:89-166` |
| Form là tham số tĩnh: giá trị hiện tại trên cụm **không hiện** trong form | Người vận hành gõ mù rồi đọc log để biết mình đã đổi gì | — |
| Máy chủ Jenkins **chỉ có bộ plugin lõi** và **dùng chung với dự án khác** | Active Choices là plugin mới trên một controller dùng chung; script Groovy duyệt ở đó có hiệu lực toàn controller | `deploy/README.md:160-161` |
| Nhật ký `vigov-deploy` cắt sau 200 lượt | Vết "ai làm gì" trong log build **không bền** | `deploy/Jenkinsfile:69`, `deploy/README.md:61-62` |

## Quyết định

Một job **mới**, `deploy/van-hanh/Jenkinsfile`, chạy **song song** `deploy/Jenkinsfile`. Job cũ ở lại
tới khi chủ dự án cho nghỉ — không gỡ, không chuyển việc cũ sang tự động.

| Vùng | Nội dung | Vì sao |
|---|---|---|
| 1. Ngữ cảnh | Môi trường `prod` \| `staging` + **đọc sống từ cụm**: dịch vụ sẵn sàng, thẻ ảnh đang chạy, khoá cấu hình còn thiếu so với `.env.example` | Người bấm thấy trạng thái thật trước khi chọn việc; `.env.example` là sổ đăng ký biến (luật 11 bất biến 6) |
| 2. Việc | Nhóm (Kiểm tra · Cấu hình · Bí mật · CSDL · Tài nguyên · Triển khai · Xã/tenant · Bảo trì) → việc | Thay danh sách phẳng 16 việc |
| 3. Tham số | Hiện **theo việc**, điền sẵn bằng giá trị **đọc từ cụm** — xem bảng dưới | Việc tay chỉ còn là **gõ giá trị** |
| 4. An toàn | Chạy thử **BẬT mặc định**, in kế hoạch cũ → mới · chỉ khởi động lại dịch vụ dùng khoá đã đổi · ghi prod phải gõ đúng namespace **và** số phiếu | Một lần bấm nhầm trên prod chỉ in ra kế hoạch, không đổi gì |

| Việc | Tham số hiện ra |
|---|---|
| ConfigMap | Mỗi dòng: khoá · giá trị hiện tại · giá trị mới · dịch vụ dùng · ý nghĩa |
| Secret | Mỗi dòng: khoá · **có / không có**. **Không bao giờ** hiện giá trị |
| Tài nguyên | Bảng request/limit điền sẵn từ cụm; mẫu đặt sẵn chỉ là **đề xuất** |
| CSDL mới | Máy chủ · tài khoản quản trị · `sslmode`; mỗi dịch vụ: tên CSDL + tài khoản; ô tick từng bước |
| Di trú | Danh sách migration chưa chạy (diff) |
| Triển khai | Thẻ ảnh · quay lui |

### Lần dùng đầu

Đổi cấu hình tài nguyên prod, và **chuyển prod sang một CSDL PostgreSQL MỚI**:

| Điểm | Chốt |
|---|---|
| Hình dạng | Một máy chủ, **mỗi dịch vụ một CSDL** — như hôm nay (lý do tách CSDL: `deploy/README.md:332-338`) |
| Bảng | Rỗng, do **migration của từng dịch vụ** tạo |
| CSDL cũ | **Không bị đụng** — không ghi, không xoá (luật 7) |
| DSN cũ | Giữ được để **quay lui** |
| Dữ liệu | **KHÔNG chuyển** ở đợt này — mặc định chủ dự án 10/10/2026, có thể đổi. Chuyển dữ liệu là một quyết định riêng |

### Ràng buộc mang theo

| Luật | Job phải giữ |
|---|---|
| 11 | Chỉ ghi khoá có trong `.env.example`. Địa chỉ giữ **hình dạng cụm** (danh sách `host:port`, DSN nhiều host) — không cắt về một host. Không giá trị riêng của xã. Danh sách "dịch vụ dùng khoá" phải **suy ra** từ nguồn có sẵn (nhóm biến dịch vụ khai, ADR 0057; bảng kiểm bởi `tools/check_env_map.py`), không chép tay vào Jenkinsfile (luật 9) |
| 8 | Bí mật **không bao giờ** vào log hay đối số dòng lệnh |
| 8 — rủi ro chấp nhận | Jenkins **lưu tham số** của mỗi lượt build. Tham số kiểu password được mã hoá khi lưu nhưng **quản trị viên Jenkins đọc được**; controller này dùng chung với dự án khác |
| 13 | `sslmode` **không bao giờ** `disable`. Mật khẩu/khoá do job sinh lấy từ **CSPRNG** |
| 7 | Không `DROP`, không xoá. CSDL cũ để nguyên |
| 6 (tương tự) | Log build ghi: ai bấm · việc gì · lúc nào · namespace · số phiếu. **Không bền** — bị cắt theo `buildDiscarder` như job cũ |

## Hệ quả — những điều phải biết trước lần dùng đầu

| # | Hệ quả | Nguồn |
|---|---|---|
| 1 | Staging và prod **đang dùng chung CSDL, Redis, khoá ký phiên**. Chuyển prod sang CSDL mới thì **staging ở lại CSDL cũ** và vẫn ghi vào đó — "CSDL cũ không bị đụng" chỉ đúng với job, không đúng với staging. Các stage cũ viết theo giả định chung CSDL (`MT=prod` CHỈ) sẽ sai giả định | ADR 0046:153, :166; `deploy/Jenkinsfile:1404`, `:1524` |
| 2 | CSDL prod mới **không có dòng xã nào** ⇒ mọi tên miền xã trên prod trả **404** (luật 1 bất biến 3) cho tới khi xã được tạo lại; **không có tài khoản vận hành** ⇒ `platform-admin` không đăng nhập được cho tới khi tạo lại (`operatorctl`); dòng `mini_app` và App Secret của xã nằm lại CSDL cũ. Chỉ dữ liệu do migration gieo mới có mặt | ADR 0048 §*Chốt bổ sung 01/10* #6 |
| 3 | Mọi hồ sơ nghiệp vụ đã ghi trên prod ở lại CSDL cũ, **không hiện** trên prod. Đó là hồ sơ lưu trữ (luật 7): CSDL cũ là bản duy nhất cho tới khi có quyết định chuyển dữ liệu | Luật 7 |
| 4 | `vihat-miniapp` đang **dùng chung CSDL của một dịch vụ ViGov** (chủ dự án cho phép 08/10/2026). DSN của nó nằm ở `vihat-miniapp-bi-mat`, không phải khoá của ViGov | `deploy/Jenkinsfile:1825-1827` |
| 5 | Plugin Active Choices và mỗi lần duyệt script đổi **controller dùng chung** — ảnh hưởng cả dự án kia | `deploy/README.md:160-161` |

## Còn mở — hỏi chủ dự án, phiên dựng không tự chọn

| # | Câu hỏi | Ghi chú |
|---|---|---|
| 1 | Active Choices **đã cài** trên controller chưa? | Chưa cài thì job **từ chối rõ ràng**, nêu tên plugin — không lùi về form tĩnh |
| 2 | Ai duyệt script (*In-process Script Approval*) trên controller dùng chung? | Duyệt có hiệu lực toàn controller |
| 3 | **Cách viết khoá.** Luật 11 bất biến 4 nói khoá ConfigMap/Secret viết **gạch ngang**; nhưng cụm thật nạp bằng `envFrom`, nên khoá **phải gạch dưới** — khoá gạch ngang bị bỏ qua im lặng | `deploy/README.md:308-315`; `deploy/cau-hinh/README.md:4-5`. Job ghi theo dạng Deployment đang đọc; hai nguồn này phải được chủ dự án hoà giải, không phải job. **Đã giải quyết: ADR 0093** (11/10/2026) |
| 4 | DSN cũ **giữ ở đâu** để quay lui? | Không được là một khoá thêm trong Secret nạp bằng `envFrom`: nó thành một biến môi trường không có dòng `.env.example` (luật 11 cấm #4). Một Secret riêng không gắn vào Deployment là một cách |
| 5 | Staging sau khi tách: ở lại CSDL cũ, hay cũng chuyển? | Hệ quả #1 |
| 6 | `vihat-miniapp` theo prod sang CSDL mới, hay ở lại? | Hệ quả #4 |
| 7 | Nhóm **Xã/tenant** làm gì? | Hướng đã chốt: việc của xã làm ở `platform-admin`, stage SQL Jenkins là tạm (ADR 0048 §*Chốt* #6d; ADR 0070 #1). Nhóm này không được dựng lại những gì `platform-admin` đã làm |
| 8 | Nhóm **Triển khai** (đặt thẻ, quay lui) và job của từng dịch vụ: ai giữ lịch sử "ai đưa bản nào lên"? | Hôm nay lịch sử ấy nằm ở job dịch vụ (`deploy/README.md` mục 1–2). Hai job cùng đặt ảnh là hai nơi ghi một sự kiện |
| 9 | Mẫu tài nguyên đặt sẵn | Chỉ là **đề xuất**; con số do chủ dự án chọn |

Ingress vẫn dựng tay trong Rancher — job mới **không** áp Ingress (ADR 0046 #4).

## Sửa đổi 11/10/2026 — dựng lại chỉ cho "đổi hạ tầng"

**Người quyết:** chủ dự án, 11/10/2026, trong phiên chính: "UI UX trên Jenkins đang rất kém, tôi chả dùng
được gì ở đó", "xóa hết công việc đang có trong job vận hành này đi và làm lại mục hạ tầng, kiểu đập đi
xây lại", "chỉ của jenkins vihat-van-hanh này thôi"; chọn **cài Active Choices** thay vì form tĩnh; giao
phiên toàn quyền chọn phương án còn lại.

| Điểm | Chốt | Vì sao |
|---|---|---|
| Phạm vi job | **Một việc**: đổi hạ tầng mà dịch vụ trỏ tới — PostgreSQL · MinIO · Redis · ClamAV. 14 việc cũ (nhóm Kiểm tra · Cấu hình · Bí mật · CSDL · Tài nguyên · Triển khai · Bảo trì) **bỏ** | Form cũ hiện 25 ô cho mọi việc; người vận hành nghĩ theo hạ tầng ("MinIO của mình"), không theo đối tượng k8s — đổi MinIO phải biết khoá nào ở `common-config`, khoá nào ở ba Secret |
| Form | `MT` → `HA_TANG` (một hạ tầng) → bảng **chỉ các ô của hạ tầng ấy**, cạnh giá trị đang chạy; **ô trống = giữ** | Gõ đúng thứ cần đổi, không gõ lại thứ đang đúng |
| Mật khẩu / khoá | Bảy tham số password `BI_MAT_1..7`; bảng chọn ô nào vào khoá nào | Giá trị ô động của Active Choices được Jenkins lưu **chữ thường**; chỉ tham số password được mã hoá (rủi ro chấp nhận ở bảng "Ràng buộc mang theo" vẫn đúng). Bảy = số dịch vụ giữ `DATABASE_DSN` |
| Trước khi ghi | Pod một lượt **kiểm từ trong cụm**: mở được cổng máy chủ mới; **đăng nhập được** bằng từng DSN PostgreSQL mới. Không đạt → không ghi | Một DSN sai được phát hiện trước khi bảy dịch vụ cùng khởi động lại vào nó |
| Ghi | Sao lưu giá trị cũ sang `<dịch vụ>-secrets-backup-<giờ>` (Secret riêng — trả lời câu mở #4) → ghi → đọc lại → khởi động lại platform → identity → còn lại | Như ràng buộc cũ |
| Dữ liệu | Job **không** chuyển dữ liệu, **không** tạo CSDL. Hạ tầng mới do người quản trị dựng sẵn | Luật 7. Việc `csdl-moi` (CSDL mới rỗng) bỏ cùng các việc cũ |
| `vigov-deploy` | Không đổi việc nào. `nen-ban-do` chỉ còn ở đó; thân việc (`deploy/van-hanh/basemap-task.sh` + `ops.py`) giữ nguyên chỗ | Lời chủ dự án: chỉ job `vigov-van-hanh` |

**Hệ quả:** mục "Lần dùng đầu" (CSDL prod mới rỗng) và câu mở #7–#9 không còn việc nào trong job gánh;
bảng "Vùng"/"Việc" ở mục Quyết định là lịch sử. Khôi phục Secret từ bản sao lưu nay làm bằng tay
(`deploy/van-hanh/README.md` mục 4). Cách làm: `deploy/van-hanh/README.md`.
