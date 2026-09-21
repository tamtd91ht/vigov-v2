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
| **1 — now** | A **real, working product app of ViHAT Group**: the company introduction plus three features that use `getPhoneNumber` · `getLocation` · `scanQRCode`. The **discovery layer** — commune suggestion, commune picker, commune page — is built and ships only in the `day-du` build | This is the submission Zalo reviews, so the OA `Vihat` can verify the App ID **and** grant the three permissions. Zalo grants them only when the submission **visibly uses** them, and phase 2 needs all three on **this** App ID |
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

### Đổi pháp nhân đứng tên app — 21/09/2026

**App chuyển quyền sở hữu từ VihatSoftware sang ViHAT Group (công ty mẹ), OA xác thực đổi từ
`VihatSoftware` sang `Vihat`.** Giai đoạn 1 từ nay là app giới thiệu **Tập đoàn ViHAT Group**,
không còn là app giới thiệu công ty con.

Ba hệ quả đã vào mã, và mỗi thứ có một ca kiểm giữ:

| Đổi gì | Vì sao không chỉ là đổi chuỗi |
|---|---|
| `COMPANY` và `GROUP` **nhập làm một** | Hai hằng ấy tồn tại để công ty con không nhận vơ số liệu, hệ sinh thái và đầu mối liên hệ của công ty mẹ. Bên phát hành nay CHÍNH LÀ công ty mẹ, nên ba ghi chú disclaim hết lý do tồn tại — gỡ khỏi cả nội dung lẫn từng màn đang vẽ chúng, không để lại node rỗng (`screens.test.tsx`) |
| `ownsThisApp` **biến mất** | Không đơn vị thành viên nào còn "sở hữu app này". VihatSoftware **vẫn là một trong sáu đơn vị thành viên** và vẫn hiện trong danh sách |
| Câu định vị · mô tả · website **đã điền lại từ `vihatgroup.com`** (21/09, chiều) | Ba chuỗi cũ là văn bản đã công bố **của VihatSoftware** và đã bị xoá sáng 21/09; chiều 21/09 chúng được điền lại từ trang của **chính tập đoàn**, và `company-profile.test.ts` nay ghim cả ba **nguyên văn** thay vì ghim chúng là `undefined`. ⚠ **Câu định vị đổi sang TIẾNG VIỆT, và đó là một quyết định thay khách**: tập đoàn không công bố câu định vị tiếng Anh nào, câu tiếng Anh cũ là của công ty con. Đánh dấu tại chỗ trong `company-profile.ts` để khách bác được |
| **Tầng nền** — chuyển sắc + bóng đổ thay cho đường kẻ | Hướng thị giác của chủ dự án (21/09): đây là một công ty **công nghệ** — AI · tổng đài · CRM · SMS · ZNS — nên nền phải có chiều sâu, không phẳng trơn. Bốn token mới, **CSS thuần, không thêm thư viện nào**: `--bong-the` · `--bong-noi` (bóng đổ dựng từ chính `--navy`, nên nó xanh chứ không xám) và `--nen-the` · `--nen-the-luc` (nền thẻ chuyển sắc). Xanh lá `#78bd1a` nay có mặt trên **bề mặt sáng** chứ không chỉ trên panel tối. ⚠ Một dải chuyển sắc là chỗ độ tương phản **chết đầu tiên**, và nó chết ở phía **dưới** thẻ: nên hai đầu của mỗi dải đều là màu đã đo, `accessibility.test.ts` có thêm **6 cặp đo chữ ở đầu kia** và **3 ca buộc dải chỉ đi qua token đã đo** |
| Màu đậm `#1e3150` → **`#144a80`** | `#1e3150` là màu khung website **của công ty con**. `#144a80` là `--color-primary` của `vihatgroup.com`. Bốn chỗ giữ giá trị này, ba chỗ được `bundle-for-zalo.test.ts` ghim chung: `BRAND_NAVY` · `app-config.json` · `index.html` · `--navy`. Bộ icon **không** phải chỗ thứ năm — `tools/logo.py` đọc màu từ `fill` của vector logo |

