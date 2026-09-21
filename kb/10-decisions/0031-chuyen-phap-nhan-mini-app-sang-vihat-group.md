---
id: 0031-chuyen-phap-nhan-mini-app-sang-vihat-group
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 917ce3f
expires: null
owns_facts:
  - "chuyển quyền sở hữu Mini App và OA xác thực sang ViHAT Group, chốt 21/09/2026"
  - "phạm vi lần chuyển pháp nhân — vai trò OA nào đổi tên, vai trò OA nào giữ nguyên"
  - "hai câu còn mở do lần chuyển pháp nhân sinh ra: bên vận hành backend vihat-miniapp, và tệp vector biểu tượng của tập đoàn"
---

# 0031. Chuyển quyền sở hữu Mini App sang ViHAT Group

**Trạng thái:** đã chốt · **Ngày:** 2026-09-21 · **Thay thế §*Pháp nhân đứng tên* của ADR 0018**

## Bối cảnh

ADR 0018 §*Pháp nhân đứng tên* chốt ngày 17/09/2026: **VHS** đứng tên Mini App, OA
`VihatSoftware` là OA xác thực. Ngày 21/09/2026 chủ dự án chốt chuyển sang **công ty mẹ**.

Đây không phải tình huống bất ngờ: **ADR 0018 ĐIỀU KIỆN DỪNG #4 đã gọi tên trước đúng nó** —
*"có đề xuất chuyển quyền sở hữu Mini App hoặc OA xác thực sang một pháp nhân khác"*. Cơ chế
ấy hoạt động, và đây là ADR nó yêu cầu.

`kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md` **giữ nguyên từng chữ, kể cả dòng
trạng thái**. Tầng T1 không sửa ADR, và §*Pháp nhân đứng tên* của 0018 là **bằng chứng** rằng
ngày 17/09 đã chốt như vậy, kèm lý do vì sao. Xoá hay sửa nó đi là xoá lý do người ta nghĩ khác
vào ngày hôm đó — thứ đắt nhất một ADR giữ. Chính 0018 đã làm đúng như thế với ADR 0006.

## Quyết định

| Thứ | Trước (0018, 17/09) | Từ 21/09/2026 |
|---|---|---|
| Pháp nhân đứng tên Mini App | VihatSoftware (VHS) | **`ViHAT Group`** — công ty mẹ |
| OA xác thực Mini App | `VihatSoftware` | **`Vihat`** |
| Nội dung app giai đoạn 1 | Giới thiệu công ty con | **Giới thiệu Tập đoàn** |

**Lập luận của 0018 không đổi, chỉ đổi ai trong nhóm nhà cung cấp đứng tên.** Vẫn là *nhà cung
cấp* đứng tên chứ không phải một cơ quan nhà nước, vì một Mini App phục vụ 200+ xã thì không xã
nào là chủ sở hữu tự nhiên của nó. Lần này chọn pháp nhân **cao hơn một cấp** trong cùng nhóm.

## Cái gì KHÔNG đổi — và đây là chỗ dễ suy rộng quá tay

ADR 0018 tách **hai vai trò rất khác nhau** của một OA. Lần chuyển này động tới **đúng vai trò
thứ nhất**.

| Vai trò của OA | Phạm vi | Lần chuyển này |
|---|---|---|
| **OA xác thực Mini App** | **MỘT**, ở tầng nền tảng | **ĐỔI TÊN**: `VihatSoftware` → `Vihat` |
| **OA gửi thông báo cho công dân** (ZNS) | **Theo từng xã** | **KHÔNG đổi một chữ nào** |

Vai trò thứ hai giữ nguyên toàn bộ: `service-comms` vẫn sở hữu adapter, khoá vẫn theo từng xã
(ADR 0009), và xã chưa cấu hình vẫn suy giảm nhìn thấy được qua trạng thái `chua-cau-hinh-kenh`
— `service-comms/internal/domain/thong_bao_gui_cong_dan.go:69`. Người gửi mà người dân thấy khi
nhận tin về hồ sơ của mình **vẫn là "UBND xã X"**, không phải `ViHAT Group`.

Câu để nhớ của 0018 — **app có một OA, tin nhắn có nhiều OA** — vẫn đứng. Lần này chỉ đổi tên
của cái **MỘT**.

