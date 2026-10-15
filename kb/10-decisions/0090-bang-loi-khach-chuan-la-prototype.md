---
id: 0090-bang-loi-khach-chuan-la-prototype
tier: T1
source: CURATED
owner: domain
derived_from_commit: f81168fe
expires: null
owns_facts:
  - "luật xử lý bảng lỗi của khách 10–11/10/2026: chuẩn là prototype — mọi tính năng prototype có thì web-admin phải có; tính năng prototype không có mà bảng lỗi đòi gỡ thì GIỮ khi nó là đường duy nhất tới một khả năng; luật cứng thắng cả bảng lỗi lẫn prototype (chủ dự án 10–11/10/2026)"
  - "bảy tính năng ngoài prototype được GIỮ ngày 11/10/2026 dù bảng lỗi đòi gỡ, và khả năng mỗi tính năng là đường duy nhất tới"
  - "những dòng bảng lỗi 10/10/2026 bị từ chối vì va luật cứng: phiên 7 ngày, ô mật khẩu tự đặt, hiện đủ SĐT, nội dung phản ánh và số giờ trễ trên Tổng quan"
  - "xoá trên web-admin theo bảng lỗi 10/10/2026: ẩn ô lý do, máy chủ ghi câu cố định nói rõ người xoá không nhập lý do; vẫn xoá mềm và có vết cùng giao dịch — mở rộng tiền lệ ADR 0075 #4a / ADR 0077"
  - "quy ước đánh số dòng của bảng lỗi khách 10/10/2026 trong kb/ và trong chú thích mã"
  - "chi tiết đơn thư theo bảng lỗi 13/10/2026: ô Trả lời công dân giữ khoá trước Thụ lý và sau khi kết thúc, câu gợi ý chỉ đúng nút trên màn; nút Sửa hạn xử lý giữ; số điện thoại vẫn che (§Sửa đổi 13/10/2026)"
  - "thứ tự mặc định 'mới thêm lên đầu' của danh sách Người dùng, Dự án giải ngân, nhóm/liên kết Zalo, Sổ địa điểm (trong nhóm loại), và các danh sách CỐ Ý giữ thứ tự khác (chủ dự án 15/10/2026, §Sửa đổi 15/10/2026)"
  - "bốn khoá cấu hình riêng nhóm Quản trị — admin.village, admin.message, admin.mail, admin.zalo — tách khỏi admin.lookup / admin.org, cấp sẵn theo khoá cũ; lệch prototype có chủ ý; đảo lần chốt 'không thêm khoá' cùng ngày (chủ dự án 15/10/2026, bảng lỗi web-admin-141026 dòng 22, §Sửa đổi 15/10/2026)"
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

## Sửa đổi 13/10/2026 — bảng lỗi web-admin-131026, chi tiết đơn thư

Chủ dự án chốt 13/10/2026, lời do phiên chính chuyển lại. Số dòng là dòng của bảng lỗi 13/10. Bảng lỗi này
cũng đổi Tự động hoá và Danh mục (ADR 0079 §*Sửa đổi 13/10/2026*) và vai trò cán bộ (ADR 0094).

| Điểm | Quyết định | Commit |
|---|---|---|
| Ô "Trả lời công dân" | **Giữ khoá** trước Thụ lý và sau khi đơn kết thúc (thứ tự TT 05/2021, C3). Câu gợi ý chỉ đúng nút **có trên màn**: ô "Đang xử lý" trên thanh trạng thái rồi "Xác nhận" để thụ lý; đơn đã kết thúc thì nói không sửa được nữa. Lý do: ba trạng thái thật cùng vẽ thành một ô "Đang xử lý" (ADR 0084 §2), nên câu gọi tên "Thụ lý" làm cán bộ không tìm ra nút | `47f35a10` |
| Nút "Sửa hạn xử lý" | **Giữ** — xác nhận lại dòng 43 ở §*Giữ dù bảng lỗi đòi gỡ* | — |
| Số điện thoại người gửi | **Vẫn che** — xác nhận lại dòng 43 ở §*Từ chối vì va luật cứng* (luật 3) | — |

