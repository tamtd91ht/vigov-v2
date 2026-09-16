# 02 — Quản lý nhiệm vụ

**Route:** `/nhiem-vu` · **Tiêu đề trang:** `Quản lý nhiệm vụ · ViGov` · **Menu:** Nhiệm vụ (nhóm Điều hành)

> Đây là **module lõi** của ViGov. Dựng module này trước, các module khác đều đổ việc vào đây.

---

## 1. Mục đích

Mô tả trong giao diện: *"Giao việc từ kết luận họp, theo dõi tiến độ và đôn đốc tự động."*

Ba khả năng chính:
1. Giao việc cho **bộ phận** hoặc **đích danh cán bộ**.
2. Theo dõi vòng đời nhiệm vụ qua Kanban / Danh sách / Sổ theo dõi.
3. Đôn đốc tự động theo SLA và leo thang khi trễ.

Điểm đặc thù hành chính: nhiệm vụ có **hai loại** với bộ trường khác nhau — `Theo văn bản` (có sổ theo dõi văn bản chỉ đạo đầy đủ) và `Nhiệm vụ cơ bản` (gọn).

---

## 2. Bố cục trang

```
PageHeader
  "Quản lý nhiệm vụ"
  "Giao việc từ kết luận họp, theo dõi tiến độ và đôn đốc tự động."
  Nút: [⬆ Nhập từ Excel]  [+ Giao việc mới]  (nút chính, nền tối)

Hàng lọc 1:
  [Toàn xã | Giao cho tôi | Liên quan đến tôi]   [🔍 Tìm theo tên nhiệm vụ…]
  [Tất cả bộ phận ▾] [Tất cả người thực hiện ▾] [Mọi mức ưu tiên ▾]

Hàng lọc 2:
  [Mọi loại nhiệm vụ ▾] [Mọi khối ▾] [Mọi nguồn giao ▾] [☐ Chỉ việc quá hạn] [☐ Sắp đến hạn]
                                                    (phải) [▦ Kanban][☰ Danh sách][▤ Sổ theo dõi]

Vùng nội dung: Kanban | Bảng danh sách | Bảng sổ theo dõi
Chân trang: "Hiển thị {N} nhiệm vụ."
```

Khi chọn checkbox ở bảng, hàng lọc 2 bên phải đổi thành: `Đã chọn {N} nhiệm vụ` + nút đỏ `🗑 Xoá đã chọn`.

---

## 3. Bộ lọc — giá trị đầy đủ

| Bộ lọc | Giá trị |
|---|---|
| **Phạm vi** | `Toàn xã` (tất cả hồ sơ trong xã) · `Giao cho tôi` (đích danh tôi là người xử lý) · `Liên quan đến tôi` (tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ) |
| **Tìm kiếm** | placeholder `Tìm theo tên nhiệm vụ…` — tìm trong mã + tiêu đề |
| **Bộ phận** | `Tất cả bộ phận` + 5 bộ phận |
| **Người thực hiện** | `Tất cả người thực hiện` + danh sách cán bộ |
| **Mức ưu tiên** | `Mọi mức ưu tiên` · `Khẩn` · `Cao` · `Thường` |
| **Loại nhiệm vụ** | `Mọi loại nhiệm vụ` · `Theo văn bản` · `Nhiệm vụ cơ bản` |
| **Khối** | `Mọi khối` · `Khối Uỷ ban` · `Khối Đảng` · `Khác` |
| **Nguồn giao** | `Mọi nguồn giao` · `Giao trực tiếp` · `Từ kết luận họp` · `Từ văn bản đến` · `Từ phản ánh` |
| **Chỉ việc quá hạn** | checkbox |
| **Sắp đến hạn** | checkbox — dùng ngưỡng giờ ở `Cấu hình → Thời hạn xử lý` (mặc định 72 giờ) |

Tất cả bộ lọc **kết hợp AND** và nên đồng bộ vào query string để chia sẻ link.