⚠ **Chính sách quyền riêng tư đổi ĐÚNG MỘT VẾ.** Vế "ai phát hành / ai chịu trách nhiệm" sang
ViHAT Group; vế "dữ liệu đăng nhập đi tới **máy chủ của VihatSoftware**" (`vihat-miniapp`)
**giữ nguyên** — đó là lời khai nơi nhận dữ liệu theo Nghị định 13, và **ai vận hành máy chủ ấy
sau chuyển giao là câu chưa ai trả lời**. Ba ca trong `chinh-sach.test.ts` canh cả hai chiều, để
một lượt tìm-thay trên cả tệp không biến lời khai ấy thành một lời khai sai.

Why the verifying OA is an OA of the publisher and not a commune, and why the notification OA is a
different OA per commune: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md` (ADR ấy
được soạn lại theo quyết định 21/09 — đọc ở đó, không chép lại ở đây). How the
commune gets resolved at runtime with no domain to key off:
`kb/10-decisions/0005-miniapp-tenant-resolution.md`. Both are read there, not repeated here.

### The app stores nothing on the device, and sends exactly ONE thing — keep it that way

**No OTP, no form, no input control of any kind, no device storage.** The outbound links are
`tel:`, `mailto:`, a map page opened only when the user taps "Chỉ đường", and the page behind a
QR code the user just scanned, and the company's own website. (`COMPANY.website` was supplied on
2026-09-21, so that anchor is back — on the Liên hệ screen and on a solution detail page. The
privacy policy counts **three kinds of destination**, not three buttons: two anchors to one address
are one destination, and counting buttons would mean editing the policy every time a button moves.)

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

## Hai nửa nghiệp vụ trong MỘT bundle — ba ràng buộc, mỗi cái là một ca test

Dựng ngày 21/09/2026, **trước khi `src/cong-dan/` có tệp nghiệp vụ đầu tiên**. Chi tiết và thủ tục:
`src/cong-dan/index.ts` (chú thích đầu tệp). Dây bẫy: `src/ranh-gioi-hai-nua.test.ts`.

| Nửa | Ở đâu |
|---|---|
| Thương mại (khách hàng doanh nghiệp) | `src/content/` · `src/features/company-intro/` · `src/features/tinh-nang/` · `src/features/dang-nhap/` |
| **Nhà nước (công dân)** | **`src/cong-dan/`** — còn rỗng, có tệp giữ chỗ để lượt quét đọc tới nó |
| Lớp vỏ trung lập | `App.tsx` · `main.tsx` · `components/` · `lib/` · `features/kham-pha/` · `features/diagnostics/` |

1. **Ranh giới hai chiều.** Nửa này không nhập tệp của nửa kia — cả hai chiều. Và **không tệp nào
   ngoài `./cong-dan/` được nhập client API của ViGov** (`./cong-dan/api/`), kể cả lớp vỏ: `App.tsx`
   không nằm sau `resolve.alias` nào, nên một `import` ở đó đi thẳng vào **bản nộp**. Một tệp không
   thuộc khu nào cũng đỏ — thư mục mới **buộc phải khai**, vì một thư mục ngoài mọi tiền tố là một
   thư mục ranh giới không cấm được gì.
2. **Không lưu trữ định danh, ở cả hai nửa.** Một bundle là **một origin**: `localStorage` ·
   `sessionStorage` · `IndexedDB` là **chung** giữa hai nửa theo cấu tạo, không có partition nào.
   Phiếu phiên, số điện thoại, xã đã chọn sống trong `useState`. Lệnh cấm bắt cả họ tên `IDB*` —
   hình thức duy nhất chạm IndexedDB mà không gõ ra cái tên trần.
3. **Mỗi lời gọi SDK khai mục đích tại chỗ.** Bảng `KHAI_BAO_LOI_GOI` trong
   `features/tinh-nang/zalo-api.ts` (nửa nào · màn nào · tính năng nào · để làm gì · Zalo có hỏi
   không · gì rời khỏi máy). Màn **Quản lý quyền** vẽ từ bảng ấy, **không** từ một danh sách chép
   tay. Quyền cấp theo **App ID**, nên nửa nhà nước **thừa hưởng nguyên vẹn** mọi quyền nửa thương
   mại xin được — cột `nua` làm điều đó nhìn thấy được thay vì là một hệ quả nền tảng không ai nói.

⚠ Một lời gọi trốn khỏi bảng khai bằng cách **ép kiểu** (`(sdk as X).openChat()`) hoặc **đổi tên**
(`const s = sdk`). Cả hai hình dạng bị cấm riêng — **lỗ hổng ấy tìm ra bằng một lần thử đột biến
thất bại**, không bằng suy luận; xem khối chú thích của ca ấy.

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

Hai tệp nội dung nữa, thêm 21/09/2026 (tối), và cả hai ở đó vì **nguồn của chúng không phải
vihatgroup.com**:

| Tệp | Giữ gì | Vì sao tách riêng |
|---|---|---|
| `src/content/tin-tuc.ts` | Hai bài mới nhất trên trang tin — **ảnh chụp ngày 21/09/2026**, kèm ngày đăng và cách cập nhật | Chữ của trang tin, chép nguyên văn. App **không gọi mạng** để lấy tin: xem §"Còn thiếu" |
| `src/content/dich-ra-ngoai.ts` | Năm đích ứng dụng mở ra ngoài, và câu khai trong chính sách dựng ra từ đó | Đây là nguồn của một câu trong **văn bản pháp lý**, không phải một câu giới thiệu |

Câu slogan trong hero (`SLOGAN_HERO` trong `company-profile.ts`) là **chữ của bản mẫu PM**, không
phải câu đã công bố — nên nó là một đối tượng mang cả `nguon` đi kèm, chứ không phải một chuỗi
trần: một chú thích tách khỏi giá trị được, một trường thì không. Câu định vị đã công bố
(`COMPANY.positioning`) **giữ nguyên chỗ của nó**, ngay dưới slogan.

**Nothing may be added to `company-profile.ts` without a source.** The app carries the name of a
real legal entity: an unsourced founding year, customer name, price, efficiency figure or award is
a false statement published under that name. Facts that are missing are left out, never filled in.

## Hai biến thể bản dựng

Người chạy lệnh chọn **đẩy bản nào**. Biến thể quyết định ở **tầng dựng**, qua biến môi trường
`VIGOV_BIEN_THE` (`vite.config.ts`).

| Biến thể | Nội dung | Dùng để | `dist/assets/app.js` |
|---|---|---|---|
| **`goc`** | Ứng dụng sản phẩm đầy đủ: bốn màn giới thiệu + tab Danh thiếp (ba tính năng) + hai khối trên màn Liên hệ (đăng nhập · tìm văn phòng) + trang chi tiết giải pháp + màn Quản lý quyền + bốn khối màn chủ (menu nhanh · giải pháp nổi bật · Tin ViHAT · quyền tóm tắt) + nút Chat nổi | **BẢN NỘP** | **602.274 B** thô · 164.993 B gzip |
| **`day-du`** (mặc định) | `goc` + lớp khám phá + danh mục xã mẫu + trang xã + bảng chẩn đoán | Thử nghiệm nội bộ, demo | **613.821 B** thô · 167.989 B gzip |

Hai con số ấy **đo ngày 21/09/2026 (tối)**, bằng `node scripts/dung.mjs goc` và `… day-du`, đọc từ
chính tệp phát ra (gzip mức 9). So với lần đo cùng ngày buổi chiều (591.294 / 602.844): **+10.980 B**
ở bản nộp (+1,9%; gzip +2.423 B, +1,5%), **+10.977 B** ở bản đầy đủ. Toàn bộ phần tăng nằm ở phần
CHUNG và **phần lớn là CHỮ**: hai bài tin chụp sẵn, sáu mục menu, câu slogan, câu khai đích ra ngoài
trong chính sách. Không một thư viện nào được thêm; CSS mới là 4 khối thuần.

Đoạn dưới là phép đo của lần trước, giữ lại để thấy cái giá của từng lượt:
từ chính tệp phát ra. So với lần đo cùng ngày buổi sáng (576.805 / 588.346): **+14.489 B** ở bản
nộp (+2,5%; gzip +3.136 B, +2,0%), **+14.498 B** ở bản đầy đủ. Trong đó **lớp nền/chiều sâu chỉ
chiếm 1.492 B thô / 207 B gzip** — toàn bộ là CSS thuần, không một thư viện hoạt hoạ hay bộ icon
nào được thêm; phần còn lại — hai bản tăng gần bằng nhau, vì mọi thứ thêm vào đều nằm ở phần
CHUNG: ba chuỗi thương hiệu, dải chín mốc lịch sử, bảng khai mười hai lời gọi, màn Quản lý quyền và
trang chi tiết giải pháp. Phần lớn số ấy là CHỮ, và đó là cái giá rẻ nhất trong toàn bộ bảng này.

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

### `.env.local` — cấu hình cho máy đẩy bản

Bản lên Zalo được đẩy từ **máy local**, nên hai biến trên nằm trong một tệp, không phải gõ vào
shell mỗi lần:

```bash
cp .env.local.example .env.local     # rồi điền địa chỉ máy chủ vào .env.local
```

| | |
|---|---|
| Đọc bằng | `loadEnv` trong `scripts/cau-hinh.mjs` — **một nguồn sự thật**, `vite.config.ts` và `scripts/deploy.mjs` cùng nhập nó |
| Ưu tiên | **biến shell THẮNG tệp** — CI và một lần đẩy tay phải đè được tệp local. Đã đo, không suy đoán |
| Vào git? | Không. `.gitignore` gốc kho chặn `.env.local` (dòng 3). `.env.local.example` thì có trong kho, chỉ chứa placeholder |

**Hai cái bẫy đã trả giá để biết, ghi ra để không ai vấp lại:**

1. **Vite KHÔNG tự nạp `.env.local` vào `process.env`.** Chỉ `loadEnv()` mới đọc các tệp
   `.env*`. Bản trước của `vite.config.ts` đọc `process.env.VIGOV_API_HOST`, nên đặt tệp xuống
   thì nó **im lặng** trả rỗng và `deploy.mjs` chặn đường đẩy vì tưởng chưa khai host.
2. **Tham số thứ ba của `loadEnv` phải là `""`.** Mặc định nó chỉ lấy biến có tiền tố `VITE_`,
   và đó cũng là một cái hỏng không báo gì.

`scripts/deploy.mjs` chạy **ngoài** Vite nên không tự thấy tệp — nó nhập cùng `docCauHinh()`.
Chép logic đọc ra hai nơi thì ngày chúng lệch là ngày script chặn một bản hợp lệ, hoặc tệ hơn:
cho qua một bản không có host rồi đẩy lên Zalo.

⚠ **`.env.local` KHÔNG PHẢI CHỖ ĐỂ BÍ MẬT — và đây là một cái rào, không phải một lời khuyên.**
Mọi giá trị ở đây đi qua `define:` và được **nung thẳng vào bundle** gửi lên Zalo rồi tải về máy
người dùng (luật 8, bất biến 4 — cùng lý do với `NEXT_PUBLIC_*`). Một người quen `.env` phía máy
chủ sẽ đặt secret key vào đây theo phản xạ. Nên:

| Cơ chế | Chặn được gì |
|---|---|
| **Danh sách trắng hai tên** (`VIGOV_API_HOST`, `VIGOV_BIEN_THE`) trong `scripts/cau-hinh.mjs` | Một tên lạ trong `.env.local` làm **bước dựng DỪNG** kèm câu giải thích và chỉ ra chỗ đúng — đã thử: `ZALO_MINIAPP_SECRET_KEY` cho `exit=1` |
| **`docCauHinh()` trả về ĐÚNG HAI KHOÁ**, không bao giờ trả cả môi trường | `loadEnv(…, "")` gom toàn bộ `process.env` (đo được **87 khoá**). Trả nguyên đống ấy ra là đặt sẵn đường cho một lượt sửa `define: { ...docCauHinh() }` nung `ZMP_TOKEN` vào bundle |

**`.env` là TỆP KHÁC, và nó ở đúng chỗ của nó.** `citizen-app/.env` thuộc về `zmp-cli`
(`APP_ID`, `ZMP_TOKEN` — token đăng nhập Zalo, một bí mật thật). Công cụ dòng lệnh đọc nó trên
máy; nó **không** đi vào bundle. Danh sách trắng vì thế **chỉ soi `.env.local`** — bắt `.env`
theo luật của bước dựng là làm hỏng một thiết lập đang chạy đúng.

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

### Grep thật trên bản `goc`, đo lại 21/09/2026 (tối)

```
cơ quan: 0 · công dân: 0 · chính quyền: 0 · hành chính: 0 · thủ tục: 0 · Chọn xã: 0 · Đổi xã: 0
xã hội: 1 (câu tầm nhìn đã công bố — phải còn)
zalo.me: 25 — đường dẫn cửa sổ trò chuyện với Official Account của TA đếm ĐÚNG 1 lần
              (`https://zalo.me/<OA id>`); 24 lần còn lại nằm trong chính `zmp-sdk`
