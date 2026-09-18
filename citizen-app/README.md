# citizen-app

The citizen-facing **Zalo Mini App**. React + Vite. **`zmp-sdk` is imported from exactly one
directory** — `src/features/quyen/`, the three permission screens — and from nowhere else. The
tripwire in `src/phase1-collects-nothing.test.ts` is what keeps that true: it was not removed when
those screens landed, it was **narrowed to one directory**, and it has a case proving it still
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
| **1 — now** | A static introduction to **VihatSoftware**, the company that publishes the app. The **discovery layer** — commune suggestion, commune picker, commune page — is built, and ships only in the `day-du` build | This is the submission Zalo reviews, so the OA `VihatSoftware` can verify the App ID |
| **1b — now** | The same introduction **plus three permission screens** (phone · location · QR), in the `quyen` build | Zalo grants `getPhoneNumber` / `getLocation` / `scanQRCode` only when the submission **visibly uses** them. Phase 2 needs all three, and they must be granted on **this** App ID before that surface can be built |
| **2 — next** | The commune / citizen surface behind a real session | It lands on the **same App ID**, already verified |

**The discovery layer is a secondary view of the SAME app, never a second app.** A second App ID
would need its own review, so "hiding the real app behind another App ID" buys nothing and costs
a second approval.

**Which build carries it is a choice made at deploy time** — see §"Ba biến thể bản dựng". The
`goc` and `quyen` builds — the two that get submitted — do **not contain** the discovery layer at
all: not the code, not the eight invented administrative names, not the sample phone numbers, not
the diagnostics panel. That is checked by building all three variants and reading the bundle, not
asserted. The `day-du` build carries it for demos and internal testing.

**Nothing of phase 2 gets deleted to make room for phase 1.** `src/lib/commune-resolution.ts`
is the foundation phase 2 builds on and stays untouched.

