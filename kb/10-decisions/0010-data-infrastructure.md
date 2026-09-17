---
id: 0010-data-infrastructure
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "chọn PostgreSQL làm CSDL nghiệp vụ duy nhất và vì sao không dùng MongoDB"
  - "vai trò của Redis, Kafka, RabbitMQ, Elasticsearch"
  - "số partition và chiến lược phân mảnh vật lý theo tenant"
---

# 0010. Hạ tầng dữ liệu

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16

## Bối cảnh

ADR 0004 chốt `tenant_id` là shard key nhưng không nói dùng CSDL nào, và không nhắc tới hàng
đợi, cache hay công cụ tìm kiếm. Trước khi viết bảng nghiệp vụ đầu tiên phải chốt toàn bộ
hạ tầng dữ liệu, vì đổi về sau là di trú trên hồ sơ lưu trữ.

## Quyết định

| Thành phần | Dùng cho | Không dùng cho |
|---|---|---|
| **PostgreSQL** | Toàn bộ dữ liệu nghiệp vụ, audit, read model của `reporting` | — |
| **Redis** | Cache `Host→tenant`, cache danh mục, rate limit | Lưu trữ bền |
| **Kafka** | Sự kiện giữa service, nguồn dựng read model | Tác vụ có lịch |
| **RabbitMQ** | Tác vụ nền có độ trễ (nhắc hạn, leo thang, 5 job tự động hoá) | Sự kiện giữa service |
| **Elasticsearch** | Tìm kiếm toàn hệ thống | **Số liệu báo cáo** |

**MongoDB: không dùng.**

## Vì sao chỉ PostgreSQL, không thêm MongoDB

Đề xuất ban đầu là dùng cả hai tuỳ nghiệp vụ. Bác, vì bốn lý do cụ thể:

| # | Lý do |
|---|---|
| 1 | **Luật 6 #3 đòi audit ghi cùng giao dịch với nghiệp vụ.** `core/audit.Write` có chữ ký bắt buộc nhận `*store.ScopedTx`. Nghiệp vụ ở Mongo còn audit ở Postgres thì không có giao dịch chung — đúng mô hình ADR 0001 đã bác khi từ chối tách audit thành service riêng |
| 2 | **Dữ liệu ở đây là quan hệ.** Phiếu trỏ tới bộ phận, cán bộ, thôn, lĩnh vực, SLA; nhiệm vụ trỏ tới nguồn giao đa hình; kết luận họp join ngược nhiệm vụ để đếm tiến độ |
| 3 | **Hồ sơ hành chính cần ràng buộc ở tầng CSDL.** Khoá ngoại, `UNIQUE (tenant_id, code)`, `CHECK`. Mất ràng buộc trên tài liệu lưu trữ là thứ không sửa lại được |
| 4 | **Migration đã viết bằng PostgreSQL.** `*/migrations/0001_init.sql` dùng `BIGSERIAL`, `JSONB`, `TIMESTAMPTZ`, `PARTITION BY HASH` — là hiện thân của ADR 0004 |

Chỗ Mongo thường thắng, ở đây đã có lời giải:

| Nhu cầu | Giải bằng Postgres |
|---|---|
| Trường tuỳ biến bản đồ (`gia_tri_tuy_bien`) | `JSONB` + GIN index — prototype đã thiết kế sẵn vậy |
| `dinh_kem`, `thanh_phan`, `log_loi` | `JSONB` |
| Schema linh hoạt | `JSONB` cho phần động, cột thật cho phần cần ràng buộc |

**Nếu sau này vẫn cần Mongo**, ranh giới: chỉ cho dữ liệu **phi nghiệp vụ, không cần giao
dịch, không cần ghi vết** — ví dụ log đồng bộ cổng thông tin. Tuyệt đối không cho hồ sơ hành
chính. Nhưng thêm một CSDL là thêm một thứ phải sao lưu, giám sát, vá; với 200+ xã chi phí đó
là thật và phải có lý do mạnh hơn "linh hoạt hơn".

## Vì sao Elasticsearch KHÔNG dùng cho báo cáo

`docs/ui-ux/13-bao-cao.md §10` yêu cầu `/tong-quan` và `/bao-cao` **không được lệch số**.

