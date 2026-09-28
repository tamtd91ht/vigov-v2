---
id: 0051-english-code-identifiers
tier: T1
source: CURATED
owner: architecture
derived_from_commit: cfdd90a
expires: null
owns_facts:
  - "ngôn ngữ đặt tên định danh, tệp, thư mục, bảng và cột MỚI trong mã nguồn (từ 28/09/2026)"
  - "vì sao đoạn đường dẫn người dùng thấy (route web-admin, màn hình Mini App) vẫn giữ tiếng Việt"
  - "vì sao mã và bảng CŨ đặt tên tiếng Việt không được đổi tên hàng loạt"
---

# 0051. Định danh trong mã nguồn mới viết bằng tiếng Anh

**Trạng thái:** đã chốt · **Ngày:** 2026-09-28 · **Chủ dự án chốt** · **Thay bảng ranh giới ngôn
ngữ của ADR 0001** (`0001-service-decomposition.md:76-83`, dòng "Tên bảng, tên cột") và **dòng "Tên
bảng, cột, định danh nghiệp vụ tầng domain" của ADR 0011** (`0011-contract-surface-language.md:49`)
— cho mã **mới**. ADR 0011 phần đường dẫn URL, tên sự kiện, giá trị enum giữ nguyên; ADR 0014 không đổi.

## Bối cảnh

`CLAUDE.md` đã ghi định danh, tên tệp, chú thích mã viết **tiếng Anh**. Nhưng ADR 0001 và ADR
0011:49 lại chốt bảng, cột và "định danh nghiệp vụ tầng domain" bằng **tiếng Việt không dấu**, và
`kb/00-foundation/ubiquitous-language.md` §Quy ước đặt tên ghi kiểu Go theo nghiệp vụ (`DonThu`). Ba
nguồn nói khác nhau về một fact — đúng thứ luật 9 cấm — và thực tế trôi theo nguồn tiếng Việt:

| Bề mặt | Ví dụ đang có |
|---|---|
| Phương thức Go | `BienBanHopStore.TaoBienBan` — `service-petitions/internal/store/bien_ban_hop_ghi.go:170` |
| Hàm TS | `layDanhSachCanBo` — `web-admin/src/lib/api/can-bo.ts:184` |
| Tệp nguồn | `web-admin/src/features/bien-ban/nhan-bien-ban.ts` |

Tiếng Việt **không dấu** là nguồn đọc nhầm có thật: `ban` là bản / bàn / bán, `hop` là hợp / họp /
hộp. Người đọc phải đoán dấu từ ngữ cảnh, và công cụ, thư viện, người đóng góp sau này không đoán được.

## Quyết định

| Bề mặt | Từ nay |
|---|---|
| Định danh Go/TS trong mã **mới** — hàm, phương thức, kiểu, interface, biến, hằng, trường struct, export | **Tiếng Anh** |
| Tệp và thư mục nguồn **mới** | **Tiếng Anh** — **trừ** đoạn đường dẫn người dùng thấy: thư mục route `app/` của `web-admin` (ví dụ `app/nhiem-vu/bien-ban/`), route màn hình Mini App. Những đoạn này **giữ tiếng Việt** |
| Bảng và cột **mới** (migration) | **Tiếng Anh**. Bảng và cột tiếng Việt đã có **không đụng tới** |
| Tên bucket lưu trữ đối tượng, các đoạn trong khoá đối tượng | **Tiếng Anh** |
| Đường dẫn REST, trường JSON, proto | Đã tiếng Anh (ADR 0011 · 0014 · 0012) — không đổi |
| Chuỗi giao diện, chuỗi cán bộ/dân đọc, văn xuôi `kb/` | **Tiếng Việt** — không đổi |
| Chú thích mã | **Tiếng Anh** — không đổi |
| **Mã ĐÃ CÓ** | **Không đổi tên hàng loạt.** Định danh cũ giữ nguyên; chỉ đổi khi một thay đổi viết lại **cả tệp**, không bao giờ tiện tay (`CLAUDE.md` "Stay in scope"). Một tệp cũ trộn hai thứ tiếng là **chấp nhận được** |

## Vì sao

