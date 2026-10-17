---
id: 0100-webhook-zalo-oa-chuyen-kafka
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 0dc8d1f9
expires: null
owns_facts:
  - "webhook Zalo OA: POST https://<ZALO_OA_CALLBACK_HOST>/api/v1/zalo-oa-events ở service-comms, kiểm chữ ký X-ZEvent-Signature bằng ZALO_OA_WEBHOOK_SECRET (OA Secret Key của Zalo App dùng chung); sai ký → 403 (chủ dự án 17/10/2026)"
  - "xã của sự kiện webhook tìm từ oa_id qua dòng zalo_commune_oa đang sống; OA không thuộc xã nào → 200, không đẩy gì (chủ dự án 17/10/2026)"
  - "sự kiện webhook chuyển NGUYÊN VĂN thân gói Zalo vào Kafka, bọc Envelope core/events tên/topic comms.zalo_oa_event_received.v1, khoá = tenant_id; chủ dự án nhận dữ liệu cá nhân nằm trong Kafka (17/10/2026)"
  - "biến Kafka: KAFKA_EVENT_ADDRESS; KAFKA_EVENT_USERNAME, KAFKA_EVENT_PASSWORD, KAFKA_EVENT_SASL_MECHANISM TUỲ CHỌN — có thì SASL qua TLS, không có thì nối thẳng địa chỉ không xác thực (chủ dự án 17/10/2026)"
---

# 0100. Webhook Zalo OA chuyển nguyên văn vào Kafka

**Trạng thái:** đã chốt · **Ngày:** 2026-10-17 · **Người quyết:** chủ dự án, 17/10/2026, phiên chính, phiếu
hỏi 3 câu · **Nguồn yêu cầu:** *"build thêm 1 endpoint webhook để nhận webhook từ zalo oa về nhé. Nhận xong
bắn vào kafka, hãy cấu hình kafka trên env luôn, thông số tôi sẽ config trên k8s"* · **Dựng trên** ADR 0098
(Zalo App dùng chung, OA của xã), 0099 (nhiều OA mỗi xã), 0010 (Kafka cho sự kiện giữa dịch vụ).

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Tuyến | `POST /api/v1/zalo-oa-events` ở service-comms, cùng host với callback uỷ quyền (`ZALO_OA_CALLBACK_HOST`). Công khai (Zalo gọi), có giới hạn lượt |
| 2 | Xác thực | Kiểm `X-ZEvent-Signature` = sha256(app_id + thân gói + timestamp + OA Secret Key), so thời gian hằng; `app_id` phải là app dùng chung. Sai/thiếu → **403**, không đẩy gì. Khoá: biến mới `ZALO_OA_WEBHOOK_SECRET` (Secret) |
| 3 | Xã | Lấy oa_id từ sự kiện, tìm dòng `zalo_commune_oa` **đang sống** (đọc xuyên xã có đánh dấu). Không thấy → **200** (để Zalo thôi gửi lại) và không đẩy — fail closed, không đoán xã |
| 4 | Nội dung | **Nguyên văn** thân gói Zalo trong `Envelope.Payload`, kèm `tenant_id`. Không ghi thân gói vào log |
| 5 | Kafka | Đẩy đồng bộ; broker chưa nhận → **503** để Zalo gửi lại, không bao giờ trả 200 trước khi broker nhận |
| 6 | Biến | `KAFKA_EVENT_ADDRESS` (danh sách host:port, ConfigMap). Tài khoản **tuỳ chọn**: có `KAFKA_EVENT_USERNAME` + `KAFKA_EVENT_PASSWORD` (Secret) thì SASL (`KAFKA_EVENT_SASL_MECHANISM`: PLAIN · SCRAM-SHA-256 · SCRAM-SHA-512) qua TLS; không có thì nối thẳng, không xác thực |

## Hệ quả

- **Rủi ro nhận:** (a) Kafka chứa dữ liệu cá nhân người dân (tin nhắn, user id Zalo, có thể số điện thoại) —
  trái ADR 0006 hệ quả 5 cho riêng topic này; thời gian giữ topic và ai được đọc là việc vận hành. (b) Khi
  không đặt tài khoản, kênh tới Kafka **không mã hoá** — lệch luật 13 #1, ghi ở `tools/security_debt.json`.
- Bên tiêu thụ topic phải coi mỗi bản tin là dữ liệu cá nhân của đúng một xã (`tenant_id`) và xử lý lặp
  (Zalo gửi lại, Kafka giao ít nhất một lần).
- Phải đặt URL webhook và OA Secret Key ở mục Webhook của Zalo App (`deploy/README.md` §14).

## Còn mở

| # | Việc | Ai |
|---|---|---|
| 1 | Ai tiêu thụ topic và để làm gì (trạng thái giao ZBS, hộp thư OA…) | chủ dự án |
| 2 | Thời gian giữ topic, quyền đọc topic | vận hành |

→ ADR 0098 · 0099 · 0010 · 0006 → Luật 1 · 3 · 8 · 11 · 13
