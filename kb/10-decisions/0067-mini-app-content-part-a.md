---
id: 0067-mini-app-content-part-a
tier: T1
source: CURATED
owner: architecture
derived_from_commit: a3460acb
expires: null
owns_facts:
  - "thân bài nội dung Mini App là HTML đã làm sạch ở máy chủ lúc ghi theo danh sách cho phép (p, br, strong, em, ul, ol, li, h2, h3, a chỉ href https); Mini App dựng từ body_blocks có cấu trúc, không bao giờ dựng HTML; body văn bản thuần giữ cho bản app cũ — thay quyết định 27/09/2026 'không HTML nào tới dân'"
  - "dòng nội dung đã lưu không bị viết lại; dòng cũ chưa làm sạch được làm sạch lại trên tuyến đọc công khai"
  - "đồng bộ Cổng TTĐT: chỉ API cổng dùng chung của Đà Nẵng; https và host đuôi .gov.vn; ảnh chỉ cùng host với api_url; mặc định chờ duyệt; không ghi đè tin đã có, không nhập lại tin đã xoá mềm; giới hạn 90 ngày, 100 tin/lượt, 3 chuyên mục và 6 ảnh song song, nhịp 0–24 giờ mặc định 6"
  - "chuyên mục Cổng (chuyen_muc_cong) là bảng riêng, không gộp với danh mục của xã; không chép cây chuyên mục của Cổng — hỏi Cổng lúc mở cấu hình, chỉ lưu lựa chọn và ánh xạ loại (tin-tuc, su-kien, thong-bao)"
  - "ma_bao_mat của Cổng mã hoá theo xã (ADR 0009), chỉ ghi, hiện dạng che; đổi api_url buộc nhập lại mã"
  - "danh mục nội dung: sửa tên, cha, thứ tự; slug cố định; cờ ẩn/hiện; xoá mềm có lý do, từ chối khi còn nội dung hoặc danh mục con còn sống"
  - "truyền thanh: mp3/m4a, ≤ 30 MB, thời lượng cán bộ gõ, bucket private, link ký ngắn hạn, mục đích tải lên content-audio, người tải cần content.update"
  - "banner: ảnh bắt buộc, tiêu đề là alt, link_to tuỳ chọn, display_order tăng dần theo xã, không lịch, chỉ ở dải trang chủ Mini App, chỉ trả khi ?type=banner"
  - "ẩn danh mục chỉ gỡ chip lọc của nó và của danh mục con trên Mini App; bài vẫn hiện ở Tất cả (chốt 01/10/2026)"
  - "Cổng chuyển hướng: tối đa 3 lần, mỗi đích qua cùng phép kiểm của api_url (chốt 01/10/2026)"
  - "liên kết trong thân bài: hỏi xác nhận rời ứng dụng rồi mở bằng openWebview của Zalo (chốt 01/10/2026)"
---

# 0067. Nội dung Mini App — phần A: thân bài định dạng, đồng bộ Cổng TTĐT, danh mục, truyền thanh, banner

**Trạng thái:** đã chốt · **Ngày:** 2026-10-01 · **Người quyết:** chủ dự án, 01/10/2026 (chọn mọi
phương án đề xuất) · **Thay** quyết định 27/09/2026 *"không HTML nào tới dân"* (quyết định ấy chưa
từng có ADR — xem §1) · **Kết thúc**, khi dựng xong, ngoại lệ ảnh bìa trong bundle của ADR 0047
(`kb/10-decisions/0047-hai-luong-dung-citizen-app-theo-ten-mien.md:251`)

## Bối cảnh

Đặc tả `docs/ui-ux/11-noi-dung-mini-app.md` (§3, §6–§10) để lại năm phần mà màn quản trị đang ghi là
**chưa dựng**, mỗi phần kèm lý do bị chặn (`web-admin/src/features/noi-dung/nhan-noi-dung.ts:757-799`,
`PHAN_CHUA_DUNG`):

