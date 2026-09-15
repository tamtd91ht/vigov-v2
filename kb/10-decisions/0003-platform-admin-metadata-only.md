---
id: 0003-platform-admin-metadata-only
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "quyền của quản trị viên nhà cung cấp và ranh giới đọc dữ liệu nghiệp vụ"
---

# 0003. Quản trị nhà cung cấp chỉ thao tác siêu dữ liệu

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15 · **Đóng câu hỏi mở #3**

## Bối cảnh

Hệ thống có một web quản trị tổng do **VIHAT** (nhà cung cấp) vận hành, quản lý nhiều xã.
Câu hỏi: nhà cung cấp có được đọc dữ liệu nghiệp vụ của xã không?

Đây là câu hỏi pháp lý và niềm tin, không phải câu hỏi kỹ thuật. Một cơ quan nhà nước giao
dữ liệu công dân cho nền tảng của bên thứ ba cần biết ranh giới đó nằm ở đâu.

## Quyết định

**Không.** Quản trị nhà cung cấp chỉ thao tác **siêu dữ liệu**.

| Được | Không được |
|---|---|
| Tạo, khoá, đổi tên, gán tên miền cho xã | Đọc nội dung hồ sơ, đơn thư, văn bản |
| Hạn mức, gói dịch vụ, tình trạng hoạt động | Đọc dữ liệu cá nhân công dân |
| Nhật ký hệ thống, sức khoẻ dịch vụ | Đọc nhật ký thao tác nghiệp vụ của xã |
| Số đếm tổng hợp nhận **qua sự kiện** | Truy vấn thẳng vào CSDL nghiệp vụ |

## Cưỡng chế bằng kiến trúc, không bằng quyền

`nentang` **không có gRPC client** tới bất kỳ service nghiệp vụ nào để đọc nội dung. Nó nhận
số đếm qua sự kiện. Nhà cung cấp không đọc được **vì không có đường**, chứ không phải vì có
một cờ quyền đang tắt — cờ thì bật được, đường thì phải viết thêm mã và sẽ bị rà thấy.

## Nếu sau này cần hỗ trợ kỹ thuật chạm dữ liệu thật

Phải là **phiên hỗ trợ do xã cấp**: có thời hạn, phạm vi hẹp, ghi vết đầy đủ, và **xã nhìn
thấy được** phiên đó. Không bao giờ là quyền thường trực.

## Hệ quả

- Web quản trị tổng và web quản trị xã là **hai ứng dụng khác nhau**, không phải một ứng dụng
  đổi vai trò
- Hỗ trợ sự cố sẽ khó hơn: không nhìn được dữ liệu thật thì phải dựa vào nhật ký hệ thống và
  mô tả của xã. **Đây là cái giá đã chấp nhận.**
