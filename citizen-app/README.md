# citizen-app

The citizen-facing **Zalo Mini App**. React + Vite. **`zmp-sdk` is imported from exactly one
directory** — `src/features/tinh-nang/`, the six real features — and from nowhere else. The
tripwire in `src/phase1-collects-nothing.test.ts` is what keeps that true: it was not removed when
those features landed, it was **narrowed to one directory**, and it has a case proving it still
fires everywhere outside it.

Citizens are identified by their Zalo phone number — one tap, **no OTP** (ADR 0020) — which is
still a **weak identity**, not to be trusted: Zalo asserts that the number belongs to that Zalo
account, never that the person holding the phone owns the number. And they
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

### The app stores nothing on the device, and sends exactly ONE thing — keep it that way

**No OTP, no form, no input control of any kind, no device storage.** The outbound links are
`tel:`, `mailto:`, the company website, and a map page opened only when the user taps "Chỉ đường".

**Sign-in exists, it is one tap** (`getPhoneNumber` + `getAccessToken`, ADR 0020) — never a
six-digit code to type — **and it calls a server, in both builds including the submitted one**.
That call is the ONE outbound request this repository allows: one route, from ONE file
(`src/features/dang-nhap/goi-may-chu.ts`), enforced by the tripwire in
`src/phase1-collects-nothing.test.ts`.

What crosses the wire is **two single-use Zalo codes, never a phone number** — the number cannot
reach the device at all (`GetPhoneNumberReturns` has only `token`). The server —
**`vihat-miniapp`, VihatSoftware's own backend, a separate repository, not ViGov** — exchanges
them and stores the phone number as the login name. The session ticket it returns lives in
`useState`: never written to the device, never drawn on screen, gone when the app closes.

⚠ The privacy policy says all of that, in both builds, in one text — §"Chính sách quyền riêng
tư". **It no longer claims the app sends nothing, because that claim became false**, and the
sentence was removed in the same commit that made it false.

Sáu tính năng ở `src/features/tinh-nang/` **lấy dữ liệu rồi hiện lên màn hình, hết**: không
`localStorage`, không `console.log`, không một lời gọi mạng nào — lời gọi duy nhất của cả kho
nằm ở `features/dang-nhap/goi-may-chu.ts`. Lệnh cấm lưu trữ trong `phase1-collects-nothing.test.ts`
**không được nới một dòng nào**; lệnh cấm `fetch` thì được **thu hẹp về đúng MỘT tệp**, và có
một ca cho nó ăn một `fetch` đặt ở tệp NGAY CẠNH để chứng minh ngoại lệ ấy hẹp đúng bằng một
tệp. §"Sáu tính năng thật" nói rõ vì sao hai trong các màn ấy **không có dữ liệu cá nhân ngay từ
đầu**, và vì sao màn quét mã thì có — của người khác.

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
| **`goc`** | Ứng dụng sản phẩm đầy đủ: bốn màn giới thiệu + tab Danh thiếp (ba tính năng) + hai khối trên màn Liên hệ (đăng nhập · tìm văn phòng) | **BẢN NỘP** | **578,41 kB** thô · 159,79 kB gzip |
| **`day-du`** (mặc định) | `goc` + lớp khám phá + danh mục xã mẫu + trang xã + bảng chẩn đoán | Thử nghiệm nội bộ, demo | **589,96 kB** thô · 162,91 kB gzip |

Hai con số ấy **đo ngày 20/09/2026**, bằng `npm run build:goc` và `npm run build`, đọc từ chính
tệp phát ra. Phần tăng so với lần đo 18/09 (566,48 / 578,03) là khối đăng nhập cộng các điều
khoản mới của chính sách: **+11,93 kB** ở bản nộp, **+11,93 kB** ở bản đầy đủ — hai bản tăng
bằng nhau, vì **khối đăng nhập và chính sách giống hệt nhau ở cả hai**. Phần lớn số ấy là CHỮ:
một văn bản pháp lý khai đủ thì dài, và đó là cái giá rẻ nhất trong toàn bộ bảng này.

