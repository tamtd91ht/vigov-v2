---
description: In bảng tiến độ theo PHÂN HỆ SẢN PHẨM — lọc được theo bề mặt và phân hệ, bỏ trống là toàn bộ
group: Tiến độ & bàn giao
argument-hint: "[API|WebAdmin|Miniapp][-<phân hệ>] | --excel [đường-dẫn.xlsx] — ví dụ API-Nhiệm vụ, Miniapp, 02, --excel (báo cáo kỹ thuật 7 sheet cho BA/PM: tổng quan · chức năng theo menu · API · danh mục API · Web Admin · Mini App · deployment). Bỏ trống = toàn bộ"
allowed-tools: Read, Bash, Edit, Write
---

# /tien-do-san-pham

Trả lời câu **"phân hệ nào dùng được rồi"** — câu của chủ dự án, không phải câu của phiên sau.

`/progress` và `kb/90-ephemeral/tien-do.md` xếp theo **module kho mã**. Một người hỏi *"màn
Nhiệm vụ xong chưa"* không đọc được câu trả lời từ một bảng xếp theo `service-petitions`: phân
hệ Nhiệm vụ nằm rải ở `service-petitions`, `web-admin` và một phần `service-identity`. Hai tệp,
hai trục, hai người đọc — và chúng **liên kết** với nhau chứ không chép của nhau (luật 9 #2).

---

## CHẠY

```sh
python tools/tien_do_san_pham.py                    # TOÀN BỘ — ghi tệp, bốn phần
python tools/tien_do_san_pham.py API-Nhiệm vụ       # một phân hệ, một bề mặt
python tools/tien_do_san_pham.py WebAdmin-Giải ngân
python tools/tien_do_san_pham.py Miniapp            # một bề mặt, mọi phân hệ
python tools/tien_do_san_pham.py 02                 # một phân hệ, cả ba bề mặt
```

**Bộ lọc là `[BỀ MẶT][-PHÂN HỆ]`, mỗi vế bỏ được.**

| Vế | Nhận |
|---|---|
| Bề mặt | `API` · `WebAdmin` · `Miniapp` — và các cách gọi khác: `backend`, `web`, `admin`, `zalo`, `công dân`… |
| Phân hệ | số chương (`02`) hoặc tên chương (`Nhiệm vụ`, `nhiem-vu`, `Quản lý nhiệm vụ`) |

Có dấu hay không, hoa hay thường, gạch nối hay khoảng trắng — đều như nhau: bộ lọc bỏ dấu, bỏ
hoa thường, bỏ mọi ký tự không phải chữ số trước khi so. Tên phân hệ là tiếng Việt có dấu, và
bắt người gõ đúng từng dấu trên dòng lệnh là bắt họ đi đọc mã.

**Gõ nhầm thì ĐỎ, thoát 1, kèm danh sách giá trị đúng** — không âm thầm in toàn bộ. Một người gõ
`WebAdim-Nhiem vu` mà nhận được cả bảng sẽ đọc nó như bảng của riêng phân hệ mình hỏi.

---

## KHÔNG LỌC GHI TỆP · CÓ LỌC CHỈ IN RA

| | |
|---|---|
| **Không tham số** | ghi `kb/90-ephemeral/tien-do-san-pham.md`, đủ bốn phần. Đây là thứ `make kb` chạy |
| **Có tham số** | **chỉ in ra màn**, không đụng tệp sinh |

Một bản đã lọc nằm ở đường dẫn của bản toàn cảnh là một tệp **trông như toàn cảnh mà chỉ chứa
một phân hệ** — loại tài liệu nói dối mà không ai phát hiện được, vì nó không sai ở bất kỳ dòng
nào, nó chỉ thiếu.

Khi in cho người dùng, **in nguyên văn bảng** — đừng tóm tắt lại bằng lời. Tóm tắt là chỗ một
con số đo được biến thành một con số ước lượng.

---

## `--excel` — BÁO CÁO TIẾN ĐỘ KỸ THUẬT CHO BA / PM

Trả lời một câu: *"từng menu đã làm gì, còn thiếu gì, API / Web Admin / Mini App / deploy đang ở
đâu"* — đủ để BA, PM đọc xong **không phải hỏi thêm**. **Luôn là TOÀN BỘ dự án**; bộ lọc không áp
vào bản Excel. Format v3 (25/09/2026) thay hẳn bản v2 (ba sheet có % và mốc dự kiến).

### Bảy sheet

Tên sheet và cột bảng chính nằm ở một chỗ duy nhất, `SHEETS` trong `tools/xuat_tien_do.py`.

| Sheet | Nội dung | Ô sinh từ kho | Ô agent viết |
|---|---|---|---|
| `1. Tổng quan` | Tóm tắt · chỉ số chính (công thức, có liên kết sang sheet chi tiết) · điểm cần lưu ý · sổ theo module · cách đọc | chỉ số, sổ module | tóm tắt, điểm lưu ý |
| `2. Chức năng theo menu` | Một dòng mỗi chương đặc tả = một menu | #, tên, service, API gọi/tổng, Web Admin | đã làm · còn thiếu · đang xử lý · mức độ |
| `3. API Backend` | Một dòng mỗi `service-*` + khối nền tảng `core/` | số tuyến REST (công thức) | phạm vi · đã làm · còn thiếu · trạng thái |
| `4. Danh mục API` | Mọi tuyến trong `kb/20-contracts/openapi.json`, lọc được | **toàn bộ** | — |
| `5. Web Admin` | A: từng mục menu · B: từng phần chưa dựng · C: ghi chú kỹ thuật | menu, đường dẫn, khoá quyền, có màn, số đã dựng / chưa dựng, danh sách B | chức năng đã dựng · ghi chú |
| `6. Mini App` | Nhóm theo giai đoạn + phát hành Zalo | — | toàn bộ |
| `7. Deployment` | A: đơn vị triển khai · B: checklist đưa lên cụm · C: rủi ro | loại/cổng, Dockerfile, Jenkinsfile, manifest | ghi chú đơn vị, checklist, rủi ro |

### Hai bước, và vì sao không phải một

Số liệu đếm được thì sinh. Phần chữ là **tóm tắt** — sinh máy móc từ sổ thì thành bản chép dài và
khó đọc, còn chép cố định vào tool thì đứng yên trong khi mã chạy (luật 9 #1, #2). Nên agent chạy
lệnh này **viết lại phần chữ mỗi lần xuất**, từ bằng chứng tool đưa ra; phần chữ chỉ sống trong
`tmp/`, không vào git.

**Thủ tục cho agent khi người dùng gõ `/tien-do-san-pham --excel [đường-dẫn.xlsx]`:**

```sh
python tools/tien_do_san_pham.py --excel --khung            # 1. → tmp/tien-do/noi-dung-<ngày>.json
#                                                             2. agent viết khoá `viet` của tệp ấy
python tools/tien_do_san_pham.py --excel --tu tmp/tien-do/noi-dung-<ngày>.json [đường-dẫn.xlsx]   # 3.
```

1. **Khung.** Tệp JSON có ba khoá: `_huong_dan` (quy tắc), `bang_chung` (chỉ để đọc — chương, menu,
   tuyến, phần chưa dựng, mục sổ theo menu và theo module, đơn vị triển khai) và `viet` (ô trống
   cần điền). Chạy lại `--khung` trên tệp cũ **giữ nguyên phần chữ đã viết**, chỉ thêm khoá mới.
2. **Viết.** Đọc `bang_chung`, mở tệp sổ `kb/90-ephemeral/tien-do/<module>.json` và
   `deploy/README.md` mục 10 để kiểm từng khẳng định trước khi viết. Điền mọi ô trong `viet`.
   **Không sửa `bang_chung`** — bước 3 đọc lại số liệu từ kho, sửa ở đó vô tác dụng.
3. **Xuất.** Tool tự tính lại số liệu, kiểm phần chữ, rồi mới ghi. Đỏ thì sửa đúng ô nó chỉ ra,
   chạy lại.

Xong: báo người dùng đường dẫn tệp và 3–5 con số chính đọc từ sheet Tổng quan.

### Văn phong — có rào chặn

| Quy tắc | Vì sao |
|---|---|
| Kỹ thuật, ngắn, một ý một câu, tiếng Việt có dấu | BA/PM đọc để nắm hiện trạng, không đọc nhật ký phiên |
| **Không đặt câu hỏi cho người đọc.** Việc còn chờ xác nhận / chờ bên khác viết thành việc **đang xử lý** | Người dùng quyết 25/09/2026: câu hỏi xác minh bổ sung sau khi deploy v1, không nằm trong báo cáo |
| Không "chờ khách", "cần BA/PM quyết", "bị chặn", "treo", "tạm dừng", "câu #n", dấu `?` | `loi_van_phong` từ chối — ca kiểm `VAN_PHONG_XLSX_CASES` |
| Không viết con số tool đã tự đếm (số API, số phần chưa dựng) | Hai nguồn cho một con số là hai con số sẽ lệch |
| `muc_do` ∈ Cơ bản xong · Đang hoàn thiện · Đang xử lý · Chưa khởi công | Tool kiểm tập giá trị; màu ô theo trạng thái |
| `web.<menu>.da_dung` — mỗi phần tử **một** chức năng đã dựng, kèm § đặc tả nếu có | Cột "Phần đã dựng" là công thức đếm dòng của danh sách này |
| Không dữ liệu cá nhân (SĐT, CCCD, họ tên công dân) | Luật 3 — tool soi mọi ô trước khi ghi |

⚠ **Hai cột đã dựng / chưa dựng không cộng được thành %.** "Chưa dựng" là danh sách màn tự khai
(`PHAN_CHUA_DUNG`); "đã dựng" là tóm tắt của agent. Một chức năng đã dựng có thể bằng một hay
nhiều phần chưa dựng. Màn **không khai** `PHAN_CHUA_DUNG` in `0` kèm dòng "chưa khai" ở mục B —
đọc là *chưa đếm được*, không phải *không còn gì*.

### Đẩy lên Google Sheet — thủ công, có chủ ý

File → Import → Tải lên → **Replace spreadsheet**. Đẩy tự động qua Google Sheets API cần một khoá
service account — một bí mật bên thứ ba MỚI và lần đầu gửi dữ liệu dự án ra dịch vụ ngoài (STOP
CONDITION luật 8 và luật 3); chưa ai quyết ai giữ khoá ấy. Công thức ghi không kèm giá trị đệm;
Excel và Google Sheet tự tính lúc mở (`fullCalcOnLoad`).

⚠ **v2 → v3 đổi toàn bộ tên sheet và cột**, bỏ % và mốc dự kiến. Công thức team dựng trên bản v2
phải dựng lại.

| Mã thoát | Nghĩa |
|---|---|
| 0 | Ghi xong — in đường dẫn và số chương / tuyến / menu |
| 1 | Thiếu nguồn — chạy `make kb` trước. Hoặc hai phép đếm phần chưa dựng lệch nhau (`.md` vs Excel) — sửa bộ đếm, đừng chọn một bên. Hoặc tệp đích đang mở trong Excel |
| 3 | Có ô trông như **số điện thoại / CCCD thật** — KHÔNG ghi tệp, chỉ in toạ độ ô (luật 3). Sửa nguồn |
| 4 | Đường dẫn nằm trong kho mà git không bỏ qua — tệp sinh ra không vào git |
| 5 | Thiếu `--tu`, hoặc phần chữ thiếu ô / sai tập trạng thái / sai văn phong — in từng đường dẫn khoá |

Đổi format: tăng `PHIEN_BAN_FORMAT` và cập nhật ca khoá format `FORMAT_XLSX` trong
`tools/test_hooks.py` — ca ấy đỏ đúng để không ai đổi format mà quên báo team.

---

## KHI SỐ CÓ THỂ ĐÃ CŨ

Bộ lọc đọc `kb/20-contracts/openapi.json` **đang có trên đĩa**. Mã Go vừa đổi mà `make kb` chưa
chạy thì sinh lại hợp đồng trước:

```sh
go run ./tools/apidoc && python tools/tien_do_san_pham.py
```

⚠ **KHÔNG làm thế khi còn agent đang ghi mã.** `apidoc` đọc cả `web-admin/src/lib/api/**` để
biết tuyến nào đã có màn gọi; đọc giữa lúc một agent viết dở là đo một cây đang động, và nó sẽ
chuyển nhầm việc sang `done/`.

---

## ĐỌC BẢNG CHO ĐÚNG — ba chỗ dễ đọc sai

**`ĐÃ GỌI` không phải phần trăm hoàn thành.** Nó là một phép chia giữa hai con số đếm được: bao
nhiêu tuyến của chương ấy đã có mã client gọi tới, trên tổng số tuyến thuộc **bề mặt web**. Một
chương `8/8` vẫn có thể còn nửa đặc tả chưa ai chạm — vì phần chưa chạm ấy chưa có tuyến nào,
nên nó không nằm trong mẫu số.

**`+n ngoài web` là tuyến của kênh khác**, không phải thiếu sót. Chương 09 có hai tuyến
`my-citizen-reports` thuộc Mini App công dân; web quản trị không bao giờ gọi chúng. Đếm chúng
vào mẫu số cho ra `6/8` và đọc thành "còn thiếu hai màn" — sai.

**`không khai` ≠ `0`.** `0` nghĩa là màn có khai một danh sách phần-chưa-dựng và danh sách ấy
rỗng. `không khai` nghĩa là **chưa ai nói màn ấy còn thiếu gì**. Ba màn đang ở trạng thái sau,
và in `0` cho chúng là một lời trấn an không có gì đứng sau.

**Phần Mini App đo hai thứ rời nhau, và khoảng cách giữa chúng mới là tin.** Một bên là tuyến
ViGov khai `citizen-only` trong hợp đồng; một bên là tuyến `citizen-app` thật sự gọi. Hôm nay là
`2` và `0` — không phải vì app làm thiếu, mà vì nó đang phục vụ bề mặt của **kho anh em**
`vihat-miniapp` (CLAUDE.md, mục hai kho). Gộp hai con số ấy làm một là mất đúng thông tin đáng
giữ.

---

## VÌ SAO BẢN `.md` KHÔNG CÓ CỘT `% HOÀN THÀNH`

*(Bản Excel v3 cũng không có — xem mục `--excel`.)*

Ngày 23/09/2026 chủ dự án đọc một con số tiến độ rồi nói *"vậy mà tôi tưởng làm xong hết rồi"*.
Con số ấy đếm **mục việc trong sổ**, không đếm sản phẩm. Một tỷ lệ tự nghĩ ra trông chính xác
hơn hẳn thứ nó biết, và người đọc không có cách nào thấy nó được nghĩ ra.

Nên mọi con số trong tệp sinh ra đều **truy ngược được về một tệp trong kho** — biểu đọc từ đâu
ghi ngay trong `tools/tien_do_san_pham.py`. Muốn thêm một cột thì thêm một phép đo, đừng thêm
một ước lượng.

---

## KHI MỘT CON SỐ TRÔNG SAI

Đừng sửa `kb/90-ephemeral/tien-do-san-pham.md` — nó sinh ra, lần `make kb` sau xoá sạch (luật 9
bất biến 8). Sửa **nguồn**:

| Số sai | Nguồn |
|---|---|
| tuyến của một chương | `@screen` trên câu lệnh route trong `*/internal/http/routes.go` |
| một tuyến "chưa gọi" mà đã có màn | `tools/apidoc/manhinh.go` — nó suy từ `web-admin/src/lib/api/**` |
| mục menu, đường dẫn, khoá quyền | `web-admin/src/components/muc-menu.ts` |
| số phần chưa dựng | mảng `PHAN_CHUA_DUNG` của chính màn ấy |
| trạng thái module | `kb/90-ephemeral/tien-do/<module>.json`, rồi `/progress` |

→ Bảng theo module: `/progress` · Sức khoẻ tầng tri thức: `/knowledge-health`