| Phần | Vì sao bị chặn tới 30/09 |
|---|---|
| Ô soạn thảo rich text (§7, §8) | Máy chủ lưu thân bài nguyên văn, kho chưa có bộ làm sạch HTML; chỉ quyền `content.update` đứng giữa một thẻ `<script>` và điện thoại của mọi cư dân |
| Đồng bộ Cổng TTĐT (§3) | Thiếu bảng cấu hình, bảng chuyên mục Cổng, bộ lập lịch, adapter HTTP ra ngoài theo xã. Mã hoá `ma_bao_mat` nay đã có (`core/crypto`, ADR 0009 §Sửa đổi 28/09) |
| Sửa / xoá danh mục (§6) | Hợp đồng chỉ có `GET`, `POST` |
| Truyền thanh (§7) | Chưa có lối tải tệp âm thanh |
| Banner (§7) | Chưa có cột, chưa có câu chốt hình dạng (`service-comms/migrations/0011_content_item_media_and_event.sql:15-16`) |

Bản mẫu `../vigov-require` đã chạy phần đồng bộ thật với `thangbinh.danang.gov.vn`
(`apps/api/app/integrations/cttdt/client.py:7-20`). Phần lớn con số mặc định dưới đây lấy từ đó;
chỗ nào ViGov chọn khác thì §6 ghi rõ.

## 1. Thân bài định dạng tới người dân — THAY quyết định 27/09/2026

### Quyết định cũ, và nơi nó nằm

Ngày 27/09/2026 chủ dự án chốt: *"không HTML nào tới dân"*. Quyết định ấy **chỉ được ghi ở hai chỗ,
không có ADR**:

- chú thích mã `service-comms/internal/domain/van_ban_thuan.go:9-16` (`VanBanThuanChoDan` — bộ
  **gỡ** thẻ, không phải bộ làm sạch; tuyến công khai gọi nó ở
  `service-comms/internal/http/tin_xa_cong_khai.go:198-229`);
- một mục sổ tiến độ, `kb/90-ephemeral/tien-do/service-comms.json:158`.

Đó là lý do ADR này ghi lại nó trước khi thay: một quyết định chỉ sống trong chú thích thì không ai
biết nó đã bị thay.

### Phương án

| Phương án | Được | Mất |
|---|---|---|
| Giữ 27/09: mọi thứ ra ngoài là văn bản thuần | Không có mặt tấn công nào; không phải chọn thẻ nào an toàn | Bài của xã mất đầu mục, danh sách, chữ đậm, liên kết. Ô soạn thảo rich text của §7 vô nghĩa. Tin Cổng về thành một khối chữ |
| Làm sạch ở **Mini App**, dựng HTML ở client | Ít việc ở máy chủ | Một lần cập nhật client là quay về `innerHTML`; luật 13 cấm #3; bản app cũ trên máy dân không vá được |
| **Làm sạch ở máy chủ lúc ghi + gửi cấu trúc, không gửi HTML** | Bộ lọc nằm ở một chỗ, kiểm được; client không bao giờ thông dịch markup | Máy chủ dựng thêm một bộ chuyển HTML → khối; hai dạng thân bài phải cùng nhất quán |

### Quyết định

**Phương án 3.** Chủ dự án, 01/10/2026:

| # | Điều |
|---|---|
| 1 | Thân bài **có định dạng tới người dân**. Máy chủ làm sạch **lúc ghi**, theo **danh sách cho phép** (kiểu bluemonday): `p`, `br`, `strong`, `em`, `ul`, `ol`, `li`, `h2`, `h3`, và `a` **chỉ với `href` https**. **Không** `img`, `iframe`, `style`, `script`, không thuộc tính sự kiện |
| 2 | Bộ làm sạch chạy trên **mọi đường ghi** — cán bộ soạn tay và lượt đồng bộ Cổng (§2) như nhau. Không đường ghi nào được bỏ qua nó |
| 3 | Mini App **không bao giờ** dựng HTML (`innerHTML`, `dangerouslySetInnerHTML` — luật 13 cấm #3). Tuyến công khai thêm trường **`body_blocks`**: đoạn · đầu mục · danh sách · dấu định dạng trong dòng · liên kết, dựng **ở máy chủ** từ HTML đã làm sạch |
| 4 | Trường **`body` văn bản thuần giữ nguyên** cho các bản app cũ đang nằm trên máy dân |
| 5 | Dòng đã lưu là hồ sơ lưu trữ (luật 7): **không viết lại**. Dòng cũ chưa qua làm sạch được **làm sạch lại trên tuyến đọc công khai** |
| 6 | Ô soạn thảo cho cán bộ: **Tiptap** (headless), giới hạn đúng các nút của danh sách ở điều 1 |

**Vì sao làm sạch cả lúc đọc (điều 5) dù đã làm lúc ghi:** dòng ghi trước ngày có bộ làm sạch vẫn
còn markup tuỳ ý, và luật 7 không cho viết lại chúng. Làm sạch lại trên đường ra là bức tường thứ hai
cho đúng những dòng ấy — và cho mọi dòng nếu danh sách cho phép sau này hẹp lại.

**Vì sao `body_blocks` chứ không gửi HTML đã làm sạch:** một HTML "đã sạch" vẫn chỉ an toàn khi client
dựng nó bằng một đường an toàn. Gửi cấu trúc thì client không có markup nào để thông dịch — cùng lý do
`van_ban_thuan.go:12-16` đã nêu, nay giữ được định dạng.

## 2. Đồng bộ tin từ Cổng TTĐT

### Phạm vi

**Chỉ API chia sẻ của Cổng dùng chung Đà Nẵng:** `GET {base}/chuyenmuc` và
`GET {base}/tintheochuyenmuc?lstChuyenMuc=<id>&secret_code=<mã>`. Giao diện nhà cung cấp chung
(provider) **để sau** — chưa có Cổng thứ hai để biết hình dạng chung là gì.

### An toàn đường ra (SSRF) và bí mật

| # | Điều |
|---|---|
| 1 | Chỉ `https`; host phải **kết thúc bằng `.gov.vn`** |
| 2 | Ảnh chỉ tải từ **cùng host** với `api_url`; ảnh ở host khác bị **bỏ** |
| 3 | URL bị **che trong nhật ký** — mã bảo mật nằm trong query string |
| 4 | `ma_bao_mat` **mã hoá theo xã** (ADR 0009, `core/crypto`), **chỉ ghi**, trả ra ở dạng che (ADR 0009 điều 6) |
| 5 | **Đổi `api_url` buộc nhập lại mã bảo mật** — mã cũ không được đi theo sang một host khác |

**Vì sao điều 5:** nếu mã cũ được giữ khi đổi địa chỉ, ai sửa được cấu hình là gửi được mã bảo mật
của Cổng xã tới một máy chủ khác — kể cả một host `.gov.vn` không phải của xã.

### Chế độ đăng và chuyên mục

| # | Điều |
|---|---|
| 1 | Mặc định **chờ duyệt** (`cho-duyet`); xã có thể chuyển sang **đăng thẳng** |
| 2 | Một chuyên mục Cổng chỉ ánh xạ vào **`tin-tuc` · `su-kien` · `thong-bao`** (không vào truyền thanh, video, banner như §3 của đặc tả liệt kê) |
| 3 | Chuyên mục Cổng là **bảng riêng `chuyen_muc_cong`**, không bao giờ gộp với `danh_muc_mini_app` của xã — lý do: `service-comms/migrations/0006_noi_dung_mini_app.sql:141-145` |
| 4 | **Không chép cây chuyên mục của Cổng.** Cây được hỏi trực tiếp Cổng mỗi lần mở cấu hình; chỉ lưu **lựa chọn** và **ánh xạ loại**. Câu `0006:72-78` để ngỏ, nay đã đóng |
| 5 | Tin đồng bộ về **hiện tên chuyên mục Cổng** của nó và **chưa xếp danh mục** trong cây của xã |
| 6 | **Giữ dòng ghi nguồn** "Nguồn: …" của Cổng (Cổng đăng lại tin báo khác) |

### Ghi

| # | Điều |
|---|---|
| 1 | **Không bao giờ cập nhật** tin đã có — kể cả bù ảnh |
| 2 | **Không nhập lại** tin đã xoá mềm: khử trùng theo `nguon_id_ngoai` **tính cả dòng đã xoá** — khoá ở `0006:343-358` đã như vậy |
| 3 | Cửa sổ **90 ngày**; trần **100 tin/lượt**; **3 chuyên mục** và **6 ảnh** chạy song song |
| 4 | Nhịp chọn từ **0 (chỉ chạy tay) tới 24 giờ**, mặc định **6 giờ** |
| 5 | Ảnh đi qua **đường ảnh bìa**: ClamAV, bỏ EXIF, bản dẫn xuất public, theo chính sách tải lên `content-image` (ADR 0052 §9, §10; ADR 0047:256) |
| 6 | Một chuyên mục hỏng **không làm hỏng cả lượt** (đặc tả §10.3); lỗi vào **nhật ký theo lượt**, không dữ liệu cá nhân (luật 3) |

### Việc nền

Theo ADR 0058: chạy trong **service sở hữu dữ liệu — `comms`**, một khoá advisory, **chủ thể hệ
thống** cho vết (luật 6 bất biến 6), `tenant_id` nằm **trong đơn vị công việc** (luật 1 bất biến 9).
Cấu hình đồng bộ là của `comms` (cạnh nội dung nó đổ vào), không phải của bảng cấu hình việc nền ở
`identity` mà ADR 0058 §2 dành cho tab Tự động hoá.

## 3. Danh mục nội dung của xã

| # | Điều |
|---|---|
| 1 | Sửa **tên, danh mục cha, thứ tự**. **Slug cố định**. Đổi cha tạo vòng → **từ chối** |
| 2 | Thêm cờ **ẩn/hiện**: ẩn thay cho xoá |
| 3 | Xoá **mềm**, **bắt buộc lý do** (luật 7); **từ chối** khi còn nội dung sống hoặc danh mục con sống — câu từ chối gợi ý ẩn |
| 4 | Slug **không bao giờ cấp lại** — đã đúng ở khoá `0006:184-188` |

**Vì sao từ chối xoá thay vì gỡ danh mục khỏi các bài:** gỡ hàng loạt là sửa âm thầm nhiều hồ sơ một
lúc; ẩn đạt cùng mục đích mà không chạm bài nào.

## 4. Truyền thanh

| # | Điều |
|---|---|
| 1 | **mp3 / m4a**, **≤ 30 MB**, thời lượng **cán bộ gõ** — xác nhận lại G7 (ADR 0047:255) |
| 2 | Lưu ở bucket **private**, phát qua **link ký ngắn hạn**. **Không** đặt bản gốc vào bucket public (ADR 0052 điều kiện dừng #2) |
| 3 | Mục đích tải lên mới của platform: **`content-audio`**; người tải cần **`content.update`** |

**Vì sao không có bản dẫn xuất public như ảnh bìa:** dẫn xuất cần chuyển mã, mà chưa có `ffmpeg`
(§Còn mở). Không có dẫn xuất thì chỉ còn bản gốc, và bản gốc không được vào bucket public.

## 5. Banner

| # | Điều |
|---|---|
| 1 | **Ảnh bắt buộc**; **tiêu đề là `alt`** |
| 2 | `link_to` **tuỳ chọn**: đường trong app hoặc URL https; rỗng = **không bấm được** |
| 3 | `display_order` số nguyên, **tăng dần**, theo xã. **Không lịch** hiện/ẩn theo giờ |
| 4 | Chỉ hiện ở **dải trang chủ Mini App** |
| 5 | Banner **không có** trong danh sách tin công khai mặc định; chỉ trả khi **`?type=banner`** |
| 6 | Thay ảnh bìa theo tên miền nung vào bundle `citizen-app` — ngoại lệ ADR 0047:251 **chấm dứt khi phần này dựng xong** |

**Vì sao loại banner khỏi danh sách mặc định** (cách đọc của người viết ADR): banner là ảnh quảng bá, không phải tin. Trộn vào thì
tab Tin tức của bản app hiện có hiện một "tin" chỉ có ảnh và tiêu đề.

## 6. Chỗ ViGov khác bản mẫu `vigov-require` — có chủ ý

| Bản mẫu | ViGov | Vì sao |
|---|---|---|
| Thân bài thành chữ thuần, ngày cần định dạng thì thêm bộ lọc (`modules/content/sync.py:8-13`) | Làm sạch theo danh sách cho phép, gửi `body_blocks` | Chính "ngày ấy" của bản mẫu — §1 |
| Đăng thẳng mặc định (`modules/content/models.py:280-281`) | **Chờ duyệt** mặc định | Tin tự về mang tên một cơ quan nhà nước; mặc định là có người xem trước |
| Chép chuyên mục Cổng thành danh mục của mình (`sync.py:82-89`) | Bảng riêng, không chép cây | `0006:141-145` |
| Bù ảnh vào tin đã có (`sync.py:169-186`) | Không bao giờ cập nhật tin đã có | Một lời hứa đơn giản kiểm được; `da_sua_tay` (`0006:319-326`) khỏi phải phân biệt ô nào được chạm |
| `follow_redirects=True`, ảnh lấy từ mọi host (`integrations/cttdt/client.py:110-113`) | https, `.gov.vn`, ảnh cùng host | SSRF — máy chủ ViGov không đi tới nơi Cổng chỉ |
| `pin_order` số lớn đứng trước (`models.py:157-159`) | `display_order` tăng dần | Chủ dự án chọn, 01/10/2026 — không có lý do kỹ thuật nào ép chiều nào |

## Hệ quả

- **Dễ hơn:** cán bộ soạn bài có định dạng; xã không gõ lại tin đã đăng trên Cổng; ảnh bìa trang chủ
  thôi nằm trong bundle.
- **Khó hơn:** `comms` mang thêm một bộ làm sạch, một bộ chuyển HTML → `body_blocks`, một adapter HTTP
  ra ngoài, một việc nền. Hai dạng thân bài (`body`, `body_blocks`) phải giữ khớp nhau.
- **Phải đổi theo, khi dựng:** chú thích `van_ban_thuan.go:9-16` (nó vẫn khai quyết định 27/09) và các
  mục tương ứng của `PHAN_CHUA_DUNG`. Một chú thích còn nói "không HTML" cạnh mã đã gửi định dạng là
  hai nguồn cho một sự thật.
- **Rủi ro chấp nhận:** host `.gov.vn` không đồng nghĩa với host của xã — một cán bộ có quyền sửa cấu
  hình vẫn trỏ được sang Cổng của đơn vị khác. Điều 5 §2 giữ mã bảo mật không đi theo.

## Còn mở / chưa làm

| # | Việc | Của ai |
|---|---|---|
| 1 | Nhà cung cấp Cổng khác ngoài Đà Nẵng, và giao diện provider chung | Chủ dự án, khi có xã thứ nhất ngoài Đà Nẵng |
| 2 | Bản dẫn xuất / chuyển mã âm thanh — chưa có `ffmpeg` | Devops / chủ dự án |
| 3 | Bốn việc ADR 0009 còn nợ (`kb/10-decisions/0009-per-tenant-secret-encryption.md:148-155`) **vẫn nợ** — đồng bộ Cổng thêm một bí mật nữa phụ thuộc vào chúng | Chủ dự án / vận hành |
| 4 | Ẩn một danh mục có ẩn luôn tin trong nó trên Mini App không — §3 chỉ chốt cờ, chưa chốt hệ quả ở tuyến công khai | **Đã đóng** 01/10/2026 → §Chốt bổ sung, A1 |
| 5 | Cổng trả chuyển hướng (redirect) thì xử lý ra sao — cách đọc của người viết: đích chuyển hướng phải qua cùng phép kiểm §2, nhưng chưa ai chốt | **Đã đóng** 01/10/2026 → §Chốt bổ sung, A2 |
| 6 | Mini App mở một liên kết `a` trong thân bài thế nào, và câu khai đích ra ngoài trong chính sách quyền riêng tư (ADR 0047:258) | **Đã đóng** 01/10/2026 → §Chốt bổ sung, A3 (câu chữ chính sách còn chờ xác nhận — mục C1) |

## ĐIỀU KIỆN DỪNG

1. Một đường ghi thân bài bỏ qua bộ làm sạch, hoặc mở rộng danh sách cho phép (thêm `img`, `iframe`,
   thuộc tính, scheme khác https)
2. Mini App dựng HTML ở bất kỳ đâu (luật 13 cấm #3)
3. Viết lại hàng loạt các dòng nội dung đã lưu để "làm sạch cho đồng bộ" (luật 7)
4. Nới phép kiểm host của Cổng, hoặc tải ảnh từ host khác `api_url`
5. Đưa bản gốc âm thanh vào bucket public (ADR 0052 điều kiện dừng #2)

→ ADR 0009 (bí mật theo xã) · ADR 0047 (citizen-app, G7, ngoại lệ banner) · ADR 0052 (kho tệp) ·
ADR 0058 (việc nền) · luật 1, 3, 6, 7, 13

## Chốt bổ sung — 01/10/2026, sau khi dựng

Phần trên giữ nguyên. Mục này ghi (A) ba câu chủ dự án trả lời sau ADR, (B) những chỗ khi dựng phải
chọn mà phần trên chưa nói — chỉ phần **vì sao**; cách làm nằm ở mã được dẫn — và (C) việc còn mở.
Dựng ở `abf0ca1d`, `026ae398`, `cce556ec`, `07bec99d`, `75ea3b5c`, `2fca3658`/`a7e7d364`, `fa7b8377`.

### A. Chủ dự án chốt, 01/10/2026

| # | Câu hỏi (Còn mở) | Chốt | Ở mã |
|---|---|---|---|
| A1 | #4 — ẩn danh mục | Ẩn **chỉ gỡ chip lọc** của danh mục ấy và của mọi danh mục con trên Mini App. Bài vẫn hiện ở "Tất cả" và vẫn mở được từng bài | `service-comms/internal/domain/public_category.go:50-55` |
| A2 | #5 — Cổng chuyển hướng | Theo **tối đa 3** lần; mỗi đích phải qua **đúng phép kiểm của `api_url`** (https, host đuôi `.gov.vn`, cổng 443, IP công khai lúc quay số). Trái điều nào → lượt chạy ghi lỗi | `service-comms/internal/portal/client.go:48-50`, `:152-161` |
| A3 | #6 — liên kết trong thân bài | Bấm → hỏi *"Bạn sắp rời ứng dụng để mở <tên miền>"* → mở trong **trình duyệt trong Zalo** (`openWebview`). Chính sách quyền riêng tư thêm một câu | `citizen-app/src/cong-dan/man/leave-app.tsx:49`; `citizen-app/src/features/tinh-nang/zalo-api.ts:573-582`; `citizen-app/src/content/chinh-sach-rieng-tu.ts:471` |

**Vì sao A1 chỉ gỡ chip:** ẩn là để thay xoá (§3 điều 2). Nếu ẩn làm bài biến mất thì ẩn một danh
mục thành gỡ hàng loạt bài khỏi tay dân — đúng điều §3 đã từ chối. Danh mục con cũng mất chip vì
một chip con mất cha sẽ bị đẩy lên hàng đầu, chỗ xã chưa bao giờ đặt nó.

**Vì sao A3 hỏi trước:** dân phải biết mình đang rời ứng dụng của cơ quan nhà nước sang một tên miền
khác — một liên kết trong bài của xã trông như lời xã bảo đảm cho trang đích.

### B. Chọn khi dựng — điều đáng giữ là lý do

**Thân bài (§1)**

| # | Điều | Vì sao | Ở mã |
|---|---|---|---|
| B1 | Bộ làm sạch là gói riêng `service-comms/internal/richtext`, **không** ở `internal/domain` | `domain` chỉ dùng thư viện chuẩn; bộ làm sạch tự viết trên thư viện chuẩn là cách bộ làm sạch bị viết sai. Một hàm duy nhất, nên không đường ghi nào mang chính sách thứ hai | `service-comms/internal/richtext/richtext.go:13-17`; `service-comms/go.mod:9` (bluemonday v1.0.27) |
| B2 | Mọi liên kết còn lại mang `rel="noopener noreferrer nofollow"`, bất kể đầu vào | Trang đích không điều khiển được trang mở nó; không lộ địa chỉ quản trị; trang cơ quan nhà nước không cho trang lạ mượn thứ hạng | `richtext/richtext.go:29-32` |
| B3 | Markup đến ở dạng mã thực thể (`&lt;script&gt;`) bị **bỏ** khỏi `body_blocks` | Để `body_blocks` khớp `body` văn bản thuần — hai dạng thân bài phải nói cùng một điều (§Hệ quả) | `richtext/blocks.go:82` |

**Truyền thanh (§4)**

| # | Điều | Vì sao | Ở mã |
|---|---|---|---|
| B4 | Tệp **gắn vào bài lúc hoàn tất tải lên**, cùng thời lượng cán bộ gõ, trong **một giao dịch** | Chính sách `content-audio` chỉ cho **một tệp sống mỗi bài**: tệp hoàn tất mà chưa gắn giữ mất chỗ duy nhất; CHECK "có cả hai hoặc không có gì" đòi tệp và thời lượng trong cùng một lần ghi | `service-comms/internal/app/content_audio.go:17-24`; `service-comms/migrations/0012_content_item_audio_banner.sql:138` |
| B5 | Thay âm thanh = **gỡ rồi tải lại** | Một chỗ duy nhất; tệp cũ xoá mềm, không bao giờ hai tệp sống trên một bài | `content_audio.go:25-27` |
| B6 | Link công khai = GET ký trước **15 phút** tới bản gốc private, **không tên tệp**, **không gắn danh tính** | Luật 4 bất biến 7 gắn danh tính cho **tệp đính kèm của công dân**; đây là tin xã phát cho mọi cư dân, trên tuyến không có phiên để gắn. Không tên tệp: tên cán bộ đặt cho tệp không tới URL của dân | `content_audio.go:83`, `:782-787` |
| B7 | m4a chỉ nhận khi **có rãnh âm thanh và không có rãnh hình** | Video mang nhãn M4A vẫn là video. Hệ quả đã biết: tệp âm thanh major brand `isom` không có nhãn M4A bị **từ chối nhầm** — danh sách đóng chọn từ chối nhầm hơn là đoán | `core/storage/audio.go:20-21`; `core/storage/mime.go:69-77` |
| B8 | Trình phát tua bằng **hai nút ±15 giây**, không thanh kéo | `citizen-app/src/phase1-collects-nothing.test.ts:397-399` chỉ cho ô nhập ở đúng hai tệp; `<input type="range">` là ô nhập, thanh ARIA tự dựng là cùng thứ ấy né phép kiểm. Muốn thanh kéo → chủ dự án quyết về phép kiểm ấy (C6) | `citizen-app/src/cong-dan/man/broadcast-player.tsx:240-242` |

**Banner (§5)**

| # | Điều | Vì sao | Ở mã |
|---|---|---|---|
| B9 | Đường trong app mà banner mở được là **bảng đóng** ở Mini App; web-admin gợi ý đúng danh sách ấy; một phép kiểm đỏ khi hai bên lệch | Máy chủ nhận mọi đường `/…`; đường không dẫn tới màn nào là nút bấm không mở gì trên trang chủ cơ quan nhà nước — tệ hơn không bấm được | `citizen-app/src/cong-dan/man/TrangXa.tsx:183-194`; `web-admin/src/features/noi-dung/nhan-noi-dung.ts:634`; `web-admin/src/features/noi-dung/banner-paths.test.ts:27-33` |
| B10 | Banner không có ảnh bìa (dòng cũ) bị **bỏ qua** ở dải công khai | Ràng buộc "banner phải có ảnh" là trigger, không phải CHECK, để không vỡ vì dòng cũ; dòng cũ thì tuyến đọc bỏ — một ô trống trên màn hình mọi cư dân là lỗi | `service-comms/internal/http/tin_xa_cong_khai.go:545-546`; `0012_content_item_audio_banner.sql:239-253` |
| B11 | Ảnh banner theo tên miền nung trong bundle **giữ làm dự phòng** tới khi mọi xã đã đăng banner — ngoại lệ ADR 0047:251 chưa đóng hẳn (khác §5 điều 6) | Gỡ sớm thì trang chủ của xã chưa đăng banner thành trống | `TrangXa.tsx:161-167` |

**Đồng bộ Cổng (§2)**

| # | Điều | Vì sao | Ở mã |
|---|---|---|---|
| B12 | **Chỉ cổng 443**; IP kiểm **lúc quay số**, nối tới đúng IP đã kiểm | Kiểm tên thôi không đủ: `x.gov.vn` có thể trỏ đi bất cứ đâu, và câu trả lời DNS đổi được giữa lúc kiểm và lúc nối (rebinding). NetworkPolicy mở 443 ra mọi nơi, nên hai phép kiểm này mới là ranh giới | `service-comms/internal/portal/guard.go:6-19`, `:36-38` |
| B13 | Lỗi chỉ mang **lớp lỗi**, lỗi gốc bị bỏ | Mã bảo mật nằm trong query string; lỗi của `net/http` trích nguyên URL vào nhật ký, nhật ký lượt chạy hay một 500 | `service-comms/internal/portal/errors.go:13-15` |
| B14 | Ảnh: chỉ lưu bản dẫn xuất `thumb-1280` đã mã hoá lại, **không lưu bản gốc** | `core/storage` cố ý từ chối bản gốc do máy chủ ghi (ADR 0052); Cổng tự giữ bản của nó. Nhận bản gốc tải về là sửa danh sách luồng của ADR 0052 — việc của chủ dự án | `service-comms/internal/app/portal_image.go:9-15` |
| B15 | Trần **10 MiB mỗi ảnh** khi tải | Giới hạn **bộ nhớ**, không phải con số của khách: 6 ảnh song song × 50 MB của chính sách vượt pod | `portal_image.go:21-22`, `:43-46` |
| B16 | Khoá khử trùng có tiền tố nhà cung cấp: `cttdt-danang:<TinTucID>` | `UNIQUE (tenant_id, nguon_id_ngoai)` là toàn bộ việc khử trùng; Cổng thứ hai trùng id sẽ bị đọc là "đã nhập" | `service-comms/internal/domain/portal_sync.go:345-351` |
| B17 | Ghi nguồn là **đoạn cuối thân bài**, lấy từ `NguonTin`; `TacGia` **không bao giờ đọc** | `TacGia` nêu tên một người (luật 3). Đặt trong thân bài, không ở tóm tắt — tóm tắt là dòng xem trước | `service-comms/internal/portal/client.go:288-289`; `portal_sync.go:353-361`; `service-comms/internal/app/portal_sync_runner.go:756-763` |
| B18 | `nguon_url` để **NULL** | API Cổng không trả địa chỉ bài; một địa chỉ đoán là một liên kết sai trong hồ sơ lưu trữ | `service-comms/internal/store/portal_sync.go:453-454` |
| B19 | Mọi tin nhập ghi vết bằng **chủ thể hệ thống**, kể cả lượt chạy tay | Bài là của Cổng, hành vi là của lượt chạy; người bấm nút nằm trên dòng lượt chạy và vết bắt đầu của nó | `portal_sync_runner.go:719-721` |
| B20 | Chế độ `dang-thang`: `published_at` = lúc nhập | G1 (ADR 0047 §6): lần đăng đầu chính là hành vi nhập; ngày Cổng đăng nằm ở `ngay_dang` | `portal_sync_runner.go:769-783` |

### C. Còn mở sau khi dựng

| # | Việc | Của ai |
|---|---|---|
| C1 | Câu chữ câu thêm vào chính sách quyền riêng tư (`chinh-sach-rieng-tu.ts:471`) — **chờ chủ dự án xác nhận** | Chủ dự án |
| C2 | Tên chuyên mục Cổng trong danh sách của cán bộ — hợp đồng danh sách chưa có `portal_category_id`, nên §2 "Chế độ đăng" #5 chưa đủ ở màn quản trị | Kỹ thuật |
| C3 | Thử trên máy Zalo thật: trình phát, liên kết, danh sách trắng nguồn media | Kỹ thuật / chủ dự án |
| C4 | Chưa chạy với PG, MinIO, clamd và Cổng thật | Kỹ thuật / vận hành |
| C5 | Đối tượng dẫn xuất mồ côi nằm lại trong bucket tới khi có worker dọn của ADR 0052 §6 (`service-comms/internal/store/stored_file.go:259`) | Chủ dự án / kỹ thuật |
| C6 | Thanh kéo tua cho trình phát — chỉ khi chủ dự án nới phép kiểm ở B8 | Chủ dự án |
