---
id: 0047-hai-luong-dung-citizen-app-theo-ten-mien
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 6f01382
expires: null
owns_facts:
  - "một biến thể dựng citizen-app cho cả app chung và app riêng; 'demo' là cách nói về giai đoạn, không phải khái niệm kỹ thuật (trả lời 27/09/2026)"
  - "đích đẩy của zmp-cli 4.0.3 là claim appId trong ZMP_TOKEN: một token một App ID, Jenkins giữ một credential cho mỗi App ID"
  - "rủi ro đã chấp nhận: app chung xác nhận theo tên xã nhưng gửi tên miền, trỏ lại tên miền ở giữa đưa công dân vào xã không thấy tên"
  - "hai luồng dựng citizen-app: truyền tên miền xã thì đẩy lên App ID riêng của xã, không truyền thì đẩy lên app chung — một mã nguồn, một bundle, tên miền chỉ chọn App ID đích"
  - "vì sao tệp ánh xạ tên miền → App ID ở citizen-app/scripts không vi phạm ADR 0044 điều kiện dừng #1, và vì sao nó không phải nguồn sự thật về xã của một App ID"
  - "tham số QR/URL của app chung mang TÊN MIỀN xã thay cho t=<ULID>; máy chủ phân giải; tên miền không bao giờ được lưu làm tham chiếu xã"
  - "xã sáp nhập: trỏ tên miền cũ sang xã kế thừa, có vết, thay cho thông báo kế thừa qua t"
  - "vì sao phát hành app riêng được phép khi UNKNOWN #1 của ADR 0045 chưa đo, và cái giá của quyết định ấy"
---

# 0047. Hai luồng dựng `citizen-app` theo tên miền xã

**Trạng thái:** đã chốt · **Ngày:** 2026-09-27 · **Chủ dự án chốt** tám quyết định dưới đây ·
**Thay thế một phần** ADR 0005, 0019, 0044, 0045 (chỉ các điểm ở §*Thay thế gì*; thân các ADR ấy
giữ nguyên từng chữ, chỉ thêm một dòng trỏ có ngày) · ADR 0018, 0022, 0031, 0032 đứng nguyên

## Bối cảnh

ADR 0044 chốt hai chế độ (app chính mở xã bằng QR `t=<ULID>`, app riêng gắn xã theo App ID) và
một bản build. ADR 0045 dựng cầu phiên theo khuôn ấy. Ngày 27/09/2026, khi bắt đầu làm luồng phản
ánh của người dân trên Mini App, chủ dự án chốt lại **cách dựng và cách mở xã**:

> *"App gồm 2 phần: 1 là miniapp vihat làm app chính và chuyển tới citizen app qua params url
> chứa thông tin domain. 2 là app citizen submit chính theo domain luôn, mỗi domain là 1 mini app
> (ID riêng)."*
>
> *"Nó là 2 luồng, khi nhấn build nếu truyền domain lên thì nó build theo appId riêng của xã đó,
> nếu không thì build app chung."*

Hiện trạng đo được lúc ghi:

| Điều | Nơi |
|---|---|
| Một xã giữ được **nhiều** tên miền — thiết kế sẵn cho sáp nhập | `service-platform/internal/domain/tenant.go:19-25` |
| Platform đã có `ResolveHost` (host → xã) | `proto/vigov/platform/v1/platform.proto:74` |
| Cầu phiên nhận `tenant_hint` là **ULID** của `t=` | `proto/vigov/identity/v1/citizen_session_bridge.proto:80-82` |
| Hai biến thể dựng `goc` / `day-du` đã có | `citizen-app/scripts/dung.mjs:6-7`, `:26` |
| Chưa có bộ sinh QR ở kho nào; **chưa phát hành tấm QR nào** | lời chủ dự án, 27/09 |

## Quyết định

