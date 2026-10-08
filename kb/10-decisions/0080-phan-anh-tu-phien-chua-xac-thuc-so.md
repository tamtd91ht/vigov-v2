---
id: 0080-phan-anh-tu-phien-chua-xac-thuc-so
tier: T1
source: CURATED
owner: domain
derived_from_commit: d77d739a
expires: null
owns_facts:
  - "phiếu phản ánh gửi từ phiên Mini App CHƯA xác thực số: chỉ mở khi Zalo không cho số (công dân từ chối chia sẻ, hoặc Zalo trả lỗi), vẫn mời chia sẻ số Zalo trước; ngoài Zalo vẫn từ chối (chốt 08/10/2026)"
  - "họ tên + số điện thoại tự khai trên phiếu chưa xác thực chỉ là thông tin liên hệ; phiếu KHÔNG gắn danh tính công dân theo số (cong_dan_id rỗng); chủ phiếu là tài khoản Zalo của phiên (chốt 08/10/2026)"
  - "phiếu chưa xác thực theo dõi được bằng mã tra cứu VÀ cùng tài khoản Zalo — không tra cứu công khai, không bao giờ nằm trong 'Phản ánh của tôi' (chốt 08/10/2026)"
  - "không ZNS tới số tự khai chưa xác thực (chốt 08/10/2026)"
  - "web-admin gắn nhãn 'Số tự khai — chưa xác thực' theo một trường tường minh của máy chủ, không suy từ kênh + có công dân (chốt 08/10/2026)"
  - "app riêng của xã: identity phát phiên KHÔNG số (accessToken xác minh bằng secret của app xã) khi Zalo không cho số; phiên ấy chỉ gửi/theo dõi phiếu chưa xác thực (chốt 08/10/2026)"
  - "ngưỡng 10 phiếu chưa xác thực / ngày / tài khoản Zalo (ngưỡng bảo mật người dùng chọn, luật 13) (chốt 08/10/2026)"
  - "phiếu chưa xác thực: có ảnh hiện trường (chủ là tài khoản Zalo), không đánh giá, không mở lại, cán bộ đóng thẳng từ 'đã xử lý' (chốt 08/10/2026)"
  - "'ai' của vết và chủ thể khoá chống gửi trùng (idem) trên phiếu chưa xác thực là mã tài khoản Zalo (chốt 08/10/2026)"
---

# 0080. Phản ánh từ phiên chưa xác thực số — liên hệ tự khai, chủ phiếu là tài khoản Zalo

