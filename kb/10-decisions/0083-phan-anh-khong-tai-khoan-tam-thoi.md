---
id: 0083-phan-anh-khong-tai-khoan-tam-thoi
tier: T1
source: CURATED
owner: domain
derived_from_commit: 29a1b904
expires: null
owns_facts:
  - "phiếu phản ánh KHÔNG TÀI KHOẢN (không phiên, không tài khoản Zalo): đường TẠM cho tới khi App ViHAT qua duyệt Zalo; luôn bật, gỡ bằng một lần sửa mã (chốt 08/10/2026)"
  - "xã của phiếu không tài khoản: tên miền xã trên QR gửi trong thân, máy chủ tra ra đúng một xã đang hoạt động, không ra thì 404; không bao giờ nhận tenant_id — ngoại lệ có tên của luật 1 cấm #2 (chốt 08/10/2026)"
  - "ngưỡng phiếu không tài khoản: 5 phiếu / giờ / IP và 200 phiếu / ngày / xã, vượt thì 429 (ngưỡng bảo mật người dùng chọn, luật 13) (chốt 08/10/2026)"
  - "tra cứu phiếu không tài khoản: công khai theo mã tra cứu, chỉ trả tình trạng · hạn · kết quả trả lời; không họ tên, số, nội dung, địa chỉ; mã sai và mã của xã khác cùng một 404 (chốt 08/10/2026)"
  - "phiếu không tài khoản không nhận ảnh hiện trường (chốt 08/10/2026)"
  - "biểu mẫu phiếu không tài khoản như phiếu thường, có chọn lĩnh vực qua danh mục đọc công khai theo tên miền xã (chốt 08/10/2026)"
  - "tra cứu công khai phiếu không tài khoản: 30 lần / giờ / IP (ngưỡng bảo mật người dùng chọn, luật 13) (chốt 08/10/2026)"
  - "chống gửi trùng phiếu không tài khoản: khoá ngẫu nhiên 128 bit mỗi lần soạn, máy chủ nhớ theo (xã, khoá) 24 giờ (chốt 08/10/2026)"
  - "phiếu không tài khoản không có cột đánh dấu: suy ra ở một hàm của service-petitions, trả trường tường minh accountless (chốt 08/10/2026)"
---

# 0083. Phản ánh không tài khoản — đường tạm khi App ViHAT chưa được Zalo duyệt

**Trạng thái:** đã chốt · **Ngày:** 2026-10-08 · **Người quyết:** chủ dự án, 08/10/2026, hai lượt hỏi
trong phiên chính, kèm lời *"nới lỏng điều luật đi, sẽ chặn lại khi app review xong, chứ không thì không
kịp"*. **Sửa một phần** ADR 0080 (ĐIỀU KIỆN DỪNG #3 và #6) và ADR 0008 §*Công dân gửi phản ánh* #1 — chi
tiết ở §*Quan hệ với ADR cũ*. Đây là **đường tạm**, không phải thiết kế đích.

## Bối cảnh

ADR 0080 mở đường gửi **không số**, nhưng vẫn đòi **tài khoản Zalo** lấy từ `getAccessToken`. Đo trên
máy thật ngày 08/10/2026 (`kb/90-ephemeral/tien-do/citizen-app.json`, mục `do-access-token-1401-app-chung`):
App ViHAT mở bằng QR xã, cả khi đồng ý lẫn từ chối chia sẻ tên, `getAccessToken` đều trả **-1401** —
App ViHAT chưa được Zalo duyệt nên chưa có quyền. Nghĩa là trên app chung, **không đường nào** gửi được
phản ánh cho tới khi được duyệt, kể cả đường của ADR 0080.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Khi nào dùng | App chỉ đi đường này khi **không lấy được tài khoản Zalo** (`getAccessToken` hỏng). Còn lấy được thì đi ADR 0080 hoặc đường có số như cũ |
| 2 | Phiếu thuộc xã nào | Thân gửi **tên miền xã trên QR** (`host`). Máy chủ tra ra **đúng một xã đang hoạt động** như các tuyến đọc công khai; không ra thì **404**. Không bao giờ nhận `tenant_id` |
| 3 | Chống lạm dụng | **5 phiếu / giờ / IP** và **200 phiếu không tài khoản / ngày / xã**; vượt thì **429** kèm câu chỉ bộ phận tiếp nhận của xã |
| 4 | Theo dõi | **Tra cứu công khai theo mã tra cứu**: chỉ trả tình trạng, hạn, kết quả trả lời. **Không** trả họ tên, số điện thoại, nội dung, địa chỉ. Mã sai và mã của xã khác cùng một **404**. Có giới hạn tần suất chống dò mã |
| 5 | Ảnh hiện trường | **Không nhận** |
| 6 | Thời hạn | **Luôn bật**, không công tắc. Gỡ bằng một lần sửa mã khi App ViHAT qua duyệt |
| 8 | Biểu mẫu | **Như phiếu thường** — chỉ khác chủ phiếu (lời chủ dự án: *"nó chỉ khác nhau author thôi"*). Có chọn lĩnh vực: danh mục lĩnh vực đọc **công khai theo tên miền xã**, cùng nội dung tuyến có phiên. Thêm một tuyến đọc công khai này là phần của quyết định, không phạm ĐIỀU KIỆN DỪNG #1 |
| 9 | Chống tra cứu dò mã | **30 lần / giờ / IP** (ngưỡng bảo mật người dùng chọn, luật 13) |
| 10 | Gửi trùng | App sinh **một khoá ngẫu nhiên 128 bit cho mỗi lần soạn**; máy chủ nhớ theo (xã, khoá) 24 giờ — gửi lại cùng khoá nhận lại đúng mã cũ, không tạo phiếu thứ hai |
| 11 | Đánh dấu | **Không migration.** Phiếu kênh `zalo-mini-app` không có công dân lẫn tài khoản Zalo = phiếu không tài khoản; quy tắc nằm ở **đúng một hàm** của `service-petitions`, máy chủ trả trường tường minh `accountless` cho Web Admin |
| 12 | Chọn của phiên chính (không phải lời chủ dự án) | "Ai" của vết: loại `anonymous`, IP ẩn trên màn vết như vết công dân. 5/giờ/IP đếm theo (xã, IP) trên Redis, Redis hỏng thì từ chối (503). 200/ngày/xã đếm trong CSDL từ 00:00 giờ Việt Nam, tính cả phiếu đã xoá mềm (như ADR 0080). Tra cứu trả tình trạng, `acknowledge_due`, `resolve_due`, kết quả và lý do. Thân 201 chỉ gồm mã tra cứu và các trường của tra cứu |
| 7 | Phần giữ của ADR 0080 | Họ tên + số tự khai là **thông tin liên hệ**, không phải danh tính; **không ZNS**; không đánh giá, không mở lại, cán bộ đóng thẳng từ `da-xu-ly`; Web Admin gắn nhãn từ **một trường tường minh** của máy chủ |

