---
id: 0030-hai-khoa-quyen-phan-anh
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 0960b2a
expires: null
owns_facts:
  - "vì sao phân loại phiếu cần khoá riêng chứ không dùng lại feedback.assign"
  - "vì sao xem đầy đủ người gửi cần khoá riêng chứ không dùng lại feedback.restricted"
  - "quy ước đặt tên khoá quyền: nhóm.mộttừ, không gạch dưới ở vế sau"
  - "khách chốt ĐÚNG HAI khoá và cố ý để lại phần còn thiếu"
---

# 0030. Hai khoá quyền cho phân hệ phản ánh — và quy ước đặt tên khoá

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Nối tiếp ADR 0028** · **Liên quan câu mở #27**

## Bối cảnh

Hai hành vi trong phân hệ phản ánh **không có khoá nào canh**. Luật 5 nói hậu quả bằng một câu:
guard toàn cục chỉ chặn người **chưa đăng nhập**, nên một tuyến không khai quyền là tuyến **mọi
vai trò cán bộ gọi được**. Hỏng trong im lặng — không test nào đỏ, không dòng log nào.

Chỗ thiếu đã được ghi lại từ trước, tại đúng nơi nó chặn việc:
`service-petitions/internal/http/routes.go:238-243`.

## Quyết định

**Thêm ĐÚNG HAI khoá: `feedback.classify` và `feedback.unmask`.**

### Vì sao không dùng lại khoá đang có

| Khoá đang có | Nghĩa của nó | Vì sao không phủ được |
|---|---|---|
| `feedback.assign` | Giao phiếu cho ai xử lý | **Phân loại là hành vi ẤN ĐỊNH HẠN** — lĩnh vực quyết số giờ SLA, nên chốt lĩnh vực là phát ra một cam kết của chính quyền với một người dân đang cầm mã tra cứu (ADR 0028 quyết định E). Người được giao **việc** không đương nhiên là người được **hứa thay cơ quan** |
| `feedback.restricted` | Xem được phiếu thuộc lĩnh vực `can-bo` (tố cáo tác phong cán bộ) | Đó là phạm vi **NỘI DUNG**, không phải **dữ liệu cá nhân**. Người xác minh một tố cáo cán bộ không đương nhiên cần số điện thoại của mọi người phản ánh rác thải, và ngược lại |

**Nền của cả hai ô trên là luật 5 bất biến 3b:** các quyền này **không phải tích Đề-các** để
suy ra, và `task.approve` cố ý không phải `task.extend`. Gộp hai nghĩa vào một khoá là **cấp
quyền ngoài ý định**, và không ai thấy — vì màn Phân quyền chỉ hiện **một** ô.

### `feedback.unmask` mở đúng một thứ đang hỏng trong vận hành

Tuyến đọc phiếu hôm nay che **cả họ tên lẫn số điện thoại**, vô điều kiện —
`service-petitions/internal/http/phieu_phan_anh.go:129-130`. Tức **cán bộ không gọi lại được
cho người phản ánh**.

Đó không phải lỗi: luật 3 bất biến 3 bắt che trừ khi bên gọi có quyền xem đầy đủ **tường
minh**, và khoá ấy chưa tồn tại — nên không có đường nào là "tường minh" và chỉ còn hai trạng
thái, che tất hoặc mở tất. Đây là **cái giá thật của nguyên tắc hỏng-thì-đóng**, và khoá này là
chỗ trả nó.

**Ràng buộc đi kèm, không tách rời khỏi quyết định:** mỗi lần đọc đầy đủ **phải ghi vết** —
luật 6 bất biến 7. Lược đồ không cưỡng chế được điều đó; nó thuộc tầng use case, và là điều
kiện để khoá này không thành một cửa hậu im lặng vào dữ liệu cá nhân của cả xã.

**Khoá này KHÔNG trả lời câu mở #11.** #11 hỏi về số di động của **cán bộ** hiện cho cán bộ
khác; khoá này nói về **người gửi phản ánh**. Hai chủ thể khác nhau, hai màn hình khác nhau.
Dùng `feedback.unmask` để mở danh bạ cán bộ là đúng thứ ADR này vừa cấm ở bảng trên.

## QUY ƯỚC ĐẶT TÊN KHOÁ QUYỀN — áp cho mọi khoá sau này

> **Một khoá là `nhóm.mộttừ`. Không gạch dưới ở vế sau.**

