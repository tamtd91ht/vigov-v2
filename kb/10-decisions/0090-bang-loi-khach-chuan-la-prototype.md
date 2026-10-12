---
id: 0090-bang-loi-khach-chuan-la-prototype
tier: T1
source: CURATED
owner: domain
derived_from_commit: 3421992a
expires: null
owns_facts:
  - "luật xử lý bảng lỗi của khách 10–11/10/2026: chuẩn là prototype — mọi tính năng prototype có thì web-admin phải có; tính năng prototype không có mà bảng lỗi đòi gỡ thì GIỮ khi nó là đường duy nhất tới một khả năng; luật cứng thắng cả bảng lỗi lẫn prototype (chủ dự án 10–11/10/2026)"
  - "bảy tính năng ngoài prototype được GIỮ ngày 11/10/2026 dù bảng lỗi đòi gỡ, và khả năng mỗi tính năng là đường duy nhất tới"
  - "những dòng bảng lỗi 10/10/2026 bị từ chối vì va luật cứng: phiên 7 ngày, ô mật khẩu tự đặt, hiện đủ SĐT, nội dung phản ánh và số giờ trễ trên Tổng quan"
  - "xoá trên web-admin theo bảng lỗi 10/10/2026: ẩn ô lý do, máy chủ ghi câu cố định nói rõ người xoá không nhập lý do; vẫn xoá mềm và có vết cùng giao dịch — mở rộng tiền lệ ADR 0075 #4a / ADR 0077"
  - "quy ước đánh số dòng của bảng lỗi khách 10/10/2026 trong kb/ và trong chú thích mã"
---

# 0090. Bảng lỗi của khách 10–11/10/2026: chuẩn là prototype, tính năng dư giữ khi là đường duy nhất

**Trạng thái:** đã chốt · **Ngày:** 2026-10-11 · **Người quyết:** chủ dự án, 10/10 và 11/10/2026.
Lời của chủ dự án do phiên chính chuyển lại; kho không có phiếu hỏi để đối chiếu nguyên văn · **Bổ
sung** ADR 0068 §*Sửa đổi 06/10/2026 (lần 5)* #3–#4 · Áp dụng cụ thể ở mục *Sửa đổi 10–11/10/2026*
của ADR 0053, 0059, 0062, 0076, 0077, 0079, 0084.

## Bối cảnh

Ngày 10/10/2026 khách gửi bảng lỗi 61 dòng (ngoài git). Ngoài lỗi thật, bảng lỗi có hai loại dòng
mà "sửa theo bảng" sẽ sai:

- dòng đòi **gỡ** một thứ prototype không có (chữ "Dư …"), trong khi thứ ấy là cách duy nhất để làm
  một việc;
- dòng đòi một thứ **va luật cứng**: dữ liệu cá nhân, phiên, mật khẩu, hạn giờ làm việc.

**Đánh số dòng.** "Dòng N" trong `kb/` và trong chú thích mã là số đứng đầu mỗi dòng của bản văn bản
xuất từ bảng lỗi. Dòng 0 là dòng tiêu đề. Số này **không** trùng cột TT: từ dòng 6 trở đi TT lệch,
và các dòng 46–61 không có TT. Bảng tính gốc đếm cả dòng tiêu đề, nên số của nó lớn hơn một (dòng 49
ở đây là dòng 50 trên bảng tính).

Lời chủ dự án, do phiên chính chuyển lại:

| Ngày | Lời |
|---|---|
| 10/10 | "so prototype đi" · "theo prototype" · "bảo mật dữ liệu cá nhân là bắt buộc" · về xoá: "ẩn ô, máy ghi câu cố định" |
| 11/10 | "chuẩn là prototype, tính năng dư thừa đã confirm có thể không cần gỡ, nhưng tính năng trên prototype phải được đảm bảo" |

## Quyết định

