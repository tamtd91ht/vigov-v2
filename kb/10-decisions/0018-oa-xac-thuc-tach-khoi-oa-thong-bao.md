---
id: 0018-oa-xac-thuc-tach-khoi-oa-thong-bao
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d952aaf
expires: null
owns_facts:
  - "ràng buộc một Mini App chỉ gắn được một OA xác thực, và hình dạng kênh thông báo sau ràng buộc đó"
  - "tách vai trò OA xác thực Mini App khỏi vai trò OA gửi thông báo cho dân"
  - "pháp nhân đứng tên Mini App và tên OA xác thực"
  - "kết quả tra tài liệu Zalo ngày 17/09/2026 kèm mức chất lượng chứng cứ từng câu"
---

# 0018. OA xác thực Mini App tách khỏi OA gửi thông báo

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Thay thế ADR 0006**

## Bối cảnh

ADR 0006 chốt hình dạng kênh thông báo, và tự đính kèm một khối *"điều kiện tiên quyết CHƯA
kiểm chứng"*: nó giả định **một Mini App làm việc được với nhiều OA**, và ghi rõ rằng nếu giả
định ấy sai thì quay lại viết **ADR thay thế, không sửa tệp đó**.

Ngày 17/09/2026 đã tra. **Giả định sai.** Đây là ADR thay thế.

`kb/10-decisions/0006-per-commune-zalo-oa.md` giữ nguyên từng chữ, kể cả dòng trạng thái — vì
tầng T1 không sửa ADR, và vì khối điều kiện tiên quyết trong đó chính là thứ đã khiến việc tra
tài liệu xảy ra trước khi có dòng mã nào. Xoá nó đi là xoá bằng chứng rằng cơ chế ấy hoạt động.

## Kết quả tra tài liệu — 2026-09-17

Bốn câu đầu là bốn câu ADR 0006 liệt kê. Câu 5 phát sinh trong lúc tra và là câu quyết định
liệu mục tiêu của 0006 còn đạt được hay không.

| # | Câu hỏi | Kết quả | Chứng cứ |
|---|---|---|---|
| 1 | Một Mini App gắn được nhiều OA không? | **KHÔNG.** Mỗi Mini App chỉ được xác thực bởi **đúng một** OA. Chiều ngược lại thì được: một OA xác thực được nhiều Mini App. Kèm ràng buộc thủ tục: Mini App và OA phải có *"liên kết chặt chẽ về mặt pháp lý"*, và người gửi yêu cầu xác thực phải là **Admin của cả hai** | **TRUNG BÌNH** |
| 2 | Tham số deep link có tới app trong mọi đường mở không? | Khuôn `https://zalo.me/s/<app_id>/?k=v` xác nhận là khuôn đúng. Đường mở từ danh sách app ghim hoặc từ tìm kiếm trong Zalo thì đương nhiên **không** mang tham số. **Chưa xác nhận:** cách đọc tham số bên trong app, và việc mở lại app đang chạy nền có giữ tham số hay không | **THẤP** |
| 3 | QR do Zalo sinh hay ta tự sinh? | **Ta tự sinh được** — QR chỉ là mã hoá của URL deep link. Ngoài ra `scanQRCode` của `zmp-sdk` (từ **2.5.3**) cho phép chính Mini App mở trình quét của Zalo và nhận nội dung QR thô: `const { content } = await scanQRCode({})` | **CAO** |
| 4 | `getPhoneNumber` có bị ràng buộc theo OA không? | **KHÔNG.** Luồng: `getAccessToken()` + `getPhoneNumber()` ở client → gửi token về backend → backend đổi tại `https://graph.zalo.me/v2.0/me/info` kèm **secret key của Mini App**. Không đụng tới OA. Số trả về là số điện thoại đăng ký tài khoản Zalo | **TRUNG BÌNH – CAO** |
| 5 | ZNS gửi được từ OA của từng xã không, dù OA đó **không** gắn Mini App? | **ĐƯỢC.** ZNS / ZBS Template Message gửi **theo số điện thoại**; người dân **không cần** theo dõi OA, và không đòi OA đó phải là OA đã gắn Mini App. Chỉ việc tích hợp ZNS *vào bên trong* Mini App mới đòi Mini App đã xác thực | **TRUNG BÌNH** |

