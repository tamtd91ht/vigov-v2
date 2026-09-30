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
  - "cờ --vao-thang: app riêng nung tên miền xã vào bundle và mở thẳng vào xã, không màn ViHAT, không bước xác nhận — ngoại lệ duy nhất của câu 1 (chủ dự án, 27/09/2026)"
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
| 0044 | Phương án bị loại *"Build riêng cho từng xã"* (`:47`) | Được dùng **hẹp** cho app riêng có `--vao-thang`: chỉ tên miền xã vào bundle (mục 6, 27/09 tối) |

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
| 5 | Tên tham số tên miền và số phận `v` | Chi tiết cài đặt; không có QR nào đã in nên chưa có nợ. **Đóng 30/09/2026** — §*Trả lời của người dùng — 30/09/2026* |

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

#### 6. Cờ `--vao-thang` — app riêng mở thẳng vào xã (27/09/2026, tối)

Phát hành thử app riêng Thăng Bình (App ID `3291993990104489440`) cho thấy câu 1 để lại một lỗ: bundle
**giống hệt từng byte** giữa app chung và app riêng, nên app riêng mở ra màn giới thiệu ViHAT Group
như app chung. Chủ dự án chốt:

> *"nên có 1 cờ đi … nếu có cờ đó thì vào thẳng app xã, nếu không có thì vào từ app vihat nhưng qr
> chuyển hướng sang app citizen"* — cờ ở **lệnh đẩy** (lúc dựng), không ở tham số QR.