---

## 4. Ba chế độ xem

### 4.1 Kanban (mặc định)

5 cột ứng với 5 trạng thái chính, mỗi cột có chấm màu + tên + số lượng:

| Cột | Màu chấm | Ví dụ số |
|---|---|---|
| Chưa thực hiện | xám | 7 |
| Đã tiếp nhận | cam | 6 |
| Đang thực hiện | xanh dương | 11 |
| Chờ duyệt | tím | 0 |
| Hoàn thành | xanh lá | 4 |

> Hai trạng thái rẽ nhánh (`Tạm dừng`, `Chuyển tiếp`) **không có cột riêng** trên Kanban.

**Thẻ nhiệm vụ**
```
☐ Chọn                          ← checkbox chọn hàng loạt
NV19                            ← mã nhiệm vụ, chữ nhỏ xám
Báo cáo tổng kết việc thực hiện  ← tiêu đề, tối đa 3 dòng
chủ trương của Bộ Chính trị…
[3 việc con]                    ← chip chỉ hiện khi có nhiệm vụ con
⏱ Trễ 87 ngày                   ← đỏ; hoặc "Hạn 20/12/2026"; hoặc "Hạn —"
Huỳnh Văn 3                     ← người thực hiện, hoặc "Chưa phân công"
```
- Viền trái thẻ tô màu theo mức ưu tiên/tình trạng hạn (đỏ = trễ, cam = sắp đến hạn, xanh = bình thường).
- Thẻ đã hoàn thành trễ hiện chip xanh lá `Hoàn thành trễ hạn`.
- **Kéo thả** thẻ giữa các cột để đổi trạng thái (tuân thủ quy tắc chuyển trạng thái mục 6).
- Cột rỗng hiện `Không có nhiệm vụ`.

### 4.2 Danh sách

Bảng cuộn ngang, cột có thể sắp xếp (icon ⇅):

| Cột | Nội dung |
|---|---|
| ☐ | checkbox chọn |
| Mã | `NV34` |
| Tên việc | tiêu đề + dòng phụ nhỏ ghi **nguồn giao** (`Giao trực tiếp`, `Từ văn bản đến`, `Từ kết luận họp`, `Từ phản ánh`) |
| Người thực hiện | hoặc `Chưa phân công` |
| Bộ phận | hoặc `—` |
| Ưu tiên | `Thường` / `Cao` / `Khẩn` |
| Hạn | `10/9/2026 (trễ 6 ngày)` — phần trễ hiện đỏ; hoặc `—` |
| Trạng thái | StatusChip |

Hàng có việc quá hạn tô nền hồng rất nhạt.

### 4.3 Sổ theo dõi

Bảng dạng **sổ công văn chỉ đạo** — chính là bản in mà văn phòng xã đang dùng. Cột:

| Cột | Nguồn |
|---|---|
| ☐ | chọn |
| Mã | `NV33` |
| Nội dung nhiệm vụ / Trích yếu văn bản | tiêu đề + dòng phụ mô tả + nhãn khối (`Khối Uỷ ban`) |
| Cơ quan chủ trì tham mưu | bộ phận |
| Chuyên viên VP tham mưu / theo dõi | cán bộ |
| Đơn vị thực hiện | bộ phận + dòng phụ tên người |
| Văn bản cấp trên giao | `90-TB/TU · 30/11/2026` + dòng phụ trích yếu; `Không số` nếu thiếu |
| *(cuộn tiếp)* Văn bản chỉ đạo của Đảng uỷ, Văn bản sản phẩm đầu ra, Hạn xử lý, Tóm tắt kết quả, Ghi chú, các ô tick phê duyệt | |

Xuất Excel của bảng này phải giữ đúng thứ tự cột.

---

## 5. Chi tiết nhiệm vụ (DetailDrawer)

