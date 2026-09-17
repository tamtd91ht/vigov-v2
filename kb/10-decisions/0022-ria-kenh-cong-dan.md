---
id: 0022-ria-kenh-cong-dan
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 8d410d4
expires: null
owns_facts:
  - "rìa HTTP của kênh công dân: xã lấy từ phiên công dân, không từ Host"
  - "hai lớp endpoint của kênh công dân và cách khai báo bắt buộc trong cùng câu lệnh route"
  - "vì sao phép đối chiếu token-với-Host của luật 1 bất biến 8 không có đối ứng ở rìa công dân"
---

# 0022. Rìa kênh công dân là một rìa riêng, xã đến từ phiên

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Người dùng chốt phương án A** · **Nối tiếp ADR 0005**

## Bối cảnh

Luật 1 bất biến 3: xã suy ra từ `Host` ở **rìa ngoài cùng**, không phân giải được = **404**.
`core/httpx.TenantMiddleware` làm đúng thế và chú thích gói nói thẳng: *"This is where the
commune is resolved, and it is the only place it may be resolved."*

Mini App **không có tên miền**. Nó gọi **một API host duy nhất** và host đó **không ứng với xã
nào** (ADR 0005). Chạy `TenantMiddleware` trên đường ấy thì mọi yêu cầu của mọi công dân đều
404. Nhưng bỏ nó đi mà không thay bằng gì thì `store.For(ctx)` panic, hoặc tệ hơn, có người
"sửa" bằng cách nhận xã từ client — thứ luật 1 cấm #2.

## Quyết định

**Kênh công dân có rìa riêng. Xã đến từ phiên công dân do server phát hành.**

| | Đường cán bộ (giữ nguyên) | Đường công dân (mới) |
|---|---|---|
| Xã đến từ | `Host` → `tenant.Directory.ByHost` | **Phiên công dân** |
| Không xác định được xã | **404** ở rìa | **401** ở rìa, trừ lớp `KhongThuocXa` |
| Đặt vào context bằng | `tenant.IntoFull` — biết cả tên xã | `tenant.Into` — **chỉ có id** |

### Vì sao là một rìa riêng chứ không phải một nhánh trong rìa cũ

Kênh công dân có một lớp endpoint mà đường cán bộ **không có và không nên có**: endpoint hợp lệ
**khi chưa biết xã** — danh mục xã để chọn, tra alias của QR xã đã sáp nhập. Nhét lớp ấy vào
cùng một middleware với đường cán bộ là dựng sẵn một danh sách "không cần xã" ngay cạnh các
tuyến nghiệp vụ, và mời người đến sau thêm nhầm một tuyến nghiệp vụ vào đó. Không test nào đỏ,
không ai nhìn thấy.

Phương án *"đăng ký host công dân như một xã trong `Directory`"* bị loại thẳng: một xã giả mà
mọi yêu cầu chưa xác định rơi vào **chính là tenant mặc định**, luật 1 cấm #1.

### `tenant.Into`, KHÔNG phải `IntoFull` — và đó là điều đúng, không phải thiếu sót

Rìa công dân không phân giải `Host` nên nó **không biết tên xã**, chỉ biết `tenant_id` từ phiên.
`core/tenant/tenant.go` đã dựng sẵn đúng tình huống này bằng hai khoá context: `From`/`MustFrom`
luôn có, còn `Current`/`MustCurrent` chỉ có sau rìa HTTP đã phân giải `Host`. Chú thích ở đó nói
rõ vì sao không ép một khoá duy nhất: hai đường chỉ có `tenant_id` sẽ phải **bịa ra một Tenant
rỗng tên**, và một tên rỗng đi tới màn hình của một cơ quan nhà nước thì tệ hơn hẳn một lỗi.

