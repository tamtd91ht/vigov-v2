---
tier: T1
source: quyết định kiến trúc
owner: kiến trúc
derived_from_commit: HEAD
expires: null
owns_facts:
  - "vì sao mỗi đơn vị triển khai nằm ở cấp một của kho"
  - "vì sao core/ là một thư mục cấp một chứ không phải pkg/ nằm dưới gốc chung"
---

# ADR 0015 — Mỗi đơn vị triển khai nằm ở cấp một

**Trạng thái:** đã chấp nhận · 2026-09-17

## Bối cảnh

Kho từng nhóm theo loại: `services/<tên>/` cho tám dịch vụ Go, `apps/<tên>/` cho ba ứng dụng
front-end, `pkg/` cho mã dùng chung. Cây thư mục nói *"đây là tám thư mục con của services"*.

Nhưng tám dịch vụ này **triển khai độc lập, chạy độc lập, và có thể có đơn vị bảo trì riêng**.
Thư mục `services/` không phải một ranh giới — nó là một cái ngăn kéo. Nó không có chủ, không
có pipeline, không có ảnh, và không ai chịu trách nhiệm về nó.

## Quyết định

Mỗi **đơn vị triển khai** là một thư mục **cấp một**, ngang cấp với `.claude/`, `kb/`, `docs/`:

- tám dịch vụ Go (`identity/`, `platform/`, `comms/`, …)
- ba ứng dụng front-end (`web-admin/`, `platform-admin/`, `citizen-app/`)
- `core/` — mã dùng chung, trước đây là `pkg/`

Một thư mục cấp một là **dịch vụ Go** khi nó có `<tên>/cmd/server`. Đây là dấu hiệu duy nhất,
và mọi công cụ đọc nó **từ đĩa** thay vì mang một danh sách gõ tay: `tools/kb`, `tools/apidoc`,
`tools/check_build.py`, `.claude/hooks/_common.dich_vu_cua`, và từng `Jenkinsfile`.

**`citizen-app/` giữ nguyên tên, KHÔNG đổi thành `web-client/`.** Kênh công dân là Zalo Mini
App và **không có tên miền** — nó phân giải xã theo cách khác hẳn (luật 1, bất biến 3). Một cái
tên bắt đầu bằng "web" mời người sau giả định nó có `Host`, và giả định đó là chỗ cách ly giữa
các xã bị phá.

## Hệ quả

**Được:**

- Quyền sở hữu nhìn thấy được từ cây thư mục. Mỗi đơn vị mang `Dockerfile` và `Jenkinsfile`
  của chính nó: nó tự quyết build cái gì, khi nào, ra ảnh nào (ADR kèm theo: `Jenkinsfile` ở
  gốc chỉ còn giữ bất biến toàn kho).
- Ngày một dịch vụ tách sang kho riêng, nó đã ở đúng hình dạng để đi.

**Mất — và đây là phần phải nói thẳng:**

- Đoạn `/services/` từng là thứ **sáu hook dùng để nhận ra "đây là mã của một dịch vụ"**. Nó
  biến mất. Hook vẫn chạy, vẫn thoát 0, và **không còn khớp gì nữa** — không có gì đỏ. Một
  quy tắc không bao giờ khớp trông y hệt một quy tắc chưa bị ai vi phạm.
  Nên việc di chuyển **phải là một thay đổi nguyên khối**: `_common.dich_vu_cua()` nhận diện
  theo hình dạng đường dẫn (`<đoạn>/internal/`, `/cmd/`, `/migrations/`) trừ đi danh sách
  `KHONG_PHAI_DICH_VU`, và 90 ca tự kiểm hook chạy trên bố cục mới.
- Cùng lý do, `tools/kb` suýt công bố `kb/`, `docs/`, `tools/` là dịch vụ. Một index sai tệ
  hơn không có index: nó **được tin**.

## Vẫn CHỈ MỘT `go.mod` — ĐÃ ĐƯỢC THAY THẾ bởi [ADR 0016](0016-module-per-deployable-unit.md)

> Phần dưới đây mô tả trạng thái ngày 2026-09-17 **trước** khi tách module, và lập luận của
> nó vẫn đúng: phần đắt là biến `core/` thành ranh giới phiên bản. Cái nó bỏ sót là tách
> module **không bắt buộc** kéo theo ghim phiên bản — xem ADR 0016. Giữ nguyên văn ở đây
> chứ không sửa, vì một ADR ghi lại điều đã quyết tại thời điểm đó.

Cây thư mục phẳng **không** kéo theo tách module. Hôm nay vẫn một `go.mod`, và `core/` vẫn được
import trực tiếp chứ không qua phiên bản.

Lý do: phần đắt và khó rút lại không phải tách tám module dịch vụ — đó là phần rẻ. Phần đắt là
biến `core/` thành một **ranh giới phiên bản**. Khi mỗi dịch vụ ghim phiên bản `core/` riêng,
hệ quả trực tiếp là **tám dịch vụ có thể chạy tám phiên bản khác nhau của `core/authz` và
`core/tenant`** — tức tám bản cài đặt khác nhau của mô hình cách ly, trong một hệ thống mà rò
dữ liệu giữa hai xã là vi phạm giữa hai cơ quan công quyền (luật 1).

Hôm nay điều đó được chặn bằng **đường kích hoạt** trong mỗi `Jenkinsfile`: `core/**`,
`proto/**`, `go.mod`, `go.sum`. Tách module là bỏ cơ chế ấy đi, nên nó phải đi kèm một biện
pháp thay thế — **kiểm sàn phiên bản** trong CI: không dịch vụ nào được phụ thuộc một `core/`
cũ hơn mốc đã định.

Điều **không** được làm khi tách: chép `core/authz`, `core/tenant` thành bản riêng cho từng
dịch vụ. Tám bản cài đặt phân kỳ của mã cách ly là kết cục tệ nhất có thể cho hệ thống này.

## Số liệu tại thời điểm quyết định

`identity` 2487 dòng · `platform` 725 · sáu dịch vụ còn lại 216–221 mỗi cái (khung sinh sẵn).
`core/` 3587 dòng, 16 gói, trong đó `authz`, `config`, `migrate` được **cả 8/8** dịch vụ dùng.

Tức tính độc lập mà việc tách module mang lại là **thật nhưng chưa ai dùng**, còn chi phí phối
hợp thì phát sinh ngay. Đó là lý do bố cục đi trước, module đi sau.