## Sửa đổi 15/10/2026 — bảng lỗi web-admin-141026

Chủ dự án chốt 15/10/2026, lời do phiên chính chuyển lại. Bảng lỗi này cũng đổi phiếu nhập hộ (ADR 0028
§*Sửa đổi 15/10/2026*) và lý do xoá nhiệm vụ (ADR 0076 §*Sửa đổi 15/10/2026* — áp #4 ở trên cho menu
Nhiệm vụ).

### Thứ tự mặc định: mới thêm lên đầu

| Danh sách | Mặc định nay | Trước | Commit |
|---|---|---|---|
| Người dùng — `GET /api/v1/staff`, `POST /api/v1/staff/searches` | `created_at` giảm dần | Mã cán bộ tăng dần | `7737d780` |
| Dự án giải ngân — `GET /api/v1/investment-projects` | `tao_luc` giảm dần | Theo mã | `7737d780` |
| Nhóm Zalo, liên kết Zalo — `GET /api/v1/zalo-groups`, `/zalo-links` | `linked_at` giảm dần | — | `7737d780` |
| Sổ địa điểm — `GET /api/v1/map-assets` | **Vẫn gom theo loại** (mã loại tăng dần), **trong nhóm** `created_at` giảm dần | Nhóm rồi tên | `99c99592` |

Sổ địa điểm giữ gom nhóm vì bảng web gom các dòng liền nhau cùng loại thành một nhóm
(`service-comms/internal/store/map_asset.go:54-71`); sắp thuần theo ngày thì một loại vỡ thành nhiều
nhóm rời.

**Cố ý KHÔNG đổi** — đừng "sửa" theo lời bảng lỗi:

| Danh sách | Thứ tự giữ | Vì sao |
|---|---|---|
| Danh mục có cột Thứ tự (`thu_tu`) | Theo `thu_tu` | Xã tự sắp; "mới lên đầu" sẽ phá thứ tự xã đã đặt |
| Cây tổ chức (bộ phận) | Theo cây | Thứ tự là cấp bậc, không phải ngày tạo |
| Hàng chờ gia hạn, quá hạn | Theo hạn | Việc gấp nhất phải ở đầu |
| Danh bạ cán bộ | Theo `thu_tu_danh_ba` | Thứ tự do xã đặt |

Cột "Vì sao" là cách đọc của người ghi từ tên cột; chủ dự án chỉ liệt kê các danh sách giữ, không nêu lý
do từng dòng.

### Hai dòng khác