Mở khi bấm vào tiêu đề thẻ/hàng. Lớp phủ gần toàn màn hình, nút `✕` góc phải trên.

### 5.1 Đầu drawer
```
[NV19] Báo cáo tổng kết việc thực hiện chủ trương của Bộ Chính trị về công tác cán bộ
— · Huỳnh Văn 3                       ← bộ phận · người thực hiện
```

### 5.2 StatusStepper
Dải ngang 4 bước chính, bước hiện tại tô nền xanh đậm + hiển thị thời gian đã ở trạng thái:
```
[Chưa thực hiện]  [Đã tiếp nhận]  [Đang thực hiện]  [Hoàn thành]
 19 ngày 23 giờ         —                 —                —
Rẽ nhánh:  [Tạm dừng —]   [Chuyển tiếp —]
```
Dưới stepper là **câu giải thích trạng thái hiện tại**, ví dụ: *"Đã giao nhưng người nhận chưa bấm tiếp nhận."*
Bấm vào một bước = chuyển trạng thái (nếu có quyền).

### 5.3 Ba ô tóm tắt

| Ô | Nội dung |
|---|---|
| `HẠN XỬ LÝ` | `Trễ 87 ngày` (đỏ) + `Hạn 20/6/2026` |
| `ĐANG GIAO CHO` | tên bộ phận (hoặc `Chưa giao bộ phận nào`) + `Họ tên — email` |
| `MỨC ƯU TIÊN` | `Cao` + `0% tiến độ ghi nhận` |

### 5.4 Khối "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO" *(chỉ với loại `Theo văn bản`)*

Có nút `✎ Sửa` ở góc. Các trường:

| Nhãn | Kiểu | Ví dụ |
|---|---|---|
| Mã nhiệm vụ | text | `NV19` |
| Nội dung nhiệm vụ / Trích yếu văn bản | text | |
| Hạn xử lý | date | `20/6/2026` |
| Cơ quan chủ trì tham mưu | bộ phận | `VĂN PHÒNG ĐẢNG ỦY` |
| Chuyên viên Văn phòng tham mưu / theo dõi | cán bộ | `Huỳnh Văn 3` |
| Văn bản cấp trên giao | danh sách văn bản `{số}-{ký hiệu} · {ngày}` + trích yếu | `1742-CV/BTCTU · 9/6/2026` / `Công văn của Ban Tổ chức Thành uỷ` |
| Văn bản chỉ đạo của Đảng uỷ | danh sách văn bản | `—` |
| Văn bản sản phẩm đầu ra | danh sách văn bản | `324-BC/ĐU · 15/6/2026` / `Báo cáo của Ban Thường vụ Đảng uỷ` |
| Tóm tắt kết quả thực hiện | textarea | |
| Ghi chú | textarea | `—` |
| ☑ Lãnh đạo xã đã phê duyệt hoàn thành | checkbox | |
| ☐ Cấp trên đã công nhận hoàn thành | checkbox | |

> Chú thích bắt buộc hiển thị: *"Hai ô này đánh dấu bằng tay và không làm đổi trạng thái nhiệm vụ."*

### 5.5 Mô tả nhiệm vụ
Khối văn bản tự do.

### 5.6 Thời hạn
```
HẠN XỬ LÝ   20/6/2026        HẠN BAN ĐẦU   20/6/2026
```
`HẠN BAN ĐẦU` không đổi khi gia hạn — dùng cho thống kê đúng hạn.

### 5.7 Giao việc, chuyển việc
- `Bộ phận` — select, mặc định `— Chưa xác định —`.
- `Người thực hiện` — select, mặc định `— Để bộ phận tự phân công —`, mỗi option hiển thị `Họ tên — email · Chức danh`.
- `Lý do chuyển (bỏ trống nếu chỉ giao lần đầu)` — text.
- Nút `Giao việc`.

