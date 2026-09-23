---
id: 0036-sua-chung-tu-da-xac-nhan-thi-ve-nhap
tier: T1
source: CURATED
owner: domain
derived_from_commit: eaf06ea
expires: null
owns_facts:
  - "sửa một chứng từ giải ngân ĐÃ XÁC NHẬN thì nó quay về `ke-toan-nhap`, và dấu người xác nhận bị xoá"
  - "vì sao đây là một chuyển tiếp LÙI có chủ ý chứ không phải nới lỏng ràng buộc khoá"
  - "vì sao quyết định này KHÔNG chạm câu hỏi mở #29 và #30"
---

# 0036. Sửa một chứng từ đã xác nhận thì nó về nháp

**Trạng thái:** đã chốt · **Ngày:** 2026-09-23 · **Chủ dự án quyết**
**Nối tiếp** `docs/ui-ux/06-giai-ngan.md` §8.2 · §13 quy tắc 3

## Bối cảnh

Vòng đời chứng từ giải ngân trong đặc tả là một chiều:

```
ke-toan-nhap  →  da-xac-nhan  →  da-khoa
```

Đặc tả nói **duy nhất một** điều về việc sửa: *"Chứng từ đã `Đã khoá` thì không sửa/xoá; muốn
sửa phải mở khoá"* (§13 quy tắc 3). Nó **không nói** chuyện gì xảy ra khi sửa một chứng từ đang
ở `da-xac-nhan`.

Nếu không có luật nào lấp khoảng ấy thì kế toán sửa được số tiền của một chứng từ lãnh đạo đã
xác nhận, `trang_thai` giữ nguyên `da-xac-nhan`, và `nguoi_xac_nhan_id` vẫn trỏ vào người ấy —
**chữ xác nhận đi theo sang một con số người ký chưa từng nhìn thấy.** Không test nào đỏ, vì
không luật nào nói nó sai.

**MÃ ĐÃ LẤP KHOẢNG ẤY TRƯỚC KHI CÓ ADR NÀY**, ở `bda5dd5` (23/09/2026):
`domain.TrangThaiSauKhiSua` + `app.Sua()` hạ trạng thái và xoá `NguoiXacNhanID`, kèm ba ca kiểm.

Nhưng nó **chưa có nơi nào ghi vì sao**, và tệ hơn: `docs/ui-ux/06-giai-ngan.md` §13 vẫn chỉ
nói về `Đã khoá`, nên **đặc tả và mã nói hai điều khác nhau về cùng một bảng**. Một luật chỉ
sống trong mã là một luật lần sau có người "dọn cho gọn" — đúng lớp lỗi luật 9 sinh ra để chặn.

ADR này ghi lại quyết định, không cấp phép cho một việc mới.

**Ghi chú về cách tìm ra:** ban đầu phiên này báo với chủ dự án rằng luật còn thiếu, sau khi đọc
`ChoSua()` — hàm ấy đúng là chỉ chặn `da-khoa` — cộng 60 dòng đầu của `Sua()`. Khối `veNhap` nằm
ở dòng 349. **Đọc một hàm rồi kết luận về cả đường ghi là phương pháp sai**, và nó suýt tạo ra
một lần viết lại thứ đã chạy. Ghi ra đây vì lần sau sẽ có người đọc `ChoSua()` và kết luận y hệt.

## Vì sao câu hỏi này nổi lên hôm nay

Kho yêu cầu `../vigov-require` gặp đúng ca này ở xã thật và đã sửa — `c3f4d6a`,
`apps/admin/src/components/budget/DisbursementForm.tsx`. Lý do họ ghi tại chỗ:

> *"lãnh đạo xác nhận những con số kia, không phải những con số này"*

Đối chiếu đầy đủ: `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md` mục 5.

## Quyết định

**Sửa một chứng từ đang ở `da-xac-nhan` thì nó quay về `ke-toan-nhap`, và `nguoi_xac_nhan_id`
bị xoá.** Muốn nó xác nhận lại thì phải có người bấm xác nhận lại — một hành vi, một người, một
dòng vết.

| Trạng thái trước khi sửa | Sau khi sửa |
|---|---|
| `ke-toan-nhap` | `ke-toan-nhap` — không đổi |
| `da-xac-nhan` | **`ke-toan-nhap`**, `nguoi_xac_nhan_id` = NULL |
| `da-khoa` | **từ chối** — không đổi, đúng §13 quy tắc 3 |

## Vì sao là LÙI TRẠNG THÁI chứ không phải CẤM SỬA

Cấm sửa bản đã xác nhận nghe an toàn hơn, và nó sai với việc thật: giữa `xác nhận` và `khoá`
là đúng quãng người ta soát lại và phát hiện gõ nhầm. Cấm sửa ở đó đẩy kế toán sang **gỡ rồi
nhập lại** — một chứng từ mới, một mã mới, và chứng từ cũ nằm lại trong sổ như một bản ghi bị
bỏ. Đắt hơn hẳn, và nó làm hỏng chính thứ `da-khoa` sinh ra để bảo vệ.

Lùi trạng thái giữ **một** bản ghi, **một** dòng lịch sử, và nói đúng sự thật: bản này chưa
được ai xác nhận nữa.

## Vì sao KHÔNG chạm câu #29 và #30

Hai câu ấy đang mở, và dễ tưởng quyết định này lấn vào:

| Câu | Hỏi gì | Quan hệ |
|---|---|---|
| **#29** | **MỞ KHOÁ** một chứng từ `da-khoa`: có phải ghi lý do · người vừa khoá có tự mở lại được · có trần số lần | **Không chạm.** Đây là quãng `da-khoa`, quyết định này dừng trước đó |
| **#30** | Có **chứng từ hoàn** không, hay mọi điều chỉnh giảm phải đi qua mở khoá | **Không chạm.** Đó là điều chỉnh một chứng từ **đã khoá**; đây là sửa một chứng từ **chưa khoá** |

Quyết định này lấp đúng khoảng đặc tả bỏ trống — quãng giữa `xác nhận` và `khoá` — và không
phát biểu gì về hai quãng kia.

## Hệ quả

| | |
|---|---|
| Quyền | Không đổi. `budget.update` sửa, `budget.confirm` xác nhận — đúng §8.2. Việc lùi trạng thái là **hệ quả của hành vi sửa**, không phải một hành vi riêng cần khoá quyền riêng |
| Vết kiểm toán | Lần sửa đã ghi vết (luật 6). Vết ấy nay mang thêm `trang_thai` trước/sau, nên câu *"ai làm mất chữ xác nhận"* trả lời được |
| Dữ liệu cũ | Không có. Chưa xã nào chạy, chưa chứng từ thật nào tồn tại — nên đây **không** phải migration trên hồ sơ lưu trữ |
| Đặc tả | `docs/ui-ux/06-giai-ngan.md` §13 phải mang luật này, nếu không mã và đặc tả nói hai điều khác nhau về cùng một bảng |

## Cái giá, nói thẳng

Kế toán sửa một dấu phẩy trên chứng từ đã xác nhận thì lãnh đạo phải bấm xác nhận lại. Với xã
sửa nhiều, đó là thao tác lặp. Chủ dự án chấp nhận cái giá ấy ngày 2026-09-23: **một chữ ký
trên con số người ký chưa nhìn thấy thì đắt hơn một lần bấm.**

→ Luật 6 (vết kiểm toán) · luật 7 (không xoá cứng)
→ Đối chiếu: `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md`
