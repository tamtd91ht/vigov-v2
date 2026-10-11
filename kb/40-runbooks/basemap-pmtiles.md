---
id: basemap-pmtiles
tier: T4
source: CURATED
owner: web-admin
derived_from_commit: 31f96fbc
expires: null
owns_facts:
  - "quy trình dựng, tải lên, kiểm chủ quyền có ký và đưa vào dùng tệp nền PMTiles của bản đồ phản ánh và Bản đồ kinh tế số (/ban-do)"
---

# Nền bản đồ tự host (PMTiles) — dựng, làm mới, xử lý sự cố

Dùng chung cho bản đồ phản ánh (`/phan-anh`) và, từ 11/10/2026, Bản đồ kinh tế số (`/ban-do`) — ADR 0072
§Sửa đổi 11/10/2026. Thiếu nền thì cả hai cùng hiện câu "Chưa cấu hình bản đồ nền"; Sổ địa điểm của
`/ban-do` vẫn dùng được.

**Vì sao có nền tự host, phạm vi tệp, ai quyết:** ADR 0072 §Sửa đổi 09/10/2026 và §Trả lời 09/10/2026
(`kb/10-decisions/0072-ban-do-kinh-te-so-nen-tu-host.md`). Tệp này chỉ nói **làm thế nào**. Khung
bbox, mức thu nhỏ và mức phóng to cố định trong `web-admin/scripts/build-basemap.sh` (đầu tệp có lý
do từng con số) — không chép lại ở đây.

Đường đi của một byte: trình duyệt → `https://<xã>/basemap/<tệp>` (cùng gốc, sau cổng phiên của
`web-admin/src/proxy.ts`) → `web-admin/src/lib/may-chu/basemap.ts` → `BASEMAP_URL/<tệp>` trong MinIO.
Danh sách tệp được phát: `web-admin/src/lib/basemap/assets.ts`.

## Triệu chứng → việc làm

| Thấy gì | Nguyên nhân thường gặp | Làm gì |
|---|---|---|
| Bản đồ phản ánh hoặc `/ban-do` hiện **"Chưa cấu hình bản đồ nền."**; `/basemap/...` trả 503 | Key `BASEMAP_URL` chưa có trong `common-config`, hoặc sai dạng (không https, có thông tin đăng nhập, có truy vấn). Log pod web-admin có **một** dòng gọi tên biến | Mục "Đưa vào dùng" |
| `/basemap/vn-mainland.pmtiles` trả **404** "Không tìm thấy tệp bản đồ nền." | Tiền tố trong `BASEMAP_URL` sai ngày, hoặc chưa tải lên | `mc ls <alias>/<bucket>/config/map/` — đối chiếu tên thư mục ngày với giá trị key |
| Trả **502** | MinIO trả 403 (bucket không cho đọc ẩn danh), chứng chỉ MinIO do CA nội bộ ký mà pod không tin, hoặc NetworkPolicy chặn | Lần lượt: `mc anonymous get <alias>/<bucket>` phải in `download`; CA nội bộ → mount CA và đặt `NODE_EXTRA_CA_CERTS` cho web-admin (**không bao giờ** tắt kiểm chứng chỉ); `kubectl get networkpolicy allow-web-admin-egress-storage` phải có, và cổng trong key phải là 9000 |
| Trả **504** | MinIO không phản hồi trong 30 giây | Kiểm MinIO; không phải lỗi web-admin |
| Bản đồ trắng, không câu nào, tab Network không có `/basemap/` | Trang gọi `new Map` trước `registerPmtilesProtocol` (`web-admin/src/lib/basemap/protocol.ts`) | Lỗi mã — báo người dựng màn |
| Bản đồ trắng sau khi bật CSP | CSP thiếu nguồn | Cần `connect-src 'self'`, `img-src 'self' data: blob:`, `worker-src 'self' blob:` — ghi ở nợ `missing-security-headers`, `tools/security_debt.json` |

