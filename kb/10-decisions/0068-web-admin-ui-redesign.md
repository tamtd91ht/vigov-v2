---
id: 0068-web-admin-ui-redesign
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d357dee6
expires: null
owns_facts:
  - "làm mới giao diện web-admin chỉ đổi phần trình bày: không đổi đường dẫn, lời gọi API, state, logic lọc/phân trang/phân quyền, tên trường, name/id ô nhập, handler, thứ tự bước nghiệp vụ; không thêm tính năng 'cho hiện đại' (chốt 02/10/2026)"
  - "nền tảng giao diện web-admin là Tailwind CSS v4 + shadcn/ui, icon lucide-react, phông Be Vietnam Pro tự phục vụ qua @fontsource (không gọi Google lúc build hay lúc chạy); màu chính #1565C0, đỏ/vàng chỉ làm điểm nhấn nhận diện; token chế độ tối chuẩn bị sẵn nhưng chưa bật"
  - "globals.css cũ nằm trong cascade layer `legacy` để tiện ích Tailwind thắng; preflight của Tailwind KHÔNG bật ở đợt 1"
  - "không dùng component shadcn vi phạm rào chắn của kho: sidebar (document.cookie, Math.random), chart (dangerouslySetInnerHTML), sonner (chèn <style> lúc chạy); component bọc button/select/input gốc, không thay chúng"
  - "khi làm mới giao diện, kỳ vọng trình bày trong test (chuỗi class, ký tự emoji, markup bao ngoài) được đổi theo; khẳng định hành vi thì không"
  - "mục menu chưa có màn hiện huy hiệu 'Chưa có' kèm tooltip 'Chưa có màn hình' — không bao giờ 'Sắp có'"
  - "topbar hiện 'Ủy ban nhân dân' làm dòng chú thích phía trên displayName của xã giữ nguyên văn — tên xã không bao giờ được ghép chuỗi"
  - "đặc tả giao diện chủ dự án cung cấp 02/10/2026 thay docs/ui-ux/15 về mặt hình thức (sidebar trắng thay navy); docs/ui-ux là bản sao yêu cầu, không sửa"
  - "làm mới giao diện được ưu tiên hơn mọi việc web-admin khác; Phản ánh đứng đầu đợt 2; đợt 1 = nền móng + Danh bạ, Tổng quan, Nhiệm vụ, Đăng nhập; đợt 2 = 9 màn còn lại + bố cục thẻ trên điện thoại + rà căn hàng/trợ năng"
  - "đặc tả giao diện v2 (02/10/2026): hiện đại = ít ma sát, không trang trí — không backdrop-blur, không gradient làm phong cách chính; quyết định đã chốt ('Chưa có', @fontsource, topbar 'Ủy ban nhân dân') thắng chỗ v2 viết khác; mục ROADMAP_PHASE2 không làm trong đợt này"
  - "mọi thanh lọc web-admin: ô tìm đứng đầu bên trái, tối đa 2 bộ lọc chính hiện sẵn, còn lại sau nút 'Bộ lọc' hiện số bộ lọc ẩn đang khác mặc định và mở sẵn khi số ấy > 0; mọi ô cao bằng nhau, nhãn trên ô (chốt 02/10/2026)"
---

# 0068. Làm mới giao diện web-admin — chỉ trình bày, Tailwind v4 + shadcn/ui

**Trạng thái:** đã chốt · **Ngày:** 2026-10-02 · **Người quyết:** chủ dự án, 02/10/2026 · **Thay**
`docs/ui-ux/15-phu-luc-giao-dien-chung.md` về **hình thức** (không thay về hành vi, xem §8)

## Bối cảnh

Chủ dự án đưa một bản đặc tả nâng cấp giao diện (UI only) cho web-admin, kèm ảnh minh hoạ và
HTML tham chiếu. Bản ấy nằm trong `tmp/`, **không được đưa vào git** — nên các điểm cốt lõi
được tóm tắt dưới đây; chi tiết thi công (token, thang chữ, bảng ánh xạ icon, quy tắc căn hàng)
sẽ sống trong mã nguồn khi dựng, không chép vào `kb/`.

Đặc tả nêu mười vấn đề của giao diện hiện tại. Nặng nhất:

| Vấn đề | Hệ quả |
|---|---|
| Ô lọc lệch nhãn (trên/trái lẫn lộn), lệch chiều cao, rải 3–4 hàng | Màn trông cẩu thả trước mặt cán bộ |
| Trang tràn ngang (bảng và thanh lọc đẩy cả trang cuộn) | Thiếu `min-width: 0` trong grid/flex, bảng không có vùng cuộn riêng |
| Không icon; quá nhiều chữ; thuật ngữ kỹ thuật và lỗi thô (`ngan_sach: …`) tới người dùng cuối | Phải đọc mới hiểu; một cơ quan công quyền hiện lỗi máy chủ ra màn |
| Phông `system-ui` | Dấu tiếng Việt hiển thị khác nhau giữa các máy |
| Chưa có bố cục điện thoại; thiếu nhận diện cơ quan nhà nước | |

Hướng đặc tả đề ra: sáng sủa, trang trọng, ít chữ nhiều icon, lưới 8px, control cao 40px, nhãn
luôn ở trên, mỗi trạng thái có icon + chữ (không bao giờ chỉ bằng màu — đặc tả đã đo cặp cam/xanh
lá không phân biệt được với người mù màu đỏ), tương phản WCAG AA.

Chủ dự án chấp nhận đặc tả, với mười quyết định dưới đây — ba trong số đó **sửa** chính đặc tả
(§2 phông, §6 nhãn mục chưa có, §7 tên xã).

## Quyết định

### 1. Chỉ trình bày

Không đổi đường dẫn, lời gọi API, state, logic lọc/phân trang/phân quyền, tên trường, `name`/`id`
ô nhập, handler, thứ tự bước nghiệp vụ. Không thêm tính năng để "trông hiện đại" — ô tìm kiếm
toàn cục, nút xoá lọc, đếm số trên menu, ẩn/hiện mật khẩu đều **không** thêm. Mọi nút và ô đang
có phải còn; được đổi dạng hiển thị (select → segmented, nút chữ → nút icon có `aria-label` +
`title`) nếu phát ra **đúng giá trị** cũ.

**Vì sao:** một đợt đổi giao diện mà đồng thời đổi hành vi thì không ai còn phân biệt được lỗi nào
do hình, lỗi nào do logic — và test hành vi (§5) mất vai trò lưới an toàn. Kho cũng có tiền lệ
từ chối vẽ chức năng không tồn tại (`web-admin/src/components/muc-menu.ts:75-77`, ô tìm kiếm
trong `dau-trang.tsx`).

### 2. Tailwind CSS v4 + shadcn/ui

Chủ dự án chọn phương án này thay vì giữ CSS thuần, **chấp nhận** thêm phụ thuộc và viết lại
markup. Hướng phong cách: hiện đại "thời đại chuyển đổi số" nhưng trang trọng.

| Mục | Chốt |
|---|---|
| Màu chính | Xanh công vụ `#1565C0` |
| Đỏ cờ / vàng sao | Chỉ làm điểm nhấn nhận diện (logo, dải trên cùng), không dùng tràn lan |
| Icon | `lucide-react`; bỏ emoji làm icon |
| Phông | Be Vietnam Pro **tự phục vụ qua `@fontsource`** — không request nào tới Google lúc build hay lúc chạy |
| Chế độ tối | Token chuẩn bị sẵn, **chưa bật** |

**Vì sao `@fontsource` chứ không `next/font/google` như đặc tả:** `next/font/google` tải phông từ
Google lúc build — bản build trên máy CI hoặc mạng nội bộ của cơ quan bị buộc vào một dịch vụ
ngoài, và mọi thay đổi phía Google thành lỗi build. Tự phục vụ thì phông là một tệp trong gói,
như mọi tài sản khác. Lý do thứ hai, cũng nằm trong phương án chủ dự án chọn: trình duyệt của cán
bộ không gửi request nào ra Google Fonts — không lộ việc truy cập hệ thống ra bên thứ ba, và hợp
với CSP chặt sau này.

**Đánh đổi đã chấp nhận:** mỗi phụ thuộc mới là thêm bề mặt cho rule 13 invariant 6 (`make vuln`);
mỗi màn viết lại markup là một lần có thể làm rơi hành vi — §5 là cái giữ chiều ấy.

### 3. CSS cũ trong layer `legacy`, preflight tắt ở đợt 1

`globals.css` hiện có được đặt trong cascade layer `legacy`, để tiện ích Tailwind luôn thắng mà
không cần `!important`. Preflight (reset) của Tailwind **không bật** ở đợt 1.

**Vì sao:** preflight đặt lại kiểu của mọi phần tử trên mọi trang — bật nó khi chín màn đợt 2 chưa
chuyển là đổi giao diện của chính những màn chưa ai đụng tới, ngoài phạm vi và ngoài test.
Bật preflight là việc của lúc màn cuối cùng đã chuyển.

### 4. Component shadcn vi phạm rào chắn thì không dùng

