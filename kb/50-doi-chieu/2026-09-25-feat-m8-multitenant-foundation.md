---
id: doi-chieu-2026-09-25-feat-m8-multitenant-foundation
tier: T2
source: CURATED
owner: architecture
derived_from_commit: 283e426
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: 233f57f
sha_den: 0053854
ngay_review: 2026-09-25
anh_huong: [service-finance, web-admin]
owns_facts:
  - "khoảng 233f57f..0053854 của ../vigov-require không đổi luật nghiệp vụ nào"
  - "bốn commit thu-chi ngân sách TRƯỚC NEO (bc512e7 · e1a5cf3 · 26b2025 · fff8465) lệch gì so với service-finance và web-admin, và người dùng đã quyết gì ngày 25/09/2026"
---

# Đối chiếu `feat/m8-multitenant-foundation` · 25/09/2026

Ghi chú này có **hai phần khác loại nhau**. Đừng đọc phần B như một khoảng commit.

| Phần | Đọc gì | Khoảng |
|---|---|---|
| A | Phần mới kể từ neo | `233f57f` → `0053854` · 1 commit · đọc ngày 25/09/2026 |
| B | Đợt đối chiếu **trước neo** cho M3 thu-chi ngân sách | **Danh sách commit**, không phải khoảng: `bc512e7` · `e1a5cf3` · `26b2025` · `fff8465` (06/09/2026, cả bốn là tổ tiên của `93cff7f`, đã kiểm bằng `git merge-base --is-ancestor`). Chỉ đọc trong phạm vi đường dẫn nêu ở §B |

`sha_tu`/`sha_den` ở frontmatter là khoảng của **phần A**. Phần B không nới `sha_tu`: nới về `bc512e7^`
là khai đã đọc mọi commit giữa đó, trong khi phần B chỉ đọc bốn commit trong một vùng đường dẫn.

Cây bên kia sạch, đứng đúng branch này, `fetch --all --prune` rồi `pull --ff-only`: *Already up to date*,
`HEAD` = `origin/feat/m8-multitenant-foundation` = `0053854`.

---

## A. Phần mới kể từ neo: không có gì đổi

`0053854` chỉ chụp lại 20 ảnh sổ tay (`apps/admin/public/tai-lieu/huong-dan/anh/*.png`) và sửa một
chú thích trong `apps/admin/public/tai-lieu/huong-dan/index.html` (+3/−1). Không luật nghiệp vụ nào
đổi. Xem §Không ảnh hưởng.

---

## B. Thu-chi ngân sách: đợt đối chiếu trước neo

**Vì sao cần đợt này.** Hai ghi chú trước bắt đầu từ `93cff7f`. Bốn commit dựng thu-chi bên kia nằm
**trước** mốc ấy, nên chưa ghi chú nào so chúng ở mức commit. Ghi chú 23/09 chỉ so hai commit sửa sau
(`502d6f4`, `0142452`), và dòng `0142452` của nó còn thiếu (xem §Đã sửa ghi chú 23/09 ở cuối).

**Phạm vi đọc bên kia.** `apps/api/app/modules/budget/fiscal_*` · migration `0033`, `0034` ·
`apps/admin/src/components/budget/Fiscal*` · `apps/api/app/modules/reports/service.py` `_fiscal_block`.
Trạng thái đọc: **tại `93cff7f`** cho bốn commit, **tại `0053854`** cho `_fiscal_block`, `pick_headline`,
`headline_figures` (cả ba vào ở `0142452`; `git diff 0142452 0053854` không đổi phần thân của chúng).

**Phạm vi đối chiếu kho này.** `service-finance/migrations/0006_thu_chi_ngan_sach.sql` ·
`service-finance/internal/{domain,app,http}/thu_chi_ngan_sach.go` · `service-finance/internal/http/routes.go:883-1126` ·
`web-admin/src/features/thu-chi/*` · `docs/ui-ux/07-thu-chi-ngan-sach.md` · ADR 0035 §A.

