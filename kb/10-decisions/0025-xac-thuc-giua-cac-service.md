---
id: 0025-xac-thuc-giua-cac-service
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 16294e5
expires: null
owns_facts:
  - "xác thực giữa các service bằng ĐÚNG MỘT cặp khoá dùng chung, tên header là hằng số toàn hệ thống, giá trị đọc từ biến môi trường nguồn k8s secret"
  - "hai lớp phòng thủ của ranh giới gRPC: khoá dùng chung và mạng nội bộ của cụm k8s — lớp thứ hai là nửa quyết định, không phải chú thích"
  - "vì sao chọn một cặp thay vì khoá riêng mỗi service hoặc khoá kèm key-id để xoay: chi phí bảo trì"
  - "THỨ CƠ CHẾ NÀY KHÔNG MUA ĐƯỢC: không quy trách nhiệm được bên gọi, bán kính nổ bằng cả cụm, xoay khoá là sự kiện hình dạng downtime"
  - "hệ quả đã mở khoá nhưng CHƯA quyết: ListTenants và ResolveTenantSuccession vào danh sách miễn xã"
---

# 0025. Xác thực giữa các service: một cặp khoá dùng chung, cộng cách ly mạng

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Người dùng chốt** ·
**THAY THẾ quyết định 3 của ADR 0012**

## Bối cảnh

ADR 0012 quyết định 3 ghi nguyên văn: *"Hiện tại không có mTLS, không có token giữa service
với service. Một tiến trình nào đó gọi được tới cổng gRPC nội bộ thì gọi được RPC."* Nó ghi
đó là **rủi ro đã chấp nhận có chủ ý**, kèm ba lý do **không** dựng cơ chế bí mật chia sẻ tạm,
và kèm điều kiện gỡ: *"Danh tính theo service (**mTLS** hoặc mesh) trước khi xã đầu tiên chạy
thật."* Mã nói cùng một câu: `service-platform/cmd/server/main.go:163` mở đầu bằng
*"TODO(security): CALLER AUTHENTICATION ON THE gRPC PORT IS NOT IMPLEMENTED"*, và liệt kê ba
thứ bắt buộc trước khi chạy thật, thứ nhất là *"mTLS between services, or a signed service
token verified by a server interceptor"*.

Căn cứ để chấp nhận rủi ro ấy có **ngày hết hạn đã biết trước**: nó đứng được **chỉ vì** ranh
giới gRPC hiện chưa chở dữ liệu nghiệp vụ nào (ADR 0003). Bốn service khung đang xếp hàng chờ
đúng chỗ này — `kb/90-ephemeral/tien-do/*.json`, mục `xac-thuc-can-bo`, cả bốn ghi *"Đọc ADR
0012 quyết định 3 trước"*. Ngày đầu tiên một service hỏi service khác một câu về nghiệp vụ là
ngày căn cứ ấy tan.

Người dùng chốt hôm nay: **dựng xác thực bên gọi ngay, ở hình dạng đơn giản nhất chạy được.**

## Các phương án, được gì và mất gì

| # | Phương án | Được | Mất |
|---|---|---|---|
| A | Giữ nguyên: không có gì, chỉ mạng nội bộ | Không tốn gì | Căn cứ hết hiệu lực đúng lúc bốn service khung cần gọi nhau — tức là bây giờ |
| B | **Một cặp header/giá trị dùng chung cho cả hệ thống** | Một hằng số, một biến môi trường, một secret. Không có phần nào để quên cập nhật | Không biết **ai** gọi; xoay khoá phải đổi đồng thời mọi nơi |
| C | Mỗi service một khoá riêng, bên nhận giữ danh sách | Quy trách nhiệm được bên gọi; thu hồi được từng cái | N service × M bên nhận cấu hình phải khớp nhau; thêm một service là sửa cấu hình mọi service còn lại |
| D | Khoá kèm **key-id**, hai khoá sống song song khi xoay | Xoay khoá không cần đồng thời — đúng hình dạng `core/config.khoaKy` đang dùng cho khoá ký phiên (`core/config/config.go:264`: khoá đầu ký, các khoá sau vẫn kiểm được) | Thêm một trục trạng thái phải có người trông: khoá nào đang ký, khoá nào còn kiểm, khi nào gỡ khoá cũ |
| E | mTLS hoặc service mesh | Danh tính theo service ở tầng dưới, không phải mã nghiệp vụ | Chứng chỉ, CA, vòng đời chứng chỉ, và một tầng hạ tầng mà **đội devops** phụ trách chứ không phải kho mã này (`kb/90-ephemeral/ban-giao-phien.md:53`) |

## Quyết định

**Đúng MỘT cặp khoá/giá trị dùng chung cho toàn hệ thống. Phòng thủ hai lớp: khoá này, và
việc lời gọi chỉ tồn tại trong mạng nội bộ của cụm k8s.**

