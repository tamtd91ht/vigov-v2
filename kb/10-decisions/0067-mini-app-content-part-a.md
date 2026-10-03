---
id: 0067-mini-app-content-part-a
tier: T1
source: CURATED
owner: architecture
derived_from_commit: c0e068cc
expires: null
owns_facts:
  - "thân bài nội dung Mini App là HTML đã làm sạch ở máy chủ lúc ghi theo danh sách cho phép (p, br, strong, em, ul, ol, li, h2, h3, a chỉ href https); Mini App dựng từ body_blocks có cấu trúc, không bao giờ dựng HTML; body văn bản thuần giữ cho bản app cũ — thay quyết định 27/09/2026 'không HTML nào tới dân'. Từ 03/10/2026 danh sách ấy là chính sách ghi của đường Cổng; bài cán bộ soạn và đường đọc theo chính sách cán bộ (§Sửa đổi 03/10/2026, K1, K7)"
  - "bài cán bộ soạn có ảnh xen giữa các đoạn (tải lên hoặc dán link https), ảnh có chú thích tuỳ chọn, trích dẫn, dòng tác giả/nguồn chữ tự do; sapo là ô Tóm tắt hiện in đậm dưới tiêu đề; trần 20 ảnh thân bài mỗi bài ở upload_policy; app chung hiện ảnh thân bài như app riêng (chốt 03/10/2026)"
  - "hai chính sách làm sạch, một cho mỗi nguồn ghi: chính sách cán bộ cho POST/PATCH của cán bộ và cho làm sạch lại trên đường đọc (ảnh chỉ giữ khi mã tệp đổi ra đúng tệp); chính sách cũ cho đường ghi của Cổng — thay B1 'một hàm duy nhất' (03/10/2026)"
  - "ảnh thân bài lưu MÃ TỆP, không lưu URL; máy chủ đổi mã ra URL công khai lúc đọc, chỉ cho tệp của đúng xã, đúng bài, mục đích content-body-image; link ảnh dán vào do MÁY CHỦ tải về qua rào SSRF, quét mã độc, bỏ EXIF — người dân không tải ảnh từ host lạ (03/10/2026)"
  - "ảnh trong thân tin đồng bộ từ Cổng TTĐT để đợt sau; chính sách làm sạch của đường Cổng không đổi (03/10/2026)"
  - "dòng nội dung đã lưu không bị viết lại; dòng cũ chưa làm sạch được làm sạch lại trên tuyến đọc công khai"
  - "đồng bộ Cổng TTĐT: chỉ API cổng dùng chung của Đà Nẵng; https và host đuôi .gov.vn; ảnh chỉ cùng host với api_url; mặc định chờ duyệt; không ghi đè tin đã có, không nhập lại tin đã xoá mềm; 90 ngày và 100 tin/lượt là mặc định và TRẦN (1..90, 1..100), trần 30 chuyên mục được chọn mỗi xã, dòng vượt trần bị kẹp lúc đọc/chạy chứ không viết lại (chốt 02/10/2026, D1); 3 chuyên mục và 6 ảnh song song, nhịp 0–24 giờ mặc định 6"
  - "chuyên mục Cổng (chuyen_muc_cong) là bảng riêng, không gộp với danh mục của xã; không chép cây chuyên mục của Cổng — hỏi Cổng lúc mở cấu hình, chỉ lưu lựa chọn và ánh xạ loại (tin-tuc, su-kien, thong-bao)"
  - "ma_bao_mat của Cổng mã hoá theo xã (ADR 0009), chỉ ghi, hiện dạng che; đổi api_url buộc nhập lại mã"
  - "danh mục nội dung: sửa tên, cha, thứ tự; slug cố định; cờ ẩn/hiện; xoá mềm có lý do, từ chối khi còn nội dung hoặc danh mục con còn sống"
  - "truyền thanh: mp3/m4a, ≤ 30 MB, thời lượng cán bộ gõ, bucket private, link ký ngắn hạn, mục đích tải lên content-audio, người tải cần content.update"
  - "banner: ảnh bắt buộc, tiêu đề là alt, link_to tuỳ chọn, display_order tăng dần theo xã, không lịch, chỉ ở dải trang chủ Mini App, chỉ trả khi ?type=banner"
  - "ẩn danh mục chỉ gỡ chip lọc của nó và của danh mục con trên Mini App; bài vẫn hiện ở Tất cả (chốt 01/10/2026)"
  - "Cổng chuyển hướng: tối đa 3 lần, mỗi đích qua cùng phép kiểm của api_url VÀ ở lại đúng host của api_url — cho cả lời gọi API lẫn ảnh (chốt 02/10/2026, D4, hẹp lại A2 của 01/10)"
  - "tuyến đọc tin công khai Mini App: 120 yêu cầu/phút mỗi (host, mạng khách — IPv4 hoặc IPv6 /64), 429 + Retry-After; Redis không hỏi được thì CHO QUA (ngoại lệ riêng của chính sách này), đăng nhập vận hành vẫn đóng (chốt 02/10/2026, D2)"
  - "xem cây chuyên mục Cổng trực tiếp cần content.update, không phải content.read (chốt 02/10/2026, D3)"
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

