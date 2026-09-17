---
id: 0021-khai-quyen-so-huu-thuc-the
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 8d410d4
expires: null
owns_facts:
  - "cơ chế sinh kb/30-indexes/data-ownership.json từ migration bằng dấu khai thực thể"
  - "vì sao quyền sở hữu thực thể không khai tay vào tệp sinh và cũng không khai trong README"
  - "chỗ đặt vùng dữ liệu xuyên xã của service-identity trong cây thư mục"
---

# 0021. Quyền sở hữu thực thể khai tại nơi bảng ra đời, chỉ mục sinh từ đó

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Nối tiếp ADR 0002 · 0019**

## Bối cảnh

Luật 2 bất biến 1 nói: *"Mỗi thực thể có đúng một service sở hữu, khai trong
`kb/30-indexes/data-ownership.json`."* `.claude/hooks/service_boundary_guard.py` nạp tệp đó.
`kb/INDEX.yaml` bảo agent đọc nó **trước khi thêm bất kỳ thực thể, truy vấn hay lời gọi chéo
service nào**.

Tệp đó **rỗng**, và không phải vì chưa ai chịu điền. Đọc `tools/kb/main.go`: bộ sinh có hàm
`scanServices` và `scanSymbols`, nhưng với `data-ownership.json` nó chỉ gọi `ensurePlaceholder`
— ghi một tệp rỗng trung thực rồi thôi. **Không có cơ chế nào để khai.** Một bất biến được ba
nơi viện dẫn mà không có đường nào thi hành là một bất biến sẽ bị bỏ, và bỏ trong im lặng.

ADR 0019 đã chạm đúng chỗ hở này và ghi lại: *"hôm nay không có chỗ nào kiểm được câu trên"*.
Tệp này đóng nó.

## Hai cách sai, cả hai đều trông hợp lý

| Cách | Vì sao sai |
|---|---|
| Gõ thẳng vào `kb/30-indexes/data-ownership.json` | Tầng SINH. Lần `make kb` kế tiếp xoá mất — luật 9 cấm #4. Và nó không còn là câu trả lời kiểm được, nó là một ý kiến |
| Khai ở mục `## Owns` trong README của service | **Đã có, và đã trôi.** `service-platform/README.md` liệt kê `TenantAlias` trong khi `service-platform/migrations/` chưa có bảng nào tên đó; `service-identity/README.md` liệt kê `Citizen · CitizenCommune · Session · OtpCode` trong khi migration chỉ có `phien` của cán bộ. README là văn xuôi cho người đọc, không có gì kiểm nó — đúng dạng lạm phát luật 9 mô tả |

## Quyết định

**Quyền sở hữu khai bằng một dấu trong migration, ngay trên `CREATE TABLE`. `data-ownership.json`
sinh ra từ việc quét các dấu đó.**

```sql
-- @entity: CitizenIdentity
-- @scope:  cross-tenant
CREATE TABLE IF NOT EXISTS cong_dan ( ... );
```

| Thành phần | Nguồn | Vì sao |
|---|---|---|
| Service sở hữu | **Suy ra từ đường dẫn tệp** | Đây là sự thật hay trôi nhất và cũng là sự thật duy nhất tra được không cần hỏi ai: bảng nằm trong `migrations/` của service nào thì service ấy sở hữu. Gõ tay nó là gõ tay thứ máy đọc được |
| `@entity` | **Gõ tay, một dòng** | Tên bảng không phải tên thực thể. `nguoi_dung` là `Staff`; `vai_tro_quyen` không phải thực thể mà là một bảng nối. Không có phép biến đổi nào từ cái này ra cái kia, nên đây là **phần không sinh được** — đúng thứ luật 9 nói là đáng viết |
| `@scope` | **Gõ tay, một dòng** | `tenant` (theo xã) · `platform` (toàn nền tảng, không có `tenant_id`) · `cross-tenant` (có dữ liệu của nhiều xã trong một vùng được phép). Cột `tenant_id` có mặt hay không **không** phân biệt được `platform` với `cross-tenant`, mà hai thứ đó khác nhau về hệ quả an ninh |

### Bốn quy tắc của bộ sinh

| # | Quy tắc | Hỏng gì nếu không có |
|---|---|---|
| 1 | Bảng **không có dấu** thì không vào chỉ mục | Nếu không, chỉ mục đầy `audit_log` tám lần và 256 phân mảnh `_p00`…`_p31`. Một chỉ mục nhiễu là chỉ mục không ai đọc |
| 2 | Hai service khai **cùng một `@entity`** → bộ sinh **đỏ**, không phải cảnh báo | Đây chính là luật 2 bất biến 1. Một bất biến chỉ cảnh báo là một bất biến không có |
| 3 | `@scope: cross-tenant` ngoài `identity` và `platform` → bộ sinh **đỏ** | ADR 0002 chốt hai service đó là ngoại lệ duy nhất. Service thứ ba tự cho mình vùng xuyên xã phải dừng ở cổng kiểm, không ở vòng rà soát |
| 4 | Chỉ mục ghi **những gì đang có**, không ghi dự định | Một chỉ mục liệt kê thực thể chưa có bảng là một lời hứa, và người đọc sau sẽ hành động theo nó |

### Trước khi bảng ra đời thì quyền sở hữu nằm ở đâu

