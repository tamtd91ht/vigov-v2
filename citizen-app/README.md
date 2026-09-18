# citizen-app

The citizen-facing **Zalo Mini App**. React + Vite + zmp-sdk.

Citizens are identified by phone plus OTP — a **weak identity**, not to be trusted. And they
**do not choose** this software: being unable to use it means being unable to reach a public
service. That makes accessibility a rights question, not a preference.

## Two phases, ONE App ID

A Mini App is identified by its App ID, and an App ID is what Zalo reviews and what a
verifying Official Account is bound to. That single identifier is why this app ships in two
phases instead of two apps.

| Phase | What ships | Why |
|---|---|---|
| **1 — now** | A static introduction to **VihatSoftware**, the company that publishes the app | This is the submission Zalo reviews, so the OA `VihatSoftware` can verify the App ID |
| **2 — next** | The commune / citizen surface | It lands on the **same App ID**, already verified |

**Nothing of phase 2 gets deleted to make room for phase 1.** `src/lib/commune-resolution.ts`
is the foundation phase 2 builds on and stays untouched.

Why the verifying OA is VihatSoftware and not a commune, and why the notification OA is a
different OA per commune: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`. How the
commune gets resolved at runtime with no domain to key off:
`kb/10-decisions/0005-miniapp-tenant-resolution.md`. Both are read there, not repeated here.

### Phase 1 collects no personal data — keep it that way

No sign-in, no `getPhoneNumber`, no OTP, no form, no backend call, no commune logic, no
`tenant_id`. The only outbound links are `tel:`, `mailto:` and the company website.

That is a deliberate design choice, not an accident of scope. It makes rule 3 (personal data)
hold **by construction** rather than by argument, and it gives the Zalo review nothing to
weigh: an app that asks for nothing has no permission to justify. Adding any collection to
phase 1 changes what was submitted for review — raise it before writing it.

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

Rules 6, 7 and 8 already bind phase 1. Rules 1–5 bind phase 2 — and rules 1, 2 and 3 have already
been **built and demonstrated**, then deliberately kept out of the submitted bundle. See
§"Giai đoạn 2 bắt đầu từ đâu" below: that is the one place this repository records where that
work lives.

## Error message shape

| Wrong | Right |
|---|---|
| `Error 422: Validation failed` | "Số điện thoại chưa đúng. Nhập 10 số, bắt đầu bằng 0." |
| `Unauthorized` | "Phiên đăng nhập đã hết. Đăng nhập lại để tiếp tục." |

## Where the content lives

Every user-visible string of the company introduction sits in `src/content/company-profile.ts`.
One file, because phase 2 replaces this content wholesale and the edit should land in one place.
The submitted app contains no other user-visible string: no launch-parameter reading, no
diagnostics panel, no commune screens.

**Nothing may be added to that file without a source.** The app carries the name of a real
legal entity: an unsourced founding year, customer name or award is a false statement
published under that name. Facts that are missing are left out, never filled in.

## Nộp lên Zalo

### Chuỗi lệnh

```bash
npm run zmp:login     # một lần, cần App ID
npm run zmp:deploy    # build -> sync-config -> deploy bản thử nghiệm
```

`zmp:deploy` gộp ba bước vì **bỏ sót bước giữa là nộp một app trắng trơn**. Zalo không dùng
`index.html` của chúng ta: nó tự dựng vỏ rồi nạp đúng những tệp khai trong `app-config.json`,
và `zmp-cli sync-config` là thứ điền danh sách ấy từ trang đã dựng. Dựng xong mà quên đồng bộ
thì `app-config.json` trỏ vào bản dựng của lần trước.

**Không hỏi câu nào.** CLI vốn dừng ba lần — *"This is not a ZMP Project?"*, *"where is your
dist folder"*, *"description"* — và cả ba đã tắt bằng `-e`, `-o dist`, `-m`, cộng `-p`.

Mô tả phiên bản **sinh theo từng lần đẩy**, không cố định: `<sha ngắn> · <ngày giờ>`, cộng
`dirty` khi cây làm việc còn thay đổi chưa commit. Một nhãn cố định thì mọi bản trong console
Zalo trông như nhau và lúc cần biết *"bản đang chạy là bản nào"* thì không còn gì để tra; còn
`dirty` nói ra rằng bản ấy **không ứng với commit nào** — không ai dựng lại được nó, kể cả
người vừa đẩy. Tính trong `scripts/deploy.mjs` chứ không trong `package.json`, vì trên Windows
`npm run` chạy qua `cmd`, nơi `$(git rev-parse …)` chỉ là một chuỗi ký tự.

`-t` là **bản thử nghiệm**. Bỏ `-t` là đẩy bản phát hành — và nó cố ý **không** có cờ trong
script: đẩy bản phát hành phải là một quyết định có người gõ ra, không phải mặc định của một
lệnh chạy tự động.

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
| `location.search` mang **đúng** những gì `getRouteParams()` mang | Đo trên Version 6, mở nguội qua deep link. Nghĩa là `zmp-sdk` **bỏ được** — nó tốn **256 kB thô / 64 kB gzip**, gần một nửa bundle. **ĐÃ GỠ KHỎI MÃ NGUỒN** cùng với lớp khám phá: bản nộp dựng ra **245,86 kB thô / 75,67 kB gzip**, so với **513,20 / 142,51** khi còn nhập SDK. Gói vẫn nằm trong `package.json` vì ADR 0020 chốt `getPhoneNumber` là đường đăng nhập của giai đoạn 2 — một phụ thuộc **không được nhập** thì không vào bundle và không tốn gì, và `phase1-collects-nothing.test.ts` là thứ giữ cho nó không được nhập |

### Còn thiếu

| # | Cái gì | Ghi chú |
|---|---|---|
| 1 | Ảnh chụp màn hình và mô tả trên store | Bắt buộc để duyệt. Icon thì đã có — `tools/logo.py` dựng từ `brand/lg_vhs_full.svg` |
| 2 | Các khoá còn lại trong `app-config.json` | `app.*` viết từ nguồn thứ cấp và **chưa đối chiếu** với Developer Console. Ba khoá `list*` thì đã do `sync-config` sinh, không phải đoán |

## Giai đoạn 2 bắt đầu từ đâu — KHÔI PHỤC, đừng dựng lại

Lớp khám phá của ADR 0005 **đã được dựng, chạy được và demo trước lãnh đạo**. Nó nằm ở commit
**`25591e8`** (`feat(citizen-app): màn xác nhận xã và màn chọn xã`) và bị gỡ khỏi bản nộp ngay
sau đó — **gỡ, không phải bỏ**. Ai bắt đầu giai đoạn 2 thì `git show 25591e8` trước khi viết
dòng đầu tiên.

| Ở `25591e8` có sẵn | Việc nó làm |
|---|---|
| `src/features/kham-pha/goi-y.ts` | Mức tin theo nguồn, hàm thuần, có test |
| `src/features/kham-pha/GoiYXaScreen.tsx` | Màn xác nhận: tên xã to, một chạm đồng ý |
| `src/features/kham-pha/ChonXaScreen.tsx` | Danh mục xã, mỗi dòng một nút ≥44px, **không ô nhập** |
| `src/features/kham-pha/kham-pha.test.tsx` | 31 ca: mức tin, fail-closed, tên xã trên mọi màn hình |
| `src/lib/launch-params.ts` · `src/features/diagnostics/` | Đọc tham số mở app, và bảng đo in ra máy thật |

**Ba thứ đã đo được, đừng đi đo lại** — chi tiết và bằng chứng nằm ở §"Hai thứ đã kiểm bằng cách
chạy thật" phía trên, đây chỉ trỏ tới:

1. **Khuôn link bản thử nghiệm**, và tham số riêng (`t`, `src`, `debug`) **đi tới được app**.
2. **`location.search` mang đúng những gì `getRouteParams()` mang** — nghĩa là lớp khám phá chạy
   được mà không cần `zmp-sdk`, và 64 kB gzip kia là tiền không phải trả.
3. **Mức tin theo nguồn đã cài và demo được**: `qr`/`zns` chọn sẵn một chạm xác nhận, `share` và
   không khai nguồn thì **luôn** bắt chọn tường minh.

**Vì sao gỡ khỏi bản nộp** (cả ba lý do đều hết hiệu lực khi giai đoạn 2 bắt đầu):

| # | Lý do |
|---|---|
| 1 | Tám **tên đơn vị hành chính đặt ra** không nên nằm trong một app xuất bản dưới tên một pháp nhân thật |
| 2 | Màn dịch vụ công trong một app đang duyệt theo diện **hồ sơ doanh nghiệp** mời đúng câu hỏi "nhóm ngành đặc thù" mà ADR 0018 còn treo |
| 3 | Gỡ luôn `zmp-sdk`: **513,20 → 245,86 kB** thô, **142,51 → 75,67 kB** gzip |

**Hai thứ phải đổi khi khôi phục, không được bê nguyên:**

1. Danh mục xã mẫu (`demo-danh-muc-xa.ts`) phải thay bằng **`ListTenants`** của service
   `platform` — đã khai trong `proto/vigov/platform/v1/platform.proto`, **chưa có cài đặt**, và
   còn chờ xác thực người gọi trên cổng gRPC (đọc chú thích trong chính tệp proto ấy).
2. Xã đã xác nhận ở `25591e8` là **trạng thái giao diện phía client**, không phải phiên. ADR 0005:
   xã của phiên do **máy chủ** ghi sau khi công dân xác nhận. Thay bằng xã đọc từ phiên — **không
   phải "đồng bộ thêm" với nó**: hai nguồn cho một sự thật thì một trong hai sẽ cũ, và cái cũ là
   cái đi vào hồ sơ gửi nhầm cơ quan.

`src/lib/commune-resolution.ts` **chưa bao giờ bị gỡ** và chưa bao giờ được nối vào — nó vẫn là
bộ khung giai đoạn 2 dựng lên.

## Lệnh

| Lệnh | Việc |
|---|---|
| `npm run dev` | Máy chủ phát triển Vite |
| `npm run build` | Gói tĩnh vào `dist/` |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest — ghim các sự thật đã công bố, hình dạng bundle và sổ màn hình |
| `npm run zmp:sync` | Dựng rồi đồng bộ `app-config.json` theo trang đã dựng |
| `npm run zmp:deploy` | Như trên, rồi đẩy bản thử nghiệm |

→ Skills: `.claude/skills/zalo-miniapp-multi-tenant` · `.claude/skills/accessibility-elderly`
