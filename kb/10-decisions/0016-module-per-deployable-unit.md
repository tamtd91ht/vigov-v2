---
tier: T1
source: quyết định kiến trúc
owner: kiến trúc
derived_from_commit: HEAD
expires: null
owns_facts:
  - "vì sao mỗi đơn vị triển khai là một Go module riêng"
  - "vì sao core được replace bằng đường dẫn thay vì ghim phiên bản"
  - "vì sao thư mục dịch vụ backend mang tiền tố service-"
  - "vì sao go.work tồn tại và vì sao nó KHÔNG đi vào ảnh Docker"
---

# ADR 0016 — Mỗi đơn vị triển khai là một Go module

**Trạng thái:** đã chấp nhận · 2026-09-17
**Thay thế:** phần *"Vẫn CHỈ MỘT `go.mod`"* của [ADR 0015](0015-flat-deployable-units.md)

## Bối cảnh

ADR 0015 đưa mỗi đơn vị triển khai lên cấp một nhưng giữ **một** `go.mod` cho cả kho, với lập
luận: phần đắt không phải tách module mà là biến `core/` thành **ranh giới phiên bản**, vì khi
mỗi dịch vụ ghim phiên bản `core` riêng thì tám dịch vụ có thể chạy **tám bản cài đặt khác
nhau của mô hình cách ly** (`core/authz`, `core/tenant`).

Lập luận đó vẫn đúng. Cái nó bỏ sót: **tách module không bắt buộc kéo theo ghim phiên bản.**

## Quyết định

Mười module: `core/`, `tools/`, và tám `service-<tên>/`. Gốc kho **không có** `go.mod`.

Ba điều đi kèm, và mỗi điều giải một nửa của đánh đổi trên:

**1. `core` được `replace` bằng đường dẫn, không ghim phiên bản.**

```
require  github.com/vihat/vigov/core v0.0.0
replace  github.com/vihat/vigov/core => ../core
```

Chừng nào còn dòng `replace`, cả tám dịch vụ dùng **chung một bản** `core` — rủi ro phân kỳ
mô hình cách ly **không tồn tại**. Ta lấy được ranh giới module mà chưa phải trả giá phiên bản.

Ngày ai đó bỏ `replace` để ghim phiên bản riêng (chẳng hạn khi một dịch vụ tách sang repo
riêng), rủi ro ấy xuất hiện **ngay lúc đó**, và CI phải có **kiểm sàn phiên bản** trước khi
điều đó xảy ra. `replace` trong một module *main* không ảnh hưởng ai khác: dịch vụ không bao
giờ bị import.

**2. Thư mục backend mang tiền tố `service-`.**

Đường module = đường thư mục = tên repo tương lai (`vigov-service-identity`). Không phải đổi
tên lần thứ hai vào ngày tách repo.

Tiền tố là dấu hiệu **hạ tầng**, không phải một phần tên gọi. Tên **nghiệp vụ** vẫn là
`identity`, và đó là tên xuất hiện trong hợp đồng REST (`identity.canBoTomTat`), trong
`kb/30-indexes/services.json`, trong hàng đợi việc web. Để tiền tố lọt vào hợp đồng thì mọi
bên đọc hợp đồng đều phải học cách bỏ nó đi — và bên nào quên sẽ hiện
`service-identity.canBoTomTat` lên một nhãn màn hình.

**3. `go.work` tồn tại cho phát triển, và bị loại trừ khỏi ảnh Docker.**

Không có `go.work`, mỗi lần sửa `core` là một vòng phát hành phiên bản rồi tám lần `go get`.
Chi phí phối hợp đó đủ lớn để người ta bắt đầu **tránh sửa `core`** — và mã dùng chung không
ai dám sửa là mã sẽ bị chép.

Nhưng `go.work` làm mọi module thấy nhau **qua thư mục**, nên nó **che mất một `require` bị
thiếu**. Vì thế nó nằm trong `.dockerignore`, và có hai phép kiểm giữ điều đó không trôi:
`make standalone` build từng module với `GOWORK=off`, và mỗi pipeline dịch vụ chạy
`cd <module> && GOWORK=off go build ./...` trước khi đóng ảnh. Thiếu chúng, một `go.mod` thiếu
require vẫn xanh suốt ở máy trạm rồi đổ ở CI lúc đóng ảnh — xa nhất có thể khỏi chỗ gây ra lỗi.

## Cái được, đo được chứ không phải cảm tính

**Ranh giới dịch vụ nay do TRÌNH BIÊN DỊCH giữ.** Trước đây luật 2 cấm #1 (*"không import
`internal/` của dịch vụ khác"*) chỉ có một hook canh. Nay không module nào `require` một dịch
vụ nào, nên một import chéo là **lỗi biên dịch**:

```
no required module provides package github.com/vihat/vigov/service-comms/internal/app
```

Đã đột biến thử để xác nhận. Một hook chỉ thấy những sửa đổi đi qua agent; trình biên dịch
thấy tất cả.

Kèm theo: mỗi dịch vụ có danh sách phụ thuộc **của riêng nó** (trước đây tám dịch vụ dùng
chung một danh sách, nên không ai biết dịch vụ nào thật sự cần gì), và mỗi dịch vụ build được
độc lập — điều kiện để nó rời sang repo riêng mà không phải sửa gì.

## Cái mất — nói thẳng

- `go build ./...` ở gốc kho **không còn bao được cả kho**. Go từ chối to tiếng, nên chuyện đó
  không âm thầm; cái âm thầm là nếu ai "sửa" bằng cách liệt kê tay vài module, thì module thứ
  mười một sẽ không được kiểm. Nên `Makefile` hỏi `go list -m`, không gõ tay.
- `go.mod` **không còn đánh dấu gốc kho**. Ba công cụ đi ngược lên tìm `go.mod` đã lặng lẽ
  nhận `tools/` làm gốc rồi công bố một hợp đồng rỗng. Dấu hiệu nay là `go.work`.
- Mã sinh từ proto chuyển vào `core/gen`: một module riêng cho nó sẽ cần một `go.mod` mà chính
  dòng `.gitignore` của nó nuốt mất.