> **Đã sửa 03/10/2026** — điều 1 (phần "Không `img`") và điều 3 (các loại khối) **không còn đúng cho bài
> cán bộ soạn**: xem §Sửa đổi 03/10/2026. Với đường Cổng và làm sạch lại trên đường đọc, điều 1 vẫn
> nguyên hiệu lực. Chữ của bảng trên giữ nguyên làm hồ sơ.

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
   thuộc tính, scheme khác https) — **đã thay 03/10/2026** bằng điều kiện dừng của §Sửa đổi 03/10/2026
   (chủ dự án đã cho thêm ảnh vào chính sách cán bộ; chữ điều này giữ làm hồ sơ)
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

## Chốt của chủ dự án — 02/10/2026 (sau rà cô lập + rà bảo mật)

Phần trên, kể cả §Chốt bổ sung 01/10, giữ nguyên chữ. Mục này ghi bốn câu chủ dự án chốt sau hai lượt
rà (mỗi câu chọn đúng phương án đề xuất), rồi lý do của các bản vá đi kèm mà chủ dự án để "làm theo đề
xuất". Dựng ở `300301fb` (comms, core, deploy) và `8b62b914` (citizen-app).

### D. Chủ dự án chốt, 02/10/2026

| # | Chốt | Vì sao / cái giá | Ở mã |
|---|---|---|---|
| D1 | 90 ngày và 100 tin/lượt là **TRẦN**, không chỉ mặc định: cửa sổ 1..90, số tin 1..100. Thêm trần **30 chuyên mục được chọn** mỗi xã (lưu vượt → 422 `too_many_categories`). Dòng đã lưu vượt trần bị **kẹp lúc đọc/chạy, không viết lại** | Luật 7: không viết lại hàng loạt. CHECK của migration 0013 (1..3650 ngày, 1..1000 tin) **giữ nguyên** — migration đã áp, checksum đã ghi; **tầng domain là tầng chặn**. Thu hẹp CHECK sau này là việc của người sở hữu migration dữ liệu, khi không còn dòng vượt | `service-comms/internal/domain/portal_sync.go:44-64`, `:150-157` (`Clamped`), `:307-315`; `service-comms/internal/app/portal_sync_runner.go:631-637` |
| D2 | Ba tuyến đọc tin công khai của Mini App: **120 yêu cầu/phút** mỗi khách (địa chỉ IPv4 / mạng IPv6 /64), khoá `t:<tenant>:rl:public-news:host:<host>:ip:<net>`, vượt → **429 + `Retry-After`**. Redis không hỏi được → **CHO QUA**, chỉ cho các tuyến nội dung công khai này, một cảnh báo bảo mật mỗi phút. Đăng nhập vận hành **vẫn đóng** | Đây là **ngoại lệ có chủ ý** với mặc định fail-closed của bộ giới hạn: thứ nó chặn là tải trên bảng tin xã đã công bố cho mọi cư dân, và một sự cố Redis biến bảng tin mọi xã thành 503 là hỏng nặng hơn. Ngoại lệ là trường của **từng chính sách**, không phải chế độ của gói — chính sách khác không được chép nếu không có quyết định tương tự. **Cái giá:** cư dân sau cùng một IPv4 NAT nhà mạng dùng chung một hạn mức; tra host chạy **trước** khi đếm (phải biết xã mới dựng khoá) — bộ nhớ đệm theo host, **không nhớ lỗi**, giới hạn chi phí ấy. Host không thuộc xã nào đếm theo khoá **không tiền tố xã**, cùng ngưỡng, để 429 không lộ tên miền nào là của xã | `core/ratelimit/ratelimit.go:11-13`, `:46-52`, `:64-74`, `:85-88`; `core/ratelimit/middleware.go:91-92`; `service-comms/internal/http/tin_xa_cong_khai.go:401-435`; `core/tenant/cache.go:109-112` |
| D3 | Xem **cây chuyên mục Cổng trực tiếp** cần `content.update`, dù là GET | Lời gọi dùng **mã bảo mật của xã** để gọi ra ngoài; tiền lệ: gửi thư thử ở cài đặt thư | `service-comms/internal/http/routes_portal_sync.go:15-16`, `:92-108` |
| D4 | Chuyển hướng của **API Cổng** phải ở lại **đúng host của `api_url`** (như ảnh đã vậy từ §2 điều 2). Sang host khác → lời gọi bị từ chối, lượt ghi lỗi theo chuyên mục | Mã bảo mật nằm trong query string; chuyển hướng là Cổng chọn nơi lời gọi kế tiếp đi tới. "Một host `.gov.vn` khác" không còn đủ. `sameHost` rỗng → từ chối mọi bước (đóng khi quên) | `service-comms/internal/portal/client.go:49-56`, `:158-175` |