**KHỐI ĐĂNG NHẬP KHÔNG PHẢI MỘT BIẾN THỂ, VÀ NÓ TỪNG LÀ.** Bản đầu đặt lời gọi máy chủ sau một
cửa `bien-the/dang-nhap` để bản nộp không gọi mạng. Tiền đề ấy đảo chiều trong cùng ngày: **bản
nộp gọi máy chủ thật**, vì một nút đăng nhập bấm là được thuyết phục vòng duyệt hơn hẳn một nút
nói "bản này chưa nối máy chủ" (điều 3.3.4). Cửa ấy bị **gỡ hẳn** — giữ một cơ chế tách đôi khi
hai nửa nói y hệt nhau là giữ lại đúng cái bẫy biến thể `quyen` đã để lại một lần.

**`VIGOV_API_HOST` — biến lúc dựng thứ hai, và CẢ HAI bản đều cần.** Địa chỉ máy chủ của khối
đăng nhập, đọc trong `vite.config.ts` (`define`), chỉ `features/dang-nhap/` đọc tới. Không khai
thì **fail closed**: không một lời gọi nào được phát đi, và màn hình nói ra rằng bản dựng chưa
được khai địa chỉ. Không có địa chỉ mặc định — đoán một địa chỉ là gửi hai mã đăng nhập của một
người thật tới một máy chủ không ai chọn.

⚠ **Quên biến ấy = nộp một nút đăng nhập không đăng nhập nổi.** Nên `scripts/deploy.mjs` **chặn
đường đẩy** khi nó rỗng, và in địa chỉ ra trước khi làm gì. Chặn ở đó chứ không ném lỗi lúc
dựng, vì `npm test` dựng cả hai biến thể trong mọi lần chạy và phải chạy được trên máy chưa có
địa chỉ nào.

