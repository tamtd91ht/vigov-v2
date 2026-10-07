---
id: 0068-web-admin-ui-redesign
tier: T1
source: CURATED
owner: architecture
derived_from_commit: b91b2827
expires: null
owns_facts:
  - "làm mới giao diện web-admin chỉ đổi phần trình bày: không đổi đường dẫn, lời gọi API, state, logic lọc/phân trang/phân quyền, tên trường, name/id ô nhập, handler, thứ tự bước nghiệp vụ; không thêm tính năng 'cho hiện đại' (chốt 02/10/2026) — NGOẠI LỆ DUY NHẤT 05/10/2026: URL theo hộp chi tiết lớn (?task=<mã>)"
  - "nền tảng giao diện web-admin là Tailwind CSS v4 + shadcn/ui, icon lucide-react, phông tự phục vụ qua @fontsource (không gọi Google lúc build hay lúc chạy); đỏ/vàng chỉ làm điểm nhấn nhận diện; token chế độ tối chuẩn bị sẵn nhưng chưa bật (màu chính #1565C0 và phông Be Vietnam Pro đã bị thay — xem §Sửa đổi 05/10/2026 (lần 2))"
  - "ngôn ngữ hình ảnh web-admin từ 05/10/2026 (lần 2) theo bản quy chuẩn OMICALL CRM (§1–§11 của bản ấy) trừ 4 điều chỉnh chữ cho WCAG AA (bỏ ở lần 6 #7) và 2 điểm loại trừ: navy #1E3150 cho chữ/icon/header/tab đang chọn; accent #00B1FF chỉ làm nền hover/trạng thái chọn/mảng tô, KHÔNG làm chữ; vòng focus dùng accent ĐẬM đạt ≥3:1 (#00B1FF chỉ 2.40:1, WCAG 1.4.11); nút chính xanh lá chữ trắng #1d853c (#56CC6E chỉ làm nền nhạt/viền); đỏ #FF5955, cam #FEA220; nền ngoài #D1D1D1, app #F5F6FA, thẻ #FFFFFF, nền phụ navy 5%/10%; bóng nhuộm navy, thẻ trên nền app không bóng; Roboto 15px, phân cấp bằng độ đậm 400/500/600, chú thích 12px; bo góc 6/8/12/16/32/50%; control cao 36px; header navy 68px dính trên cùng (điều hướng module KHÔNG còn ở header — xem §Sửa đổi 05/10/2026 (lần 4)); thanh lọc 48px nền navy 5%; dòng bảng 64px không kẻ sọc; áp một lần cho toàn web-admin, không theo xã (thay bảng màu #102b43/#2fb1f9 của §Sửa đổi 05/10/2026) — toàn bộ dòng này bị THAY bởi §Sửa đổi 07/10/2026 (lần 6) #1"
  - "chữ mờ web-admin = navy ở độ mờ ≈0.72 (≥4.5:1); độ mờ 0.5 CHỈ cho trạng thái vô hiệu; chữ liên kết #0369a1; huy hiệu thông báo đỏ #D93A36 chữ trắng ≥11px (chốt 05/10/2026 lần 2) — THAY bởi §Sửa đổi 07/10/2026 (lần 6) #7: web-admin dùng đúng mã màu spec, tương phản dưới AA là nợ trợ năng chủ dự án chấp nhận"
  - "web-admin không có nút nổi (FAB) chat/gọi; không có tên hay logo OMICALL/ViHAT trên màn cán bộ — chỉ mượn ngôn ngữ hình ảnh (chốt 05/10/2026 lần 2)"
  - "'sống động hơn' (chủ dự án 05/10/2026) = điểm nhấn màu + thẻ trắng bo góc trên nền xám nhạt + icon/ảnh đại diện + phản hồi khi tương tác; vẫn không blur/gradient/hình trang trí (§11 giữ) — vế không blur bị bỏ ở §Sửa đổi 07/10/2026 (lần 6) #3"
  - "mọi màn chi tiết web-admin mở dạng HỘP LỚN đè lên trang danh sách, mỗi đợt một màn, Nhiệm vụ thí điểm trước (rồi Đơn thư, Phản ánh): thanh tab thao tác trên (Xem chi tiết · Chỉnh sửa · Xoá) + 'Tạo bởi … lúc …' bên phải; khối trạng thái nổi bật; thân hai cột (thông tin trái, nhật ký/trao đổi phải); dải nút tròn bên phải CHỈ cho thao tác phụ, mỗi nút có tooltip; thao tác chính giữ nút có chữ; chip bước không bấm để chuyển trạng thái ngoài allowed_transitions (chốt 05/10/2026) — riêng Nhiệm vụ: thanh tab thao tác THAY bởi ADR 0076 #1; dải nút tròn bên phải THAY bởi ADR 0076 §Sửa đổi 07/10/2026 (lần 2) #1 (ngăn theo spec 07, không cột icon dọc)"
  - "tab 'Xoá' của hộp chi tiết mở đúng luồng xoá mềm kèm lý do đang có (quyền task.delete), chỉ hiện khi có quyền (chốt 05/10/2026)"
  - "URL theo hộp chi tiết: mở hộp đẩy MỘT mục lịch sử /nhiem-vu?task=<mã>, Back đóng hộp về danh sách cùng bộ lọc; đi tới việc cha/con trong hộp THAY mục lịch sử; đổi bộ lọc giữ ?task= (chốt 05/10/2026)"
  - "tab nhiều bản ghi trong ngăn chi tiết: mở bản ghi mới thêm tab và chuyển sang nó, đóng ngăn giữ tab, ✕ đóng từng tab, 'Đóng tất cả'; giữ trong phiên trình duyệt theo xã + người dùng, tối đa 8 (cũ nhất tự đóng); ?task= = tab đang xem, đổi tab thay mục lịch sử (chốt 05/10/2026, làm ngay — thay 'hoãn')"
  - "globals.css cũ nằm trong cascade layer `legacy` để tiện ích Tailwind thắng; preflight của Tailwind KHÔNG bật ở đợt 1"
  - "không dùng component shadcn vi phạm rào chắn của kho: sidebar (document.cookie, Math.random), chart (dangerouslySetInnerHTML), sonner (chèn <style> lúc chạy — dòng sonner bị THAY bởi §Sửa đổi 07/10/2026 (lần 6) #4); component bọc button/select/input gốc, không thay chúng"
  - "khi làm mới giao diện, kỳ vọng trình bày trong test (chuỗi class, ký tự emoji, markup bao ngoài) được đổi theo; khẳng định hành vi thì không"
  - "mục menu chưa có màn (Sổ tay lãnh đạo, Bản đồ kinh tế số, Báo cáo) mang dấu '?' như mọi phần chưa dựng (thay huy hiệu 'Chưa có' từ 02/10/2026), không có trang khung — không bao giờ 'Sắp có' (dấu '?' cạnh mục sidebar bị bỏ ở §Sửa đổi 07/10/2026 (lần 6) #3; 'Sắp có' vẫn cấm)"
  - "topbar hiện 'Ủy ban nhân dân' làm dòng chú thích phía trên displayName của xã giữ nguyên văn — tên xã không bao giờ được ghép chuỗi"
  - "đặc tả giao diện chủ dự án cung cấp 02/10/2026 thay docs/ui-ux/15 về mặt hình thức (sidebar trắng thay navy); docs/ui-ux là bản sao yêu cầu, không sửa"
  - "làm mới giao diện được ưu tiên hơn mọi việc web-admin khác; Phản ánh đứng đầu đợt 2; đợt 1 = nền móng + Danh bạ, Tổng quan, Nhiệm vụ, Đăng nhập; đợt 2 = 9 màn còn lại + bố cục thẻ trên điện thoại + rà căn hàng/trợ năng"
  - "đặc tả giao diện v2 (02/10/2026): hiện đại = ít ma sát, không trang trí — không backdrop-blur, không gradient làm phong cách chính; quyết định đã chốt (@fontsource, topbar 'Ủy ban nhân dân'; dấu '?' thay 'Chưa có' theo §14) thắng chỗ v2 viết khác; mục ROADMAP_PHASE2 không làm trong đợt này — vế không backdrop-blur bị bỏ ở §Sửa đổi 07/10/2026 (lần 6) #3"
  - "mọi thanh lọc web-admin: ô tìm đứng đầu bên trái, tối đa 2 bộ lọc chính hiện sẵn, còn lại sau nút 'Bộ lọc' hiện số bộ lọc ẩn đang khác mặc định và mở sẵn khi số ấy > 0; mọi ô cao bằng nhau, nhãn trên ô (chốt 02/10/2026)"
  - "web-admin không hiện chữ 'ViGov' ở bất cứ chỗ nào cán bộ nhìn thấy; chỗ đứng tên sản phẩm thay bằng tên xã đang đăng nhập (đọc lúc chạy theo tên miền), câu 'hệ thống ViGov' thành 'hệ thống'; đăng nhập = tên xã + 'Hệ thống điều hành số', tab mặc định 'Hệ thống điều hành số cấp xã', tab màn '<Màn> · <tên xã>'; định danh mã, chú thích, tên gói, tên biến môi trường và tên miền vigov.vn giữ nguyên; issuer TOTP 'ViGov' của tài khoản vận hành nhà cung cấp giữ nguyên vì cán bộ xã không thấy (chốt 02/10/2026) — khối brand 'ViGov' ở sidebar được đưa lại bởi §Sửa đổi 07/10/2026 (lần 6) #3"
  - "phần đặc tả màn web-admin chưa dựng hiện đúng vị trí đặc tả dưới dạng control nó sẽ là, bị vô hiệu, mang dấu '?': di chuột hiện 'Tính năng đang phát triển', bấm mở mô tả; không gọi máy chủ, không lưu gì; việc chủ dự án quyết không làm thì không có chỗ giữ, mục ROADMAP_PHASE2 thì có (mô tả ghi giai đoạn 2); khối gập 'phần chưa dựng' cuối màn bị bỏ; bảng vị trí từng màn chủ dự án đã duyệt, kèm bảng điều chỉnh khi dựng thắng dòng tương ứng — Điểm hài lòng Tổng quan dựng số thật, không còn là chỗ giữ (chốt 02/10/2026)"
  - "bố cục màn Tổng quan web-admin từ 05/10/2026 (lần 3) theo KHUNG prototype DashboardWorkspace.tsx: header 'Tổng quan điều hành' + dòng kỳ + nút kỳ bên phải, PDF/XLSX/PPTX, Trình chiếu; MỘT lưới 1/2/3 cột gồm sáu khối bằng nhau (Nhiệm vụ · Văn bản & Đơn thư · Giải ngân · Thu – chi · Phản ánh · Kinh tế & Tài nguyên) + ô thứ bảy 'Cần xử lý ngay' cuộn trong ô; thay bố cục 3 hàng của đặc tả v2 (02/10) RIÊNG cho Tổng quan; chi tiết hình ảnh theo §Sửa đổi 05/10/2026 (lần 2), không theo CSS prototype"
  - "điều hướng module web-admin từ 05/10/2026 (lần 4) là THANH DỌC BÊN TRÁI theo khung prototype AppSidebar.tsx: icon + chữ, chia nhóm, mục đang mở nổi bật, thu gọn được về dải chỉ icon (tooltip); thay header navy với nút module chỉ icon của lần 2 #6; header navy giữ tên xã, chuông, menu người dùng; dưới 768px giữ ngăn điều hướng có chữ; hình ảnh vẫn theo lần 2 — header navy và hình ảnh bị THAY bởi §Sửa đổi 07/10/2026 (lần 6) #1–#2"
  - "sau đợt 2 giữ nguyên: thanh lọc Phản ánh hiện sẵn Tìm + Phạm vi + Trạng thái; nút thanh soạn thảo Nội dung 36px; 'Thông báo' ở nhóm Công việc của menu; câu 'Ngừng dùng <tên>?' và 'Xác nhận khôi phục câu mặc định' (chốt 02/10/2026)"
  - "từ 06/10/2026 (lần 5) cấu trúc MỌI màn web-admin theo ../vigov-require/apps/admin (không theo vigov-prototype.html): bố cục, nhãn, thứ tự, nút, cột, trường, hộp thoại hay tại chỗ — từng màn một; CSS giữ lần 2 + lần 4; điểm sở thích trình bày ghi trước đó mà prototype nói khác thì bị prototype THAY (mỗi điểm thay ghi ở mục sổ của màn đổi nó); luật cứng và quyết định chủ dự án liệt kê ở §Sửa đổi 06/10/2026 (lần 5) #4 giữ nguyên; chỉ front-end, phần cần tuyến backend mới là control vô hiệu dấu '?' (§14) — vế 'CSS giữ lần 2 + lần 4' bị THAY bởi §Sửa đổi 07/10/2026 (lần 6)"
  - "từ 07/10/2026 (lần 6) design system TOÀN web-admin theo spec Giải ngân của chủ dự án (tmp/web/giai-ngan/vigov-giai-ngan-spec/00-design-system.md, 01-layout-shell.md — ngoài git): phông Inter, bảng token + biến shadcn spec 00 §2, thang chữ 00 §3, mặc định shadcn bản mới 00 §5; THAY lần 2 (token OMICALL, control 36px, header 68px, dòng 64px) và phần hình ảnh của lần 4; giá trị token sống trong mã khi dựng, không chép vào kb"
  - "khung web-admin từ 07/10/2026 (lần 6): sidebar navy cố định thu gọn w-60/w-16 (PanelLeftClose/Open), khối brand 'VG · ViGov · Điều hành số cấp xã', footer CHỈ 'Phiên bản …' (không dòng môi trường); header TRẮNG 64px dính trên, bên trái hai phần tử 'Ủy ban nhân dân' + displayName in hoa bằng CSS, không ghép chuỗi (§7 giữ), + tỉnh/thành (vẫn đọc lúc chạy theo Host, không ghi cứng), ô tìm luôn hiện nhưng VÔ HIỆU dấu '?', chuông, menu người dùng; KHÔNG còn dải banner xã dưới header (bỏ ADR 0069 #5 trên web-admin); dưới 768px giữ ngăn điều hướng có chữ"
  - "lần 6 bỏ §11 (cấm blur: overlay hộp thoại backdrop-blur-xs, header bg-white/95 backdrop-blur) và §13 (khối brand ViGov ở sidebar); bỏ dấu '?' cạnh mục sidebar, nhưng control vô hiệu '?' trong thân màn cho phần chưa dựng (§14) giữ"
  - "web-admin dùng toast sonner chung (một Toaster ở khung app) từ 07/10/2026 (lần 6) — thay dòng sonner của §4; Giải ngân dùng câu chữ của spec; lỗi trong biểu mẫu vẫn hiện tại chỗ"
  - "phông Inter của web-admin nạp bằng @fontsource/inter (tự phục vụ), không next/font/google như spec ghi — lựa chọn kỹ thuật của phiên 07/10/2026, ghi để chủ dự án phủ quyết"
  - "thứ tự nguồn web-admin sau lần 6: luật cứng lần 5 #4 thắng (trừ điểm lần 6 nói rõ thay); cấu trúc = ../vigov-require/apps/admin + spec 02–07; CSS = spec 00/01. Lượt đầu = khung + design system + màn Giải ngân; Thu – Chi (/giai-ngan/thu-chi) lượt riêng"
  - "web-admin dùng ĐÚNG mã màu spec Giải ngân (chủ dự án 07/10/2026, lần 6 #7) — thay yêu cầu AA cho chữ của §Bối cảnh và 4 điều chỉnh AA của lần 2 #7; tương phản dưới AA (chữ phụ #8aa2b8 2.64:1, chữ/focus #2fb1f9 2.39:1, viền ô #dde7ef 1.25:1 …) là NỢ TRỢ NĂNG đã biết, chủ dự án chấp nhận"
  - "hành vi spec Giải ngân trái luật máy chủ hiện tại (Gỡ chứng từ không lý do, Khoá nháp = xác nhận + khoá một lần, không Mở khoá, chặn tự xác nhận, câu xoá hàng loạt / một dự án một nguồn) = BACKEND DEPENDENCY (lần 6 #9): lượt này đặt control '?' ở đó; control đang chạy đúng luật máy chủ (Gỡ kèm lý do, Mở khoá kèm lý do) GIỮ tới khi máy chủ đổi"
