---
id: tien-do-san-pham
tier: T5
source: GENERATED
owner: architecture
derived_from_commit: d7ac7b3f
expires: 2027-01-07
owns_facts:
  - "tiến độ theo PHÂN HỆ SẢN PHẨM: chương đặc tả nào có bao nhiêu tuyến, bao nhiêu tuyến đã có màn gọi, và mỗi mục menu web-admin đang ở đâu"
---

# Tiến độ theo sản phẩm

**SINH RA — đừng sửa tệp này.** Sửa nguồn rồi chạy `make kb`. Mọi con số dưới đây đọc từ mã;
không dòng nào viết tay (luật 9, bất biến 1).

Trục ở đây là **phân hệ sản phẩm**. Trục theo **module kho mã** nằm ở `kb/90-ephemeral/tien-do.md`;
hai tệp trả lời hai câu khác nhau và không chép của nhau.

Sinh ngày **2026-10-09** · hết hạn **2027-01-07**.

## 1 · Theo chương đặc tả

`ĐÃ GỌI / TUYẾN` là **một phép chia giữa hai con số đếm được**: bao nhiêu tuyến của chương ấy
đã có mã client gọi tới, trên tổng số tuyến chương ấy khai trong hợp đồng. Nó **không phải** một
ước lượng hoàn thành — nó không biết màn dựng tới đâu (xem cột **chưa dựng**) và không biết
phần nào của đặc tả chưa có tuyến nào. Một chương `4/4` vẫn có thể còn nửa đặc tả chưa ai chạm.

| Chương | Tuyến | Đã gọi | Màn web |
|---|---|---|---|
| **00** ViGov — Tổng quan hệ thống | — | — | — |
| **01** Tổng quan điều hành | 6 | 6/6 | ✓ |
| **02** Quản lý nhiệm vụ | 22 | 22/22 | ✓ |
| **03** Sổ tay lãnh đạo | 1 | 1/1 | ✓ |
| **04** Biên bản và kết luận họp | 13 | 13/13 | ✓ |
| **05** Văn bản đến & Đơn thư | 25 | 25/25 | ✓ |
| **06** Theo dõi giải ngân | 27 | 27/27 | ✓ |
| **07** Thu – Chi ngân sách xã | 17 | 14/17 | ✓ |
| **08** Thông báo | 2 | 2/2 | ✓ |
| **09** Phản ánh của người dân | 33 | 20/21 +12 ngoài web | ✓ |
| **10** Bản đồ phát triển kinh tế số | 13 | 13/13 | ✓ |
| **11** Quản trị nội dung Mini App | 25 | 22/22 +3 ngoài web | ✓ |
| **12** Danh bạ cán bộ | 14 | 6/12 +2 ngoài web | ✓ |
| **13** Báo cáo điều hành | 1 | 1/1 | ✓ |
| **14** Cấu hình hệ thống | 135 | 135/135 | ✓ |
| **15** Phụ lục: giao diện dùng chung & xác thực | 9 | 9/9 | ✓ |

Tổng **373 tuyến** trong hợp đồng. **17** tuyến chưa khai `@screen` nên không gom được vào chương nào — chúng không mất đi, chỉ chưa nói được mình phục vụ màn nào.

## 2 · web-admin, theo từng mục menu

`duong: null` nghĩa là mục **cố ý hiện mà không bấm được** — `muc-menu.ts` giải thích vì sao giữ
chúng thay vì xoá. **Chưa dựng** đếm các mục trong `PHAN_CHUA_DUNG`, tức số câu cán bộ THẬT SỰ
đọc được trên màn, không phải số việc ai đó nhớ ra lúc viết tài liệu.

| # | Mục menu | Đường dẫn | Khoá quyền | Màn | Chưa dựng |
|---|---|---|---|---|---|
| 1 | Tổng quan | `/tong-quan` | `REPORT_READ_PERMISSION` | ✓ | 3 |
| 2 | Nhiệm vụ | `/nhiem-vu` | `QUYEN_XEM_NHIEM_VU` | ✓ | 0 |
| 3 | Sổ tay lãnh đạo | `/nhiem-vu/so-tay` | `QUYEN_XEM_NHIEM_VU` | ✓ | **không khai** |
| 4 | Biên bản họp | `/nhiem-vu/bien-ban` | `QUYEN_XEM_NHIEM_VU` | ✓ | 2 |
| 5 | Văn bản & Đơn thư | `/van-ban` | `QUYEN_XEM_VAN_BAN` | ✓ | 9 |
| 6 | Giải ngân | `/giai-ngan` | `QUYEN_XEM_GIAI_NGAN` | ✓ | 1 |
| 7 | Thu - Chi ngân sách | `/giai-ngan/thu-chi` | `QUYEN_XEM_GIAI_NGAN` | ✓ | 0 |
| 8 | Thông báo nội bộ | `/thong-bao` | `QUYEN_SOAN_THONG_BAO` | ✓ | 9 |
| 9 | Danh bạ người dân | — | — | ✗ | |
| 10 | Gửi tin ZNS / SMS | — | — | ✗ | |
| 11 | Phản ánh người dân | `/phan-anh` | `QUYEN_XEM_PHAN_ANH` | ✓ | 10 |
| 12 | Bản đồ kinh tế số | `/ban-do` | `ASSET_READ_PERMISSION` | ✓ | 7 |
| 13 | Nội dung Mini App | `/mini-app` | `MINI_APP_MENU_KEYS` | ✓ | 11 |
| 14 | Báo cáo | `/bao-cao` | `REPORT_READ_PERMISSION` | ✓ | 2 |
| 15 | Người dùng | `/nguoi-dung` | `QUYEN_QUAN_LY_NGUOI_DUNG` | ✓ | 3 |
| 16 | Phân quyền | `/nguoi-dung/phan-quyen` | `QUYEN_PHAN_QUYEN` | ✓ | 3 |
| 17 | Hướng dẫn sử dụng | — | — | ✗ | |
| 18 | Cấu hình | `/cau-hinh` | `KHOA_MO_CAU_HINH` | ✓ | 3 |

