# ViGov — Bộ tài liệu đặc tả để dựng lại ứng dụng

Tài liệu này mô tả đầy đủ bản web prototype **ViGov — Điều hành số cấp xã**
(`https://vigov-admin-production.up.railway.app`, phiên bản 0.1.0) để Claude Code dựng lại ứng dụng.

Đơn vị mẫu trong prototype: **Uỷ ban nhân dân xã Thăng Bình — Thành phố Đà Nẵng**.

---

## Cách dùng với Claude Code

1. Đặt cả thư mục này vào repo, ví dụ `docs/vigov/`.
2. Bắt đầu bằng `00-tong-quan-he-thong.md` — kiến trúc, design system, mô hình dữ liệu tổng thể, thứ tự dựng.
3. Mỗi lần dựng một module, nạp **file tổng + file module đó**, không nạp cả bộ (tiết kiệm ngữ cảnh).
4. Prompt gợi ý:
   > Đọc `docs/vigov/00-tong-quan-he-thong.md` và `docs/vigov/02-nhiem-vu.md`.
   > Dựng module Nhiệm vụ đúng đặc tả: route, bộ lọc, 3 chế độ xem, drawer chi tiết,
   > form Giao việc mới, schema Prisma và các API đã nêu. Giữ nguyên toàn bộ chuỗi tiếng Việt.

---

## Danh mục tài liệu

| File | Nội dung | Route |
|---|---|---|
| [`00-tong-quan-he-thong.md`](00-tong-quan-he-thong.md) | **Đọc trước tiên.** Sản phẩm là gì, vai trò người dùng, bản đồ 14 route, design system, mô hình dữ liệu tổng, 9 quy tắc xuyên suốt, gợi ý stack, thứ tự dựng | — |
| [`01-tong-quan-dieu-hanh.md`](01-tong-quan-dieu-hanh.md) | Bảng điều hành: 6 nhóm KPI, khối Cần xử lý ngay, xuất file, trình chiếu | `/tong-quan` |
| [`02-nhiem-vu.md`](02-nhiem-vu.md) | **Module lõi.** Kanban/Danh sách/Sổ theo dõi, 7 trạng thái, 2 loại nhiệm vụ, giao việc, lùi hạn, nhật ký | `/nhiem-vu` |
| [`03-so-tay-lanh-dao.md`](03-so-tay-lanh-dao.md) | Ba cột: việc quá hạn, chờ tôi duyệt, việc tôi đã giao | `/nhiem-vu/so-tay` |
| [`04-bien-ban-hop.md`](04-bien-ban-hop.md) | Biên bản → kết luận → tách thành nhiệm vụ, truy vết ngược | `/nhiem-vu/bien-ban` |
| [`05-van-ban-don-thu.md`](05-van-ban-don-thu.md) | Sổ đơn thư công dân, 6 trạng thái, tab báo cáo năm | `/van-ban` |
| [`06-giai-ngan.md`](06-giai-ngan.md) | Dự án, hạng mục vốn, nguồn vốn, chứng từ, vướng mắc, điểm chậm | `/giai-ngan` |
| [`07-thu-chi-ngan-sach.md`](07-thu-chi-ngan-sach.md) | Bảng cây ngân sách động sinh từ Excel, 3 cách tính, đợt thu chi | `/giai-ngan/thu-chi` |
| [`08-thong-bao.md`](08-thong-bao.md) | Phát thông báo nội bộ, bắt buộc xác nhận, gửi email | `/thong-bao` |
| [`09-phan-anh-nguoi-dan.md`](09-phan-anh-nguoi-dan.md) | 9 trạng thái, 12 lĩnh vực, ảnh trước/sau, đánh giá sao, kiểm duyệt công khai | `/phan-anh` |
| [`10-ban-do-kinh-te-so.md`](10-ban-do-kinh-te-so.md) | 11 nhóm tài nguyên, trường tuỳ biến, heatmap phản ánh, sổ địa điểm | `/ban-do` |
| [`11-noi-dung-mini-app.md`](11-noi-dung-mini-app.md) | CMS Zalo Mini App, đồng bộ từ Cổng thông tin điện tử | `/mini-app` |
| [`12-danh-ba-can-bo.md`](12-danh-ba-can-bo.md) | Danh bạ nội bộ + cờ công khai trên Mini App | `/danh-ba` |
| [`13-bao-cao.md`](13-bao-cao.md) | Báo cáo theo kỳ, xếp hạng bộ phận, xuất PDF/XLSX/PPTX | `/bao-cao` |
| [`14-cau-hinh.md`](14-cau-hinh.md) | **Dựng trước.** 10 tab: tổ chức, thôn/tổ, người dùng, 43 quyền, 55 danh mục, trường bản đồ, lời hệ thống, SLA, tự động hoá, máy chủ thư | `/cau-hinh` |
| [`15-phu-luc-giao-dien-chung.md`](15-phu-luc-giao-dien-chung.md) | Xác thực, sidebar, header, tìm kiếm, chuông, trình chiếu, component dùng chung, responsive, a11y | mọi trang |