### Nguồn và vì sao mức chứng cứ chỉ tới đó

| # | Nguồn | Vì sao không cao hơn |
|---|---|---|
| 1 | Bốn nguồn thứ cấp độc lập, nội dung nhất quán: `cnv.vn` · `miniapp.vn` · `pandaloyalty.com` · `academy.abaha.vn` | Trang tài liệu chính thức của Zalo render bằng JS nên không đọc được trực tiếp. **Phải xác nhận lại với Zalo lúc nộp hồ sơ đăng ký** |
| 2 | Khuôn deep link: tài liệu chính thức. Phần còn lại: không có nguồn | Hành vi runtime — **phải thử trên máy thật**, không tra ra được |
| 3 | `docs.zaloplatforms.com/docs/MA/api/device/qr/scanQRCode` | Tài liệu chính thức, đọc được, có chữ ký hàm |
| 4 | Tài liệu chính thức có mục này nhưng không render được; nhiều bài trong diễn đàn chính thức của Zalo mô tả **trùng khớp** | Chưa đọc được nguyên văn tài liệu |
| 5 | Tài liệu OA của Zalo (`oa.zalo.me`) + blog chính thức `miniapp.zaloplatforms.com` | Nguồn chính thức nhưng là văn bản tiếp thị/hướng dẫn, không phải đặc tả API |

Đây đúng là thứ ADR 0006 đòi phải có **trước dòng mã đầu tiên** của kênh thông báo. Ghi lại tại
đây để lần sau không phải tra lại, và để biết chỗ nào còn phải kiểm.

## Tiền đề sai ở đâu — và vì sao mục tiêu vẫn đạt

ADR 0006 hỏng ở **một chỗ khác chỗ nó tưởng**. Nó gộp hai vai trò rất khác nhau của một OA vào
một khái niệm duy nhất là *"OA của xã"*:

| Vai trò | Zalo ràng buộc gì | Người dân thấy gì |
|---|---|---|
| **Xác thực Mini App** | Đúng **một** OA cho một Mini App | Huy hiệu đã xác thực, nút theo dõi OA trong app |
| **Gửi thông báo tới một số điện thoại** | Không ràng buộc theo Mini App (câu 5) | Tên người gửi trên tin nhận được về hồ sơ của mình |

Vai trò thứ nhất **đúng là bị chặn** ở con số một. Vai trò thứ hai **không bị chặn gì cả** — và
vai trò thứ hai mới là thứ ADR 0006 thật sự muốn: uy tín của cơ quan với chính người dân của
mình nằm ở **tên người gửi trên tin nhắn**, không nằm ở app nào xác thực bởi ai.

## Quyết định

**Tách hai vai trò đó ra, mỗi vai trò một phạm vi.**

| Vai trò của OA | Phạm vi | Vì sao |
|---|---|---|
| **OA xác thực Mini App** | **MỘT**, ở tầng nền tảng | Zalo chỉ cho một. Mọi bề mặt OA bên trong app — `followOA`, `openChat`, huy hiệu đã xác thực — chỉ trỏ tới OA này |
| **OA gửi thông báo cho công dân** | **Theo từng xã** — giữ nguyên quyết định của ADR 0006 | ZNS gửi theo số điện thoại, từ OA của chính xã; người gửi hiện ra vẫn là *"UBND xã X"*. Đây mới là thứ người dân nhìn thấy khi nhận tin về hồ sơ của mình |

Một câu để nhớ: **app có một OA, tin nhắn có nhiều OA.**

### Pháp nhân đứng tên — chốt 17/09/2026