**15/18** mục menu có màn thật. **63** phần chưa dựng đang hiện trên các màn ấy.

⚠ **1 màn KHÔNG KHAI khối `PHAN_CHUA_DUNG`**, và ô của chúng đọc là `không khai` chứ không phải `0` —
hai thứ khác hẳn nhau. `0` nghĩa là màn có khai một danh sách và danh sách ấy rỗng, tức mọi phần
đặc tả đã dựng. `không khai` nghĩa là **chưa ai nói màn ấy còn thiếu gì**, nên con số 0 ở đó sẽ là
một lời trấn an không có gì đứng sau.

## 3 · Mini App công dân

| | |
|---|---|
| Tuyến ViGov của kênh công dân (`citizen-only`, hoặc công khai kèm `@consumer citizen-app`) | 17 |
| Trong đó `citizen-app` đang gọi | 9 |
| Thư mục tính năng trong `citizen-app/src/features/` | 6 |

| | Tuyến | `citizen-app` gọi chưa |
|---|---|---|
| GET | `/api/v1/commune-external-contacts` | ✗ |
| GET | `/api/v1/commune-news` | ✓ |
| GET | `/api/v1/commune-news/categories` | ✓ |
| GET | `/api/v1/commune-news/{id}` | ✓ |
| GET | `/api/v1/commune-staff` | ✓ |
| GET | `/api/v1/my-citizen-report-fields` | ✓ |
| GET | `/api/v1/my-citizen-reports` | ✓ |
| POST | `/api/v1/my-citizen-reports` | ✓ |
| GET | `/api/v1/my-citizen-reports/{maTraCuu}` | ✗ |
| GET | `/api/v1/my-citizen-reports/{maTraCuu}/photos` | ✗ |
| POST | `/api/v1/my-citizen-reports/{maTraCuu}/photos` | ✗ |
| POST | `/api/v1/my-citizen-reports/{maTraCuu}/photos/{id}/completion` | ✗ |
| POST | `/api/v1/my-citizen-reports/{maTraCuu}/rating` | ✗ |
| GET | `/api/v1/my-citizen-reports/{maTraCuu}/verification-photos` | ✗ |
| GET | `/api/v1/public-citizen-report-fields` | ✓ |
| POST | `/api/v1/public-citizen-reports` | ✓ |
| GET | `/api/v1/public-citizen-reports/{maTraCuu}` | ✗ |

⚠ **`citizen-app` hôm nay chưa gọi một tuyến ViGov nào**, và đó không phải thiếu sót của nó: nó đang
gọi `/api/v1/citizen-sessions`, `/api/v1/client-errors`, `/api/v1/commune-news`, `/api/v1/commune-news/categories`, `/api/v1/commune-news/{id}`, `/api/v1/commune-profiles`, `/api/v1/commune-staff`, `/api/v1/communes`, `/api/v1/location`, `/api/v1/my-citizen-report-fields`, `/api/v1/my-citizen-reports`, `/api/v1/public-citizen-report-fields`, `/api/v1/public-citizen-reports`, `/api/v1/public-citizen-reports/{code}`, `/api/v1/requests`, `/api/v1/sessions`, `/api/v1/x` — bề mặt của **kho anh em**
`vihat-miniapp`, không phải của ViGov (CLAUDE.md, mục hai kho). Giai đoạn 2 — màn nghiệp vụ xã —
bị chặn ở `service-identity`, xem `kb/90-ephemeral/tien-do/citizen-app.json`.

## 4 · Sổ tiến độ theo module

Chi tiết từng mục — bằng chứng, câu chờ khách, bước kế tiếp — ở `kb/90-ephemeral/tien-do.md`.

| Module | xong | đang làm | chưa làm | treo |
|---|---|---|---|---|
| `_chung` | 22 | 3 | 8 | 6 |
| `citizen-app` | 44 | 24 | 4 | 0 |
| `core` | 34 | 3 | 1 | 1 |
| `deploy` | 19 | 12 | 1 | 0 |
| `platform-admin` | 8 | 3 | 0 | 0 |
| `proto` | 19 | 1 | 0 | 0 |
| `service-comms` | 24 | 12 | 4 | 3 |
| `service-documents` | 12 | 4 | 3 | 0 |
| `service-finance` | 31 | 5 | 2 | 0 |
| `service-identity` | 50 | 14 | 2 | 1 |
| `service-petitions` | 51 | 18 | 5 | 0 |
| `service-platform` | 16 | 17 | 5 | 1 |
| `service-reporting` | 4 | 0 | 1 | 0 |
| `tools` | 20 | 0 | 0 | 0 |
| `web-admin` | 84 | 38 | 3 | 1 |