Hệ quả cho người viết handler công dân: `tenant.MustCurrent` **panic** ở đó. Đó là ý đồ. Tên xã
hiện trên mọi màn hình Mini App (`skills/zalo-miniapp-multi-tenant`, REQUIRED #2) là dữ liệu app
lấy bằng một lời gọi riêng và giữ lại, không phải thứ rìa gắn kèm vào mọi yêu cầu.

## Hai lớp endpoint, khai bắt buộc, mặc định là TỪ CHỐI

Đúng kỷ luật luật 5 bất biến 2: thiếu khai nghĩa là **deny**, không phải allow.

| Lớp | Khai | Xã trong context | Dùng cho |
|---|---|---|---|
| Thuộc một xã | `httpx.XaTuPhien()` | có, từ phiên | **mọi** tuyến nghiệp vụ |
| Không thuộc xã nào | `httpx.KhongThuocXa("<lý do>")` | **không có** | chỉ đường phân giải xã |

**Chỉ có hai. Không có đường thứ ba, và không có mặc định.**

### Khai trong CÙNG câu lệnh route, và danh sách được SINH ra

`tools/apidoc/route.go` đã đọc `authz.*` và `idem.*` ngay trong câu lệnh đăng ký route, và
`kiemTuyen` **từ chối** một route không khai quyền. Lớp xã đi vào đúng cơ chế đó — không dựng cơ
chế thứ hai:

```go
mux.Handle("GET /api/v1/communes",
    authz.CitizenOnly()(
        httpx.KhongThuocXa("màn hình chọn xã: công dân chưa chọn xã nào, đây là lời gọi TẠO RA lựa chọn đó")(
            http.HandlerFunc(h.list))))
```

Ba thứ có được, mà một danh sách giữ tay không có:

| # | Điều | Vì sao quan trọng |
|---|---|---|
| 1 | Route công dân **không khai lớp xã** → `apidoc` **đỏ** | Mặc định là từ chối, và từ chối lúc dựng chứ không lúc chạy |
| 2 | `KhongThuocXa` trên route **không** phải `authz.CitizenOnly()` → `apidoc` **đỏ** | Hai trục vuông góc: `authz.*` trả lời *ai*, lớp xã trả lời *xã nào*. Một tuyến cán bộ không bao giờ được mượn lối này |
| 3 | Danh sách `KhongThuocXa` **sinh vào** `kb/20-contracts/openapi.json` kèm lý do | Luật 9: danh sách sinh được thì không giữ tay. Người rà soát đọc một chỗ thấy hết, và chỗ đó không trôi (ADR 0014) |

Lý do là **bắt buộc và phải cụ thể**, đúng kỷ luật `authz.Public(reason)` — luật 5 cấm #4: một
`Public()` không lý do là thứ sáu tháng sau không ai dám gỡ. *"để hiển thị"* không phải lý do.

### Vì sao `KhongThuocXa` không thể chạm dữ liệu nghiệp vụ — hai bức tường độc lập

Đây là ràng buộc quan trọng nhất của lớp ấy, và nó **không** được giữ bằng một quy ước:

| # | Tường | Cơ chế |
|---|---|---|
| 1 | Không có xã trong context | `core/store.For(ctx)` gọi `tenant.MustFrom`, tức **panic** (`core/store/scoped.go:30`). Không có API nào bỏ qua xã. Một handler `KhongThuocXa` chạm CSDL nghiệp vụ là 500 ngay lần chạy đầu, trong lúc phát triển |
| 2 | Thứ duy nhất với tới được là sổ đăng ký xã | Qua `platformclient` sang service `platform`, và ADR 0003 giữ service ấy ở mức **siêu dữ liệu**. `TenantSummary` trong `proto/vigov/platform/v1/platform.proto` không có trường nào chở nổi nội dung nghiệp vụ |

Hai tường độc lập nhau: hỏng một cái vẫn còn cái kia. Đó là lý do lớp này an toàn được mà không
cần ai nhớ điều gì.

**Nếu một thiết kế cần một endpoint vừa "không cần xã" vừa đọc dữ liệu của xã** thì thiết kế ấy
sai — đó là điều kiện dừng, không phải chỗ để nới một trong hai tường.

## Những thứ giữ nguyên, và một thứ KHÔNG có đối ứng

| Thứ | Ở rìa công dân |
|---|---|
| `httpx.StripTenantHeaders` | **Vẫn chạy.** Lý do y nguyên: client tự khai xã là client tự cấp quyền. Ở đây còn nặng hơn — rìa này không có `Host` để đối chiếu, nên một header lọt qua không có gì mâu thuẫn với nó |
| `tenant.MustFrom` panic | **Giữ nguyên.** Handler nghiệp vụ chạy mà context không có xã thì đổ to tiếng, không âm thầm đọc mọi xã |
| Tham số `t=` trên deep link | **Không bao giờ chạm tới rìa.** Nó dẫn giao diện (ADR 0005); xã vào phiên là một hành động tường minh của công dân, và server phát hành lại phiên |
| Phiên đựng ở đâu | `Authorization`, **không phải cookie**. Một host phục vụ mọi xã nghĩa là một cookie ở đó được gửi kèm lưu lượng của **mọi** xã theo đúng cấu tạo — chính hình dạng luật 1 cấm #3 |

**Luật 1 bất biến 8 không có đối ứng ở rìa này, và phải nói rõ vì sao.** Bất biến ấy là: token
mang `tenant_id`, mọi yêu cầu **đối chiếu nó với xã suy từ `Host`**, lệch thì 401 + báo động. Ở
rìa công dân **không có xã suy từ `Host`** để mà đối chiếu — phiên là nguồn duy nhất, và một
nguồn duy nhất không tự đối chiếu với chính nó được.

Điều này **không** được "sửa" bằng cách đối chiếu phiên với bất cứ thứ gì client gửi lên: làm
thế là dựng lại đúng cái cửa luật 1 cấm #2 đã đóng, dưới dạng một phép kiểm tra trông có vẻ
nghiêm ngặt. Phép đối chiếu tương đương ở kênh công dân nằm **trong luồng**, không ở rìa: ADR
0019 bất biến 8 bắt xã của mã ghép phải khớp xã của phiên công dân, lệch thì 401 + báo động.

## Phải trả

- **Trả ngay:** một middleware mới, hai hàm khai báo, và `apidoc` phải học lớp thứ ba. Mỗi tuyến
  công dân từ nay khai thêm một dòng
- **Trả sau:** tra phiên công dân nằm trên đường nóng của mọi yêu cầu, giống `phien` của cán bộ.
  Nó cần được xử lý như một đường nóng ngay từ đầu, không phải sau khi chậm
- **Không mua được:** rìa công dân không thể fail-closed *sang 404* như rìa cán bộ. Nó trả 401,
  tức nó thừa nhận API host tồn tại. Đó là sự thật hiển nhiên với một Mini App công khai, nên
  không mất gì — nhưng đừng đọc nhầm thành "rìa này lỏng hơn"

## ĐIỀU KIỆN DỪNG

1. Một endpoint công dân cần xã mà **không lấy được từ phiên** — dừng. Đường thứ ba không tồn tại
2. Một endpoint `KhongThuocXa` cần đọc dữ liệu nghiệp vụ — thiết kế sai, xem §hai bức tường
3. Đề xuất đối chiếu phiên công dân với một giá trị do client gửi, nhân danh luật 1 bất biến 8
4. Kênh công dân thứ hai không phải Mini App — quay lại ADR 0020 trước, rồi mới quay lại tệp này

→ ADR 0005 (ba lớp khám phá/phiên/uỷ quyền, một API host duy nhất, khuôn deep link): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0019 (phép đối chiếu xã nằm trong luồng ghép phiên): `kb/10-decisions/0019-qr-ghep-phien.md`
→ ADR 0020 (phiên công dân phát hành sau khi xác thực số điện thoại): `kb/10-decisions/0020-xac-thuc-so-dien-thoai-cong-dan.md`
→ ADR 0003 (service `platform` chỉ trả siêu dữ liệu): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0014 (hợp đồng REST sinh từ khai báo route): `kb/10-decisions/0014-rest-contract-generated-from-code.md`
→ Luật 1 (xã từ `Host`, cấm nhận xã từ client, cấm tenant mặc định): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 4 (cách ly công dân, danh tính yếu): `.claude/rules/critical/4-citizen-isolation.md`
→ Luật 5 (thiếu khai = deny; lý do bắt buộc): `.claude/rules/critical/5-rbac.md`
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md` · `.claude/skills/rest-api-design/SKILL.md`