## Vì sao luật 1 cấm #2 được nới ở đây, và chỉ ở đây

Luật 1 cấm #2 tồn tại để client không **tự cấp quyền đọc** một xã. Ở tuyến này người gửi chỉ chọn được
xã mình **gửi tới**, không đọc được gì của xã ấy — giống người dân bước vào UBND xã nào thì nộp đơn ở xã
ấy. Tên miền chỉ là khoá tra; xã vẫn do **máy chủ** quyết, và xã không tồn tại hay đã ngừng thì không ghi
gì. Mọi tuyến khác giữ nguyên luật 1.

## Cái phải trả

| # | Khoản | Mức |
|---|---|---|
| 1 | **Phiếu không truy được tới ai** — không số đã xác thực, không tài khoản Zalo. Yếu hơn cả ADR 0080 #1. Phiếu vu khống là vô danh tuyệt đối | Chủ dự án chấp nhận vì đường tạm |
| 2 | **Ai có tên miền xã cũng gửi được** (tên miền in trên QR công khai), kể cả ngoài Zalo — app không chứng minh được nó đang chạy trong Zalo | Chỉ ngưỡng #3 chặn |
| 3 | **Nhiều người dùng chung một IP** (mạng di động, wifi UBND): 5 phiếu/giờ/IP có thể chặn nhầm người thật | Chấp nhận; 429 chỉ đường bộ phận tiếp nhận |
| 4 | **Ai cầm mã tra cứu thì xem được tình trạng phiếu** (mã bị chụp, bị chuyển tiếp) | Rút gọn #4 là phần bù: không lộ dữ liệu cá nhân |
| 5 | **Luôn bật** — quên gỡ thì đường tạm thành đường vĩnh viễn | Chỗ gỡ ghi ở §*Gỡ bỏ* |
| 6 | **Không báo tin** (luật 10 bất biến 5) — như ADR 0080 #2 | Chấp nhận |

## Gỡ bỏ

Khi App ViHAT qua duyệt Zalo và `getAccessToken` chạy được trên máy thật: gỡ tuyến gửi và tuyến tra cứu
không tài khoản, gỡ nhánh gọi chúng ở `citizen-app`. Phiếu đã nhận **giữ nguyên** (luật 7), vẫn tra được
cho tới khi đóng. Ghi ngày gỡ vào ADR này.

## Quan hệ với ADR cũ

| ADR | Phần bị sửa | Phần giữ nguyên |
|---|---|---|
| 0080 | ĐIỀU KIỆN DỪNG #3 (*"không qua phiên"*) và #6 (*"tra cứu không đối chiếu tài khoản Zalo"*) — chỉ cho phiếu không tài khoản, trong phạm vi bảng trên | Mọi quyết định khác; phiếu chưa xác thực số có tài khoản Zalo vẫn theo ADR 0080 |
| 0008 | §*Công dân gửi phản ánh* #1 (*"không có endpoint ghi công khai"*) — một endpoint ghi công khai tạm thời | Cờ `an_danh`, luật đóng phiếu |

## ĐIỀU KIỆN DỪNG

1. Mở thêm tuyến không xác thực nào khác ngoài gửi · tra cứu · danh mục lĩnh vực, hoặc cho tra cứu đọc thêm trường nào ngoài #4/#12
2. Nhận `tenant_id`, hoặc tên miền của một xã ngừng hoạt động
3. Đổi ngưỡng #3
4. Nhận ảnh hoặc tệp
5. Gắn phiếu không tài khoản vào một danh tính công dân hay tài khoản Zalo về sau
6. Giữ đường này sau khi App ViHAT đã qua duyệt

→ ADR 0080 · 0008 · 0041 · 0047 (tên miền là khoá tra)
→ Luật 1 cấm #2 · luật 3 · luật 4 · luật 10 bất biến 5 · luật 13