**VHS đứng tên Mini App, và OA `VihatSoftware` là OA xác thực Mini App.**

Tức **nhà cung cấp** đứng tên, không phải một cơ quan nhà nước. Đây là lời giải cho ràng buộc
thủ tục ở câu 1 của bảng chứng cứ: Zalo đòi Mini App và OA xác thực có liên kết pháp lý chặt
chẽ và chung một Admin, mà một Mini App phục vụ 200+ xã thì **không xã nào là chủ sở hữu tự
nhiên** của nó.

| Hệ quả | |
|---|---|
| **Huy hiệu "đã xác thực" trong Mini App hiện tên doanh nghiệp**, không phải tên cơ quan nhà nước | Đây là bề mặt người dân nhìn thấy, nên nó thuộc phạm vi luật 10 — không phải chi tiết đăng ký |
| **Thông báo về hồ sơ vẫn đến từ OA của chính xã** qua ZNS, nên người gửi mà người dân thấy lúc nhận tin vẫn là *"UBND xã X"* | Chính vì vậy việc tách hai vai trò ở trên **giữ được mục tiêu của ADR 0006**: chỗ đắt hơn là tin nhắn, và chỗ đó không mất |
| **Bước nộp hồ sơ đăng ký hết bị chặn** | Điều kiện phải giữ: Admin của Mini App và Admin của OA `VihatSoftware` là **cùng một người thuộc VHS** |

Cái mất và cái giữ được nằm ở hai chỗ khác nhau, và không bù trừ cho nhau một cách hoàn hảo:
mất ở huy hiệu trong app, giữ được ở người gửi trên tin nhắn. Người thiết kế giao diện phải
biết điều này trước khi vẽ màn hình đầu tiên, chứ không phát hiện ra lúc đã chạy.

## Hệ quả

**Sáu hệ quả kiến trúc của ADR 0006 giữ nguyên toàn bộ cho kênh thông báo** — adapter, khoá
theo từng xã trong kho bí mật, xã chưa cấu hình phải suy giảm nhìn thấy được, nhất quán cuối
cùng với ghi nghiệp vụ, thông điệp mang mã nghiệp vụ chứ không mang dữ liệu cá nhân, và bước
thêm vào quy trình onboard. Nội dung đầy đủ của sáu mục ấy nằm ở
`kb/10-decisions/0006-per-commune-zalo-oa.md` §*Hệ quả kiến trúc* — đọc ở đó, không chép sang
đây. `service-comms` vẫn là chỗ sở hữu adapter.

Thay đổi so với 0006 gồm đúng ba điểm:

| # | Thay đổi |
|---|---|
| 1 | Hệ quả 6 đổi nội dung: onboard một xã nay là **"đăng ký OA + duyệt template ZNS"**, không phải "gắn OA vào Mini App". Việc gắn OA vào Mini App xảy ra **một lần cho cả nền tảng**, không phải một lần mỗi xã |
| 2 | Thêm một cấu hình ở tầng nền tảng — OA xác thực — nằm **ngoài** cấu hình theo xã. Nó không phải của xã nào, nên không đi qua đường cấu hình theo xã (luật 1 bất biến 10) |
| 3 | Mọi bề mặt OA hiển thị **bên trong** Mini App trỏ về OA nền tảng, **không** trỏ về OA của xã. Giao diện phải nói rõ điều này, nếu không người dân bấm "theo dõi" và tưởng mình đã theo dõi xã của mình |

**Điểm 3 là chỗ đắt nhất và nó là một câu hỏi giao diện, không phải câu hỏi kỹ thuật.** Người
dân thấy đúng một OA trong app nhưng nhận tin từ OA của xã: hai cái tên khác nhau trong cùng
một hành trình. Phải thiết kế để điều đó không trông giống lừa đảo.

## `getPhoneNumber` không đi qua OA

