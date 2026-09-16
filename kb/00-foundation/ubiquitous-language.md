---
id: ubiquitous-language
tier: T0
source: CURATED
owner: domain
derived_from_commit: null
expires: null
owns_facts:
  - "ánh xạ thuật ngữ hành chính sang tên dùng trong mã"
  - "phân biệt phản ánh, khiếu nại, tố cáo"
  - "tên gọi các bước trong vòng đời phiếu phản ánh"
  - "tên gọi vai trò cán bộ cấp xã"
---

# Ngôn ngữ chung

**Bảng duy nhất** ánh xạ thuật ngữ hành chính sang tên trong mã. Đặt tên sai ở schema thì
mọi tầng trên đều sai theo, và sửa về sau là di trú dữ liệu.

## Ba thuật ngữ khác nhau về pháp lý — hay bị dùng lẫn

| Thuật ngữ | Nghĩa | Tên trong mã | Hệ quả nếu gọi nhầm |
|---|---|---|---|
| **Phản ánh, kiến nghị** | Người dân nêu vấn đề, đề xuất | `phan_anh` | — |
| **Khiếu nại** | Không đồng ý với **quyết định hành chính** cụ thể | `khieu_nai` | Áp sai thủ tục và **sai thời hạn luật định** |
| **Tố cáo** | Báo hành vi vi phạm pháp luật | `to_cao` | Mất **bảo vệ người tố cáo** |

## Bảng thuật ngữ

| Tiếng Việt hành chính | Tên trong mã | Ghi chú |
|---|---|---|
| Văn bản đến | `van_ban_den` | Từ nơi khác gửi tới xã |
| Văn bản đi | `van_ban_di` | Xã ban hành gửi đi |
| Số đến / số đi | `so_den` / `so_di` | Đánh số **theo từng cơ quan**, lại từ 01 mỗi năm |
| Thụ lý | `thu_ly` | Nhận và bắt đầu xử lý — không dùng "xử lý" |
| Luân chuyển | `luan_chuyen` | Chuyển giữa các bộ phận — không dùng "chuyển" |
| Ban hành | `ban_hanh` | Ký và phát hành chính thức |
| Hồ sơ một cửa | `ho_so_mot_cua` | Không viết "1 cửa" |
| Đơn thư | `don_thu` | Bao gồm khiếu nại, tố cáo, kiến nghị |
| Giải ngân | `giai_ngan` | |
| Nhiệm vụ | `nhiem_vu` | Việc giao cho cán bộ |
| Cán bộ, công chức | `can_bo` | Người dùng nội bộ |
| Công dân | `cong_dan` | Người dân — **không** gọi là "khách hàng", "user" |
| Đơn vị hành chính | `don_vi_hanh_chinh` | Xã / phường / thị trấn |

## Phản ánh và SLA

Vòng đời đầy đủ ở `skills/petition-lifecycle`; bảng này chỉ chốt **từ dùng**.

| Tiếng Việt hành chính | Tên trong mã | Ghi chú |
|---|---|---|
| Phiếu phản ánh | `phieu_phan_anh` | Một lượt phản ánh, có **mã tra cứu** trả cho dân |
| Mã tra cứu | `ma_tra_cuu` | Không tuần tự, không tái cấp (luật 4, 7) |
| Lĩnh vực | `linh_vuc` | Rác thải, giao thông, trật tự đô thị… — **cấu hình theo xã** |
| Tiếp nhận | `tiep_nhan` | **Mốc bắt đầu đếm hạn** — không phải lúc phân công |
| Phân loại | `phan_loai` | Cán bộ xác định lĩnh vực — **không** để dân tự chọn |
| Phân công | `phan_cong` | Giao cán bộ/đơn vị xử lý |
| Nghiệm thu | `nghiem_thu` | Xác nhận đã xử lý trên thực địa, thường kèm ảnh |
| Đóng phiếu | `dong_phieu` | Kết thúc — **bắt buộc có kết quả dân đọc được** |
| Hạn xử lý | `sla_deadline` | Lưu **một lần** lúc tiếp nhận, tính bằng **ngày làm việc** |
| Quá hạn | — | **Suy ra**, không có cột. Xem luật 10 |

## Vai trò cán bộ

| Tiếng Việt | Tên trong mã | Việc thật ở xã |
|---|---|---|
| Lãnh đạo | `lanh_dao` | Chủ tịch / Phó chủ tịch — **duyệt**, không tự xử lý |
| Chuyên viên | `chuyen_vien` | Công chức xử lý chuyên môn |
| Kế toán | `ke_toan` | Giải ngân |
| Tiếp nhận một cửa | `tiep_nhan` | Cán bộ tiếp dân |
| Quản trị | `quan_tri` | Phụ trách kỹ thuật của xã |

## Quy ước đặt tên

| Loại | Quy ước | Ví dụ |
|---|---|---|
| Bảng, cột | `snake_case` không dấu, theo nghiệp vụ | `don_thu`, `ngay_tiep_nhan` |
| Kiểu Go | `PascalCase`, tiếng Anh nếu là khái niệm kỹ thuật | `DonThu`, `TenantID` |
| Sự kiện | `<miền>.<việc đã xảy ra>.<phiên bản>` | `donthu.da_tiep_nhan.v1` |
| Service | tiếng Việt không dấu, theo nghiệp vụ | `donthu`, `vanban` |

**Không trộn tiếng Anh vào khái niệm nghiệp vụ.** `feedback` không phải `phan_anh`:
"feedback" gợi ý góp ý sản phẩm, "phản ánh" là một loại đơn có quy trình hành chính.

→ Kỹ năng: `skills/administrative-language`