## Tóm tắt — kho này phải làm gì

"Đã tiếp nhận" dưới đây nghĩa là **người dùng đã quyết ngày 25/09/2026** trong lượt `/develop-feature`
Thu chi ngân sách. Theo định nghĩa ở `kb/50-doi-chieu/README.md`, ghi chú chỉ **được coi là đã tiếp
nhận** khi `tien-do/service-finance.json` và `tien-do/web-admin.json` trích tên tệp này. Việc ấy thuộc
agent xây module, không thuộc tầng này.

| # | Bên kia làm | Kho này đang có | Đã tiếp nhận (người dùng 25/09) | Module |
|---|---|---|---|---|
| 1 | **Đơn vị**: số lưu **theo đơn vị của tệp** (JSONB chuỗi thập phân), `unit_label` là chữ tự do. Khối tổng quan quy về đồng **chỉ với 4 chữ đã biết**; chữ lạ thì **bỏ số tuyệt đối**, giữ phần trăm | Lưu **đồng, BIGINT** (`0006:13-18`, `:388-390`); `don_vi_tinh` chỉ là nhãn (`:166-167`). Web **in thẳng số đồng, không quy đổi**, và ghi rõ đó là câu chưa chốt (`nhan-thu-chi.ts:38-57`) | **Lưu đồng, web quy đổi** khi hiển thị. Máy chủ giữ nguyên; `nhan-thu-chi.ts:38-57` phải đổi | web-admin |
| 2 | **Dòng tổng mặc định**: chưa ai đánh sao thì **đoán** dòng cấp cao nhất có số thực hiện lớn nhất (`pick_headline`) | Không đoán: không có dòng đánh dấu thì không có tổng, trả câu lý do (`0006:38-66`; nhiều dòng đánh dấu thì từ chối, `domain/thu_chi_ngan_sach.go:696-725`) | Không có quyết định mới. Kho này đã đúng hướng ADR 0035 §A (*"để trống kèm lý do"*). **Không có việc** | — |
| 3 | **Nạp lại tệp**: một bản duy nhất cho mỗi (xã, năm, loại); nạp lại **xoá mềm toàn bộ dòng rồi ghi dòng mới** | Nhập Excel **chưa dựng** (`0006:107-109`). Nạp lại đi qua `🗑 Gỡ` + bảng mới có `lan` mới (`0006:141-149,185`) | Chưa có quyết định. **Chưa rõ ảnh hưởng** tới lượt dựng nhập Excel — câu hỏi ở §Chi tiết, dòng `bc512e7` | service-finance (lượt nhập Excel) |
| 4 | **Ba chế độ** `manual` · `entries` · `children`; ghi **đợt đầu tiên thì tự chuyển** dòng sang `entries` | Chỉ `manual` · `children`; `entries` và bảng đợt **cố ý chưa tạo** (`0006:97-102`, CHECK `:344-345`) | **Dựng đợt theo đặc tả**: dòng lá chọn `manual`/`entries` bằng tay (không tự chuyển); đổi về `manual` thì **giữ số vừa tính** làm số khởi đầu (`07-thu-chi-ngan-sach.md:227`). URL **`budget-entries`** | service-finance · web-admin |
| 5 | **Khoá quyền**: mọi thao tác ghi thu-chi, kể cả gỡ và đánh sao, đều `budget.update` | Ba khoá: `budget.update` cho nhập liệu; `budget.confirm` cho gỡ bảng, gỡ dòng, đánh sao (`routes.go:977-1003,1062-1091,1099-1125`) | **Ghi đợt `budget.update`, gỡ đợt `budget.confirm`**. Ba tuyến `budget.confirm` đang có giữ nguyên | service-finance |
| 6 | **Cột thu dùng cho cân đối**: khối tổng quan lấy cột vai `actual` **đầu tiên** — trên mẫu Thăng Bình là `Thu ngân sách NSNN` | `Cân đối` lấy cột vai `thu-xa-huong` do xã đánh dấu (`domain/thu_chi_ngan_sach.go:90-96,881-888`), theo **#32 DECIDED** | **MÂU THUẪN** với #32 → §Mâu thuẫn. Người dùng quyết: **hỏi khách, tạm đổi nhãn ô KPI** | web-admin |