| Component | Vi phạm | Rào chắn |
|---|---|---|
| `sidebar` | Ghi `document.cookie` (cookie do JS ghi không thể `HttpOnly`); `Math.random` cho skeleton | Rule 13 invariant 4; rule 13 cấm #2 (`security_guard` chặn `Math.random`) |
| `chart` | `dangerouslySetInnerHTML` | Rule 13 cấm #3 |
| `sonner` | Chèn `<style>` lúc chạy | CSP nghiêm sau này (xem §Hệ quả) |

Component của kho **bọc** `button`/`select`/`input` gốc, không thay chúng bằng phần tử tự dựng.

**Vì sao bọc:** phần tử gốc mang sẵn hành vi bàn phím, trợ năng, submit form và giá trị mà test
hành vi đang khẳng định; thay nó là đổi hành vi (vi phạm §1) dưới vỏ một thay đổi hình.

### 5. Test: kỳ vọng trình bày được đổi, khẳng định hành vi thì không

Chuỗi class, ký tự emoji, markup bao ngoài trong test được phép đổi cùng thay đổi hình. Khẳng định
hành vi — giá trị phát ra, lời gọi, điều hướng, quyền, nội dung nghiệp vụ hiển thị — **không được
đổi** để test xanh lại.

**Vì sao:** đặc tả đòi "test hiện có pass 100%", nhưng một số test đang khẳng định đúng cái đợt này
phải đổi (ví dụ emoji `🗑`). Không có ranh giới này thì hoặc không làm được việc, hoặc "sửa test
cho xanh" lan sang cả hành vi — chính kiểu phép kiểm xanh sai lý do mà kho này đã trả giá nhiều lần.

### 6. Mục menu chưa có màn: "Chưa có", không "Sắp có"

Gộp dòng "Chưa có màn hình" vào chính mục đó dưới dạng huy hiệu **"Chưa có"**, tooltip **"Chưa có
màn hình"**. Mục vẫn không bấm được, không có liên kết. Đặc tả đề xuất "Sắp có" — **bị bác**.

**Vì sao:** "Sắp có" là một lời hứa về thời gian, và một cơ quan công quyền không hứa điều chưa ai
cam kết. Vì sao các mục này vẫn hiện thay vì bị xoá (nhận xét của chủ dự án 23/09/2026): xem
`web-admin/src/components/muc-menu.ts:72-91` — không lặp lại ở đây.

### 7. Topbar: "Ủy ban nhân dân" làm chú thích, tên xã nguyên văn

Topbar hiện **"Ủy ban nhân dân"** làm dòng chú thích nhỏ phía trên `displayName` của xã, và
`displayName` được hiện **nguyên văn**. Tên xã **không bao giờ** được ghép chuỗi (không
`"UBND " + …`, không viết HOA, không thêm "xã"). Đặc tả minh hoạ "UBND xã Thăng Bình" — hình thức ấy
**không** được dựng bằng cách ghép.

**Vì sao:** sai tên một đơn vị hành chính là sự cố có người phải chịu trách nhiệm (CLAUDE.md, điểm
4). Đơn vị có thể là xã, phường hay đặc khu; tên đã sáp nhập có thể đã mang sẵn tiền tố — ghép
chuỗi sẽ sai ở đúng những xã đó, lặng lẽ.

### 8. Đặc tả mới thay docs/ui-ux/15 về hình thức

Về hình thức, đặc tả 02/10/2026 thắng `docs/ui-ux/15-phu-luc-giao-dien-chung.md`: sidebar **trắng**
thay nền navy (`:58`), thu gọn 72px thay dải ~64px (`:60`, `:172`). `docs/ui-ux/` là **bản sao
yêu cầu** của khách hàng và **không sửa** — người đọc gặp mâu thuẫn hình thức giữa hai bên thì ADR
này là câu trả lời. Về hành vi (quyền, tìm kiếm, thông báo…), `docs/ui-ux/15` không bị ADR này
thay.

### 9. Phối hợp giữa các phiên

Làm mới giao diện **được ưu tiên** hơn mọi việc web-admin khác. Phiên khác tạm dừng web-admin trong
lúc đợt 1 dựng nền móng, rồi viết theo token và component mới. **Phản ánh** đứng đầu đợt 2.

**Vì sao:** hai phiên cùng viết markup web-admin theo hai hệ kiểu là hai bản sẽ phải viết lại; tạm
dừng ngắn rẻ hơn hợp nhất về sau.

### 10. Hai đợt