**Kiểm từ trong cụm** (pod gỡ lỗi cùng namespace, không từ máy cá nhân):

```sh
curl -sI -H 'Range: bytes=0-126' "<giá trị BASEMAP_URL>/vn-mainland.pmtiles"   # phải: 206, Content-Range: bytes 0-126/<cỡ tệp>
```

## Dựng lần đầu hoặc làm mới

Bước 2–3 (dựng + tải lên) chạy theo **một trong hai** cách — **không bao giờ** trong pod:

- **Job Jenkins, việc `nen-ban-do`** (chủ dự án chốt 11/10/2026) — job `vigov-deploy`
  (`HANH_DONG=nen-ban-do`), chạy `deploy/van-hanh/basemap-task.sh`. `vigov-van-hanh` không còn việc này
  từ khi dựng lại chỉ cho đổi hạ tầng (ADR 0089 §Sửa đổi 11/10/2026). Việc chạy
  trên máy build, tự tải `pmtiles` + `mc` đã ghim SHA-256, lấy bucket và địa chỉ từ `common-config` của
  `MT`, dùng cặp khoá MinIO **có sẵn** của `comms-secrets` (thiếu thì `platform-secrets`), in dòng
  SHA-256, `BUILD-INFO.txt` và giá trị `BASEMAP_URL` để ghi vào danh mục ký. Job **không** đổi
  `BASEMAP_URL`. Dựng job lần đầu (mã băm công cụ): `deploy/van-hanh/README.md` §Nền bản đồ.
- **Bằng tay** trên máy người vận hành có Internet, `pmtiles` (go-pmtiles v1.31.2), `curl`, `mc` có alias
  tới MinIO của ViGov — các lệnh ở bước 2–3 dưới.

1. **Chọn bản dựng:** một ngày `YYYYMMDD` có ở `https://build.protomaps.com/<YYYYMMDD>.pmtiles`
   (Protomaps chỉ giữ các ngày gần đây). Script tự từ chối bản dựng có lược đồ tile khác v4.
2. **Dựng:** `web-admin/scripts/build-basemap.sh build --date <YYYYMMDD> --out <thư mục>` — cắt vùng
   (vài GB), tải glyph + sprite ở commit đã ghim, ghi `BUILD-INFO.txt` + `SHA256SUMS`.
3. **Tải lên:** `web-admin/scripts/build-basemap.sh upload --date <YYYYMMDD> --out <thư mục> --alias <alias> --bucket <bucket>`.
   Tiền tố mới `config/map/<YYYYMMDD>/` trong bucket (chủ dự án chốt 11/10/2026; không liên quan tuyến
   `/basemap/*` của web-admin); script **từ chối ghi đè** tiền tố đã có. Tải lên **chưa** phải
   đưa vào dùng — chưa ai đọc tiền tố này.
4. **Kiểm chủ quyền có ký** — mục dưới. Không ký thì dừng ở đây.
5. **Đưa vào dùng** — mục dưới.

**Bucket:** đề xuất của người dựng là bucket `<tiền tố>-public` đã có (đọc ẩn danh, `deploy/cau-hinh/README.md`
mục "Bucket media công khai"): web-admin không giữ bí mật nào, nên chỉ đọc được bucket cho đọc ẩn danh.
Hệ quả: ai biết địa chỉ MinIO công khai cũng tải được tệp nền (dữ liệu OSM công khai, nhưng tốn băng
thông). Muốn bucket riêng có khoá thì web-admin phải giữ khoá MinIO — chủ dự án quyết.

## Kiểm chủ quyền — BẮT BUỘC sau MỖI lần làm mới

ADR 0072 §Trả lời 09/10/2026: kiểm tay + danh mục kiểm **có ký**, mọi lần, không ngoại lệ. Kiểm trên
**staging** đã trỏ `BASEMAP_URL` vào tiền tố mới (mục "Đưa vào dùng", bước 1), bằng màn bản đồ phản ánh
thật, ở các xã có khung chạm biển và biên giới.