---

# 0068. Làm mới giao diện web-admin — chỉ trình bày, Tailwind v4 + shadcn/ui

**Trạng thái:** đã chốt · **Sửa đổi 05/10/2026** (hộp chi tiết lớn, URL theo hộp — §*Sửa đổi
05/10/2026*; bảng màu của sửa đổi ấy đã bị thay) · **Sửa đổi 05/10/2026 (lần 2)** (ngôn ngữ thiết kế
OMICALL CRM: màu, phông Roboto, bo góc — §*Sửa đổi 05/10/2026 (lần 2)*; điểm header chỉ icon thay
sidebar của lần ấy đã bị thay) · **Sửa đổi 05/10/2026 (lần 3)** (Tổng quan theo khung prototype —
§*Sửa đổi 05/10/2026 (lần 3)*) · **Sửa đổi 05/10/2026 (lần 4)** (điều hướng về thanh dọc bên trái —
§*Sửa đổi 05/10/2026 (lần 4)*) · **Sửa đổi 06/10/2026 (lần 5)** (cấu trúc mọi màn giống prototype
nhất có thể, CSS giữ lần 2 + lần 4 — §*Sửa đổi 06/10/2026 (lần 5)*; đang dựng từng màn; vế CSS đã bị
lần 6 thay) · **Sửa đổi 07/10/2026 (lần 6)** (design system theo spec Giải ngân cho toàn web-admin:
Inter, bảng màu spec, header trắng, sidebar thu gọn; bỏ §11, §13, dấu "?" ở sidebar — §*Sửa đổi
07/10/2026 (lần 6)*) · **Ngày:** 2026-10-02 · **Người quyết:** chủ dự án, 02/10/2026 · **Thay**
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