| Đợt | Phạm vi |
|---|---|
| 1 | Nền móng (token, phông, icon, khung sidebar/topbar, bộ component dùng chung, chặn tràn ngang) + Danh bạ, Tổng quan, Nhiệm vụ, Đăng nhập |
| 2 | 9 màn còn lại — 8 màn có trên menu (Phản ánh trước, rồi Biên bản họp, Văn bản & Đơn thư, Giải ngân, Thu – Chi, Thông báo, Nội dung Mini App, Cấu hình) và trang đổi mật khẩu (`mat-khau`) — + bố cục thẻ trên điện thoại + rà căn hàng và trợ năng |

### 11. Đặc tả v2: "hiện đại = ít ma sát, không phải trang trí"

Chủ dự án đưa bản đặc tả thứ hai cùng ngày 02/10/2026 (`UI_UPGRADE_SPEC.md` v2 + `ROADMAP_PHASE2.md`).
Bản v2 thắng bản đầu ở chỗ hai bản khác nhau: nền phẳng sáng, bóng rất nhẹ, **không** kính mờ
(backdrop-blur), **không** gradient làm phong cách chính; dòng bảng 48px, tiêu đề bảng dính; thao
tác phụ vào menu "⋯", thao tác quan trọng luôn có chữ; mỗi màn có đủ trạng thái đang tải / rỗng /
rỗng theo bộ lọc / lỗi + Tải lại / không có quyền — chỉ những trạng thái mã nguồn đã phân biệt được.
Ba chỗ v2 khác quyết định đã chốt thì **quyết định đã chốt thắng**: huy hiệu "Chưa có" (§6, v2 viết
"Sắp có"), phông `@fontsource` (§2, v2 viết `next/font`), topbar "Ủy ban nhân dân" + tên xã nguyên
văn (§7). Các mục của `ROADMAP_PHASE2.md` (tìm kiếm toàn hệ thống, lịch sử thao tác, xuất Excel,
chọn nhiều dòng, ẩn/hiện cột, tổng số bản ghi, trợ lý AI, biểu đồ, chế độ tối…) **không** làm trong
đợt này — giao diện chỉ chừa chỗ (khoảng giữa topbar, khe phải của Toolbar, khe metadata của
PageHeader, mọi màu qua token).

**Vì sao:** đây là cơ quan nhà nước; hiệu ứng trang trí làm mất vẻ trang trọng mà không bớt việc
nào cho cán bộ.

### 12. Thanh lọc: ô tìm bên trái, chỉ hiện bộ lọc chính, còn lại sau nút "Bộ lọc"

Chủ dự án chốt 02/10/2026, sau khi xem thanh lọc sổ văn bản đến (ô cao thấp lệch nhau, ô tìm nhảy
lên trên): áp dụng cho **mọi** thanh lọc của web-admin.

| Quy tắc | Nội dung |
|---|---|
| Hàng chính | Ô tìm kiếm **đứng đầu, bên trái**, giãn rộng; tiếp theo tối đa 2 bộ lọc quyết định nhất (năm/kỳ của sổ, rồi trạng thái); nút "Bộ lọc" ở cuối hàng |
| Căn hàng | Mọi ô cao bằng nhau (40px), nhãn đặt trên ô cùng một kiểu, cạnh dưới thẳng hàng |
| Phần mở rộng | Các bộ lọc còn lại xếp lưới cột đều, mở bằng nút "Bộ lọc" |
| Bộ lọc ẩn đang dùng | Nút ghi "Bộ lọc · N" (N = số bộ lọc ẩn khác mặc định) và phần mở rộng mở sẵn |
| Hàng ≤ 3 ô | Không có nút "Bộ lọc"; ô tìm vẫn đứng đầu, vẫn căn hàng |

**Vì sao số đếm trên nút là bắt buộc:** một bộ lọc đang chọn mà bị giấu thì danh sách rỗng trông
như "chưa có văn bản nào" — cán bộ kết luận sai rằng văn bản chưa vào sổ.

## Hệ quả

- **CSP:** Radix (nền của shadcn/ui) phát thuộc tính `style` nội tuyến. Một CSP nghiêm sau này
  (`style-src` không `'unsafe-inline'`) phải tính tới điều đó. Mục `missing-security-headers` của
  web-admin trong `tools/security_debt.json` **giữ nguyên** — đợt làm mới không đóng khoản nợ ấy.
- Phụ thuộc mới (Tailwind, shadcn/ui và Radix, `lucide-react`, `@fontsource`) đi qua `make vuln`
  như mọi phụ thuộc khác.
- Trong thời gian chuyển tiếp, hai hệ kiểu cùng tồn tại (layer `legacy` + Tailwind); màn chưa
  chuyển giữ nguyên hình cho tới đợt của nó.