| # | Luật |
|---|---|
| 1 | Prototype (`../vigov-require/apps/admin`) có tính năng nào thì web-admin **phải có** tính năng ấy. Thiếu là lỗi |
| 2 | Bảng lỗi đòi gỡ một tính năng prototype không có: **giữ** nếu nó là đường duy nhất tới một khả năng. Nếu không, gỡ hoặc ẩn theo prototype |
| 3 | Luật cứng (ADR 0068 lần 5 #4 và các luật trong `.claude/rules/critical/`) thắng cả bảng lỗi lẫn prototype |
| 4 | Xoá: **ẩn ô lý do**. Để trống thì máy chủ ghi một câu cố định vào `delete_reason`, câu ấy nói rõ người xoá không nhập lý do. Vẫn xoá mềm, vết kiểm toán ghi cùng giao dịch |

Cách đọc #2 ("đường duy nhất tới một khả năng") là của phiên chính. Phiên chính yêu cầu ghi nó làm
luật ngày 11/10/2026.

### Giữ dù bảng lỗi đòi gỡ (11/10/2026)

| Tính năng | Dòng | Gỡ đi thì mất |
|---|---|---|
| Ba màn Lịch làm việc · Nhật ký hệ thống · Nhận diện xã — **từ 12/10/2026 không còn là tab Cấu hình** mà ở mục menu riêng "Hệ thống" (`/he-thong`), xem *Sửa đổi 12/10/2026* | 28 | Chỗ duy nhất nhập giờ làm việc, ngày lễ, ngày làm bù, tức đầu vào của mọi hạn tính theo giờ làm việc (ADR 0007). Chỗ duy nhất đọc vết kiểm toán (ADR 0054). Chỗ duy nhất đặt tên và logo xã (ADR 0069) |
| Cụm Thời hạn đơn thư ở tab Thời hạn xử lý | 35 | Số ngày theo loại đơn, nguồn của hạn đơn thư tự điền (ADR 0084 §3) |
| Tab Văn bản đi / Văn bản đến | 42 | Sổ văn bản đến và đi chỉ có ở đây |
| Sửa hạn xử lý (chi tiết đơn thư) | 43 | Cách duy nhất đặt hoặc sửa hạn sau khi vào sổ (ADR 0079 Q18, ADR 0084 §3) |
| Sửa thông tin bảng (Thu – Chi) | 47 | Cách duy nhất đổi tiêu đề và cột của bảng ngân sách mà không nạp lại. Đây cũng là đường sửa dòng 49, 54, 55 |
| Xuất Excel ở Sổ theo dõi nhiệm vụ | 15 | Đường duy nhất lấy sổ theo dõi ra tệp (ADR 0076 #13) |
| + Thêm việc con | 17 | Đường duy nhất trên màn để tạo việc con (ADR 0076 #14, ADR 0037) |

Theo #1, mục "Có thể trùng với phiếu khác" ở chi tiết phản ánh (dòng 60 đòi gỡ) cũng **giữ**: prototype
có mục này (`web-admin/src/features/phan-anh/petition-merge.tsx:24`, ADR 0087).

### Từ chối vì va luật cứng

| Dòng | Bảng lỗi đòi | Giữ | Chủ sở hữu |
|---|---|---|---|
| 1 | Nhớ đăng nhập, làm mới phiên 7 ngày | Phiên cán bộ tối đa 12 giờ | Câu mở #18 (DECIDED 22/09/2026) · `skills/session-and-token` |
| 41 | Ô Mật khẩu trên popup thêm người dùng | Mật khẩu tạm do hệ thống cấp, trả một lần | Câu mở #9 · ADR 0059 §1 |
| 43 | Hiện đủ số điện thoại ở chi tiết đơn thư | Số điện thoại vẫn che | Luật 3 · ADR 0084 #1 |
| 5 | Dòng "Cần xử lý ngay" có nội dung, có "quá hạn … ngày … giờ" | Dòng nhiệm vụ có tên nhiệm vụ. Dòng phản ánh chỉ có mã. Không hiện số giờ trễ | Luật 3 · luật 10 cấm #2 · ADR 0053 §Sửa đổi 10–11/10/2026 |

## Hệ quả

- **Ai đối chiếu bảng lỗi với web sẽ thấy dòng chưa làm.** Các dòng ở hai bảng trên được bỏ qua có chủ
  ý; đọc tệp này trước khi mở lại chúng.
- **`delete_reason` không còn phân biệt được lý do.** Mọi lần xoá không nhập lý do mang cùng một câu
  (ADR 0077 §*Cái giá* đã nêu cho hạng mục). Người tra vết biết *ai* xoá, *khi nào*, không biết *vì sao*.
- **Một luật cứng bị thu hẹp.** ADR 0068 lần 5 #4 ghi "cán bộ **không** nhập đánh giá thay công dân
  (ADR 0062)". Dòng 60 đòi khối "Ghi nhận đánh giá của người dân" của prototype, nên ADR 0062 đã
  được đổi (**ĐÃ LÀM** ở `f33dd452` + `3421992a`, xem §*Sửa đổi* của ADR 0062). Phần giữ: điểm cán bộ
  ghi **không** cộng vào điểm hài lòng. Còn treo ở đó: 1–2 sao cán bộ ghi chưa tự mở lại phiếu.
- **Bảng lỗi va quyết định cũ.** Thứ tự xử lý vẫn như lần 5: luật cứng trước, rồi prototype, rồi quyết
  định trình bày cũ. Mỗi ADR bị đảo ghi riêng điểm bị đảo trong mục *Sửa đổi 10–11/10/2026* của nó.

→ ADR 0068 lần 5 #3–#4 · ADR 0075 #4a · ADR 0077 · `kb/90-ephemeral/tien-do/` (mục `*-10-11` của từng
module)

## Sửa đổi 12/10/2026 — bảng lỗi mở lại

Chủ dự án chốt hai dòng khách mở lại:

| Dòng | Quyết định | Lý do |
|---|---|---|
| 28 (S27) | `/cau-hinh` chỉ còn 9 tab đúng prototype. Ba màn Lịch làm việc · Nhật ký hệ thống · Nhận diện xã **chuyển** sang mục menu "Hệ thống" (`/he-thong`, nhóm Quản trị, ngay sau Cấu hình). Không gỡ | Vẫn theo #2: là đường duy nhất tới ADR 0007 · 0054 · 0069. Chuyển chỗ thoả cả prototype lẫn #2. Cổng từng màn giữ nguyên; link `/cau-hinh?tab=` cũ tự chuyển (`moved-tabs.ts`) |
| 36 (S35) | Câu mô tả Thời hạn xử lý dùng nguyên văn prototype "hồ sơ **tiếp nhận** sau thời điểm lưu" (thay "được đặt hạn" của 08/10). Câu cuối vẫn là "Mặc định {n} giờ." lấy từ dữ liệu xã | "tiếp nhận" hiểu theo ADR 0028 §(c). Con số "72 giờ, tức ba ngày" không viết cứng (luật 10 cấm #3; giờ làm việc ≠ ngày lịch) |
