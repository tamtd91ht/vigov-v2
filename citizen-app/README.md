# citizen-app

The citizen-facing **Zalo Mini App**. React + Vite. **`zmp-sdk` is imported from exactly one
directory** — `src/features/tinh-nang/`, the six real features — and from nowhere else. The
tripwire in `src/phase1-collects-nothing.test.ts` is what keeps that true: it was not removed when
those features landed, it was **narrowed to one directory**, and it has a case proving it still
fires everywhere outside it.

Citizens are identified by phone plus OTP — a **weak identity**, not to be trusted. And they
**do not choose** this software: being unable to use it means being unable to reach a public
service. That makes accessibility a rights question, not a preference.

## Two phases, ONE App ID

A Mini App is identified by its App ID, and an App ID is what Zalo reviews and what a
verifying Official Account is bound to. That single identifier is why this app ships in two
phases instead of two apps.

| Phase | What ships | Why |
|---|---|---|
| **1 — now** | A **real, working product app of VihatSoftware**: the company introduction plus three features that use `getPhoneNumber` · `getLocation` · `scanQRCode`. The **discovery layer** — commune suggestion, commune picker, commune page — is built and ships only in the `day-du` build | This is the submission Zalo reviews, so the OA `VihatSoftware` can verify the App ID **and** grant the three permissions. Zalo grants them only when the submission **visibly uses** them, and phase 2 needs all three on **this** App ID |
| **2 — next** | The commune / citizen surface behind a real session | It lands on the **same App ID**, already verified |

**No detail of a government body appears in the submitted build.** That is a requirement from the
user, not a preference: this app is published under the name of a technology company that sells
cloud contact-centre, CRM and AI solutions, and a product app that talks about "thủ tục" to
"công dân" is an app nobody can tell what it sells. It is **measured**, not promised — see
§"Hai biến thể bản dựng" and the forbidden-word sweep in `src/bundle-for-zalo.test.ts`.

(`xã hội` inside ViHAT Group's published vision sentence is an ordinary word and **stays**. The
sweep therefore lists whole administrative phrases, never the bare string `xã` — there is a test
for that too, because "clean it up by banning `xã`" would edit a published legal-entity statement
to fit a test.)

**The discovery layer is a secondary view of the SAME app, never a second app.** A second App ID
would need its own review, so "hiding the real app behind another App ID" buys nothing and costs
a second approval.

**Nothing of phase 2 gets deleted to make room for phase 1.** `src/lib/commune-resolution.ts`
is the foundation phase 2 builds on and stays untouched.