### 5.8 Đề nghị lùi hạn
- `Hạn mới` (date) · `Lý do` (text) · nút `Gửi đề nghị lùi hạn`.
- Chú thích: *"Hạn gốc vẫn được giữ lại để báo cáo đúng hạn không bị lùi theo."*
- Đề nghị gửi tới **Lãnh đạo giao việc**, qua chuông và qua thư.

### 5.9 Nhật ký & Trao đổi (cột phải)
- Ô nhập `Đã làm được gì, còn vướng gì…`
- Nút `📎 Đính kèm` và `➤ Ghi nhật ký`
- Dòng thời gian: avatar + tên + thời điểm + chip trạng thái tại thời điểm đó + nội dung + thông tin bộ phận/phụ trách khi có thay đổi phân công.
- Rỗng: `Chưa có ghi chép nào.`

### 5.10 Nhiệm vụ con
Thẻ có `3 việc con` ⇒ drawer hiển thị danh sách nhiệm vụ con (tiêu đề, người thực hiện, hạn, trạng thái) và cho thêm mới. Việc con có hạn riêng và cũng lên `Sổ tay lãnh đạo`.

---

## 6. Vòng đời trạng thái

Trạng thái lấy từ danh mục `Trạng thái nhiệm vụ` (sửa được ở Cấu hình):

| Mã | Nhãn | Thứ tự | Vai trò |
|---|---|---|---|
| `moi-giao` | Mới giao *(Kanban gọi "Chưa thực hiện")* | 1 | chính |
| `da-tiep-nhan` | Đã tiếp nhận | 2 | chính |
| `dang-thuc-hien` | Đang thực hiện | 3 | chính |
| `cho-duyet` | Chờ duyệt | 4 | chính |
| `hoan-thanh` | Hoàn thành | 5 | chính |
| `tam-dung` | Tạm dừng | 6 | rẽ nhánh |
| `chuyen-tiep` | Chuyển tiếp | 7 | rẽ nhánh |

```
moi-giao ──tiếp nhận──> da-tiep-nhan ──bắt đầu──> dang-thuc-hien ──báo xong──> cho-duyet ──duyệt──> hoan-thanh
    │                        │                          │                          │
    └────────────────────────┴──────────┬───────────────┴──────────────────────────┘
                                        ├──> tam-dung  ──tiếp tục──> (trạng thái trước)
                                        └──> chuyen-tiep (chuyển bộ phận khác, sinh bản ghi liên kết)
```

Nhãn phụ khi hoàn thành sau hạn: chip `Hoàn thành trễ hạn`.
Quyền tương ứng: `task.create`, `task.assign`, `task.update`, `task.approve` (duyệt hoàn thành), `task.extend` (duyệt gia hạn), `task.delete`, `task.read`.

---

## 7. Form "Giao việc mới"

Modal, tiêu đề `Giao việc mới`, mô tả:
*"Giao cho một bộ phận hoặc trực tiếp cho cán bộ. Giao cho bộ phận mà quá lâu chưa phân công người thì hệ thống báo lên lãnh đạo."*

### 7.1 Trường chung (cả hai loại)

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Loại nhiệm vụ | select | ✔ | `Theo văn bản` (mặc định) \| `Nhiệm vụ cơ bản` |
| Khối nhiệm vụ | select | | `— Chưa xác định —` \| Khối Uỷ ban \| Khối Đảng \| Khác |
| Mã nhiệm vụ | text + checkbox `Tự sinh mã` (mặc định bật) | | Chú thích: *"Tự sinh sẽ cấp số tiếp theo trong dãy NV01, NV02… Nhập từ Excel cũng được đánh số tự động theo dãy này."* |
| Mô tả | textarea | | |
| Lãnh đạo giao việc | combobox tìm kiếm (`Gõ tên để tìm…`) | | Chú thích: *"Đề nghị lùi hạn sẽ gửi tới người này, qua chuông và qua thư."* |
| Hạn hoàn thành | date `dd/mm/yyyy` | | |
| Mức ưu tiên | select | | `Thường` (mặc định) \| `Cao` \| `Khẩn` |