> **Một ngoại lệ 05/10/2026:** URL theo hộp chi tiết lớn (`?task=<mã>`) — §*Sửa đổi 05/10/2026* #5.
> Mọi điểm khác của §1 vẫn giữ.

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
| Màu chính | **Thay 05/10/2026, rồi thay lần nữa bằng §*Sửa đổi 05/10/2026 (lần 2)* #1.** Xanh công vụ `#1565C0` |
| Đỏ cờ / vàng sao | Chỉ làm điểm nhấn nhận diện (logo, dải trên cùng), không dùng tràn lan |
| Icon | `lucide-react`; bỏ emoji làm icon |
| Phông | **Phông thay bằng Roboto — §*Sửa đổi 05/10/2026 (lần 2)* #3; cách tự phục vụ giữ.** Be Vietnam Pro **tự phục vụ qua `@fontsource`** — không request nào tới Google lúc build hay lúc chạy |
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

> **Huy hiệu "Chưa có" đã được thay bằng dấu "?" (§14, chốt 02/10/2026).** Việc bác "Sắp có" và lý do
> bên dưới vẫn giữ.

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
Ba chỗ v2 khác quyết định đã chốt thì **quyết định đã chốt thắng**: không "Sắp có" (§6; huy hiệu
nay là dấu "?", §14), phông `@fontsource` (§2, v2 viết `next/font`), topbar "Ủy ban nhân dân" + tên xã nguyên
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

### 13. Không còn chữ "ViGov" trên web-admin — thay bằng tên xã đang đăng nhập

Chủ dự án chốt 02/10/2026: *"trong web admin sẽ không còn khái niệm Vigov nữa nên ở đâu đang xài thì
bỏ đi"*; hỏi lại thì chốt chữ thay là **"Tên xã đang đăng nhập"**, phạm vi **mọi màn Web Admin**.

| Chỗ | Làm gì |
|---|---|
| Chỗ cán bộ nhìn thấy (chữ trên màn, tiêu đề tab trình duyệt, khối thương hiệu màn đăng nhập, thông báo lỗi, chữ trong biểu mẫu) mà đang đứng tên sản phẩm | Hiện **tên xã đang đăng nhập** |
| Câu nhắc tới sản phẩm ("hệ thống ViGov") | Bỏ chữ "ViGov": **"hệ thống"** |
| Định danh trong mã, chú thích, tên gói, tên biến môi trường | **Giữ** — cán bộ không nhìn thấy; đổi tên cũ là việc rule 12 bất biến 3 không cho |
| Tên miền `vigov.vn` | **Giữ** — là tên miền thật, không phải tên sản phẩm hiện ra |
| Issuer TOTP "ViGov" trong ứng dụng xác thực của tài khoản vận hành nhà cung cấp (`totpIssuer`, `service-identity/internal/app/operator_auth.go`) | **Giữ** — cán bộ xã không nhìn thấy, §13 không áp (chốt 02/10/2026) |

Tên xã đọc **lúc chạy** theo tên miền (luật 1 bất biến 10) qua `communes/current` — xem
`kb/00-foundation/ubiquitous-language.md` mục "Xã của yêu cầu này". Hiện **nguyên văn**, không ghép
chuỗi — §7 vẫn áp.

Câu chữ thay thế, chủ dự án duyệt 02/10/2026:

| Chỗ | Chữ hiện |
|---|---|
| Khối thương hiệu màn đăng nhập | Tiêu đề = **tên xã**; dòng phụ **"Hệ thống điều hành số"** |
| Tiêu đề tab mặc định (layout gốc, trang 404 — nơi chưa xác định được xã) | **"Hệ thống điều hành số cấp xã"** |
| Tiêu đề tab của từng màn | **"<Màn> · <tên xã>"** |
| Gợi ý ô tên hiển thị người gửi thư | **Tên xã** |
| Câu máy chủ: câu báo phạm vi mặc định của service-finance, tiêu đề/nội dung thư thử của service-comms | "ViGov" → **"hệ thống"** |
| Câu báo phạm vi mà xã đã tự lưu đè | **Không** viết lại — là dữ liệu của xã |

**Vì sao:** chủ dự án không nêu lý do ngoài câu trên. Ghi lại để phiên sau không đưa chữ "ViGov" trở
lại màn và không hỏi lại.

### 14. Phần đặc tả chưa dựng: control thật ở đúng vị trí, vô hiệu, dấu "?"

Chủ dự án chốt 02/10/2026: *"Các phần của bản thiết kế chưa dựng được, tôi không muốn để ntn hiện
tại, hãy đề xuất vị trí, tạo sẵn menu/nut...tùy theo loại, để dấu ? trên đó, hover vào thì thông báo
tính năng đang phát triển, có thể view thêm mô tả"*.

| Quy tắc | Nội dung |
|---|---|
| Vị trí | Đúng chỗ đặc tả của màn ấy đặt nó |
| Hình dạng | Đúng loại control nó sẽ là: mục menu, tab, nút, thẻ, cột, ô nhập… |
| Trạng thái | Vô hiệu, mang dấu **"?"** |
| Di chuột | **"Tính năng đang phát triển"** |
| Bấm | Mở mô tả — chính đoạn lý do trước đây nằm trong khối "Chưa dựng" (`PHAN_CHUA_DUNG`, `khoi-chua-dung`) |
| Hành vi | **Không** gọi máy chủ, **không** lưu gì |
| Việc chủ dự án quyết **không làm** (ví dụ ADR 0062) | **Không** có chỗ giữ |
| Mục của `ROADMAP_PHASE2.md` | **Có** chỗ giữ "?"; mô tả ghi rõ thuộc **giai đoạn 2** — vẫn là hoãn, không phải bác (§11 không đổi) |
| Khối gập "N phần của bản thiết kế chưa dựng được" cuối màn | **Bỏ**. Các mảng `PHAN_CHUA_DUNG*` giữ lại làm danh mục mô tả cho dấu "?" |
| Trình tự | Claude đề xuất vị trí từng phần; chủ dự án **đã duyệt nguyên đề xuất** 02/10/2026 (bảng dưới) |

**Vị trí đã duyệt** (02/10/2026):

| Màn | Phần | Vị trí | Loại control |
|---|---|---|---|
| Tổng quan | Điểm hài lòng | Khối Phản ánh | Thẻ KPI |
| Tổng quan | "Giải ngân ngân sách", "Kinh tế & Tài nguyên" | Lưới KPI | Thẻ |
| Tổng quan | "Đơn thư trong kỳ" | Khối Văn bản | Thẻ KPI |
| Tổng quan | PDF / XLSX / PPTX, ⤢ Trình chiếu (giai đoạn 2) | PageHeader | Nút |
| Khung chung | Tìm kiếm toàn hệ thống (giai đoạn 2) | Giữa topbar | Ô nhập |
| Biên bản | Tệp đính kèm (bản quét) | Ô cuối biểu mẫu Nhập biên bản | Ô tệp |
| Biên bản | Hạn gợi ý | Cạnh ô hạn trong biểu mẫu Giao việc | Dòng gợi ý |
| Văn bản & Đơn thư | [Đơn thư công dân] [Báo cáo] | Dưới PageHeader | Tab |
| Văn bản & Đơn thư | Quét & OCR | Nút thứ 2 của PageHeader | Nút |
| Văn bản & Đơn thư | 4 nút chuyển trạng thái | Ngăn kéo văn bản đến | Nút |
| Giải ngân (danh sách) | ☰ Hạng mục, ⬆ Nhập giải ngân | PageHeader | Nút |
| Giải ngân (danh sách) | 4 thẻ KPI, biểu đồ luỹ kế, bảng Tiến độ theo hạng mục, mục Tiến độ theo nguồn vốn | Thân màn | Thẻ / biểu đồ / bảng / mục |
| Giải ngân (danh sách) | ☐ Chỉ dự án chậm, ☑ Gộp theo hạng mục | Thanh lọc | Ô chọn |
| Giải ngân (danh sách) | Nguồn vốn, Vướng mắc mới nhất | Bảng dự án | Cột |
| Giải ngân (chi tiết) | Giải ngân theo nguồn vốn | Thân chi tiết | Mục |
| Giải ngân (chi tiết) | Vướng mắc / Biểu đồ / Trao đổi | Thân chi tiết | Tab |
| Giải ngân (chi tiết) | Nguồn vốn | Bảng chứng từ | Cột |
| Giải ngân (Thêm dự án) | Tự sinh mã; Đơn vị, Cán bộ; danh sách Nguồn vốn | Hộp Thêm dự án | Nút / ô chọn / danh sách |
| Thu – Chi | ⬆ Nạp từ Excel | Trước nút 🗑 Gỡ | Nút |
| Thông báo | "Gửi cho tôi" | Thanh phân đoạn | Phân đoạn |
| Thông báo | Chip Bộ phận nhận, Lưu nháp | Biểu mẫu Soạn | Chip / nút |
| Thông báo | Danh sách Người nhận, 🗑 Gỡ, chip Đã xác nhận, chip trạng thái thư | Chi tiết thông báo | Danh sách / nút / chip |
| Phản ánh | Bản đồ nhiệt, Báo cáo | Thanh tab | Tab |
| Phản ánh | "Liên quan đến tôi" | Tab phạm vi | Tab |
| Phản ánh | Bản đồ nhỏ | Ngăn kéo phiếu | Khung bản đồ |
| Phản ánh | Thôn; Đính ảnh hiện trường | Hộp Nhập hộ | Ô chọn / ô tệp |
| Danh bạ | Tổng số cán bộ, Đang hiện trên Mini App | Đầu màn | 2 thẻ KPI |
| Danh bạ | Ảnh đại diện | Bảng + biểu mẫu | Cột + ô tệp |
| Cấu hình | "Gửi báo cáo định kỳ" | Tab Tự động hoá | Thẻ |
| Nhiệm vụ | — | — | Không có |
| Menu | Sổ tay lãnh đạo, Bản đồ kinh tế số, Báo cáo | Mục menu | Dấu "?" **thay** huy hiệu "Chưa có" (§6); không dựng trang khung |

