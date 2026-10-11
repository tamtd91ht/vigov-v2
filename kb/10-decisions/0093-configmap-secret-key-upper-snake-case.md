---
id: 0093-configmap-secret-key-upper-snake-case
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 3382b5de
expires: null
owns_facts:
  - "vì sao key ConfigMap/Secret chỉ có MỘT cách viết UPPER_SNAKE_CASE, trùng tên biến môi trường, nạp cả đối tượng bằng envFrom — bỏ quy ước key gạch ngang và mọi ánh xạ từng key (chủ dự án chốt 11/10/2026)"
  - "key có gạch ngang trong ConfigMap/Secret là LỖI, không phải một cách viết khác: envFrom bỏ qua nó im lặng (11/10/2026)"
---

# 0093. Tên key ConfigMap/Secret: một cách viết UPPER_SNAKE_CASE, trùng tên biến, nạp bằng envFrom

**Trạng thái:** đã chốt · **Ngày:** 2026-10-11 · **Người quyết:** chủ dự án, 11/10/2026, truyền qua phiên
chính · **Đóng** câu mở #3 của ADR 0089 · **Ghi vào luật:** commit `427970b1` (luật 11 bất biến 4 + cấm #3,
`skills/infra-config` §1).

## Bối cảnh

| # | Sự việc |
|---|---|
| 1 | Luật 11 bất biến 4 (bản trước 11/10/2026) nói key ConfigMap/Secret viết **gạch ngang**, env var viết gạch dưới, `-` → `_` khi ánh xạ |
| 2 | Cụm thật không theo luật ấy: mọi Deployment nạp `common-config` và `<dịch vụ>-secrets` bằng `envFrom`, mà `envFrom` lấy **chính key làm tên biến**. Tài liệu vận hành (`deploy/README.md`, `deploy/cau-hinh/README.md`) vì thế đã bắt viết **gạch dưới** |
| 3 | Hai nguồn mâu thuẫn được ADR 0089 ghi thành câu mở #3; job vận hành `deploy/van-hanh` ghi theo dạng Deployment đang đọc và không tự quyết |
| 4 | Kiểm kê 11/10/2026: 22 key, 21 đã gạch dưới. Key duy nhất gạch ngang là `BASEMAP-URL` (web-admin đọc thành env `BASEMAP_URL`) — nó cần riêng một khối `configMapKeyRef` trong `deploy/base/web-admin/deployment.yaml` và một bước `kubectl set env --from … --keys=BASEMAP-URL` trong `web-admin/Jenkinsfile` |
| 5 | Người vận hành gõ `BASEMAP_URL` (gạch dưới) vào `common-config` trên cụm — job tìm `BASEMAP-URL`, bản đồ không có nền. Hai cách viết của một tên đã gây lệch thật |

## Phương án

| Phương án | Được | Mất |
|---|---|---|
| Giữ luật gạch ngang, ánh xạ từng key | Đúng chữ luật cũ | Mỗi key thêm một chỗ nối tay ở Deployment/Jenkinsfile; quên là pod thiếu biến **im lặng**; trái với 21/22 key đang chạy |
| **Một cách viết `UPPER_SNAKE_CASE`, key = tên biến, `envFrom`** (chọn) | Theo đúng hệ đang chạy; thêm biến = thêm key, không sửa Deployment; không gõ nhầm được giữa hai cách viết | Một key phải đổi tên trên cụm (`BASEMAP-URL`) |

## Quyết định

Bảng quy tắc nằm ở **`.claude/skills/infra-config/SKILL.md` §1** (nguồn duy nhất, không chép lại ở đây);
luật máy kiểm được ở `.claude/rules/critical/11-infra-config-contract.md` bất biến 4 + cấm #3. Tóm một câu:
key ConfigMap = key Secret = env var = dòng `.env.example` = tên trong `core/config` = dòng danh mục khoá của
job vận hành; mỗi pod nạp cả đối tượng bằng `envFrom`, không ánh xạ đổi tên từng key.

## Hệ quả

| # | Hệ quả |
|---|---|
| 1 | `BASEMAP-URL` đổi thành `BASEMAP_URL` khắp kho. `deploy/base/web-admin/deployment.yaml` bỏ khối `configMapKeyRef`; `web-admin/Jenkinsfile` bỏ bước `set env --from --keys`, thay bằng: gỡ env tường minh `BASEMAP_URL` lượt trước để lại (env tường minh **đè** `envFrom`, và trỏ vào key sắp xoá → `CreateContainerConfigError`), rồi cảnh báo nếu `common-config` thiếu key |
| 2 | Key có gạch ngang nay là **lỗi**: `envFrom` bỏ nó qua im lặng (chỉ một Event `InvalidVariableNames`). Job vận hành (`deploy/van-hanh/ops.py`) từ chối tên khoá có gạch ngang và chỉ coi một key là "đã đăng ký" khi trùng **đúng** một dòng `.env.example` — không còn đọc `-` thành `_` |
| 3 | `BASEMAP_URL` có hậu tố `_URL` nên nay đi qua phép kiểm `https://` của job vận hành — tên gạch ngang từng lọt khỏi phép kiểm ấy |
| 4 | Ngoài phạm vi: job cũ `deploy/Jenkinsfile` (khoảng dòng 2076–2128) nối biến cho `vihat-miniapp` bằng `kubectl set env --from … --keys=…`. Các key ấy **đã gạch dưới**, nên không lệch tên; cách nối từng key là của kho `vihat-miniapp`, không sửa ở đây |
| 5 | Cụm: người vận hành xoá key cũ `BASEMAP-URL` khỏi `common-config` của từng namespace **sau** khi job `web-admin` đã chạy bản mới (bản mới gỡ env tường minh trỏ vào key cũ) |
