---
description: In bảng tiến độ theo PHÂN HỆ SẢN PHẨM — chương đặc tả và từng mục menu web-admin
argument-hint: "[--moi] — thêm --moi để sinh lại hợp đồng trước khi đo"
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
python tools/tien_do_san_pham.py     # đọc hợp đồng đang có trên đĩa
```

Kết quả ghi vào `kb/90-ephemeral/tien-do-san-pham.md`. Mở tệp ấy và **in nguyên văn hai bảng
đầu ra cho người dùng** — đừng tóm tắt lại bằng lời, vì tóm tắt là chỗ một con số đo được biến
thành một con số ước lượng.

**Với `--moi`**, sinh lại hợp đồng trước rồi mới đo. Chỉ cần khi mã Go vừa đổi và `make kb` chưa
chạy:

```sh
go run ./tools/apidoc && python tools/tien_do_san_pham.py
```

⚠ **KHÔNG chạy `--moi` khi còn agent đang ghi mã.** `apidoc` đọc cả `web-admin/src/lib/api/**`
để biết tuyến nào đã có màn gọi; đọc giữa lúc một agent viết dở là đo một cây đang động, và nó
sẽ chuyển nhầm việc sang `done/`.

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
