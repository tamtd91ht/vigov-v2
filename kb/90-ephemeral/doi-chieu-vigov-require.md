---
id: doi-chieu-vigov-require
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 4c96e9a
expires: 2026-12-22
owns_facts:
  - "mức độ lệch giữa đặc tả ../vigov-require/docs/spec và kho vigov-v2, đo ngày 2026-09-22"
  - "bản chất quan hệ giữa hai kho: vigov-require LÀ nguồn của prototype mà docs/ui-ux chép ra"
  - "những điểm đặc tả vigov-require TRẢ LỜI ĐƯỢC câu hỏi mở của kho này"
  - "sổ việc đối chiếu hai kho: mục nào đã kiểm, mục nào còn nợ, mục nào chờ khách"
---

# Đối chiếu `../vigov-require/docs/spec` với kho này

**Đo ngày 2026-09-22**, trên `vigov-require@b159f0e` và `vigov-v2@4c96e9a`. Hết hạn
**22/12/2026** — sau đó một trong hai kho đã đi xa, đừng tin tệp này nữa.

Tệp này **chỉ đọc tài liệu**, chưa đọc mã của `vigov-require`. Mọi con số dưới đây đều
trích từ tệp có đường dẫn ghi kèm.

---

# SỔ VIỆC — sửa dần, đừng xoá dòng

**Đây là phần sống của tệp.** Mọi phần dưới §0 trở đi là **bằng chứng đã đo ngày 22/09/2026**
— đọc thì được, sửa thì chỉ khi đo lại. Phần này mới là chỗ ghi tiếp.

**Luật ghi:** một mục xong thì **đổi trạng thái và ghi bằng chứng `file:line` + ngày**, KHÔNG
xoá dòng. Dòng đã xoá là dòng phiên sau lại đi đo lại từ đầu — đúng cái giá tệp này sinh ra để
tránh. Một mục hoá ra sai thì ghi **vì sao sai**, cũng không xoá.

**Trạng thái:** `chưa làm` · `ĐANG LÀM` · `xong` · `treo` (chờ thứ ngoài tầm) · `hỏi khách`

## D — Điểm lệch cần QUYẾT (không ai trong phiên tự quyết được)

| # | Việc | Trạng thái | Bằng chứng / ghi chú |
|---|---|---|---|
| D1 | **Hỏi khách câu #27** kèm hai dữ kiện mới: nhóm quyền thứ 11 là `dossier.*`, và **luật nắm giữ** (§4.1, §4.3) | `hỏi khách` | Đang chặn 6 mục ở 5 module. Trước đây hỏi tay không; nay có bằng chứng |
| D2 | **Hỏi chủ dự án: có bật RLS trong Postgres không** (§2) | `hỏi khách` | Đo 22/09: `grep -ri "ROW LEVEL SECURITY"` trên 31 migration = **0**. Đắt dần theo số xã đã chạy → hỏi muộn là hỏi đắt |
| D3 | **Ràng buộc hạ tầng đặt tại Việt Nam** — kèm ADR 0020, `citizen-app` đang chờ (§1) | `chưa làm` | Bên kia có bằng chứng Zalo cưỡng chế bằng IP: định danh lấy được, **số điện thoại và vị trí thì không** |
| D4 | Phạm vi **Hồ sơ công dân** — khách đã chốt ngoài phạm vi 20/09/2026; chỉ mở lại nếu khách đổi ý (§3.1) | `treo` | ADR 0001 §Bổ sung 2026-09-20. Thiết kế bên kia dùng lại được nguyên vẹn nếu mở lại |
| D5 | Cặp trạng thái `out_of_scope` ↔ `chuyen-cap-tren` — **không hiển nhiên**, tám cặp kia khớp (§3.2) | `chưa làm` | Kiểm nghĩa trước khi ai đó ánh xạ vội. Đổi một trong chín chuỗi từ 20/09 là **di trú hồ sơ lưu trữ** (luật 7) |

## K — Phép kiểm cần CHẠY trên kho này (rẻ, đo được ngay)

Cả năm mục đều là **bẫy bên kia đã trả giá** (§5). Chưa mục nào được kiểm.