| # | Câu | Trả lời của chủ dự án |
|---|---|---|
| 1 | Dựng thế nào | **Một mã nguồn, một bundle.** Tên miền truyền lúc dựng **chỉ chọn App ID đích** (*"Chỉ chọn App ID"*). Không giá trị nào theo xã vào bundle |
| 2 | Ánh xạ tên miền → App ID ở đâu | Một **tệp cấu hình dựng** trong `citizen-app/scripts/`. App ID không phải bí mật. Tệp không vào bundle |
| 3 | App chung mở xã bằng gì | Tham số URL/QR mang **tên miền xã** (ví dụ `xa-a.vigov.vn`) thay cho `t=<ULID>`. Máy chủ phân giải tên miền → xã (`ResolveHost`). Công dân **xác nhận một lần**, rồi **làm việc ngay trong app chung** — không chuyển sang app riêng |
| 4 | `t=<ULID>` | **Bỏ hẳn.** Xã sáp nhập: người vận hành trỏ tên miền cũ sang xã kế thừa, có ghi vết — QR đã in vẫn mở được |
| 5 | Biến thể dựng | App chung = `goc`, app riêng của xã = `day-du`. Jenkins chọn theo **có hay không** tham số tên miền |
| 6 | Tuyến "tên miền → {tên xã, tỉnh}" cho màn xác nhận | Thuộc `service-identity` (gọi `ResolveHost` qua gRPC). `CitizenOnly` + `KhongThuocXa`. Trả **chỉ** tên và tỉnh, **không bao giờ** ULID. **Chưa dựng** |
| 7 | Hợp đồng cầu | Thêm **một trường tuỳ chọn MỚI** vào `OpenCitizenSessionRequest` mang gợi ý tên miền. `tenant_hint` **không bị đổi nghĩa** thành tên miền; vì `t` bỏ hẳn (câu 4) nó chỉ còn là trường ngừng dùng. Tên và hình dạng trường mới nằm ở `.proto`. Identity phân giải bằng `ResolveHost`. **Chưa dựng**; `vihat-miniapp` phải chuyển tiếp trường ấy |
| 8 | Jenkins | Một Jenkinsfile riêng trong `citizen-app/` (chưa có) có tham số `domain`, theo các chốt chặn của `deploy/Jenkinsfile`. `ZMP_TOKEN` là **credential của Jenkins**, ghi vào `.env` của đúng lần chạy rồi xoá. Chủ dự án chọn *"Credential, cho phát hành luôn"*: được **phát hành** (không chỉ bản thử) lên App ID riêng ngay bây giờ |

## Vì sao từng điểm không phá luật đang đứng

**Câu 1–2 và ADR 0044 điều kiện dừng #1.** Điều kiện ấy cấm *"tệp cấu hình theo App ID"* vì nó sợ
**giá trị theo xã vào bundle**. Tệp ở câu 2 chỉ quyết **đẩy bundle lên đâu**; bundle đẩy lên mọi
App ID là cùng một bundle. Vì vậy điều kiện #1 được nới **đúng cho một tệp chọn đích** — mọi giá trị
theo xã trong bundle (biến môi trường, hằng số, nhánh mã) **vẫn cấm**.

**Tệp dựng không phải nguồn sự thật về xã.** Bảng `app_id → tenant_id` của `service-platform`
(ADR 0044, ADR 0045 §*Bảng*) vẫn là nơi **duy nhất** máy chủ đọc để biết một App ID phục vụ xã nào.
Hai nơi cùng nói về một cặp (xã, App ID) thì **sẽ lệch**: tệp dựng đẩy bundle lên App ID X, còn bảng
platform gắn X với xã khác hoặc chưa gắn. Khi lệch, máy chủ thắng — app riêng mở ra bị **từ chối**
hoặc vào đúng xã bảng nói, không bao giờ vào xã tệp dựng nói. Lệch là lỗi vận hành **ồn ào**, không
phải rò dữ liệu. Chưa có phép kiểm nào đối chiếu hai nơi — xem §*CÒN MỞ*.

**Câu 3 và luật 1 bất biến 2 / cấm #5.** Tên miền ở đây chỉ là **khoá tra** mà máy chủ phân giải.
Phiên, xã đã nhớ, vết đều giữ **ULID**. Tên miền **không bao giờ** được lưu làm tham chiếu xã — lưu
nó là đúng thứ cấm #5 cấm, vì tên miền đổi khi sáp nhập còn bản ghi lưu trữ thì không được viết lại.

**Câu 3 và luật 1 cấm #2.** Tham số vẫn là **dữ liệu client cung cấp**: nó dẫn giao diện, không cấp
gì. Xã vào phiên chỉ bằng **hành vi xác nhận tường minh** của công dân (ADR 0045 §*Chế độ*, ADR 0022).
Quy tắc `src` ∈ {`qr`, `zns`} của ADR 0045:311 áp nguyên: `src` khác, hoặc tham số tên miền không
kèm `src`, thì **bỏ qua tham số** — cư xử như mở không tham số.

**Câu 4 — vì sao bỏ được `t` mà không mất QR đã in.** Khuôn `t` bị khoá (ADR 0019:94) vì QR in ra
sống nhiều năm. Hôm nay **chưa có tấm nào**, nên không có nợ tương thích. Sau sáp nhập, việc giữ QR
cũ mở được chuyển từ *"bảng kế thừa tra theo `t`"* sang *"tên miền cũ trỏ sang xã kế thừa"* — đúng
khả năng một xã giữ nhiều tên miền đã có sẵn (`tenant.go:19-25`). Việc trỏ lại là thao tác của
người vận hành và **phải ghi vết** (luật 6).

