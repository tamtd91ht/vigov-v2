---
id: 0032-webhook-mini-app-roi-khoi-vigov
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 917ce3f
expires: null
owns_facts:
  - "phép thử quyết định một bề mặt tích hợp Zalo thuộc kho nào: khoá bí mật nào ký nó"
  - "vì sao endpoint webhook Zalo Mini App rời service-platform sang kho vihat-miniapp, chốt 21/09/2026"
  - "quan hệ giữa kho vihat-miniapp và vigov-v2/citizen-app: ai quản trị app chính, ai là mã giao diện"
---

# 0032. Webhook Mini App rời khỏi ViGov — và phép thử quyết định điều đó

**Trạng thái:** đã chốt · **Ngày:** 2026-09-21 · **Nối tiếp ADR 0031** · **ADR 0018 giữ nguyên**

## Bối cảnh

Endpoint nhận webhook của Zalo Mini App từng nằm ở `service-platform`, gắn trên mux ngoài cùng
cạnh `/healthz`. Ngày 21/09/2026 chủ dự án chốt: nó **rời khỏi kho ViGov**, dựng lại ở kho
`vihat-miniapp` (`github.com/tamtd91ht/vihat-miniapp`, `internal/httpapi/`).

ADR này không ghi lại thao tác gỡ — `git log` giữ việc đó. Nó ghi lại **vì sao nó không thuộc
về đây**, và ghi lại **phép thử** để lần sau không phải suy luận lại từ đầu.

## Quyết định — phép thử: KHOÁ BÍ MẬT NÀO KÝ NÓ

Một bề mặt tích hợp Zalo thuộc hệ thống nào được quyết bởi **khoá bí mật nào ký nó**, không
phải bởi câu hỏi *"ai là nhà cung cấp"*.

| Bề mặt | Ký bằng khoá của ai | Thuộc kho |
|---|---|---|
| Webhook + sự kiện của **Mini App** | App secret của **bên đứng tên app** (ViHAT Group — ADR 0031) | `vihat-miniapp` |
| Đổi `accessToken` / `phoneToken` tại `graph.zalo.me` | **Cùng** app secret ấy | `vihat-miniapp` |
| Gửi **ZNS từ OA của TỪNG XÃ** | Khoá của **xã**, nằm trong kho bí mật theo từng xã (ADR 0009) | **Ở LẠI ViGov** — `service-comms` |

Phép thử này chia đúng chỗ ranh giới thật đi qua, vì khoá ký là thứ **không co giãn theo cách
đọc**: nó hoặc là khoá của một pháp nhân thương mại, hoặc là khoá của một cơ quan nhà nước.
Câu *"ai là nhà cung cấp"* thì co giãn được, và mục kế tiếp là bằng chứng nó đã co giãn.

**Dòng thứ hai đã đúng sẵn, không phải việc phải làm:** `citizen-app` gọi
`POST /api/v1/sessions` sang backend `vihat-miniapp`, khai rõ máy chủ ấy không phải ViGov —
`citizen-app/src/features/dang-nhap/hop-dong.ts:9-11`, hằng đường dẫn ở `:47`.

**Dòng thứ ba phải viết ra, vì suy rộng là cách hỏng tiếp theo.** *"Mọi thứ dính chữ Zalo đều
rời đi"* là một câu dễ nói và sai: ZNS gửi cho công dân về hồ sơ của chính họ đi từ OA của xã,
bằng khoá của xã, về nghiệp vụ của xã. `service-comms` **vẫn là chỗ sở hữu adapter** —
`kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md:108`. Sự phân biệt ấy đã nằm sẵn
trong tên kênh: `service-comms/internal/domain/thong_bao_gui_cong_dan.go:35-43` đặt tên
`zalo-zns` chứ không phải `zalo`, chính vì lý do này.

## Lập luận cũ sai ở chỗ nào

Comment biện hộ cho vị trí cũ nằm ở `service-platform/internal/http/webhook_zalo.go:27-30`
(tệp đã gỡ; đọc bằng `git show HEAD:service-platform/internal/http/webhook_zalo.go`). Nó dẫn
`kb/00-foundation/domain-boundaries.md`, chỗ định nghĩa `platform` = *"Nền tảng — nhà cung cấp
vận hành"*.