Không phải thẩm mỹ, và đã **đếm chứ không đoán**: trong toàn bộ khoá đã nạp của
`service-identity/migrations/`, **không khoá nào** có gạch dưới ở vế sau — `task.extend`,
`report.export`, `admin.audit`, `feedback.restricted`.

Vì sao nó ràng buộc: luật 5 bất biến 3b nói khoá là **CHÍNH CHUỖI** mà màn Phân quyền hiện ra
và bảng `quyen` lưu. Một chuỗi lệch quy ước **không sửa lại được** sau khi một xã đã gán nó cho
một vai trò — lúc ấy đổi tên là migration trên dữ liệu phân quyền đang chạy.

**Ca đã xảy ra, ghi lại để không ai lập luận lại vòng đó:** khoá thứ hai suýt mang tên
`feedback.view_full`. Đã bác vì quy ước trên, và vì `unmask` **đúng việc hơn** — nó **GỠ CHE**,
đúng cặp với `MaskPhone` / `MaskCccd` của luật 3, chứ không phải một quyền "xem" chung chung.
Gặp lại chuỗi `feedback.view_full` ở đâu đó: đó là **tên đã bị bác**, không phải cách gọi thứ
hai đang song song.

## KHÁCH CHỐT ĐÚNG HAI KHOÁ — phần còn thiếu là CỐ Ý để lại

Đặc tả đòi nhiều khoá hơn số đã nạp, và **trọn một nhóm** chưa có mặt. Chênh lệch ấy đã đo, và
**câu mở #27 giữ con số cùng chỗ đo** — không chép sang đây, vì hai bản của một con số là hai
bản sẽ lệch.

Khách chọn thêm **hai khoá cần ngay** và **để lại phần còn lại**. Đó là một quyết định, không
phải một lần bỏ sót: **đừng bịa các khoá còn thiếu cho đủ số.** Một khoá bịa ra là một dòng
trong màn Phân quyền mà không ai ở xã hiểu nghĩa, và nó không xoá đi được sau khi có xã tick
vào nó.

## Phải trả

- **Trả ngay:** hai khoá nữa để quản trị xã phải hiểu và phải quyết cấp cho ai
- **Trả ngay:** mọi tuyến đọc đầy đủ phải kèm đường ghi vết — không có đường tắt
- **Không mua được:** hai khoá này **không** tự cấp cho ai. Khoá tồn tại để quản trị xã tick
  trên màn Phân quyền; tự gán là tự quyết ai trong một cơ quan nhà nước được hứa thay cơ quan ấy

## ĐIỀU KIỆN DỪNG

1. **Thêm bất kỳ khoá quyền nào** không nằm trong đặc tả và chưa được khách chốt — kể cả khi
   một tuyến đang cần nó để viết xong (tiền lệ: câu mở #20 đang chặn đúng kiểu ấy)
2. **Đặt một khoá có gạch dưới ở vế sau**, hoặc một khoá suy ra từ khoá khác theo kiểu tích
   Đề-các
3. **Gán khoá cho một vai trò** trong migration hạt giống
4. **Trả dữ liệu cá nhân đầy đủ mà không ghi vết**, ở bất kỳ tuyến nào — kể cả tuyến xuất Excel

→ ADR 0028 (vì sao phân loại là hành vi ấn định hạn): `kb/10-decisions/0028-moc-dat-han-hai-dong-ho.md`
→ ADR 0013 (vì sao migration đã chạy thì không sửa, nên hai khoá đi bằng migration riêng): `kb/10-decisions/0013-schema-migration-and-append-only.md`
→ Chi tiết gieo hạt, `thu_tu`, và vì sao không sửa `0001`: `service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql`
→ Vì sao khoá `feedback.*` lệch với tài nguyên `citizen-reports`: `kb/00-foundation/ubiquitous-language.md`
→ Câu mở #27 (số khoá còn thiếu và trọn một nhóm): `kb/00-foundation/open-questions.json`
→ Luật 5 (khoá là một chuỗi phẳng, không phải tích Đề-các): `.claude/rules/critical/5-rbac.md`
→ Luật 3 (che dữ liệu cá nhân) · luật 6 (ghi vết mọi lần đọc đầy đủ): `.claude/rules/critical/3-personal-data.md` · `.claude/rules/critical/6-audit-log.md`
→ Kỹ năng: `.claude/skills/petition-lifecycle/SKILL.md`