**Không có chỗ giữ** (chốt 02/10/2026):

| Phần | Vì sao |
|---|---|
| ⟳ Tính lại ngay, huy hiệu "cũ hơn 10 phút" | Đã quyết không làm — ADR 0053 |
| Ghi nhận đánh giá của người dân | Đã quyết không làm — ADR 0062 |
| "{n} chuyên mục" (Nội dung) | Đã quyết không làm |
| Tác vụ Tính lại số liệu Tổng quan | Đã quyết không làm — ADR 0053 |
| Lựa chọn bố cục thuần (hộp thoại hay tại chỗ, cổng quyền phía trình duyệt, ghim cả sổ) | Không phải tính năng, không có gì để giữ chỗ |

**02/10/2026 — cột Lượt xem ra khỏi bảng trên:** chủ dự án đảo quyết định "không lượt xem", nên cột
"Lượt xem" và dòng ở ngăn chi tiết của Nội dung Mini App **được dựng** (số thật, không sắp xếp theo
nó), không phải chỗ giữ. Nội dung quyết định: ADR 0047 §6, dòng "Lượt xem tin".

**Mục "chưa dựng" đã cũ — bỏ hoặc sửa:**

| Mục | Thực tế |
|---|---|
| Chuông Thông báo | Đã dựng — bỏ |
| Cổng quyền phía trình duyệt của Thông báo | Đã dựng — bỏ |
| Nhập Danh bạ từ Excel | Đã dựng ở Cấu hình (ADR 0059) — bỏ |
| Bản quét Biên bản, lý do "kho chưa có nơi lưu tệp" | Nơi lưu tệp đã có (ADR 0052) — viết lại lý do, giữ chỗ |

**Điều chỉnh khi dựng — chủ dự án chốt 02/10/2026** (thắng dòng tương ứng của bảng vị trí trên):

| Màn | Phần | Chốt |
|---|---|---|
| Văn bản & Đơn thư | 4 nút chuyển trạng thái (ngăn kéo văn bản đến) | Nhãn theo vòng đời văn bản đến đã chốt 30/09 (C2): Chờ trình/phân luồng · Đã chuyển xử lý · Đang xử lý · Hoàn thành — **không** dùng nhãn đơn thư vẽ ở spec 05 §3.5. Một dấu "?" cho cả hàng |
| Thu – Chi | ⬆ Nạp từ Excel | Ở PageHeader. Nút "Gỡ bảng" thật nằm trên thanh công cụ của cây và chỉ hiện khi đã có bảng, nên "trước nút 🗑 Gỡ" được hiểu là vị trí PageHeader |
| Thông báo | Chip Đã xác nhận, chip trạng thái thư | Ở khung chi tiết, **không** trên từng thẻ |
| Biên bản | Hạn gợi ý | Ngay trên biểu mẫu Giao việc mở bằng Tách, **không** cạnh ô hạn — ô ấy thuộc `FormGiaoViec` dùng chung với Nhiệm vụ và Phản ánh |
| Tổng quan | Điểm hài lòng | **Dựng số thật** (máy chủ đã trả `rating_sum`/`rating_sample`) — bỏ khỏi danh sách chỗ giữ |

**Vì sao:** chủ dự án không muốn giữ cách hiện tại (khối chữ "Chưa dựng" đứng riêng). Không gọi máy
chủ, không lưu: chỗ giữ là trình bày, không phải tính năng (§1). Việc đã quyết không làm thì không có
gì để giữ chỗ — một chỗ giữ cho nó là báo một tính năng cơ quan đã từ chối. Mục menu chưa có màn
theo cùng một quy tắc với phần chưa dựng trong màn, nên dùng chung dấu "?" thay huy hiệu riêng.

### 15. Bốn điểm trình bày chủ dự án chốt sau đợt 2

Chủ dự án chốt 02/10/2026, sau khi đợt 2 dựng xong — giữ nguyên như đã dựng:

| Điểm | Chốt |
|---|---|
| Thanh lọc Phản ánh hiện sẵn 3 bộ lọc (Tìm + Phạm vi + Trạng thái), vượt mức 2 của §12 | **Tạm giữ** — Phạm vi đóng vai thanh tab của màn |
| Nút thanh soạn thảo Nội dung cao 36px (spec v2), không 44px như trước | **Giữ 36px** |
| "Thông báo" nằm ở nhóm **Công việc** của menu (sổ thông báo nội bộ, spec v2 §5) | **Giữ** |
| Câu hộp xác nhận mới ở Cấu hình: "Ngừng dùng <tên>?", "Xác nhận khôi phục câu mặc định" | **Giữ** |

**Vì sao ghi:** để phiên sau không "sửa" lại cho khớp đặc tả và không hỏi lại.

## Hệ quả

- **CSP:** Radix (nền của shadcn/ui) phát thuộc tính `style` nội tuyến. Một CSP nghiêm sau này
  (`style-src` không `'unsafe-inline'`) phải tính tới điều đó. Mục `missing-security-headers` của
  web-admin trong `tools/security_debt.json` **giữ nguyên** — đợt làm mới không đóng khoản nợ ấy.
- Phụ thuộc mới (Tailwind, shadcn/ui và Radix, `lucide-react`, `@fontsource`) đi qua `make vuln`
  như mọi phụ thuộc khác.
- Trong thời gian chuyển tiếp, hai hệ kiểu cùng tồn tại (layer `legacy` + Tailwind); màn chưa
  chuyển giữ nguyên hình cho tới đợt của nó.

## Sửa đổi 05/10/2026 — sống động hơn, bảng màu mới, hộp chi tiết lớn

Mục này ghi thêm, không sửa phần trên: phần trên là quyết định lúc viết, mục này thắng khi nói khác.
**Người quyết:** chủ dự án, 05/10/2026, trong phiên chính. **Chưa dựng** — mỗi dòng là điều phải đúng
khi dựng.

Lời chủ dự án: *"tôi muốn web có ui sống động hơn hiện tại"*, kèm ảnh mẫu một màn chi tiết CRM (ảnh
không nằm trong kho).