| # | Kiểm | Đạt khi |
|---|---|---|
| 1 | `pmtiles show <tệp>` | `min zoom` = 8, `max zoom` = 15, bounds trong khung của script |
| 2 | Kéo, thu nhỏ hết mức ở một xã ven biển miền Trung và một xã ven biển Kiên Giang | Không thấy Hoàng Sa, Trường Sa, không thấy đường nào trên biển |
| 3 | Vịnh Bắc Bộ, vùng biển Tây Nam, biên giới Việt–Trung, Việt–Lào, Việt–Campuchia | Không có đường ranh giới tranh chấp hay ranh giới biển nào |
| 4 | Tên biển, vịnh, đảo gần bờ (Bạch Long Vĩ, Cô Tô, Cồn Cỏ, Lý Sơn, Phú Quý, Côn Đảo, Phú Quốc, Thổ Chu) | Hiện bằng tiếng Việt; không có tên tiếng Anh hay tiếng Trung thay chỗ tên Việt |
| 5 | Đảo gần bờ vẫn có trên nền | Có hình đảo và tên |
| 6 | Góc bản đồ | Có dòng ghi nguồn `Protomaps © OpenStreetMap contributors` |
| 7 | `SHA256SUMS` trên MinIO | Khớp tệp đã dựng (`mc cat …/SHA256SUMS`) |

**Danh mục ký:** ghi ngày bản dựng, giá trị `BASEMAP_URL`, dòng SHA-256 của `vn-mainland.pmtiles`, kết
quả từng dòng 1–7, người kiểm, người duyệt, ngày ký. Bản ký lưu ở **hồ sơ vận hành ngoài kho mã** — không
commit (tên người ký không thuộc kho). **Ai ký** sau lần chạy thật đầu tiên: chủ dự án chưa nêu tên —
trước lần chạy thật đầu tiên là pháp chế ViHAT + đầu mối UBND (ADR).

Một dòng không đạt: **không** đưa vào dùng; giữ `BASEMAP_URL` cũ; báo chủ dự án. Không sửa style hay
khung để "cho qua" — đó là điểm dừng của ADR 0072.

## Đưa vào dùng và quay lui

1. Staging: đặt key `BASEMAP_URL` = `https://<MinIO nội bộ>:<cổng>/<bucket>/config/map/<YYYYMMDD>` trong
   `common-config` của `vigov-staging` — `kubectl -n vigov-staging edit configmap common-config`
   (job `vigov-van-hanh` chỉ đổi hạ tầng, không đặt khoá này) — rồi
   `kubectl -n vigov-staging rollout restart deploy/vigov-web-admin`: key trùng tên biến, tới pod qua
   `envFrom`, mà `envFrom` chỉ đọc lúc pod khởi động (ADR 0093). Kiểm, ký.
2. Prod: cùng giá trị ở `vigov-prod`, rồi `rollout restart` như bước 1.
3. **Quay lui:** đặt lại giá trị cũ, `rollout restart` lại. Vì vậy **không xoá** tiền tố cũ ngay — giữ ít nhất tới
   lần làm mới sau.

Nhịp làm mới: **chưa chốt** (ADR 0072, việc còn mở #3 sau trả lời). Đề xuất của ADR: khi xã báo sai đường
hoặc tên, hoặc hằng năm.

## Ghi nguồn ODbL

Dữ liệu là OpenStreetMap (giấy phép ODbL). Style mang dòng ghi nguồn ở nguồn tile
(`BASEMAP_ATTRIBUTION`, `web-admin/src/lib/basemap/style.ts`); MapLibre hiện nó ở góc bản đồ. **Không
được gỡ**, kể cả khi màn hẹp. Câu chữ pháp lý đầy đủ và xác nhận pháp lý về thể hiện chủ quyền vẫn là
việc còn mở của ADR 0072.