**Trạng thái:** đã chốt · **Ngày:** 2026-10-08 · **Người quyết:** chủ dự án, 08/10/2026, hai lượt hỏi
trong phiên chính · **Sửa một phần** ADR 0008 (§*Công dân gửi phản ánh* #2 và lý do chống spam / ẩn
danh), ADR 0020 (*"đường duy nhất"*), ADR 0045 (ĐIỀU KIỆN DỪNG #6) — chi tiết ở §*Quan hệ với ADR cũ*.
Mở lại theo **điều kiện #1 của ADR 0020**.

## Bối cảnh

Hôm nay mọi đường gửi phản ánh đòi phiên **đã xác thực số**:

| Chỗ chặn | Nơi |
|---|---|
| Tuyến công dân trả 403 `chua_xac_thuc_so` với phiên chưa có số | `core/httpx/citizen.go`, mã lỗi `chua_xac_thuc_so` (dòng 220 ở `d77d739a`; tệp đang được sửa) |
| Use case gửi từ chối chủ thể không có mã công dân | `service-petitions/internal/app/gui_phan_anh.go:236` |
| Khoá chống gửi trùng từ chối chủ thể `anon` — chung một không gian khoá thì người gửi này nhận mã tra cứu của người khác | `core/idem/idem.go:486-506` |
| App riêng của xã: thiếu `phoneToken` → `phone_required`, không phát phiên | `service-identity/internal/http/routes_cong_dan.go:215` |

Zalo **không cấp** `getPhoneNumber` cho tới khi app được duyệt (`../vigov-require/docs/runbook/zalo-miniapp.md:139-152`).
Ngày 05/10/2026 Zalo **từ chối** bản duyệt app Xã Thăng Bình vì *"tính năng gửi phản ánh chưa hoạt
động"* (`kb/90-ephemeral/tien-do/deploy.json:219`). Vòng khoá chết: không có số thì không gửi được,
không gửi được thì không được duyệt, không được duyệt thì không có số. Đó đúng là điều kiện mở lại #1
của ADR 0020 (*Zalo gắn quyền `getPhoneNumber` vào một ràng buộc mới*).

Đã có sẵn: biểu mẫu có hai ô họ tên / số điện thoại gõ tay tuỳ chọn
(`citizen-app/src/cong-dan/man/GuiPhanAnhScreen.tsx:305-320`), lưu ở `nguoi_gui_ho_ten` /
`nguoi_gui_dien_thoai` (`service-petitions/migrations/0004_phieu_phan_anh.sql:271-272`). Số lưu ở đó
**luôn là số gõ tay**, không bao giờ là số Zalo. Prototype của kho yêu cầu cũng nhận phiếu không công
dân (`../vigov-require/apps/api/app/modules/feedback/router.py:441-465`) và chỉ cho tra theo mã khi
khớp `zalo_user_id` (`:544-558`).

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Khi nào được gửi không số | **Chỉ khi Zalo không cho số**: công dân từ chối chia sẻ, hoặc Zalo trả lỗi. Mời chia sẻ số Zalo **trước**; nhập tay họ tên + số là đường lui, không phải đường chính |
| 2 | Số tự khai là gì | **Thông tin liên hệ**, không phải danh tính. Phiếu **không** gắn danh tính công dân theo số — `cong_dan_id` rỗng. Chủ phiếu = **tài khoản Zalo** của phiên (`tai_khoan_zalo`, `service-identity/migrations/0011_tai_khoan_zalo_va_phien_chua_co_so.sql:97`) |
| 3 | Công dân theo dõi thế nào | Bằng **mã tra cứu VÀ cùng tài khoản Zalo**. Không tra cứu công khai. Phiếu **không bao giờ** hiện trong "Phản ánh của tôi" |
| 4 | Báo tin | **Không ZNS** tới số chưa xác thực. Về cấu trúc: không có `cong_dan_id` thì không có sự kiện outbox (`service-petitions/internal/app/xu_ly_phan_anh.go:1339`) |
| 5 | Web Admin | Nhãn **"Số tự khai — chưa xác thực"**, đọc từ **một trường tường minh** máy chủ trả — không suy từ kênh + có/không công dân |
| 6 | App riêng của xã | Zalo không cho số → identity phát **phiên không số** (`accessToken` xác minh bằng secret của chính app xã, ADR 0066). Phiên ấy **chỉ** được gửi và theo dõi phiếu chưa xác thực |
| 7 | Chống lạm dụng | **10 phiếu chưa xác thực / ngày / tài khoản Zalo** — ngưỡng bảo mật do chủ dự án chọn (luật 13) |
| 8 | Ảnh, đánh giá, mở lại, đóng | **Có ảnh hiện trường** (chủ ảnh là tài khoản Zalo; trigger của `service-petitions/migrations/0026_petition_scene_photo.sql:119-175` phải đổi). **Không đánh giá, không mở lại.** Cán bộ đóng thẳng từ `da-xu-ly` như phiếu cán bộ vào hộ (`service-petitions/internal/domain/xu_ly_phan_anh.go:372-390`); công dân đọc kết quả qua tra cứu |
| 9 | "Ai" của vết và chủ thể idem | **Mã tài khoản Zalo** — ADR 0045:242-244 đã dự liệu đúng trường hợp này |

**Phiên chính chọn (không phải lời chủ dự án), ghi rõ:** **ngoài Zalo vẫn từ chối**. Không có phiên
thì không có xã, và nhận xã từ client là luật 1 cấm #2. Phiếu chưa xác thực không phải cửa công khai —
nó vẫn đi qua một phiên Zalo đã xác minh bằng secret của app.

## Vì sao chủ phiếu là tài khoản Zalo, không phải số tự khai

Gắn phiếu vào danh tính theo số gõ tay là để **ai gõ số của người khác thì đọc được phiếu của người
ấy** — và ngược lại, chủ số thật mở "Phản ánh của tôi" thấy một phiếu mình không gửi. Hỏng đúng luật 4
cả hai chiều. Tài khoản Zalo thì đã được Zalo xác minh qua `accessToken`: yếu hơn số đã xác thực, nhưng
**là thật**, còn số gõ tay thì không chứng minh gì.

Tài khoản Zalo khoá theo (`app_id`, `zalo_user_id`) — gửi qua app chung thì không theo dõi được từ app
riêng của xã, và ngược lại. Đó là hệ quả của khoá, không phải lỗi.

**Vì sao nhãn là một trường máy chủ, không suy ra:** phiếu cán bộ vào hộ cũng có `cong_dan_id` rỗng,
và một kênh mới mai sau cũng vậy. Quy tắc suy luận ở giao diện là một bản sao thứ hai của sự thật
"phiếu này có danh tính hay không" — lệch ngày có kênh thứ ba (luật 9).

## Cái phải trả

| # | Khoản | Mức |
|---|---|---|
| 1 | **Truy trách nhiệm yếu hơn ADR 0008.** ADR 0008:98-100 nói ẩn danh vẫn lưu danh tính để chống spam và để phiếu vu khống không vô danh tuyệt đối. Phiếu chưa xác thực chỉ có tài khoản Zalo — truy được tới một tài khoản, **không** tới một số đã xác minh | Chấp nhận có ý thức; ngưỡng #7 là phần bù |
| 2 | **Khoảng trống luật 10 bất biến 5.** Phiếu này không nhận lời báo nào khi chuyển trạng thái (ADR 0041) — công dân phải tự mở tra cứu mới biết | Chấp nhận. *Đề xuất, chưa chốt:* lúc gửi, giao diện nói rõ với công dân rằng họ sẽ không được báo |
| 3 | **Số tự khai là dữ liệu cá nhân** dù chưa xác thực. Luật 3 không đổi: che khi ra API, không log, không vào vết | Không nới |
| 4 | Ngưỡng 10/ngày là ngưỡng bảo mật; nới hay siết về sau là điều kiện dừng của luật 13 | — |
| 5 | Phiếu chưa xác thực **không** chuyển thành phiếu có danh tính khi tài khoản Zalo về sau xác thực số. Làm vậy là "nhận hồ sơ cũ" — ADR 0020 CÒN MỞ #1 | Không đụng |

## Còn mở — chưa quyết

| # | Câu | Vì sao không tự chọn |
|---|---|---|
| 1 | Luật 6 bất biến 8 đòi "ai" là **mã nghiệp vụ**, nhưng `tai_khoan_zalo.id` là ULID (`0011_…sql:98-100`, ghi chú cột nói nó chính là "ai" của vết). Điểm 9 có cần một mã nghiệp vụ riêng cho tài khoản Zalo không | Đổi hình dạng một khoá đã dùng ở vết phiên; người dùng chốt |
| 2 | Phiếu chưa xác thực có vào thống kê / báo cáo lên trên chung với phiếu có danh tính, hay tách dòng | Là cách xã báo cáo, không phải thiết kế phần mềm |

## Quan hệ với ADR cũ

| ADR | Phần bị sửa | Phần giữ nguyên |
|---|---|---|
| 0008 | §*Công dân gửi phản ánh* #2 (`:78`) và lý do `:95-107`: gửi được qua phiên Mini App **chưa** xác thực số, trong phạm vi điểm 1 | #1 (phải có phiên, không có endpoint ghi công khai), cờ `an_danh`, luật đóng phiếu |
| 0020 | *"đường duy nhất"* (`:50`): ở kênh Mini App có thêm đường **gửi phản ánh** không số; **đăng nhập có danh tính** vẫn chỉ qua `getPhoneNumber` | Mọi bất biến, CÒN MỞ #1 |
| 0045 | ĐIỀU KIỆN DỪNG #6 (`:336`): một tuyến **được** nhận phiên chưa có số — gửi và theo dõi phiếu chưa xác thực của chính tài khoản Zalo ấy | Mặc định từ chối phiên chưa có số trên mọi tuyến khác; tuyến mới vẫn phải khai tường minh |

## ĐIỀU KIỆN DỪNG

1. Gắn phiếu chưa xác thực vào `dinh_danh_cong_dan` theo số tự khai, hoặc hiện nó trong "Phản ánh của tôi"
2. Gửi ZNS / tin bất kỳ tới số tự khai
3. Mở đường gửi không số **ngoài** Zalo, hoặc không qua phiên
4. Mở thêm hành vi cho phiên không số ngoài gửi / theo dõi phiếu chưa xác thực
5. Đổi ngưỡng 10 phiếu / ngày / tài khoản Zalo
6. Tra cứu theo mã mà không đối chiếu tài khoản Zalo, hoặc trả 404 khác 403 cho phiếu người khác (luật 4 cấm #2)

→ ADR 0008 · 0020 · 0045 · 0041 (bảng báo) · 0066 (đăng nhập app riêng của xã) · 0050 (đánh giá, mở lại)
→ Luật 1 cấm #2 · luật 3 · luật 4 · luật 6 bất biến 8 · luật 10 bất biến 5 · luật 13
