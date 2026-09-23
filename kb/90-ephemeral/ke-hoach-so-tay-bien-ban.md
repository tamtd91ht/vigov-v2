---
id: ke-hoach-so-tay-bien-ban
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 1233dd1
expires: 2026-10-23
owns_facts:
  - "thứ tự bung bốn việc của Sổ tay lãnh đạo và Biên bản họp, và vì sao chúng không cùng lượt"
  - "hai điều kiện dừng của hai chương ấy đã được trả lời bằng cách đọc ../vigov-require, kèm bằng chứng"
---

# Kế hoạch — Sổ tay lãnh đạo (ch.03) · Biên bản họp (ch.04)

Soạn **23/09/2026**, chưa giao. Hết hạn **23/10/2026** — quá hạn nghĩa là kế hoạch này không còn
ai dùng, tin `git log` chứ đừng tin tệp này.

**Điều kiện bung:** agent nền nhiệm vụ (`service-petitions/migrations/0006_nhiem_vu.sql` + domain
+ kho + hai tuyến đọc) đã về và xanh.

## KHÔNG PHẢI HAI VIỆC — LÀ BỐN

| # | Việc | Loại | Chờ |
|---|---|---|---|
| 1 | **Biên bản backend** — `bien_ban` · `ket_luan` · liên kết ngược trên `nhiem_vu` | Go | nền nhiệm vụ |
| 2 | **Sổ tay backend** — ba phạm vi lọc cá nhân trên tuyến danh sách nhiệm vụ | Go, **nhỏ** | nền nhiệm vụ |
| 3 | **Biên bản web** — `/nhiem-vu/bien-ban` | web | (1) |
| 4 | **Sổ tay web** — `/nhiem-vu/so-tay` | web | (2) |

**Sổ tay không có dữ liệu riêng** — `docs/ui-ux/03-so-tay-lanh-dao.md:11` nói thẳng: *"Không có
dữ liệu riêng — chỉ là ba truy vấn được trình bày cạnh nhau."* Nên (2) nhỏ, gộp được vào lượt sau
của agent nền nhiệm vụ thay vì một agent riêng.

**Thứ tự, tôn trọng trần máy** (`skills/parallel-agents`: Go 1 đồng thời, tối đa 2 · web 2):

```
lượt A   (1) Biên bản backend  — 1 agent Go
lượt B   (3) + (4) web         — 2 agent web, sau khi (1) và (2) xanh
```

## HAI ĐIỀU KIỆN DỪNG — ĐÃ CÓ ĐÁP, đọc từ `../vigov-require`

Người dùng chỉ đạo 23/09: *"tham khảo bên vigov-require nếu hiện tại chưa có thông tin, để tránh
lệch hướng require."* Cả hai câu dưới đây kho này chưa trả lời được, bên kia đã chạy thật.

### S1 — `bien_ban` / `ket_luan` thuộc service nào → **`service-petitions`**

`kb/30-indexes/data-ownership.json` chưa có `Meeting`/`BienBan`/`KetLuan`, và hai lời đọc đều
đứng được (`petitions` vì kết luận tách thành nhiệm vụ · `documents` vì biên bản là văn bản của
Văn phòng).

**Bằng chứng bên kia:** `meetings` và `conclusions` khai trong
`apps/api/app/modules/tasks/models.py` và `router.py` — **cùng module với nhiệm vụ**, không phải
module văn bản. Lược đồ ở `docs/spec/03-mo-hinh-du-lieu.md`: `meetings` (title · held_at ·
location · chaired_by_id · reference_no · minutes) · `conclusions` (meeting_id · ordinal · content).