Câu 4 của bảng trên có một hệ quả đáng ghi thành mục riêng: **lấy được số điện thoại của công
dân không phụ thuộc vào việc xã đã có OA hay chưa.** Luồng đổi token chạy bằng secret key của
Mini App tại `graph.zalo.me`, không chạm tới OA nào.

Nghĩa là một xã mới onboard **đăng nhập được ngay**, kể cả trước khi OA của xã được duyệt. Thứ
duy nhất chưa chạy là kênh gửi tin ra — đúng cái đã được xử lý bằng hệ quả 3 và 4 của ADR 0006
(suy giảm nhìn thấy được, không chặn ghi nghiệp vụ).

Quyết định dùng luồng này làm đường đăng nhập, và cái giá của nó, nằm ở ADR 0020 —
`kb/10-decisions/0020-xac-thuc-so-dien-thoai-cong-dan.md`.

## CÒN MỞ — khách phải chốt, tuyệt đối không tự chọn hộ

| # | Câu hỏi | Vì sao không phải việc của người viết mã |
|---|---|---|
| 1 | **Chi phí ZNS tính theo từng tin — ai trả, và trả theo xã hay tập trung?** | Quyết định thương mại. Nó còn kéo theo một câu kỹ thuật: hạn mức và cảnh báo vượt hạn mức đặt ở mức xã hay mức nền tảng |
| 2 | **Xác nhận lại ràng buộc 1 Mini App ↔ 1 OA với chính Zalo lúc nộp hồ sơ** | Câu 1 của bảng chứng cứ mới ở mức **nguồn thứ cấp**. Cả ADR này đứng trên nó. Nếu Zalo trả lời khác, quay lại đây và viết ADR mới |

Hai câu trên không được quyết trong lúc viết mã.

*Câu thứ ba của mục này — pháp nhân nào đứng tên Mini App và OA xác thực — đã được trả lời
trong cùng ngày, xem §Pháp nhân đứng tên ở trên.*

## ĐIỀU KIỆN DỪNG

1. Có người đề xuất **một Mini App cho mỗi xã** để lách ràng buộc 1↔1 — đó là quay lại đúng
   hướng ADR 0005 đã loại, và nhân nó lên 200 lần
2. Có nhu cầu **hiển thị OA của xã bên trong Mini App** (nút theo dõi, mở chat với xã) — Zalo
   chỉ cho một OA ở bề mặt đó, nên đây là câu hỏi giao diện phải hỏi khách
3. Câu 2 của bảng chứng cứ (**hành vi tham số khi mở lại app**) chạm tới thiết kế nào đó — chưa
   thử máy thật thì chưa được coi là biết
4. Có đề xuất **chuyển quyền sở hữu Mini App hoặc OA xác thực sang một pháp nhân khác** — đó là
   thủ tục với Zalo, không phải thay đổi cấu hình, và nó động tới huy hiệu người dân đang thấy

→ ADR 0006 (quyết định gốc, nội dung sáu hệ quả): `kb/10-decisions/0006-per-commune-zalo-oa.md`
→ ADR 0005 (một Mini App duy nhất, khuôn deep link, mức tin theo nguồn): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0019 (QR ghép phiên — dùng `scanQRCode` xác nhận ở câu 3): `kb/10-decisions/0019-qr-ghep-phien.md`
→ ADR 0020 (đường đăng nhập dựng trên câu 4): `kb/10-decisions/0020-xac-thuc-so-dien-thoai-cong-dan.md`
→ ADR 0009 (bí mật theo từng xã — nơi khoá OA của xã nằm): `kb/10-decisions/0009-per-tenant-secret-encryption.md`
→ Luật 1 (cấu hình theo xã đọc lúc chạy): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 8 (bí mật không nằm trong nguồn, không nằm trong tài liệu): `.claude/rules/critical/8-secrets-config.md`
→ Luật 10 (bề mặt người dân nhìn thấy): `.claude/rules/critical/10-citizen-commitment.md`
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md`