| Câu | Nay |
|---|---|
| 1 — *"Không giá trị nào theo xã vào bundle"* | **Ngoại lệ đúng một:** `deploy.mjs --domain=<x> --vao-thang` nung tên miền `<x>` vào `__VIGOV_XA_CO_DINH__`. Không có cờ thì bundle vẫn là một, như câu 1 |
| Mở app riêng có cờ (chủ dự án, 28/09: *"chọn ra nó trên zalo mini app … vào ứng dụng thì sẽ thấy view của xã"*) | Mở là **trang của xã**: tra `/communes` theo tên miền, rồi kênh công dân — tin tức, danh bạ đọc ngay. **Không bước xác nhận, không đăng nhập lúc mở, không qua `vihat-miniapp`.** `d` trên QR bị bỏ qua — một app riêng chỉ phục vụ một xã. Xã sau (Sông Hàn…) y hệt, không sửa mã |
| Giao diện app riêng có cờ (27/09 khuya: *"bỏ hết thông tin VihatGroup đi"*, *"nút xác nhận xã … nên bỏ nó"*) | **Không một chữ ViHAT Group**: không màn giới thiệu, không thanh tab, không nút chat OA; header chỉ tên xã; **thanh tiêu đề gốc của Zalo bị ẩn** (`app.actionBarHidden` cho đúng lần đẩy, vì `app.title` chung là *"ViHAT Group"* — 28/09: *"bỏ luôn view đó đi"*). Tra xã hỏng: một câu và nút "Thử lại". Kênh là màn gốc, không nút "Quay lại". Mã phần thương mại vẫn nằm trong bundle nhưng không đường nào tới được — `App.tsx` `AppRieng`, `cong-dan/man/TrangXa.tsx` |
| Nguồn giao diện app riêng (chủ dự án, 28/09: *"lấy từ D:\works\vihat\outsources\vi-gov\zalo-miniapp"*) | Bố cục, màu, hình của bản mẫu ấy: header xanh bo đáy, lưới truy cập nhanh, "Phản ánh của tôi", "Tin tức mới", thanh tab dưới có nút "Gửi phản ánh" nổi. **Không lấy** router, `localStorage`, tên xã từ biến môi trường, lớp gọi máy chủ, màn định danh bắt buộc, quét căn cước, bản đồ, truyền thanh, video. Ô không có dữ liệu thật thì **không hiện**. Cỡ chữ, vùng chạm, độ tương phản nâng theo `accessibility.test.ts` — `cong-dan/man/TrangXa.tsx`, lớp `.xa-*` trong `styles.css` |
| Logo xã trên header (chủ dự án, 28/09: *"copy tạm sang đâu đó dùng trước, sau này nó sẽ cấu hình ở platform-admin"*) | **Tạm:** `citizen-app/scripts/logo-xa/<tên-miền>.png`, `deploy.mjs` chép vào bản dựng `--vao-thang` rồi xoá — giá trị theo xã thứ hai trong bundle, chủ dự án chấp nhận cho tới khi `ho_so_hien_thi_xa.logo_url` được cấu hình qua platform-admin và đọc lúc chạy |
| Đủ màn như bản mẫu + người dùng trải nghiệm (chủ dự án, 28/09: *"làm đủ các màn như bản mẫu đi… Nếu phải xin quyền thì hãy fake tạm 1 cái tên và 1 số điện thoại… tôi muốn nó phải là 1 hành động"*) | Định danh → trang chủ (lời chào, tên người dùng, chuông, 6 ô) → Phản ánh · Tin tức · Cá nhân, cùng gửi phản ánh, chi tiết phiếu, tra cứu hồ sơ, danh bạ, truyền thanh, video, bản đồ, thông báo. MỘT hành động `LayNguoiDung` (`cong-dan/man/trai-nghiem.ts`): hôm nay trả người dùng giả lập cố định (`0900000000`), vì tên cần hộp xin quyền `getUserInfo` và số cần app secret ở máy chủ; ngày có quyền, `App.tsx` tiêm hàm thật vào đúng chỗ ấy. Phiếu trải nghiệm CHỈ TRONG BỘ NHỚ, mã `TN-…`, gắn nhãn, **không gửi vào hệ thống của xã** (phiếu mang danh tính giả không được thành hồ sơ lưu trữ — luật 4, 6). Khác bản mẫu có chủ đích: không bước chọn lĩnh vực (ADR 0028), không số ngày cam kết viết cứng (luật 10), không ảnh/GPS (chưa có kho tệp), không sửa/thu hồi/đánh giá, không quét căn cước (luật 3), màn chưa có dữ liệu hiện trạng thái trống bằng lời |
| Phản ánh trải nghiệm theo hợp đồng thật (28/09, chủ dự án cho tự làm theo đề xuất) | Form đúng năm trường `service-petitions` nhận (không "tiêu đề" như bản mẫu), phiếu mang kiểu `PhieuCuaToi`, nhãn `TRANG_THAI`, dòng thời gian theo vòng đời thật, chi tiết đủ trường của `THE_PHIEU`, tra cứu theo mã. Hai mốc hạn để trống có lời — hạn chỉ `identity` đếm được. Nút "Lấy vị trí hiện tại" chỉ nhận MÃ vị trí (`getLocation` không trả toạ độ); không vẽ bản đồ vì hiện điểm trên bản đồ là gửi toạ độ cho bên thứ ba (luật 3, điều kiện dừng #2). Danh sách API còn thiếu: sổ tiến độ `citizen-app/api-con-thieu-app-rieng` |
| Không đăng nhập — xin quyền họ tên (chủ dự án, 28/09: *"không còn đăng nhập nữa, chỉ cần xin quyền để lấy được name, phone number"*) | Bỏ màn định danh và người dùng giả lập. **Họ tên:** `getUserInfo` kèm hộp xin quyền của Zalo, nút "Lấy họ tên từ Zalo" ở form gửi phản ánh và màn Cá nhân — lời khai `nua: "nha-nuoc"` đầu tiên; test cấm `getUserInfo` chuyển từ tuyệt đối sang có phạm vi (chỉ `features/tinh-nang/`). **Số điện thoại:** KHÔNG xin — `getPhoneNumber` chỉ trả mã, đổi ra số cần máy chủ có app secret; xin quyền mà không dùng được là làm phiền người dân và là lý do Zalo trả hồ sơ; người dân tự gõ số. Cần bật quyền `scope.userInfo` cho App ID ở trang quản trị Zalo |
| Đính chính các dòng trên (28/09 tối) | Nút "Gửi phản ánh" nổi ĐÃ BỎ — thanh tab 4 mục theo prototype khách; "không bước chọn lĩnh vực" ĐÃ THAY bởi ADR 0049 (dân gợi ý, cán bộ chốt). Các dòng trên giữ nguyên chữ vì là lịch sử quyết định; dòng này thắng khi nói khác |
| 28/09/2026 → ADR 0050 | ADR 0049 ở dòng trên **đã bị ADR 0050 thay**: lĩnh vực dân chọn là lĩnh vực của phiếu. Dòng này thắng dòng trên khi nói khác |
| Giao diện theo prototype khách bản Zalo v30 (người dùng, 30/09/2026: *"1. Màu đỏ. 2. làm giống logo. 3. bỏ. 4. làm luôn"*; ảnh `citizen-app/ui-design/*.jpg`) | **Màu: ĐỎ** theo `vigov-require/apps/miniapp/src/styles/tokens.css` (`--gov`), thay "header xanh" của dòng nguồn giao diện ở trên. Màu báo lỗi giữ riêng; lỗi luôn có chữ và biểu tượng. **Ảnh bìa trang chủ:** như logo, là ngoại lệ tạm: `scripts/banner-xa/<tên-miền>.png` chép vào bản dựng `--vao-thang`, không có tệp thì **ẩn khối**. Đây là giá trị theo xã thứ ba trong bundle, được chấp nhận tới khi banner do xã tự đăng (loại nội dung `banner`, spec 11) đọc được lúc chạy. **Không "lượt xem"**: máy chủ không đếm. **Tab Tin tức:** 3 tab Tin tức · Sự kiện · Thông báo, không "Tất cả", cộng chip danh mục hai tầng của xã (cha → con, chọn cha gồm cả con, chỉ hiện danh mục có tin). Tuyến công khai `GET /api/v1/commune-news/categories` và `?category=` được người dùng đồng ý mở (luật 13, tuyến không xác thực mới). **Tab Phản ánh:** nút "Gửi phản ánh mới" ở đáy. Dòng này thắng các dòng trên khi nói khác |
| Cờ `--demo` khi chưa nộp Zalo (người dùng, 30/09/2026: *"thêm cờ demo trong config, để chạy khi app chưa được submit, app submit tôi sẽ bỏ cờ"*, *"chỉ làm phía citizen-app"*) | `deploy.mjs --vao-thang --demo` dựng `__VIGOV_DEMO__`. Zalo không trả tên thì dùng "Nguyễn Văn Hùng". Bước số điện thoại hỏng thì vẫn mở màn, điền sẵn `0900000000`; việc cần máy chủ báo **một câu** "Chế độ demo…", không phiên, không gọi mạng, không mã phiếu bịa. Có dải "Chế độ demo" trên mọi màn. **Máy chủ không nới:** người dùng từ chối cờ ở `vihat-miniapp`, vì bỏ lượt đổi `phoneToken` là bỏ bước xác minh App ID (luật 1). Thiếu `--vao-thang` thì từ chối. Bản dựng thường không chứa chuỗi demo nào (`bundle-for-zalo.test.ts`) |
| Theo `citizen-app/ui-design/prototpye-spec/PROTOTYPE.md` (người dùng, 30/09/2026: *"giao diện từng màn"*, *"theo §12"*, *"nối api luôn"*, *"ok hết"* với 12 đề xuất) | **Chỉ lấy giao diện** §4–§7: khung, bộ thành phần giao diện, từng màn. **Không lấy** kiến trúc §2–§3, §9, §11 (mock, `X-Tenant-Code`/`X-Citizen-Id`, `localStorage`, lĩnh vực viết cứng). Giữ: nút chính đỏ, chữ ≥16px, chữ "Quay lại" cạnh mũi tên, "đang tải" là chữ, logo xã (không quốc huy). **Nối API:** (1) ảnh tin: tải lên kho tệp (ADR 0052), bản dẫn xuất ở bucket public, tuyến công khai trả `image_url`; (2) giờ đăng, thời gian và địa điểm sự kiện; (3) Truyền thanh: tệp âm thanh + thời lượng; Video: link mở ra ngoài; (4) Cảnh báo thiên tai: thực thể mới của `service-comms`, cán bộ đăng, **không bản đồ vị trí người dân**; (5) ảnh hiện trường: **không bắt buộc, tối đa 5**, bucket private, link ký có hạn gắn người gửi + xã; (6) giờ từng trạng thái người dân thấy (ADR 0041), không lịch sử chuyển nội bộ; (7) tra cứu hồ sơ một cửa **để sau** (luật 4 đk dừng #2); (8) Cá nhân hiện SĐT **đã che** từ phiên, không thôn; (9) chia sẻ Zalo và nhắn Zalo do lớp vỏ tiêm. **Giao diện:** (10) chuông rời header, thành dòng "Thông báo" ở Cá nhân; (11) giữ 3 mức cỡ chữ; (12) không công tắc "Nhận thông báo"; ô ảnh trên thẻ phiếu chỉ khi có ảnh; số phiên bản = nhãn bản dựng. **Tên xã chỉ ở header Trang chủ**, không thêm dòng tên xã vào header 12 màn con và 3 tab gốc còn lại (người dùng, 30/09/2026: *"vào app là đang biết làm việc với ai rồi"* — app riêng chỉ phục vụ một xã) |
| Chi tiết nối API (người dùng, 30/09/2026: *"còn lại đồng ý"*; G3 chờ trả lời) | **G1** giờ đăng ghi ở lần đăng ĐẦU, sau không đổi; `ngay_dang` giữ. **G2** cảnh báo thiên tai (`DisasterAlert`, `/disaster-alerts`) dùng quyền có sẵn `content.read`/`content.update`; loại: bão · lũ · sạt lở · mưa lớn; cấp độ cán bộ gõ; điểm sơ tán và số khẩn cấp nhập theo từng cảnh báo; chưa gửi ZNS. **G4** hai cột giờ mới trên phiếu (chuyển xử lý, chờ dân xác nhận), ghi lúc chuyển trạng thái, mở lại lấy giờ mới nhất; không đọc `nhat_ky_phan_anh`. **G5** `GET /api/v1/citizen/me` ở identity trả `phone_masked`. **G6** bỏ "Nhắn Zalo"; chia sẻ tin = link `zalo.me/s/<App ID>` kèm mã bài. **G7** âm thanh mp3/m4a, 30 MB, thời lượng cán bộ gõ; giới hạn thật đặt ở chính sách tải lên của platform. **G8** ảnh "sau xử lý" của cán bộ để sau. **G9** lời văn chính sách quyền riêng tư do Claude viết nháp, phiên bản 1.1 "chờ duyệt", chủ dự án duyệt trước khi nộp Zalo |
| Đăng nhập trong app riêng | **Chưa dựng.** Chỉ khi người dân làm việc cá nhân (gửi, xem phản ánh); máy chủ lấy xã từ App ID đã xác minh qua `mini_app`. Cần app secret của từng app và nơi giữ nó — quyết khi tới lượt. Tới lúc ấy ba lối phản ánh nói "chưa đăng nhập được" |
| Chính sách quyền riêng tư trong app riêng | Chưa có đường tới. Chủ dự án 28/09: Zalo đòi lúc nộp duyệt thì làm lúc ấy |
| App riêng **không** cờ, và app chung | Như cũ: màn ViHAT, vào xã bằng QR `d` + xác nhận |

Cài đặt — `citizen-app/scripts/deploy.mjs` (môi trường dựng `env_dung`) · `scripts/cau-hinh.mjs`
(`xaCoDinh`) · `vite.config.ts` (`define`) · `src/lib/xa-co-dinh.ts` · `src/App.tsx` ·
`src/cong-dan/man/TrangXa.tsx` · `scripts/dich-den.mjs` (`appConfigChoLanDay`).

**Vì sao nung được mà không mở lỗ cô lập** (luật 1 bất biến 10, cấm #2):

- Tên miền chỉ **dẫn giao diện**, cùng bản chất `d` trên QR công khai: tên xã vẫn do `/communes` trả,
  xã của phiên vẫn do máy chủ phân giải (`ResolveHost`). Nó không cấp gì mà một QR mang `d` không cấp.
- Người đặt duy nhất là `deploy.mjs`, và giá trị luôn là **đúng `--domain`** vừa chọn App ID đích —
  không có đường nào để app của xã A mang tên miền xã B. `.env.local` đặt tên ấy thì bước dựng DỪNG;
  shell còn sót tên ấy thì `deploy.mjs` xoá nó khỏi môi trường dựng khi không có cờ.
- Không có ULID, cấu hình hay số liệu nào của xã trong bundle: tên, hồ sơ, SLA vẫn đọc lúc chạy.

**Cái giá chủ dự án chấp nhận:** mỗi xã một bản dựng — sửa lỗi phải phát hành lại từng app riêng
(đúng lý do ADR 0044 §*Phương án* từng loại *"build riêng cho từng xã"*).

**Vì sao app riêng không mở phiên lúc mở** (đo 27/09 khuya, bản a861a9b–6859c14 đã làm thế rồi bỏ):
phiên chỉ mở được qua cầu của `vihat-miniapp`, cầu ấy gửi **App ID app chung**
(`vihat-miniapp/internal/httpapi/sessions_vigov.go:101`), nên máy chủ đi chế độ **chính** + đã xác nhận
(`service-identity/internal/app/cau_phien_cong_dan.go:214-227`, `:319-336`) và: đổi xã đã nhớ của
tài khoản dưới App ID app chung, thu hồi phiên xã khác, ghi vết `doi_xa_da_nho` như một lần công dân
xác nhận không ai bấm. Việc đổi token của app xã bằng secret của app chung **đã chạy được** trên máy
thật 27/09 — một chứng cứ cho UNKNOWN #1 (Zalo không chặn), chưa phải phép đo có kiểm soát.

**Hướng của chủ dự án (28/09/2026), ghi lại chưa dựng:** *"mô hình SSO. Tất cả các cấu hình của xã
bao gồm cả appId đều lưu database hết, platform-admin sẽ làm việc đó."* Hệ quả khi dựng: tệp
`citizen-app/scripts/ung-dung-theo-ten-mien.mjs` (câu 2) là **tạm** — nguồn cặp tên miền ↔ App ID sẽ là
bảng `mini_app` của `service-platform`, ghi qua platform-admin; việc một lần `gan-mini-app-thang-binh`
trong `deploy/Jenkinsfile` gỡ khi màn ấy có.

### Trả lời của người dùng — 30/09/2026: CÒN MỞ #5 đóng

Người dùng duyệt tên phiên làm việc đã chọn (hỏi trong phiên chính, chọn phương án đề xuất):

| Câu | Trả lời | Mã hôm nay |
|---|---|---|
| Tên tham số tên miền | **`d`**, đi cùng **`src`** ∈ {`qr`, `zns`} | `citizen-app/src/lib/launch-params.ts:96-103` (`thamSoXa`) đọc `d` và `src`; thiếu `src` hay `src` lạ thì bỏ qua |
| Số phận `v` | **Bỏ.** Có mặt thì bị lờ đi — không đọc, không báo | `launch-params.ts:85`; ca kiểm `launch-params.test.ts:66-67` ghim điều ấy |

Tên chuẩn của tham số và của tuyến tra xã theo tên miền (`GET /api/v1/communes?host=`, cùng lượt
duyệt) nằm ở `kb/00-foundation/ubiquitous-language.md` §Tên tài nguyên trên URL.

**Đính chính mục 5 ở trên:** dòng `GET /api/v1/communes?host=` ghi `CitizenOnly` + `KhongThuocXa`;
mã hôm nay khai `authz.Public(...)` (`service-identity/internal/http/routes_cong_dan.go:100-102`,
`danh_muc_xa_test.go:18` ghi *"PUBLIC since 2026-09-27"*). Mục này không quyết lại điều ấy, chỉ ghi
thứ mã đang làm.

## ĐIỀU KIỆN DỪNG

1. Đề xuất lưu **tên miền** làm tham chiếu xã ở bất kỳ đâu (phiên, xã đã nhớ, vết, bản ghi nghiệp vụ)
2. Đề xuất tệp dựng hay bundle mang **giá trị theo xã** ngoài việc chọn App ID đích — trừ tên miền
   của `--vao-thang` (mục 6); nới ngoại lệ ấy (thêm giá trị khác, hay cho nguồn khác ngoài `--domain`
   đặt nó) là quyết định mới
3. Đề xuất để tệp dựng **quyết xã** của một App ID thay cho bảng platform
4. Đổi nghĩa `tenant_hint`, hoặc cho tham số tên miền **của QR/URL** vào phiên mà **không** qua xác
   nhận (tên miền nung bằng `--vao-thang` được miễn — mục 6)
5. Tuyến tên miền → xã trả **ULID** cho client
6. Trỏ tên miền sang xã khác mà **không ghi vết**

→ ADR 0005 · 0019 · 0044 · 0045: `kb/10-decisions/`
→ ADR 0022 (xã từ phiên, `KhongThuocXa`) · ADR 0046 (host nào là của xã)
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md`