**Hệ quả quan trọng:** cùng một service thì liên kết ngược **không** là khoá ngoại xuyên service
(luật 2 cấm #2). Bên kia còn đi xa hơn — `tasks.source_type` + `tasks.source_id` là một **cặp mờ
tổng quát**, không phải cột `conclusion_id` riêng. Hình dạng ấy đáng mượn: nó phục vụ luôn ca
"đơn thư → nhiệm vụ" mà `05-van-ban-don-thu.md` đòi.

### S2 — "Chờ tôi duyệt" đòi quyền theo BỘ PHẬN → **bên kia KHÔNG dùng chiều ấy**

`docs/ui-ux/03-so-tay-lanh-dao.md` §3 khai cột giữa là `trang_thai = cho-duyet` AND
(`lanh_dao_giao_viec_id = me` **OR tôi có quyền `task.approve` với bộ phận đó**). Vế sau **không
biểu diễn được** bằng luật 5 bất biến 3 — phân quyền kiểm `(tenant_id, role, permission)`, không
có chiều bộ phận.

**Bằng chứng bên kia** (`apps/api/app/modules/tasks/service.py`):

| Dòng | Làm gì |
|---|---|
| `:634` | `decider = task.assigner_id or task.created_by` — **người trên bản ghi**, không phải một khoá quyền theo bộ phận |
| `:632-633` | chú thích của họ: *"Về lãnh đạo giao việc, không phải về người bấm nút tạo. Chưa có ai thì rơi về người tạo — thà tới nhầm bàn còn hơn không tới bàn nào"* |
| `:719-723` | luật duy nhất còn lại: **không ai duyệt đề nghị của chính mình** |

Không chỗ nào kiểm quyền theo bộ phận. Đây là **cùng hình dạng với "luật nắm giữ"** đang chờ
khách ở `kb/90-ephemeral/tien-do/service-petitions.json` → `luat-nam-giu-hoi-khach`: thẩm quyền
gắn vào **một người ghi trên bản ghi**, không vào một khoá quyền.

**Vẫn còn một nửa phải hỏi khách**, và đừng lẫn hai nửa: *ai được duyệt* thì bên kia đã trả lời;
*có nới luật 5 để thêm chiều bộ phận không* thì **không**, và đó là câu nên gộp vào cùng lần hỏi
với `luat-nam-giu-hoi-khach` — trả lời một lần được cả hai.

### S3 — "Bao gồm cả nhiệm vụ con" (`03` §3, cột quá hạn) — CÒN MỞ

Cây nhiệm vụ con là điều kiện dừng **agent nền nhiệm vụ đang nêu**. Chưa có đáp thì cột "Việc quá
hạn" chưa khai đúng phạm vi được. Đợi câu ấy, đừng đoán.

## BA ĐÒI HỎI CỦA ĐẶC TẢ, dễ bị bỏ khi dựng web

1. `03` §5 — *"không có bộ lọc, không có ô tìm kiếm, không có nút tạo mới. **Giữ nguyên sự tối
   giản**"*
2. `03` §5 — *"Số liệu phải khớp **tuyệt đối** với `/nhiem-vu` khi lọc tương đương — nếu lệch,
   người dùng mất tin tưởng. **Viết test so sánh hai nguồn**."* Đây là đòi hỏi, không phải gợi ý
3. `04:14` — nhiệm vụ tách ra **vĩnh viễn** mang liên kết `nguồn giao = Từ kết luận họp`; và nút
   `✂ Tách thành nhiệm vụ` **dùng lại nguyên** form Giao việc mới của `02` §7 — import, không
   dựng form thứ hai

## NHẮC CHO PHIÊN CHÍNH

- Kiểm chạy **sau**, một mình. Không `go build`/`go test`/`make kb`/`go mod tidy` khi agent Go
  đang viết
- Sổ tiến độ do **phiên chính** ghi; `kb/90-ephemeral/tien-do/` không nằm trong ranh giới ghi của
  agent nào
- Ranh giới ghi từng agent và danh sách "không được đụng" nằm trong chính đề bài lúc giao — kế
  hoạch này chỉ giữ thứ tự và các câu đã/chưa có đáp

→ Đối chiếu hai kho: `kb/90-ephemeral/doi-chieu-vigov-require.md`
→ Ghi chú lần đối chiếu gần nhất: `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md`