Why the verifying OA is VihatSoftware and not a commune, and why the notification OA is a
different OA per commune: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`. How the
commune gets resolved at runtime with no domain to key off:
`kb/10-decisions/0005-miniapp-tenant-resolution.md`. Both are read there, not repeated here.

### The app stores nothing and sends nothing — keep it that way

No sign-in, no OTP, no form, no input control of any kind, **no backend call, no device storage**.
The outbound links are `tel:`, `mailto:`, the company website, and a map page opened only when the
user taps "Chỉ đường".

Ba tính năng ở `src/features/tinh-nang/` gọi `getPhoneNumber` · `getLocation` · `scanQRCode`. Chúng
**lấy dữ liệu rồi hiện lên màn hình, hết**: không `fetch`, không `localStorage`, không
`console.log`, không gửi đi đâu. Hai lệnh cấm ấy trong `phase1-collects-nothing.test.ts` **không
được nới một dòng nào** — chúng là thứ biến "không gửi đi đâu" từ lời hứa thành ràng buộc kiểm
được. §"Ba tính năng thật" nói rõ vì sao hai trong ba màn ấy **không có dữ liệu cá nhân ngay từ
đầu**, và vì sao màn thứ ba thì có — của người khác.

**The discovery layer changes none of that**, in any build. It reads a parameter the platform
already put in the URL before our code ran, it holds the confirmed commune in `useState` — lost
when the app closes, never written to the device, never sent anywhere — and its commune list,
including the commune page and its duty numbers, is a constant in the bundle. Nothing about a
citizen is collected, and `src/phase1-collects-nothing.test.ts` sweeps the whole source tree to
keep it so.

The duty phone numbers on the commune page are the **agreed fake range `090000000x`** (rule 3,
invariant 5). The sweep allows that range and **nothing else**, and it has its own test proving
the exemption is that narrow: a real mobile number published in a reviewed app is a personal-data
incident that cannot be recalled.

The hotline and email on the contact screen are **ViHAT Group corporate contact points**
published on its website. They identify no individual, so they are business data, not personal
data. No individual's number belongs in this app.

## One app, every commune

A Mini App is identified by its platform App ID, not a domain, so the "tell communes apart by
domain" strategy does not apply here. The commune is resolved at runtime — see
`src/lib/commune-resolution.ts`.

## Non-negotiables

| # | Rule |
|---|---|
| 1 | GPS **suggests**, never **decides** |
| 2 | Once selected, the commune name appears on **every** screen |
| 3 | Switching commune is an explicit action, never automatic |
| 4 | Identity comes from the session; the backend never trusts a client-supplied identity |
| 5 | The commune is confirmed again at the final step before submitting |
| 6 | Body text ≥ 16px · touch targets ≥ 44×44px · contrast ≥ 4.5:1 · status never by colour alone |
| 7 | Error messages say what to do next, never an error code |
| 8 | Nothing personal in logs, URLs, or file names |

Rules 6, 7 and 8 bind phase 1. **Rules 1, 2 and 3 are live in the `day-du` build** (they have
nothing to bind in `goc`, which knows no commune), in `src/features/kham-pha/`. Rules 4 and 5 wait
for a server — the confirmed commune is client-side UI state, **not a session**; read the block at
the top of `src/features/kham-pha/GoiYXaScreen.tsx` before building anything on top of it.

## Error message shape

| Wrong | Right |
|---|---|
| `Error 422: Validation failed` | "Số điện thoại chưa đúng. Nhập 10 số, bắt đầu bằng 0." |
| `Unauthorized` | "Phiên đăng nhập đã hết. Đăng nhập lại để tiếp tục." |

## Where the content lives

Every user-visible string of the company introduction sits in `src/content/company-profile.ts`.
One file, because phase 2 replaces this content wholesale and the edit should land in one place.
The discovery layer (`src/features/kham-pha/`) keeps its own strings next to the rules they
belong to, and every commune name it shows in **exactly one file** — see §Còn thiếu #0. The three
features keep theirs in `src/features/tinh-nang/noi-dung.ts`, for a third reason: those are the
exact sentences the Zalo reviewer reads when deciding whether to grant the permissions, and a
justification scattered through JSX is one nobody re-reads before submitting.

**Nothing may be added to `company-profile.ts` without a source.** The app carries the name of a
real legal entity: an unsourced founding year, customer name, price, efficiency figure or award is
a false statement published under that name. Facts that are missing are left out, never filled in.

## Hai biến thể bản dựng

Người chạy lệnh chọn **đẩy bản nào**. Biến thể quyết định ở **tầng dựng**, qua biến môi trường
`VIGOV_BIEN_THE` (`vite.config.ts`).

| Biến thể | Nội dung | Dùng để | `dist/assets/app.js` |
|---|---|---|---|
| **`goc`** | Ứng dụng sản phẩm đầy đủ: bốn màn giới thiệu + tab Danh thiếp (ba tính năng) + hai tính năng trên màn Liên hệ | **BẢN NỘP** | **566,48 kB** thô · 156,10 kB gzip |
| **`day-du`** (mặc định) | `goc` + lớp khám phá + danh mục xã mẫu + trang xã + bảng chẩn đoán | Thử nghiệm nội bộ, demo | **578,03 kB** thô · 159,16 kB gzip |

Hai con số ấy **đo ngày 18/09/2026**, bằng `npm run build:goc` và `npm run build`, đọc từ chính
tệp phát ra. Gần trọn 536 kB của bản `goc` là **`zmp-sdk`** (≈264 kB thô / ≈66 kB gzip): app không
có SDK từng nặng 254,39 kB. Đó là cái giá của việc chín quyền nay là sáu tính năng thật, và nó được
trả một lần cho cả ứng dụng.

**Phần tăng so với bản ba tính năng (536,30 kB): +30,18 kB thô / +8,43 kB gzip.** Trong đó bộ mã
hoá QR chiếm **10,23 kB thô / 3,74 kB gzip** — đo bằng cách dựng lại bản `goc` với `encode` của
`uqr` thay bằng một hàm giả trả về ma trận cố định (556,25 kB thô / 152,36 kB gzip), rồi lấy
hiệu. Phần còn lại (≈19,95 kB thô) là mã của ba tính năng, câu chữ tiếng Việt của chúng, và
những nhánh của `zmp-sdk` mà sáu lời gọi mới kéo vào — thân hàm `openMediaPicker`,
`downloadFile`, `keepScreen` trước đây bị tree-shaking loại đi vì không ai gọi tới.

**Biến thể `quyen` không còn.** Nó từng tồn tại vì ba màn quyền là một lớp trình diễn thêm vào một
app giới thiệu tĩnh — gỡ được, và "bản nộp tối thiểu" thì gỡ nó đi. Nay ba quyền ấy thuộc về chính
ứng dụng sản phẩm, nên `quyen` trùng hoàn toàn với `goc`, và hai biến thể nói cùng một thứ là hai
biến thể sẽ lệch nhau. `bien-the.test.ts` có một ca khẳng định cái tên ấy đã biến mất khỏi **cả
hai** tệp giữ danh sách, và `bundle-for-zalo.test.ts` có một ca khẳng định `VIGOV_BIEN_THE=quyen`
nay **ném lỗi** thay vì dựng ra một bản không ai còn định nghĩa.

**Vì sao tách bằng BUILD chứ không bằng một cờ lúc chạy.** Một cờ lúc chạy để tám tên đơn vị
hành chính **đặt ra**, tám số điện thoại mẫu và bảng chẩn đoán nằm nguyên trong bundle gửi
duyệt — chỉ là không vẽ ra. Tách ở tầng dựng thì bản `goc` **thật sự không chứa** chúng, và
điều đó **kiểm được bằng `grep` trên `dist/assets/app.js`**, không phải bằng lời hứa.

### Grep thật trên bản `goc`, đo 18/09/2026

```
cơ quan: 0 · công dân: 0 · chính quyền: 0 · hành chính: 0 · thủ tục: 0 · Chọn xã: 0 · Đổi xã: 0
xã hội: 1 (câu tầm nhìn đã công bố — phải còn)

chín tên API đều CÓ MẶT: getPhoneNumber 7 · getLocation 7 · scanQRCode 6 · getNetworkType 6 ·
keepScreen 9 · vibrate 7 · requestCameraPermission 5 · openMediaPicker 6 · downloadFile 6