## Chi tiết theo phân hệ

### M3 — Thu-chi ngân sách

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `bc512e7` | `fiscal_import.py` | Đọc thẳng tệp Excel của xã: tìm hàng tiêu đề bằng ô `TT`/`STT`, cột sinh từ tiêu đề, cấp suy từ số thứ tự (La Mã thử **trước** chữ cái vì `I` luôn là La Mã) rồi từ thụt lề **so với dòng trước**. Cột không tiêu đề thì bỏ. Tỷ lệ % **không tính lại**, lấy nguyên tệp. Sheet không có hàng tiêu đề bị bỏ **im lặng** | **Đầu vào cho lượt dựng nhập Excel** (`07-thu-chi-ngan-sach.md:146-149`, chưa dựng). Hai luật đọc cây đáng mượn nguyên văn. Luật "% lấy nguyên tệp" **ngược** `07:§9 quy tắc 3` của kho này (% không lưu, tính khi hiển thị, `0006:248-252`); kho này đã chọn theo đặc tả của chính nó, không đổi |
| `bc512e7` | `fiscal_service.py` `import_workbook` · `_upsert_report` · `_replace_lines` | Nạp lại = **xoá mềm mọi dòng cũ, ghi dòng mới** vào cùng một bản (hồi sinh cả bản đã gỡ). Lý do bên kia: Phòng Tài chính phát bản **luỹ kế**, cộng thêm là nhân đôi | Kho này đã khác hình: mỗi lần nạp là một `lan` mới, `ma` không cấp lại (`0006:141-149`). **Hệ quả bên kia chưa ghi, đo từ mã**: dòng mới mang `value_mode = manual` và `is_headline = false` (mặc định của cột), còn đợt đã ghi **treo trên dòng đã xoá mềm** (`_replace_lines` không chạm `fiscal_entries`). Nạp lại một lần là mất lặng lẽ cả đánh sao, chế độ tính và đợt thu-chi. **Chưa rõ ảnh hưởng — hỏi người dùng khi mở lượt nhập Excel**: nạp lại trên một bảng đã có đợt thì chặn, mang sang, hay bắt `🗑 Gỡ` trước |
| `bc512e7` | migration `0033_fiscal_report` | `fiscal_reports` `UNIQUE (tenant_id, year, kind)` · `unit_label VARCHAR(64)` · số tiền JSONB theo khoá cột | Duy nhất theo (xã, năm, loại) **tính cả dòng đã xoá mềm**: đúng cái bẫy `0006:141-149` tả và tránh bằng `lan`. Không việc |
| `bc512e7` | `seed_fiscal.py` · `scripts/data/fiscal_structure.json` | Khung khoản mục + tên cột mẫu, đơn vị `Triệu đồng`; số do bộ nạp sinh | Kho này cố ý không nạp dữ liệu mẫu (`0006:81-84`). Không việc |
| `e1a5cf3` | `FiscalReportPanel.tsx` · `useFiscal.ts` | Màn giữ hình biểu mẫu giấy. Thẻ tóm tắt **cộng mọi dòng gốc** | Đúng lỗi đếm đôi mà `0142452` sửa sau đó; `web-admin` đã không có (tổng lấy từ máy chủ, `nhan-thu-chi.ts:6-9`). Không việc |
| `26b2025` | migration `0034_fiscal_entries` · `fiscal_service.py` `_resolve` · `create_entry` · `update_line` | Ba chế độ `manual`/`entries`/`children`; bảng `fiscal_entries` (ngày, nội dung, đối tượng, số chứng từ, số theo cột). **Ghi đợt đầu thì tự chuyển** `manual`→`entries`. Đổi về `manual` thì số hiện ra là `values` **cũ** của dòng (số nhập tay hoặc số nạp trước khi chuyển), không phải số vừa cộng. Cột `role` (`plan`/`actual`/`percent`/`other`) **đoán từ nhãn** để tính lại % | Tóm tắt #4. Người dùng đã chọn **ngược cả hai luật hành vi**: không tự chuyển; đổi về `manual` thì giữ số vừa tính (`07:227`). Cột `counterparty` của đợt có thể là **tên người nộp/người nhận** → khi dựng `dot_thu_chi` phải xét luật 3 (che khi trả ra, không vào log). Vai cột đoán từ nhãn là cái bẫy `0006:223-234` đã từ chối; kho này bắt xã đánh dấu |
| `26b2025` | `test_fiscal_rollup.py` | Cộng dồn: dòng `children` cộng giá trị **đã giải** của con trực tiếp; % không cộng mà tính lại từ cột `plan` và `actual` | Khớp `07:228` (chỉ con trực tiếp). Không việc |
| `fff8465` | `FiscalEntriesDialog.tsx` · `FiscalReportPanel.tsx` | Sửa cây và số ngay trên trang; hộp thoại đợt chỉ cho nhập cột tiền, % không nhập được | Đầu vào cho hộp thoại `⇄` (`07:127-142`) khi dựng đợt. `web-admin` chưa có hộp thoại này |
| `0142452` (sau neo, đọc lại) | `fiscal_service.py` `pick_headline` · `headline_figures` · `reports/service.py` `_to_dong` · `_fiscal_block` | Xem §Đã sửa ghi chú 23/09 | Tóm tắt #1, #2, #6 |

