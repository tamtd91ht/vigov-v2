---
id: 0014-rest-contract-generated-from-code
tier: T1
source: CURATED
owner: architecture
derived_from_commit: e982b51
expires: null
owns_facts:
  - "vì sao hợp đồng REST sinh từ chú thích trong mã Go chứ không từ .proto"
  - "vì sao quyền và chế độ chống lặp không được khai lại trong chú thích route"
  - "hai lần từ chối cứng của bộ sinh apidoc: thiếu thẻ json, và trường gợi bí mật trong reply"
  - "vì sao đầu ra của bộ sinh phải tất định tới byte và không mang dấu thời gian"
  - "vì sao hàng đợi việc cho web dùng thư mục làm trạng thái và os.Rename làm khoá"
  - "vì sao hàng đợi việc nằm ngoài kb/, ở tasks/web/"
---

# 0014. Hợp đồng REST sinh từ mã, và hàng đợi việc cho web

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Nối tiếp ADR 0011, 0012**

## Bối cảnh

Web quản trị (Next.js) dựng **song song** với backend. Nó cần hai thứ kho mã chưa có:

| Thiếu | Hệ quả nếu để nguyên |
|---|---|
| Một **hợp đồng kiểu** cho bề mặt HTTP | Web tự gõ tay kiểu cho mọi response. Đây đúng thất bại đã đo ở v1 và ghi lại ở `.claude/agents/ROUTING.md:229`: nguồn kiểu chuẩn sống ở frontend, trong ba bản chép tay |
| Một cách biết **API nào đã xong** | Câu hỏi đi qua chat, và câu trả lời là thứ ai đó còn nhớ |

ADR 0011 đã chốt **hình dạng** của bề mặt REST — ngôn ngữ của đoạn đường dẫn, cách đánh phiên
bản. ADR 0012 đã chốt bề mặt **giữa các service**. Cả hai đều không nói **ai giữ sự thật** về
bề mặt HTTP: route nào tồn tại, đòi quyền gì, trả ra hình dạng nào.

Khoảng trống đó lấp bây giờ thì giá gần bằng 0: mới có **hai route** nghiệp vụ.

---

## Trước hết — vì sao điều này KHÔNG mâu thuẫn với luật 2 bất biến 7

Luật 2 bất biến 7 ghi: *".proto là nguồn chuẩn cho hợp đồng. Mã sinh ra từ nó, không bao giờ
viết tay."* Tệp này nói hợp đồng REST sinh từ **chú thích trong Go**. Đọc thoáng thì đó là
mâu thuẫn với một luật critical, nên phải giải quyết tường minh, không lờ đi.

**Đây là hai bề mặt khác nhau, không phải hai câu trả lời cho một bề mặt:**