**D4 THAY A2** (§Chốt bổ sung 01/10). Chữ của A2 giữ nguyên làm hồ sơ; từ 02/10/2026 điều có hiệu lực
là: tối đa 3 lần, mỗi đích qua phép kiểm của `api_url` **và** cùng host với nó. Dòng `owns_facts` về
chuyển hướng đã đổi theo.

**B12 hẹp lại theo E4 (dưới):** câu "NetworkPolicy mở 443 ra mọi nơi" của B12 không còn đúng hẳn —
luật 7c nay trừ các dải nội bộ. Phép kiểm lúc quay số vẫn là ranh giới chính; mạng là lớp thứ hai.

### E. Bản vá đi kèm — điều đáng giữ là lý do

| # | Điều | Vì sao | Ở mã |
|---|---|---|---|
| E1 | Mỗi chuyên mục giữ một **heap N tin mới nhất CHƯA NHẬP**; sổ tin đã có (tính cả dòng xoá mềm) được hỏi **theo lô 200 trước khi** một tin vào heap | Nếu heap giữ N tin mới nhất bất kể đã nhập, một tồn đọng lớn hơn trần một lượt không bao giờ vơi: lượt nào cũng thấy lại đúng N tin đã có. Hỏi trước heap thì mỗi lượt nhập N tin mới nhất chưa có, lượt sau đi tiếp bên dưới. Bộ nhớ = heap + một lô | `service-comms/internal/portal/client.go:329-347`, `:377`, `:409-494`; `portal_sync_runner.go:40-44` |
| E2 | Mỗi tiến trình **tối đa 2 lượt đồng thời** (lịch và tay dùng chung); quỹ **10 phút/lượt**, **30 phút/nhịp lịch**; xã đến hạn xếp `last_run_at NULLS FIRST` | Một Cổng chậm không được giữ khoá lịch hàng giờ; N xã không được giữ N lần bộ nhớ một lượt (mỗi lượt tới 3 thân chuyên mục và 6 ảnh). Đây là giới hạn **của nhà cung cấp**, không phải con số của khách. Lượt tay không chờ chỗ: 503 `portal_sync_busy` — sức chứa tiến trình, không phải dữ liệu của xã, nên không 409. Xếp cũ nhất trước để xã bị cắt ở nhịp này đứng đầu nhịp sau, không xã nào bị bỏ đói | `portal_sync_runner.go:27-38`, `:93-101`; `service-comms/internal/store/crosstenant/portal_sync.go:48` |
| E3 | Sự kiện bảo mật **`outbound_url_refused`** cho mọi đích ra ngoài bị từ chối (URL, địa chỉ phân giải, chuyển hướng) | `skills/security-logging`: từ chối SSRF là sự kiện an ninh, không chỉ là lỗi lượt chạy. **Không bao giờ** ghi URL (query mang mã), host cán bộ gõ hay trường bài | `portal_sync_runner.go:305-310`; `service-comms/internal/portal/errors.go:64-68` |
| E4 | Luật mạng 7c trừ RFC 1918, link-local (`169.254.0.0/16`, metadata) và CGNAT (`100.64.0.0/10`) khỏi đường 443 | Lớp thứ hai cho đúng thứ phép kiểm lúc quay số đã chặn: mã sai một lần thì mạng vẫn không cho 443 vào cụm hay vào metadata. Cổng xã là máy công khai nên không mất gì | `deploy/base/mang/netpol.yaml:389-412` |
| E5 | Chặn thêm dải IPv6 **IPv4-compatible `::/96`** và **IPv4-translated `::ffff:0:0:0/96`** | Không chặn thì một câu trả lời v6 gọi tên được một máy v4 nội bộ. IPv4-mapped không cần dòng riêng | `service-comms/internal/portal/guard.go:86-95` |
| E6 | Liên kết trong thân bài có `@` trong phần authority bị **bỏ** | `https://gov.vn@other.example` hiện tên một host mà dân không bao giờ tới — câu hỏi xác nhận của A3 thành nói sai. `@` trong đường dẫn không phải userinfo nên vẫn nhận | `service-comms/internal/richtext/richtext.go:140`; `service-comms/internal/domain/noi_dung_mini_app.go:359` |
| E7 | Mã bảo mật niêm phong gắn AAD = bảng/cột + **xã** + **sha256(`api_url`)**. Dòng niêm phong trước 02/10 mở **một lần** bằng AAD cũ (chỉ xã) rồi **niêm phong lại**, cùng vết `ActionResealPortalKey` | §2 điều 5 chỉ chặn ở màn hình; gắn `api_url` vào AAD thì một lần ghi đi vòng qua màn hình cũng không gửi được mã sang host mới. Băm chứ không phải URL: độ dài AAD không phụ thuộc chữ gõ. Có đường lùi vì không gì trong kho chứng minh chưa xã nào lưu mã từ `fa7b8377`; không có nó thì những xã ấy hỏng `credential-unavailable` tới khi gõ lại mã. **Gỡ đường lùi khi không còn dòng cũ** (đếm vết reseal). Chú thích của 0013 còn tả AAD cũ — migration đã áp, hàm là nguồn sự thật | `portal_sync_runner.go:240-264`, `:266-290` |
| E8 | Nhập hỏng → **xoá bản dẫn xuất private mồ côi** của ảnh đã chuẩn bị, sau khi kiểm không dòng `stored_file` nào giữ nó; xoá hỏng thì ghi theo khoá đối tượng | Thu hẹp C5 cho đường đồng bộ: không để rác trong bucket chờ worker dọn của ADR 0052 §6. Khoá đối tượng chỉ chứa xã, ngày, hai id ngẫu nhiên — không người nào | `portal_sync_runner.go:1000-1006` |
| E9 | Cửa mở ra ngoài của citizen-app chỉ nhận **https, có host, không userinfo** cho liên kết trong bài (`lien-ket-xa`) và **video**; URL trong danh thiếp quét được chỉ có nút mở khi là **http(s)** — `javascript:`/`intent:`/`data:` hiện dạng chữ | Cửa là chỗ duy nhất mọi lối ra đi qua: người gọi sau quên kiểm thì cửa vẫn chặn (phòng thủ chiều sâu, không phải vá lỗ đang mở). Trường `URL` của vCard là chữ người làm thẻ gõ, trước đó tới `openWebview` không kiểm | `citizen-app/src/features/tinh-nang/mo-ra-ngoai.ts:40-85`; `citizen-app/src/features/tinh-nang/danh-thiep.ts:113-118` |