serverUploadUrl: 2 — CẢ HAI là của chính `zmp-sdk` (lược đồ zod của `openMediaPicker`, và thân
hàm đọc `e.serverUploadUrl`). Mã của ta đóng góp 0. Phép đo đúng là "không tệp nào GÁN một
chuỗi cho tham số ấy": 0 lần, ở cả hai biến thể. Xem `bundle-for-zalo.test.ts`.
```

Hai nhãn `Chọn xã` · `Đổi xã` từng **nằm lại** trong bản `goc`: `App.tsx` là vỏ chung, không nằm
sau alias, nên chuỗi viết thẳng trong nó đi vào bundle gửi duyệt kể cả khi nhánh vẽ chúng không
bao giờ chạy. Nay chúng đọc từ `NHAN_KHAM_PHA` sau cửa `bien-the/kham-pha`, và bản rỗng trả về
chuỗi rỗng. Đó là lý do bảng trên đọc `0`.

Cơ chế là `resolve.alias`, không phải tree-shaking — tree-shaking **không** loại được một
`import` tĩnh đã có mặt trong mã. Hai cái tên `bien-the/kham-pha` và `bien-the/chan-doan` là
**hai cửa duy nhất** vào hai phần gỡ được; bản `goc` thì cửa trỏ sang `index.rong.ts`.

| Tệp | Việc nó làm |
|---|---|
| `src/features/kham-pha/index.ts` · `index.rong.ts` | Bề mặt lớp khám phá, và bản rỗng của nó — kể cả hai nhãn của vỏ |
| `src/features/diagnostics/index.ts` · `index.rong.ts` | Như trên, cho bảng chẩn đoán |
| `src/features/kham-pha/bien-the.test.ts` | Hai bản rỗng khai đúng bề mặt · **không tệp nào nhập thẳng vòng qua alias** · danh sách biến thể ở `vite.config.ts` và `scripts/dung.mjs` **không lệch nhau** |
| `src/bundle-for-zalo.test.ts` | Dựng thật **cả hai** biến thể rồi đọc bundle — bằng chứng cuối cùng, kèm lượt quét từ cấm |

`tsc` luôn nhìn bản **đầy đủ** (`tsconfig.json` → `paths`); bản rỗng khai kiểu bằng `typeof`
của bản thật, nên thiếu một export là `tsc --noEmit` đỏ chứ không phải bản `goc` vỡ lúc dựng.

## Sáu tính năng thật — và chín quyền chúng cần

Zalo **chỉ cấp** một quyền khi bản nộp **có chỗ dùng nó nhìn thấy được**, và chính sách
Mini App điều 3.3.4 (trích ngay trên `getPhoneNumber` trong
`node_modules/zmp-sdk/index.d.ts`) nói thẳng: *"chúng tôi sẽ từ chối xét duyệt cho những Mini App
có luồng xin cấp quyền chưa rõ ràng, không nêu được mục đích xin quyền đến người dùng"*.

**TÍNH NĂNG, không phải MÀN QUYỀN, và khác biệt ấy là cả vấn đề.** Một tab tên "Quyền" nói
với người duyệt rằng đây là app đi xin quyền; sáu tính năng đặt đúng chỗ người ta cần
chúng nói rằng đây là app có việc để làm. **Sáu tính năng, chín quyền** — hai con số không
bằng nhau vì một việc người dùng làm có thể cần hai quyền: chìa danh thiếp ra cho người
khác quét cần cả `keepScreen` lẫn `downloadFile`. Tách chúng thành hai "tính năng" để con số
đẹp lên là dựng hai cái nút không ai hiểu để làm gì — đúng thứ điều 3.3.4 từ chối.

| Tính năng | Ở đâu | API | Chạy được tới đâu |
|---|---|---|---|
| **Quét danh thiếp số** | Tab "Danh thiếp" | `scanQRCode` | **Trọn vẹn.** Quét → bóc tách vCard → thẻ có cấu trúc → nút Gọi (`openPhone`) · Gửi email (`mailto:`) · Mở liên kết (`openWebview`) · Quét mã khác |
| **Tìm văn phòng gần bạn** | Tab "Liên hệ" | `getLocation` | **Một nửa.** Nhận được token; ba văn phòng thật và nút "Chỉ đường" (`openWebview` → bản đồ) chạy ngay. **Không xếp được theo khoảng cách** — xem ranh giới dưới |
| **Đăng ký nhận tư vấn** | Tab "Liên hệ" | `getPhoneNumber` | **Một nửa.** Nhận được token; **không có đường gửi nó đi đâu**. Màn hình nói thẳng điều đó rồi đưa ngay hotline và email — hai đường chạy được bây giờ |
| **Kiểm tra đường truyền** | Tab "Giải pháp" | `getNetworkType` + `vibrate` | **Trọn vẹn.** Tổng đài đám mây chạy trên chính đường mạng của máy, nên một nhà cung cấp VoIP có lý do thật để hỏi. Bốn kiểu kết nối → nhãn tiếng Việt + một câu đúng sự thật. **Không một con số nào** — xem ranh giới dưới |
| **Danh thiếp số của chúng tôi** | Tab "Danh thiếp" | `keepScreen` + `downloadFile` | **Trọn vẹn.** Mã QR chứa vCard dựng từ `COMPANY`/`CONTACT` → giữ màn sáng khi người khác quét (**tắt lại khi rời màn**) → tải `.vcf` bằng `fileBase64Data`, **không máy chủ nào** |
| **Số hoá danh thiếp giấy** | Tab "Danh thiếp" | `requestCameraPermission` + `openMediaPicker` | **Một nửa.** Xin quyền → chọn ảnh → hiện ảnh. **Chưa đọc được chữ trên ảnh** (cần OCR ở máy chủ) — màn hình nói thẳng. Ảnh **không rời khỏi máy** |

### SỰ THẬT ĐÃ ĐO TỪ `zmp-sdk` 2.53.0 — đừng tra lại tài liệu web

```
GetPhoneNumberReturns = { number?: @deprecated; token?: string }
GetLocationReturns    = { latitude?/longitude?/timestamp?/provider?: @deprecated; token?: string }
ScanQRCodeReturns     = { content: string }
openPhone(args: { phoneNumber: string }): Promise<void>            // index.d.ts:4141, @zaloOnly
openWebview(args: { url: string; config?: {…} }): Promise<void>    // index.d.ts:4491, @zaloOnly

