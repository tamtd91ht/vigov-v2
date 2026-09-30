---
id: 0062-can-bo-khong-ghi-danh-gia-thay-dan
tier: T1
source: CURATED
owner: domain
derived_from_commit: b3cc9582
expires: null
owns_facts:
  - "cán bộ không ghi nhận đánh giá sao thay người dân — không có tuyến cán bộ ghi điểm hài lòng, qua điện thoại hay tại quầy"
  - "điểm hài lòng trong KPI và báo cáo chỉ đếm lời đánh giá do chính người dân gửi qua tuyến công dân"
  - "vì sao ViGov lệch có chủ ý khỏi tuyến POST /{id}/rating (feedback.resolve) của kho yêu cầu và khỏi §8.6 của đặc tả phản ánh"
---

# 0062. Cán bộ không ghi nhận đánh giá sao thay người dân

**Trạng thái:** đã chốt · **Ngày:** 2026-09-30 · **Người dùng chốt** (hỏi trong phiên chính,
30/09/2026, chọn phương án đề xuất) · Ý này người dùng nêu lần đầu 27/09, giữ lại 28/09 · **Thay**
`docs/ui-ux/09-phan-anh-nguoi-dan.md` §8.6 · **Lệch có chủ ý** khỏi kho yêu cầu · ADR 0050 điểm 2
giữ nguyên

## Bối cảnh

Hai nguồn cho cán bộ nhập hộ điểm hài lòng:

| Nguồn | Nói gì |
|---|---|
| `docs/ui-ux/09-phan-anh-nguoi-dan.md` §8.6 | Khối "Ghi nhận đánh giá của người dân", năm nút 1★…5★, *"Dùng khi người dân đánh giá qua điện thoại hoặc tại quầy. Một đến hai sao sẽ tự mở lại phiếu."* |
| `../vigov-require` tại `0053854`, `apps/api/app/modules/feedback/router.py:391-404` | `POST /{feedback_id}/rating`, quyền `feedback.resolve`, *"Record a verdict a citizen gave over the counter or by telephone."* |

Nguyên tắc ADR 0050 (*"xung đột thì theo kho yêu cầu"*) sẽ kéo tuyến ấy vào. Người dùng chọn
**không** theo, và vì đây là chỗ một người đọc kho yêu cầu sẽ hỏi lại, quyết định cần một chủ.

Hiện trạng lúc ghi: chỉ có tuyến công dân `POST /api/v1/my-citizen-reports/{maTraCuu}/rating`
(`service-petitions/internal/http/petition_rating.go:3`, `:11-13`); `web-admin` không dựng khối
§8.6 và ghi lý do tại `web-admin/src/features/phan-anh/nhan-phieu.ts:941-945`.

## Quyết định

1. **Chỉ người dân chấm sao**, qua tuyến công dân (ADR 0050 điểm 2). Không có tuyến cán bộ nào ghi
   điểm hài lòng thay họ — qua điện thoại, tại quầy, hay bất kỳ kênh nào.
2. **KPI và báo cáo "điểm hài lòng" chỉ đếm lời đánh giá của chính người dân.**
3. `web-admin` không có khối "Ghi nhận đánh giá của người dân". Màn chi tiết phiếu chỉ **hiện** điểm
   người dân đã chấm.

## Vì sao

| Lý do | |
|---|---|
| **Đánh giá là tiếng nói của người dân VỀ cán bộ** | Người ghi điểm chính là bên bị chấm. Một xã tự chấm mình thì con số không còn đo gì |
| **1–2 sao nhập hộ là thứ né được** | 1–2 sao tự mở lại phiếu (ADR 0050 điểm 2). Người nhập hộ biết điều đó; điểm thấp mà người dân nói qua điện thoại có thể không bao giờ được gõ vào |
| **5 sao nhập hộ thổi phồng KPI** | Không có gì phân biệt được 5 sao người dân thật sự nói với 5 sao cán bộ tự điền — và con số ấy đi lên lãnh đạo |
| **Luật 4 — hai lớp danh tính** | Lời đánh giá phải gắn với **phiên của chính người dân** (luật 4 bất biến 2). Ghi hộ biến một lời kể qua điện thoại — không xác minh được ai nói — thành một hành vi của tài khoản cán bộ. Vết ghi khi ấy chỉ chứng minh *ai gõ*, không chứng minh *người dân đã nói vậy* |

## Cái giá — đừng giấu

- Người dân không dùng Mini App (gọi điện, ghé trụ sở, phiếu nhập hộ) **không có đường chấm sao**.
  Phiếu của họ không có điểm hài lòng, và mẫu số của KPI nhỏ hơn số phiếu đã xử lý.
- Lệch khỏi kho yêu cầu ở một tuyến kho ấy **đã triển khai**. Đối chiếu sau này sẽ thấy nó thiếu —
  thiếu **có chủ ý**, đọc tệp này.
- Phiếu mở lại vì 1–2 sao vẫn chỉ đến từ người dân; không ai "mở lại hộ" bằng đường chấm sao.

## ĐIỀU KIỆN DỪNG

1. Một tuyến cán bộ ghi, sửa hay xoá điểm hài lòng của một phiếu
2. Một nguồn điểm hài lòng thứ hai (khảo sát qua điện thoại, phiếu giấy nhập lại) cộng vào cùng KPI —
   nếu xã cần, đó là một chỉ số **khác**, tên khác, và là câu hỏi mới cho người dùng
3. Dựng khối §8.6 của đặc tả

→ ADR 0050 điểm 2 (dân chấm 1–5 sao, 1–2 sao tự mở lại): `kb/10-decisions/0050-kenh-cong-dan-theo-kho-yeu-cau.md`
→ Danh từ `rating` trên URL: `kb/00-foundation/ubiquitous-language.md` §Tên tài nguyên trên URL
→ Luật 4 (hai lớp danh tính): `.claude/rules/critical/4-citizen-isolation.md`