| | |
|---|---|
| Tên header | Một **hằng số**, giống hệt nhau ở mọi service, khai ở **đúng một chỗ** |
| Giá trị | Đọc từ **biến môi trường**, nguồn là **k8s secret** — không bao giờ nằm trong mã, trong tệp mẫu, trong tài liệu (luật 8 bất biến 1) |
| Ai đặt vào lời gọi | **Interceptor phía client**, lấy từ cấu hình — y hệt kỷ luật của `x-tenant-id` (ADR 0012 quyết định 1): mã nghiệp vụ không truyền được nó như một tham số |
| Ai kiểm | **Interceptor phía server**, trước mọi handler. Sai hoặc thiếu = từ chối |
| Lớp thứ hai | Cổng gRPC **chỉ lắng nghe trên mạng nội bộ cụm**, có network policy, không bao giờ `0.0.0.0` trên host công khai |

### Vì sao một cặp chứ không phải C, D hay E — lý do người dùng nêu là BẢO TRÌ

Nguyên văn lý do: **một cơ chế có nhiều mảnh chuyển động hơn là một cơ chế không ai giữ cho
chạy.** Đây không phải lý do kỹ thuật, và ghi nó như một lý do kỹ thuật là ghi sai.

Nó đáng tin vì nó khớp với chỗ khác trong dự án: phạm vi hạ tầng của kho mã này **dừng ở
Dockerfile + Jenkinsfile**, cụm k8s do đội devops phụ trách. Một cơ chế đòi CA, chứng chỉ hay
danh sách khoá theo service là một cơ chế nằm **vắt qua ranh giới hai đội** — và một cơ chế
vắt qua hai đội là một cơ chế mà ngày nó hỏng, không đội nào nhận. Phương án B hỏng thì hỏng
trong đúng một tệp cấu hình.

Điều này **không** có nghĩa C, D, E sai. Nó có nghĩa: ở quy mô hiện tại, một cơ chế đơn giản
**đang chạy** đáng giá hơn một cơ chế đúng hơn **đang hỏng lặng lẽ**.

### Lý do thứ hai và thứ ba của ADR 0012 quyết định 3 — thứ nào còn đúng

ADR 0012 nêu ba lý do không dựng bí mật chia sẻ. Phải đối chiếu từng cái, không được lờ đi:

| Lý do cũ | Còn đúng không |
|---|---|
| 1. *"Gần như chắc chắn bị thay"* bởi mTLS/mesh khi lên hạ tầng thật | **Vẫn có thể đúng.** Nhưng "sẽ bị thay" không phải lý do để hôm nay **không có gì cả** — và cái giá bị thay là xoá một interceptor, không phải viết lại một tầng |
| 2. *"Nó bảo vệ ít hơn vẻ ngoài"* — không phân biệt được ai gọi, không thu hồi riêng được | **VẪN ĐÚNG NGUYÊN VẸN.** Người dùng chọn trả cái giá đó, không phải bác bỏ nó. Toàn bộ mục dưới đây tồn tại để câu này không bị quên |
| 3. *"Nó tạo cảm giác an toàn giả"* — người review thấy "đã có xác thực" rồi không hỏi nữa | **Đây là rủi ro thật của quyết định hôm nay.** Chống lại nó là việc của ADR này, và chỉ bằng một cách: viết ra rành mạch thứ nó **không** mua được |

---

## THỨ QUYẾT ĐỊNH NÀY KHÔNG MUA ĐƯỢC

**Mục quan trọng nhất của tệp.** Một ADR chỉ ghi quyết định sẽ được đọc như một lời bảo đảm
an ninh mà nó không hề đưa ra.

### 1. Khoá dùng chung KHÔNG nói service nào đang gọi

Mọi service cầm **cùng một giá trị**. Bên nhận chứng minh được *"bên gọi có khoá"*, và **không
gì hơn**. Hai hệ quả phải nói thẳng:

| Hệ quả | Vì sao nặng |
|---|---|
| Một lời gọi liên service **không quy trách nhiệm được về bên gọi** trong bản ghi kiểm toán | Luật 6 bất biến 2 đòi bản ghi mang **ai · làm gì · trên bản ghi nào · lúc nào · từ IP nào · ở xã nào**. Ô **"ai"** ở đây chỉ điền được *"một tiến trình trong cụm"* |
| `x-tenant-id` trong metadata **vẫn chỉ là lời khai của bên gọi**, không phải bằng chứng | Cơ chế này không nâng nó lên thành bằng chứng, và đừng ai đọc nhầm như thế. Thứ giữ cho lời khai ấy vô hại vẫn là ADR 0003 (ranh giới chưa chở nội dung nghiệp vụ) chứ không phải khoá |