| # | Việc | Trạng thái | Bằng chứng / ghi chú |
|---|---|---|---|
| K1 | ~~Quét migration tìm `UNIQUE (…)` **thiếu** `WHERE deleted_at IS NULL`~~ → **MỤC NÀY VIẾT NGƯỢC, đã bác 23/09/2026** | `xong` | **HAI KHO ĐỐI LẬP CÓ CHỦ Ý Ở ĐÚNG ĐIỂM NÀY — xem §2.1 dưới.** `tools/check_khoa_duy_nhat.py:175-186` **TỪ CHỐI** hình dạng ấy và gọi tên nó là bẫy. Làm theo mục K1 như viết ban đầu là làm `make check` đỏ. Cổng chạy 23/09: `[PASS] 36 tệp · 54 khai báo · 0 khoá tính sót dòng đã xoá mềm` |
| K2 | Kiểm đường gọi Zalo của `service-comms` có ghi URL ra log không | `chưa làm` | Bên kia: mã bot nằm **trong đường dẫn**, thư viện HTTP ghi URL ở mức INFO → dòng log vô hại chính là bí mật. Luật 8 không lường đường vòng này |
| K3 | `GET /api/v1/my-citizen-reports/{maTraCuu}` — có **giới hạn số lần tra** không | `chưa làm` | Luật 4 cấm #2 nói vế 404-vs-403, **không nói** vế giới hạn số lần. Bên kia: 20 lần / 10 phút / một địa chỉ |
| K4 | `service-finance` — `chung_tu_giai_ngan` có cột **nguồn vốn** không | `chưa làm` | `migrations/0004` có `du_an_id`. Bên kia: *"không có cột ấy thì tiến độ theo nguồn phải suy từ dự án, và suy sai ngay khi dự án có hai nguồn"* |
| K5 | Mọi nhánh "bỏ qua" ở kênh thông báo có **ghi log kèm lý do** không | `chưa làm` | Bên kia có sáu lý do bỏ qua chính đáng; không ghi thì cả sáu nhìn hệt như hỏng |

## N — Nghiệp vụ nên MƯỢN khi phân hệ tới lượt (§6)

Không phải lệch — là **khuyết**. Đọc bên kia trước rẻ hơn tự nghĩ lại. Mục nào chưa tới lượt
thì để `chưa làm`, đừng dựng sớm.

| # | Phân hệ / việc | Trạng thái | Nguồn bên kia |
|---|---|---|---|
| N1 | **M1 Nhiệm vụ** — giao việc không có ô "người thực hiện"; "Chờ duyệt lùi hạn" là NHÃN không phải trạng thái; bàn giao ghi từ-ai-sang-ai | `chưa làm` | `05-nghiep-vu.md` §M1 |
| N2 | **Việc nền + thông báo** — một ngưỡng `warn_before_hours` duy nhất; bản tin gộp một tin/người/ngày; giờ yên tĩnh | `chưa làm` | `07-viec-nen-va-thong-bao.md`. ADR 0029 có số giờ cam kết, **chưa có** ngưỡng cảnh báo sớm |
| N3 | **M2** — nhập Excel kiểm cả tệp trước khi ghi; cảnh báo trùng đơn (unaccent + pg_trgm) lúc còn ở quầy | `chưa làm` | `05-nghiep-vu.md` §M2 |
| N4 | **M3 Giải ngân** — tiến độ tính từ **ngày khởi công**, không phải tháng 1→12 | `chưa làm` | `05-nghiep-vu.md` §M3. Trùng vùng câu hỏi mở #31 |
| N5 | **M3 Thu-chi** — đơn vị tính đọc từ chính tệp; đơn vị lạ thì **bỏ hẳn số tuyệt đối**, chỉ giữ phần trăm | `chưa làm` | `05-nghiep-vu.md` §M3. Trùng vùng #32, #33 |
| N6 | **M7 Báo cáo** — mẫu số rỗng trả `None` (dấu gạch), **không** trả 0; mỗi số bấm sâu ra đúng bấy nhiêu dòng | `chưa làm` | `05-nghiep-vu.md` §M7. `service-reporting` chưa dựng — **ghi trước khi dựng** |
| N7 | **Zalo Bot** làm kênh nhắc việc cán bộ — bot không nhắn trước được, ghép nối bằng mã 8 ký tự sống 10 phút | `chưa làm` | `08-tich-hop-ngoai.md`. **Kho này không có khái niệm Zalo Bot nào** |
| N8 | **Cổng TTĐT của xã** → `content_sources`, đồng bộ tin về Mini App | `chưa làm` | `08-tich-hop-ngoai.md`. `service-comms` chưa có |

## X — Việc của chính tệp này

| # | Việc | Trạng thái | Ghi chú |
|---|---|---|---|
| X1 | **Đọc MÃ `vigov-require`** — chỉ sau khi D1–D3 đã rõ | `chưa làm` | Đọc 49 migration + 16 module trước khi biết định hỏi gì là đọc để tìm câu mình chưa đặt |
| X2 | Đối chiếu **22 câu Q-01…Q-22** của bên kia với 34 câu của kho này | `chưa làm` | `../vigov-require/docs/open-questions.md` — **bộ khác hẳn**, chưa đối chiếu dòng nào |
| X3 | Đo lại toàn bộ tệp khi tới hạn | `chưa làm` | Hạn **22/12/2026**. Quá hạn nghĩa là một trong hai kho đã đi xa — đo lại, đừng tin |

---

## 0. Điều phải hiểu trước mọi so sánh

`vigov-require` **không phải một bản yêu cầu mới**. Nó là **bản cài đặt đang chạy** mà
`docs/ui-ux/` của kho này đã chép ra.

| Bằng chứng | Ở đâu |
|---|---|
| `docs/ui-ux/README.md` khai nguồn: *"mô tả đầy đủ bản web prototype ViGov … `vigov-admin-production.up.railway.app` … Tài liệu lập ngày 16/9/2026 từ bản prototype đang chạy"* | `docs/ui-ux/README.md` cuối tệp |
| `vigov-require` triển khai bằng Railway, đúng cái tên miền ấy | `../vigov-require/docs/spec/01-kien-truc.md` §Triển khai |
| `vigov-require/vigov-prototype.html` — 144 KB, chính bản mẫu | gốc kho |
| Commit `a878889 docs: a specification complete enough to rebuild the system from nothing` | `git log` của kho ấy |