**Hai luật đọc cột khác nhau cùng dựa vào "cột đầu tiên mang vai X".** `headline_figures` lấy cột
`plan` đầu tiên làm mẫu số và cột `actual` đầu tiên làm tử số (`fiscal_service.py:971-974` tại `0053854`).
Trên mẫu Thăng Bình (`fiscal_structure.json:10-29`) thứ tự cột là `Dự toán TP giao` · `Dự toán Xã giao` ·
`Thu ngân sách NSNN` · `Thu xã hưởng`. Vậy mẫu số **trùng #33 do tình cờ thứ tự cột**, còn tử số của cân đối
**trái #32**. Mẫu năm sau đổi thứ tự cột là cả hai con số đổi theo, không gì báo.

## Mâu thuẫn với quyết định đã chốt

**MỘT mâu thuẫn.** Người dùng đã quyết cách xử lý tạm ngày 25/09/2026. Câu gốc vẫn phải hỏi khách.

### `Cân đối thu - chi` lấy `Tổng thu` từ cột nào

**1 · Bên kia nói gì.** `0142452` · `apps/api/app/modules/reports/service.py` `_fiscal_block`
(`:361-406` tại `0053854`): `balance["thu"]` là số `actual` của dòng tổng, và `actual` là cột vai `actual`
**đầu tiên** (`fiscal_service.py` `headline_figures`, `:972-974`). Vai `actual` đoán từ nhãn có chữ *thực
hiện* / *ngân sách* / *đã* (`fiscal_import.py` `_role_of`, `bc512e7`); migration `0034` đặt vai theo khoá
`thu_ngan_sach%`. Trên mẫu Thăng Bình cả hai cột thu đều ra vai `actual`, và cột đứng trước là
`Thu ngân sách NSNN`.

**2 · Kho này đã chốt gì.** Câu hỏi mở **#32**, `DECIDED` 22/09/2026, ADR 0035 §A
(`kb/10-decisions/0035-muoi-lam-cau-mo-quyet-theo-thuc-te-cap-xa.md:61-79`): `Tổng thu` là **`Thu xã
hưởng`**, vì hai cột lệch hơn một triệu đơn vị trên số mẫu, đủ đảo dấu ô Cân đối. Mã:
`service-finance/internal/domain/thu_chi_ngan_sach.go:90-96,881-888`.