| # | Điểm | Chốt |
|---|---|---|
| 1 | "Sống động hơn" nghĩa là gì | Điểm nhấn màu + **thẻ trắng bo góc trên nền xám nhạt** + icon / ảnh đại diện + phản hồi khi tương tác (hover, nhấn, focus). **Vẫn không** blur, gradient, hình trang trí — §11 giữ nguyên |
| 2 | Bảng màu | **Đã bị thay bởi §*Sửa đổi 05/10/2026 (lần 2)* — đừng dựng theo dòng này.** Áp **một lần cho toàn web-admin** (token là toàn cục), **thay** dòng "Màu chính" của §2. Lấy từ prototype `../vigov-require/apps/admin/src/app/globals.css:61-100`. Nền trang `#f4f8fb` · thẻ `#fff` · nút chính navy `#102b43` · nền nhấn `#e8f5fe` · cyan `#2fb1f9` **chỉ** làm nền, viền, trạng thái hiện tại, vòng focus — **không bao giờ** làm chữ trên nền trắng · chữ / liên kết xanh = cyan đậm `#0369a1` · chữ mờ giữ tương phản **≥ 4.5:1** (không dùng `#8aa2b8` của prototype cho chữ; dùng một màu xám đậm hơn). Đỏ cờ / vàng sao của §2 không đổi. **Không theo xã** — ADR 0069 chỉ cho xã đổi logo / banner |
| 3 | Hộp chi tiết lớn | Mọi màn chi tiết mở dạng **hộp thoại lớn đè lên trang danh sách**, **mỗi đợt một màn**: Nhiệm vụ thí điểm trước, Đơn thư và Phản ánh ở các đợt sau. Bố cục: thanh **tab thao tác** trên cùng (Xem chi tiết · Chỉnh sửa · Xoá), *"Tạo bởi … lúc …"* bên phải · **khối trạng thái** nổi bật (trạng thái hiện tại, chip các bước, *"Cập nhật gần nhất"*, liên kết nhật ký / hành trình) · thân **hai cột** (thông tin trái, nhật ký / trao đổi phải — `docs/ui-ux/02-nhiem-vu.md` §5.9) · **dải nút tròn bên phải chỉ cho thao tác PHỤ**, mỗi nút có tooltip. Thao tác chính (chuyển trạng thái, giao lại, lùi hạn) **giữ nút có chữ** — câu "thao tác quan trọng luôn có chữ" của §11 giữ. Chip bước **không** bấm được để chuyển sang trạng thái ngoài `allowed_transitions` |
| 4 | Xoá | Giữ. Tab "Xoá" mở đúng luồng **xoá mềm kèm lý do** đang có (quyền `task.delete`, luật 7), **chỉ hiện khi có quyền** |
| 5 | URL theo hộp | Mở hộp đẩy **một** mục lịch sử `/nhiem-vu?task=<mã>`; Back **đóng hộp** về danh sách với **cùng bộ lọc**. Đi tới việc cha / con trong hộp **thay** mục lịch sử (không đẩy thêm). Đổi bộ lọc **giữ** `?task=`. **Sửa** §1 ("chỉ trình bày — không đổi điều hướng / state") **riêng ở điểm này** |
| 6 | Tab nhiều bản ghi ("Đóng tất cả") | ~~**Hoãn** sang đợt sau~~ **Thay 05/10/2026 — làm ngay.** Chủ dự án: *"mở nhiệm vụ A xong đóng lại, khi mở nhiệm vụ B thì nó vẫn còn tab nhiệm vụ A nhưng focus vào nhiệm vụ B, người dùng có thể đảo qua tab nhiệm vụ A hoặc nhấn nút x để tắt nó đi"*. Đóng ngăn chỉ ẩn ngăn, tab còn; ✕ trên tab đóng tab ấy; "Đóng tất cả" đóng hết. Giữ trong `sessionStorage` theo xã + người dùng (chỉ mã + nhãn, không dữ liệu cá nhân), tối đa 8 tab, tab dùng lâu nhất tự đóng. `?task=` luôn là tab đang xem; đổi tab **thay** mục lịch sử, một lần Back vẫn đóng ngăn |

**Vì sao hộp lớn không phải hình thức mới:** đặc tả của khách hàng đã viết màn chi tiết nhiệm vụ là
*"lớp phủ gần toàn màn hình"* (`docs/ui-ux/02-nhiem-vu.md:134-136`, DetailDrawer). Sửa đổi này đưa
web-admin về đúng đặc tả ấy và dùng chung một bố cục cho mọi màn chi tiết.

**Vì sao cyan không làm chữ:** `#2fb1f9` trên nền trắng chỉ đạt **2.39:1**, dưới ngưỡng WCAG AA 4.5:1
mà §Bối cảnh đã nhận; `#8aa2b8` đạt 2.64:1, cùng lý do. `#0369a1` đạt 5.93:1. Trạng thái vẫn **icon +
chữ, không bao giờ chỉ bằng màu** (§Bối cảnh) — bảng màu mới không đổi điều đó.

**Vì sao URL theo hộp (ngoại lệ của §1):** hộp che trọn danh sách nên cán bộ đọc nó như một trang; bấm
Back mà rời luôn màn Nhiệm vụ, mất bộ lọc, là mất việc đang làm. Một mục lịch sử cho mỗi lần mở (thay,
không đẩy, khi đi cha / con) giữ cho một lần Back luôn về đúng danh sách.

**Việc đang dở:** phần làm lại giao diện chi tiết Phản ánh (mục sổ tiến độ `giao-dien-phan-anh-w2-s1`)
sẽ chuyển sang bố cục hộp lớn **ở đợt riêng của Phản ánh**, không trong đợt thí điểm Nhiệm vụ.

## Sửa đổi 05/10/2026 (lần 2) — theo ngôn ngữ thiết kế OMICALL CRM

Mục này ghi thêm, không sửa phần trên; mục này thắng mọi chỗ phía trên khi nói khác — kể cả **bảng
màu** của §*Sửa đổi 05/10/2026* #2 (`#102b43` / `#2fb1f9`), nay **bị thay**. Các điểm #1, #3–#6 của
sửa đổi ấy (thẻ trắng trên nền xám, hộp chi tiết lớn, Xoá, URL theo hộp, hoãn tab nhiều bản ghi)
**giữ**. **Người quyết:** chủ dự án, 05/10/2026, trong phiên chính. **Chưa dựng.**

Nguồn: bản quy chuẩn OMICALL CRM chủ dự án đưa 05/10/2026 (ngoài kho). Bản ấy không vào git; các giá
trị đã chốt ghi dưới đây, đây là chỗ sở hữu duy nhất của chúng.

Lời chủ dự án, theo thứ tự: *"tôi muốn theo hướng này"* (bản quy chuẩn); về trợ năng: *"miniapp mới cần
cho người lớn tuổi chứ web-admin thì đa số là trẻ tuổi"*; rồi *"làm đi"* với phương án được đề xuất
**"theo guide + chỉnh 4 màu chữ cho đạt chuẩn"**.