vihatgroup.com: 4 = 1 địa chỉ trang chủ + 2 đường dẫn bài viết + 1 lời khai nguồn câu slogan

VihatSoftware: 7  = 5 câu "máy chủ của VihatSoftware" (cố ý) + 1 tên đơn vị thành viên
                    + 1 mốc lịch sử "Thành lập VihatSoftware" (mới 21/09)
"ViHAT Software" (có dấu cách): 0 — một chính tả trên màn, và đó là ca trong company-profile.test.ts
vihatgroup.com: 1 · vihatsoftware.com: 0 · #144a80: 1 · #1e3150: 0
2012: 0 — dải lịch sử bắt đầu 2013, đúng ngày thành lập
"12 năm": 1 — nằm TRONG câu mô tả nguyên văn của khách. Con số app tự in ra là con số TÍNH RA,
          nên nó không có mặt như một chuỗi trong bundle.

serverUploadUrl: 2 — CẢ HAI là của chính `zmp-sdk` (lược đồ zod của `openMediaPicker`, và thân
hàm đọc `e.serverUploadUrl`). Mã của ta đóng góp 0. Phép đo đúng là "không tệp nào GÁN một
chuỗi cho tham số ấy": 0 lần, ở cả hai biến thể. Xem `bundle-for-zalo.test.ts`.