---

## Thứ tự dựng đề xuất

```
1. Khung ứng dụng + xác thực + Cấu hình  ─────────────── 14, 15, 00
2. Nhiệm vụ → Sổ tay lãnh đạo → Biên bản họp ──────────── 02, 03, 04
3. Phản ánh người dân ─────────────────────────────────── 09
4. Văn bản & Đơn thư ──────────────────────────────────── 05
5. Giải ngân → Thu – Chi ngân sách ────────────────────── 06, 07
6. Bản đồ kinh tế số ──────────────────────────────────── 10
7. Thông báo, Danh bạ, Nội dung Mini App ──────────────── 08, 12, 11
8. Tổng quan + Báo cáo + xuất file + trình chiếu ──────── 01, 13
9. Tự động hoá (job nền) ──────────────────────────────── 14 §9
```

---

## Ba nguyên tắc phải giữ khi dựng lại

1. **Toàn bộ chuỗi giao diện bằng tiếng Việt**, dùng đúng thuật ngữ hành chính. Các câu dẫn/chú thích trong tài liệu được trích nguyên văn từ prototype — sao chép nguyên vẹn, chúng là một phần của sản phẩm.
2. **Enum là dữ liệu, không phải code.** Trạng thái, lĩnh vực, loại văn bản, hạng mục vốn, nhãn KPI trên báo cáo, thông báo lỗi với người dân — tất cả nằm trong `danh_muc` / `loi_he_thong` và sửa được ở `/cau-hinh`.
3. **ViGov là công cụ theo dõi và điều hành, không phải phần mềm kế toán.** Banner này phải hiện ở màn Giải ngân và đi kèm mọi số liệu API trả về (khoá `budget.scope_notice`).

---

## Những điểm prototype còn dở, cần quyết định khi dựng lại

| Vấn đề | Ghi chú |
|---|---|
| Tìm kiếm toàn hệ thống | Ô tìm có, nhưng chưa trả kết quả. Xem `15-phu-luc` §3.1 để thiết kế. |
| `Quét & OCR` ở Văn bản | Nút có badge `Sắp có`, đang disabled. |
| Trang đăng nhập | Không lộ ra trong prototype; cần bổ sung. Xem `15-phu-luc` §1. |
| Hai tập dữ liệu người dùng lệch nhau | `/danh-ba` có 26 cán bộ tên thật; `Cấu hình → Người dùng` có 12 tài khoản demo. Cần hợp nhất về một bảng `nguoi_dung`. Xem `12-danh-ba-can-bo.md` §7. |
| 3 nhóm bản đồ ngoài danh mục | `coop`, `tourism`, `public_project` đang hard-code; nên đưa hết 11 nhóm vào danh mục. |
| Dòng SLA mồ côi | Lĩnh vực `ve-sinh-moi-truong` hiện mã thô vì danh mục đã đổi mã; cần ràng buộc khoá ngoại. |
| Nhãn nhóm quyền chưa dịch | `ANNOUNCEMENT`, `CONTENT` → `THÔNG BÁO`, `NỘI DUNG MINI APP`. |
| Định dạng số chưa nhất quán | KPI dùng dấu chấm (`10.3%`), bảng dùng dấu phẩy (`10,33%`). Thống nhất về dấu phẩy. |
| Cả 5 job tự động hoá đang tắt | Mặc định tắt là có chủ đích; giữ nguyên. |

---

*Tài liệu lập ngày 16/9/2026 từ bản prototype đang chạy.*