⚠ **Không một bí mật nào đi qua biến ấy.** `define` chèn giá trị thẳng vào bundle — tức vào tệp
tải về máy người dùng (luật 8, bất biến 4). Khoá bí mật của Mini App chỉ nằm ở backend (ADR
0020, bất biến 2). Gần trọn 536 kB của bản `goc` là **`zmp-sdk`** (≈264 kB thô / ≈66 kB gzip): app không
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
| `src/features/dang-nhap/PhatHanhPhien.tsx` | **Bước máy chủ** của khối đăng nhập — có mặt ở CẢ HAI bản dựng, không còn cửa biến thể nào |
| `src/features/dang-nhap/hop-dong.ts` | **Hợp đồng với máy chủ, một tệp** — đường dẫn, tên hai trường gửi đi, hình dạng phản hồi, và `VIGOV_API_HOST`. Máy chủ là kho riêng `vihat-miniapp`, **đang dựng song song**: đổi hợp đồng là sửa tệp này và `dang-nhap.test.tsx` nằm cạnh, không sửa gì khác |
| `src/features/dang-nhap/goi-may-chu.ts` | **Tệp DUY NHẤT trong kho được `fetch`.** Năm nhánh kết quả, không ném ra ngoài, không log |
| `src/features/dang-nhap/dang-nhap.test.tsx` | 11 ca: năm nhánh của bước máy chủ · 401 và 502 KHÔNG được gộp · gọi đúng một lần bằng POST · thân yêu cầu mang đúng hai mã · bearer không ra màn hình |
| `src/content/chinh-sach.test.ts` | 22 ca về chính văn bản pháp lý: câu "không gửi đi đâu" đã biến mất · mục Đăng nhập nói đủ **gửi gì · ai nhận · lưu gì · vì sao** · thời gian lưu nói đủ **không có hạn tự động · cửa yêu cầu xoá · phạm vi xoá** · nhật ký khai đủ **IP · thời điểm · kết quả · mã lý do · chỉ-ghi-thêm** · **lượt THẤT BẠI cũng bị ghi** · danh sách **KHÔNG lưu** · **90 ngày là TRẦN (dọn theo lô tuần, 83–90), áp cả dòng của lượt thất bại**, kèm ca canh chiều ngược nếu ai viết lại thành "đúng 90 ngày" · **dòng bằng chứng của một lần xoá** khai đủ bốn vế · và MỘT số phiên bản, vì chưa bản nào tới tay ai |
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
| **Đăng nhập bằng số Zalo** | Tab "Liên hệ" | `getPhoneNumber` + `getAccessToken` | **Trọn vẹn, ở CẢ HAI bản.** Một chạm, **không OTP, không ô nhập sáu số** (ADR 0020) → hai mã → máy chủ `vihat-miniapp` đổi mã và phát hành phiên → phiên giữ trong `useState`, không ghi xuống máy. ⚠ **Chưa gọi thử trên máy chủ thật** — backend đang dựng song song. Hotline và email nằm ngay dưới nút: đường lui cho người từ chối quyền |
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
| **Không đổi được mã Zalo ngay trên máy** | Đổi mã cần **khoá bí mật của Mini App**, và khoá ấy chỉ nằm ở máy chủ (ADR 0020, bất biến 2 · luật 8 cấm #5). Đưa nó xuống thiết bị là điều kiện dừng #1 của chính ADR ấy | Gửi hai mã tới máy chủ `vihat-miniapp` và để nó đổi. Ứng dụng không bao giờ thấy số điện thoại — chỉ thấy mã, và một phiếu phiên trả về |
| **Phiếu phiên là TẠM, không phải "đã đăng nhập vĩnh viễn"** | ADR 0005: sau khi công dân chọn xã thì máy chủ **phát hành lại** phiên | Phiên sống trong `useState` và mất khi đóng app. Không một dòng mã nào dựng trên giả định giữ mãi — xem khối chú thích đầu `PhatHanhPhien.tsx` |
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
| `src/features/tinh-nang/zalo-api.ts` | **Nơi duy nhất** nhắc `zmp-sdk`. Mười hai lời gọi (thêm `getAccessToken` của khối đăng nhập), quy mọi đường về bốn nhánh: `xong` · `tu-choi` · `ngoai-zalo` · `khong-lay-duoc` |
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

**Phiên bản `1.0`, hiệu lực 20/09/2026.**

⚠ **MỘT SỐ PHIÊN BẢN, KHÔNG PHẢI SÁU — và đây là quyết định dựa trên bằng chứng, không phải dọn
cho gọn.** Trong một ngày soạn thảo, văn bản này đã đi `1.0 → 1.1 → 1.2 → 1.3 → 1.4 → 1.5`, mỗi
lần vì một lý do đúng. Nhưng lý do tồn tại của việc lên số là *"một người đã bấm đồng ý ở bản cũ
không biết về thứ mới"*, và nó chỉ có nghĩa khi **có** một người như thế.

Đã kiểm, 20/09/2026:

| Nguồn | Kết quả |
|---|---|
| `git log -- citizen-app` | không một commit nào nói tới một lần phát hành |
| Sổ tiến độ, mục `nop-zalo-duyet` | `chua_lam` |
| README §"Còn thiếu" #1 | ảnh chụp màn hình và mô tả store **chưa có** — Zalo bắt buộc phải có mới xét duyệt được |
| README §"Hai thứ đã kiểm bằng cách chạy thật" | đường công khai trả *"ứng dụng đang trong giai đoạn phát triển"*, tức **chưa phát hành**; chỉ có bản THỬ NGHIỆM (`env=TESTING`, Version 6–7) mà chỉ tài khoản người dựng mở được |

**Không một người dùng nào từng đọc một bản nào của văn bản này.** Sáu số trong một ngày vì thế
không bảo vệ ai — chúng kể lại quá trình soạn thảo bên trong một văn bản pháp lý, và làm người
đọc tưởng đã có sáu đợt thay đổi được công bố. Nên bản đầu tiên ra ngoài mang số **`1.0`**.
Quá trình soạn thảo **không bị xoá**: nó nằm trong `git log` của `chinh-sach-rieng-tu.ts`, đúng
nơi lịch sử soạn thảo thuộc về.

⚠ **TỪ LẦN PHÁT HÀNH ĐẦU TIÊN TRỞ ĐI, QUY TẮC ĐẢO NGƯỢC:** mỗi thay đổi về **hành vi xử lý dữ
liệu** phải lên một số mới, kể cả khi cách nhau vài giờ, và **không được gộp**. Ba ví dụ thật từ
lượt soạn thảo này, giữ lại vì chúng nói rõ ranh giới hơn mọi định nghĩa:

| Thay đổi | Lên số? |
|---|---|
| `getPhoneNumber` đổi mục đích: "gọi lại tư vấn" → **định danh + thông báo ZNS** | **CÓ** — người đã đồng ý cho việc này chưa đồng ý cho việc kia |
| Khai thêm rằng máy chủ ghi **địa chỉ IP** mỗi lượt đăng nhập | **CÓ** — người đọc bản trước không biết |
| Sửa một câu cho dễ đọc, không đổi hành vi nào | KHÔNG |

### Văn bản nói gì — bốn điều nặng nhất

**Một văn bản, đúng cho cả hai biến thể.** Không còn câu *"không lưu trữ và không gửi đi bất kỳ
dữ liệu nào của bạn"*: nó thành sai ngày bản nộp bắt đầu gọi máy chủ, và bị gỡ trong đúng lượt
làm nó sai. Câu mở đầu nay nói ngay ba điều: không lưu gì xuống máy · gửi đi **đúng một việc** ·
và chỉ khi chính người dùng bấm đăng nhập.

**Văn bản được đối chiếu với LƯỢC ĐỒ THẬT** (`vihat-miniapp/migrations/0001_init.sql`), không
với một mô tả. Bốn thứ được khai nhờ lần đối chiếu ấy:

| Máy chủ thật sự lưu | Khai ở đâu |
|---|---|
| IP của **MỌI LƯỢT đăng nhập, kể cả lượt THẤT BẠI** — một đoạn riêng, in hoa | mục Đăng nhập |
| Kết quả từng lượt + mã lý do + mã định danh nội bộ nếu thành công | mục Đăng nhập |
| Thời điểm tạo/cập nhật bản ghi định danh · tạo/hết hạn từng phiên (**7 ngày**) | mục Đăng nhập · Cách xử lý |
| Bản băm SHA-256 của phiếu phiên — **có khai**, vì bỏ đúng một mục khỏi một danh sách đầy đủ là mời câu hỏi *"còn bỏ gì nữa"*, và vì nó là điều **tốt** nói được ra | mục Đăng nhập |

**Hai thời hạn, hai câu trả lời khác nhau, cả hai đã chốt:**

| Lưu gì | Bao lâu | Vì sao không giống nhau |
|---|---|---|
| Số điện thoại | **không có hạn tự động**, tới khi người dùng yêu cầu xoá | nó là danh tính — hết nó là hết tài khoản, nên chủ của nó quyết |
| Nhật ký đăng nhập | **chậm nhất 90 ngày** — thực tế 83–90, **kể cả dòng của lượt thất bại** | nó chứa IP của cả những người **chưa từng có tài khoản**: họ không có gì để yêu cầu xoá, nên một hạn tự động là cách duy nhất thứ ấy mất đi |
| Dòng bằng chứng của một lần xoá | **vô thời hạn** | một bằng chứng tự huỷ thì không còn là bằng chứng |

⚠ **"90 ngày" là TRẦN, không phải một cái mốc đúng ngày — và câu chữ nói ra đúng cơ chế chạy.**
Backend dọn theo **lô tuần**: `nhat_ky_don_qua_han()` DROP cả một phân mảnh tuần khi **đầu**
khoảng của nó quá hạn (`migrations/0002_…sql`, điều kiện `d <= nguong`), nên mỗi dòng sống **tối
đa 90 ngày, tối thiểu 83**. Bản đầu của hàm ấy DROP khi **đuôi** khoảng quá hạn — nghe như
"không xoá sớm của ai", nhưng đẩy dòng cũ nhất lên **97 ngày**, tức hệ thống **vượt qua chính
cái trần đã hứa** trong văn bản pháp lý. Chính sách từng viết *"sau 90 ngày, dòng nhật ký được
xoá"*: một câu mô tả cơ chế xoá-đúng-ngày **không hề tồn tại**, và sai về phía giữ lâu hơn. Nay
văn bản nói cả cận trên lẫn cận dưới, và có **ca canh chiều ngược** để một lượt "biên tập cho
gọn" không rút nó về câu cũ.

⚠ **Một bảng nữa được khai: dòng bằng chứng của một lần xoá** (`nhat_ky_an_danh`). Đây là dữ
liệu **duy nhất** về một người còn ở lại **sau khi** họ đã yêu cầu xoá — chỗ dễ im lặng nhất của
cả văn bản. Nó gồm mã định danh nội bộ · yêu cầu tới qua hotline hay email · người tiếp nhận ·
ghi chú · thời điểm; **không chứa số điện thoại** (lúc ghi thì số đã bị ghi đè trong cùng giao
dịch); **giữ vô thời hạn**, vì nó là thứ chứng minh chúng tôi đã làm điều đã hứa. Văn bản cũng
nói ra rằng yêu cầu xoá **thu hồi mọi phiên**, nên máy đang đăng nhập sẽ bị đăng xuất.

Cơ chế canh chỗ-trống (`THOI_GIAN_LUU_CHUA_CHOT`, rồi `THOI_HAN_LUU_NHAT_KY_CHUA_CHOT`) đã làm
đúng việc của nó **hai lần**: không ai bịa một con số vào văn bản pháp lý, và ngày khách chốt thì
ca kiểm đỏ lên bắt đi trọn bốn việc — câu thật · bỏ hằng · số phiên bản · sửa ca kiểm. Cả hai
hằng nay đã biến mất vì cả hai câu hỏi đã có đáp án.

⚠ **MỘT LỜI HỨA ĐÃ PHẢI RÚT LẠI, VÌ LƯỢC ĐỒ KHÔNG CHO GIỮ NÓ.** Một bản nháp cam kết *"khi xoá,
chúng tôi xoá số điện thoại **và bản ghi định danh**"*. Lược đồ: `nhat_ky_dang_nhap` tham chiếu
`nguoi_dung(id)` và là bảng **chỉ ghi thêm** (trigger chặn `UPDATE`/`DELETE`), nên PostgreSQL
**từ chối** xoá hàng định danh của bất cứ ai đã từng đăng nhập thành công. Văn bản nay hứa đúng
thứ làm được: **xoá số điện thoại** — thứ duy nhất nhận ra người dùng — và nói rõ bản ghi còn
lại chỉ là một mã không gắn với số nào. → §"Còn thiếu"

#### Ba bài học của lượt soạn thảo, giữ lại vì chúng không thuộc về một số phiên bản nào

**Một cơ chế tách đôi khi không còn gì để tách là một cái bẫy.** Đã mắc hai lần trong tệp này:
`MUC_TRUOC_QUYEN`/`MUC_SAU_QUYEN` (tách mục quyền theo biến thể, khi mọi biến thể đều xin quyền),
rồi một cửa `bien-the/dang-nhap` tách câu mở đầu chính sách (khi cả hai biến thể đều gọi máy chủ).
Cả hai lần, cách sửa là **gỡ cơ chế**, không phải nuôi hai nửa giống hệt nhau.

**Bốn câu trong chính sách đúng vì `phase1-collects-nothing.test.ts` cấm điều ngược lại**, không
phải vì ai hứa: "gửi đi đúng MỘT việc" ← dây bẫy cấm `fetch`/XHR/WebSocket/EventSource/axios ở
mọi tệp, miễn cho **đúng một tệp** · "không lưu lại" (trên máy) ← dây bẫy cấm
`localStorage`/`sessionStorage`/`cookie`/`indexedDB`, **không nới một dòng nào** · "ảnh không rời
khỏi máy" ← dây bẫy cấm `serverUploadUrl`, **không nới một dòng nào**. **Ai nới một trong ba dây
bẫy ấy phải sửa chính sách TRƯỚC** — nếu không, chính sách thành sai mà không có gì đỏ lên. Lần
thu hẹp 20/09 đã đi đúng thứ tự ấy: **gỡ câu "không gửi đi đâu"** trong cùng một lượt với việc
nới dây bẫy.

**Số mục không viết cứng vào tiêu đề**: React đánh số lúc vẽ. Một con số viết cứng sẽ lệch ngay
lần thêm hoặc bớt một mục — lệch trong một văn bản pháp lý, im lặng.

### Còn thiếu — hai việc vận hành, hai câu hỏi cho khách hàng, và hai phép thử phải chạy thật

| Thiếu | Vì sao chưa điền |
|---|---|
| ⚠ **LỊCH CHẠY HẰNG NGÀY CHO `nhat_ky_don_qua_han()`** | Trần 90 ngày chỉ đúng **nếu hàm dọn chạy hằng ngày**: khoảng cách giữa hai lần chạy **cộng thẳng** vào tuổi của dòng cũ nhất, nên cron hằng tuần biến trần 90 thành 97 — đúng cái vừa sửa. Kho backend **ghi rõ yêu cầu ấy** (`make don-nhat-ky`, README §cron) nhưng **không chứa một định nghĩa lịch nào** — không cron file, không systemd timer, không job nền tảng. Cam kết trong một văn bản pháp lý đang phụ thuộc vào việc có người nhớ gõ lệnh |
| **Quy trình nhận yêu cầu xoá** | Lệnh `an-danh` **chạy tay** bởi người tiếp nhận — cố ý, vì một tuyến công khai xoá theo số điện thoại là tuyến xoá dữ liệu người khác. Nhưng "ai trực hotline/email, trả lời trong bao lâu, ghi số phiếu ở đâu" thì chưa ai mô tả. Một cam kết pháp lý không có quy trình đằng sau là một cam kết sẽ lỡ |
| **Phép gọi thật tới `vihat-miniapp`** | Backend đang dựng song song, chưa có địa chỉ để gọi. Năm nhánh kết quả đã có test bằng `fetch` giả, nhưng **chưa một lần nào chạm máy chủ thật**. Phải gọi thử một lần — đủ cả 201, 401, 502 — trước khi nộp |
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

⚠ **Cả bốn đều cần `VIGOV_API_HOST`**, vì khối đăng nhập đọc địa chỉ máy chủ lúc dựng:

```bash
VIGOV_API_HOST=https://<host> npm run zmp:phat-hanh:goc
```

Thiếu biến thì script **dừng với mã thoát 2 trước khi dựng gì cả** — đẩy một bản chưa khai địa
chỉ là nộp một nút đăng nhập không đăng nhập nổi, kèm một câu chữ dành cho người dựng bản.
Script cũng **in địa chỉ ấy ra** cùng biến thể và nhãn phiên bản trước khi làm gì: nó được nung
thẳng vào bundle, nên người chạy lệnh phải đọc được nó. `--thu` thì không cần biến — nó chỉ in
kế hoạch rồi dừng.

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