getNetworkType()               -> { networkType: "none"|"wifi"|"cellular"|"unknown" }  // :1226 · :3232
vibrate({ type?, milliseconds? })        -> Promise<void>   // :4400–4433  (milliseconds CHỈ Android)
keepScreen({ keepScreenOn: boolean })    -> Promise<void>   // :4201–4231  (khai void — KHÔNG đọc .success)
requestCameraPermission()      -> { userAllow: boolean; message: string }              // :1293 · :4002
openMediaPicker({ type, serverUploadUrl?, … }) -> { data: string[] | string }          // :1403 · :4804
downloadFile({ url?, fileBase64Data? })  -> Promise<void>   // :6060–6116
```

Token của cả hai: **dùng được một lần, hết hạn sau 2 phút**, và chỉ đổi được ở **máy chủ** bằng
app secret.

**Hệ quả thiết kế: số điện thoại và toạ độ KHÔNG BAO GIỜ tới thiết bị.** Chỉ có token. Hai màn ấy
không có gì để che vì chúng **không có dữ liệu cá nhân ngay từ đầu** — và màn hình **nói ra điều
đó bằng tiếng Việt**, vì đó là lý do đáng tin nhất để một người bấm đồng ý. `number` / `latitude`
/ `longitude` đều `@deprecated`: đọc chúng là tự rước dữ liệu cá nhân về máy đúng lúc nền tảng
vừa bỏ đường ấy đi.

**`serverUploadUrl` BỊ BỎ ĐI, và đó là quyết định quan trọng nhất của ba tính năng thêm vào.**
`index.d.ts` dòng 4721 ghi rõ: *"Tham số serverUploadUrl không còn bắt buộc. Mặc định nếu không
truyền, SDK sẽ trả về đường dẫn tạm thời (local cache path) của media mà không tự động upload
lên server"*. Có nó thì ảnh của người dùng đi lên một máy chủ — **không qua `fetch`, nên mọi
dây bẫy còn lại đều không thấy gì**. Nên nó có một dây bẫy riêng trong
`phase1-collects-nothing.test.ts`, cấm ở **mọi tệp**, không miễn cho cả `src/features/tinh-nang/`
— vì đó là thư mục duy nhất nó lọt được, nên miễn cho nó là không cấm gì cả.

### RANH GIỚI — nói ra, không giả vờ vượt qua

| Ranh giới | Vì sao không vượt được | Màn hình làm gì thay thế |
|---|---|---|
| **Không xếp được văn phòng theo khoảng cách** | `getLocation` chỉ trả token; đổi token cần một bước máy chủ có app secret. `navigator.geolocation` thì dây bẫy cấm, và là một quyền khác | Hiện đủ ba văn phòng thật kèm nút "Chỉ đường", và **một câu tiếng Việt nói rõ danh sách chưa sắp theo khoảng cách** |
| **Không gửi được yêu cầu tư vấn** | Chỉ có token, và dây bẫy cấm `fetch` | Nói thẳng rằng bản này chưa gửi gì, rồi đưa hotline và email **ngay dưới nút** |
| **`openPhone` / `openWebview` chỉ chạy trong Zalo** | `@zaloOnly` trong chính `index.d.ts` | Một câu tiếng Việt nói mở lại trong Zalo, không mã lỗi |
| **Không đọc được chữ trên ảnh danh thiếp giấy** | Bóc tách cần OCR, OCR cần một bước máy chủ, và dây bẫy cấm mọi đường gửi ra | Hiện ảnh lên màn hình và **nói thẳng bằng một câu** rằng bản này chưa đọc được chữ. **Không một dòng mã nào giả vờ đang nhận dạng** |
| **Không đo được tốc độ hay độ trễ đường truyền** | `getNetworkType` trả đúng MỘT chuỗi: kiểu kết nối. SDK không có phép đo nào khác | Nói kiểu kết nối và một câu đúng về bản chất của nó. **Cấm mọi con số ms / Mbps / điểm chất lượng** — có một ca test quét chính những câu ấy |
| **Không chọn được chỗ lưu và tên tệp `.vcf`** | `downloadFile` không có tham số tên tệp, và `fileBase64Data` **không có một dòng tài liệu nào** trong `index.d.ts` | Một câu nói rằng Zalo là bên quyết định tệp nằm ở đâu. ⚠ **CHƯA THỬ TRÊN MÁY THẬT** — xem §"Còn thiếu" |

Không có một dòng mã nào tính khoảng cách, và không một câu chữ nào hứa một việc bản dựng này
không làm. Đó là điều kiện để chính sách quyền riêng tư **mô tả đúng bản dựng nó nằm trong**.

`scanQRCode` là API **duy nhất** ở đây trả về dữ liệu thật. Nội dung một tấm danh thiếp là **dữ
liệu cá nhân của NGƯỜI KHÁC** (luật 3). **Hiện lên màn hình được; `console.log` thì không**, và
không chỗ lưu nào. Bộ bóc tách (`danh-thiep.ts`) là hàm **thuần** — chuỗi vào, đối tượng ra,
không tác dụng phụ — nên nó không có chỗ nào để rò rỉ, và kiểm được đầy đủ mà không cần điện thoại.

### Ràng buộc kỹ thuật đã trả giá để biết

| Điều | Hệ quả trong mã |
|---|---|
| `zmp-sdk` **đụng `window` ngay lúc nạp mô-đun** | Không `import` tĩnh ở cấp cao nhất — nó làm sập mọi test chạy dưới Node. Phải `await import("zmp-sdk")` **bên trong hàm**, trong `try/catch` |
| Ngoài Zalo thì lời nhập ấy hỏng | Màn hình **nói ra bằng tiếng Việt**, không trắng trơn. `catch {}` im lặng ở đây là một màn trống trên máy người duyệt |
| Người dùng **từ chối** (`code === -201`, lấy từ ví dụ trong chính `index.d.ts`) | Là **đường đi bình thường**, không phải lỗi: một câu tiếng Việt nói họ bấm lại được, không mã lỗi, không màu đỏ. Và **từ chối không làm mất tính năng**: ba văn phòng, hai đường liên hệ vẫn còn nguyên |
| Ở môi trường phát triển, hai API token **luôn thành công và trả token rỗng** | Màn hình nói ra đúng như vậy thay vì hiện một ô trống |

| Tệp | Việc nó làm |
|---|---|
| `src/features/tinh-nang/zalo-api.ts` | **Nơi duy nhất** nhắc `zmp-sdk`. Mười một lời gọi, quy mọi đường về bốn nhánh: `xong` · `tu-choi` · `ngoai-zalo` · `khong-lay-duoc` |
| `src/features/tinh-nang/vcard.ts` | Bộ **sinh** vCard của chính chúng tôi + mã hoá base64 UTF-8. **Thuần** — mặt đối xứng của `danh-thiep.ts` |
| `src/features/tinh-nang/MaQR.tsx` | Vẽ mã QR bằng SVG nội tuyến từ ma trận `uqr`. Một `<path>`, không phải nghìn `<rect>` |
| `src/features/tinh-nang/giu-man-sang.ts` | Hợp đồng bật/tắt `keepScreen`, tách ra để kiểm được "**luôn tắt khi rời màn**" mà không cần DOM |
| `src/features/tinh-nang/danh-thiep.ts` | Bộ bóc tách vCard / liên kết / văn bản. **Thuần**, không tác dụng phụ |
| `src/features/tinh-nang/noi-dung.ts` | Mọi chữ người dùng đọc trên sáu tính năng, và ánh xạ `networkType` sang nhãn tiếng Việt — `bundle-for-zalo.test.ts` dùng lại đúng danh sách này |
| `src/features/tinh-nang/khung.tsx` | Khung chung: tiêu đề · lý do · nút · chỗ hiện kết quả. `KhungTinhNang` là bản **thuần** để bốn nhánh kết quả kiểm được mà không cần Zalo |
| `src/features/tinh-nang/ManDanhThiep.tsx` · `LienHeTinhNang.tsx` · `KiemTraDuongTruyen.tsx` · `ManThiepCuaChungToi.tsx` · `SoHoaThiepGiay.tsx` | Tab Danh thiếp (ba khối), hai khối trên màn Liên hệ, một khối trên màn Giải pháp |
| `src/features/tinh-nang/vcard.test.ts` · `tinh-nang-them.test.tsx` | 52 ca: bộ **sinh** vCard (đủ trường · thiếu trường · ký tự thoát · tiếng Việt · base64 giải ngược đúng nguyên văn · đọc ngược bằng chính bộ bóc tách), bốn kiểu mạng + một giá trị lạ, `keepScreen` **tắt lại khi rời màn**, ba ranh giới được nói ra |
| `src/features/tinh-nang/tinh-nang.test.tsx` | 51 ca: **bộ bóc tách vCard trước hết** (đủ trường · thiếu trường · tham số · dòng gập · ký tự thoát · URL · văn bản · rỗng · rác), rồi từ chối · ngoài Zalo · token không hiện trọn · ranh giới được nói ra · từ chối không làm mất tính năng |

## Phần nhìn — "sống động" làm bằng gì

Thuần **CSS + SVG nội tuyến**. Không thư viện hoạt hoạ, không phông ngoài, không ảnh: bundle tự
chứa, và một Mini App tải tài nguyên từ bên thứ ba là một câu hỏi ở vòng duyệt.

**MỘT ngoại lệ, và nó không phải về phần nhìn: `uqr`** (MIT, không phụ thuộc gì khác, **nằm
trong bundle**, không CDN). Sinh mã QR đúng gồm chọn chế độ mã hoá, chọn phiên bản, chèn khối
sửa lỗi Reed–Solomon rồi chọn mặt nạ. Sai **một bit** là một mã trông hoàn hảo trên màn hình và
không máy nào quét nổi — mắt không bắt được, và một phép kiểm dựng bằng chính bộ mã hoá sai
ấy cũng không. Giá: **10,23 kB thô / 3,74 kB gzip**, đo bằng hiệu hai lần dựng — §"Hai biến
thể bản dựng". Ứng dụng chỉ nhập `encode`; ba hàm vẽ sẵn của thư viện bị tree-shaking loại
đi, và SVG do `MaQR.tsx` tự dựng để màu đọc từ biến CSS — tức cặp màu của mã QR **đi qua ca
tương phản** như mọi cặp màu khác.

| Thứ chuyển động | Làm bằng |
|---|---|
| Nền chòm sao sau các tấm nền tối | `transform` trôi chậm (`drift`) |
| Thẻ và nút hiện lên khi vào màn, so le | `opacity` + `translate3d` (`hien-len`, bốn bậc trễ) |
| Huy hiệu tròn của ba tính năng | `background-position` trôi trên dải màu thương hiệu |

**Chỉ `transform` và `opacity`.** Cả hai chạy trên trình tổng hợp, nên một máy Android yếu không
phải vẽ lại cả trang mỗi khung hình. Bất cứ thứ gì buộc bố cục lại (`height`, `top`, `margin`)
không được phép ở đây.

**`prefers-reduced-motion` tắt sạch mọi chuyển động**, và điều đó là một ràng buộc kiểm được:
`accessibility.test.ts` quét toàn bộ biểu mẫu kiểu và **đỏ lên nếu có một `animation:` nào khai
ngoài khối `@media (prefers-reduced-motion: no-preference)`**. Chuyển động ở đây là trang trí; với
một người rối loạn tiền đình hay đau nửa đầu thì không phải, và điện thoại của họ đã trả lời sẵn.

Mọi màu mới đều có **tên trong danh sách cặp màu** của `accessibility.test.ts`. Một màu đẹp mà
trượt 4,5:1 thì **đổi màu, không nới ngưỡng**.

## Chính sách quyền riêng tư

Xin chín quyền mà không có văn bản này thì vòng duyệt trả về. Nội dung nằm ở
`src/content/chinh-sach-rieng-tu.ts`, vẽ ở **cuối màn Liên hệ** (không phải một tab riêng — năm
tab đang nói năm việc, và người tìm thông tin pháp lý đã đứng sẵn ở màn ấy).

**Quy tắc chi phối toàn bộ tệp ấy: chính sách phải mô tả ĐÚNG bản dựng nó nằm trong.** Văn bản
này công bố dưới tên một pháp nhân có thật — một câu mô tả hành vi mà mã không có là tuyên bố
sai dưới tên ấy; giấu một hành vi mà mã CÓ là vi phạm chính Nghị định 13/2023/NĐ-CP.

**Phiên bản `1.2`, hiệu lực 18/09/2026.**

**Lên `1.2` vì bề mặt quyền riêng tư MỞ RỘNG THẬT SỰ**, không vì câu chữ — bốn thứ mới:

| Mới | Ứng dụng làm gì | Khai ở mục |
|---|---|---|
| **Máy ảnh** | Hỏi quyền để chụp lại một tấm danh thiếp giấy | `cac-quyen` · `tung-quyen` |
| **Thư viện ảnh** | Mở cửa sổ chọn ảnh của Zalo; ảnh **không rời khỏi máy** | `du-lieu` · `cac-quyen` |
| **Thông tin mạng** | Chỉ **kiểu** kết nối — không IP, không tên mạng Wi-Fi | `cac-quyen` · `tung-quyen` |
| **Ghi một tệp xuống máy** | Tệp danh thiếp **của chúng tôi**, không phải dữ liệu của người dùng | `ghi-tep`, mục MỚI |

Ba thứ đầu là dữ liệu ĐI VÀO ứng dụng; thứ tư là thứ ứng dụng VIẾT RA — một loại hành vi
bản 1.1 hoàn toàn không có. Nó được cho một mục riêng chứ không phải một câu lẫn trong mục
khác: người đọc để biết "ứng dụng này làm gì với máy tôi" phải tìm thấy nó bằng một dòng
tiêu đề, không phải bằng cách đọc hết.

**MỘT CÂU CỦA BẢN 1.1 ĐÃ THÀNH SAI VÀ PHẢI SỬA**: *"không đọc thư viện ảnh"*. Nay ứng dụng
mở cửa sổ chọn ảnh, nên câu ấy được thay bằng *"không **tự** đọc thư viện ảnh; chỉ nhận đúng
tấm ảnh bạn tự chọn trong cửa sổ của Zalo"*. Một câu đúng ở bản trước mà không ai sửa khi hành
vi đổi là loại lỗi nặng nhất một văn bản như thế này có thể mắc — và là loại không có gì đỏ
lên để báo.

**Câu thứ tư được thêm vào danh sách "đúng vì có dây bẫy"**: *"ảnh không rời khỏi máy"* ← dây
bẫy cấm `serverUploadUrl` trên toàn cây mã, không miễn cho thư mục nào.

#### Lịch sử số phiên bản

**Phiên bản `1.1`, hiệu lực 18/09/2026.** Lên số vì **hành vi** đã đổi, không vì câu chữ: ba
quyền nay gắn với ba tính năng sản phẩm, và mục "Chuyển dữ liệu cho bên thứ ba" nay nói ra việc
ứng dụng mở trang bản đồ và trang web khi người dùng bấm. Hai thay đổi ấy phải đi kèm một số
phiên bản mới, nếu không thì "phiên bản 1.0" chỉ tên hai văn bản khác nhau.

**Mục về các quyền nay nằm trong danh sách chung**, không còn sau một cửa alias: ba quyền có mặt ở
**mọi** biến thể, nên cơ chế `MUC_TRUOC_QUYEN`/`MUC_SAU_QUYEN` đã thành thừa và bị bỏ. Giữ lại một
cơ chế tách đôi khi không còn gì để tách là giữ lại một cái bẫy.

Bốn câu trong chính sách đúng **vì `phase1-collects-nothing.test.ts` cấm điều ngược lại**, không
phải vì ai hứa: "không gửi đi đâu" ← dây bẫy cấm `fetch`/XHR/WebSocket/EventSource/axios ·
"không lưu lại" ← dây bẫy cấm `localStorage`/`sessionStorage`/`cookie`/`indexedDB` · "ảnh không
rời khỏi máy" ← dây bẫy cấm `serverUploadUrl`. **Ai nới một trong ba dây bẫy ấy phải sửa chính
sách TRƯỚC** — nếu không, chính sách thành sai mà không có gì đỏ lên.

Số mục **không viết cứng vào tiêu đề**: React đánh số lúc vẽ. Một con số viết cứng sẽ lệch ngay
lần thêm hoặc bớt một mục — lệch trong một văn bản pháp lý, im lặng.

### Còn thiếu — hai câu hỏi cho khách hàng, và một phép thử phải chạy trên máy thật

| Thiếu | Vì sao chưa điền |
|---|---|
| **Mã số thuế**, **người đại diện theo pháp luật** | Không có nguồn. `content/company-profile.ts` chỉ chứa thứ đã công bố trên vihatsoftware.com và vihatgroup.com. Bịa hai trường này trong một văn bản pháp lý là thứ không sửa lại được sau khi nộp |
| **URL trang chính sách** | Developer Console còn một ô URL ngoài bản trong app. Chưa biết đăng ở đâu, nên chưa dựng bộ sinh trang tĩnh — dựng cho một đích chưa biết là đoán. Khi chốt, trang ấy phải sinh ra TỪ `chinh-sach-rieng-tu.ts`, không chép tay, để trang đăng và app không lệch nhau |
| **Tên và chỗ lưu của tệp `.vcf`** | `downloadFile` không có tham số tên tệp, và `fileBase64Data` **không có một dòng tài liệu nào** trong `index.d.ts` — bảng định dạng được hỗ trợ ở đó chỉ nói về đường `url`, và **không liệt kê `.vcf`**. Chưa thử được trên máy thật trong phiên này. Nút vẫn ghi "(.vcf)" vì **nội dung** đúng là vCard; cái chưa biết là Zalo đặt tên tệp ra sao. **Phải mở bằng Zalo trên một máy thật rồi bấm nút ấy một lần** trước khi nộp |

## Nộp lên Zalo

### Chuỗi lệnh

```bash
npm run zmp:login             # một lần, cần App ID
npm run zmp:deploy            # bản ĐẦY ĐỦ, bản thử nghiệm (-t)
npm run zmp:deploy:goc        # bản GỐC,    bản thử nghiệm (-t)
npm run zmp:phat-hanh:goc     # bản GỐC,    BẢN PHÁT HÀNH (bỏ -t) — đây là bản đem duyệt
```

Cả bốn đi qua `scripts/deploy.mjs`: dựng đúng biến thể → `sync-config` → `deploy`. Dựng nằm
**trong** script vì biến thể quyết định lúc dựng — dựng ngoài rồi đẩy trong là hai lệnh có thể
lệch nhau, và lần lệch ấy nộp bản `day-du` dưới nhãn `goc`.

Thêm `--thu` vào bất kỳ lệnh nào để **in ra rồi dừng**, không dựng và không đẩy gì cả.

`zmp:deploy` gộp ba bước vì **bỏ sót bước giữa là nộp một app trắng trơn**. Zalo không dùng
`index.html` của chúng ta: nó tự dựng vỏ rồi nạp đúng những tệp khai trong `app-config.json`,
và `zmp-cli sync-config` là thứ điền danh sách ấy từ trang đã dựng. Dựng xong mà quên đồng bộ
thì `app-config.json` trỏ vào bản dựng của lần trước.

**Không hỏi câu nào.** CLI vốn dừng ba lần — *"This is not a ZMP Project?"*, *"where is your
dist folder"*, *"description"* — và cả ba đã tắt bằng `-e`, `-o dist`, `-m`, cộng `-p`.

Mô tả phiên bản **sinh theo từng lần đẩy**, không cố định: `<biến thể> · <sha ngắn> · <ngày giờ>`, cộng
`dirty` khi cây làm việc còn thay đổi chưa commit. Một nhãn cố định thì mọi bản trong console
Zalo trông như nhau và lúc cần biết *"bản đang chạy là bản nào"* thì không còn gì để tra; còn
`dirty` nói ra rằng bản ấy **không ứng với commit nào**. Tính trong `scripts/deploy.mjs` chứ không
trong `package.json`, vì trên Windows `npm run` chạy qua `cmd`, nơi `$(git rev-parse …)` chỉ là
một chuỗi ký tự.

`-t` là **bản thử nghiệm**. Bỏ `-t` là đẩy **bản phát hành**, và đường ấy có trong script
(`zmp:phat-hanh:goc`). Script **in ra biến thể, loại bản và nhãn phiên bản trước khi làm gì**, rồi
đường phát hành **dừng 5 giây** để người chạy kịp `Ctrl-C`. Đừng bỏ phần in ra và phần đếm ngược —
đó là cái phanh còn lại.

CLI gọi qua `npx --yes zmp-cli@4.0.3`, không phải devDependency: nó kéo theo hơn hai trăm gói,
nhiều gói đã ngừng hỗ trợ, và không có lý do gì để chúng nằm trong một kho sắp gửi ra ngoài.
Phiên bản ghim cứng để lần chạy sau ra đúng kết quả lần chạy trước.

### Hai thứ đã kiểm bằng cách chạy thật, đừng đi kiểm lại

| Đã biết | Bằng chứng |
|---|---|
| Thư mục nộp là `dist/`, không phải `www/` | `zmp deploy --help` ghi *"Default www"*, nên `-o dist` là bắt buộc và đã nằm trong `zmp:deploy` |
| Thẻ script phải **cổ điển**, không `type="module"` | Chạy `sync-config` trên hai bản HTML khác nhau đúng một chỗ: bản module cho `listSyncJS: ["inline.js"]` — **thiếu chính bundle của app**; bản cổ điển cho thêm `"./assets/app.js"`. Không có lỗi nào báo ra. `vite.config.ts` sửa thẻ ở bước phát HTML, và `src/bundle-for-zalo.test.ts` ghim lại |
| Khuôn link mở **bản thử nghiệm**, và tham số riêng **đi tới được app** | `https://zalo.me/s/<APP_ID>/?env=TESTING&version=<n>` — và nối thêm `t`, `src`, `debug` thì chúng tới nơi, đứng cạnh `env`/`version`. Đo 18/09/2026 trên Version 6–7. Đường công khai **không kèm `env`/`version`** chỉ phục vụ bản đã phát hành: mở khi chưa phát hành thì Zalo trả *"ứng dụng đang trong giai đoạn phát triển"* **trước khi** mã của ta chạy — đó là rào nền tảng, không phải app trắng |
| `h5.zdn.vn/zapps/…` là host **nội bộ**, không quét được | Đó là thứ webview nạp **sau khi** Zalo phân giải deep link. Quét thẳng nó thì Zalo báo *"liên kết không được hỗ trợ"*; mở trong trình duyệt thường thì báo *"vui lòng truy cập trên ứng dụng Zalo"*. Nó chỉ hữu ích như một phép đo: `location.href` in ra nó là cách rẻ nhất để biết khuôn link |
| `location.search` mang **đúng** những gì `getRouteParams()` mang | Đo 18/09/2026 trên Version 6, mở nguội qua deep link: cả tham số nền tảng (`env`, `version`) lẫn tham số riêng (`t`, `src`, `debug`) đều có mặt. Nên lớp khám phá đọc tham số **chỉ từ `location.search`**, không qua SDK — kể cả khi SDK đã có mặt trong bundle. Chế độ hỏng nếu phép đo sai ở một đường mở nào đó là chế độ hỏng **lành**: không có tham số ⇒ app sản phẩm bình thường ⇒ đường ADR 0005 bắt buộc phải chạy được vẫn chạy |