**Câu 7 và luật 2 cấm #4.** Đổi nghĩa `tenant_hint` từ ULID sang tên miền là đổi một trường đã có
bên tiêu thụ. Trường mới tuỳ chọn thì bên cũ không gãy. Trường ấy cùng bản chất `tenant_hint`: một
**gợi ý**, chỉ có hiệu lực kèm `commune_confirmed` ở app chính — **không phải** *"trường xã khai cho
bên gọi"* mà ADR 0045 điều kiện dừng #3 cấm.

## Câu 8 — cái giá của *"cho phát hành luôn"*

Quyết định này **thay** ADR 0045 điều kiện dừng #4 (*"Phát hành app riêng khi UNKNOWN #1 chưa
đo"*). Chủ dự án chọn nó khi biết giá sau:

| Rủi ro | Mức |
|---|---|
| Nếu Zalo **không** từ chối `accessToken` đổi bằng secret của app khác (UNKNOWN #1, chưa đo) thì *"App ID đã xác minh"* không có thật: công dân có thể vào một xã khác qua app riêng | Chỉ vào được xã mà họ **vốn vào được** bằng QR công khai. **Không** lộ dữ liệu của công dân khác — phiên chỉ đọc hồ sơ của chính mình (ADR 0045:281, luật 4) |
| **Một** `ZMP_TOKEN` đẩy được lên **mọi** App ID là một chứng cứ mới do CI giữ, ảnh hưởng **mọi xã** (luật 8) | Lộ nó là ai cũng thay được app của mọi xã. **Vòng đời và cách xoay chưa ai chốt** (luật 8 bất biến 6) |

UNKNOWN #1 **vẫn mở và vẫn phải đo** — quyết định này bỏ chốt chặn, không trả lời câu hỏi.

## Thay thế gì của ADR cũ

| ADR | Điểm bị thay | Thay bằng |
|---|---|---|
| 0005 | Khuôn deep link `t=<tenant_ulid>` (`:54-61`) | Tham số mang tên miền xã (câu 3). `src` giữ như ADR 0045:311 |
| 0005 | Bảng alias/kế thừa tra theo `t` để báo *"đã sáp nhập vào X"* (`:85-89`) | Tên miền cũ trỏ sang xã kế thừa, có vết (câu 4) |
| 0019 | Khoá khuôn QR khám phá (`:94`) | Không còn khuôn `t` để khoá; chưa có QR nào đã in |
| 0044 | *"Không thay: … Khuôn deep link `t`/`src`/`v`"* (`:95-96`) | Câu 3–4 |
| 0044 | Điều kiện dừng #1, chữ *"tệp cấu hình theo App ID"* | Nới cho **tệp chọn đích** (câu 2). Giá trị theo xã trong bundle vẫn cấm |
| 0045 | ZNS trỏ *"app chính kèm `t`"* (`:55`) | App chính kèm tham số tên miền |
| 0045 | Điều kiện dừng #4 | Câu 8 |

**Không thay:** ba lớp khám phá – phiên – uỷ quyền (ADR 0005). Một API host duy nhất. Xã do máy chủ
quyết (ADR 0044). Toàn bộ cầu phiên, khoá cầu, cổng riêng (ADR 0045). QR ghép phiên và `src=pair`
(ADR 0019, trừ dòng khoá khuôn). Một OA xác thực (ADR 0018, 0031).

## Hệ quả

- **Dễ hơn:** thêm một app riêng = một dòng tệp dựng + một dòng `MiniApp` ở platform + một secret
  ở `vihat-miniapp`. Không đụng mã.
- **Khó hơn:** hai nơi nói về cùng một cặp xã–App ID (tệp dựng, bảng platform) phải được giữ khớp.
- **Chưa dựng, chặn luồng app chung:** tuyến tên miền → tên xã (câu 6) và trường mới trên cầu
  (câu 7, cần cả `vihat-miniapp`).

## CÒN MỞ

| # | Câu | Vì sao chưa đóng |
|---|---|---|
| 1 | Vòng đời và cách xoay `ZMP_TOKEN` | Luật 8 bất biến 6 đòi con số; chủ dự án chưa nêu |
| 2 | UNKNOWN #1 của ADR 0045 | Vẫn phải đo; câu 8 chỉ bỏ chốt chặn |
| 3 | Phép kiểm đối chiếu tệp dựng với bảng `MiniApp` của platform | Chưa có; lệch hôm nay chỉ lộ khi công dân mở app |
| 4 | Nội dung hai biến thể: `dung.mjs:6-7` hiện ghi `day-du` = thêm lớp khám phá, **danh mục xã mẫu, bảng chẩn đoán**, *"thử nghiệm, demo"*; còn `goc` = *"bản nộp, không lớp khám phá"*. Câu 5 đặt `day-du` cho app riêng **phát hành** và `goc` cho app chung **cần** nhận tham số tên miền | Chủ dự án chốt tên biến thể, chưa chốt nội dung. Phát hành `day-du` như hiện trạng là đưa danh mục xã mẫu và bảng chẩn đoán vào app thật của một xã |
| 5 | Tên tham số tên miền và số phận `v` | Chi tiết cài đặt; không có QR nào đã in nên chưa có nợ |

### Trả lời của chủ dự án — 27/09/2026, sau khi ADR được viết

Mục này ghi thêm, không sửa phần trên: phần trên là quyết định lúc viết, mục này là câu trả lời và
phần đính chính đo được sau đó. Chỗ nào mục này nói khác phần trên thì **mục này thắng**.

#### 1. Câu 5 bị thay — một biến thể cho cả hai luồng

> *"bỏ hoàn toàn demo đi, demo ở đây là ngôn ngữ nói hiểu về cách làm, không phải là khái niệm kỹ
> thuật, về kỹ thuật nó là app dùng thật"*

Chủ dự án chọn **"Gộp một biến thể"**:

| Trước (câu 5, CÒN MỞ #4) | Nay |
|---|---|
| App chung = `goc`, app riêng = `day-du` | **Một biến thể** cho cả app chung và app riêng |
| `day-du` mang danh mục xã mẫu, bảng chẩn đoán | **Bỏ** tách `goc`/`day-du`, **bỏ** danh mục xã mẫu, **bỏ** bảng chẩn đoán |
| Jenkins chọn biến thể theo có/không tham số tên miền | Tham số tên miền **chỉ** chọn App ID đích (câu 1 đứng nguyên) |
| Luật *"bản nộp không có chữ nhà nước"* — lượt quét từ cấm của `citizen-app/src/bundle-for-zalo.test.ts` | **Bị thay.** Bản nộp Zalo của app chung phải **nói rõ** app mang kênh phản ánh tới các xã |

"Demo" từ nay chỉ là cách nói về **giai đoạn** làm việc với một xã (mục 2), không phải một bản dựng,
một cờ hay một nhánh mã. **CÒN MỞ #4 đóng** theo bảng trên. Việc sửa mã `citizen-app` (bỏ biến thể,
đổi lượt quét từ cấm) **chưa làm** ở thời điểm ghi mục này.

#### 2. Hai giai đoạn với một xã — lời chủ dự án

> *"Có 2 giai đoạn để thực hiện dự án với 1 xã bất kỳ, 1 là giai đoạn demo, lúc đó chưa xin được
> công văn của xã để submit 1 app riêng cho xã đó, nên sẽ dùng app ở vihat-miniapp tạo ra 1 qr có
> gắn domain của xã này để redirect về màn hình app citizen, thời điểm này tôi sẽ cấu hình thông tin
> xã trên database (có thể từ web admin vihat), sau đó chạy lệnh tạo 1 qr app ở bản test để họ vào
> trải nghiệm. giai đoạn 2 là sau khi chốt hợp đồng với xã, xin được công văn yêu cầu xác thực để
> zalo chấp nhận submit app, lúc đó mới tiến hành submit app riêng cho xã (appId riêng), người dân
> của xã đó vào kho mini app, tìm và chọn app này để sử dụng."*

Thêm hai điểm chủ dự án nêu cùng lúc:

- App chung **cũng được phát hành** lên kho Zalo (*"cả bản phát hành"*), không chỉ chạy bản thử.
  QR có thể trỏ bản thử hoặc bản phát hành.
- QR được sinh từ **khu vực vận hành ViHAT trong web-admin** → ADR 0048. Hai giai đoạn gốc: ADR
  0044 §Bối cảnh.

#### 3. Đính chính câu 8 — đích đẩy do token quyết, không do `APP_ID`

Đo ở commit `60bf7a8`, đầu tệp `citizen-app/scripts/dich-den.mjs` (zmp-cli **4.0.3**):

| Câu 8 và §*Câu 8 — cái giá* ghi | Thực tế đo được |
|---|---|
| `ZMP_TOKEN` ghi vào `.env` của đúng lần chạy rồi xoá | **Không ghi gì vào `.env`.** Đường app riêng đọc `ZMP_TOKEN` từ **môi trường** và kiểm claim `appId` của nó khớp App ID đích trước khi dựng (`citizen-app/scripts/deploy.mjs`, cùng commit) |
| **Một** `ZMP_TOKEN` đẩy được lên **mọi** App ID | **Sai.** Đích đẩy là claim `appId` **trong** `ZMP_TOKEN`; `APP_ID` không có tác dụng. **Một token — một App ID** |

Hệ quả cho Jenkins và luật 8:

- Jenkins cần **một credential cho mỗi App ID** — N xã có app riêng là N credential, cộng một cho
  app chung.
- Lộ một token là thay được **đúng một** app, không phải mọi xã. Riêng token của **app chung** chạm
  mọi xã đang dùng app chung ở giai đoạn 1 — đó là credential nặng nhất.
- Luật 8 bất biến 6 vẫn chưa đạt: **vòng đời và cách xoay của từng credential chưa ai chốt**. CÒN MỞ
  #1 nay đọc là *"vòng đời và cách xoay của N credential `ZMP_TOKEN`"*.

#### 4. Rủi ro chủ dự án chọn chấp nhận — xác nhận theo tên, gửi theo tên miền

Ở app chung, màn xác nhận hiện **tên xã** (tuyến câu 6), nhưng lời xác nhận gửi lên cầu **chỉ mang
tên miền**; máy chủ phân giải tên miền **lúc xác nhận**. Nếu người vận hành trỏ lại tên miền (câu 4)
đúng khoảng giữa hai lần ấy, công dân vào một xã **mà họ không thấy tên** trên màn xác nhận.

| Vì sao chấp nhận | |
|---|---|
| Hiếm | Chỉ xảy ra khi có thao tác trỏ lại tên miền rơi đúng vào khoảng giữa lúc hiện tên và lúc xác nhận |
| Chỉ người vận hành gây ra được | Công dân hay client không đổi được ánh xạ; việc trỏ lại có vết (ĐIỀU KIỆN DỪNG #6) |
| Phiên nói thật | Phiên trả `tenant_display_name` của xã **thật sự** đã vào, không phải tên đã hiện trên màn xác nhận |

App riêng **không có bước xác nhận nào**: *"Người dân vào app riêng là vào xã rồi còn xác nhận gì
nữa"*. Xã của app riêng do bảng `mini_app` quyết (ADR 0044).

#### 5. Đã dựng đến đâu — chứng cứ

| Việc | Commit |
|---|---|
| `.proto`: trường tuỳ chọn `commune_host_hint` trên cầu (câu 7) | `39d4397` |
| `GET /api/v1/communes?host=` ở `service-identity`, `CitizenOnly` + `KhongThuocXa`, chỉ trả tên và tỉnh (câu 6) — `service-identity/internal/http/danh_muc_xa.go` | `2f075b9` |
| Identity phân giải `commune_host_hint` qua `ResolveHost`, thôi nhận `tenant_hint` | `6f01382` |
| `deploy.mjs --domain` chọn App ID đích theo tên miền (câu 1–2) | `60bf7a8` |
| `vihat-miniapp`: bên gọi cầu `OpenCitizenSession` — **tắt khi chưa cấu hình**, và **bị chặn** bởi UNKNOWN #2 của ADR 0045: chưa endpoint nào **đã đo** trả mã tài khoản Zalo | `vihat-miniapp` `e274d21` |

**Luồng xác nhận của app chung chưa chạy được đầu–cuối.** Trước khi xác nhận, app chung mở một
**phiên không xã**; phiên ấy đòi **bảng vết chưa thuộc xã** của câu mở #25 (ADR 0045:247-251) —
**chưa dựng** (không migration nào tạo bảng ấy ở thời điểm ghi mục này).

## ĐIỀU KIỆN DỪNG

1. Đề xuất lưu **tên miền** làm tham chiếu xã ở bất kỳ đâu (phiên, xã đã nhớ, vết, bản ghi nghiệp vụ)
2. Đề xuất tệp dựng hay bundle mang **giá trị theo xã** ngoài việc chọn App ID đích
3. Đề xuất để tệp dựng **quyết xã** của một App ID thay cho bảng platform
4. Đổi nghĩa `tenant_hint`, hoặc cho tham số tên miền vào phiên mà **không** qua xác nhận
5. Tuyến tên miền → xã trả **ULID** cho client
6. Trỏ tên miền sang xã khác mà **không ghi vết**

→ ADR 0005 · 0019 · 0044 · 0045: `kb/10-decisions/`
→ ADR 0022 (xã từ phiên, `KhongThuocXa`) · ADR 0046 (host nào là của xã)
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md`