Câu ấy nghĩa là **nhà cung cấp vận hành NỀN TẢNG ViGov** — sổ đăng ký xã, vòng đời xã, siêu dữ
liệu. ADR 0003 khoá quyền của quản trị nhà cung cấp ở đúng mức siêu dữ liệu. Nó **không** có
nghĩa *"mọi thứ thuộc nhà cung cấp thì để vào đây"*.

**Trích dẫn ấy còn bị cắt mất nửa câu.** Dòng thật ở `kb/00-foundation/domain-boundaries.md:79`
là:

> `platform` | Nền tảng — nhà cung cấp vận hành, **chỉ siêu dữ liệu**

Ba chữ *"chỉ siêu dữ liệu"* — đúng cái ràng buộc chặn việc này — bị bỏ lại khi trích. Đó là
hình dạng đáng ghi nhớ hơn cả kết luận: ranh giới không mất nghĩa vì ai đó phản đối nó, mà vì
một lần trích thiếu đi qua review.

Đọc rộng như thế thì `platform` dần thành **sọt đựng** — nơi mọi thứ không rõ thuộc về đâu
được để tạm. Và không ai thấy ngày ranh giới mất nghĩa, vì mỗi lần thêm đều có vẻ hợp lý
riêng lẻ.

## Cái giá của việc để lại — số liệu từ manifest

`service-platform` là dịch vụ đắt nhất hệ thống để triển khai lại:

| Số liệu | Nguồn |
|---|---|
| `replicas: 3` | `deploy/base/platform/deployment.yaml:18` |
| `maxUnavailable: 0, maxSurge: 1` | `deploy/base/platform/deployment.yaml:24` |
| `minAvailable: 2` (PDB) | `deploy/base/platform/pdb.yaml:7` |

Lý do nằm ở chú giải đầu tệp, `deploy/base/platform/deployment.yaml:3-6`: nó **lên trước mọi
dịch vụ khác**, vì các dịch vụ còn lại quay số tới cổng **9090** của nó để phân giải `Host` →
xã. Platform chết thì `Host` không phân giải được, luật 1 bất biến 3 bắt trả **404** chứ không
được mặc định một xã — nghĩa là **mọi xã đều trả 404**.

Hệ quả: sửa cách xử lý **một cảnh báo của Zalo** sẽ phải lăn lại đúng dịch vụ ấy — tiêu lượt
triển khai đắt nhất hệ thống cho một việc **không thuộc xã nào**. Chi phí này không xuất hiện
trong ngày viết mã; nó xuất hiện mỗi lần sửa, mãi mãi.

## Ba hệ quả khác, mỗi cái đủ một mình

| # | Hệ quả | Vì sao đủ nặng |
|---|---|---|
| a | **App secret của một sản phẩm thương mại nằm trong k8s Secret của hệ thống nhà nước** | Luật 8: bí mật sống trong kho bí mật của hệ thống sở hữu nó. Trộn khoá thương mại vào kho của hệ thống nhà nước làm mờ ai chịu trách nhiệm khi phải xoay khoá, và ai được đọc |
| b | **Cảnh báo *"app không sống"* gửi nhầm người** | Cảnh báo ấy nói về bundle Mini App + backend `vihat-miniapp`. Người trực ViGov **không hành động được** trên cả hai. Cảnh báo gửi cho người không sửa được là tiếng ồn, và tiếng ồn là thứ làm người ta ngừng đọc cảnh báo thật |
| c | **Nới ranh giới service trong im lặng** | Luật 2 bất biến 1: mỗi thực thể có đúng một dịch vụ sở hữu, khai ở `kb/30-indexes/data-ownership.json`. Một bề mặt không thuộc nghiệp vụ nào của xã mà vẫn ở trong service xã là ranh giới đã nới mà không ai ghi |

## Vì sao gỡ BÂY GIỜ là rẻ nhất

Endpoint **chưa từng nhận một lời gọi nào**. Cả hai overlay chỉ có **hai path**, và **không
path nào trỏ tới `platform`**:

