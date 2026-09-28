---
id: 0056-org-unit-delete-guard
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 6982547
expires: null
owns_facts:
  - "ba điều kiện chặn xoá bộ phận (cán bộ còn hoạt động, bộ phận con chưa xoá, hồ sơ còn mở) và vì sao là HỢP của đặc tả v2 và kho yêu cầu"
  - "vị từ 'còn mở' của từng loại hồ sơ khi đếm cho việc xoá bộ phận, và những thứ cố ý KHÔNG đếm (dự án Thu-Chi, dòng lịch sử, người nhận thông báo)"
  - "chiều phụ thuộc mới identity → petitions/documents cho việc xoá bộ phận, và cái giá: xoá hỏng đóng khi một trong hai service sập"
  - "vì sao hai kênh gRPC mới của identity dùng lối thoát @security-exception thay vì một mục nợ trong tools/security_debt.json"
---

# 0056. Xoá bộ phận: chặn khi còn cán bộ, bộ phận con hoặc hồ sơ còn mở

**Trạng thái:** đã chốt (người dùng, **28/09/2026**) · Phạm vi: menu `/cau-hinh`, tab Sơ đồ tổ
chức, quy tắc `docs/ui-ux/14-cau-hinh.md` §12.4 · Hợp đồng: commit `d1b43da` (proto), máy chủ:
commit `4dd6c33` · Ranh giới giao dịch: `kb/30-indexes/transaction-boundaries.json`, mục
`chan_xoa_bo_phan_con_giu_ho_so`.

## Bối cảnh

Bộ phận (`bo_phan`) thuộc `service-identity`. Nhưng hồ sơ **giữ** một bộ phận nằm ở service
khác: phản ánh và nhiệm vụ ở `petitions`, văn bản đến ở `documents`. Identity không tự biết một
bộ phận còn giữ gì.

Hai nguồn nói khác nhau về điều kiện chặn:

| Nguồn | Chặn khi |
|---|---|
| `docs/ui-ux/14-cau-hinh.md:390` | còn **cán bộ** HOẶC **hồ sơ đang giữ** |
| `../vigov-require/docs/spec/04-api.md:226` · `../vigov-require/apps/api/app/modules/org/service.py:281-294` | còn **cán bộ** HOẶC **bộ phận con** |

Không nguồn nào định nghĩa "hồ sơ đang giữ". Đó là câu sáu tháng sau sẽ bị hỏi lại.

## Quyết định

### 1. Chặn theo HỢP của hai nguồn

Xoá (mềm) bộ phận bị từ chối khi còn **một** trong ba:

| # | Điều kiện | Đọc ở đâu |
|---|---|---|
| a | cán bộ còn hoạt động thuộc bộ phận | CSDL identity |
| b | bộ phận con chưa xoá | CSDL identity |
| c | hồ sơ còn mở do bộ phận giữ | `CountOrgUnitHoldings` trên petitions và documents |

Người dùng chọn hợp, không chọn một bên: bỏ (b) để lại bộ phận con treo dưới cha đã xoá; bỏ (c)
để lại hồ sơ mở trên bộ phận không còn ai nhận.

### 2. Vị từ "còn mở" — từng loại

| Loại | Service | Còn mở khi | Vì sao |
|---|---|---|---|
| `open_petitions` | petitions | trạng thái **không** thuộc `da-dong`, `khong-tiep-nhan`, `chuyen-cap-tren` | `da-xu-ly` và `cho-dan-xac-nhan` **vẫn chặn**: công dân còn gửi trả lại được, phiếu quay về bộ phận |
| `open_tasks` | petitions | trạng thái khác `hoan-thanh`; giữ qua `bo_phan_id` **hoặc** `co_quan_chu_tri_id`, một nhiệm vụ đếm **một lần** | `tam-dung` và dòng `chuyen-tiep` cũ vẫn chặn: việc chưa xong |
| `open_incoming_documents` | documents | `!DaKetThuc` theo `FinishedIncomingStatuses` | một nguồn cho "đã kết thúc", không định nghĩa lại |

**Cố ý KHÔNG đếm:**

