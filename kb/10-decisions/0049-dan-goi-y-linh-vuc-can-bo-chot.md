---
id: 0049-dan-goi-y-linh-vuc-can-bo-chot
tier: T1
source: CURATED
owner: domain
derived_from_commit: 2e9a859
expires: null
owns_facts:
  - "người dân chọn 'lĩnh vực gần đúng nhất' khi gửi phản ánh; lựa chọn ấy là GỢI Ý, không phải lĩnh vực của phiếu"
  - "hạn xử lý xong chỉ tính từ lĩnh vực cán bộ chốt — lĩnh vực gợi ý không bao giờ đặt hay rút ngắn hạn"
  - "vì sao lỗ khai thác hạn mà ADR 0028 chặn vẫn đóng khi dân được chọn lĩnh vực"
---

# 0049. Người dân GỢI Ý lĩnh vực, cán bộ CHỐT lĩnh vực

**Trạng thái:** BỊ THAY bởi ADR 0050 (28/09/2026, cùng ngày) · **Ngày:** 2026-09-28 · **Chủ dự án chốt** (chọn hướng B) · **Sửa một phần ADR
0028** (câu mở #23) — quyết định E và F của 0028 đứng nguyên

## Bối cảnh

ADR 0028 đóng câu #23 **theo uỷ quyền** ("theo rule chung của hành chính xã, bên bạn quyết"): lĩnh vực
chỉ do cán bộ xác định ở bước phân loại; đường "cho dân chọn lĩnh vực lúc gửi" bị bác vì ghép với quyết
định C của ADR 0027 (đổi lĩnh vực sau lần chốt đầu chỉ được RÚT NGẮN hạn) nó mở một lỗ khai thác —
dân chọn `An ninh trật tự` cho một ổ gà thì được hạn gấp, cán bộ không kéo dài được.

Ngày 28/09/2026 chủ dự án chỉ ra prototype Mini App của khách
(https://vigov-production.up.railway.app/, mã nguồn `../vigov-require/apps/miniapp`): bước 1 của luồng
gửi phản ánh là **"Bà con chọn lĩnh vực gần đúng nhất với sự việc"**, mười hai lĩnh vực trùng danh mục
ViGov. Ba hướng được đưa ra; chủ dự án chọn **B**.

## Quyết định

| # | Câu | Trả lời |
|---|---|---|
| 1 | Người dân có chọn lĩnh vực không | **Có** — một bước trong luồng gửi, đúng câu chữ prototype: "lĩnh vực **gần đúng nhất**" |
| 2 | Lựa chọn ấy là gì | Một **gợi ý** gắn vào phiếu (`linh_vuc_goi_y`), hiện cho cán bộ lúc phân loại. **KHÔNG** phải lĩnh vực của phiếu |
| 3 | Lĩnh vực của phiếu | Vẫn do **cán bộ chốt** ở bước phân loại, như ADR 0028 |
| 4 | Hạn | `han_tiep_nhan` như cũ (dòng mặc định, lúc sinh phiếu); `han_xu_ly_xong` **chỉ** đặt lúc cán bộ chốt lĩnh vực (ADR 0028 quyết định E). Lĩnh vực gợi ý **không bao giờ** đặt, rút ngắn hay kéo dài hạn nào |
| 5 | Người dân thấy gì trên phiếu | "Lĩnh vực bạn chọn" (gợi ý) tách khỏi "Lĩnh vực" (cán bộ chốt; "Cán bộ xã chưa phân loại" khi chưa chốt) |

## Vì sao lỗ khai thác của ADR 0028 vẫn đóng

Lỗ ấy chỉ có khi lựa chọn của dân **đặt hạn**. Ở đây nó không đặt gì: hạn xử lý xong vẫn sinh ra ở lần
cán bộ chốt lĩnh vực, từ lĩnh vực cán bộ chọn. Dân chọn `An ninh trật tự` cho một ổ gà thì cán bộ chốt
`Hạ tầng giao thông`, và hạn là hạn của `Hạ tầng giao thông` — lần chốt đầu là lần **ấn định**, không phải
lần **đổi**, nên quyết định C không chạm tới.

Cái được thêm: cán bộ phân loại nhanh hơn vì có một điểm bắt đầu, và người dân thấy app hỏi đúng thứ họ
nghĩ tới trước tiên ("đây là chuyện rác").

## Cái giá — đừng giấu

- **Máy chủ phải đổi.** Hôm nay `POST /api/v1/my-citizen-reports` TỪ CHỐI mọi trường lĩnh vực (400,
  `hop-dong-phan-anh.ts`). Cần: một trường TUỲ CHỌN mới cho gợi ý (luật 2 cấm #4: thêm tuỳ chọn, không
  bắt buộc), một cột ở `service-petitions`, và một tuyến đọc danh mục lĩnh vực của xã cho Mini App (theo
  phiên hoặc `?host=`). Tên trường, hình dạng, chủ sở hữu danh mục: việc của `contract-designer` và của
  câu mở về danh mục (ADR 0024).
- **Danh mục tạm trong Mini App.** Cho tới khi có tuyến đọc, bản trải nghiệm của app riêng dùng mười hai
  tên của prototype, **không kèm số giờ nào** — SLA là cấu hình từng xã (luật 10 cấm #3). Ngày có tuyến,
  danh sách ấy bị gỡ.
- **Một ô dân có thể chọn sai.** Không sao: gợi ý sai chỉ làm cán bộ mất một cú chạm, không làm sai hạn.

## ĐIỀU KIỆN DỪNG

1. Đề xuất để lĩnh vực gợi ý **đặt hay đổi hạn** theo bất kỳ cách nào
2. Đề xuất coi lĩnh vực gợi ý là lĩnh vực của phiếu khi cán bộ chưa phân loại (kể cả trong số liệu báo cáo)
3. Viết số giờ SLA vào danh mục tạm của Mini App

→ ADR 0027 (quyết định C) · ADR 0028 (quyết định E) · ADR 0024 (sở hữu danh mục) · luật 10
