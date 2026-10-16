# 16 — Danh bạ người dân

**Route:** `/nguoi-dan` · **Tiêu đề trang:** `Danh bạ người dân` · **Menu:** Danh bạ người dân (nhóm Điều hành)

> Chương này chép từ **mã nguồn prototype** (`../vigov-require/apps/admin/src/components/citizens/{CitizenWorkspace,CitizenForm,GroupManager}.tsx`
> và `apps/api/app/modules/citizens/{models,router,service,schemas}.py`, require@0053854), không phải từ
> trình duyệt — chưa có ảnh chụp. Các chỗ **[Lệch prototype — Dn]** là quyết định của chủ dự án
> 16/10/2026, lý do ở **ADR 0097** (`kb/10-decisions/0097-danh-ba-nguoi-dan.md`); không chép lại lý do ở đây.
> Mọi số điện thoại trong chương là số giả `0900000xxx` (luật 3, bất biến 5).

---

## 1. Mục đích

Mô tả trong giao diện (prototype, `CitizenWorkspace.tsx:66-70`): *"Số điện thoại để xã nhắn tin tới
bà con, phân theo nhóm và gắn nhãn. Ai đã dùng Mini App thì có thêm mã Zalo, gửi qua Zalo rẻ hơn tin
nhắn thường."*

**[Lệch prototype — D3, D13]** Câu thứ hai bỏ: kho này không lưu mã Zalo trên dòng danh bạ, và phần
gửi tin chưa có. Dòng mô tả dùng: *"Số điện thoại để xã liên lạc với bà con, phân theo nhóm và gắn
nhãn. Cột Zalo cho biết số đã đăng nhập Mini App của xã."*

Danh bạ **không phải sổ hộ tịch, không phải định danh công dân**: chỉ họ tên và số điện thoại là
bắt buộc (`CitizenForm.tsx:30-32`). Một dòng là *xã muốn liên lạc với số này*.

> Phân biệt với `Danh bạ cán bộ` (`12-danh-ba-can-bo.md`, nay là tab của `/mini-app`): đó là người
> **của** xã, hiện cho dân gọi. Trang này là người dân **mà** xã gọi.

---

## 2. Bố cục

```
PageHeader
  "Danh bạ người dân"
  "Số điện thoại để xã liên lạc với bà con, phân theo nhóm và gắn nhãn.
   Cột Zalo cho biết số đã đăng nhập Mini App của xã."
  Nút (chỉ khi có citizen.update): [⬆ Nhập từ Excel]  [+ Thêm người dân]

Tabs: [Danh sách (n)] [Nhóm (m)]

── Tab Danh sách ──
  [🔍 Tìm theo tên, số điện thoại, địa chỉ…] [Tất cả nhóm ▾] ☐ Chỉ người có Zalo
  Dải nhãn:  🏷 hộ kinh doanh · 12   tổ trưởng · 8   …   (bấm để lọc, bấm lại để bỏ)
  Bảng người dân
  Phân trang: [← Trước]  Trang k  [Sau →]

── Tab Nhóm ──
  Ô "Lập nhóm mới" + nút [+ Lập nhóm]   (chỉ khi có citizen.update)
  Bảng nhóm
```

**[Lệch prototype — D3]** Nút `Nhập từ Excel` prototype không có; thêm theo quyết định nguồn
`nhap-excel`.

**[Lệch prototype — D8]** Prototype hiện `Trang {k}/{tổng trang}` từ `total` trong phản hồi. Kho này
phân trang keyset; tổng lấy từ tuyến đếm riêng cùng bộ lọc để hiện *"n người"* ở nhãn tab. Không hiện
"trang k/N".

Bấm vào một dòng (khi có `citizen.update`) mở hộp thoại **Sửa thông tin người dân** (§5). Không có
`citizen.update` thì dòng không bấm được và cột hành động ẩn (`CitizenWorkspace.tsx:178, 185`).

---

## 3. Bộ lọc