### Còn thiếu

| # | Cái gì | Ghi chú |
|---|---|---|
| 0 | **Xoá danh mục xã mẫu** — `src/features/kham-pha/demo-danh-muc-xa.ts` — trước khi kênh công dân phục vụ người thật | Tám tên đơn vị hành chính **đặt ra**, kèm nội dung riêng của từng xã. Số trực dùng **dải giả đã thoả thuận `090000000x`** (luật 3, bất biến 5) — `kham-pha.test.tsx` ghim dải ấy, `phase1-collects-nothing.test.ts` quét toàn cây mã và **chỉ** miễn đúng dải ấy. Nguồn thật là `ListTenants` của service `platform` — đã khai trong proto, **chưa có cài đặt** |
| 1 | Ảnh chụp màn hình và mô tả trên store | Bắt buộc để duyệt. Icon thì đã có — `tools/logo.py` dựng từ `brand/lg_vhs_full.svg` |
| 2 | Các khoá còn lại trong `app-config.json` | `app.*` viết từ nguồn thứ cấp và **chưa đối chiếu** với Developer Console. Ba khoá `list*` thì đã do `sync-config` sinh, không phải đoán |

## Lớp khám phá — cái gì đang chạy, và cái gì còn thiếu

Lớp KHÁM PHÁ của ADR 0005 **đã dựng xong và nằm trong biến thể `day-du`** — không nằm trong bản
`goc` gửi duyệt. Ba lớp của ADR 0005 vẫn tách rời, và đây là bảng phải đọc trước khi sửa bất cứ
thứ gì trong `src/features/kham-pha/`:

| Lớp | Trả lời | Nguồn | Tin được? | Trạng thái |
|---|---|---|---|---|
| **Khám phá** | Công dân MUỐN làm việc với xã nào | `t` + `src` trên đường liên kết · danh mục | **Không — chỉ là gợi ý** | **Đang chạy** |
| **Phiên** | Phiên này ĐANG thao tác ở xã nào | Máy chủ ghi sau khi công dân xác nhận | Có | Chưa có máy chủ |
| **Uỷ quyền** | Công dân này được đọc/ghi gì ở đó | Quan hệ công dân↔xã + luật 4 | Có | Chưa có máy chủ |

| Tệp | Việc nó làm |
|---|---|
| `src/features/kham-pha/goi-y.ts` | Mức tin theo nguồn, hàm thuần, có test |
| `src/features/kham-pha/GoiYXaScreen.tsx` | Màn xác nhận: tên xã to, một chạm đồng ý |
| `src/features/kham-pha/ChonXaScreen.tsx` | Danh mục xã, mỗi dòng một nút ≥44px, **không ô nhập** |
| `src/features/kham-pha/TrangXaScreen.tsx` | Trang của xã đã chọn: giới thiệu · số trực · giờ làm việc · dịch vụ |
| `src/features/kham-pha/demo-danh-muc-xa.ts` | Danh mục **tạm**, và nội dung riêng của từng xã — §Còn thiếu #0 |
| `src/features/kham-pha/kham-pha.test.tsx` | 50 ca: mức tin, fail-closed, nội dung riêng từng xã, tên xã trên mọi màn hình |
| `src/lib/launch-params.ts` · `src/features/diagnostics/` | Đọc `location.search`, và bảng đo chỉ hiện khi có `debug` |

