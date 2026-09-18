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

Rules 6, 7 and 8 already bind phase 1. Rules 1–5 bind phase 2 — and rules 1, 2 and 3 are already
**running**, on demo data, in `src/features/kham-pha/`: the deep-link parameter only ever
suggests a commune, the header slot in `src/App.tsx` holds the commune name on every screen once
one is confirmed, and switching commune is a button nobody presses on the citizen's behalf.
Rules 4 and 5 wait for a server: the confirmed commune is client-side UI state, **not a session**
(see the note at the top of `GoiYXaScreen.tsx`).

## Error message shape

| Wrong | Right |
|---|---|
| `Error 422: Validation failed` | "Số điện thoại chưa đúng. Nhập 10 số, bắt đầu bằng 0." |
| `Unauthorized` | "Phiên đăng nhập đã hết. Đăng nhập lại để tiếp tục." |

## Where the content lives

Every user-visible string of the company introduction sits in `src/content/company-profile.ts`.
One file, because phase 2 replaces this content wholesale and the edit should land in one place.
The discovery layer (`src/features/kham-pha/`) keeps its own strings next to the rules they
belong to, and its demo commune names in exactly one file — see §Còn thiếu #0.

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
| `location.search` mang **đúng** những gì `getRouteParams()` mang | Đo trên Version 6, mở nguội qua deep link. Nghĩa là `zmp-sdk` **bỏ được** — nó tốn **256 kB thô / 64 kB gzip**, gần một nửa bundle (247,83 kB khi không có, 504,34 kB khi có). **CHƯA GỠ, có lý do:** ADR 0020 chốt `getPhoneNumber` là đường đăng nhập của giai đoạn 2, nên SDK sẽ quay lại; và mới đo **một** đường mở — chưa đo lúc quay lại từ nền và lúc mở từ app ghim. Gỡ dựa trên một phép đo là đổi nó thành một giả định |

### Còn thiếu

| # | Cái gì | Ghi chú |
|---|---|---|
| 0 | **Xoá danh mục xã mẫu** — `src/features/kham-pha/demo-danh-muc-xa.ts` | Tám dòng tên xã **đặt ra** cho buổi trình diễn, kèm một dòng chữ nói rõ điều đó trên màn hình. Nguồn thật là `ListTenants` của service `platform`: đã khai trong proto, **chưa có cài đặt**, và còn chờ xác thực người gọi trên cổng gRPC. Một danh mục xã đặt ra mà đi vào bản phục vụ người thật là một cơ quan nhà nước hiển thị đơn vị hành chính không tồn tại. `kham-pha.test.tsx` ghim rằng đây là nơi **duy nhất** có tên xã, để xoá một tệp là xoá sạch |
| 1 | Ảnh chụp màn hình và mô tả trên store | Bắt buộc để duyệt. Icon thì đã có — `tools/logo.py` dựng từ `brand/lg_vhs_full.svg` |
| 2 | Các khoá còn lại trong `app-config.json` | `app.*` viết từ nguồn thứ cấp và **chưa đối chiếu** với Developer Console. Ba khoá `list*` thì đã do `sync-config` sinh, không phải đoán |

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