### 7.2 Riêng loại `Theo văn bản` — thêm

| Trường | Kiểu | Bắt buộc |
|---|---|---|
| Nội dung nhiệm vụ / Trích yếu văn bản | text | ✔ |
| Cơ quan chủ trì tham mưu | select bộ phận (`— Chọn cơ quan —`) | |
| Chuyên viên Văn phòng tham mưu / theo dõi | combobox tìm cán bộ | |
| Văn bản cấp trên giao | **danh sách động** textarea + nút `+ Thêm văn bản`, mỗi mục có nút `✕` xoá. Placeholder: `Ví dụ: Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo…` | |
| Văn bản chỉ đạo của Đảng uỷ | danh sách động, placeholder `Ví dụ: Công văn số 416-CV/ĐU ngày 15/6/2026 về tham mưu báo cáo…` | |
| Văn bản sản phẩm đầu ra | danh sách động, placeholder `Ví dụ: Báo cáo số 335-BC/ĐU ngày 29/6/2026` | |
| Ghi chú | textarea | |

### 7.3 Riêng loại `Nhiệm vụ cơ bản`
Thay `Nội dung nhiệm vụ / Trích yếu văn bản` bằng **`Tên nhiệm vụ` (bắt buộc)**, và **bỏ toàn bộ** các trường cơ quan tham mưu / chuyên viên / ba nhóm văn bản / ghi chú.

Nút: `Huỷ` · `Giao việc`.

---

## 8. Nhập từ Excel

Modal tương tự mẫu chung: mô tả → `⬇ Tải mẫu nhiệm vụ` → vùng kéo thả `Chọn tệp .xlsx` → `Đóng` / `Nhập`.
Mã nhiệm vụ được cấp tự động theo dãy `NV01, NV02…`.

---

## 9. Mô hình dữ liệu

**`nhiem_vu`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `ma` | text unique | `NV19`, tự sinh theo dãy |
| `loai` | enum | `theo-van-ban` \| `co-ban` |
| `khoi` | enum null | `khoi-uy-ban` \| `khoi-dang` \| `khac` |
| `tieu_de` | text | nội dung nhiệm vụ / trích yếu |
| `mo_ta` | text | |
| `trang_thai` | enum | 7 giá trị mục 6 |
| `muc_uu_tien` | enum | `khan` \| `cao` \| `thuong` |
| `nguon_giao` | enum | `truc-tiep` \| `ket-luan-hop` \| `van-ban-den` \| `phan-anh` |
| `nguon_id` | uuid null | trỏ về `ket_luan_hop.id` / `don_thu.id` / `phan_anh.id` |
| `bo_phan_id` | uuid null | đơn vị thực hiện |
| `nguoi_thuc_hien_id` | uuid null | null = `Chưa phân công` |
| `lanh_dao_giao_viec_id` | uuid null | nhận đề nghị lùi hạn |
| `co_quan_chu_tri_id` | uuid null | cơ quan chủ trì tham mưu |
| `chuyen_vien_theo_doi_id` | uuid null | |
| `han_xu_ly` | date null | |
| `han_ban_dau` | date null | **không đổi khi gia hạn** |
| `tien_do` | int | 0–100 |
| `nhiem_vu_cha_id` | uuid null | nhiệm vụ con |
| `tom_tat_ket_qua` | text | |
| `ghi_chu` | text | |
| `lanh_dao_phe_duyet_hoan_thanh` | bool | tick tay |
| `cap_tren_cong_nhan_hoan_thanh` | bool | tick tay |
| `ngay_hoan_thanh` | timestamp null | |
| `nguoi_tao_id`, `tao_luc`, `cap_nhat_luc` | | |

**`nhiem_vu_van_ban`** — ba nhóm văn bản liên quan