**Vì sao phải viết hẳn ra:** một lần "chuyển pháp nhân sở hữu Mini App" đọc lướt rất giống một
lần đổi thương hiệu toàn hệ thống. Suy rộng sang OA của xã là đi ngược đúng thứ ADR 0006 muốn
giữ và 0018 đã cứu được: uy tín của **chính cơ quan** với **chính người dân của mình**, nằm ở
tên người gửi trên tin nhắn.

## Điều kiện phải giữ với Zalo — là THỦ TỤC, không phải cấu hình

Câu 1 bảng chứng cứ của ADR 0018 ghi hai ràng buộc thủ tục kèm ràng buộc 1↔1: Mini App và OA
xác thực phải có *"liên kết chặt chẽ về mặt pháp lý"*, và người gửi yêu cầu xác thực phải là
**Admin của cả hai**.

Sau khi chuyển, **`ViHAT Group` và OA `Vihat` vẫn phải thoả đúng hai ràng buộc ấy** — nghĩa là
Admin Mini App và Admin OA `Vihat` phải là **cùng một người thuộc `ViHAT Group`**.

Không có biến môi trường nào, không có dòng mã nào, không có tệp cấu hình nào làm việc này xảy
ra. Nó xảy ra ở **hồ sơ nộp cho Zalo**. Ai đọc ADR này rồi đi tìm chỗ sửa trong mã là đang tìm
một thứ không tồn tại.

## Bề mặt người dân nhìn thấy

Huy hiệu **"đã xác thực"** trong Mini App từ nay hiện tên **`ViHAT Group`**. Đây là bề mặt
người dân nhìn thấy, nên nó thuộc **phạm vi luật 10**, không phải chi tiết đăng ký.

Hệ quả giao diện mà ADR 0018 §*Hệ quả* điểm 3 đã nêu — người dân thấy **một** OA bên trong app
nhưng nhận tin từ OA **của xã**, hai cái tên khác nhau trong cùng một hành trình, và phải thiết
kế để điều đó không trông giống lừa đảo — **còn nguyên**. Lần chuyển này chỉ đổi tên OA phía
app; nó không làm khoảng cách giữa hai cái tên hẹp đi, và cũng không làm nó rộng thêm. Nội dung
đầy đủ đọc ở 0018, không chép sang đây.

## Mức chứng cứ — không được nâng lên

| Điều | Mức |
|---|---|
| Chủ dự án đã chốt chuyển pháp nhân và chuỗi hiển thị | **CHẮC CHẮN** — quyết định của chủ dự án, 21/09/2026 |
| Ràng buộc 1 Mini App ↔ 1 OA xác thực | **NGUỒN THỨ CẤP** — không đổi so với 0018 |
| Zalo cho chuyển quyền sở hữu **bằng cách nào** (chuyển chủ sở hữu app đã đăng ký, hay phải đăng ký lại) | **CHƯA TRA** |

Cả ADR 0018 đứng trên nguồn thứ cấp cho ràng buộc 1↔1 (0018 §*Nguồn và vì sao mức chứng cứ chỉ
tới đó*, dòng 1: tài liệu chính thức của Zalo render bằng JS nên không đọc trực tiếp được).
Quyết định của chủ dự án làm rõ **ý định**, nó **không** nâng mức chứng cứ về phía Zalo lên.
**Phải xác nhận lại với Zalo lúc làm thủ tục** — và lần này có thêm một câu để hỏi: thủ tục
chuyển cụ thể là gì. Câu CÒN MỞ #2 của ADR 0018 vì thế **nặng thêm chứ không nhẹ đi**.

## CÒN MỞ — khách/chủ dự án phải chốt, tuyệt đối không tự chọn hộ

| # | Câu hỏi | Vì sao không phải việc của người viết mã |
|---|---|---|
| 1 | **Pháp nhân nào VẬN HÀNH backend `vihat-miniapp`** sau khi chuyển | Đây là khẳng định về **BÊN NHẬN DỮ LIỆU** theo Nghị định 13/2023/NĐ-CP, không phải một chuỗi hiển thị |
| 2 | **Biểu tượng của tập đoàn có khác biểu tượng hiện dùng không** — nếu có, tệp vector nằm ở đâu | Kho không có tệp ấy, và vẽ gần đúng là dạng sai khó phát hiện nhất |

