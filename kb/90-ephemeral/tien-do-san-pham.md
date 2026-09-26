---
id: tien-do-san-pham
tier: T5
source: GENERATED
owner: architecture
derived_from_commit: 872ca37
expires: 2026-12-25
owns_facts:
  - "tiến độ theo PHÂN HỆ SẢN PHẨM: chương đặc tả nào có bao nhiêu tuyến, bao nhiêu tuyến đã có màn gọi, và mỗi mục menu web-admin đang ở đâu"
---

# Tiến độ theo sản phẩm

**SINH RA — đừng sửa tệp này.** Sửa nguồn rồi chạy `make kb`. Mọi con số dưới đây đọc từ mã;
không dòng nào viết tay (luật 9, bất biến 1).

Trục ở đây là **phân hệ sản phẩm**. Trục theo **module kho mã** nằm ở `kb/90-ephemeral/tien-do.md`;
hai tệp trả lời hai câu khác nhau và không chép của nhau.

Sinh ngày **2026-09-26** · hết hạn **2026-12-25**.

## 1 · Theo chương đặc tả

`ĐÃ GỌI / TUYẾN` là **một phép chia giữa hai con số đếm được**: bao nhiêu tuyến của chương ấy
đã có mã client gọi tới, trên tổng số tuyến chương ấy khai trong hợp đồng. Nó **không phải** một
ước lượng hoàn thành — nó không biết màn dựng tới đâu (xem cột **chưa dựng**) và không biết
phần nào của đặc tả chưa có tuyến nào. Một chương `4/4` vẫn có thể còn nửa đặc tả chưa ai chạm.

| Chương | Tuyến | Đã gọi | Màn web |
|---|---|---|---|
| **00** ViGov — Tổng quan hệ thống | — | — | — |
| **01** Tổng quan điều hành | — | — | — |
| **02** Quản lý nhiệm vụ | 9 | 9/9 | ✓ |
| **03** Sổ tay lãnh đạo | — | — | — |
| **04** Biên bản và kết luận họp | 13 | 13/13 | ✓ |
| **05** Văn bản đến & Đơn thư | 7 | 7/7 | ✓ |
| **06** Theo dõi giải ngân | 11 | 11/11 | ✓ |
| **07** Thu – Chi ngân sách xã | 12 | 12/12 | ✓ |
| **08** Thông báo | 2 | 2/2 | ✓ |
| **09** Phản ánh của người dân | 13 | 10/10 +3 ngoài web | ✓ |
| **10** Bản đồ phát triển kinh tế số | 1 | 1/1 | ✓ |
| **11** Quản trị nội dung Mini App | 6 | 6/6 | ✓ |
| **12** Danh bạ cán bộ | 5 | 5/5 | ✓ |
| **13** Báo cáo điều hành | — | — | — |
| **14** Cấu hình hệ thống | 59 | 59/59 | ✓ |
| **15** Phụ lục: giao diện dùng chung & xác thực | 5 | 5/5 | ✓ |

Tổng **148 tuyến** trong hợp đồng. **5** tuyến chưa khai `@screen` nên không gom được vào chương nào — chúng không mất đi, chỉ chưa nói được mình phục vụ màn nào.

## 2 · web-admin, theo từng mục menu

`duong: null` nghĩa là mục **cố ý hiện mà không bấm được** — `muc-menu.ts` giải thích vì sao giữ
chúng thay vì xoá. **Chưa dựng** đếm các mục trong `PHAN_CHUA_DUNG`, tức số câu cán bộ THẬT SỰ
đọc được trên màn, không phải số việc ai đó nhớ ra lúc viết tài liệu.