### F. Còn mở sau 02/10

| # | Việc | Của ai |
|---|---|---|
| F1 | Chạy với PG và Cổng thật (nối tiếp C4) — các bản vá trên chưa chạy trên cụm thật | Kỹ thuật / vận hành |
| F2 | Chỉ mục cho bộ lọc `status` của danh sách nội dung — chỉ thêm nếu đo thấy chậm | Kỹ thuật |
| F3 | QR chỉ chứa một liên kết trần (`ma-qr`) vẫn nhận `http:` (`danh-thiep.ts:113`, `LA_LIEN_KET`) — E9 chưa siết tới đây | Chủ dự án / kỹ thuật |

## Sửa đổi 03/10/2026 — ảnh trong thân bài và bố cục kiểu tin báo

Phần trên, kể cả §Chốt bổ sung 01/10 và §Chốt 02/10, giữ nguyên chữ. Chủ dự án chốt cho menu
`noi-dung-mini-app` (tính năng *"chèn ảnh vào khung nội dung và văn phong theo cấu trúc từng đoạn như
tin tức trên báo"*). Mục này **thay**, cho bài cán bộ soạn:

| Chỗ bị thay | Thay bằng |
|---|---|
| §1 quyết định 1 — "Không `img`" | H1, H2, K1, K2 |
| §1 quyết định 3 — các loại khối của `body_blocks` | K4 |
| B1 — "một hàm duy nhất, nên không đường ghi nào mang chính sách thứ hai" | K1: **một hàm cho mỗi nguồn ghi** |
| Điều kiện dừng #1 | Điều kiện dừng ở cuối mục này |

Đường ghi của Cổng (§2) **không đổi**. Làm sạch lại trên đường đọc (§1 quyết định 5) vẫn chạy cho mọi dòng, nay bằng chính sách cán bộ (K7). Chưa dựng — "Ở mã"
dưới đây trỏ tới chỗ sẽ phải đổi hoặc chỗ làm lý do.

### H. Chủ dự án chốt, 03/10/2026

| # | Chốt | Vì sao / cái giá | Ở mã / dẫn |
|---|---|---|---|
| H1 | Ảnh trong thân bài, **xen giữa các đoạn**: **tải lên từ máy** và **dán link https** | Bố cục tin báo đặt ảnh cạnh đoạn nó minh hoạ | — |
| H2 | Bố cục tự do, hỗ trợ: **đoạn dẫn (sapo)**, **ảnh có chú thích**, **dòng tác giả/nguồn cuối bài**, **trích dẫn** | — | — |
| H3 | Chú thích ảnh **tuỳ chọn** | — | — |
| H4 | Ảnh trong thân tin **đồng bộ từ Cổng TTĐT: để đợt sau**. Chính sách làm sạch của đường Cổng **không đổi** | Ảnh của Cổng nằm ở mọi host; nhận chúng là mở lại §2 điều 2 và điều kiện dừng #4 | `service-comms/internal/app/portal_sync_runner.go:972` |
| H5 | Link ảnh dán vào: **MÁY CHỦ tải về và lưu lại** — qua rào SSRF, quét mã độc, bỏ EXIF, như ảnh tải lên | **Người dân không bao giờ tải ảnh từ một host lạ**: máy dân chỉ nối tới kho công khai của ViGov. Cái giá: một đường gọi ra ngoài mới (K6) | — |
| H6 | **Sapo = ô `Tóm tắt` sẵn có**, hiện **in đậm ngay dưới tiêu đề** ở trang bài Mini App | Một nguồn: ô ấy vẫn là dòng xem trước ở danh sách; không thêm ô thứ hai nói cùng một điều | — |
| H7 | Trần **20 ảnh thân bài mỗi bài** — giá trị cấu hình nền tảng (`upload_policy`), đổi được sau | Giới hạn theo mục đích do platform sở hữu (ADR 0052 §10); đổi bằng cấu hình, không đổi mã | `service-platform/migrations/0008_upload_policy.sql:66` |
| H8 | **App chung (ViHAT) hiện ảnh thân bài như app riêng** | Khác ảnh bìa — ảnh bìa chỉ ở app riêng (ADR 0047, dòng "Ảnh bìa tin trong app riêng", `kb/10-decisions/0047-hai-luong-dung-citizen-app-theo-ten-mien.md:259`). Ảnh thân bài là một phần nội dung bài; bỏ nó ở app chung là bài đọc thiếu | — |
| H9 | Dòng tác giả/nguồn: **chữ tự do do cán bộ gõ** | **Luật 3:** tên người xuất hiện ở đây là do cán bộ **chủ động công bố**. Hệ thống **không tự điền** tên cán bộ; đường Cổng **vẫn không đọc `TacGia`** — B17 giữ nguyên | `service-comms/internal/app/portal_sync_runner.go:756-763` (B17) |
| H10 | Bản mẫu `vigov-require` đặt ảnh **SAU** phần chữ; H1 **thay** cách ấy cho tin cán bộ soạn | Lý do của bản mẫu là thân bài chữ thuần không còn dấu vết ảnh nằm ở đoạn nào (cách đọc chú thích ấy). Bài cán bộ soạn mang vị trí ảnh trong chính thân bài, nên lý do ấy không còn | `../vigov-require/apps/miniapp/src/pages/NewsDetailPage.tsx:108-123` |

### K. Cách làm — phiên chính chọn, điều đáng giữ là lý do

| # | Điều | Vì sao | Ở mã / dẫn |
|---|---|---|---|
| K1 | **Hai chính sách làm sạch.** Chính sách **cán bộ** (thêm ảnh tham chiếu mã tệp, `figure`/`figcaption` hoặc tương đương, `blockquote`, dòng tác giả) cho POST/PATCH của cán bộ. Chính sách **hiện tại giữ nguyên** cho đường ghi của Cổng; đường đọc theo K7. §1 quyết định 2 vẫn đúng: mọi đường ghi đều qua một bộ làm sạch — nay là bộ của nguồn ghi ấy | Dùng chung một chính sách thì ảnh từ Cổng (mọi host) và thẻ `<img>` thô trong dòng cũ **lọt qua**. Thay B1 "một hàm" bằng "một hàm cho mỗi nguồn ghi". Chú thích gói phải đổi theo khi dựng — nó còn khai "one function" | `service-comms/internal/richtext/richtext.go:13-17` |
| K2 | Thân bài lưu **MÃ TỆP** của ảnh, **không lưu URL**. Máy chủ đổi mã ra URL công khai **lúc đọc**; mã phải là tệp của **đúng xã, đúng bài, đúng mục đích** | Bài nháp chưa có URL công khai; URL công khai **không được do client chọn** — một URL client gửi là một host client chọn cho máy mọi cư dân nối tới | — |
| K3 | Mục đích tải lên mới **`content-body-image`**: cùng loại tệp và kích thước với `content-image`, `max_files_per_subject` = 20 (H7). Ảnh thân bài **đăng/gỡ theo trạng thái bài** như ảnh bìa | Tách khỏi ảnh bìa vì bước công bố ảnh bìa **rút mọi tệp công khai khác** của bài. Đăng/gỡ bản dẫn xuất: ADR 0052 §1 (Bổ sung 30/09) và §11; quét và giới hạn theo mục đích: ADR 0052 §9, §10 — không chép ở đây | `service-comms/internal/app/content_cover.go:921-937` |
| K4 | Loại khối mới trên `body_blocks`: **ảnh**, **trích dẫn**, **dòng tác giả** (tên cuối cùng do builder chốt). Bản Mini App cũ **bỏ qua loại lạ** | Chấp nhận được vì Mini App **chưa lên Zalo** — chưa có bản cũ nào trên máy dân | — |
| K5 | `body` văn bản thuần (`VanBanThuanChoDan`) **giữ chữ** của chú thích, trích dẫn, dòng tác giả; **bỏ ảnh** | Hai dạng thân bài phải nói cùng một điều (§Hệ quả, B3); ảnh không có dạng chữ | `service-comms/internal/domain/van_ban_thuan.go:9-16` |
| K7 | Đường đọc công khai làm sạch **mọi dòng** bằng chính sách cán bộ, rồi chỉ giữ một ảnh khi mã tệp của nó đổi ra tệp `content-body-image` của **đúng xã, đúng bài** (K2); mã không đổi được thì bỏ cả khối ảnh. Đường **ghi** của Cổng vẫn dùng chính sách hẹp | Không cần cột hay migration để phân biệt dòng cũ với bài soạn sau 03/10: dòng Cổng đã làm sạch hẹp lúc ghi nên không mang mã tệp; `<img src>` thô trong dòng cũ không có mã tệp nên bị bỏ; một tệp `content-body-image` của bài chỉ sinh ra từ tuyến tải lên của cán bộ cho chính bài ấy. Thẻ chữ mới (trích dẫn, dòng tác giả) trong dòng cũ chỉ đổi cách hiện, nội dung vẫn là chữ React thoát ký tự | — |
| K8 | Tuyến tải ảnh thân bài và tuyến máy chủ tải link ảnh khai **`content.update`** — cùng khoá với tuyến ảnh bìa (`routes.go:781`), đã có trong bảng `quyen`. Không thêm trần tần suất riêng | Tuyến có xác thực, có vết kiểm toán, và bị chặn bởi trần 20 tệp mỗi bài của `upload_policy` (H7). Luật 13 bất biến 7 chỉ buộc trần tần suất cho tuyến **không** xác thực | `service-identity/migrations/0001_init.sql:292-293` |
| K6 | Máy chủ tải link ảnh là **đường gọi ra ngoài MỚI**: https, cổng 443, kiểm IP lúc quay số, giới hạn chuyển hướng, trần kích thước. Dùng lại rào của gói `portal` ở mức có thể; **không** giới hạn đuôi `.gov.vn` như Cổng | Cán bộ dán ảnh từ báo, từ trang khác — giới hạn `.gov.vn` làm H1 vô dụng. Ranh giới SSRF là phép kiểm lúc quay số (B12) và luật mạng 7c (E4), không phải tên miền | `service-comms/internal/portal/guard.go:6-19`, `:36-38` |

### L. Còn mở — không quyết ở đây

| # | Việc | Của ai |
|---|---|---|
| L1 | Danh sách tên miền của app Zalo cho host kho công khai — ADR 0052 §Còn mở #3 | Chủ dự án |
| L2 | Lời văn câu chính sách quyền riêng tư về ảnh thân bài — ADR 0047 G9, mục C1 ở trên | Chủ dự án |
| L3 | Ảnh trong thân tin đồng bộ từ Cổng (H4) | Chủ dự án, đợt sau |

### Điều kiện dừng — thay điều kiện dừng #1

1. Một đường ghi thân bài bỏ qua bộ làm sạch **của nguồn ghi ấy**
2. Chính sách **ghi của đường Cổng** thêm `img`, `figure`, hay bất kỳ thẻ/thuộc tính nào (H4); hoặc đường đọc giữ một ảnh mà **không** đổi mã tệp theo K7
3. Chính sách cán bộ nhận `img` mang **URL** thay cho mã tệp, `iframe`, `style`, thuộc tính sự kiện, hay scheme khác https
4. Máy chủ trả ra URL ảnh **do client gửi**, hoặc đổi mã tệp không thuộc đúng xã / đúng bài / đúng mục đích
5. Mini App tải ảnh thân bài từ host **không phải** kho công khai của ViGov (H5)
6. Hệ thống tự điền tên cán bộ vào dòng tác giả, hoặc đường Cổng đọc `TacGia` (H9, luật 3)
