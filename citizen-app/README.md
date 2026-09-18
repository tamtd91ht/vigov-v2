# citizen-app

The citizen-facing **Zalo Mini App**. React + Vite. **`zmp-sdk` is a dependency nothing imports**
— see §"Hai thứ đã kiểm bằng cách chạy thật", and the tripwire in
`src/phase1-collects-nothing.test.ts` that keeps it that way.

Citizens are identified by phone plus OTP — a **weak identity**, not to be trusted. And they
**do not choose** this software: being unable to use it means being unable to reach a public
service. That makes accessibility a rights question, not a preference.

## Two phases, ONE App ID

A Mini App is identified by its App ID, and an App ID is what Zalo reviews and what a
verifying Official Account is bound to. That single identifier is why this app ships in two
phases instead of two apps.

| Phase | What ships | Why |
|---|---|---|
| **1 — now** | A static introduction to **VihatSoftware**, the company that publishes the app, **plus the discovery layer**: opened with a `t` parameter, the app asks the citizen to confirm a commune | This is the submission Zalo reviews, so the OA `VihatSoftware` can verify the App ID |
| **2 — next** | The commune / citizen surface behind a real session | It lands on the **same App ID**, already verified |

**The discovery layer is a secondary view of the SAME app, never a second app.** A second App ID
would need its own review, so "hiding the real app behind another App ID" buys nothing and costs
a second approval. Opened with no parameter — the ordinary way, and the only way from the second
launch onward — the app is exactly the introduction Zalo reviewed.

**Nothing of phase 2 gets deleted to make room for phase 1.** `src/lib/commune-resolution.ts`
is the foundation phase 2 builds on and stays untouched.

Why the verifying OA is VihatSoftware and not a commune, and why the notification OA is a
different OA per commune: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`. How the
commune gets resolved at runtime with no domain to key off:
`kb/10-decisions/0005-miniapp-tenant-resolution.md`. Both are read there, not repeated here.

### Phase 1 collects no personal data — keep it that way

No sign-in, no `getPhoneNumber`, no OTP, no form, no input control of any kind, no backend call,
no device storage, no geolocation. The only outbound links are `tel:`, `mailto:` and the company
website.

**The discovery layer changes none of that.** It reads a parameter the platform already put in
the URL before our code ran, it holds the confirmed commune in `useState` — lost when the app
closes, never written to the device, never sent anywhere — and its commune list is a constant in
the bundle. Nothing about a citizen is collected, and `src/phase1-collects-nothing.test.ts`
sweeps the whole source tree to keep it so.

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

Rules 6, 7 and 8 already bind phase 1. **Rules 1, 2 and 3 are live in the submitted app**, in
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
belong to, and every commune name it shows in **exactly one file** — see §Còn thiếu #0.

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
| `location.search` mang **đúng** những gì `getRouteParams()` mang | Đo 18/09/2026 trên Version 6, mở nguội qua deep link: cả tham số nền tảng (`env`, `version`) lẫn tham số riêng (`t`, `src`, `debug`) đều có mặt. Nên lớp khám phá đọc tham số **chỉ từ `location.search`**, và `zmp-sdk` **không còn tệp nào nhập**: bản nộp dựng ra **256,14 kB thô / 78,20 kB gzip**, so với **513,20 / 142,51** khi còn nhập SDK — gần một nửa. Chế độ hỏng nếu phép đo sai ở một đường mở nào đó là chế độ hỏng **lành**: không có tham số ⇒ app giới thiệu bình thường ⇒ công dân tự chọn xã, đúng đường ADR 0005 bắt buộc phải chạy được. Gói vẫn nằm trong `package.json` vì ADR 0020 chốt `getPhoneNumber` là đường đăng nhập của giai đoạn 2 — một phụ thuộc **không được nhập** thì không vào bundle và không tốn gì, và `phase1-collects-nothing.test.ts` là thứ giữ cho nó không được nhập |

### Còn thiếu

| # | Cái gì | Ghi chú |
|---|---|---|
| 0 | **Xoá danh mục xã mẫu** — `src/features/kham-pha/demo-danh-muc-xa.ts` — trước khi kênh công dân phục vụ người thật | Tám tên đơn vị hành chính **đặt ra**, kèm dòng chữ *"Danh mục mẫu dùng cho bản trình diễn"* hiện ngay dưới danh sách. Chúng đi vào bản **xuất bản**, nên dòng chữ ấy càng bắt buộc: một cơ quan nhà nước hiển thị đơn vị hành chính không tồn tại là một sự cố. Nguồn thật là `ListTenants` của service `platform` — đã khai trong proto, **chưa có cài đặt**, còn chờ xác thực người gọi trên cổng gRPC. Ba ca trong `kham-pha.test.tsx` ghim rằng đây là nơi **duy nhất** có tên đơn vị hành chính, để xoá một tệp là xoá sạch |
| 1 | Ảnh chụp màn hình và mô tả trên store | Bắt buộc để duyệt. Icon thì đã có — `tools/logo.py` dựng từ `brand/lg_vhs_full.svg` |
| 2 | Các khoá còn lại trong `app-config.json` | `app.*` viết từ nguồn thứ cấp và **chưa đối chiếu** với Developer Console. Ba khoá `list*` thì đã do `sync-config` sinh, không phải đoán |

## Lớp khám phá — cái gì đang chạy, và cái gì còn thiếu

Lớp KHÁM PHÁ của ADR 0005 **nằm trong bản nộp**. Ba lớp của ADR 0005 vẫn tách rời, và đây là
bảng phải đọc trước khi sửa bất cứ thứ gì trong `src/features/kham-pha/`:

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
| `src/features/kham-pha/demo-danh-muc-xa.ts` | Danh mục **tạm** — §Còn thiếu #0 |
| `src/features/kham-pha/kham-pha.test.tsx` | 31 ca: mức tin, fail-closed, tên xã trên mọi màn hình |
| `src/lib/launch-params.ts` · `src/features/diagnostics/` | Đọc `location.search`, và bảng đo chỉ hiện khi có `debug` |

**Mức tin theo nguồn** — đây là quy tắc, không phải giao diện:

| `src` | Hành vi |
|---|---|
| `qr` · `zns` | Chọn sẵn, một chạm xác nhận. Người đang đứng ở trụ sở xã không nên bị bắt đi tìm lại chính xã đó |
| `share` · không khai · tra mã không ra | **Luôn** bắt chọn tường minh — liên kết chuyển tay không nói lên ý định người nhận |

**HAI THỨ PHẢI ĐỔI KHI CÓ MÁY CHỦ, và cả hai đều không phải việc sửa giao diện:**

1. Danh mục xã mẫu (`demo-danh-muc-xa.ts`) thay bằng **`ListTenants`** của service `platform` —
   đã khai trong `proto/vigov/platform/v1/platform.proto`, **chưa có cài đặt**, và còn chờ xác
   thực người gọi trên cổng gRPC (đọc chú thích trong chính tệp proto ấy).
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
| `npm run dev` | Máy chủ phát triển Vite |
| `npm run build` | Gói tĩnh vào `dist/` |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest — ghim các sự thật đã công bố, hình dạng bundle và sổ màn hình |
| `npm run zmp:sync` | Dựng rồi đồng bộ `app-config.json` theo trang đã dựng |
| `npm run zmp:deploy` | Như trên, rồi đẩy bản thử nghiệm |

→ Skills: `.claude/skills/zalo-miniapp-multi-tenant` · `.claude/skills/accessibility-elderly`