Elastic là near-real-time: refresh mặc định 1 giây, và khi reindex thì lệch lâu hơn. Con số
giải ngân và số phiếu quá hạn là **số gửi lên lãnh đạo** — lệch là sự cố nghiệp vụ, không
phải lỗi hiển thị.

`reporting` dựng read model trong **PostgreSQL** từ sự kiện Kafka. Elastic chỉ phục vụ tìm
kiếm toàn hệ thống (`docs/ui-ux/15 §3.1`), nơi kết quả gần đúng là chấp nhận được.

**Cân nhắc hoãn Elastic ở giai đoạn đầu.** Với ~10.000 bản ghi mỗi xã, `pg_trgm` + `unaccent`
của Postgres có thể đủ cho tìm kiếm không dấu — và prototype cũng đề xuất đúng vậy. Thêm
Elastic khi đo được là chậm thật, không phải vì đoán trước.

## Vì sao tách Kafka và RabbitMQ

Không trùng lặp — hai thứ khác nhau:

| | Kafka | RabbitMQ |
|---|---|---|
| Mô hình | Log append-only, nhiều consumer độc lập, **replay được** | Hàng đợi tác vụ |
| Dùng ở đây | Sự kiện giữa service; `reporting` dựng lại read model bằng replay | Nhắc hạn, leo thang, 5 job của `docs/ui-ux/14 §9` |
| Tính năng quyết định | Giữ lịch sử để dựng lại projection | **Delayed message plugin** — Kafka không có |

`core/events.Publisher` là interface thuần, chưa gắn hạ tầng nào, nên cắm Kafka vào là chuyện
triển khai chứ không phải sửa kiến trúc.

## Phân mảnh vật lý theo tenant

ADR 0004 chốt `PARTITION BY HASH (tenant_id)` nhưng chưa chốt **số partition**. Đổi số này
sau khi có dữ liệu là viết lại toàn bộ bảng, nên chốt ngay:

```sql
MODULUS 32
```

| Lý do | Chi tiết |
|---|---|
| Quy mô | 200+ xã ⇒ ~6 xã mỗi partition — đủ nhỏ để pruning có tác dụng |
| Vận hành | Chia hết cho số core thường gặp (8, 16, 32) |
| Dự phòng | Còn chỗ cho tăng trưởng tới ~500 xã mà chưa cần chia lại |

Ba cơ chế Postgres dùng ở đây, **khác nhau, cần cả ba**:

| Cơ chế | Giải quyết | Trạng thái |
|---|---|---|
| `PARTITION BY HASH (tenant_id)` | Truy vấn chỉ chạm 1 partition thay vì toàn bảng (*partition pruning*) | ✅ Bắt buộc từ migration đầu |
| Index bắt đầu bằng `tenant_id` | B-tree đi thẳng tới vùng của xã | ✅ Bắt buộc từ migration đầu |
| `CLUSTER` (sắp xếp vật lý) | Dòng cùng xã nằm cạnh nhau trên đĩa | ⏸ Hoãn — partition đã cho phần lớn lợi ích; `CLUSTER` khoá bảng và không tự duy trì |
| `postgres_fdw` | Tách một xã lớn sang máy riêng (ADR 0004 #6) | ⏸ Hoãn — việc vận hành, không phải thiết kế |

**Một tác dụng phụ có lợi:** HASH partition không pruning được khi truy vấn thiếu `tenant_id`.
Luật 1 đã cấm truy vấn như vậy, nên partition thành lớp cưỡng chế thứ hai — quên `tenant_id`
thì chậm thấy rõ ngay, không âm thầm trả dữ liệu xã khác.

## Hệ quả

- **Dễ hơn:** một CSDL nghiệp vụ nghĩa là một mô hình sao lưu, một mô hình giao dịch, một thứ
  phải vá
- **Khó hơn:** phần schema động phải thiết kế bằng `JSONB` có chủ đích, không thả tự do
- **Phải trả sau:** nếu một ngày cần Elastic cho báo cáo thật (khối lượng vượt Postgres), phải
  giải bài toán lệch số trước — không chỉ là cắm thêm dịch vụ

→ ADR 0004 (shard key): `kb/10-decisions/0004-shard-by-tenant.md`
→ Luật 6 (ghi vết cùng giao dịch): `.claude/rules/critical/6-audit-log.md`