| Điểm | Chốt | Chủ sở hữu |
|---|---|---|
| Tab "Cảnh báo thiên tai" trong Nội dung Mini App — bảng lỗi đòi gỡ | **Giữ.** Đây là màn duy nhất đăng cảnh báo thiên tai (#2 ở trên) | ADR 0091 §2 |
| Khoá quyền riêng cho Thôn/TDP, Lời hệ thống, Máy chủ thư, Kênh Zalo — dòng 22 đòi | **Thêm 4 khoá**, nhóm Quản trị, cấp sẵn — §*Bốn khoá cấu hình riêng* ngay dưới. Lần chốt đầu cùng ngày là "không thêm khoá, theo prototype"; chủ dự án đảo trong ngày | §dưới · sổ `service-identity`, mục `bl141026-khoa-quyen-rieng-cau-hinh` |

### Bốn khoá cấu hình riêng — dòng 22, đảo cùng ngày

**Người quyết:** chủ dự án, 15/10/2026, lời do phiên chính chuyển lại. Lần đầu chủ dự án cho đối chiếu
prototype và chốt **không thêm khoá** (prototype: nhóm Quản trị chỉ 6 khoá, mọi tab Cấu hình mở bằng
`admin.lookup`). Sau đó, cùng ngày: *"tôi thấy tách ra thêm 4 mục trên là hợp lý đó, làm đi"*, chọn nhóm
**QUẢN TRỊ** và **cấp sẵn**. Mục này thay lần chốt đầu.

| Khoá | Tên trên màn Phân quyền | Thay khoá cũ | Canh |
|---|---|---|---|
| `admin.village` | Cấu hình thôn/tổ dân phố | `admin.org` (thôn) · `admin.lookup` (loại thôn) | Ghi và nhập Excel thôn/TDP và Loại đơn vị dân cư. Tuyến đọc vẫn `AnyAuthenticated` |
| `admin.message` | Cấu hình lời hệ thống | `admin.lookup` | Mọi tuyến `*-system-messages` của petitions, finance, reporting, kể cả đọc |
| `admin.mail` | Cấu hình máy chủ thư | `admin.lookup` | `mail-settings` và gửi thư thử |
| `admin.zalo` | Cấu hình kênh Zalo | `admin.lookup` | `zalo-channel-settings`, `zalo-links` (danh sách của xã), `zalo-groups`, mã gắn nhóm, `zalo-bots/current`, và `GET commune-launch-link` của platform (QR chỉ dựng trong tab Kênh Zalo) |

Danh sách tuyến đầy đủ theo khoá: `kb/30-indexes/api-surface.json` (sinh từ mã).

**Không đổi:** `zalo-links/current` (cán bộ tự ghép Zalo của mình) giữ `AnyAuthenticated`. Danh mục
khối nhiệm vụ (`task-blocs`) và các nhóm danh mục khác giữ `admin.lookup`. Ghi Sơ đồ tổ chức giữ
`admin.org`.

**Cấp sẵn** (migration identity 0032): vai trò đang dùng có `admin.lookup` nhận cả 4 khoá; có
`admin.org` nhận `admin.village`. Mỗi vai trò được thêm có một dòng `audit_log`, chủ thể
`system`. Vai trò mẫu Chủ tịch UBND và Chánh Văn phòng có thêm 4 khoá. Trên ma trận, 4 khoá đứng ngay
sau `admin.lookup` (`thu_tu` 44–47; `admin.org`/`role`/`sla`/`user` dời xuống 48–51).

Nhận xét của người ghi, không phải lời chủ dự án: không cấp sẵn thì lúc tuyến đổi khoá, mọi cán bộ đang
dùng 4 tab này nhận 403 cho tới khi quản trị xã tick lại từng vai trò.

**Lệch prototype, có chủ ý.** Prototype mở mọi tab Cấu hình bằng `admin.lookup`. Luật #1–#3 của ADR
này không có điều "quyết định tường minh của chủ dự án thắng prototype"; đây là một lựa chọn riêng của
chủ dự án cho dòng 22, không phải luật mới. Ai đối chiếu màn Phân quyền với prototype sẽ thấy nhóm Quản
trị có 10 khoá thay vì 6 — đó là chủ ý.

**Còn treo:**

| Điểm | Vì sao treo |
|---|---|
| Tên khoá có gạch dưới ở vế sau | Trái quy ước `nhóm.mộttừ` và điều kiện dừng #2 của ADR 0030. Kho chưa ghi chủ dự án đã duyệt **tên** khoá. Đổi tên sau khi xã đã tick là migration trên dữ liệu phân quyền đang chạy (ADR 0030 §Quy ước) |
| Cấp sẵn trong migration | ADR 0030 điều kiện dừng #3 cấm gán khoá cho vai trò trong migration hạt giống. Ở đây chủ dự án chọn cấp sẵn (thân commit `54bb094f`), theo khuôn migration 0028 |
| Migration 0032 | **Chưa chạy** trên PostgreSQL thật (thân commit `54bb094f`, `f81168fe`) |

Commit: `54bb094f` (khoá + cấp sẵn) · `f81168fe` (45 tuyến) · `336f4b25` (web-admin). Câu mở #27 ghi
thêm dòng 15/10/2026.