**Hệ quả quan trọng nhất:** hai kho **không mâu thuẫn về NGHIỆP VỤ** — chúng cùng một sản
phẩm. Chúng lệch ở **hai trục khác**:

1. **Kiến trúc** — lệch gần như toàn phần, và đó là lệch **có chủ ý đã trả tiền**, không
   phải lỗi.
2. **Độ phủ đặc tả** — `vigov-require` có phần mà kho này **chưa bao giờ có**: mô hình dữ
   liệu 56 bảng, 253 đường API, luật nghiệp vụ M1–M8, việc chạy nền, tích hợp Zalo Bot.
   `docs/ui-ux/` của kho này chỉ tả **màn hình**.

---

## 1. Lệch kiến trúc — gần như toàn phần

| Hạng mục | `vigov-require` | `vigov-v2` (kho này) | Lệch |
|---|---|---|---|
| Backend | **FastAPI · Python 3.12**, monolith chia module | **Go microservices**, 7 dịch vụ | TOÀN PHẦN |
| Ranh giới dịch vụ | 16 module trong một tiến trình, gọi nhau qua `service.py` | 7 dịch vụ tách tiến trình, gRPC + sự kiện | TOÀN PHẦN |
| Hợp đồng | OpenAPI sinh từ FastAPI | `.proto` là nguồn chuẩn giữa dịch vụ (luật 2 #7); REST sinh từ mã Go (ADR 0014) | TOÀN PHẦN |
| ORM / CSDL | SQLAlchemy 2.0 async · PostgreSQL 16 + PostGIS + pgvector | SQL viết tay · `core/store` · PostgreSQL 16 | LỚN |
| Migration | Alembic — **49 bản** ở `apps/api/migrations/versions/` | SQL đánh số theo dịch vụ — **31 tệp** ở `service-*/migrations/` | LỚN |
| Cách ly xã | **RLS trong Postgres** + `FORCE ROW LEVEL SECURITY` + REVOKE DELETE khỏi vai trò `vigov_app` | **Kho có phạm vi ở tầng ứng dụng** (`core/tenant`, `core/store`) + rào hook + `tools/check_*` | **XEM §2** |
| Hàng đợi | ARQ trên Redis | sự kiện qua `core/events` | LỚN |
| Lưu tệp | MinIO tự vận hành | `core/storage` (ADR 0001 khai cố ý KHÔNG phải dịch vụ) | VỪA |
| AI | PaddleOCR + VietOCR + vLLM/Qwen **đặt tại IDC Việt Nam**, cấm gọi API ngoài | **kho này chưa có tầng AI nào** | KHUYẾT |
| Web cán bộ | Next.js App Router · React 19 · Tailwind v4 · shadcn/ui · TanStack Query | Next.js (`web-admin/`) | NHỎ |
| Mini App | `zmp-cli` v4 · React 18 · Vite 5 | `citizen-app/` · Vite | NHỎ |
| Quản trị nền tảng | `apps/platform` — ứng dụng riêng, tên miền riêng | `platform-admin/` — **chưa có màn hình nào** | KHUYẾT |
| Triển khai | Railway | k8s + Jenkins (`deploy/`, ADR 0015) | TOÀN PHẦN |

**Đọc bảng này thế nào:** trục kiến trúc lệch toàn phần là **thông tin, không phải vấn đề** —
trừ khi có ai định hợp nhất hai kho. Kho này đã trả tiền cho quyết định vi dịch vụ ở ADR 0001,
và `vigov-require` đã trả tiền cho quyết định monolith ở `01-kien-truc.md`. Cả hai đều có lý
do viết ra.

### Điều đáng chú ý nhất về kiến trúc

`vigov-require` nói thẳng một câu kho này **chưa có ở đâu**:

> *"Ràng buộc pháp lý do chủ đầu tư chốt: dữ liệu chính quyền không rời hạ tầng đặt tại Việt
> Nam. Không ngoại lệ, kể cả môi trường phát triển có dữ liệu thật."*
> — `01-kien-truc.md` §AI

Và nó kèm **bằng chứng ràng buộc ấy đã được cưỡng chế bằng kỹ thuật**:

> *"Zalo chặn lấy thông tin cá nhân theo IP máy chủ … Máy chủ đặt ngoài Việt Nam nhận đúng
> câu: 'Personal information is limited due to IP address not inside Vietnam'. Định danh thì
> vẫn lấy được … Số điện thoại và vị trí thì không."*
> — `09-bay-va-bai-hoc.md` §Zalo

**Đây là sự thật kỹ thuật ảnh hưởng thẳng tới `citizen-app` và tới ADR 0020 (xác thực số
điện thoại công dân) của kho này.** Nó không phải điều khoản giấy tờ. → §5, mục cần quyết.

---

## 2.1 KHOÁ DUY NHẤT — hai kho đối lập nhau có chủ ý, và cả hai đều đúng với luật của mình

Phát hiện ngày **23/09/2026**, khi mục K1 của Sổ việc suýt được thi hành nguyên văn.

| | Làm gì | Vì sao |
|---|---|---|
| `vigov-require` | **LUÔN** kèm `WHERE deleted_at IS NULL` | *"chặn đúng thao tác gỡ-ra-rồi-thêm-lại mà người dùng làm hằng ngày"* — `09-bay-va-bai-hoc.md` |
| **kho này** | **CẤM** mệnh đề ấy trên khoá duy nhất | **Luật 7 bất biến 3: mã đã cấp KHÔNG BAO GIỜ được cấp lại, kể cả sau xoá mềm.** Cổng `tools/check_khoa_duy_nhat.py:175-186` chặn, và câu chặn viết: *"Mã đã cấp là mã đã in ra giấy, đã đóng dấu, đã gửi đi. Hai hồ sơ cùng số trong sổ lưu trữ là hai hồ sơ không ai phân biệt được nữa — và một trong hai đã có người ký."* |

**Không bên nào sai.** Bên kia tối ưu cho thao tác hằng ngày; kho này tối ưu cho **tính toàn vẹn
của hồ sơ lưu trữ**, và đó là ràng buộc pháp lý chứ không phải khẩu vị.

**Vì sao mục này đáng nằm ở đây thay vì bị xoá đi:** K1 được viết ngày 22/09 từ một bài học có
thật của kho bên kia, và nó **đọc rất thuyết phục**. Ngày 23/09 nó được đưa nguyên văn vào chỉ
dẫn cho một agent dựng migration; agent mở cổng ra đọc, thấy cổng nói ngược, và **làm theo cổng**.
Nếu nó làm theo chỉ dẫn thì `make check` đỏ — còn nếu cổng ấy chưa tồn tại thì một mã hồ sơ đã
cấp sẽ được cấp lại, và không gì đỏ cả.

**Bài học không phải "kiểm cổng trước".** Là: một bài học đúng ở kho khác **không tự động đúng ở
kho này**, và chỗ nguy hiểm nhất là khi nó đúng *nghe có vẻ vì cùng một lý do*.

## 2. Điểm lệch NẶNG nhất: cách ly xã đặt ở tầng nào

Đây là chỗ duy nhất trong toàn bộ đối chiếu mà hai bên **cùng đòi một thứ nhưng đặt rào ở hai
tầng khác nhau**, và chênh lệch đó có thể đo được.

| | `vigov-require` | `vigov-v2` |
|---|---|---|
| Rào chính | `ENABLE ROW LEVEL SECURITY` + `FORCE ROW LEVEL SECURITY` trên **mọi bảng nghiệp vụ**, policy đọc `current_setting('app.tenant_id')` | Kho có phạm vi ở tầng Go, `tenant_id` đi trong `context.Context` (luật 1 bất biến 4–5) |
| Chống xoá cứng | Vai trò CSDL `vigov_app` **không có quyền DELETE** → xoá cứng là **không thể** | `hooks/data_safety_guard.py` (BLOCK) + soft delete theo quy ước |
| Đo được trong kho này | — | **`grep -ri "ROW LEVEL SECURITY" --include=*.sql` trên 31 tệp migration: 0 kết quả** |

**Kho này đã biết khoảng trống ấy và đang treo nó**: `kb/90-ephemeral/tien-do/_chung.json`
→ mục `revoke-audit-log` — *"REVOKE trên audit_log thuộc khâu cấp phát CSDL, không nằm trong
tay mã nguồn | treo"*.

**Vì sao đáng nêu lên chứ không im lặng.** Rào ở tầng ứng dụng chỉ đứng được chừng nào **mọi**
đường đọc đi qua nó. Rào ở tầng CSDL đứng được kể cả khi một đường đọc đi vòng — mà kho này
vừa đo được đúng lớp lỗi ấy hai lần trong tháng: ba khoá quyền bịa sống qua nhiều phiên
(`tools/check_quyen.py` dòng mở đầu), và sáu đường ghi đặt `Principal.ID` vào `actor_id`
(luật 6 bất biến 8). **Cả hai đều xanh cho tới khi có người quét toàn kho.**

`vigov-require` không cần quét vì Postgres từ chối luôn.

→ **Đây là câu nên hỏi chủ dự án**, không phải câu người viết mã tự quyết. Nó là thay đổi ở
khâu cấp phát CSDL, chạm mọi dịch vụ, và đắt dần theo số xã đã chạy.

---

## 3. Lệch nghiệp vụ — ÍT, nhưng có ba chỗ thật

Nghiệp vụ hai bên trùng nhau cao vì cùng một prototype. Ba chỗ dưới đây là lệch thật.

### 3.1 Hồ sơ công dân — kho này ĐÃ GỠ, `vigov-require` ĐANG CHẠY

| | |
|---|---|
| `vigov-require` | Phân hệ đầy đủ: bảng `citizen_dossiers`, **7 đường** `/api/v1/dossiers/*`, ba khoá quyền `dossier.{read,import,update}`, màn `/ho-so-cong-dan`, nhập từ tệp Hệ thống Một cửa, tra cứu phía công dân bằng **số hồ sơ + 4 số cuối điện thoại** |
| `vigov-v2` | `dossiers` **gỡ khỏi kho 20/09/2026** — khách chốt hồ sơ một cửa **không thuộc phạm vi hợp đồng** (ADR 0001 §Bổ sung 2026-09-20) |

**Không phải mâu thuẫn — là hai phạm vi hợp đồng khác nhau.** Nhưng nếu khách đổi ý, đặc tả
`05-nghiep-vu.md` §Hồ sơ công dân là bản thiết kế sẵn sàng dùng lại, kể cả những chỗ đã trả
giá (khớp cột theo **tên cột** chứ không theo vị trí; dòng tiêu đề nằm quanh dòng 8; khoá tự
nhiên là số hồ sơ nên nhập lại là cập nhật chứ không nhân đôi; giữ **nguyên văn** chữ trạng
thái của hệ thống nguồn).

### 3.2 Đồng hồ hạn của phiếu phản ánh — kho này ĐI XA HƠN

| | `vigov-require` | `vigov-v2` |
|---|---|---|
| Cột hạn | **một** — `feedbacks.due_at` (`03-mo-hinh-du-lieu.md` §`feedbacks`) | **hai** — `han_tiep_nhan` và `han_xu_ly_xong` (ADR 0028, luật 10 bất biến 2) |
| Trạng thái | 9 mã, **chuỗi tiếng Anh** (`received`, `screening`, `assigned`, …) | 9 mã, **tiếng Việt không dấu** (`da-tiep-nhan`, `dang-phan-loai`, …) — **khách DUYỆT NGUYÊN VĂN 20/09/2026** |
| `is_overdue` | không có cột — **khớp** luật 10 bất biến 3 | không có cột |

⚠ **Chín chuỗi tiếng Anh trong `vigov-require` chính là chín chuỗi mà
`kb/00-foundation/ubiquitous-language.md` §Phản ánh gọi tên là "tên ĐÃ BỊ THAY".** Gặp lại
chúng ở đặc tả bên kia **không** phải một cách gọi thứ hai đang song song — kho này đã đổi có
chủ ý (ADR 0011 cấm dịch giá trị enum sang tiếng Anh), và từ 20/09/2026 đổi một trong chín
chuỗi là **di trú hồ sơ lưu trữ** (luật 7).

⚠ **Một cặp cần kiểm lại chứ đừng ánh xạ vội:** `out_of_scope` (ngoài phạm vi) ↔
`chuyen-cap-tren` (chuyển cấp trên). Tám cặp kia khớp nghĩa; cặp này **không hiển nhiên**.

### 3.3 Danh tính công dân và cách nhận xã — LUẬT 1 VÀ LUẬT 4 CỦA KHO NÀY CẤM CÁCH BÊN KIA LÀM

Đây là lệch **thật sự đối lập**, không phải khác khẩu vị.

| `vigov-require` (`04-api.md` §Quy ước chung) | Kho này |
|---|---|
| *"Xã xác định theo tên miền con, **hoặc header `X-Tenant-Code` khi dùng tên miền chung** (bản trên Railway đang dùng cách sau)"* | **Luật 1 cấm #2** — *"Accepting `tenant_id` from body / query / client header"*. Reverse proxy phải **xoá sạch** mọi header tenant từ ngoài vào (`multi-tenant-model.md` §Ranh giới tin cậy) |
| *"danh tính công dân đi qua header **`X-Citizen-Id`**"* | **Luật 4 bất biến 2** — *"Citizen identity comes from the session, never from a request parameter"*. Ba lớp Khám phá / Phiên / Uỷ quyền, và lớp Khám phá **không tin được** (ADR 0022) |

**Cả hai đều có lý do.** Bên kia đang chạy trên Railway một tên miền chung nên không có tên
miền con để suy. Kho này chọn phát hành phiên sau khi công dân **xác nhận bằng một hành vi rõ
ràng**, vì gửi nhầm xã là sự cố nghiệp vụ thật.

**Nếu có ai định mượn mã hay mượn thiết kế API từ bên kia, đây là chỗ phải dừng lại trước.**

---

## 4. `vigov-require` TRẢ LỜI ĐƯỢC câu hỏi mở của kho này

Đây là giá trị lớn nhất của bộ đặc tả ấy đối với kho này. Ba mục dưới đây có bằng chứng.

### 4.1 Câu #27 — "nhóm quyền THỨ MƯỜI MỘT là nhóm gì" → **`dossier.*`**

Câu hỏi mở #27 của kho này: *"Đặc tả ghi tiêu đề 43 quyền / 11 nhóm nhưng chỉ liệt kê 33 khoá
/ 10 nhóm: TÁM khoá còn thiếu là những khoá nào, và NHÓM THỨ MƯỜI MỘT là nhóm gì?"*

`02-da-tenant-va-bao-mat.md` §Quyền liệt kê **đúng mười một nhóm**:

> `task.*`, `document.*`, `petition.*`, **`dossier.*`**, `budget.*`, `feedback.*`, `asset.*`,
> `report.*`, `content.*`, `announcement.*`, `admin.*`

Đối chiếu khoá thật (trích từ cột Quyền của `04-api.md` so với migration của kho này):

| | |
|---|---|
| Chỉ có ở `vigov-require` | `dossier.read` · `dossier.import` · `dossier.update` |
| Chỉ có ở kho này | `feedback.classify` · `feedback.restricted` · `feedback.unmask` · `task.approve` |
| Trùng nhau | **31 khoá** (đo bằng `comm -12` trên hai danh sách đã sắp; kho này 35 khoá, bên kia 34) |

**Nhóm thứ mười một là `dossier.*` — và nó biến mất khỏi kho này đúng vì `dossiers` bị gỡ
20/09/2026 (§3.1).** Điều đó giải thích luôn vì sao đếm được 10 nhóm chứ không 11.

⚠ **KHÔNG được coi đây là đáp án đã chốt.** Ba khoá `dossier.*` giải thích được nhóm thứ
mười một, **không** giải thích được đủ tám khoá còn thiếu, và luật 5 bất biến 3c nói rõ:
thiếu khoá là **phát hiện cho câu #27**, không bao giờ là một `INSERT` mới. Đây là **bằng
chứng để mang đi hỏi khách**, không phải giấy phép tự thêm khoá.

### 4.2 Câu #4 — đọc chéo nhiều xã

`vigov-require` trả lời bằng **cấu trúc**, và trả lời chặt hơn kho này đang tạm:

- `tenant_session(None, use_app_role=False)` — phiên đọc được mọi xã — **chỉ** dành cho module
  `platform` và việc cấp phát xã mới.
- **Không có tính năng đăng nhập thay mặt người khác.** *"Quyết định của chủ đầu tư, vì lý do
  bảo mật. Người vận hành nền tảng không vào được web của xã."*
- Chính sách tra cứu không-gắn-xã **chỉ đặt trên bốn bảng**, và **chỉ cho ĐỌC**:
  `tenants`, `zalo_bot_links`, `zalo_bot_link_codes`, `zalo_bot_settings`.

Điều này **khớp** ADR 0003 của kho này (platform chỉ siêu dữ liệu) và câu #3 đã DECIDED.
Nhưng nó **chưa trả lời** phần khó của #4: cấp huyện/tỉnh xem tổng hợp tới mức nào, và lĩnh
vực hạn chế `can-bo` có rời khỏi xã không. **`vigov-require` không có khái niệm cấp huyện/tỉnh
nào cả.** Câu #4 vẫn mở.

### 4.3 Lớp "quyền theo vai trò không đủ" — khái niệm kho này chưa có tên

> **Luật nắm giữ** — *"người đang giữ một bản ghi được làm việc trên chính bản ghi ấy, dù vai
> trò của họ không có quyền tương ứng. Trưởng thôn được giao một phản ánh thì đổi được trạng
> thái phiếu ấy, dù vai trò `truong-thon` không có `feedback.resolve`."*
> — `02-da-tenant-va-bao-mat.md`

Kho này **chưa có khái niệm này ở đâu**: luật 5 chỉ nói `(tenant_id, role, permission)`. Nếu
luật nắm giữ là đúng nghiệp vụ — mà lý do nó nêu rất thuyết phục: *"không có luật này thì hệ
thống giao việc cho người ta rồi chặn họ làm"* — thì đây là **lỗ hổng mô hình phân quyền của
kho này**, và nó nằm đúng trong vùng câu #27 đang chặn.

→ **Đề xuất: mang §4.3 hỏi khách CÙNG câu #27.** Hai câu này là một câu.

---

## 5. Những cái bẫy `vigov-require` đã trả giá mà kho này CHƯA gặp

`09-bay-va-bai-hoc.md` là tệp có tỷ lệ giá trị trên số chữ cao nhất trong cả bộ. Sáu mục dưới
đây áp thẳng vào kho này:

| Bẫy | Áp vào đâu ở kho này | Trạng thái |
|---|---|---|
| **Khoá duy nhất phải loại trừ dòng đã xoá mềm** — `UNIQUE (tenant_id, code)` trên bảng có `deleted_at` chặn đúng thao tác "gỡ ra rồi thêm lại" mà người dùng làm hằng ngày. Luôn kèm `WHERE deleted_at IS NULL` | **31 tệp migration** của kho này, luật 1 bất biến 6 (khoá composite) + luật 7 (soft delete) — hai luật ấy **gặp nhau đúng ở đây**, và không luật nào nói câu này | **CHƯA KIỂM** |
| **Mã bí mật lọt vào log qua thư viện HTTP** — Zalo Bot đặt mã bot **ngay trong đường dẫn**, `httpx` ghi URL ở mức INFO | Kho này gọi Zalo từ `service-comms` (ZNS theo OA từng xã, ADR 0018). Luật 8 cấm bí mật trong log nhưng **không ai nghĩ tới đường vòng qua URL của thư viện HTTP** | **CHƯA KIỂM** |
| **Zalo chặn lấy số điện thoại và vị trí theo IP máy chủ** — định danh thì lấy được, số điện thoại thì không | ADR 0020 (xác thực số điện thoại công dân) + `citizen-app` | **CẦN QUYẾT** |
| **Endpoint không đăng nhập phải trả CÙNG MỘT CÂU cho mọi kiểu sai** + giới hạn số lần tra | `GET /api/v1/my-citizen-reports/{maTraCuu}` — luật 4 cấm #2 nói vế 404-vs-403, **không nói vế giới hạn số lần** | **CHƯA KIỂM** |
| **Chia cho mẫu số rỗng trả `None` (hiện dấu gạch), KHÔNG trả 0** — *"xã chưa kết thúc phiếu nào trong tuần mà thấy 'Đúng hạn 0,0%' màu đỏ sẽ hiểu là mình hỏng hết"* | `service-reporting` (chưa dựng) | **GHI LẠI TRƯỚC KHI DỰNG** |
| **Mọi nhánh "bỏ qua" hợp lệ phải ghi log kèm lý do** — kênh Zalo có sáu lý do bỏ qua chính đáng; không ghi thì cả sáu nhìn hệt như hỏng | `service-comms`, mọi kênh thông báo | **CHƯA KIỂM** |

---

## 6. Nghiệp vụ `vigov-require` tả KỸ mà kho này chưa có tài liệu tương đương

Không phải lệch — là **khuyết**. Nếu những phân hệ này sắp tới lượt, đọc bên kia trước sẽ rẻ
hơn tự nghĩ lại.

| Nội dung | Ở đâu bên kia | Kho này |
|---|---|---|
| **Giao việc không có ô "người thực hiện"** — chỉ có cơ quan chủ trì + người theo dõi + lãnh đạo giao. Hệ quả: nhiệm vụ mới thường **chưa có ai đích danh**, phải báo cho **cả bộ phận chủ trì** | `05-nghiep-vu.md` §M1 | `service-petitions` — chưa có bảng giao việc nào (`tien-do/service-identity.json` → `ban-giao-viec-khi-khoa-tai-khoan` đang treo vì lý do này) |
| **"Chờ duyệt lùi hạn" là NHÃN, không phải trạng thái** — *"gộp vào bộ trạng thái thì bảng tiến độ đếm sai"* | `05-nghiep-vu.md` §M1 | chưa có |
| **Bản tin sắp đến hạn gộp MỘT tin mỗi người mỗi ngày**, không phải một tin mỗi việc | `05-nghiep-vu.md` §M1 · `07-viec-nen-va-thong-bao.md` | chưa có |
| **Một ngưỡng `warn_before_hours` duy nhất** dùng chung cho bản tin, khung "Sắp đến hạn", số trên chuông, và Zalo — *"hai con số trả lời một câu hỏi thì sẽ lệch nhau đúng hôm lãnh đạo đối chiếu"* | `07-viec-nen-va-thong-bao.md` | ADR 0029 (bảng `sla`) có số giờ cam kết, **chưa có ngưỡng cảnh báo sớm** |
| **Bàn giao ghi TỪ bộ phận nào SANG bộ phận nào, TỪ người nào SANG người nào** — *"nhật ký chỉ ghi 'đã chuyển xử lý' là nhật ký không trả lời được câu hỏi ai đang giữ"* | `05-nghiep-vu.md` §M1 | luật 6 bất biến 5 (before/after) — **cùng tinh thần, chưa có hình dạng cụ thể** |
| **Nhập Excel: kiểm CẢ TỆP trước khi ghi, một dòng sai thì không ghi dòng nào** | `05-nghiep-vu.md` §M2 · `06-giao-dien.md` | chưa có |
| **Cảnh báo trùng đơn** lúc vào sổ, unaccent + pg_trgm, *"báo ngay lúc công dân còn ở quầy"* | `05-nghiep-vu.md` §M2 | chưa có |
| **Giải ngân: tiến độ tính từ NGÀY KHỞI CÔNG tới ngày hoàn thành dự kiến**, không phải tháng 1→12 | `05-nghiep-vu.md` §M3 | `service-finance` — câu hỏi mở #31 đang hỏi đúng vùng này |
| **Mỗi khoản chi chọn RÚT TỪ NGUỒN NÀO** — *"không có cột ấy thì tiến độ theo nguồn phải suy từ dự án, và suy sai ngay khi dự án có hai nguồn"* | `05-nghiep-vu.md` §M3 | `service-finance/migrations/0004` có `chung_tu_giai_ngan.du_an_id` — **cần kiểm có cột nguồn vốn không** |
| **Đơn vị tính đọc từ chính dòng "Đơn vị tính" của tệp; đơn vị lạ thì BỎ HẲN số tuyệt đối, chỉ giữ phần trăm** | `05-nghiep-vu.md` §M3 | câu hỏi mở #32, #33 đang ở đúng vùng này |
| **Zalo Bot làm kênh nhắc việc cán bộ** — bot không nhắn trước được, phải ghép nối bằng mã 8 ký tự sống 10 phút; giờ yên tĩnh 21h–6h; nhịp nhắc lại do xã đặt | `07-` + `08-tich-hop-ngoai.md` | **kho này không có khái niệm Zalo Bot nào** |
| **Cổng TTĐT của xã** — `content_sources`, đồng bộ tin/thông báo/truyền thanh/video về Mini App, ảnh tải về kho tệp của mình | `08-tich-hop-ngoai.md` | `service-comms` — chưa có |

---

## 7. Chỗ lệch về ĐẶT TÊN — nhỏ về kỹ thuật, đắt nếu trộn

Kho này đã trả giá để chốt danh từ tài nguyên URL (ADR 0011 + `ubiquitous-language.md`
§Tên tài nguyên trên URL, mỗi dòng có một ô "vì sao không phải từ dễ đoán"). `vigov-require`
dùng bộ khác.

| Khái niệm | `vigov-require` | `vigov-v2` | Lý do kho này KHÔNG dùng từ bên kia |
|---|---|---|---|
| Phản ánh | `/api/v1/feedbacks` | `citizen-reports` · `my-citizen-reports` | hai lớp tin cậy (luật 4 bất biến 5) **không dùng chung handler**, nên cần hai danh từ |
| Dự án đầu tư | `/api/v1/budget/items` | `investment-projects` | `disbursements/projects` **lồng ngược quan hệ nghiệp vụ** và đã bị bác; `projects` trần thì ngày xã có "dự án dân sinh" là mất tên |
| Cán bộ | `users` | `staff` | `user` trùng với công dân — hai lớp tin cậy khác hẳn nhau |
| Thôn / Tổ dân phố | `hamlets` | `residential-units` | `Hamlet` chỉ đúng nửa (sai với phường) |
| Tài nguyên bản đồ | `assets` | `map-asset-types` (danh mục) | `asset.*` đã là khoá quyền chốt ở đặc tả — giữ từ, thu hẹp danh từ |
| Bộ phận | `/api/v1/org/units` | `org-units` | trùng nghĩa, khác hình dạng đường dẫn |

**Không có cái nào là lỗi bên nào.** Nhưng chúng là lý do **không thể chép một đường API từ
bên kia sang đây** mà không đi qua bảng ánh xạ.

---

## 8. Kết luận — lệch nhiều tới đâu

| Trục | Mức lệch | Nhận định |
|---|---|---|
| **Nghiệp vụ M1–M8** | **THẤP** | Cùng một sản phẩm, cùng một prototype. Bên kia tả **kỹ hơn nhiều** |
| **Phạm vi hợp đồng** | **MỘT CHỖ** | Hồ sơ công dân: bên kia có, kho này đã gỡ theo quyết định của khách |
| **Kiến trúc** | **TOÀN PHẦN** | Go vi dịch vụ ↔ Python monolith. Cả hai đều có ADR. Không hợp nhất được |
| **Cách ly xã** | **KHÁC TẦNG** | RLS trong CSDL ↔ kho có phạm vi ở ứng dụng. **Đây là chỗ đáng bàn nhất** |
| **Danh tính công dân / nhận xã** | **ĐỐI LẬP** | `X-Tenant-Code` + `X-Citizen-Id` là thứ luật 1 và luật 4 của kho này cấm |
| **Đặt tên URL** | **HỆ THỐNG** | Hai bộ danh từ khác nhau, mỗi bộ có lý do viết ra |
| **Bẫy đã trả giá** | **KHO NÀY ĐANG THIẾU** | §5 — sáu mục áp thẳng, chưa mục nào được kiểm |

**Câu trả lời ngắn cho câu hỏi đặt ra:** lệch **rất nhiều về kiến trúc và gần như không lệch
về nghiệp vụ**. Thứ đáng lấy từ `vigov-require` không phải mã, mà là **ba tệp**:
`05-nghiep-vu.md` (luật nghiệp vụ M1–M8), `07-viec-nen-va-thong-bao.md` (việc nền và thông
báo), `09-bay-va-bai-hoc.md` (bẫy đã trả giá).

---

## 9. Việc nên làm tiếp

**Không có danh sách thứ hai ở đây — nó nằm ở [SỔ VIỆC](#sổ-việc--sửa-dần-đừng-xoá-dòng) đầu
tệp.** Hai danh sách việc trong một tệp là hai danh sách sẽ lệch nhau, và cái lệch là cái
người ta đọc.

**Thứ tự đề xuất:** `D1` → `D2` → `K1` → `K2` → `D3` → `X1`. Ba mục nhóm `D` là câu phải hỏi
người khác, nên gửi đi trước rồi làm việc khác trong lúc chờ; `K1` và `K2` rẻ và đo được ngay;
`X1` (đọc mã bên kia) đứng cuối, lý do ghi tại chính dòng của nó.

→ Câu hỏi mở của kho này: `kb/00-foundation/open-questions.json`
→ Câu hỏi mở của kho kia: `../vigov-require/docs/open-questions.md` (22 câu Q-01…Q-22, **bộ
   khác hẳn**, chưa đối chiếu — mục `X2` ở Sổ việc)
→ Tiến độ module kho này: `kb/90-ephemeral/tien-do.md`