**Câu 1 — vì sao hai chiều đều sai.** `citizen-app` gọi `POST /api/v1/sessions` tới backend đó
(`citizen-app/src/features/dang-nhap/hop-dong.ts:47`), và chính sách riêng tư đang **khẳng định
với người dân** rằng hai mã đăng nhập đi tới *"máy chủ của VihatSoftware"*
(`citizen-app/src/content/chinh-sach-rieng-tu.ts:157`, `:205`, `:300`), đồng thời khai
VihatSoftware là **bên chịu trách nhiệm về chính sách** (`:171`).

- Bên vận hành đổi mà câu ấy không đổi → chính sách **sai**: người dân được cho biết một bên
  nhận dữ liệu khác với bên nhận thật.
- Câu ấy đổi mà bên vận hành không đổi → cũng **sai**, theo chiều ngược lại.

Không ai được sửa chuỗi đó theo phản xạ "đổi tên cho đồng bộ". Nó chỉ đúng khi **câu hỏi vận
hành được trả lời trước**. Chưa ai trả lời.

Câu này nay là **câu hỏi mở #28** trong `kb/00-foundation/open-questions.json`, kèm một tín
hiệu để `drift_guard` bắt đúng lần sửa âm thầm ấy — ADR ghi *vì sao*, câu hỏi mở là thứ chạy
mỗi phiên. Câu logo/icon **cố ý không nằm ở đó**: nó là một đầu vào còn thiếu, không phải một
quyết định khách phải ra.

**Câu 2 — vì sao không tự vẽ.** `tools/logo.py` sinh bộ icon Mini App từ
`citizen-app/brand/lg_vhs_full.svg`, và **chỉ lấy phần BIỂU TƯỢNG** nằm bên trái mốc `x = 540` trong viewBox, bỏ phần
chữ *"ViHAT SOFTWARE"* (`tools/logo.py:19-21`, mốc cắt `tools/logo.py:49`). Nếu biểu tượng của
tập đoàn **trùng** biểu tượng hiện dùng thì icon không phải sinh lại — phần chữ vốn đã bị loại.
Nếu **khác**, cần một tệp vector mới; kho chưa có. Chính `tools/logo.py` đã ghi lý do không
được dựng gần đúng: *"gần đúng là dạng sai khó phát hiện nhất — không ai soi một cái icon"*.

## ĐIỀU KIỆN DỪNG

1. Có người đề xuất đổi **OA gửi thông báo của xã** theo lần chuyển này — đọc lại §*Cái gì KHÔNG
   đổi*; đó là vai trò thứ hai và nó không nằm trong quyết định này
2. Zalo trả lời rằng chuyển quyền sở hữu buộc phải **đăng ký lại một Mini App khác** (App ID
   mới) — App ID là thứ ADR 0005 dựng khuôn deep link lên, nên đó là ADR mới, không phải một
   bước thủ tục
3. Có người muốn sửa chuỗi `VihatSoftware` trong **chính sách riêng tư** khi câu CÒN MỞ #1 chưa
   được trả lời
4. Có đề xuất chuyển quyền sở hữu Mini App hoặc OA xác thực sang một pháp nhân khác nữa — ĐIỀU
   KIỆN DỪNG #4 của ADR 0018 vẫn còn hiệu lực, và lần sau cũng là một ADR mới

→ ADR 0018 (tách hai vai trò OA; §*Pháp nhân đứng tên* mà ADR này thay thế; bảng chứng cứ và
  mức nguồn): `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`
→ ADR 0006 (quyết định gốc về kênh thông báo theo xã): `kb/10-decisions/0006-per-commune-zalo-oa.md`
→ ADR 0005 (một Mini App duy nhất, App ID và khuôn deep link): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0020 (đường đăng nhập bằng `getPhoneNumber` — tuyến gọi tới backend nói ở CÒN MỞ #1): `kb/10-decisions/0020-xac-thuc-so-dien-thoai-cong-dan.md`
→ Luật 3 (dữ liệu cá nhân, Nghị định 13/2023): `.claude/rules/critical/3-personal-data.md`
→ Luật 10 (bề mặt người dân nhìn thấy): `.claude/rules/critical/10-citizen-commitment.md`
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md`