⚠ #32 là **đề xuất của nhà cung cấp**, chưa phải câu khách ký (ADR 0035:19,29-34). Bên kia chọn khác
bằng thứ tự cột, không bằng một câu chốt. Vậy chưa bên nào có chữ của khách.

**3 · Ai phải quyết.** Khách. **Người dùng 25/09/2026 đã quyết: hỏi khách; trong lúc chờ, tạm đổi
nhãn ô KPI cân đối.** Chữ cụ thể của nhãn tạm chưa có trong lời quyết; builder `web-admin` phải hỏi lại,
không tự đặt. Mã tính (`CanDoiThuChi`) **không đổi** theo bên kia.

## Không ảnh hưởng

| Commit / vùng | Vì sao bỏ |
|---|---|
| `0053854` (phần A) | 20 ảnh sổ tay chụp lại + một chú thích `index.html`. Tài liệu của kho họ, không luật nghiệp vụ |
| `bc512e7` `test_fiscal_import.py` · `models.py` · `schemas.py` | Bài kiểm và khuôn dữ liệu Python. Luật chúng mang đã ghi ở dòng commit tương ứng |
| `e1a5cf3` `navigation.ts` · `app/(workspace)/giai-ngan/thu-chi/page.tsx` | Đặt màn dưới menu Giải ngân. Kho này có tuyến màn riêng; URL và điều hướng là vùng cố ý lệch |
| Mọi đường dẫn `/api/v1/budget/fiscal/**` | Tên tài nguyên URL cố ý lệch. Kho này dùng `budget-sheets` · `budget-lines` · `budget-indicators` (`routes.go:885-890`) và, theo quyết định 25/09, `budget-entries` |
| RLS `tenant_isolation` + `GRANT SELECT, INSERT, UPDATE` trong `0033`, `0034` | Kiến trúc cố ý lệch: kho này cách ly ở tầng Go và chặn xoá cứng bằng trigger (`0006:208-211`) |
| `26b2025` `_is_descendant` · `_redepth` · `_slot_after` (đổi cha, chèn dòng) | Cách cài đặt. Luật "không đưa dòng vào nhánh của chính nó" kho này đã có (`0006:323-327,347-349`) |

---

## Vì sao `anh_huong` là hai module này

| Module | Việc thật sự đòi đổi mã hoặc thiết kế |
|---|---|
| `service-finance` | Dựng `dot_thu_chi` + mở CHECK `cach_tinh` thêm `entries` + tuyến `budget-entries` (ghi `budget.update`, gỡ `budget.confirm`); luật đổi về `manual` giữ số vừa tính (Tóm tắt #4, #5). Câu nạp-lại-khi-đã-có-đợt phải có trước lượt nhập Excel (#3) |
| `web-admin` | Quy đổi đơn vị khi hiển thị (#1, `nhan-thu-chi.ts:38-57`); hộp thoại `⇄` và công tắc chế độ trên dòng lá (#4); nhãn tạm của ô cân đối (#6) |

**Cố ý không đưa vào:** `service-reporting` (chưa dựng; `/tong-quan` đọc chỉ số từ `service-finance`
qua `budget-indicators`, `routes.go:930-950`, nên luật #32 nằm ở `service-finance`) · `service-identity`
(không khoá quyền mới: `budget.update` và `budget.confirm` đã nạp ở `service-identity/migrations/0001_init.sql:282-284`,
theo `routes.go:901-902`).

---

## Đã sửa ghi chú 23/09 (dòng `0142452`)

Sửa tại chỗ trong `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md`: dòng `0142452` ở §M3
giữ nguyên chữ cũ, thêm dấu **SỬA 25/09**; bảng §Đã sửa của tệp ấy thêm hàng 7. Lý do và `file:line` nằm
ở đó, không chép sang đây.