Nói gọn: khoá trả lời *"lời gọi này đến từ trong nhà"*, không trả lời *"lời gọi này đến từ
ai"*. Hai câu đó khác nhau, và bản ghi kiểm toán hỏi câu thứ hai.

### 2. Bất kỳ ai trong cụm cầm khoá đều gọi được mọi thứ — bán kính nổ bằng cả cụm

Không có phân quyền theo bên gọi. Một tiến trình bất kỳ **bên trong cụm** đọc được biến môi
trường ấy — một pod bị chiếm, một sidecar, một job debug, một service tương lai không liên
quan — thì gọi được **mọi RPC của mọi service**.

Thứ chặn điều đó lại **không phải khoá**, mà là **lớp mạng**: cổng không ra ngoài cụm, và
network policy giới hạn ai nói chuyện được với ai bên trong cụm. Đó là lý do mạng là **một
nửa quyết định chứ không phải một dòng chú thích**: bỏ lớp mạng đi thì thứ còn lại là một mật
khẩu dùng chung nằm trong biến môi trường của mọi tiến trình.

**Hệ quả vận hành, không thương lượng:** cấu hình network policy của cụm là **một phần của cơ
chế xác thực này**. Ngày nó bị nới, ADR này phải được đọc lại — chứ không phải chỉ đội devops
biết.

### 3. Một khoá nghĩa là xoay khoá phải đổi đồng thời ở mọi nơi

Luật 8 bất biến 6 đòi khoá ký và khoá phiên có **vòng đời và quy trình xoay được ghi lại**.
Ghi lại trung thực ở đây là ghi rằng **hình dạng đã chọn làm cho việc xoay có hình dạng một sự
cố**:

| | Khoá ký phiên (đã có) | Khoá service (quyết định này) |
|---|---|---|
| Hình dạng | **Danh sách** — khoá đầu ký, các khoá sau vẫn kiểm được (`core/config/config.go:264`) | **Một giá trị.** Không có chỗ cho khoá thứ hai |
| Xoay thế nào | Thêm khoá mới vào đầu, khoá cũ còn kiểm, gỡ sau | Đổi **đồng thời** mọi service. Trong khoảng lệch, bên cũ và bên mới **không nói chuyện được** |
| Cái giá | Một lần triển khai thường | Một cửa sổ mà lời gọi liên service **bị từ chối** |

Kho mã này đã có sẵn hình dạng xoay được, ngay cạnh, cho khoá ký phiên. Không dùng nó ở đây
là một lựa chọn có ý thức, và cái giá là mục bảng trên.

**Đường ra, nếu có ngày phải trả cái giá đó: sửa ADR này** — hoặc viết ADR mới thay nó. Đường
sai là **lặng lẽ thêm một header thứ hai**, hoặc lặng lẽ cho phép hai giá trị cùng hợp lệ.
Một cơ chế xác thực có hai hình dạng cùng lúc, mà không tệp nào nói vì sao, là cơ chế mà sáu
tháng sau không ai dám sửa.

---

## Bất biến