**Phạm vi:** áp §1–§11 của bản quy chuẩn, **trừ** 4 điều chỉnh (#7) và 2 điểm loại trừ (#8). Áp **một
lần cho toàn web-admin**, không theo xã (ADR 0069 chỉ cho xã đổi logo / banner).

| # | Điểm | Chốt |
|---|---|---|
| 1 | Màu lõi | Navy chính `#1E3150` — chữ, icon, header, tab đang chọn. Accent `#00B1FF` — nền hover, trạng thái đang chọn, mảng tô; **không làm chữ**. **Vòng focus** dùng accent đậm đạt ≥3:1 trên nền trắng (`#00B1FF` chỉ 2.40:1 — WCAG 1.4.11; bản quy chuẩn ghi focus = accent, đây là điều chỉnh thứ 5 cho AA). Thành công `#56CC6E` (họ màu của nút chính, xem #7). Nguy hiểm `#FF5955`. Cảnh báo `#FEA220` |
| 2 | Nền | Ngoài cùng `#D1D1D1` · vùng app `#F5F6FA` · thẻ `#FFFFFF` · nền phụ `rgba(30,49,80,.05)` · nền phụ 2 `rgba(30,49,80,.10)`. Nền nhạt (tint) = màu gốc ở alpha `.1` (cảnh báo, focus) hoặc `.2–.3` (tag, chip). Bảng màu phân loại cho tag / nhãn / ảnh đại diện theo §2.4 bản quy chuẩn: `#00B1FF` `#56CC6E` `#FF5955` `#FEA220` `#E82A8F` `#57BFDB` `#A540B8` `#6C63FF` `#F0557F` `#10A37F` `#B161F8` `#155AEF` `#229FDA` |
| 3 | Chữ | **Roboto**, tự phục vụ qua `@fontsource` (cách tự phục vụ của §2 giữ), **thay Be Vietnam Pro**. Cỡ gốc **15px**; phân cấp bằng độ đậm 400 / 500 / 600, không bằng cỡ; chú thích 12px |
| 4 | Bo góc · bóng | Bo góc 6 / 8 / 12 / 16 / 32 px / 50%. Bóng nhuộm navy theo §5 bản quy chuẩn (`0 4px 16px` ở alpha `.16` / `.20` / `.32` cho popover / thẻ nổi / header). Thẻ trên nền app **không bóng** |
| 5 | Kích thước | Control cao **36px** — **thay** 40px của §Bối cảnh và §12 (quy tắc căn hàng của §12 giữ). Header navy **68px** dính trên cùng. Thanh lọc **48px** nền navy 5%. Dòng bảng **64px**, không kẻ sọc, hover nền accent `.04` + viền accent `.1` — **thay** dòng 48px của §11. Ô tìm dạng viên thuốc (bo 32px). Popover bo 12px, bóng `.16` |
| 6 | Điều hướng | **Đã bị thay bởi §*Sửa đổi 05/10/2026 (lần 4)* — đừng dựng theo dòng này.** Header navy với **nút module chỉ icon + tooltip**, **thay** sidebar chữ (sidebar trắng của §8). Câu "thao tác quan trọng luôn có chữ" của §11 vẫn áp cho **thao tác**; nút module là điều hướng |
| 7 | 4 điều chỉnh cho WCAG AA | Yêu cầu AA của §Bối cảnh **giữ** — bảng dưới |
| 8 | Loại trừ | (a) **Không** nút nổi (FAB) chat / gọi — web-admin không có tính năng ấy, và §1 / `ROADMAP_PHASE2.md` không cho thêm tính năng. (b) Trạng thái / tag **không bao giờ chỉ bằng màu** — icon + chữ giữ (§Bối cảnh) |
| 9 | Không đổi | Vẫn **không** blur, gradient (§11). **Không** tên hay logo OMICALL / ViHAT trên màn cán bộ — chỉ mượn ngôn ngữ hình ảnh; chỗ đứng tên vẫn là tên xã (§13) |

**4 điều chỉnh (#7)** — tỉ lệ đo lại khi ghi (công thức độ chói tương đối WCAG 2.x):

| Chỗ | Bản quy chuẩn | Chốt | Tương phản |
|---|---|---|---|
| Chữ mờ | navy độ mờ `.5` — 2.91:1 trên trắng, trượt | navy độ mờ **≈ `.72`**; `.5` **chỉ** cho trạng thái vô hiệu | 5.42:1 trên trắng · 5.21:1 trên `#F5F6FA` · 4.97:1 trên nền phụ navy 5% |
| Nút chính chữ trắng | `#56CC6E` — 2.05:1, trượt | **`#1d853c`**; `#56CC6E` giữ cho nền nhạt và viền nút viền | 4.70:1 |
| Chữ liên kết | `#00B1FF` — 2.40:1, trượt | **`#0369a1`** | 5.93:1 trên trắng · 5.49:1 trên `#F5F6FA` |
| Huy hiệu thông báo | `#FF5955` chữ trắng 9–10px — 3.08:1, trượt | đỏ đậm hơn **`#D93A36`**, chữ **≥ 11px**; `#FF5955` giữ cho icon và nền nhạt | 4.56:1 |

**Vì sao vẫn giữ AA dù người dùng trẻ:** chủ dự án nêu tuổi người dùng để nói web-admin không cần mức
chăm chút như Mini App, và đã chọn phương án "chỉnh 4 màu chữ cho đạt chuẩn" chứ không bỏ chuẩn. AA là
ngưỡng §Bối cảnh đã nhận cho một cơ quan công quyền; bốn chỗ trên là những chỗ bản quy chuẩn dùng màu
nhấn **làm chữ**, nên chỉnh đúng bốn chỗ ấy và giữ nguyên phần còn lại.

**Vì sao `#1d853c`, không `#1f8a3e` như phương án đề xuất:** đo lại khi ghi, `#1f8a3e` với chữ trắng chỉ
đạt 4.41:1 — dưới 4.5:1, tức không "đạt chuẩn" như chủ dự án chốt. `#1d853c` cùng sắc độ, đậm hơn một
chút, vượt ngưỡng.

**Vì sao bỏ sidebar chữ:** chủ dự án chấp nhận header chỉ icon vì người dùng web-admin đa số là cán bộ
trẻ; tooltip thay nhãn chữ. *(Lý do lúc ấy; điểm #6 đã bị §Sửa đổi 05/10/2026 (lần 4) thay.)*

## Sửa đổi 05/10/2026 (lần 3) — Tổng quan theo khung prototype

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác, **chỉ cho màn Tổng quan**.
**Người quyết:** chủ dự án, 05/10/2026, trong phiên chính — chốt qua một câu hỏi đã nhắc lại nguyên
phương án. **Đang dựng** (thẻ việc TASK-P2).

Lời chủ dự án: *"prototype chỉ là khung thôi còn chi tiết css thì theo mẫu ui mới"*.

Nguồn khung: `../vigov-require/apps/admin/src/components/reports/DashboardWorkspace.tsx`
(`vigov-require@0053854`) — header dòng 125-209, lưới khối dòng 220, ô "Cần xử lý ngay" dòng 266.

| # | Điểm | Chốt |
|---|---|---|
| 1 | Header | Tiêu đề **"Tổng quan điều hành"** + dòng phụ nêu kỳ đang xem; **bên phải**: nút chọn kỳ, **PDF / XLSX / PPTX**, **Trình chiếu** |
| 2 | Lưới | **MỘT** lưới, **1 / 2 / 3 cột** theo bề rộng màn. Sáu khối **bằng nhau**: Nhiệm vụ · Văn bản & Đơn thư (tên theo prototype, chủ dự án chốt 05/10 — khối giữ cả số văn bản đến lẫn đơn thư) · Giải ngân · Thu – chi · Phản ánh · Kinh tế & Tài nguyên. **Ô thứ bảy** "Cần xử lý ngay" nằm trong cùng lưới, có trần chiều cao và **cuộn trong ô** |
| 3 | Thay gì | **Thay bố cục 3 hàng** (hàng 1 "Cần xử lý" + "Tình hình trong kỳ" · hàng 2 "Cần xử lý ngay" · hàng 3 thẻ gọn các module) đã dựng theo đặc tả v2 §8.2 + §8b ngày 02/10 — bố cục ấy ghi ở sổ tiến độ `kb/90-ephemeral/tien-do/web-admin.json` → `giao-dien-tong-quan-w1-s2`. **Chỉ thay cho Tổng quan**; §11 và phần còn lại của đặc tả v2 giữ cho mọi màn khác |
| 4 | Hình ảnh | Màu, phông, cỡ, bo góc, bóng, chiều cao control theo §*Sửa đổi 05/10/2026 (lần 2)* (OMICALL CRM) — **không** theo CSS của prototype. Prototype chỉ cho **khung**: thứ tự, vị trí, số cột |
| 5 | Giữ nguyên | **Không** nút "Tính lại ngay", **không** nhãn "số liệu cũ" (ADR 0053 — dù khung prototype có cả hai, dòng 144-181) · bấm sâu ra **danh sách đã lọc**, không ra hộp thoại (ADR 0053) · phần chưa dựng là control vô hiệu dấu "?" **tại đúng vị trí trong khung mới** (§14) — kể cả PDF / XLSX / PPTX / Trình chiếu · khối nào tài khoản không có quyền đọc thì **ẩn**, như trước |

**Vì sao đổi khung mà không đổi hình:** chủ dự án tách hai việc (lời trên) — khung lấy từ prototype,
ngôn ngữ hình ảnh đã chốt ở lần 2. Lấy luôn CSS của prototype là quay lại bảng màu `#102b43` /
`#2fb1f9` mà lần 2 đã thay. Lý do chọn một lưới thay ba hàng: chủ dự án **không nêu** ngoài việc theo
prototype — đừng suy thêm.

## Sửa đổi 05/10/2026 (lần 4) — điều hướng về thanh dọc bên trái

Mục này ghi thêm, không sửa phần trên; mục này **thay** §*Sửa đổi 05/10/2026 (lần 2)* #6 (header navy
với nút module chỉ icon thay sidebar chữ — đã dựng ở commit `b6a1eb12`). Mọi điểm khác của lần 2 **giữ**.
**Người quyết:** chủ dự án, 05/10/2026, trong phiên chính. **Đang dựng** (thẻ việc TASK-P3).

Lời chủ dự án, theo thứ tự: *"nó làm mất ô menu bên trái rồi, phải giữ ô menu đó lại chứ"*; rồi, khi
được hỏi xử lý các nút icon trên header thế nào: *"Menu trái + giữ cả nút icon nhưng là nằm dọc bên
trái thay vì nằm ngang phía trên, hãy xem prototype"*.

Nguồn khung: `../vigov-require/apps/admin/src/components/layout/AppSidebar.tsx` (`vigov-require@0053854`).

| # | Điểm | Chốt |
|---|---|---|
| 1 | Điều hướng module | **Thanh dọc bên trái** theo khung prototype: mỗi mục **icon + chữ**, **chia nhóm** có nhãn nhóm, mục đang mở **nổi bật** (`aria-current`, vạch đánh dấu), **thu gọn được** về dải chỉ icon có tooltip — prototype có (rộng `w-60` / thu gọn `w-16`, `AppSidebar.tsx:22-23`) |
| 2 | Header navy | **Giữ**: tên xã (§7, §13), chuông thông báo, menu người dùng. Nút module **rời khỏi** header |
| 3 | Dưới 768px | Ngăn điều hướng có chữ **giữ** như đã dựng ở `b6a1eb12` |
| 4 | Hình ảnh | Màu, Roboto, cỡ, bo góc, control theo lần 2 — **"prototype = khung, CSS = mẫu UI mới"** (lần 3). **Không** lấy màu `bg-sidebar` của prototype |
| 5 | Giữ nguyên | Cổng quyền của từng mục; mục chưa có màn mang dấu "?" (§14); không dùng component `sidebar` của shadcn (§4) |

**Ghi chú khi dựng (#1, thu gọn):** prototype nhớ trạng thái thu gọn bằng `localStorage`
(`SidebarState.tsx:24,70`), không bằng cookie — khác component `sidebar` của shadcn mà §4 đã loại vì ghi
`document.cookie`. Trạng thái này là tuỳ chọn hiển thị, không phải dữ liệu cá nhân.

**Vì sao đổi lại:** chủ dự án thấy header chỉ icon làm **mất ô menu bên trái** và muốn giữ nó; các nút
icon không bỏ mà **xếp dọc** bên trái như prototype. Ngoài câu trên chủ dự án **không nêu** lý do — đừng
suy thêm (chẳng hạn về tuổi người dùng, lý do của lần 2 #6).

## Sửa đổi 06/10/2026 (lần 5) — web-admin giống prototype nhất có thể

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác, **trừ** các điểm ở #4.
**Người quyết:** chủ dự án, tối 05/10/2026, trong phiên chính. **Đang dựng từng màn một** từ
06/10/2026.

Lời chủ dự án: *"dựa vào prototype để sửa lại thật chính xác nội dung của web admin nhé, yêu cầu
giống prototype nhất"*. Cùng ngày, trước đó: *"prototype chỉ là khung thôi còn chi tiết css thì theo
mẫu ui mới"* (lần 3).

| # | Điểm | Chốt |
|---|---|---|
| 1 | Nguồn cấu trúc | `../vigov-require/apps/admin` — **không** phải `vigov-prototype.html` (bản mẫu tĩnh đời đầu). Bố cục, nhãn, thứ tự, nút, cột, trường, cách trình bày **hộp thoại hay tại chỗ** theo prototype, **từng màn một** |
| 2 | CSS | **Giữ** lần 2 (token OMICALL CRM) + lần 4 (thanh dọc bên trái). "Prototype = khung, CSS = mẫu UI mới" (lần 3) vẫn đúng |
| 3 | Sở thích trình bày đã ghi trước | Điểm **sở thích** ghi trước đó mà prototype nói khác thì bị prototype **THAY**. Ví dụ: bộ lọc sau nút "Bộ lọc" (§12) → theo hàng lọc của prototype; biểu mẫu tại chỗ → hộp thoại ở chỗ prototype dùng hộp thoại; nhóm menu → chỉ đổi nếu cấu trúc prototype đòi, và **giữ** thanh dọc bên trái. Mỗi điểm bị thay được liệt kê ở **mục sổ của màn đổi nó** |
| 4 | Giữ bất kể prototype (luật cứng, không phải sở thích) | Xã lấy từ `Host`, **không** ô chọn xã (luật 1) · che dữ liệu cá nhân + quyền mở che riêng (luật 3, ADR 0030) · xoá mềm kèm lý do, không xoá cứng / xoá cứng hàng loạt (luật 7) · cán bộ **không** nhập đánh giá thay công dân (ADR 0062) · mã đã cấp bất biến, không tự sinh mã dự án trước khi chốt định dạng (luật 7, ADR 0065) · mã trạng thái tiếng Việt + hai đồng hồ hạn (ADR 0011, 0027, 0028) · sửa chứng từ đã xác nhận → về nháp (ADR 0036) · ô đồng ý trước khi công khai cán bộ lên Mini App (NĐ 13/2023) · khung bản đồ + xác nhận pháp lý (ADR 0072) · quyền theo luật 5, **không** thêm khoá mới · **không** "Tính lại ngay", bấm sâu ra danh sách đã lọc (ADR 0053 — số đếm trực tiếp làm nút ấy vô nghĩa) · quyết định chủ dự án trong ngày giữ: hộp chi tiết nhiệm vụ dạng ngăn phải + tab bản ghi (05/10 #3/#6), thanh dọc bên trái (lần 4), khung Tổng quan (lần 3) |
| 5 | Phạm vi | **Chỉ front-end `web-admin`**; không đổi API, proto, migration. Thứ prototype có mà cần tuyến backend mới → đặt **đúng vị trí prototype** dưới dạng control vô hiệu dấu "?" (§14), **không bao giờ làm giả** |

**Vì sao #3 và #4 tách nhau:** chủ dự án yêu cầu "giống prototype nhất", nên mọi lựa chọn trình bày
trước đây chỉ là sở thích đều nhường prototype. Các điểm ở #4 không phải sở thích: mỗi điểm có luật
hoặc ADR sở hữu, và prototype viết trước hoặc ngoài các ràng buộc ấy. Ngoài lời trên, chủ dự án
**không nêu** lý do — đừng suy thêm.

## Sửa đổi 07/10/2026 (lần 6) — design system theo spec Giải ngân cho toàn web-admin

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác — kể cả **lần 2** (token OMICALL
CRM) và **phần hình ảnh của lần 4**, nay **bị thay**. **Người quyết:** chủ dự án, 07/10/2026, trong
phiên chính (lệnh `/fix-web-admin --menu=giai-ngan`). **Chưa dựng** — mỗi dòng là điều phải đúng khi
dựng.

Nguồn: bản spec Giải ngân chủ dự án viết từ mã prototype, `tmp/web/giai-ngan/vigov-giai-ngan-spec/`
(ngoài git), chủ yếu `00-design-system.md` và `01-layout-shell.md`; prototype
`../vigov-require/apps/admin`. Giá trị token **không chép vào đây** (luật 9) — chúng sống trong mã khi
dựng; spec là nguồn lúc dựng.

Câu trả lời của chủ dự án (chọn trong câu hỏi của phiên, nguyên văn lựa chọn): phạm vi design system →
**"Đổi toàn app theo spec"** (Inter + bảng màu spec + header trắng + sidebar thu gọn cho cả 14 menu;
thay lần 2/4); các điểm spec đi ngược quyết định cứng (chữ/logo "ViGov" + footer "Môi trường phát
triển", bỏ dấu "?" ở mục chưa dựng, blur nền hộp thoại) → **"Theo spec"**; spec 08 (`/giai-ngan/thu-chi`)
→ **"Để lượt riêng"**; toast → **"Thêm sonner, dùng chung"**.

| # | Điểm | Chốt |
|---|---|---|
| 1 | Design system | Spec 00/01 cho **toàn web-admin**: phông **Inter**; bảng token (navy, surface, line, ink, ink-muted, brand, leaf, tangerine, violet, teal, danger, các nền nhạt) và biến shadcn theo spec 00 §2; thang chữ spec 00 §3; mặc định shadcn bản mới spec 00 §5 (Button `h-8` …). **Thay** lần 2 (token OMICALL, control 36px, header 68px, dòng bảng 64px) và phần hình ảnh của lần 4. Vẫn một lần cho toàn web-admin, không theo xã (ADR 0069) |
| 2 | Khung | Sidebar navy **cố định**, thu gọn `w-60` / `w-16` (`PanelLeftClose` / `PanelLeftOpen`), khối brand **"VG · ViGov · Điều hành số cấp xã"**, footer phiên bản. Header **TRẮNG** 64px dính trên: bên trái tên xã in hoa + tỉnh/thành — **vẫn đọc lúc chạy theo `Host`** (luật 1 bất biến 10; `web-admin/src/lib/tenant-config.ts:168`); "Thăng Bình" / "Đà Nẵng" trong spec chỉ là dữ liệu mẫu, **không** ghi cứng. Ô tìm luôn hiện, chuông, menu người dùng. **Thay** header navy của lần 2 / lần 4. Dưới 768px: **giữ** ngăn điều hướng có chữ (lần 4 #3) — spec không nói, chưa ai chốt khác |
| 3 | Bỏ §11, §13, một phần §14 | **§11 (cấm blur) bỏ:** overlay hộp thoại `backdrop-blur-xs`, header `bg-white/95 backdrop-blur` theo spec. **§13 (không chữ ViGov) bỏ ở khối brand sidebar.** **§14:** bỏ dấu "?" **cạnh mục sidebar**; control vô hiệu "?" **trong thân màn** cho phần chưa dựng **giữ** — spec không nói tới phần ấy. Ranh giới này là **cách phiên đọc phạm vi câu trả lời** "Theo spec", không phải chủ dự án nói thêm. "Sắp có" vẫn cấm (§6) |
| 4 | Toast | `sonner` dùng chung, **một** `Toaster` ở khung app — **thay** dòng `sonner` của §4. Giải ngân dùng câu chữ của spec; lỗi trong biểu mẫu **vẫn hiện tại chỗ** |
| 5 | Phạm vi lượt đầu | Khung + design system + màn Giải ngân (`/giai-ngan`, chi tiết dự án, các hộp thoại, nguồn vốn). Thu – Chi (`/giai-ngan/thu-chi`, spec 08) **lượt riêng**. Các menu khác **đổi hình ngay** theo token / khung (hệ quả tự nhiên của token toàn cục); cấu trúc từng màn của chúng **chưa sửa** |
| 6 | Cách nạp phông | `@fontsource/inter` (tự phục vụ), **không** `next/font/google` như spec ghi — cùng phông, khác cách nạp. Lý do của §2 giữ: bản dựng Jenkins không phụ thuộc mạng ngoài (commit `ccef7ab3`, `7aa5fe23`: bản dựng đỏ khi phải kéo từ Internet), và hợp với CSP chặt sau này (web-admin chưa gửi CSP — mục `missing-security-headers` trong `tools/security_debt.json`). **Lựa chọn kỹ thuật của phiên**, ghi để chủ dự án phủ quyết |

**Lô câu trả lời thứ hai, cùng phiên 07/10/2026** (nguyên văn lựa chọn): tương phản → **"Đúng mã màu
spec"**; footer / logo → **"Bỏ dòng môi trường, giữ logo VG"**; hành vi spec trái luật máy chủ → **"Đổi
theo spec (cần backend)"**; kiểm thử → **"Khung dựng tĩnh + kiểm lại trên prod"**.

| # | Điểm | Chốt |
|---|---|---|
| 7 | Tương phản | **Đúng mã màu spec**, giống prototype từng điểm ảnh. **Thay**, cho web-admin, yêu cầu AA cho chữ của §Bối cảnh, 4 điều chỉnh AA của lần 2 #7 (chữ mờ ≈0.72, `#1d853c`, `#0369a1`, `#D93A36`) và vòng focus ≥3:1 của lần 2 #1. Chủ dự án chấp nhận chữ phụ nhạt khó đọc: đây là **nợ trợ năng đã biết** — bảng đo dưới. Trạng thái vẫn **icon + chữ, không bao giờ chỉ bằng màu** (lần 2 #8b giữ) |
| 8 | Footer · logo · banner | Khối brand **"VG · ViGov · Điều hành số cấp xã"** như spec. Footer **chỉ** "Phiên bản 0.1.0" — dòng "Môi trường phát triển" **bỏ** (sai trên site thật của xã). **Dải banner xã dưới header (ADR 0069 #5) bị BỎ** trên web-admin — prototype không có |
| 9 | Hành vi spec trái luật máy chủ | Gỡ chứng từ **không** lý do · Khoá chứng từ nháp = xác nhận + khoá **một lần gọi** · **không** "Mở khoá" · chặn tự xác nhận ("Không thể tự xác nhận khoản do chính mình nhập") · câu spec về xoá hàng loạt "phiếu chi được khoá" và "mỗi dự án thuộc về một nguồn" → **BACKEND DEPENDENCY**, đề xuất `/develop-backend-api giai-ngan` sau; lượt này đặt control "?" (§14) ở các chỗ ấy. **Cách phiên đọc, không phải chủ dự án nói:** control đang chạy đúng luật máy chủ hiện tại (Gỡ kèm lý do + `budget.confirm`, Mở khoá kèm lý do) **giữ** tới khi máy chủ đổi — bỏ chúng bây giờ là bỏ một khả năng máy chủ vẫn đòi; chỉ hành vi spec mà máy chủ **chưa có** mới thành "?". Lượt backend sẽ gặp luật 7 (`delete_reason`, xoá mềm) và câu mở **#29** (DECIDED: mở khoá phải có lý do, người khoá không tự mở) — đó là **STOP CONDITION** của lượt ấy, **không** được quyết ở đây |
| 10 | Kiểm thử thị giác | Trang xem trước tĩnh, **chỉ ở dev**, dựng khung + component Giải ngân với dữ liệu mẫu và CSS đã build để chụp ảnh trước mỗi commit; sau khi Jenkins triển khai, kiểm lại trên `thangbinh-danang.vigov.vn` |
| 11 | Mặc định phiên nêu, chủ dự án **không phản đối** | Dòng header = **hai phần tử** "Ủy ban nhân dân" + `displayName`, in hoa bằng CSS, **không ghép chuỗi** (§7 giữ) · ô tìm ở header **luôn hiện nhưng vô hiệu, dấu "?"** (chưa có tuyến tìm — lần 5 #5) · 3 mục sidebar chưa có màn: **mờ, không bấm, không "?"**, di chuột hiện "Tính năng đang phát triển" · biểu đồ luỹ kế giữ **SVG tự vẽ**, đổi kiểu (không recharts; `chart` của shadcn vẫn cấm, §4) · ô "Đơn vị thực hiện" nhập tự do = lựa chọn "?" + BACKEND DEPENDENCY · **giữ như hiện tại:** cách hiện lỗi / rỗng theo bộ lọc, tổng của dòng nhóm (tổng máy chủ chỉ khi không lọc), tiêu đề cột "Nguồn vốn", "Gỡ dự án" ở chi tiết, "Đổi mật khẩu" + nhãn vai trò trong menu tài khoản, chặn Tắt/Bật hạng mục theo cấp, tên dự án vẫn là liên kết |

**Nợ trợ năng (#7)** — tỉ lệ đo khi chuẩn bị câu hỏi (WCAG 2.x); ngưỡng chữ thường 4.5:1, thành phần
giao diện 3:1:

| Màu spec | Dùng cho | Tương phản |
|---|---|---|
| `#8aa2b8` (ink-muted) | Chữ phụ, nhãn | 2.64:1 trên trắng · 2.48:1 trên `#f4f8fb` |
| `#2fb1f9` (brand) | Liên kết · vòng focus | 2.39:1 (chữ) · 2.39:1 (focus, cần 3.0) |
| `#86b940` (leaf) | Chữ "tốt / đã xong" | 2.33:1 |
| `#ff7a1a` (tangerine) | Chữ cảnh báo | 2.61:1 |
| `#12b5c9` (teal) | Chữ chip | 2.48:1 |
| `#e5484d` (danger) | Chữ nhỏ · chữ trắng trên nút nguy hiểm | 3.91:1 · 3.91:1 |
| `#dde7ef` (line / input) | Viền ô nhập | 1.25:1 (cần 3.0) |

**Thứ tự nguồn sau lần 6:** luật cứng của lần 5 #4 vẫn thắng (trừ điểm lần 6 nói rõ thay) · **cấu trúc**
= prototype `../vigov-require/apps/admin` + spec 02–07 · **CSS** = spec 00/01 — thay vế "CSS giữ lần 2
+ lần 4" của lần 5 #2.

**Còn mở / hệ quả:**

| # | Điểm | Thực tế |
|---|---|---|
| a | Chữ phụ `#8aa2b8` và các màu chữ dưới AA | **Đã chốt (#7):** giữ đúng mã màu spec; nợ trợ năng chủ dự án chấp nhận — bảng đo ở #7 |
| b | Footer "Môi trường phát triển" | **Đã đóng (#8):** dòng ấy bỏ, footer chỉ còn phiên bản |
| c | Mọi menu khác đổi hình cùng lúc | Ảnh chụp / kiểm thử thị giác các màn ấy **chưa làm** |
| d | Tên xã in hoa | §7 viết "không viết HOA". Spec 01 in hoa cả dòng "Uỷ ban nhân dân xã …". **Đã rõ (#11):** hai phần tử, in hoa bằng CSS, `displayName` nguyên văn, không ghép chuỗi |
| e | `sonner` và CSP | §4 loại `sonner` vì chèn `<style>` lúc chạy. Lần 6 #4 nhận điểm ấy; một CSP nghiêm sau này phải tính tới nó, như thuộc tính `style` của Radix (§Hệ quả) |
| f | ADR 0069 nói khác | ADR 0069 #5 (dải banner dưới topbar ở mọi trang) và dòng "logo xã dùng chung: thanh bên" của ADR ấy nay trái #8 (khối brand VG thay logo xã ở thanh bên). ADR 0069 đã có dòng trỏ sang đây (07/10/2026). **Câu hỏi cho chủ dự án:** ảnh banner xã vẫn tải lên được ở Cấu hình › Nhận diện xã nhưng không còn chỗ hiện trên web-admin — giữ ô tải lên hay gỡ nó? Chưa ai chốt |

**Vì sao:** ngoài các lựa chọn trên, chủ dự án **không nêu** lý do — đừng suy thêm.
