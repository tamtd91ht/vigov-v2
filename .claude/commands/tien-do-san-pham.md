---
description: In bảng tiến độ theo PHÂN HỆ SẢN PHẨM — lọc được theo bề mặt và phân hệ, bỏ trống là toàn bộ
group: Tiến độ & bàn giao
argument-hint: "[API|WebAdmin|Miniapp][-<phân hệ>] | --excel [đường-dẫn.xlsx] — ví dụ API-Nhiệm vụ, Miniapp, 02, --excel. Bỏ trống = toàn bộ"
allowed-tools: Read, Bash
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

## `--excel` — TỆP CHO GOOGLE SHEET CỦA TEAM

```sh
python tools/tien_do_san_pham.py --excel                 # → tmp/tien-do/vigov-tien-do-<ngày>.xlsx
python tools/tien_do_san_pham.py --excel D:/share/td.xlsx
```

**Luôn là TOÀN BỘ dự án** — bộ lọc không áp vào bản Excel. Sheet chung của cả team phải đủ;
ai cần một phần thì dùng bộ lọc của chính Google Sheet (dòng tiêu đề đã bật sẵn).

Năm sheet, **tên sheet và tên cột cố định** — định nghĩa từng cột in trong sheet `Thong_tin`
và nằm ở một chỗ duy nhất, `SHEETS` trong `tools/xuat_tien_do.py`:

| Sheet | Mỗi dòng |
|---|---|
| `Tong_quan` | một chương đặc tả — tuyến, đã gọi, màn web, số hạng mục theo trạng thái |
| `Menu_web` | một mục menu web-admin — có màn chưa, số phần chưa dựng |
| `Hang_muc` | một mục sổ tiến độ — cột `Ma` (`module/id`) là **khoá ổn định** |
| `Cho_khach` | một câu hỏi còn chờ khách chốt, và hạng mục nào đang chờ nó |
| `Thong_tin` | ngày xuất · commit · **phiên bản format** · ý nghĩa từng cột |

**Đẩy lên Google Sheet — thủ công, có chủ ý:** File → Import → Tải lên → **Replace
spreadsheet**. Đẩy tự động qua Google Sheets API cần một khoá service account — một bí mật bên
thứ ba MỚI và lần đầu gửi dữ liệu dự án ra dịch vụ ngoài (STOP CONDITION luật 8 và luật 3);
chưa ai quyết ai giữ khoá ấy.

**Ghi chú của team để ở sheet RIÊNG** (vd `Ghi_chu`), tra sang bằng
`VLOOKUP(Ma; Hang_muc!A:H; …)`. *Replace* xoá mọi cột team tự thêm vào sheet nhập; sheet riêng
thì còn nguyên qua mọi lần cập nhật.

| Mã thoát | Nghĩa |
|---|---|
| 0 | Ghi xong — in đường dẫn và số dòng từng sheet |
| 1 | Thiếu nguồn — chạy `make kb` trước |
| 3 | Có ô trông như **số điện thoại / CCCD thật** — KHÔNG ghi tệp, chỉ in toạ độ ô (luật 3). Sửa nguồn |
| 4 | Đường dẫn nằm trong kho mà git không bỏ qua — tệp nhị phân không vào git |

Đổi format (thêm cột): **chỉ thêm vào cuối** `SHEETS`, tăng `PHIEN_BAN_FORMAT`, và cập nhật ca
khoá format trong `tools/test_hooks.py` — ca ấy đỏ đúng để không ai đổi format mà quên báo team.

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

## VÌ SAO KHÔNG CÓ CỘT `% HOÀN THÀNH`

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