Why the verifying OA is VihatSoftware and not a commune, and why the notification OA is a
different OA per commune: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`. How the
commune gets resolved at runtime with no domain to key off:
`kb/10-decisions/0005-miniapp-tenant-resolution.md`. Both are read there, not repeated here.

### Phase 1 collects no personal data — keep it that way

No sign-in, no OTP, no form, no input control of any kind, **no backend call, no device storage**.
The only outbound links are `tel:`, `mailto:` and the company website.

**Kể từ biến thể `quyen`, câu ấy có một ngoại lệ có phạm vi, và nó phải được đọc nguyên văn:**
bản dựng `quyen` — và chỉ bản ấy — có ba màn gọi `getPhoneNumber` · `getLocation` · `scanQRCode`.
Ba màn ấy **lấy dữ liệu rồi hiện lên màn hình, hết**: không `fetch`, không `localStorage`, không
`console.log`, không gửi đi đâu. Hai lệnh cấm ấy trong `phase1-collects-nothing.test.ts` **không
được nới một dòng nào** — chúng là thứ biến "không gửi đi đâu" từ lời hứa thành ràng buộc kiểm
được. §"Ba màn quyền" nói rõ vì sao màn hình ấy **không có dữ liệu cá nhân ngay từ đầu**.

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

That is a deliberate design choice, not an accident of scope. It makes rule 3 (personal data)
hold **by construction** rather than by argument. In the `goc` build it also gives the Zalo review
nothing to weigh: an app that asks for nothing has no permission to justify.

**The `quyen` build deliberately gives up that second property, and only that one.** It asks for
three permissions, so it has three things to justify — and it justifies each of them on screen, in
Vietnamese, next to the button that asks (§"Ba màn quyền"). What it does **not** give up is the
first property: nothing is stored, nothing is sent, so there is still no personal data anywhere in
this app. Adding collection **beyond** those three screens changes what was submitted for review —
raise it before writing it.

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

Rules 6, 7 and 8 already bind phase 1. **Rules 1, 2 and 3 are live in the `day-du` build** (they
have nothing to bind in `goc`, which knows no commune), in
`src/features/kham-pha/`: the deep-link parameter only ever *suggests* a commune, the header slot
in `src/App.tsx` carries the commune name on every screen once one is confirmed, and switching
commune is a button nobody presses on the citizen's behalf. Rules 4 and 5 wait for a server — the
confirmed commune is client-side UI state, **not a session**; read the block at the top of
`src/features/kham-pha/GoiYXaScreen.tsx` before building anything on top of it.

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
permission screens keep theirs in `src/features/quyen/noi-dung.ts`, for a third reason: those are
the exact sentences the Zalo reviewer reads when deciding whether to grant the permissions, and a
justification scattered through JSX is one nobody re-reads before submitting.

**Nothing may be added to that file without a source.** The app carries the name of a real
legal entity: an unsourced founding year, customer name or award is a false statement
published under that name. Facts that are missing are left out, never filled in.

## Ba biến thể bản dựng

Người chạy lệnh chọn **đẩy bản nào**. Biến thể quyết định ở **tầng dựng**, qua biến môi trường
`VIGOV_BIEN_THE` (`vite.config.ts`).

| Biến thể | Nội dung | Dùng để | `dist/assets/app.js` |
|---|---|---|---|
| **`goc`** | **Chỉ** app giới thiệu bốn màn | Bản nộp **tối thiểu** | **254,39 kB** thô · 77,11 kB gzip |
| **`quyen`** | `goc` + ba màn quyền (`zmp-sdk`) | **BẢN NỘP XIN QUYỀN** | **518,50 kB** thô · 143,34 kB gzip |
| **`day-du`** (mặc định) | `quyen` + lớp khám phá + danh mục xã mẫu + trang xã + bảng chẩn đoán | Thử nghiệm nội bộ, demo | **530,02 kB** thô · 146,77 kB gzip |

Ba con số ấy **đo ngày 18/09/2026**, bằng `npm run build:goc` · `build:quyen` · `build`, đọc từ
chính dòng Vite in ra. Chênh lệch `goc` → `quyen` đúng bằng cái giá của `zmp-sdk`: **+264,11 kB
thô / +66,23 kB gzip**, tức app **hơn gấp đôi**. Đó là lý do SDK không nằm trong bản `goc`, và
cũng là lý do `lib/launch-params.ts` đọc tham số bằng `URLSearchParams` chứ không bằng SDK.

(Bản `goc` trước đây là 252,16 kB; nay 254,39 kB. Phần chênh là `screens.ts` đọc cờ
`CO_MAN_QUYEN` qua alias, cộng **≈2,2 kB các lớp CSS của ba màn quyền** — xem §"Thứ bản `goc`
VẪN mang" ngay dưới.)

**Vì sao tách bằng BUILD chứ không bằng một cờ lúc chạy.** Một cờ lúc chạy để tám tên đơn vị
hành chính **đặt ra**, tám số điện thoại mẫu và bảng chẩn đoán nằm nguyên trong bundle gửi
duyệt — chỉ là không vẽ ra. Tách ở tầng dựng thì bản `goc` **thật sự không chứa** chúng, và
điều đó **kiểm được bằng `grep` trên `dist/assets/app.js`**, không phải bằng lời hứa. Đó cũng
là điều làm việc này trung thực: bản nộp duyệt đúng bằng thứ người duyệt đọc.

### Thứ bản `goc` VẪN mang — nói ra, không giấu

**Ranh giới là DỮ LIỆU, không phải từ vựng.**

| Còn lại trong bản `goc` | Vì sao để nguyên |
|---|---|
| Hai nhãn `Chọn xã` · `Đổi xã` (`App.tsx`) | Vỏ không nằm sau alias, nhánh ấy không bao giờ chạy. Từ ngữ hành chính thông thường, không phải đơn vị hành chính đặt ra |
| **≈2,2 kB các lớp CSS của ba màn quyền** (`.quyen__*`, `.quyen-khu__*`) | `src/styles.css` là **một** tệp, `main.tsx` nạp trọn, và `main.tsx` không nằm sau alias. Tách biểu mẫu kiểu theo biến thể là dựng **cơ chế thứ hai** cạnh `resolve.alias` cho 2,2 kB — đắt hơn thứ nó mua. Trong 2,2 kB ấy không có một câu chữ nào: chỉ tên lớp và thuộc tính, và không màn nào ở bản `goc` mang các lớp ấy |

Cả hai đều có **một ca trong `src/bundle-for-zalo.test.ts` khẳng định chúng CÓ mặt**, để lần sau
ai `grep` thấy thì đọc được ngay lý do thay vì tưởng alias đã thủng. Ngược lại, bản `goc` **không**
chứa `zmp-sdk`, `getPhoneNumber` hay `scanQRCode` — cũng là một ca kiểm, dựng thật rồi đọc bundle.

Cơ chế là `resolve.alias`, không phải tree-shaking — tree-shaking **không** loại được một
`import` tĩnh đã có mặt trong mã. Ba cái tên `bien-the/kham-pha`, `bien-the/chan-doan` và
`bien-the/quyen` là **ba cửa duy nhất** vào ba phần gỡ được; biến thể nào không có phần ấy thì
cửa trỏ sang `index.rong.ts`.

| Tệp | Việc nó làm |
|---|---|
| `src/features/kham-pha/index.ts` · `index.rong.ts` | Bề mặt lớp khám phá, và bản rỗng của nó |
| `src/features/diagnostics/index.ts` · `index.rong.ts` | Như trên, cho bảng chẩn đoán |
| `src/features/quyen/index.ts` · `index.rong.ts` | Như trên, cho ba màn quyền — cửa chỉ có **hai** cái tên: `CO_MAN_QUYEN` và `MAN_QUYEN` |
| `src/features/kham-pha/bien-the.test.ts` | Ba bản rỗng khai đúng bề mặt · **không tệp nào nhập thẳng vòng qua alias** · danh sách biến thể ở `vite.config.ts` và `scripts/dung.mjs` **không lệch nhau** |
| `src/bundle-for-zalo.test.ts` | Dựng thật **cả ba** biến thể rồi đọc bundle — bằng chứng cuối cùng |

`tsc` luôn nhìn bản **đầy đủ** (`tsconfig.json` → `paths`); bản rỗng khai kiểu bằng `typeof`
của bản thật, nên thiếu một export là `tsc --noEmit` đỏ chứ không phải bản `goc` vỡ lúc dựng.

## Ba màn quyền — biến thể `quyen`, bản nộp xin quyền

Zalo **chỉ cấp** `getPhoneNumber` · `getLocation` · `scanQRCode` khi bản nộp **có chỗ dùng chúng
nhìn thấy được**, và chính sách Mini App điều 3.3.4 (trích ngay trên `getPhoneNumber` trong
`node_modules/zmp-sdk/index.d.ts`) nói thẳng: *"chúng tôi sẽ từ chối xét duyệt cho những Mini App
có luồng xin cấp quyền chưa rõ ràng, không nêu được mục đích xin quyền đến người dùng"*. Nên mỗi
màn có **một nút, một lời giải thích vì sao app cần quyền ấy, và một chỗ hiện kết quả**.

| Màn | API | Lời giải thích trên màn | Kết quả hiện ra |
|---|---|---|---|
| Số điện thoại | `getPhoneNumber` | Xác thực người dùng khi gửi yêu cầu hỗ trợ | Đã nhận **token** · độ dài · vài ký tự đầu (đã che) |
| Vị trí | `getLocation` | Gợi ý điểm hỗ trợ gần nhất | Như trên |
| Quét QR | `scanQRCode` | Quét mã tra cứu thay cho gõ tay | **Nội dung quét được**, nguyên văn |

### SỰ THẬT ĐÃ ĐO TỪ `zmp-sdk` 2.53.0 — đừng tra lại tài liệu web

```
GetPhoneNumberReturns = { number?: @deprecated; token?: string }
GetLocationReturns    = { latitude?/longitude?/timestamp?/provider?: @deprecated; token?: string }
ScanQRCodeReturns     = { content: string }
```

Token của cả hai: **dùng được một lần, hết hạn sau 2 phút**, và chỉ đổi được ở **máy chủ** bằng
app secret.

**Hệ quả thiết kế, và đây là điều quan trọng nhất của cả lớp này: số điện thoại và toạ độ KHÔNG
BAO GIỜ tới thiết bị.** Chỉ có token. Màn hình không có gì để che vì nó **không có dữ liệu cá
nhân ngay từ đầu** — và màn hình **nói ra điều đó bằng tiếng Việt**, vì đó là lý do đáng tin nhất
để một người dân bấm đồng ý. `number` / `latitude` / `longitude` đều `@deprecated`: đọc chúng là
tự rước dữ liệu cá nhân về máy đúng lúc nền tảng vừa bỏ đường ấy đi.

`scanQRCode` là API **duy nhất** ở đây trả về dữ liệu thật. Nội dung ấy có thể là bất cứ thứ gì,
kể cả dữ liệu cá nhân. **Hiện lên màn hình được; `console.log` thì không**, và không chỗ lưu nào.

### Giới hạn cứng của ba màn này

Ba màn **lấy được dữ liệu và hiện ra màn hình, rồi thôi. Không gửi đi đâu cả.** Các dây bẫy cấm
`fetch`/XHR/WebSocket/EventSource/axios và `localStorage`/`sessionStorage`/`document.cookie`/
`indexedDB` trong `src/phase1-collects-nothing.test.ts` **giữ nguyên, không nới một dòng**. Nếu
một thay đổi cần nới một trong hai, **dừng lại và nói ra** — nghĩa là thiết kế đã lệch.

Lệnh cấm `zmp-sdk` và cấm ba tên hàm ấy thì **đổi**, vì đó chính là ranh giới giai đoạn đang được
cố ý bước qua. Cách đổi: **không xoá, mà thu hẹp phạm vi** — chỉ `src/features/quyen/` được nhắc
tới chúng; mọi tệp khác vẫn bị cấm như cũ, và có một ca cho lệnh cấm ăn một chuỗi vi phạm **đặt ở
ngoài thư mục ấy** để chứng minh nó còn sống. Một lệnh cấm bị xoá là một lệnh cấm không ai biết là
đã mất.

### Ràng buộc kỹ thuật đã trả giá để biết

| Điều | Hệ quả trong mã |
|---|---|
| `zmp-sdk` **đụng `window` ngay lúc nạp mô-đun** | Không `import` tĩnh ở cấp cao nhất — nó làm sập mọi test chạy dưới Node. Phải `await import("zmp-sdk")` **bên trong hàm**, trong `try/catch` |
| Ngoài Zalo thì lời nhập ấy hỏng | Màn hình **nói ra bằng tiếng Việt**, không trắng trơn. `catch {}` im lặng ở đây là một màn trống trên máy người duyệt |
| Người dùng **từ chối** (`code === -201`, lấy từ ví dụ trong chính `index.d.ts`) | Là **đường đi bình thường**, không phải lỗi: một câu tiếng Việt nói họ bấm lại được, không mã lỗi, không màu đỏ |
| Ở môi trường phát triển, hai API token **luôn thành công và trả token rỗng** | Màn hình nói ra đúng như vậy thay vì hiện một ô trống |

| Tệp | Việc nó làm |
|---|---|
| `src/features/quyen/zalo-api.ts` | **Nơi duy nhất** nhắc `zmp-sdk`. Ba lời gọi, quy mọi đường về bốn nhánh: `xong` · `tu-choi` · `ngoai-zalo` · `khong-lay-duoc` |
| `src/features/quyen/noi-dung.ts` | Mọi chữ người dùng đọc trên ba màn — `bundle-for-zalo.test.ts` dùng lại đúng danh sách này |
| `src/features/quyen/ManQuyen.tsx` | Ba màn + khu vực chọn màn. `ManQuyenThuan` là bản **thuần** để bốn nhánh kết quả kiểm được mà không cần Zalo |
| `src/features/quyen/quyen.test.tsx` | 29 ca: từ chối · ngoài Zalo · token không hiện trọn · câu "dữ liệu thật không tới thiết bị" · một `<h1>` mỗi màn |

## Nộp lên Zalo

### Chuỗi lệnh

```bash
npm run zmp:login             # một lần, cần App ID
npm run zmp:deploy            # bản ĐẦY ĐỦ, bản thử nghiệm (-t)
npm run zmp:deploy:goc        # bản GỐC,    bản thử nghiệm (-t)
npm run zmp:deploy:quyen      # bản QUYỀN,  bản thử nghiệm (-t)
npm run zmp:phat-hanh:goc     # bản GỐC,    BẢN PHÁT HÀNH (bỏ -t)
npm run zmp:phat-hanh:quyen   # bản QUYỀN,  BẢN PHÁT HÀNH (bỏ -t) — đây là bản gửi XIN QUYỀN
```

Cả sáu đi qua `scripts/deploy.mjs`: dựng đúng biến thể → `sync-config` → `deploy`. Dựng nằm
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
`dirty` nói ra rằng bản ấy **không ứng với commit nào** — không ai dựng lại được nó, kể cả
người vừa đẩy. Tính trong `scripts/deploy.mjs` chứ không trong `package.json`, vì trên Windows
`npm run` chạy qua `cmd`, nơi `$(git rev-parse …)` chỉ là một chuỗi ký tự.

`-t` là **bản thử nghiệm**. Bỏ `-t` là đẩy **bản phát hành**, và đường ấy nay có trong script
(`zmp:phat-hanh:goc` · `zmp:phat-hanh:quyen`). Trước đây nó cố ý không có, để việc phát hành phải là một quyết định có
người gõ tay ra; nay người dùng cần đường ấy, nên **ma sát chuyển chỗ chứ không biến mất**: từ
*"không có lệnh"* sang *"lệnh nói rõ nó đang làm gì"*. Script **in ra biến thể, loại bản và
nhãn phiên bản trước khi làm gì**, rồi đường phát hành **dừng 5 giây** để người chạy kịp
`Ctrl-C`. Đừng bỏ phần in ra và phần đếm ngược — đó là cái phanh còn lại.

Nhãn phiên bản mang cả **biến thể**: hai lần đẩy cùng một commit, một `goc` một `day-du`, mà
nhãn giống nhau thì console Zalo có hai dòng không phân biệt được — và dòng đem đi duyệt là
dòng đoán ra.

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
| `location.search` mang **đúng** những gì `getRouteParams()` mang | Đo 18/09/2026 trên Version 6, mở nguội qua deep link: cả tham số nền tảng (`env`, `version`) lẫn tham số riêng (`t`, `src`, `debug`) đều có mặt. Nên lớp khám phá đọc tham số **chỉ từ `location.search`**, không qua SDK. Cái giá của SDK đo lại ngày 18/09/2026 sau khi có biến thể `quyen`: `goc` **254,39 kB thô / 77,11 kB gzip** ↔ `quyen` **518,50 / 143,34** — **hơn gấp đôi**, và gần trọn phần chênh là `zmp-sdk` (§"Ba biến thể bản dựng"); ba màn quyền tự thân chỉ vài kB. Chế độ hỏng nếu phép đo sai ở một đường mở nào đó là chế độ hỏng **lành**: không có tham số ⇒ app giới thiệu bình thường ⇒ công dân tự chọn xã, đúng đường ADR 0005 bắt buộc phải chạy được. Nhập SDK **chỉ để đọc tham số** là bắt mọi người dùng trả hơn 66 kB gzip trong mọi lần mở, cho một việc `URLSearchParams` làm không tốn gì — nên ngay cả khi SDK đã có mặt ở biến thể `quyen`, `launch-params.ts` vẫn không dùng nó |

### Còn thiếu

| # | Cái gì | Ghi chú |
|---|---|---|
| 0 | **Xoá danh mục xã mẫu** — `src/features/kham-pha/demo-danh-muc-xa.ts` — trước khi kênh công dân phục vụ người thật | Tám tên đơn vị hành chính **đặt ra**, kèm nội dung riêng của từng xã (giới thiệu · số trực · giờ làm việc · dịch vụ), và hai dòng chữ nói rõ đây là dữ liệu mẫu. Số trực dùng **dải giả đã thoả thuận `090000000x`** (luật 3, bất biến 5) — `kham-pha.test.tsx` ghim dải ấy, `phase1-collects-nothing.test.ts` quét toàn cây mã và **chỉ** miễn đúng dải ấy. Nguồn thật là `ListTenants` của service `platform` — đã khai trong proto, **chưa có cài đặt**, còn chờ xác thực người gọi trên cổng gRPC. Ba ca trong `kham-pha.test.tsx` ghim rằng đây là nơi **duy nhất** có tên đơn vị hành chính, để xoá một tệp là xoá sạch |
| 1 | Ảnh chụp màn hình và mô tả trên store | Bắt buộc để duyệt. Icon thì đã có — `tools/logo.py` dựng từ `brand/lg_vhs_full.svg` |
| 2 | Các khoá còn lại trong `app-config.json` | `app.*` viết từ nguồn thứ cấp và **chưa đối chiếu** với Developer Console. Ba khoá `list*` thì đã do `sync-config` sinh, không phải đoán |

## Lớp khám phá — cái gì đang chạy, và cái gì còn thiếu

Lớp KHÁM PHÁ của ADR 0005 **đã dựng xong và nằm trong biến thể `day-du`** — không nằm trong bản
`goc` gửi duyệt (§"Hai biến thể bản dựng"). Ba lớp của ADR 0005 vẫn tách rời, và đây là bảng
phải đọc trước khi sửa bất cứ thứ gì trong `src/features/kham-pha/`:

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
ấy**: một dòng giới thiệu, số điện thoại trực, giờ làm việc, và 3–5 dịch vụ **khác nhau giữa
các xã**. Không thêm tab thứ năm và không che thanh tab — phần giới thiệu, thứ Zalo đã duyệt,
luôn còn đường tới, và bấm tab đầu là về lại trang xã.

Một trang xã giống hệt trang xã bên cạnh thì tên trên header là bằng chứng **duy nhất** rằng
công dân vào đúng chỗ, và một bằng chứng duy nhất đọc lướt thì không ai đọc. Nội dung khác nhau
là thứ làm cho "vào nhầm xã" nhìn ra được — trước khi nó thành một hồ sơ gửi nhầm cơ quan.

Các mục dịch vụ **chưa dẫn đi đâu**: mỗi mục mang chữ *"Chưa mở"* (bằng **chữ**, không bằng
màu) và bấm vào thì nói rõ đang xây dựng kèm việc cần làm bây giờ. Số trực **không** là liên kết
`tel:` — số ấy là số giả của bản trình diễn, và một liên kết bấm được là một cuộc gọi mất không.

**Mức tin theo nguồn** — đây là quy tắc, không phải giao diện:

| `src` | Hành vi |
|---|---|
| `qr` · `zns` | Chọn sẵn, một chạm xác nhận. Người đang đứng ở trụ sở xã không nên bị bắt đi tìm lại chính xã đó |
| `share` · không khai · tra mã không ra | **Luôn** bắt chọn tường minh — liên kết chuyển tay không nói lên ý định người nhận |

**HAI THỨ PHẢI ĐỔI KHI CÓ MÁY CHỦ, và cả hai đều không phải việc sửa giao diện:**

1. Danh mục xã mẫu (`demo-danh-muc-xa.ts`) thay bằng **`ListTenants`** của service `platform` —
   đã khai trong `proto/vigov/platform/v1/platform.proto`, **chưa có cài đặt**, và còn chờ xác
   thực người gọi trên cổng gRPC (đọc chú thích trong chính tệp proto ấy).
1b. **Danh sách dịch vụ của xã là NGHIỆP VỤ, không phải giao diện.** Mỗi xã chỉ được hiện những
   dịch vụ mình thực sự tiếp nhận — hiện một dịch vụ xã chưa mở là mời công dân chờ một thứ
   không tồn tại. Ở giai đoạn 2 danh sách ấy đọc **lúc chạy** từ cấu hình của từng xã (luật 1,
   bất biến 10), **không bao giờ** là hằng số trong mã: một mã nguồn phục vụ nhiều xã. Hôm nay
   nó là hằng số **chỉ vì** đó là dữ liệu trình diễn, và nó biến mất cùng `demo-danh-muc-xa.ts`.
   Các mục **chưa dẫn đi đâu**: mỗi mục mang chữ *"Chưa mở"* và bấm vào thì nói rõ đang xây
   dựng — không có màn giả nào được dựng.

2. Xã đã xác nhận hôm nay là **trạng thái giao diện phía client**, không phải phiên: sống trong
   `useState`, mất khi app đóng, không lưu xuống máy, không gửi đi đâu, **không cấp quyền gì**.
   ADR 0005: xã của phiên do **máy chủ** ghi sau khi công dân xác nhận. Thay bằng xã đọc từ phiên
   — **không phải "đồng bộ thêm" với nó**: hai nguồn cho một sự thật thì một trong hai sẽ cũ, và
   cái cũ là cái đi vào hồ sơ gửi nhầm cơ quan.

`src/lib/commune-resolution.ts` vẫn **chưa được nối vào** — nó là bộ khung của giai đoạn 2, nơi
bốn đường phân giải xã (deep link → hồ sơ → GPS gợi ý → danh mục) gặp nhau khi có máy chủ. Hôm
nay mới có đường 1 và đường 4.

## Lệnh

| Lệnh | Việc |
|---|---|
| `npm run dev` | Máy chủ phát triển Vite (biến thể **đầy đủ**) |
| `npm run build` | Gói tĩnh vào `dist/` — biến thể **đầy đủ** |
| `npm run build:goc` | Như trên, biến thể **gốc** (bản nộp tối thiểu) |
| `npm run build:quyen` | Như trên, biến thể **quyền** (bản nộp xin quyền) |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest — sự thật đã công bố, hình dạng bundle, sổ màn hình, ba màn quyền, **và ba biến thể đúng bằng thứ người duyệt đọc** |
| `npm run zmp:sync` | Dựng (đầy đủ) rồi đồng bộ `app-config.json` theo trang đã dựng |
| `npm run zmp:deploy` | Đầy đủ → bản thử nghiệm |
| `npm run zmp:deploy:goc` | Gốc → bản thử nghiệm |
| `npm run zmp:deploy:quyen` | Quyền → bản thử nghiệm |
| `npm run zmp:phat-hanh:goc` | Gốc → **bản phát hành**, có in ra và đếm ngược 5 giây |
| `npm run zmp:phat-hanh:quyen` | Quyền → **bản phát hành**, có in ra và đếm ngược 5 giây |

→ Skills: `.claude/skills/zalo-miniapp-multi-tenant` · `.claude/skills/accessibility-elderly`
