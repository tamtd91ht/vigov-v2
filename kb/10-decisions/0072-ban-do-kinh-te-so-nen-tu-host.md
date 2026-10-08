---
id: 0072-ban-do-kinh-te-so-nen-tu-host
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 55eef86a
expires: null
owns_facts:
  - "Bản đồ kinh tế số — khung MẶC ĐỊNH theo xã (tâm + bán kính) do quản trị nền tảng đặt ở khu vận hành, lưu ở service-platform, là siêu dữ liệu xã trong ranh giới ADR 0003; giá trị riêng của xã thắng, mặc định chỉ áp khi xã chưa có giá trị riêng; 'Về mặc định' ngừng dùng giá trị riêng, có vết, không xoá cứng (chốt 04/10/2026, lần 2)"
  - "Bản đồ kinh tế số — bán kính khung: > 0 và trần CỨNG 50 km, không cận chính sách nào khác; người đặt chịu trách nhiệm; khu vận hành cảnh báo và đòi xác nhận khi lệch xa mức khuyến nghị (đề xuất 10 km, lệch xa = dưới 3 km hoặc trên 20 km); phép kiểm chủ quyền với trần 50 km (chốt 04/10/2026, lần 2)"
  - "Bản đồ kinh tế số — văn bản lưu ý pháp lý khi xã đổi khung bản đồ, phiên bản 2026-10-04.1: mỗi lần đổi tâm hoặc bán kính ở web-admin phải tích xác nhận, máy chủ từ chối lưu khi thiếu xác nhận, vết ghi xác nhận + phiên bản văn bản (chốt 04/10/2026, lần 2)"
  - "Bản đồ kinh tế số — nền bản đồ = bản công cộng OpenFreeMap (tiles.openfreemap.org; không khoá API, không giới hạn lượt, cho dùng thương mại, không SLA; ghi nguồn bắt buộc 'OpenFreeMap © OpenMapTiles Data from OpenStreetMap'); URL style là biến phía máy chủ toàn nền tảng của web-admin đọc lúc chạy, không NEXT_PUBLIC_*, thiếu thì không có nền và một câu trên màn hình, không bao giờ rơi về host khác (chủ dự án chốt 04/10/2026, thay PMTiles tự phục vụ)"
  - "Bản đồ kinh tế số — luật 3 điểm dừng #2 ĐÃ QUYẾT chỉ cho bản đồ tài nguyên của xã phía cán bộ (viewport ra host nước ngoài được chấp nhận); vẽ toạ độ phản ánh của công dân (bản đồ nhiệt, bản đồ hiện trường) trên nền ngoài VẪN CHƯA QUYẾT (chốt 04/10/2026)"
  - "Bản đồ kinh tế số — khung xã CỨNG: giá trị riêng của xã (tâm lat/lng + bán kính km) lưu ở service-comms, áp làm maxBounds của MapLibre; người giữ admin.lookup đặt, có vết; xã không có giá trị riêng lẫn mặc định thì trang không vẽ bản đồ; tâm phải nằm trong khung đất liền Việt Nam; chủ quyền chưa có xác nhận pháp lý (chốt 04/10/2026; cận bán kính thay ở lần 2)"
  - "Bản đồ kinh tế số — danh mục loại tài nguyên = đúng 11 nhóm của đặc tả §3, mã tiếng Việt không dấu theo ADR 0011 (doanh-nghiep … cong-trinh-dau-tu-cong), vào từng xã bằng hành động 'nạp 11 nhóm mặc định' có vết, không bằng migration (chốt 04/10/2026)"
  - "Bản đồ kinh tế số — phạm vi MVP và những phần hoãn: lớp nhiệt phản ánh, mật độ theo thôn, nhập Excel, Mini App, PostGIS/vector tile, geocoding ngoài"
  - "Bản đồ kinh tế số — toạ độ numeric(10,6) bắt buộc; 3 trạng thái + cờ xác minh; số điện thoại người đại diện che khi xuất"
  - "Bản đồ kinh tế số — CSP khi trả nợ missing-security-headers: connect-src/img-src gồm tiles.openfreemap.org, worker-src blob:"
  - "Bản đồ kinh tế số — phương án dự phòng nếu bỏ OpenFreeMap: cách dựng, nơi đặt và ghi nguồn tệp PMTiles vùng tự phục vụ"
  - "geocoding ngược cho ô 'Nơi xảy ra' của phản ánh: Nominatim TỰ HOST trong cụm, dữ liệu OSM Việt Nam, không host ngoài; địa chỉ điền sẵn và sửa được, toạ độ giữ riêng; đóng việc còn mở #3 của bảng gốc (chủ dự án, 08/10/2026)"
---

# 0072. Bản đồ kinh tế số — nền tự phục vụ, 11 nhóm tài nguyên, phạm vi MVP

**Trạng thái:** đã chốt · **Ngày:** 2026-10-04 · **Người quyết:** chủ dự án, 04/10/2026 (mục 1–4
của §Quyết định); phiên chính chọn các điểm ở §5, có lý do kèm theo.
**Sửa đổi 04/10/2026:** mục 1 và 2 bị thay bởi §Sửa đổi 04/10/2026 — OpenFreeMap và khung
xã (cuối tệp). Văn bản gốc giữ nguyên bên dưới.
**Sửa đổi 04/10/2026 (lần 2):** khung mặc định ở platform-admin, trần bán kính 50 km, lưu ý pháp
lý khi xã đổi khung — thay một phần H3, H4 và việc còn mở #4 của lần 1 (§Sửa đổi 04/10/2026 (lần
2), cuối tệp).
**Sửa đổi 08/10/2026:** geocoding ngược cho "Nơi xảy ra" — Nominatim tự host; đóng việc còn mở #3 của
bảng gốc (§Sửa đổi 08/10/2026, cuối tệp).

## Bối cảnh

