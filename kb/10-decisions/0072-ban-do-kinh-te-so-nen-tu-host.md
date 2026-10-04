---
id: 0072-ban-do-kinh-te-so-nen-tu-host
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 6170a977
expires: null
owns_facts:
  - "Bản đồ kinh tế số — nền bản đồ = bản công cộng OpenFreeMap (tiles.openfreemap.org; không khoá API, không giới hạn lượt, cho dùng thương mại, không SLA; ghi nguồn bắt buộc 'OpenFreeMap © OpenMapTiles Data from OpenStreetMap'); URL style là biến phía máy chủ toàn nền tảng của web-admin đọc lúc chạy, không NEXT_PUBLIC_*, thiếu thì không có nền và một câu trên màn hình, không bao giờ rơi về host khác (chủ dự án chốt 04/10/2026, thay PMTiles tự phục vụ)"
  - "Bản đồ kinh tế số — luật 3 điểm dừng #2 ĐÃ QUYẾT chỉ cho bản đồ tài nguyên của xã phía cán bộ (viewport ra host nước ngoài được chấp nhận); vẽ toạ độ phản ánh của công dân (bản đồ nhiệt, bản đồ hiện trường) trên nền ngoài VẪN CHƯA QUYẾT (chốt 04/10/2026)"
  - "Bản đồ kinh tế số — khung xã CỨNG: mỗi xã lưu tâm (lat/lng) + bán kính (km) ở service-comms, áp làm maxBounds của MapLibre; người giữ admin.lookup đặt, có vết; xã chưa đặt khung thì trang không vẽ bản đồ; tâm phải nằm trong khung đất liền Việt Nam; bán kính có cận dưới/cận trên đặt tên; chủ quyền chưa có xác nhận pháp lý (chốt 04/10/2026)"
  - "Bản đồ kinh tế số — danh mục loại tài nguyên = đúng 11 nhóm của đặc tả §3, mã tiếng Việt không dấu theo ADR 0011 (doanh-nghiep … cong-trinh-dau-tu-cong), vào từng xã bằng hành động 'nạp 11 nhóm mặc định' có vết, không bằng migration (chốt 04/10/2026)"
  - "Bản đồ kinh tế số — phạm vi MVP và những phần hoãn: lớp nhiệt phản ánh, mật độ theo thôn, nhập Excel, Mini App, PostGIS/vector tile, geocoding ngoài"
  - "Bản đồ kinh tế số — toạ độ numeric(10,6) bắt buộc; 3 trạng thái + cờ xác minh; số điện thoại người đại diện che khi xuất"
  - "Bản đồ kinh tế số — CSP khi trả nợ missing-security-headers: connect-src/img-src gồm tiles.openfreemap.org, worker-src blob:"
  - "Bản đồ kinh tế số — phương án dự phòng nếu bỏ OpenFreeMap: cách dựng, nơi đặt và ghi nguồn tệp PMTiles vùng tự phục vụ"
---

# 0072. Bản đồ kinh tế số — nền tự phục vụ, 11 nhóm tài nguyên, phạm vi MVP

**Trạng thái:** đã chốt · **Ngày:** 2026-10-04 · **Người quyết:** chủ dự án, 04/10/2026 (mục 1–4
của §Quyết định); phiên chính chọn các điểm ở §5, có lý do kèm theo.
**Sửa đổi 04/10/2026:** mục 1 và 2 bị thay bởi §Sửa đổi 04/10/2026 — OpenFreeMap và khung
xã (cuối tệp). Văn bản gốc giữ nguyên bên dưới.

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
| 3 | Geocoding địa chỉ đường phố | Chủ dự án |
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

### H3. Khung xã — khung CỨNG

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
| 4 | Cận bán kính 1–30 km (đề xuất) và giá trị khung đất liền | Chủ dự án duyệt khi dựng |
| 5 | Tile có mang `name:vi` không | Kiểm khi dựng |