**Trang xã — mỗi xã một nội dung riêng.** Xác nhận xã xong thì **tab đầu trở thành trang của xã
ấy**. Không thêm tab và không che thanh tab — phần giới thiệu và ba tính năng, thứ Zalo đã duyệt,
luôn còn đường tới.

**Mức tin theo nguồn** — đây là quy tắc, không phải giao diện:

| `src` | Hành vi |
|---|---|
| `qr` · `zns` | Chọn sẵn, một chạm xác nhận |
| `share` · không khai · tra mã không ra | **Luôn** bắt chọn tường minh — liên kết chuyển tay không nói lên ý định người nhận |

**HAI THỨ PHẢI ĐỔI KHI CÓ MÁY CHỦ, và cả hai đều không phải việc sửa giao diện:**

1. Danh mục xã mẫu (`demo-danh-muc-xa.ts`) thay bằng **`ListTenants`** của service `platform` —
   đã khai trong `proto/vigov/platform/v1/platform.proto`, **chưa có cài đặt**.
1b. **Danh sách dịch vụ của xã là NGHIỆP VỤ, không phải giao diện.** Ở giai đoạn 2 danh sách ấy
   đọc **lúc chạy** từ cấu hình của từng xã (luật 1, bất biến 10), **không bao giờ** là hằng số
   trong mã. Hôm nay nó là hằng số **chỉ vì** đó là dữ liệu trình diễn.