**Ở ADR đã quyết, và chỉ ở đó.** Không có mục nào trong `data-ownership.json` cho một thực thể
chưa có bảng — quy tắc 4 ở trên. Điều này nghe như một lỗ hổng nhưng là chỗ đúng của nó: câu
*"ai sở hữu"* trước khi có schema là một **quyết định** (câu *vì sao*), sau khi có schema là
một **sự kiện tra được** (câu *ở đâu*). Luật 9 xếp hai câu ấy vào hai tầng khác nhau, và việc
một thực thể đi từ tầng này sang tầng kia khi migration được viết là đúng đường đi của nó.

Hệ quả thực tế cho kênh công dân giai đoạn 1 — **nơi sẽ đặt dấu**, không phải nơi quyết định:

| Thực thể | Service | `@scope` | Đã quyết ở |
|---|---|---|---|
| `CitizenIdentity` | `identity` | `cross-tenant` | ADR 0002 |
| `CitizenCommune` | `identity` | `tenant` | ADR 0002 |
| Phiên công dân | `identity` | `tenant` | ADR 0002 · 0020 bất biến 4 |
| Phiên ghép | `identity` | `tenant` | ADR 0019 |
| `TenantAlias` | `platform` | `platform` | ADR 0005 |

Ba dòng giữa chưa có **tên thực thể trong hợp đồng** đã chốt — xem §ĐIỀU KIỆN DỪNG.

## Vùng xuyên xã của `identity` nằm ở đâu trong cây thư mục

ADR 0002 đòi vùng dữ liệu xuyên xã của `danhtinh` *"phải nằm trong thư mục được đánh dấu tường
minh, không rải trong mã theo từng xã"*, và không nói thư mục nào. Chốt ở đây:

```
service-identity/internal/store/crosstenant/
```

**Tầng `store`, không phải tầng `domain`.** Thứ nguy hiểm không phải một kiểu dữ liệu không có
`tenant_id`, mà một **truy vấn** không có `WHERE tenant_id = $1`. `core/store.For(ctx)` là đường
duy nhất tới CSDL và nó `panic` khi không có xã (`core/store/scoped.go:30`), nên một kho xuyên
xã **buộc** phải cầm `*sql.DB` thô. Đó là dấu hiệu nhìn thấy được, và nó phải bị nhốt trong một
thư mục tên đúng như việc nó làm chứ không nằm cạnh các kho khác.

Đã có đúng một tiền lệ và nó xác nhận hình dạng này:
`service-platform/internal/store/directory.go` tự khai mình là *"THE ONE SANCTIONED UNSCOPED
READER"* ngay trong chú thích kiểu. Khác biệt là ở `platform` nó là một tệp đơn độc, còn ở
`identity` sẽ có nhiều hơn một, nên nó cần một thư mục.

**Điều thư mục này KHÔNG cấp phép:** nó không làm cho một truy vấn xuyên xã trở nên hợp lệ. Nó
chỉ làm cho truy vấn ấy **đếm được**. Luật 1 cấm #6 vẫn áp: mỗi truy vấn trong đó vẫn phải mang
`// @cross-tenant: <lý do>` của riêng nó.

## Phải trả

- **Trả ngay:** `tools/kb` phải có bộ quét; mỗi migration phải mang thêm hai dòng chú thích.
  Chi phí một lần, và hai dòng ấy nằm đúng chỗ người viết bảng đang nhìn
- **Trả sau:** một thực thể đổi tên phải sửa dấu — nhưng đổi tên thực thể đằng nào cũng là việc
  phải cân nhắc, và bộ sinh sẽ hiện ra nó thành một dòng diff
- **Không mua được:** chỉ mục vẫn không biết những thực thể chưa có bảng. Đó là chủ ý, quy tắc 4

## ĐIỀU KIỆN DỪNG

1. **Tên thực thể trong hợp đồng cho phiên công dân và phiên ghép chưa chốt.** `phien` hiện là
   phiên **cán bộ** (khoá ngoại tới `nguoi_dung`), nên phiên công dân không dùng lại được tên
   đó. `kb/00-foundation/ubiquitous-language.md` có dòng *Phiên đăng nhập → `phien` →
   `sessions`* và **không có dòng nào** cho hai khái niệm này. ADR 0011 cấm tự dịch: **hỏi**
2. **Tên tài nguyên URL cho `TenantAlias` và cho danh bạ xã chưa có dòng trong bảng ánh xạ.**
   Cùng lý do
3. Một service thứ ba cần `@scope: cross-tenant` — đó là sửa ADR 0002, không phải thêm một dấu
4. Đề xuất cho bộ sinh **đoán** tên thực thể từ tên bảng — một tên đoán sai trong chỉ mục là
   một tên được tin

→ ADR 0002 (định danh công dân, service nào có vùng xuyên xã): `kb/10-decisions/0002-citizen-identity-platform-level.md`
→ ADR 0005 (bảng alias là yêu cầu phát sinh cho `platform`): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0019 (sở hữu phiên ghép, và chỗ hở này được ghi nhận lần đầu): `kb/10-decisions/0019-qr-ghep-phien.md`
→ Luật 2 (mỗi thực thể một service sở hữu): `.claude/rules/critical/2-service-boundary.md`
→ Luật 9 (không gõ tay thứ sinh được; tầng SINH không sửa tay): `.claude/rules/critical/9-knowledge-single-source.md`
→ Ngôn ngữ chung (bảng ánh xạ tên, và cách xử lý khái niệm chưa có dòng): `kb/00-foundation/ubiquitous-language.md`