Menu `ban-do-kinh-te-so` (đặc tả `docs/ui-ux/10-ban-do-kinh-te-so.md`, route `/ban-do`) chưa có mã.
Bốn thứ buộc phải quyết trước khi dựng:

| Vấn đề | Vì sao không tự chọn được |
|---|---|
| Nền bản đồ lấy từ đâu | Mỗi ô tile trình duyệt tải mang theo toạ độ vùng cán bộ đang xem — tức vị trí cơ sở trên địa bàn, và sau này điểm phản ánh. Gọi máy chủ ngoài là gửi dữ liệu ra ngoài lần đầu (luật 3, điểm dừng #2) |
| Chủ quyền biển đảo | Bản đồ dựa trên OSM từng bị xử phạt ở Việt Nam vì cách thể hiện Hoàng Sa, Trường Sa. Hệ thống của cơ quan nhà nước không được phép mắc lỗi này |
| Số nhóm tài nguyên | Đặc tả tự mâu thuẫn: `10:37` là bảng **11** nhóm, `10:53` nói danh mục chỉ có **8** mục và dùng bộ mã khác. Migration `service-comms/migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql:57-74` vì thế ship bảng rỗng có chủ đích |
| Tài liệu hướng dẫn người dùng đưa 04/10/2026 | Đề xuất MapTiler Free và cấu hình `NEXT_PUBLIC_MAP_*`, cùng một bộ loại tài nguyên riêng. Không phải nguồn sự thật của kho này; ADR này ghi vì sao không theo nó |

Kho yêu cầu đã chọn cùng hướng nền tự phục vụ: `../vigov-require/CLAUDE.md:94` (MapLibre +
PMTiles, không Google Maps), `../vigov-require/docs/runbook/ban-do-nen.md` (tệp tĩnh, không máy
chủ tile, chưa có tệp thì nền xám).

## Các phương án nền bản đồ

| Phương án | Được | Mất |
|---|---|---|
| **Protomaps PMTiles tự phục vụ** | Không yêu cầu nào rời hạ tầng; không máy chủ tile (một tệp tĩnh, HTTP range); không phí theo lượt; cắt được vùng → kiểm soát được nội dung | Phải tự dựng và làm mới tệp; nhãn và kiểu dáng do mình chịu trách nhiệm |
| MapTiler Free | Có sẵn style đẹp, nhanh dựng | Gói miễn phí **chỉ cho mục đích phi thương mại** (https://www.maptiler.com/cloud/terms/) — ViGov là sản phẩm thương mại; khoá API nằm trong trình duyệt; viewport ra máy chủ ngoài |
| OpenFreeMap (bản công cộng) | Miễn phí, không khoá, cho phép thương mại (https://openfreemap.org) | Viewport ra máy chủ nước ngoài; không SLA; vẫn là luật 3 điểm dừng #2 |
| Cấu hình `NEXT_PUBLIC_MAP_*` như tài liệu hướng dẫn | Đơn giản | Bị chặn hai lớp: `web-admin/Dockerfile:56-65` từ chối build khi có `NEXT_PUBLIC_*`; `web-admin/src/ranh-gioi-nguon.test.ts:93-100` đỏ khi mã có `NEXT_PUBLIC_`. Lý do gốc: cấu hình đọc lúc chạy, không nướng vào bundle (CLAUDE.md, luật 8 bất biến 4) |

## Quyết định

### 1. Nền bản đồ — PMTiles tự phục vụ (chủ dự án chốt)

> **BỊ THAY 04/10/2026** — nền là OpenFreeMap, xem §Sửa đổi 04/10/2026 (H1). Giữ lại làm lịch sử
> và làm phương án dự phòng.

- Dữ liệu OpenStreetMap đóng gói bởi Protomaps, **một tệp `.pmtiles` cho mỗi vùng**, đặt ở một
  chỗ tĩnh do ViGov kiểm soát (ngoài `core/storage` — xem §Hệ quả), trình duyệt đọc bằng HTTP
  range request.
- MapLibre GL JS đọc qua giao thức `pmtiles`.
- **Không host bên thứ ba nào** cho tile, style, glyph hay sprite. Glyph và sprite nằm **cùng
  gốc** với tệp nền.
- Ghi nguồn bắt buộc, hiện ở góc bản đồ: `© OpenStreetMap contributors` (giấy phép ODbL).

### 2. Chủ quyền (chủ dự án chốt)

> **BỊ THAY 04/10/2026** — cách giảm rủi ro bằng cắt tệp nền không còn áp dụng (tile OpenFreeMap
> phủ cả thế giới); thay bằng khung xã cứng, xem §Sửa đổi 04/10/2026 (H3).

- Tệp nền được **cắt theo khung đất liền của tỉnh** → Hoàng Sa, Trường Sa **không có trong dữ
  liệu**, nên không thể bị vẽ sai.
- Nhãn tiếng Việt.
- MapLibre `maxBounds` = khung của chính tệp, kèm `minZoom` → không kéo hay thu nhỏ ra ngoài vùng.

**Rủi ro còn lại, nói thẳng:** (a) với tỉnh có huyện đảo thuộc quần đảo (Hoàng Sa thuộc Đà Nẵng,
Trường Sa thuộc Khánh Hoà), một bản đồ tỉnh không có quần đảo có thể bị đọc là thiếu lãnh thổ;
(b) nhãn biển trong khung đất liền lấy từ OSM — tên `vi` phải được kiểm khi dựng tệp; (c) chưa có
ý kiến pháp lý. **Xác nhận pháp lý trước khi chạy thật là việc còn mở** (§Việc còn mở #1).

### 3. Nhóm tài nguyên — 11 nhóm của đặc tả §3 (chủ dự án chốt)

- Danh mục `Loại tài nguyên bản đồ` giữ **đúng 11 nhóm** ở `docs/ui-ux/10-ban-do-kinh-te-so.md:41-51`:
  doanh nghiệp, hộ kinh doanh, hợp tác xã, chợ/TTTM, trường học, cơ sở y tế, di tích, du lịch/làng
  nghề, OCOP, hạ tầng công cộng, công trình đầu tư công. Ghi chú 8 mục ở `10:53` bị thay; bộ loại
  của tài liệu hướng dẫn không dùng.
- Một danh sách, một nguồn: cả 11 nằm trong bảng `loai_tai_nguyen_ban_do`, không nhóm nào viết
  cứng trong mã giao diện (đây đúng là lỗi `10:53` mô tả ở bản thử).
- Các hàng vào **từng xã** bằng hành động tường minh **"nạp 11 nhóm mặc định"**, có vết, idempotent
  — cùng khuôn với `POST /api/v1/roles/defaults` (ADR 0055 §2, `0055:44-51`). **Không** bằng
  migration gieo hàng cho mọi xã: đường migration không có xã trong nó (`0003:40-55`).
- **Chính tả `ma` không phải câu hỏi cho chủ dự án** — nó đã được quyết từ trước: giá trị enum là
  tiếng Việt không dấu, nối gạch (ADR 0011, `0011:47`; ADR 0051 giữ nguyên dòng này, `0051:19`,
  `0051:67`). Tám slug sẵn có ở `10:53` đã theo luật ấy. Chủ dự án chốt **nhóm nào**; mã tiếng Anh ở
  §3 chỉ là cách bản thử gọi tên. Mã đi vào hồ sơ tài nguyên và không đổi được sau khi xã có dữ liệu
  (`0003:67-70`), nên bảng dưới là nguồn duy nhất:

| `ma` | Mã ở đặc tả §3 | Nhãn (đặc tả §3) |
|---|---|---|
| `doanh-nghiep` | `enterprise` | Doanh nghiệp |
| `ho-kinh-doanh` | `household_business` | Hộ kinh doanh |
| `hop-tac-xa` | `coop` | Hợp tác xã |
| `cho` | `market` | Chợ, trung tâm thương mại |
| `truong-hoc` | `school` | Trường học |
| `co-so-y-te` | `health_facility` | Cơ sở y tế |
| `di-tich` | `heritage` | Di tích lịch sử – văn hoá |
| `du-lich-lang-nghe` | `tourism` | Du lịch, làng nghề |
| `ocop` | `ocop` | Sản phẩm OCOP |
| `ha-tang` | `infrastructure` | Hạ tầng công cộng |
| `cong-trinh-dau-tu-cong` | `public_project` | Công trình đầu tư công |

### 4. Phạm vi MVP (chủ dự án chốt)

| Làm | Hoãn — và vì sao |
|---|---|
| Sổ tài nguyên ở `service-comms` — chủ sở hữu theo ADR 0024 (`0024:62`) và `kb/30-indexes/data-ownership.json` (`MapAssetType`) | Lớp nhiệt phản ánh, "Mật độ theo thôn": đọc chéo sang `petitions` — hợp đồng hiện tại không lộ (luật 2 điểm dừng #2), và là toạ độ gắn với phản ánh của công dân (luật 3) |
| `/ban-do` trên web-admin: lớp theo nhóm, gom cụm phía trình duyệt trên nguồn GeoJSON, bộ lọc, tìm → `flyTo`, bảng chi tiết, thêm/sửa với chọn ghim, Sổ địa điểm | Nhập/mẫu Excel (§9 đặc tả) |
| Tìm địa điểm = **chỉ** tài nguyên của xã và tên thôn/tổ dân phố, không gọi ra ngoài | Geocoding ngoài (đặc tả `10:245` gợi Nominatim) |
| | Bề mặt Mini App (SRS M5.3.2 / M6.1.12) |
| | PostGIS, vector tile phía máy chủ |

### 5. Các chọn lựa của phiên chính (có lý do)

> Ba dòng cuối bảng (mở theo phạm vi tài nguyên / không tâm xã · URL tệp nền · CSP host storage)
> bị thay 04/10/2026 — xem §Sửa đổi 04/10/2026, H1, H3, H4. Hai dòng đầu và dòng số điện thoại
> vẫn giữ.

| Chọn | Vì sao |
|---|---|
| Toạ độ `numeric(10,6)`, **bắt buộc** | Đặc tả `10:199` và quy tắc `10:251`. PostGIS không có trong hạ tầng (ADR 0010, bảng `0010:40` chỉ có PostgreSQL); độ chính xác 6 chữ số ≈ 0,1 m là đủ cho một ghim |
| 3 trạng thái `10:202` + cờ xác minh có `xác minh lúc` / `người xác minh` | Đúng §10; tỷ lệ xác minh `10:253` cần cờ, không cần trạng thái thứ tư |
| Số điện thoại người đại diện là **dữ liệu cá nhân**: che khi xuất; số đầy đủ chỉ cho người giữ `asset.update` | Luật 3 bất biến 3. Hộ kinh doanh thường là số di động của một người dân |
| Bản đồ mở theo **phạm vi tài nguyên của xã**; không có cấu hình tâm xã | Toạ độ mặc định `15.730507, 108.378110` ở `10:157` và `10:257` là một xã viết cứng (luật 1 bất biến 10). Xã chưa có tài nguyên nào → mở theo khung của tệp nền |
| URL tệp nền là **biến phía máy chủ** của web-admin, toàn nền tảng, đọc lúc chạy. Thiếu → nền xám trơn kèm một câu giải thích, **không bao giờ** rơi về host bên ngoài | Nền giống nhau ở mọi xã nên là hằng số nền tảng (luật 8 bất biến 5); một fallback ra ngoài chính là rò rỉ mà §1 chặn, chỉ xảy ra đúng ngày cấu hình hỏng. Tên biến đặt khi dựng, theo `skills/infra-config` |
| CSP: khi nợ `missing-security-headers` (`tools/security_debt.json:34-37`) được trả, phải có host storage trong `connect-src` và `img-src`, và `worker-src blob:` | MapLibre đọc range bằng `fetch` và chạy worker từ blob; CSP thiếu ba mục này làm bản đồ trắng mà không báo lỗi nghiệp vụ nào |

## Hệ quả

- **Dễ:** không hoá đơn theo lượt, không khoá API, không dữ liệu nào ra khỏi hạ tầng; một tệp phục
  vụ mọi xã cùng vùng (ghim mới là dữ liệu riêng, đã có rào theo xã).
- **Khó:** phải giữ tệp nền tươi và tự chịu trách nhiệm nội dung nhãn. Thêm vùng là thêm một tệp.
- **Nơi đặt tệp nền — ngoài `core/storage`:** không service nào ghi tệp nền, nên nó **không** đi qua
  `core/storage` và **không** theo lược đồ khoá theo xã `{class}/t_{tenant_id}/…` của ADR 0052
  (`0052:107-117`) — tệp thuộc toàn nền tảng, không thuộc xã nào. Vận hành tải tệp lên một **chỗ
  tĩnh riêng** do ViGov kiểm soát: một bucket hoặc đường dẫn đọc công khai riêng trên object
  store, hay bất kỳ host HTTPS tĩnh nào của ViGov. Chỗ ấy phải trả lời HTTP `Range` và cho CORS từ
  các origin của web-admin. web-admin chỉ **đọc URL** của nó (biến phía máy chủ đọc lúc chạy, §5).

## Dựng và cập nhật tệp nền

> **Từ 04/10/2026: phương án dự phòng nếu bỏ OpenFreeMap** (§Sửa đổi 04/10/2026, H4). Không phải
> việc phải làm khi dựng `/ban-do`.

1. Lấy `pmtiles` (go-pmtiles, https://github.com/protomaps/go-pmtiles/releases).
2. Cắt vùng từ một bản dựng hằng ngày của Protomaps:

   ```sh
   pmtiles extract https://build.protomaps.com/<YYYYMMDD>.pmtiles <vung>.pmtiles \
     --bbox=<minLon,minLat,maxLon,maxLat> --maxzoom=14
   ```

   `bbox` = khung **đất liền** của tỉnh (§2) — không mở rộng ra biển để "đẹp hơn".
3. Kiểm tay trước khi đưa lên: mở tệp, xem không có đảo xa bờ, nhãn hiện tiếng Việt.
4. Đưa tệp, glyph và sprite lên **cùng một tiền tố** ở chỗ tĩnh riêng của ViGov (§Hệ quả). Máy
   chủ phải trả lời `Range` và cho CORS từ các origin của web-admin.
5. Làm mới: chưa có nhịp cố định được chốt; đề xuất khi xã báo sai đường/tên hoặc định kỳ hằng năm.
   Đổi tệp = đổi tên tệp (gắn ngày), rồi đổi biến URL — không ghi đè tệp đang được cache.
6. Ghi nguồn `© OpenStreetMap contributors` nằm trong style; không được gỡ.

## Điểm dừng — hỏi người dùng, không tự quyết

> Điểm 1, 2 và 4 được viết lại ở §Sửa đổi 04/10/2026 (H4); bản dưới là bản gốc.

1. Bất kỳ host bên ngoài nào cho tile, style, glyph, sprite hay geocoding.
2. Mở rộng `maxBounds` hoặc `bbox` ra vùng biển, đảo.
3. Thêm PostGIS.
4. Đặt tệp nền ở một host **không do ViGov kiểm soát** — kể cả CDN hay dịch vụ lưu trữ của bên
   thứ ba. Host giữ tệp phải là của ViGov.

## Việc còn mở

| # | Việc | Ai |
|---|---|---|
| 1 | Xác nhận pháp lý về thể hiện chủ quyền trước khi chạy thật | Chủ dự án |
| 2 | Lớp nhiệt phản ánh và "Mật độ theo thôn" — cần hợp đồng đọc từ `petitions` | Chủ dự án + kiến trúc |
| 3 | Geocoding địa chỉ đường phố — **đã chốt 08/10/2026: Nominatim tự host**, xem §Sửa đổi 08/10/2026 | Chủ dự án |
| 4 | Bề mặt Mini App | Chủ dự án |

## Sửa đổi 04/10/2026 — OpenFreeMap và khung xã

Chủ dự án chốt 04/10/2026, sau khi đọc khuyến nghị PMTiles ở trên: *"dùng OpenFreeMap, giới hạn
khung bản đồ trong xã"*. Mục 1 và cách giảm rủi ro của mục 2 bị thay; mục 3, 4 và phần còn hiệu
lực của §5 giữ nguyên.

### H1. Nền bản đồ — OpenFreeMap bản công cộng

| Điểm | Nội dung |
|---|---|
| Nguồn | Bản công cộng của OpenFreeMap (https://openfreemap.org): miễn phí, không khoá API, không giới hạn lượt, cho dùng thương mại. **Không SLA**, cung cấp nguyên trạng |
| Ghi nguồn | Bắt buộc, hiện ở góc bản đồ: `OpenFreeMap © OpenMapTiles Data from OpenStreetMap`. Đúng chuỗi TileJSON `https://tiles.openfreemap.org/planet` trả về (đọc 04/10/2026) |
| URL style | **Biến phía máy chủ** của web-admin, toàn nền tảng, đọc lúc chạy. **Không** `NEXT_PUBLIC_*`, không nướng vào bundle — cùng lý do bảng phương án ở trên (`web-admin/Dockerfile:56-65`, luật 8 bất biến 4–5). Tên biến đặt khi dựng, theo `skills/infra-config` |
| Thiếu biến | Không có nền, một câu giải thích trên màn hình. **Không bao giờ** rơi về host khác: fallback chỉ xảy ra đúng ngày cấu hình hỏng, và gửi viewport tới một host chưa ai chốt |
| Đường lui | Tự host bộ OpenFreeMap sau này vẫn làm được **mà không đổi mô hình dữ liệu**: cùng lược đồ OpenMapTiles, chỉ đổi URL style. Phương án PMTiles (§Dựng và cập nhật tệp nền) là dự phòng thứ hai |

Host mà style `liberty` tham chiếu (tải `https://tiles.openfreemap.org/styles/liberty` ngày
04/10/2026, kèm TileJSON của nguồn vector):

| Thành phần | URL | Host |
|---|---|---|
| Nguồn vector `openmaptiles` | `https://tiles.openfreemap.org/planet` → tile `…/planet/<phiên-bản>/{z}/{x}/{y}.pbf` | `tiles.openfreemap.org` |
| Nguồn raster `ne2_shaded` | `https://tiles.openfreemap.org/natural_earth/ne2sr/{z}/{x}/{y}.png` | `tiles.openfreemap.org` |
| Glyph | `https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf` | `tiles.openfreemap.org` |
| Sprite | `https://tiles.openfreemap.org/sprites/ofm_f384/ofm` | `tiles.openfreemap.org` |

Chỉ một host. Phiên bản trong URL tile và sprite đổi theo thời gian — CSP phải cho cả host, không
cho từng đường dẫn.

**Nhãn:** style `liberty` gốc hiện `name_en` trước `name` (biểu thức `text-field`, đọc 04/10/2026).
Phải thay biểu thức để ưu tiên tên tiếng Việt khi tile có (`name:vi`, rồi `name`). Tile có mang
`name:vi` cho từng đối tượng hay không **chưa kiểm** — kiểm khi dựng.

### H2. Hệ quả nói thẳng — luật 3 điểm dừng #2 đã quyết, chỉ cho bề mặt này

- Mỗi yêu cầu tile để lộ **viewport** — vùng của xã và những chỗ cán bộ phóng to vào — cho một
  host nước ngoài. Chủ dự án chấp nhận điều này.
- **Phạm vi quyết định:** chỉ bản đồ kinh tế số **phía cán bộ**, vẽ **tài nguyên của xã**.
- **Không thuộc phạm vi:** vẽ toạ độ phản ánh của công dân lên nền này — bản đồ nhiệt và bản đồ
  hiện trường của phiếu phản ánh. Vẫn **chưa quyết**; giao diện đang ghi đúng như vậy ở
  `web-admin/src/features/phan-anh/nhan-phieu.ts:1336-1351` (`sceneMap`, `heatMapTab`). Quyết định
  ở đây **không** được dẫn để mở hai phần ấy.
- **Hộ kinh doanh** (`ho-kinh-doanh`, thường là nhà ở) **thuộc** "tài nguyên của xã" — chủ dự án chốt
  04/10/2026 sau rà bảo mật: địa chỉ và vị trí chính xác hiện cho mọi người có `asset.read` như doanh
  nghiệp. Người đại diện, điện thoại, mã số thuế **vẫn che** (chỉ `asset.update` xem đủ, có vết). Vì
  vậy địa chỉ và toạ độ cố ý **không che** — chú thích dữ liệu cá nhân ở `0015_map_asset.sql` viết
  trước quyết định này; tệp đã áp nên không sửa, quyết định ở đây thay nó. Tên hộ kinh doanh (thường là
  tên người) được che bằng `MaskName` trong phần trước/sau của vết kiểm toán.

### H3. Khung xã — khung CỨNG

> **Thay một phần ở lần 2 (04/10/2026):** dòng "Ai đặt", "Chưa đặt", "Bán kính" và lý do 3 — xem
> §Sửa đổi 04/10/2026 (lần 2), K5. Các dòng còn lại giữ.

| Điểm | Nội dung |
|---|---|
| Lưu gì | Mỗi xã một **tâm** (lat/lng) + **bán kính** (km), ở `service-comms`, cạnh sổ tài nguyên (`service-comms/migrations/0015_map_asset.sql`). Là cấu hình theo xã đọc lúc chạy (luật 1 bất biến 10) |
| Áp thế nào | web-admin đặt `maxBounds` của MapLibre theo khung ấy → không kéo hay thu nhỏ ra tầm quốc gia được |
| Ai đặt | Người giữ `admin.lookup` (quyền "Quản lý danh mục", `service-identity/migrations/0001_init.sql:281`), **có vết** (luật 6) |
| Chưa đặt | Trang **không vẽ bản đồ nào** — chỉ một câu hướng dẫn, và biểu mẫu đặt khung cho người giữ `admin.lookup`. Sổ tài nguyên dạng danh sách vẫn dùng được. Thay dòng "mở theo phạm vi tài nguyên của xã" ở §5 |
| Bán kính | Cận dưới và cận trên là **hằng số có tên**. **Đề xuất 1–30 km** — chưa chốt |
| Tâm | Phải nằm trong **khung đất liền Việt Nam** (hằng số có tên; giá trị chính xác chốt khi dựng) |

**Vì sao cứng, không phải "mở mặc định ở xã":**

1. Tile OpenFreeMap phủ **cả thế giới** (TileJSON `bounds` = `[-180, -85.05, 180, 85.05]`, đọc
   04/10/2026), gồm Hoàng Sa, Trường Sa với tên theo OSM. Cắt dữ liệu như mục 2 gốc không còn làm
   được; chỉ còn cách không cho khung nhìn tới đó.
2. Một bản đồ Việt Nam tầm quốc gia **thiếu** hai quần đảo cũng bị xử phạt: Grab Việt Nam bị phạt
   60 triệu đồng (viettimes.vn/grab-viet-nam-bi-phat-60-trieu-dong-post165893.html — phiên chính
   tìm qua web 04/10/2026, mới đọc tóm tắt kết quả tìm kiếm, chưa đọc nguyên bài). Không có tầm quốc gia thì không có câu hỏi thể hiện hay thiếu.
3. Cận trên của bán kính là thứ giữ khung xa quần đảo: tâm trong khung đất liền + bán kính nhỏ
   → khung không chạm vùng biển xa bờ. Nới cận trên là điểm dừng (H4).

**Hệ quả chưa có lời giải:** đơn vị hành chính cấp xã nằm **trên** Hoàng Sa hay Trường Sa có tâm
ngoài khung đất liền, nên không đặt được khung và không có bản đồ. Không tự mở ngoại lệ — điểm dừng.

### H4. Những gì bị thay

> Dòng "Điểm dừng 2" bên dưới được viết lại ở lần 2 (K5): nay là nâng trần **50 km** hoặc nới khung
> đất liền.

| Gốc | Nay |
|---|---|
| Mục 1 — PMTiles tự phục vụ | OpenFreeMap (H1) |
| Mục 2 — cắt tệp nền theo khung đất liền tỉnh, `maxBounds` = khung tệp | Khung xã cứng (H3). Nhãn tiếng Việt vẫn là yêu cầu |
| §Dựng và cập nhật tệp nền | Giữ làm **phương án dự phòng nếu bỏ OpenFreeMap** |
| §5 — URL tệp nền, thiếu thì nền xám | URL style (H1), thiếu thì không có nền |
| §5 — CSP có host storage | CSP khi nợ `missing-security-headers` (`tools/security_debt.json:32-37`) được trả: `connect-src` và `img-src` gồm `tiles.openfreemap.org` (host duy nhất ở bảng H1), cộng `worker-src blob:` |
| Điểm dừng 1 — bất kỳ host ngoài nào | Bất kỳ host ngoài nào **khác OpenFreeMap** cho tile, style, glyph, sprite. **Geocoding vẫn chỉ nội bộ** — không host ngoài nào |
| Điểm dừng 2 — mở `maxBounds`/`bbox` ra biển, đảo | Nới cận trên bán kính, nới khung đất liền cho tâm, hay bỏ khung cứng |
| Điểm dừng 4 — host tệp nền phải của ViGov | Chỉ áp khi quay về phương án dự phòng PMTiles |

Điểm dừng thêm: vẽ **bất kỳ toạ độ nào của công dân** lên nền OpenFreeMap (H2); xã trên quần đảo
cần bản đồ (H3).

### Việc còn mở sau sửa đổi

| # | Việc | Ai |
|---|---|---|
| 1 | Xác nhận pháp lý về thể hiện chủ quyền — vẫn mở, nay với khung xã cứng trên tile phủ toàn cầu | Chủ dự án |
| 2 | OpenFreeMap không SLA: sập thì bản đồ trắng; sổ tài nguyên dạng danh sách vẫn chạy. Có cần tự host không | Chủ dự án |
| 3 | Nhà cung cấp nền cho lớp nhiệt phản ánh / bản đồ hiện trường (H2) — vẫn mở, cùng #2 bảng gốc | Chủ dự án |
| 4 | Cận bán kính 1–30 km (đề xuất) và giá trị khung đất liền — **phần bán kính đã chốt ở lần 2** (K2: > 0, trần 50 km); khung đất liền giữ nguyên | Chủ dự án duyệt khi dựng |
| 5 | Tile có mang `name:vi` không | Kiểm khi dựng |

## Sửa đổi 04/10/2026 (lần 2) — khung mặc định ở platform-admin, trần 50 km, cảnh báo pháp lý

Chủ dự án chốt 04/10/2026 (K1–K4, K6). K5 ghi những gì bị thay. Phần **luồng và chủ sở hữu** (K3)
là **thiết kế của phiên chính**, ghi rõ như vậy; chủ dự án sửa được.

### K1. Khung mặc định theo xã — quản trị nền tảng đặt

| Điểm | Nội dung |
|---|---|
| Ai đặt | Quản trị nền tảng, ở khu vận hành (`platform-admin`, host `OPERATOR_HOST` — ADR 0048) |
| Đặt gì | **Theo từng xã**: một tâm (lat/lng) + một bán kính (km) làm khung mặc định |
| Vì sao không trái ADR 0003 | Vị trí của xã là **siêu dữ liệu xã**, cùng loại với tên, tỉnh, tên miền — cột "Được" của ADR 0003 (`0003:30-31`). Không đụng dữ liệu nghiệp vụ, không dữ liệu cá nhân (tâm xã là địa điểm công cộng, như `service-comms/migrations/0016_map_frame.sql:68` đã ghi) |
| Ràng buộc | Cùng luật với giá trị của xã: tâm trong khung đất liền (H3, không đổi), bán kính theo K2 |
| Vết | Mỗi lần đặt/đổi có vết vận hành ở platform (luật 6) |

### K2. Bán kính — trần cứng 50 km, người đặt chịu trách nhiệm

| Điểm | Nội dung |
|---|---|
| Giới hạn cứng | **> 0 và ≤ 50 km**. Chủ dự án **không** chọn cận chính sách nào khác; người quản trị đặt giá trị **phải chịu trách nhiệm** |
| Cảnh báo ở khu vận hành | Bán kính lệch xa mức khuyến nghị → hiện cảnh báo, phải **xác nhận** mới lưu |
| Mức khuyến nghị | **Đề xuất của phiên chính**, chủ dự án chỉnh được: khuyến nghị **10 km**; "lệch xa" = **dưới 3 km hoặc trên 20 km** |
| Tâm | Vẫn trong khung đất liền của H3 (`0016_map_frame.sql:125-126`: lat 8,4–23,4, lng 102,1–109,5) — **không đổi** |
| Bước | `radius_km` là `numeric(4,1)` (`0016_map_frame.sql:113`) → "> 0" trên thực tế là ≥ 0,1 km |

**Phép kiểm chủ quyền với trần 50 km.** Tâm xa nhất về phía đông là 109,5°E. Kinh độ lớn nhất khung
chạm tới = 109,5 + 50 / (111,32 · cos φ):

| Vĩ độ φ | maxLng |
|---|---|
| 8,4°N (mép nam khung đất liền) | ≈ 109,95°E |
| 23,4°N (mép bắc) | ≈ 109,99°E |

Hoàng Sa bắt đầu ≈ 111,2°E, Trường Sa ≈ 111,5°E → còn cách **≥ 1,2° (~130 km)**. Trần 50 km vì vậy
vẫn giữ khung xa hai quần đảo — lý do 3 của H3 vẫn đúng, chỉ đổi con số.

### K3. Lưu ở đâu, đọc thế nào (thiết kế của phiên chính)

| Điểm | Nội dung |
|---|---|
| Khung mặc định | Ở **`service-platform`** — chủ sổ xã (`tenant`, `service-platform/migrations/0001_init.sql:69`). Đặt qua một tuyến của khu vận hành |
| Giá trị riêng của xã | Vẫn ở **`service-comms`**, bảng `map_frame` (`kb/30-indexes/data-ownership.json`, `MapFrame`) |
| Thứ tự ưu tiên | **Giá trị riêng của xã thắng.** Mặc định chỉ áp khi xã **chưa có** giá trị riêng (chủ dự án chốt) |
| Đọc | `GET /api/v1/map-frame` (`service-comms/internal/http/map_frame.go:6`): xã không có giá trị riêng → comms đọc mặc định qua **gRPC** từ platform (luật 2 bất biến 3; RPC do contract-designer thêm vào `.proto`). Phản hồi nói **nguồn nào đang áp** (của xã / mặc định) và **mang kèm mặc định**, để web-admin đưa ra "Về mặc định" |
| Hướng gọi | comms → platform, đọc siêu dữ liệu xã. **Không** mở chiều platform → dịch vụ nghiệp vụ mà ADR 0003 cấm (`0003:37-39`) |
| Không có cả hai | Như H3 "Chưa đặt": không vẽ bản đồ, chỉ câu hướng dẫn và biểu mẫu cho người giữ `admin.lookup` |

### K4. web-admin — xã vẫn đổi được, mỗi lần đổi phải nhận lưu ý pháp lý

| Điểm | Nội dung |
|---|---|
| Ai đổi | Người giữ `admin.lookup` (như H3). Đổi được cả tâm lẫn bán kính, cùng trần 50 km (K2) |
| Lưu ý pháp lý | **Mỗi lần đổi** tâm hoặc bán kính: hiện văn bản K6 và ô tích xác nhận |
| Máy chủ | **Từ chối lưu** khi thiếu xác nhận — không chỉ chặn ở giao diện (luật 5 điều cấm #1: giao diện sửa được) |
| Vết | Dòng vết của lần lưu ghi **đã xác nhận** + **phiên bản văn bản** (luật 6), cạnh giá trị trước/sau |
| "Về mặc định" | Ngừng dùng giá trị riêng của xã → quay về khung mặc định của platform. **Có vết.** **Không xoá cứng** (tinh thần luật 7): trigger của 0016 từ chối `DELETE` (`0016_map_frame.sql:85-89`), và chính tệp ấy đã ghi trước rằng trạng thái này "là một cột, không bao giờ là DELETE" (`0016_map_frame.sql:42-46`) |

### K5. Những gì bị thay

| Gốc | Nay |
|---|---|
| H3 "Bán kính" — cận dưới/cận trên đặt tên, **đề xuất 1–30 km** | **> 0 và ≤ 50 km** (K2). Hằng số trong mã: `service-comms/internal/domain/map_frame.go:36-37` (1,0 / 30,0) phải đổi theo |
| CHECK `map_frame_radius_range` (`radius_km BETWEEN 1 AND 30`, `0016_map_frame.sql:128`) | Một **migration mới** của comms thay bằng `radius_km > 0 AND radius_km <= 50`. **Không sửa 0016** — core/migrate so checksum tệp đã áp (`0016_map_frame.sql:13`); chú thích "~109,8°E ở trần 30 km" (`0016:31`) để nguyên làm lịch sử, K2 là con số hiện hành |
| H3 "Ai đặt" — chỉ người giữ `admin.lookup` | Thêm quản trị nền tảng đặt **mặc định** (K1); xã vẫn đặt giá trị riêng (K4) |
| H3 "Chưa đặt" — xã chưa đặt thì không có bản đồ | Chỉ khi **không có cả** giá trị riêng lẫn mặc định (K3) |
| 0016 "không có trạng thái quay về không khung" (`0016:44-46`) | Có trạng thái "Về mặc định" (K4) — bằng cột, không `DELETE` |
| H4 điểm dừng — "nới cận trên bán kính" | Nâng trần **50 km**, hoặc nới khung đất liền cho tâm |

### K6. Văn bản lưu ý pháp lý — phiên bản `2026-10-04.1`

ADR này **sở hữu** văn bản. Mã giữ nó thành **hằng số + phiên bản**, chú thích dẫn về ADR 0072 K6.
Đổi một chữ = **phiên bản mới**, ghi ở đây trước.

> Lưu ý trước khi thay đổi khung bản đồ của xã
>
> 1. Bản đồ nền do nhà cung cấp bên thứ ba (OpenFreeMap, dữ liệu OpenStreetMap) cung cấp miễn phí. Nhà cung cấp không cam kết về độ chính xác, tính đầy đủ hay tính liên tục của dịch vụ; bản đồ có thể tạm thời không hiển thị.
> 2. Ranh giới, địa danh, đường sá và vị trí trên bản đồ nền có thể chưa khớp với thực tế hoặc với bản đồ hành chính, bản đồ địa chính do cơ quan có thẩm quyền ban hành.
> 3. Bản đồ chỉ dùng để tham khảo và hỗ trợ quản lý. Không dùng làm căn cứ pháp lý để xác định ranh giới hành chính, diện tích, quyền sử dụng đất hay giải quyết tranh chấp.
> 4. Khi xem bản đồ, trình duyệt gửi yêu cầu tải bản đồ nền tới máy chủ của nhà cung cấp đặt ở nước ngoài; khu vực đang xem có thể được nhà cung cấp ghi nhận.
> 5. Việc thể hiện chủ quyền quốc gia, gồm hai quần đảo Hoàng Sa và Trường Sa, phải tuân thủ quy định pháp luật. Hệ thống giới hạn khung bản đồ trong địa bàn xã; nếu phát hiện thông tin sai lệch, báo ngay cho quản trị viên hệ thống.
> 6. Thay đổi này được ghi vào nhật ký hệ thống: người thực hiện, thời điểm, giá trị trước và sau.
>
> ☐ Tôi đã đọc, hiểu các lưu ý trên và chịu trách nhiệm về thay đổi khung bản đồ của xã.

### Điểm dừng — thêm ở lần 2

1. Nâng trần bán kính quá **50 km**.
2. Nới khung đất liền cho tâm.
3. Đổi văn bản K6 mà không ra phiên bản mới.

### Việc còn mở sau lần 2

| # | Việc | Ai |
|---|---|---|
| 1 | Pháp chế rà văn bản K6 trước khi chạy thật | Chủ dự án + pháp chế |
| 2 | Mức khuyến nghị 10 km và ngưỡng "lệch xa" 3–20 km (đề xuất của phiên chính) | Chủ dự án chỉnh được |
| 3 | ~~Khoá vận hành nào cho tuyến đặt khung mặc định~~ — **phiên chính chốt 04/10/2026:** dùng khoá sẵn có của quản lý xã (`KeyTenantManage`), không thêm khoá thứ tám. Lý do: tâm và bán kính mặc định là siêu dữ liệu của xã, cùng loại với tên xã mà khoá ấy đã canh (ADR 0073 `0073:35-37` — tập khoá đóng giữ nguyên) | Đã chốt |
| 4 | Xác nhận pháp lý về thể hiện chủ quyền (việc còn mở #1 của lần 1) — vẫn mở | Chủ dự án |

## Sửa đổi 08/10/2026 — geocoding ngược cho "Nơi xảy ra": Nominatim tự host

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác. **Người quyết:** chủ dự án,
08/10/2026, trong phiên chính, qua phiếu hỏi. **Chưa dựng** — sổ `citizen-app/noi-xay-ra-dia-chi-nominatim`.

| Điểm | Nội dung |
|---|---|
| Việc | Đổi **toạ độ** công dân vừa lấy ở màn gửi phản ánh ra **địa chỉ** để điền sẵn ô "Nơi xảy ra" (geocoding ngược) |
| Chốt | **Nominatim tự host trong cụm**, dữ liệu OpenStreetMap Việt Nam. **Không host ngoài nào** |
| Vì sao khớp ADR này | H4, dòng *"Điểm dừng 1"*: *"Geocoding vẫn chỉ nội bộ — không host ngoài nào"*. Quyết định này giữ đúng câu ấy |
| Màn gửi | Địa chỉ **điền sẵn và sửa được**. Toạ độ giữ **riêng**, không bị chữ địa chỉ thay hay suy ngược lại |
| Đóng | Việc còn mở **#3** của bảng gốc (*"Geocoding địa chỉ đường phố"*, Chủ dự án): nhà cung cấp geocoding là Nominatim tự host |

**Phạm vi — không mở rộng ngầm:** chủ dự án chốt cho ô "Nơi xảy ra" của phản ánh. Dùng cùng máy Nominatim
cho ô tìm địa điểm của `/ban-do` (geocoding xuôi) **chưa ai quyết** — §4 *"Tìm địa điểm = chỉ tài nguyên
của xã và tên thôn/tổ dân phố"* vẫn đứng tới khi có lời chủ dự án.

**Vì sao luật 3 điều kiện dừng #2 không bật:** toạ độ nơi xảy ra có thể là toạ độ nhà người dân — dữ liệu
cá nhân (luật 3). Gửi nó tới một máy **trong cụm** không phải gửi ra dịch vụ bên ngoài. Điều này chỉ đúng
chừng nào Nominatim còn ở trong cụm: chuyển nó ra host ngoài, hay "tạm" gọi `nominatim.openstreetmap.org`
khi máy nội bộ chưa có, là điều kiện dừng ấy — và là điểm dừng 1 của H4.

**Còn mở khi dựng — hỏi, không tự quyết:**

| # | Việc | Vì sao không tự chọn |
|---|---|---|
| 1 | Nominatim cần **cơ sở dữ liệu riêng có PostGIS** | Điểm dừng 3 của ADR này là *"Thêm PostGIS"*; hạ tầng dữ liệu do ADR 0010 chốt. Một CSDL riêng của Nominatim (không phải CSDL nghiệp vụ) có thuộc điểm dừng ấy không — chủ dự án nói |
| 2 | Service nào gọi Nominatim và mở tuyến cho Mini App | `citizen-app` không tới được máy trong cụm; cần một tuyến phía máy chủ — chủ sở hữu (luật 2 điều kiện dừng #1), khai báo quyền (luật 5) và trần tần suất (luật 13) |
| 3 | Biến cấu hình địa chỉ Nominatim | Phụ thuộc mới chưa có trong `core/config` — luật 11 điều kiện dừng #1 |
| 4 | Log truy cập của Nominatim chứa toạ độ trong URL | Luật 3 bất biến 1: log ấy phải tắt hoặc coi là dữ liệu cá nhân |
| 5 | Ghi nguồn ODbL `© OpenStreetMap contributors` cho địa chỉ trả về, và câu khai trong chính sách quyền riêng tư | Lời văn pháp lý — chủ dự án duyệt |
| 6 | Làm mới dữ liệu OSM Việt Nam | Chưa có nhịp; như bước 5 của §Dựng và cập nhật tệp nền |