localStorage: 1 · sessionStorage: 0 · indexedDB: 0
⚠ MỘT LẦN `localStorage` ẤY LÀ CỦA `zmp-sdk`, KHÔNG PHẢI CỦA TA — đã mở ra xem: nó nằm trong lớp
Storage của SDK (`value: localStorage` bên trong một `WeakMap`). Mã của kho này không chạm kho lưu
trữ nào, và hai dây bẫy độc lập giữ điều đó ở TẦNG MÃ NGUỒN (`phase1-collects-nothing.test.ts` và
`ranh-gioi-hai-nua.test.ts` §3b). Ghi lại con số này vì một lần đếm `1` mà không giải thích sẽ
làm phiên sau tưởng dây bẫy đã chết.

fetch(: 15 = 14 của `zmp-sdk` + đúng 1 của ta
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
| `scripts/cau-hinh.mjs` · `cau-hinh.test.mjs` | Đọc `.env.local` cho CẢ bước dựng lẫn bước đẩy, và **danh sách trắng** chặn bí mật đặt nhầm chỗ. 10 ca: hai chiều của danh sách trắng · shell thắng tệp · trả đúng hai khoá · và ba ca ghim rằng cái rào **thật sự được nối vào** `vite.config.ts`, `deploy.mjs`, `.env.local.example` |
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

### Câu ĐẾM số chỗ mở trang ngoài — không còn gõ tay (21/09/2026, tối)

Mục "Chuyển dữ liệu cho bên thứ ba" khai *"Có **năm** chỗ ứng dụng mở một trang bên ngoài"* rồi
liệt kê đủ năm. Con số ấy đã phải sửa **bốn lần trong hai ngày** (ba → hai → ba → năm), lần nào
cũng do một **người** đọc lại văn bản mà phát hiện, không lần nào do một phép kiểm.

Nên nó thôi là một con số gõ tay:

| Vế | Ở đâu |
|---|---|
| Danh sách đích đến, và quy ước **đếm theo ĐÍCH chứ không theo số NÚT** | `src/content/dich-ra-ngoai.ts` |
| Câu trong chính sách, **dựng ra** từ danh sách ấy | `cauKhaiDichRaNgoai()` |
| Cửa **duy nhất** ra ngoài — chỉ nhận một mã đích đã khai | `src/features/tinh-nang/mo-ra-ngoai.ts` |
| Lệnh cấm mọi đường đi vòng: `moTrangWeb(` gọi thẳng · `target="_blank"` · `window.open(` · gán `location.href` | `src/content/dich-ra-ngoai.test.ts` |

Năm đích hôm nay: **bản đồ** · **trang web trên mã QR vừa quét** · **trang chủ của chúng tôi** ·
**bài viết trên trang tin** · **cửa sổ trò chuyện với Official Account**. Hai neo website (màn Liên
hệ và trang chi tiết giải pháp) là **một** đích, một dòng.

⚠ Hai neo ấy đã đổi từ `<a target="_blank">` sang **nút đi qua `openWebview`**: bên trong Zalo một
liên kết mở bằng thẻ `a` không có đường quay lại Mini App. Hệ quả nói thẳng — ngoài Zalo (trình
duyệt máy tính) nút ấy không mở được, và màn hình nói ra bằng đúng câu mọi chỗ mở ngoài khác dùng.

⚠ **PHIÊN BẢN VẪN LÀ `1.0`** dù bề mặt "dữ liệu của bạn có thể tới đâu" vừa rộng ra thật sự: văn
bản chưa từng tới tay một người dùng nào (bảng bằng chứng ngay trên). Từ lần công bố đầu trở đi,
đúng thay đổi này phải lên một số mới.

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

### Còn thiếu — việc vận hành, câu hỏi cho khách hàng, và phép thử phải chạy thật

| Thiếu | Vì sao chưa điền |
|---|---|
| ⚠ **LỊCH CHẠY HẰNG NGÀY CHO `nhat_ky_don_qua_han()`** | Trần 90 ngày chỉ đúng **nếu hàm dọn chạy hằng ngày**: khoảng cách giữa hai lần chạy **cộng thẳng** vào tuổi của dòng cũ nhất, nên cron hằng tuần biến trần 90 thành 97 — đúng cái vừa sửa. Kho backend **ghi rõ yêu cầu ấy** (`make don-nhat-ky`, README §cron) nhưng **không chứa một định nghĩa lịch nào** — không cron file, không systemd timer, không job nền tảng. Cam kết trong một văn bản pháp lý đang phụ thuộc vào việc có người nhớ gõ lệnh |
| **Quy trình nhận yêu cầu xoá** | Lệnh `an-danh` **chạy tay** bởi người tiếp nhận — cố ý, vì một tuyến công khai xoá theo số điện thoại là tuyến xoá dữ liệu người khác. Nhưng "ai trực hotline/email, trả lời trong bao lâu, ghi số phiếu ở đâu" thì chưa ai mô tả. Một cam kết pháp lý không có quy trình đằng sau là một cam kết sẽ lỡ |
| **Phép gọi thật tới `vihat-miniapp`** | Backend đang dựng song song, chưa có địa chỉ để gọi. Năm nhánh kết quả đã có test bằng `fetch` giả, nhưng **chưa một lần nào chạm máy chủ thật**. Phải gọi thử một lần — đủ cả 201, 401, 502 — trước khi nộp |
| **Mã số thuế**, **người đại diện theo pháp luật** | Không có nguồn. `content/company-profile.ts` chỉ chứa thứ đã công bố trên vihatsoftware.com và vihatgroup.com. Bịa hai trường này trong một văn bản pháp lý là thứ không sửa lại được sau khi nộp |
| ⚠ **KHÁCH XÁC NHẬN CÂU ĐỊNH VỊ TIẾNG VIỆT** | Ba chuỗi thương hiệu đã điền từ `vihatgroup.com` ngày 21/09/2026 và `company-profile.test.ts` ghim cả ba **nguyên văn**. Nhưng một trong ba là một quyết định **thay khách**: trường `positioning` từng là tiếng Anh (câu của công ty con), tập đoàn **không công bố câu định vị tiếng Anh nào**, nên chỗ ấy nay là câu hero tiếng Việt của trang "Về ViHAT". Đánh dấu tại chỗ trong `company-profile.ts`. Khách bác thì sửa đúng một hằng |
| **Bản mẫu PM vs nguồn thật — bốn chỗ đã làm theo NGUỒN** | (1) "15 giải pháp, 3 nhóm nghiệp vụ" **không tồn tại** trên cả hai trang — số 15 là số **dự án** của công ty con; app dựng theo cấu trúc thật (4 dòng hệ sinh thái + 6 đơn vị thành viên). (2) Dải lịch sử bắt đầu **2013**, không phải 2012 — ngày thành lập là 06/12/2013, và một mốc trước ngày ấy là khẳng định sai về một pháp nhân. (3) **Không có mục AI nào**: toàn bộ nguồn về AI là MỘT câu gắn OMICall + một ô logo + một bài blog 2023; muốn hơn thì phải mở `omicall.com` và đó là quyết định của chủ dự án. (4) "12 năm" **không ghi cứng** — tính từ ngày thành lập. ⚠ Câu mô tả nguyên văn của khách vẫn mở đầu bằng "Hơn 12 năm", nên **từ 06/12/2026 con số tính ra (13) sẽ lệch với con số trong câu trích (12)**: chỗ sửa là khách công bố lại đoạn ấy, không phải mã sửa lời khách |
| **Ô tìm kiếm trên màn Giải pháp** | Bản mẫu yêu cầu, **chưa làm**, và không nên làm bằng cách hiện tại: một ô tìm kiếm là một `<input>`, mà `phase1-collects-nothing.test.ts` cấm `<form\|input\|textarea\|select>` ở **mọi tệp**. Nới lệnh cấm ấy cho một ô lọc trên **bốn** mục là trả một giá không tương xứng. Cần ô tìm kiếm thật thì phải đi kèm quyết định: thu hẹp lệnh cấm ấy thế nào, và ca kiểm nào chứng minh nó còn bắt ở ngoài phạm vi mới |
| **Màn "Quản lý quyền" không phải tab thứ sáu** | Đã đo, không đoán: `accessibility.test.ts` đo thanh tab trên máy 320px; với sáu tab mỗi nhãn chỉ còn ~4 ký tự cho từ dài nhất, mà "Trang", "thiếp", "ViHAT", "Quyền" đều 5. Nên nó là **màn con của tab Liên hệ**, ngay trên chính sách quyền riêng tư |
| ⚠ **AI VẬN HÀNH `vihat-miniapp` SAU KHI APP ĐỔI CHỦ** | Chính sách khai nơi nhận dữ liệu đăng nhập là "máy chủ của VihatSoftware" — **giữ nguyên, cố ý**. Nhưng bên phát hành app nay là ViHAT Group, nên hai câu cũ *"bên phát hành ứng dụng này, không phải một bên thứ ba"* đã phải gỡ: vế đầu thành sai, vế sau là kết luận pháp lý dựa trên vế đầu. **Chủ dự án phải trả lời trước lần công bố đầu tiên**; nếu bên vận hành là một pháp nhân khác bên phát hành thì văn bản còn nợ một mục khai chuyển dữ liệu cho bên thứ ba — không ai được tự viết mục ấy |
| **URL trang chính sách** | Developer Console còn một ô URL ngoài bản trong app. Chưa biết đăng ở đâu, nên chưa dựng bộ sinh trang tĩnh — dựng cho một đích chưa biết là đoán. Khi chốt, trang ấy phải sinh ra TỪ `chinh-sach-rieng-tu.ts`, không chép tay, để trang đăng và app không lệch nhau |
| ⚠ **ID CỦA OFFICIAL ACCOUNT TRONG NÚT CHAT — CHƯA ĐỐI CHIẾU CONSOLE** | Nút Chat nổi mở `https://zalo.me/<OA id>`, và con số ấy lấy **từ bản mẫu giao diện của PM** (`content/dich-ra-ngoai.ts`, `OA_NEN_TANG_ID`, kèm `OA_NEN_TANG_NGUON` nói rõ điều này). ADR 0018 §Hệ quả điểm 3: **mọi bề mặt OA bên trong Mini App phải trỏ về OA NỀN TẢNG** (`Vihat` sau ADR 0031), không trỏ về OA của xã nào. Nếu ID ấy không phải OA xác thực của Mini App thì người dùng bấm "quan tâm" và tưởng đã theo dõi đúng nơi. **Phải mở Developer Console đối chiếu một lần trước khi nộp** |
| **Hai mục của bản mẫu KHÔNG được dựng: `Brochure` và `24/7`** | Dải menu của bản mẫu có tám mục; app dựng **sáu**. Sau hai cái tên ấy **không có tính năng nào** trong kho này và chưa ai nói chúng mở ra cái gì. Một nút không dẫn đi đâu là thứ người duyệt bấm vào đầu tiên. Có người nói rõ chúng làm gì thì thêm lại — chỗ thêm là `MUC_MENU_NHANH` trong `features/company-intro/MenuNhanh.tsx`, và ca "mỗi mốc là một `id` có thật" sẽ bắt nếu đích chưa tồn tại |
| **Mục "Tư vấn" gọi hotline, không mở biểu mẫu** | Bản mẫu có màn "Đăng ký nhận tư vấn"; màn ấy **không được dựng** theo yêu cầu, và chưa ai quyết một biểu mẫu sẽ gửi dữ liệu đi đâu (một `<input>` cũng phá dây bẫy `phase1-collects-nothing`). Mục menu vì thế là một neo `tel:` tới đúng hotline đã công bố, và **nói ra điều đó ngay trên nút** ("Tư vấn / Gọi hotline"). Ngày có người quyết đích đến thật của "Tư vấn", sửa đúng một dòng trong `MUC_MENU_NHANH` |
| **Khối "Tin ViHAT" là ẢNH CHỤP, không phải tin trực tiếp** | Hai bài đọc từ `vihatgroup.com/wp-json/wp/v2/posts` ngày **21/09/2026** và nằm cứng trong `src/content/tin-tuc.ts`. App **không gọi mạng** để lấy tin: giai đoạn 1 có đúng MỘT đích mạng (tuyến đăng nhập), và thêm một đích là **sửa một lời khai trong chính sách quyền riêng tư** — một thay đổi pháp lý, không phải một tính năng. Màn hình nói ra ngày chụp; cách cập nhật ghi ở đầu tệp ấy, và có ca kiểm buộc ngày chụp không sớm hơn bài mới nhất |
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

⚠ **Cả bốn đều cần `VIGOV_API_HOST`**, vì khối đăng nhập đọc địa chỉ máy chủ lúc dựng. Cách
thường dùng là điền nó một lần vào `.env.local` (§"`.env.local` — cấu hình cho máy đẩy bản");
đè cho đúng một lần chạy thì đặt biến shell, nó thắng tệp:

```bash
npm run zmp:phat-hanh:goc                              # đọc .env.local
VIGOV_API_HOST=https://<host> npm run zmp:phat-hanh:goc # đè tệp, cho một lần chạy
```

Thiếu cả hai thì script **dừng với mã thoát 2 trước khi dựng gì cả** — đẩy một bản chưa khai địa
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
| 1 | Ảnh chụp màn hình và mô tả trên store | Bắt buộc để duyệt. Icon thì đã có — `tools/logo.py` dựng từ `brand/lg_vhs_full.svg`. ⚠ Bảy ảnh trong `tmp/xin-quyen-zalo/anh/` chụp **18/09**, tức trước cả lần dựng lại giao diện 21/09 chiều **và** trước bốn khối mới của màn chủ + nút Chat nổi (21/09 tối) — phải chụp lại **toàn bộ**, không chỉ ảnh màn đăng nhập |
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