| Overlay | Path | Backend |
|---|---|---|
| `deploy/overlays/prod/ingress.yaml:43-48` | `/api/v1` | `identity` |
| `deploy/overlays/prod/ingress.yaml:49-54` | `/` | `web-admin` |
| `deploy/overlays/staging/ingress.yaml:19-24` | `/api/v1` | `identity` |
| `deploy/overlays/staging/ingress.yaml:25-30` | `/` | `web-admin` |

Nghĩa là: **không URL đang sống, không dữ liệu, không hàng đợi, không consumer.** Không có bước
di trú, không có khoảng hai đường cùng chạy, không có ai phải đổi cấu hình. Ranh giới sai được
sửa với chi phí gần bằng không — và đây là lần duy nhất còn đúng như thế. Ngày Zalo Developer
Console trỏ thật vào URL ấy, cùng việc này thành một thủ tục với bên thứ ba.

## Quan hệ hai kho — ngữ cảnh cho mọi phiên sau

| Kho | Là gì |
|---|---|
| `vihat-miniapp` | Nơi quản trị **app chính** (Mini App): app secret, webhook, đổi token đăng nhập, sinh QR |
| `vigov-v2/citizen-app` | **Mã giao diện chạy bên trong** app ấy |

**QR do `vihat-miniapp` sinh mang tham số, và tham số ấy nạp vào `citizen-app`.**

**RÀNG BUỘC đi kèm — đừng đọc câu trên mà bỏ câu này.** Tham số trên QR / deep link là **dữ
liệu do client cung cấp**. Nó **DẪN GIAO DIỆN, KHÔNG CẤP QUYỀN**:

- ADR 0005 §*Quyết định* tách ba lớp **Khám phá – Phiên – Uỷ quyền**: QR và deep link nằm ở lớp
  Khám phá, cột *"Tin được"* ghi **Không — chỉ gợi ý**
  (`kb/10-decisions/0005-miniapp-tenant-resolution.md:40-44`)
- ADR 0022 ghi thẳng: tham số `t=` trên deep link **"Không bao giờ chạm tới rìa"**
  (`kb/10-decisions/0022-ria-kenh-cong-dan.md:118`)
- Phép đối chiếu xã nằm **TRONG LUỒNG, không ở rìa**: ADR 0019 bất biến 8 — *"Xã của mã ghép
  phải khớp xã của phiên công dân"* (`kb/10-decisions/0019-qr-ghep-phien.md:78`), và phép kiểm
  ấy chạy ở bước 5 của luồng, trên server (`:61`)

Nói gọn: kho `vihat-miniapp` **sinh** được tham số, nhưng không vì thế mà tham số ấy **nói**
được xã. Xã vào phiên là hành động tường minh của công dân, và server phát hành lại phiên.

## Còn mở

Khi endpoint ở `vihat-miniapp` bắt đầu **XỬ LÝ** payload, nó cần kiểm chữ ký. **Khuôn chữ ký
thật của Zalo cho loại webhook này chưa ai tra.**

Nguồn phải là **Developer Console / tài liệu chính thức của Zalo**, không phải trí nhớ của
agent. Tiền lệ nằm ngay trong kho này: cả ADR 0018 đứng trên nguồn thứ cấp, và tự khai mức
chất lượng chứng cứ từng câu — có câu chỉ đạt **TRUNG BÌNH**, có câu **THẤP**
(`kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md:35-40`). Một endpoint hành động
trên payload mà không kiểm chữ ký là một đường ghi không xác thực; ở đây nó không còn nằm
trong hệ thống nhà nước, nhưng nó vẫn là đường dẫn tới phiên đăng nhập của công dân.

→ ADR 0031 (pháp nhân đứng tên Mini App): `kb/10-decisions/0031-chuyen-phap-nhan-mini-app-sang-vihat-group.md`
→ ADR 0018 (ZNS từ OA của xã, ở lại ViGov): `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`
→ ADR 0003 (nhà cung cấp chỉ siêu dữ liệu): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0005 · 0019 · 0022 (tham số dẫn giao diện, không cấp quyền)
→ Luật 2 (ranh giới service) · luật 8 (bí mật và khoá)