| # | Bất biến |
|---|---|
| 1 | **Đúng một** tên header, khai bằng **một hằng số ở một chỗ**, đặt cạnh `grpcx.MetadataTenantKey` — không service nào tự khai lại chuỗi của mình |
| 2 | Tên header **không nằm trong tiền tố `x-tenant`**. Tiền tố ấy đã có đúng một nghĩa — xã mà bên gọi thuộc về (ADR 0012 quyết định 1) — và bị `httpx.StripTenantHeaders` quét ở rìa HTTP. Cho nó mang nghĩa thứ hai là làm luật quét ấy không đọc được nữa |
| 3 | Giá trị **chỉ** đến từ biến môi trường / secret store. Không mặc định, không giá trị dự phòng trong mã. Thiếu biến = **tiến trình từ chối khởi động** (`core/config.ErrThieuBienMoiTruong`, `core/config/config.go:132`) — không bao giờ là "chạy không cần khoá" |
| 4 | Giá trị đi trong kiểu bảo vệ của `core/secret` (`core/secret/secret.go:57`), không bao giờ là `string` trần: một chuỗi trần là một chuỗi có ngày lọt vào `log.Info` (luật 8) |
| 5 | Header do **interceptor phía client** đặt, lấy từ cấu hình. Không bao giờ sao chép từ một yêu cầu HTTP vào — một header do client ngoài cung cấp mà lọt vào đây là client tự cấp cho mình tư cách nội bộ |
| 6 | **Interceptor phía server kiểm trước mọi handler.** So sánh **thời gian không đổi** (`hmac.Equal` hoặc `subtle.ConstantTimeCompare`), không phải `==` |
| 7 | Thiếu hoặc sai khoá = **từ chối**, `UNAUTHENTICATED`. Không có danh sách miễn: khác với danh sách miễn xã của ADR 0012, ở đây **không** có RPC nào "chưa thể biết khoá" |
| 8 | Từ chối vì sai khoá phải **log ở mức báo động** — không phải vì nó hay xảy ra, mà vì nó **không được phép xảy ra**: mọi bên gọi hợp lệ đều có khoá |
| 9 | Cổng gRPC **không lắng nghe trên địa chỉ công khai**, và network policy của cụm là một phần của cơ chế này (mục "không mua được" #2) |

## Còn mở — ghi ra để không ai tưởng đã chốt

| # | Còn mở |
|---|---|
| 1 | **Vòng đời khoá**: bao lâu xoay một lần. Chưa ai chốt. Luật 8 bất biến 6 đòi con số này, và quyết định hôm nay **chưa** trả lời |
| 2 | **Quy trình xoay từng bước** trong cụm — thuộc đội devops, và phải viết ra ở `kb/40-runbooks/` trước khi xã đầu tiên chạy thật |
| 3 | Khi nào (hoặc có bao giờ) chuyển sang mTLS/mesh. Quyết định này **không** đóng cửa đó; nó chỉ nói cửa đó không phải việc hôm nay |
| 4 | Bản ghi kiểm toán cho lời gọi liên service điền gì vào ô **"ai"** khi cơ chế này không trả lời được (mục "không mua được" #1) |

## Một hệ quả ĐÃ MỞ KHOÁ nhưng CHƯA được quyết

`core/grpcx/grpcx.go:69` giữ `ListTenants` và `ResolveTenantSuccession` **ngoài** danh sách
miễn xã, với lý do viết thẳng trong chú thích: *"caller authentication on the gRPC port FIRST,
these two names SECOND"* — hai RPC ấy trả lời về **mọi xã một lúc**, và ULID chúng trả về là
tiền tố của mọi khoá cache, hàng đợi, phòng realtime và đường dẫn tệp trong hệ thống (luật 1
bất biến 7).

**Điều kiện tiên quyết ấy đang được dựng.** Việc thêm hai tên đó vào danh sách vì thế **không
còn bị chặn** — nhưng nó **chưa được quyết**, và ADR này **không quyết nó**. Thêm một thành
viên vào danh sách miễn là **ĐIỀU KIỆN DỪNG** của chính ADR 0012 (quyết định 1, mục A: *"Thêm
một thành viên vào danh sách là ĐIỀU KIỆN DỪNG — hỏi người dùng, đừng tự quyết trong lúc viết
mã"*), và điều kiện dừng ấy không tự gỡ khi một điều kiện khác được thoả.

Ghi ở đây vì phiên sau sẽ đọc chú thích trong `grpcx.go`, thấy điều kiện đã xong, và tưởng
phần còn lại là việc kỹ thuật. Không phải. Nó là **một câu hỏi đang chờ người dùng**.

## ĐIỀU KIỆN DỪNG

1. Một bên gọi cần **quyền khác** với bên gọi khác — khoá dùng chung không làm được, và "thêm
   một khoá nữa" là lúc phải quay lại đây chứ không phải lúc viết mã
2. Một bản ghi kiểm toán cần nêu **đích danh service nào** đã gọi — xem "không mua được" #1
3. Đề xuất **hai giá trị khoá cùng hợp lệ**, hay một header thứ hai, dù nhân danh việc xoay khoá
4. Đề xuất cho khoá này một **giá trị mặc định**, hay cho tiến trình chạy khi thiếu biến
5. Lớp mạng bị nới: cổng gRPC ra khỏi mạng nội bộ cụm, hoặc network policy bị gỡ
6. Thêm `ListTenants` / `ResolveTenantSuccession` vào danh sách miễn xã — xem mục trên

→ ADR 0012 (ranh giới gRPC; quyết định 3 **đã bị tệp này thay thế**): `kb/10-decisions/0012-grpc-boundary-contract.md`
→ ADR 0003 (`platform` chỉ trả siêu dữ liệu — căn cứ cũ, và vẫn là thứ giữ cho `x-tenant-id` khai sai vô hại): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0005 (ULID công khai trong deep link): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0009 (bí mật theo xã, mã hoá khi lưu): `kb/10-decisions/0009-per-tenant-secret-encryption.md`
→ Luật 2 (hai đường đi giữa các service): `.claude/rules/critical/2-service-boundary.md`
→ Luật 6 (ô "ai" của bản ghi kiểm toán): `.claude/rules/critical/6-audit-log.md`
→ Luật 8 (bí mật chỉ ở secret store; vòng đời và quy trình xoay): `.claude/rules/critical/8-secrets-config.md`