| Trường | Kiểu |
|---|---|
| `id`, `nhiem_vu_id` | |
| `nhom` | enum `cap-tren-giao` \| `chi-dao-dang-uy` \| `san-pham-dau-ra` |
| `so_ky_hieu` | text (`1742-CV/BTCTU`) |
| `ngay_van_ban` | date |
| `trich_yeu` | text |
| `thu_tu` | int |

**`nhat_ky_nhiem_vu`**: `id, nhiem_vu_id, nguoi_id, thoi_diem, trang_thai_tai_thoi_diem, bo_phan_id, nguoi_phu_trach_id, noi_dung, dinh_kem jsonb`

**`de_nghi_lui_han`**: `id, nhiem_vu_id, nguoi_de_nghi_id, han_moi, ly_do, trang_thai (cho-duyet|da-duyet|tu-choi), nguoi_duyet_id, thoi_diem`

---

## 10. API đề xuất

| Method | Endpoint | Mô tả |
|---|---|---|
| GET | `/api/nhiem-vu` | query: `pham_vi, q, bo_phan, nguoi_thuc_hien, uu_tien, loai, khoi, nguon_giao, qua_han, sap_den_han, che_do_xem` |
| POST | `/api/nhiem-vu` | tạo mới |
| GET/PATCH/DELETE | `/api/nhiem-vu/:id` | |
| POST | `/api/nhiem-vu/:id/trang-thai` | `{trang_thai, ghi_chu}` |
| POST | `/api/nhiem-vu/:id/giao-viec` | `{bo_phan_id, nguoi_thuc_hien_id, ly_do_chuyen}` |
| POST | `/api/nhiem-vu/:id/lui-han` | `{han_moi, ly_do}` |
| POST | `/api/nhiem-vu/:id/lui-han/:deNghiId/duyet` | |
| POST | `/api/nhiem-vu/:id/nhat-ky` | `{noi_dung, dinh_kem[]}` |
| POST | `/api/nhiem-vu/xoa-nhieu` | `{ids[]}` |
| POST | `/api/nhiem-vu/nhap-excel` | multipart |
| GET | `/api/nhiem-vu/mau-excel` | |
| GET | `/api/nhiem-vu/xuat-so-theo-doi` | xuất bảng Sổ theo dõi |

---

## 11. Quy tắc nghiệp vụ

1. Giao cho **bộ phận** mà quá lâu chưa phân công người cụ thể ⇒ báo lên lãnh đạo (job `Nhắc việc sắp đến hạn và đã quá hạn` cũng quét trường hợp này).
2. `han_ban_dau` chỉ ghi một lần khi tạo; gia hạn chỉ đổi `han_xu_ly`.
3. Tỷ lệ đúng hạn tính trên `ngay_hoan_thanh ≤ han_ban_dau`.
4. Nhiệm vụ cha hoàn thành chỉ khi toàn bộ nhiệm vụ con đã hoàn thành (hiển thị `x/y nhiệm vụ đã hoàn thành`).
5. Xoá nhiệm vụ cần quyền `task.delete`; nên xoá mềm để giữ nhật ký.
6. Thay đổi cấu hình SLA không hồi tố cho nhiệm vụ đã tạo.
7. Nhiệm vụ sinh từ kết luận họp giữ liên kết ngược để trang Biên bản hiển thị `x/y nhiệm vụ đã hoàn thành`.

---

## 12. Dữ liệu mẫu để seed

28 nhiệm vụ `NV01`–`NV34` (không liên tục), phân bố: Chưa thực hiện 7, Đã tiếp nhận 6, Đang thực hiện 11, Chờ duyệt 0, Hoàn thành 4. Trong đó `NV10 — Rà soát tiến độ tuyến đường Hà Lam – Bình Trị` có **3 việc con**; `NV32` có nguồn giao `Từ văn bản đến` (sinh từ đơn thư số 4); `NV26`, `NV02` ở trạng thái `Hoàn thành trễ hạn`.