| Không đếm | Vì sao |
|---|---|
| Hồ sơ đã xoá mềm, đã kết thúc | không còn ai phải xử lý |
| Dòng lịch sử: `nhat_ky_nhiem_vu`, `nhat_ky_phan_anh`, `tu_/den_bo_phan_id` của luồng chuyển | lịch sử không "giữ"; đếm vào thì không bộ phận nào từng nhận việc xoá được |
| `thong_bao_bo_phan` (comms) | bộ phận là **người nhận**, không giữ hồ sơ |
| `du_an.don_vi_thuc_hien_id` (`service-finance/migrations/0004_du_an_va_chung_tu_giai_ngan.sql:208`) | dự án không có trạng thái, "còn mở" không định nghĩa được; đếm vào thì bộ phận có dự án **không bao giờ** xoá được |

Tenant đi trong metadata (luật 1). Hai RPC **không** nằm trong `core/grpcx.methodsWithoutTenant`.

### 3. Nhất quán mạnh, hỏng đóng

1. Identity gọi hai RPC **ngoài và trước** giao dịch của mình (chỉ đọc).
2. Lỗi gọi **không bao giờ** đọc thành 0 → **503**, thử lại được.
3. Số đếm nào > 0 → **409** kèm số đếm từng loại, để cán bộ biết phải chuyển gì trước.
4. Trong **một** giao dịch identity: kiểm lại (a) và (b), xoá mềm, ghi nhật ký kiểm toán (luật 6, luật 7).

**Khoảng hở chấp nhận:** một hồ sơ được giao cho bộ phận ngay sau lần đếm. Chấp nhận vì xoá là
**mềm** — tên bộ phận vẫn hiện trên hồ sơ, và giao lại vẫn làm được.

### 4. Chiều phụ thuộc mới: identity → petitions / documents

Trước đây chỉ petitions và documents gọi identity. Giờ identity gọi ngược lại cho riêng việc xoá.

- Không thành vòng lúc khởi động: `grpc.NewClient` nối lười.
- Identity đọc `PETITIONS_GRPC_ADDR` và `DOCUMENTS_GRPC_ADDR`, **không bắt buộc**. Chưa đặt →
  xoá bộ phận trả **503**, mọi tuyến khác chạy bình thường.
- **Cái giá:** xoá bộ phận hỏng khi một trong hai service sập. Đúng chiều hỏng đóng — xoá là
  hành động hiếm, chờ được; một bộ phận xoá nhầm khi còn hồ sơ thì không chờ được.

### 5. Kênh gRPC không mã hoá — lối thoát `@security-exception`

Hai kênh mới đi plaintext như `core/identityclient` và `core/platformclient` (nợ trong
`tools/security_debt.json`, hạn 28/12/2026). `hooks/security_guard.py` chặn mọi
`insecure.NewCredentials` mới; sổ nợ không phải lối thoát của hook ấy.

Người dùng chọn: mỗi dòng mang `// @security-exception: <lý do>`, do `security-reviewer` duyệt —
**không** thêm mục nợ.

**Hệ quả phải nhớ:** hai dòng này **không tự hết hạn** như mục nợ. Việc nhắc nằm ở sổ tiến độ
(`kb/90-ephemeral/tien-do/`): chuyển **cả bốn** kênh sang TLS cùng một lần, không để hai kênh
mới sót lại sau khi hai kênh cũ đã xong.

## Phương án bị loại

| Phương án | Vì sao loại |
|---|---|
| Sự kiện + read model số hồ sơ trong identity | trễ, và là bản sao thứ hai của sự thật petitions/documents sở hữu (luật 9) |
| Identity đọc thẳng CSDL petitions/documents | luật 2, cấm #2 |
| Khoá phân tán quanh bộ phận | thêm một lời gọi vào **mọi** lần giao việc, để đóng một khoảng hở đã chấp nhận ở §3 |

## Phát hiện — chưa dựng

Mở lại hồ sơ đã kết thúc về một bộ phận đã xoá: phản ánh bị đánh giá 1–2 sao thì mở lại
(ADR 0008, ADR 0050); nhiệm vụ mở lại cũng vậy. Khi ấy nên **buộc giao lại** cho bộ phận còn
hiệu lực. Chưa dựng; cần một nhiệm vụ riêng.

## Liên kết

- Luật: `.claude/rules/critical/2-service-boundary.md` · `7-data-preservation.md` ·
  `10-citizen-commitment.md` · `13-security-baseline.md`
- ADR: [0012](0012-grpc-boundary-contract.md) · [0025](0025-xac-thuc-giua-cac-service.md) ·
  [0053](0053-tong-quan-dem-truc-tiep-o-service-so-huu.md)