| # | Mục menu | Đường dẫn | Khoá quyền | Màn | Chưa dựng |
|---|---|---|---|---|---|
| 1 | Tổng quan | — | — | ✗ | |
| 2 | Nhiệm vụ | `/nhiem-vu` | `QUYEN_XEM_NHIEM_VU` | ✓ | 17 |
| 3 | Sổ tay lãnh đạo | — | — | ✗ | |
| 4 | Biên bản họp | `/nhiem-vu/bien-ban` | `QUYEN_XEM_NHIEM_VU` | ✓ | 4 |
| 5 | Văn bản & Đơn thư | `/van-ban` | `QUYEN_XEM_VAN_BAN` | ✓ | **không khai** |
| 6 | Giải ngân | `/giai-ngan` | `QUYEN_XEM_GIAI_NGAN` | ✓ | 6 |
| 7 | Thu - Chi ngân sách | `/giai-ngan/thu-chi` | `QUYEN_XEM_GIAI_NGAN` | ✓ | 2 |
| 8 | Thông báo | `/thong-bao` | `QUYEN_SOAN_THONG_BAO` | ✓ | 12 |
| 9 | Phản ánh người dân | `/phan-anh` | `QUYEN_XEM_PHAN_ANH` | ✓ | 9 |
| 10 | Bản đồ kinh tế số | — | — | ✗ | |
| 11 | Nội dung Mini App | `/noi-dung` | `QUYEN_XEM_NOI_DUNG` | ✓ | 11 |
| 12 | Danh bạ cán bộ | `/danh-ba` | `QUYEN_QUAN_LY_NGUOI_DUNG` | ✓ | 4 |
| 13 | Báo cáo | — | — | ✗ | |
| 14 | Cấu hình | `/cau-hinh` | `KHOA_MO_CAU_HINH` | ✓ | 10 |

**10/14** mục menu có màn thật. **75** phần chưa dựng đang hiện trên các màn ấy.

⚠ **1 màn KHÔNG KHAI khối `PHAN_CHUA_DUNG`**, và ô của chúng đọc là `không khai` chứ không phải `0` —
hai thứ khác hẳn nhau. `0` nghĩa là màn có khai một danh sách và danh sách ấy rỗng, tức mọi phần
đặc tả đã dựng. `không khai` nghĩa là **chưa ai nói màn ấy còn thiếu gì**, nên con số 0 ở đó sẽ là
một lời trấn an không có gì đứng sau.

## 3 · Mini App công dân

| | |
|---|---|
| Tuyến ViGov dành riêng kênh công dân (`citizen-only`) | 3 |
| Trong đó `citizen-app` đang gọi | 2 |
| Thư mục tính năng trong `citizen-app/src/features/` | 7 |

| | Tuyến | `citizen-app` gọi chưa |
|---|---|---|
| GET | `/api/v1/my-citizen-reports` | ✓ |
| POST | `/api/v1/my-citizen-reports` | ✓ |
| GET | `/api/v1/my-citizen-reports/{maTraCuu}` | ✗ |

⚠ **`citizen-app` hôm nay chưa gọi một tuyến ViGov nào**, và đó không phải thiếu sót của nó: nó đang
gọi `/api/v1/my-citizen-reports`, `/api/v1/requests`, `/api/v1/sessions` — bề mặt của **kho anh em**
`vihat-miniapp`, không phải của ViGov (CLAUDE.md, mục hai kho). Giai đoạn 2 — màn nghiệp vụ xã —
bị chặn ở `service-identity`, xem `kb/90-ephemeral/tien-do/citizen-app.json`.

## 4 · Sổ tiến độ theo module

Chi tiết từng mục — bằng chứng, câu chờ khách, bước kế tiếp — ở `kb/90-ephemeral/tien-do.md`.

| Module | xong | đang làm | chưa làm | treo |
|---|---|---|---|---|
| `_chung` | 20 | 2 | 8 | 6 |
| `citizen-app` | 11 | 4 | 4 | 0 |
| `core` | 13 | 0 | 1 | 1 |
| `deploy` | 14 | 2 | 1 | 0 |
| `platform-admin` | 1 | 0 | 1 | 0 |
| `proto` | 9 | 0 | 0 | 0 |
| `service-comms` | 9 | 3 | 2 | 4 |
| `service-documents` | 7 | 2 | 4 | 0 |
| `service-finance` | 15 | 3 | 0 | 0 |
| `service-identity` | 18 | 8 | 0 | 1 |
| `service-petitions` | 15 | 7 | 5 | 0 |
| `service-platform` | 5 | 2 | 0 | 1 |
| `service-reporting` | 2 | 0 | 1 | 0 |
| `tools` | 18 | 0 | 0 | 0 |
| `web-admin` | 4 | 16 | 3 | 1 |

