---
id: 0038-duyet-lui-han-theo-nguoi-tren-ban-ghi
tier: T1
source: CURATED
owner: domain
derived_from_commit: df61ff6
expires: null
owns_facts:
  - "ai duyệt đề nghị lùi hạn của một nhiệm vụ: đúng người ghi ở `lanh_dao_giao_viec_ma`"
  - "khoá quyền `task.extend` chặn ở CỬA, người trên bản ghi chặn ở TẦNG NGHIỆP VỤ — hai lớp, không thay nhau"
  - "vì sao quyết định này KHÔNG tự động áp sang phiếu phản ánh"
---

# 0038. Duyệt lùi hạn thuộc về người ghi trên bản ghi, không thuộc người cầm khoá

**Trạng thái:** đã chốt · **Ngày:** 2026-09-23 · **Chủ dự án quyết**
**Nối tiếp** ADR 0037 · `docs/ui-ux/02-nhiem-vu.md` §5.8 · §7.1

## Bối cảnh

Luật 5 kiểm quyền theo `(tenant_id, role, permission)` — **một khoá phẳng, không có chiều "bản
ghi nào"**. Với phần lớn thao tác điều đó đúng: ai cầm `task.create` thì tạo được nhiệm vụ, bất
kỳ nhiệm vụ nào.

Duyệt lùi hạn thì không. `02-nhiem-vu.md` §5.8 và §7.1 đặt ô **"Lãnh đạo giao việc"** trên biểu
mẫu, và `0006_nhiem_vu.sql:282` đã có cột `lanh_dao_giao_viec_ma` **tách khỏi** `nguoi_tao_ma`.
Hai cột tách nhau vì một lý do có thật ở xã: **văn thư nhập hộ phần lớn nhiệm vụ**, nên người
gõ bàn phím không phải người quyết.

Nếu chỉ kiểm `task.extend`, thì bất kỳ lãnh đạo nào cầm khoá ấy cũng duyệt được đề nghị lùi hạn
của **nhiệm vụ do người khác giao** — kể cả nhiệm vụ của bộ phận mình không liên quan.

## Quyết định

**Đề nghị lùi hạn chỉ được duyệt bởi đúng người ghi ở `nhiem_vu.lanh_dao_giao_viec_ma`.**

Hai lớp, và chúng **không thay nhau**:

| Lớp | Chặn cái gì | Ở đâu |
|---|---|---|
| `task.extend` | Tài khoản này có được đụng tới việc lùi hạn nói chung không | **Cửa** — `authz.RequirePermission` |
| `Principal.Ma == lanh_dao_giao_viec_ma` | Có phải việc của đúng người này không | **Tầng nghiệp vụ** |

Bỏ lớp trên thì ai cũng gọi được tuyến. Bỏ lớp dưới thì mọi lãnh đạo duyệt được mọi nhiệm vụ.
**Giữ cả hai, và giữ đúng thứ tự ấy** — cửa trả 403 cho người không liên quan gì, tầng nghiệp vụ
trả lỗi nghiệp vụ cho người có quyền nhưng không phải người được giao.

**So bằng MÃ CÁN BỘ** (`CB-…`), không bằng id nội bộ: `Principal.Ma` và `lanh_dao_giao_viec_ma`
là cùng một từ vựng (luật 6 bất biến 8), nên đây là phép so thẳng, không phải một lần tra bảng.

## CÂU CÒN MỞ — `lanh_dao_giao_viec_ma` RỖNG thì ai duyệt

`0006:282` khai cột **nullable**. Một nhiệm vụ không có người giao đích danh thì quyết định này
nói: **không ai duyệt được** — và đề nghị lùi hạn nằm lại vĩnh viễn.

Ba lối, chưa chốt:

| Lối | Giá |
|---|---|
| **Bắt buộc khai lúc tạo** | Rẻ nhất **lúc này** vì bảng rỗng; đắt nếu có luồng tạo nhiệm vụ không biết ai giao (nhập Excel §8, tách từ kết luận họp) |
| Rơi về `nguoi_tao_ma` | `../vigov-require` làm thế (`service.py:634`) — nhưng họ rơi về cho việc **BÁO TIN**, không cho **THẨM QUYỀN**. Rơi về ở đây là trao quyền duyệt cho văn thư đã nhập hộ |
| Ai cầm `task.extend` cũng duyệt khi cột rỗng | Một lỗ mở đúng vào ca hay xảy ra nhất, và nó im lặng |

**Cho tới khi có người chốt: TỪ CHỐI, kèm câu nói rõ nhiệm vụ này chưa ghi lãnh đạo giao việc.**
Hỏng theo chiều đóng, và câu từ chối chỉ thẳng thứ phải sửa. Không im lặng cho qua.

## Vì sao KHÔNG tự động áp sang phiếu phản ánh

Cùng hình dạng, **khác bản ghi, khác quyết định đã ký**. Câu hỏi mở **#7 `DECIDED` 16/09/2026**
nói về phiếu phản ánh: *"Quyền `feedback.resolve` quyết định ai đóng được"* — thẩm quyền theo
KHOÁ, không theo người trên bản ghi.

`kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md` §Mâu thuẫn ghi đúng vùng ấy là
**điều kiện dừng chưa có đáp**, và mục `luat-nam-giu-hoi-khach` ở sổ tiến độ `service-petitions`
vẫn đang chờ. ADR này **không** đóng nó.

Hai chỗ khác nhau ở thứ đang bị quyết: lùi hạn là **dời một cam kết nội bộ** giữa hai cán bộ;
đóng phiếu phản ánh là **tuyên bố với người dân rằng việc đã xong**. Thẩm quyền của hai việc ấy
không buộc phải cùng hình dạng.

## Hệ quả

| | |
|---|---|
| Tuyến ghi | `POST` đề nghị lùi hạn (người thực hiện) và `POST` quyết định (lãnh đạo giao việc) là hai tuyến, hai lớp kiểm khác nhau |
| Không ai duyệt đề nghị của chính mình | Luật thứ hai, độc lập — `../vigov-require` có nó (`service.py:719-723`) và nó vẫn cần ở đây kể cả khi người giao việc tự đề nghị lùi |
| Vết kiểm toán | `de_nghi_lui_han.nguoi_duyet_ma` đã có ở `0006:594`. Ghi mã cán bộ, không id |
| Dữ liệu cũ | Không có. Bảng rỗng ở mọi môi trường |

→ Luật 5 (phân quyền) · luật 6 bất biến 8 (chủ thể là mã cán bộ)
→ Nền: `service-petitions/migrations/0006_nhiem_vu.sql`
→ Câu cùng hình dạng còn mở cho phản ánh: `kb/90-ephemeral/tien-do/service-petitions.json`