| | Bề mặt **giữa các service** | Bề mặt **service → trình duyệt** |
|---|---|---|
| Ai gọi | Service khác, trong mạng nội bộ | Web quản trị, qua Internet |
| Ai chốt | `proto/` — luật 2, ADR 0012 | Khai báo route trong `*/internal/` |
| Hình dạng lời gọi | Theo lô, mang xã trong metadata (ADR 0012) | Theo màn hình, xã suy từ `Host` ở rìa (luật 1 #3) |
| Mang gì mà bên kia không có | — | Đường dẫn URL, mã trạng thái HTTP, cookie phiên, quyền, chế độ chống lặp |

Toàn bộ ngữ cảnh của luật 2 là **ranh giới giữa hai service**: bất biến 3 nói đọc dữ liệu của
service khác đi bằng gRPC hoặc sự kiện, bất biến 4 nói phiên bản trong tên sự kiện, bất biến 8
nói mọi lời gọi liên service mang xã trong metadata. Bất biến 7 nằm trong cùng danh sách đó và
nói về cùng ranh giới đó. Nó **không** nói gì về bề mặt trình duyệt, vì bề mặt đó không tồn
tại trong bài toán mà luật 2 đang giải (monolith phân tán).

### Nói cho đủ: `.proto` KHÔNG phải là bất khả cho REST

Lập luận "`.proto` không mô tả được đường dẫn URL" nghe gọn nhưng **sai về mặt kỹ thuật**, và
ghi một lập luận sai vào ADR là để lại một chỗ người sau lật lại được. `google.api.http` +
grpc-gateway mô tả được đường dẫn, phương thức và ánh xạ thân message; sinh được cả OpenAPI.
Lựa chọn đó **có tồn tại** và đã bị bác. Ba lý do:

1. **Nó chỉ mang được một phần hợp đồng, nên vẫn tạo ra hai nguồn.** Quyền (`authz.*`) và chế
   độ chống lặp (`idem.*`) sống trong câu lệnh đăng ký route bằng Go và **không có chỗ đứng
   trong `.proto`**. Cookie phiên `httpOnly` không có `Domain` (luật 1 cấm #3) cũng vậy. Chọn
   grpc-gateway là để `.proto` sở hữu đường dẫn còn Go sở hữu quyền — hai tệp cùng mô tả một
   route, đúng dạng trôi dạt luật 9 cấm.
2. **Nó buộc mọi route trình duyệt phải đi qua một method gRPC.** Tức là ép bề mặt cho trình
   duyệt chui qua bề mặt liên service — chính ranh giới ADR 0012 vừa vạch. Hình dạng hai bên
   khác nhau có chủ ý: `BatchGetStaff` tồn tại vì bên gọi là một màn hình danh sách của
   **service khác**; một màn hình của web thì gọi thẳng service sở hữu dữ liệu.
3. **Nó là viết tay một bản khai thứ hai cho thứ đã có.** Hai route HTTP đã tồn tại trong Go,
   cùng middleware rìa đang giữ luật 1. Viết `.proto` cho chúng là hand-writing đúng thứ phép
   thử một dòng của luật 9 cấm: *xoá dòng này đi, công cụ dựng lại được từ mã không?* Được.

**Chốt:** `.proto` giữ nguyên vai trò nguồn chuẩn ở ranh giới liên service. Bề mặt REST cho
web có nguồn chuẩn riêng, là **khai báo route trong Go**, và `kb/20-contracts/openapi.json`
là bản sinh ra từ đó chứ không phải một tệp đặc tả để người sửa.

> **Một việc còn nợ, không làm được từ đây:** câu chữ của luật 2 bất biến 7 nói "hợp đồng"
> không kèm bổ nghĩa, nên người đọc sau vẫn sẽ vấp đúng chỗ tôi vừa vấp. Sửa thành "hợp đồng
> **giữa các service**" là một chữ, nhưng `.claude/` nằm ngoài biên ghi của tệp này.

---

## Quyết định 1 — Nguồn sự thật là mã, không phải một tệp đặc tả riêng

Chú thích `@summary` / `@screen` / `@request` / `@reply` nằm **ngay trên** câu lệnh đăng ký
route, không cách dòng trống. `tools/apidoc` trích bằng `go/ast` và phân giải tên kiểu qua AST
của gói — không so khớp chuỗi, nên đổi tên một kiểu thì bộ sinh **hỏng ồn ào** thay vì lặng lẽ
sinh ra một hình dạng cũ.

**Vì sao dán liền vào câu lệnh, không tách tệp:** đây là cùng kỷ luật `authz` và `idem` đã
dựng bằng cách sống trong cùng câu lệnh. Một bản khai để ở tệp khác **trôi khỏi** route nó mô
tả; một bản khai dán vào route thì không trôi được — sửa route mà không thấy nó là việc phải
cố ý.

Người viết tay **đúng một thứ**: `@screen`, trỏ vào `docs/ui-ux/`. Đó là **ý đồ thiết kế**,
thứ duy nhất ở đây không công cụ nào suy ra được. Mọi thứ còn lại — route nào, quyền gì, hình
dạng nào — là bản sinh.

## Quyết định 2 — Quyền và chế độ chống lặp KHÔNG khai trong chú thích

Đọc thẳng từ `authz.*` và `idem.*` trong chính câu lệnh.

Đây là luật 9 áp vào phạm vi **một dòng mã**. Một thẻ `@permission` viết tay nghe tiện hơn và
sẽ đúng trong đúng một ngày: ngày route đổi từ `AnyAuthenticated` sang `RequirePermission` mà
chú thích không đổi theo. Và cách nó hỏng là **im lặng** — OpenAPI vẫn sinh, vẫn hợp lệ, web
vẫn dựng màn hình, chỉ là dựng theo một quyền không còn đúng.

Giá phải trả: bộ sinh phải hiểu cấu trúc lời gọi Go, không chỉ đọc chuỗi chú thích. Đổi lại
**không có bản chép thứ hai để trôi**, và đó là thứ đắt hơn.

## Quyết định 3 — Hai lần từ chối cứng của bộ sinh

Cả hai đều dừng hẳn, không cảnh báo rồi đi tiếp.

### a. Trường xuất thiếu thẻ `json` → LỖI, không đoán tên

`encoding/json` sẽ phát trường đó dưới tên Go của nó, nên "đoán" có vẻ vô hại. Không phải:
đoán tên là **công bố một tên trường không ai chọn**. Kết quả là một hợp đồng **sai mà trông
như đúng** — và web sẽ dựng màn hình theo nó. Một hợp đồng thiếu thì người ta đi hỏi; một hợp
đồng sai thì người ta tin.

### b. Trường gợi bí mật trong **reply** → DỪNG, nêu đích danh tên trường

`MatKhau`, `password`, `token`, `secret`, `khoa`… nằm trong một kiểu **trả ra** mà không bị
loại bằng `json:"-"` thì bộ sinh dừng. Một tài liệu vận hành công bố hình dạng chứa mật khẩu
là tài liệu **dạy** người đọc chờ mật khẩu ở đó — và người hành động theo nó là người sẽ ghi
nó vào log (luật 3, luật 8).

Chiều **request** thì cho qua, nhưng sinh `writeOnly: true` và in cảnh báo mỗi lần chạy: biểu
mẫu đăng nhập buộc phải gửi mật khẩu, và từ chối mô tả nó là bỏ web tự đoán đúng cái hình dạng
nó không được sai. Kiểu chạm được **cả hai chiều** thì xử theo reply — hỏng thì đóng.

**Giới hạn có chủ ý:** danh sách "gợi bí mật" dừng ở **thông tin xác thực**, không mở sang dữ
liệu cá nhân. Chặn theo tên trường PII sẽ chặn nửa hệ thống, và che dữ liệu cá nhân là việc
của `MaskPhone` / `MaskCccd` ở tầng handler, không phải của một bộ sinh tài liệu. Có test
riêng khẳng định các tên vô tội (`Khoang`, `Monkey`, `Tokyo`, `Hashtag`, `TraceID`) **không**
bị bắt — vì một bộ lọc bắt nhầm là một bộ lọc sẽ bị tắt.

## Quyết định 4 — Đầu ra tất định tới byte

Khoá phát theo thứ tự cố định, danh sách sắp xếp, **không dấu thời gian, không hash commit,
không số dòng**.

Có dấu thời gian thì **mọi lần chạy là một diff**, và một diff nhiễu là một diff không ai đọc
— rồi thay đổi thật duy nhất đi qua không ai thấy. Đây không phải chuyện thẩm mỹ: tệp này là
thứ web đối chiếu để biết máy chủ đã đổi hình dạng gì.

Cưỡng chế bằng test, không bằng dặn dò: thêm một khoá `x-generated-at` làm test đỏ.

## Quyết định 5 — Hàng đợi việc: thư mục LÀ trạng thái, `os.Rename` LÀ khoá

```
tasks/web/open/<id>.json  --rename-->  claimed/<id>.json  --rename-->  done/<id>.json
```

`<id>` là hash tất định của `service|METHOD|path`. Một route giữ nguyên mã việc mãi mãi, và
việc chỉ được sinh khi mã **vắng mặt ở cả ba** thư mục — nên chạy lại bộ sinh không bao giờ
hồi sinh việc đã xong. (Test chứng minh cả hai nửa: chỉ nhìn `open/` thì việc đã `done` bị
sinh lại; bỏ `METHOD` khỏi hash thì `POST` và `DELETE` cùng đường dẫn đụng mã.)

### Vì sao KHÔNG dùng một tệp `pending.json` dùng chung

Vì lần ghi sau **xoá im lặng** lần trước. Hai agent cùng đọc tệp, cùng sửa, cùng ghi — người
ghi sau thắng và không ai biết mình vừa mất gì. Với `os.Rename`, hệ điều hành là khoá: hai
agent cùng nhận một việc thì một thành công, một nhận `ENOENT` — hỏng **ồn ào**, sửa trong một
giây. Mỗi tệp task tự mang câu giải thích này, vì người sắp "dọn cho gọn" thành một tệp chung
là người đang mở đúng tệp đó.

Đừng thêm cờ trạng thái, tệp khoá, hay một dòng CSDL: mỗi thứ đó là **nguồn thứ hai** cho một
fact hệ thống tệp đã giữ, và bản trôi là bản quyết định rằng hai agent đang cùng làm một route.

### Vì sao hàng đợi nằm NGOÀI `kb/`

`kb/INDEX.yaml`, mục `not_here`, đã chốt: trạng thái công việc thuộc công cụ quản lý việc, chứ
không thuộc `kb/`. Một hàng đợi đổi vài lần mỗi ngày mà nằm cạnh tài liệu T0 sống nhiều năm là
đúng cái lỗi xếp theo **chủ đề** thay vì theo **tuổi thọ** mà luật 9 bất biến 6 cấm: một thay
đổi làm cả tệp thành đáng ngờ, và phần lẽ ra đúng nhiều năm mục theo.

Chỉ web quản trị được xếp hàng ở đây. Zalo Mini App phân giải xã theo cách khác và bề mặt của
nó chưa thiết kế; một thư mục `tasks/citizen/` rỗng là một lời hứa về việc chưa ai định phạm vi.

---

## Giới hạn — đọc trước khi coi đây là một đặc tả API

| # | Giới hạn |
|---|---|
| 1 | Hợp đồng hiện mô tả **đúng hai route** (`POST /api/v1/sessions`, `DELETE /api/v1/sessions/{sid}`). Đây **không** phải đặc tả API đầy đủ, và không được đọc như bản kê những gì hệ thống sẽ có |
| 2 | Nó chỉ đúng **chừng nào mọi route mới đều mang khối chú thích**. Thứ canh việc đó — `rest_api_guard` — chạy PostToolUse và **chỉ cảnh báo**, không chặn. Một route quên khối chú thích vẫn vào được `main`, và cách nó hỏng là **vắng mặt lặng lẽ** khỏi hợp đồng |
| 3 | Chỉ quét `*/internal/**`. `/healthz` và mount `"/"` ở `cmd/server` nằm ngoài `/api/v1`, ngoài rìa xã, không mang quyền — cố ý bỏ ra |

**Vì sao (2) chỉ advisory, dù đó là mắt xích yếu nhất:** một route được viết qua nhiều lần
sửa, và chặn giữa chừng dạy agent đi viết route ở chỗ hook không nhìn thấy — bài học luật 5 đã
trả giá với `rbac_guard`. Chấp nhận có ý thức, không phải bỏ sót.

### Món nợ đã biết: task mồ côi

Bộ sinh **không bao giờ xoá một tệp task**. Route bị xoá hoặc đổi đường dẫn thì mã việc đổi
theo, việc mới sinh ra ở `open/`, còn tệp cũ **nằm lại** — trỏ tới một route không còn tồn tại.

Đó là cùng tinh thần luật 7 (một tiến trình không tự quyết xoá thứ người khác có thể đang làm
dở), nhưng nó **là một món nợ, không phải một tính năng**. Ghi ra đây thay vì giấu, vì cách nó
hỏng là một agent nhận một việc đã vô nghĩa và dựng một màn hình cho một đường dẫn đã chết.
Chưa có cơ chế đối soát `open/` với hợp đồng hiện hành; khi có route đầu tiên đổi đường dẫn thì
đó là việc phải làm, và phải là **báo cho người**, không phải tự xoá.

## Hệ quả

- **Dễ hơn:** web có một hình dạng để dựng theo thay vì gõ tay; "API nào xong" đọc được từ hệ
  thống tệp, không phải hỏi trong chat; `make kb` sinh lại tất cả bằng một lệnh
- **Khó hơn:** mọi route mới phải mang khối chú thích dán liền câu lệnh, và mọi trường xuất
  phải có thẻ `json` — bộ sinh không đoán hộ
- **Phải trả ngay:** `.claude/skills/write-knowledge/SKILL.md` đang liệt tầng T2 là "chưa tạo";
  và câu chữ luật 2 bất biến 7 cần bổ nghĩa "giữa các service" (xem khung ở trên)
- **Phải trả sau:** cơ chế đối soát task mồ côi; và nâng `rest_api_guard` từ cảnh báo lên chặn
  chỉ khi nào đo được là chặn không dạy agent né hook

→ ADR 0011 (hình dạng bề mặt REST — đường dẫn, phiên bản): `kb/10-decisions/0011-contract-surface-language.md`
→ ADR 0012 (ranh giới liên service — nơi `.proto` là nguồn chuẩn): `kb/10-decisions/0012-grpc-boundary-contract.md`
→ Bản sinh: `kb/20-contracts/openapi.json` · `kb/30-indexes/api-surface.json` — **không sửa tay**
→ Bộ sinh và toàn văn lập luận trong mã: `tools/apidoc/main.go`
→ Khai báo route mẫu: `identity/internal/http/routes.go`
→ Luật 2 (ranh giới service): `.claude/rules/critical/2-service-boundary.md`
→ Luật 9 (một fact một nguồn): `.claude/rules/critical/9-knowledge-single-source.md`
→ Cưỡng chế: `.claude/hooks/rest_api_guard.py` (advisory) · `.claude/hooks/doc_guard.py` (chặn sửa tay tầng sinh)