| Lý do | Nội dung |
|---|---|
| Một ngôn ngữ cho bề mặt mã | Mọi công cụ, thư viện, người đóng góp sau này đọc cùng một thứ tiếng; mã tiếng Anh nằm cạnh thư viện tiếng Anh không cần bước dịch |
| Tiếng Việt không dấu mơ hồ | `ban`, `hop` mỗi chữ ba nghĩa; đã gây đọc nhầm. Tiếng Anh không mất dấu vì không có dấu để mất |
| Ranh giới CSDL ↔ ứng dụng | Bảng mới tiếng Anh thì tên cột, trường struct, trường JSON cùng một từ — bớt một lớp ánh xạ ở mỗi tầng |
| Đường dẫn người dùng thấy giữ tiếng Việt | Cán bộ và dân **nhìn thấy** đoạn ấy trên thanh địa chỉ; đó là giao diện, không phải định danh. Giao diện nói tiếng của người dùng (cùng lý do chuỗi giao diện tiếng Việt) |

**Vì sao không đổi tên mã cũ:** đổi cột đã có dữ liệu là migration rủi ro trên hồ sơ lưu trữ (luật
7); sửa migration đã áp làm `core/migrate` dừng service vì lệch checksum; đổi định danh Go/TS hàng
loạt là một diff khổng lồ không mang giá trị nghiệp vụ, che mất các thay đổi thật khi đọc lịch sử.

## Không thay đổi — nêu rõ để không ai suy rộng

| Thứ | Vẫn theo | Vì sao nêu |
|---|---|---|
| **Giá trị enum** (`khieu-nai`, `da-tiep-nhan`, `thuong_tru`) | Tiếng Việt không dấu — ADR 0011 §"Vì sao giá trị enum KHÔNG dịch" | Giá trị là **dữ liệu** nằm trong hồ sơ lưu trữ, không phải định danh. Dịch nó là khẳng định nghiệp vụ và là di trú hồ sơ (luật 7). Cột **mới** tên tiếng Anh vẫn mang giá trị enum tiếng Việt |
| Khoá quyền (`task.extend`, `feedback.classify`) | Luật 5 bất biến 3b–3c | Khoá phải có sẵn trong bảng `quyen`; đây không phải chỗ đặt tên tự do |
| Đường dẫn URL `/api/v1/…`, tên sự kiện | ADR 0011 | Đã tiếng Anh từ đầu |

## Cái giá

- **Hai thứ tiếng trong một mô-đun, nhiều năm:** mô-đun cũ sẽ trộn `TaoBienBan` với định danh
  tiếng Anh mới. Đây là giá chấp nhận, không phải lỗi cần dọn.
- **Bước dịch khi đọc mã cũ:** người đọc gặp cả `BienBanHop` lẫn `Meeting` cho cùng một khái niệm.
- **Bảng mới nằm cạnh bảng cũ tiếng Việt** trong cùng một CSDL; khoá ngoại từ bảng mới tới bảng cũ
  sẽ trỏ tới cột tên tiếng Việt.
- **Mỗi khái niệm mới cần MỘT tên tiếng Anh đã chốt.** Tự dịch tại chỗ là cách hai phiên đặt hai
  tên cho một thứ — đúng lập luận ADR 0011 §"Chi phí phải trả". Tên tiếng Anh tra ở cột thực thể
  (`@entity`) của `kb/00-foundation/ubiquitous-language.md`; khái niệm chưa có dòng thì dừng và hỏi,
  như bảng ấy đã dặn.

## Chưa chốt — chờ chủ dự án

1. **Thuật ngữ pháp lý không có tương đương tiếng Anh an toàn** (`khieu_nai` / `to_cao` /
   `phan_anh`, `thu_ly` / `tiep_nhan`): ADR 0011 cho thấy dịch sai các từ này làm sai thủ tục. Với
   **định danh** mới chạm các khái niệm này, dùng tên tiếng Anh nào — hay dùng dấu miễn trừ
   `vi-name-ok: <lý do>` — chưa ai quyết.
2. **Bảng mới đặt tên theo thực thể `@entity` đã có, hay dịch lại:** ví dụ bảng mới liên quan biên
   bản họp gọi `meeting_*` (theo `@entity: Meeting`) là cách hiển nhiên, nhưng chưa được chốt thành luật.

## Cưỡng chế

- Luật: `.claude/rules/critical/12-english-identifiers.md`
- Hook: `.claude/hooks/english_identifier_guard.py` — **CHẶN** khai báo **mới** mang tên âm tiết
  tiếng Việt; miễn trừ bằng dấu `vi-name-ok: <lý do>`
- Kỹ năng: `.claude/skills/naming-english`

→ ADR bị thay một phần: `kb/10-decisions/0001-service-decomposition.md:66-84` ·
`kb/10-decisions/0011-contract-surface-language.md:41-50`
→ Bảng ánh xạ khái niệm sang tên: `kb/00-foundation/ubiquitous-language.md`