2. Xã đã xác nhận hôm nay là **trạng thái giao diện phía client**, không phải phiên: sống trong
   `useState`, mất khi app đóng, không lưu xuống máy, không gửi đi đâu, **không cấp quyền gì**.
   Thay bằng xã đọc từ phiên — **không phải "đồng bộ thêm" với nó**.

`src/lib/commune-resolution.ts` vẫn **chưa được nối vào** — nó là bộ khung của giai đoạn 2.

## Lệnh

| Lệnh | Việc |
|---|---|
| `npm run dev` | Máy chủ phát triển Vite (biến thể **đầy đủ**) |
| `npm run build` | Gói tĩnh vào `dist/` — biến thể **đầy đủ** |
| `npm run build:goc` | Như trên, biến thể **gốc** (BẢN NỘP) |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest — sự thật đã công bố, hình dạng bundle, sổ màn hình, bộ bóc tách vCard, ba tính năng, **và hai biến thể đúng bằng thứ người duyệt đọc** |
| `npm run zmp:sync` | Dựng (đầy đủ) rồi đồng bộ `app-config.json` theo trang đã dựng |
| `npm run zmp:deploy` | Đầy đủ → bản thử nghiệm |
| `npm run zmp:deploy:goc` | Gốc → bản thử nghiệm |
| `npm run zmp:phat-hanh:goc` | Gốc → **bản phát hành**, có in ra và đếm ngược 5 giây |

→ Skills: `.claude/skills/zalo-miniapp-multi-tenant` · `.claude/skills/accessibility-elderly`