| Bộ lọc | Giá trị | Ghi chú |
|---|---|---|
| Tìm | `Tìm theo tên, số điện thoại, địa chỉ…` | Khớp họ tên và địa chỉ **bỏ dấu**, và **bất kỳ phần nào** của số điện thoại. Từ khoá đi **trong thân yêu cầu**, không trên URL — **[D11]**, rủi ro đã chấp nhận ở ADR 0097 §4 |
| Nhóm | `Tất cả nhóm` + mỗi nhóm kèm `({số thành viên})` | |
| ☐ Chỉ người có Zalo | bật = chỉ dòng có cột Zalo "Có" | Cùng phép nối với cột Zalo (§10 #5), không phải cờ lưu sẵn |
| Nhãn | dải nút nhãn kèm số người; một nhãn một lúc | Ẩn khi xã chưa có nhãn nào |
| Thôn / tổ dân phố | *(prototype có tham số `hamlet_id` ở API, màn không có ô lọc — không thêm)* | |

Mỗi trang **50** dòng (`PAGE_SIZE`, `CitizenWorkspace.tsx:25`).

---

## 4. Bảng người dân

| Cột | Nội dung |
|---|---|
| Họ tên | đậm |
| Điện thoại | **che** (`privacy.MaskPhone`), chữ số đều hàng — **[Lệch prototype — D2]**: prototype hiện số trần |
| Thôn, tổ | tên đơn vị dân cư (`residential-units`) hoặc `—` |
| Nhãn | các nhãn nối bằng dấu phẩy, hoặc `—` |
| Zalo | **`Có`** (xanh lá, đậm) hoặc `—`. Tooltip trên tiêu đề cột: *"Đã đăng nhập Mini App của xã bằng số này"* — **[D3, D13]** |
| *(hành động)* | `🗑` aria-label `Xoá {họ tên}` — mở hộp thoại hỏi **lý do (bắt buộc)** rồi xoá mềm — **[Lệch prototype — D7]**: prototype xoá ngay, không lý do |

Ô trống: chưa có ai và không có từ khoá → *"Danh bạ chưa có ai. Bấm “Thêm người dân” để bắt đầu."*;
có bộ lọc mà không khớp → *"Không có ai khớp bộ lọc."* (`CitizenWorkspace.tsx:165-167`).

Thông báo sau xoá: *"Đã gỡ {họ tên}."* · lỗi: *"Chưa gỡ được."*

---

## 5. Hộp thoại "Thêm người dân" / "Sửa thông tin người dân"

Rộng `42rem`, cuộn trong. Mô tả: *"Bắt buộc họ tên và số điện thoại. Những ô còn lại điền được tới
đâu hay tới đó."* (`CitizenForm.tsx:105-108`).

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Họ và tên | text | ✔ | |
| Số điện thoại | text, `inputMode="tel"`, ≥ 8 ký tự | ✔ | Máy chủ chuẩn hoá: bỏ mọi ký tự không phải số, `+84…`/`84…` quy về `0…` (`service.py:45-55`). Khi **sửa**: ô hiện **số đã che**, cạnh có nút **"Xem số đầy đủ"** — **[Lệch prototype — D2]**, hành vi ở §10 #3 |
| ~~Mã Zalo~~ | — | — | **[Lệch prototype — D3]**: bỏ ô `zalo_uid` (prototype `CitizenForm.tsx:130-141`). Cột Zalo suy ra, không nhập |
| Thôn, tổ dân phố | select `residential-units`, mặc định `Chưa rõ` | | |
| Địa chỉ | text, cả hàng | | Chỉ có ở hộp thoại, **không** vào phản hồi danh sách — **[D13]** |
| Nhãn | text, cách nhau dấu phẩy, cả hàng | | Gợi ý prototype: *"Cách nhau bằng dấu phẩy: hộ nghèo, tổ trưởng, hộ kinh doanh…"*. **[D4]** Dưới ô, câu cố định không tắt được: **"Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị"** |
| Nhóm | dải nút bật/tắt theo nhóm của xã, cả hàng | | Chưa có nhóm: *"Xã chưa lập nhóm nào. Lập ở ngăn “Nhóm”."* |
| Ghi chú | textarea 2 dòng, cả hàng | | Chỉ ở hộp thoại — **[D13]** |
| Đang liên hệ | checkbox `is_active`, mặc định bật | | Prototype có ở API (`schemas.py:56`), màn không có ô; thêm ô vì không có cách nào khác để tắt — **[D6]** |

Nút: `Huỷ` · `Thêm vào danh bạ` / `Lưu thay đổi` (vô hiệu khi thiếu ô bắt buộc).
Thông báo: *"Đã thêm vào danh bạ."* · *"Đã lưu."* · trùng số: *"Số điện thoại này đã có trong danh
bạ."* · khác: *"Chưa lưu được. Vui lòng thử lại."*

### Nút "Xem số đầy đủ" — [D2]

| Bước | Việc |
|---|---|
| 1 | Chỉ hiện khi có `citizen.update`; chỉ trong hộp thoại sửa, không ở bảng, không ở hộp thoại thêm |
| 2 | Bấm → gọi máy chủ; máy chủ ghi **một vết** (ai, dòng nào, trường `phone`, khoá `citizen.update`, lúc nào, IP, xã — **không ghi số**) trong **cùng giao dịch** với lần đọc, rồi mới trả số |
| 3 | Ô đổi sang số đầy đủ, sửa được. Đóng hộp thoại là quên; mở lại bấm lại là một vết mới |
| 4 | Lỗi → giữ số che, báo *"Chưa lấy được số. Thử lại."* |

---

## 6. Tab "Nhóm"

Prototype `GroupManager.tsx`. Mô tả ô trống: *"Chưa có nhóm nào — Nhóm giúp gửi thông báo cho đúng
người: tổ trưởng dân phố, hộ kinh doanh, hội viên phụ nữ…"*

| Phần | Nội dung |
|---|---|
| Lập nhóm mới | ô tên, gợi ý *"Tổ trưởng dân phố, Hộ kinh doanh…"*, nút `+ Lập nhóm` (chỉ `citizen.update`) |
| Bảng nhóm | mỗi dòng: tên (đậm) + mô tả (nhỏ, nếu có) · `{n} người` · `🗑` aria-label `Gỡ nhóm {tên}` |
| Gỡ nhóm | xoá mềm; hỏi lý do **tuỳ chọn** — **[D7]**. Người trong nhóm **vẫn còn trong danh bạ** (`GroupManager.tsx:17`) |

Thông báo: *"Đã lập nhóm."* · lỗi: *"Tên nhóm đã có, hoặc chưa lưu được."* · *"Đã gỡ nhóm."* ·
*"Chưa gỡ được."* Tên nhóm **duy nhất trong xã** trên dòng sống.

Sửa tên/mô tả/thứ tự: prototype có `PUT /citizens/groups/{id}` nhưng màn không có nút sửa — **không
thêm** màn sửa; tuyến có hay không do người dựng theo prototype API.

---

## 7. Nhập từ Excel — [D3, D13]

| Điều | Chốt |
|---|---|
| Mẫu cột | `Họ và tên \| Số điện thoại \| Thôn, tổ dân phố \| Địa chỉ \| Nhóm \| Nhãn \| Ghi chú` (đúng thứ tự `domain.CitizenContactImportColumns()`) — nhóm và nhãn cách nhau dấu phẩy, nhóm khớp theo **tên** nhóm đang có; cột thôn là ô thả xuống `mã · tên`; tối đa 200 dòng một tệp |
| Số giả trong mẫu | `0900000000` (luật 3 bất biến 5) |
| Ba tuyến | tải mẫu · xem trước · ghi — cả ba quyền `citizen.update` |
| Tệp tải lên | **không lưu**. Mỗi dòng ghi `source = nhap-excel` và `import_batch_id`; **một vết** cho cả lượt (số dòng, mã lượt, không giá trị) — tiền lệ ADR 0059 §1 |
| Trùng số trong xã | là lỗi của dòng ấy; xem trước nêu **dòng và cột**, không nhắc lại giá trị (luật 3 cấm #3) |
| Chuẩn hoá số | như §5 |
| Tất cả-hoặc-không, hay nhận từng phần | **Tất cả-hoặc-không** — theo khuôn nhập Excel đã có của kho (ADR 0059 §1; sổ đơn thư `citizen-letters/imports`, cán bộ `staff_import.go`): một dòng sai thì không ghi dòng nào, xem trước nêu đủ lỗi để sửa tệp rồi nhập lại. Lựa chọn kỹ thuật của phiên chính 16/10/2026, không phải lời chủ dự án |

---

## 8. Mô hình dữ liệu (`service-identity`) — ADR 0097 D1, D5–D7, D9

### `citizen_contact` — phân mảnh `HASH (tenant_id) MODULUS 8`

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `tenant_id`, `id` | ULID | `PRIMARY KEY (tenant_id, id)` |
| `full_name` | text, bắt buộc | |
| `phone` | text, bắt buộc | đã chuẩn hoá (chỉ số, bắt đầu `0`) |
| `residential_unit_id` | ULID null | → `thon_to_dan_pho` cùng xã |
| `address` | text null | không vào danh sách |
| `note` | text null | không vào danh sách |
| `tags` | text[] / jsonb | nhãn tự do |
| `source` | enum `nhap-tay` · `nhap-excel` | ADR 0011; giá trị thứ ba để sau |
| `import_batch_id` | ULID null | chỉ khi `nhap-excel` |
| `household_code` | text null | **để dành** cho "bà dùng số của cháu"; đợt này không dùng |
| `is_active` | bool, mặc định true | |
| `deleted_at` · `deleted_by` · `delete_reason` | | xoá mềm; lý do bắt buộc |
| `live_key` | bool sinh từ `deleted_at` | `UNIQUE (tenant_id, phone, live_key)` — khuôn migration identity 0027 |
| `created_at` · `updated_at` · `created_by` · `updated_by` | | |

Không có cột `zalo_uid`, `cong_dan_id`, `last_synced_at` — **[D3]**: cột Zalo là phép nối lúc đọc, §10 #5.

### `citizen_group` — bảng thường

`tenant_id`, `id`, `name` (duy nhất trong xã trên dòng sống, `live_key`), `description`, `sort_order`,
xoá mềm (lý do tuỳ chọn), vết tạo/sửa.

### `citizen_contact_group` — phân mảnh `MODULUS 8`

`tenant_id`, `contact_id`, `group_id`, xoá mềm; `UNIQUE (tenant_id, group_id, contact_id, live_key)`.

---

## 9. API

Tài nguyên và quyền là phần đã chốt; **phương thức và đường dẫn đầy đủ đọc ở `kb/20-contracts/openapi.json`
sau khi dựng** (sinh từ mã, ADR 0014) — không chép tay ở đây.

| Tài nguyên | Việc | Quyền |
|---|---|---|
| `citizen-contacts` | danh sách (bộ lọc §3 **trong thân yêu cầu**, keyset 50) · chi tiết · tạo · sửa (kể cả nhóm, nhãn, `is_active`) · xoá mềm có lý do | đọc `citizen.read` · ghi `citizen.update` |
| tuyến đếm riêng cùng bộ lọc | `{"count": n}` — khuôn `task-extension-counts` (ADR 0071) | `citizen.read` |
| nhãn đang dùng | danh sách `{tag, count}` | `citizen.read` |
| "Xem số đầy đủ" | trả số đầy đủ của **một** dòng, mỗi lần một vết | `citizen.update` |
| nhập Excel | tải mẫu · xem trước · ghi | `citizen.update` |
| `citizen-groups` | danh sách kèm `member_count` · lập · (sửa) · gỡ mềm | đọc `citizen.read` · ghi `citizen.update` |

Phản hồi danh sách: `id`, `full_name`, `phone` **đã che**, `residential_unit_id` + tên, `tags`,
`has_zalo`, `is_active`, `group_ids`, `source`, `created_at`. **Không** `address`, **không** `note`.

---

## 10. Quy tắc nghiệp vụ

1. Họ tên và số điện thoại bắt buộc; số chuẩn hoá trước khi so trùng (`service.py:45-55`).
2. **Một số = một dòng sống mỗi xã.** Dòng đã xoá mềm không chặn thêm lại số ấy.
3. Số **che** ở mọi nơi trừ nút "Xem số đầy đủ" (§5); mỗi lần xem một vết, vết không mang số.
4. Tìm theo từ khoá khớp một phần số, cho mọi người giữ `citizen.read` — rủi ro đã chấp nhận (ADR 0097 §4).
5. **Cột Zalo suy ra** lúc đọc: số đã chuẩn hoá → `dinh_danh_cong_dan` → một `phien_cong_dan` **của chính xã** còn sống (`cong_dan_id` khác NULL, chưa thu hồi, chưa hết hạn). Trả **chỉ Có/—**; không id, không cho biết số có ở xã khác. Số có phiên ở xã B → xã A thấy `—` (ADR 0097 §3, bài kiểm bắt buộc).
6. Nhóm là danh mục của xã; nhãn là chữ tự do; **cả hai** đều giữ. Câu cấm nhãn nhạy cảm luôn hiện dưới ô nhãn.
7. Gỡ nhóm không gỡ người. Gỡ người cần lý do.
8. `is_active = false` = xã ngừng liên hệ; dòng vẫn hiện trong danh bạ (mờ), vẫn tìm được.
9. Tệp Excel không lưu; mỗi lượt nhập một vết.
10. Không có tuyến xuất danh bạ trong đợt này — kể cả bản đã che.
11. Trưởng thôn đọc **cả xã** (chưa lọc theo thôn) — chấp nhận cho đợt này.
12. Mọi ghi (thêm, sửa, xoá, nhập, gỡ nhóm) để vết với `actor_id` là **mã cán bộ** (luật 6 bất biến 8).

---

## 11. Quyền và vai trò mẫu — [D12]

| Khoá | Nhãn (migration 0028) | Vai trò mẫu được cấp sẵn (migration 0034) |
|---|---|---|
| `citizen.read` | Xem danh bạ người dân | `chu-tich-ubnd` · `pho-chu-tich-ubnd` · `chanh-van-phong` · `truong-bo-phan` · `chuyen-vien` · `tiep-nhan` · `truong-thon` · `quan-tri-he-thong` |
| `citizen.update` | Thêm, sửa, xoá người dân và nhóm | `chu-tich-ubnd` · `chanh-van-phong` · `tiep-nhan` · `truong-thon` · `quan-tri-he-thong` |

Nguồn: prototype `20260924_0050-0050_citizens_notices.py:44-56` (mã vai trò của kho này mang hậu tố
`-ubnd`, `-he-thong` theo ADR 0055). Không thêm khoá thứ ba (câu mở #27).

---

## 12. Khác prototype có chủ ý — đừng "sửa cho khớp"

| Điểm | Prototype | Kho này | Căn cứ |
|---|---|---|---|
| Số điện thoại ở bảng | trần | che + "Xem số đầy đủ" | ADR 0097 D2 |
| Mã Zalo | ô nhập `zalo_uid` | không có; cột Zalo suy ra từ phiên | D3 |
| Nguồn dòng | `manual` / đồng bộ Mini App | `nhap-tay` · `nhap-excel`; không đồng bộ | D3 |
| Nhập Excel | không có | có, ba tuyến | D3, D13 |
| Phân trang | offset + `total` | keyset + tuyến đếm | D8 |
| Xoá người | không lý do | lý do bắt buộc | D7 |
| Địa chỉ, ghi chú | trong danh sách | chỉ ở hộp thoại | D13 |
| Từ khoá tìm | query string | thân yêu cầu | D11 |
| Câu cấm nhãn nhạy cảm | không có | có, cố định | D4 |
| Ô "Đang liên hệ" | chỉ ở API | có ô trong hộp thoại | D6 |

---

## 13. Ngoài phạm vi đợt này

Gửi tin ZNS / SMS tới danh bạ (MC2) · xuất Excel danh bạ · đồng bộ từ Mini App · trưởng thôn chỉ
thấy thôn mình · cột đồng ý từng dòng · `household_code`. Lý do và chỗ quay lại: ADR 0097 §Ngoài
phạm vi.

---

## 14. Dữ liệu mẫu

Không chép dữ liệu của prototype. Dữ liệu thử dùng số giả:

| Họ và tên | Số điện thoại (hiện) | Thôn, tổ | Nhãn | Zalo |
|---|---|---|---|---|
| Nguyễn Văn A | `09****0001` | Thôn 1 | tổ trưởng | Có |
| Trần Thị B | `09****0002` | Thôn 2 | hộ kinh doanh | — |
| Lê Văn C | `09****0003` | — | — | — |

Cách che: `privacy.MaskPhone` (`core/privacy/mask.go:12`) — giữ 2 số đầu và 4 số cuối.
