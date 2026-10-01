---
description: Liệt kê bảng dữ liệu theo service — chức năng, cột, ràng buộc, chỉ mục, trigger — đọc từ migration, lưu local để lần sau không quét lại; xuất được ra Excel
group: Tra cứu
argument-hint: "[service] [--reload] [--brief] [--excel [đường-dẫn.xlsx]] — ví dụ finance, service-identity --reload, --excel. Bỏ trống = toàn bộ service"
allowed-tools: Bash, Read
---

# /db-tables `[service] [--reload]`

Đối số: **$ARGUMENTS**

Trả lời *"service này có những bảng nào, mỗi bảng để làm gì, cấu trúc ra sao"* mà không phải mở
từng tệp migration.

## 1. Chạy

```sh
python tools/db_tables.py                      # toàn bộ service — đọc bản lưu nếu còn khớp
python tools/db_tables.py finance              # một service (`service-finance` cũng được)
python tools/db_tables.py finance --reload     # bỏ qua bản lưu, quét lại, ghi đè bản lưu
python tools/db_tables.py --brief              # chỉ bảng tóm tắt, không in cột
python tools/db_tables.py --excel              # xuất Excel toàn bộ → tmp/db-tables/db-tables-<ngày>.xlsx
python tools/db_tables.py finance --excel D:/x.xlsx   # một service, ra đường dẫn chỉ định
```

Trên Windows khi chuyển hướng đầu ra, đặt `PYTHONIOENCODING=utf-8` trước lệnh.

| Đối số | Cách gọi tool |
|---|---|
| Bỏ trống | Có `--brief`, in nguyên văn. Bản đầy đủ (~3.000 dòng) ghi vào `tmp/db-tables/all.txt` rồi báo đường dẫn. In cả 91 bảng vào chat là chôn câu trả lời |
| Có tên service | Không `--brief`, **in nguyên văn** — không tóm tắt, không bớt cột |
| Có `--reload` | Chuyển nguyên cờ cho tool |
| Có `--excel` | **Không in bảng vào chat.** Chạy tool, báo người dùng đường dẫn tệp và hai con số tool in ra (số bảng, số cột) |

### `--excel` — ba sheet, mỗi sheet một dòng mỗi thứ, đều có bộ lọc

| Sheet | Một dòng là |
|---|---|
| `1. Bảng` | một bảng: service, entity, phạm vi, số cột, chức năng, tạo ở / sửa ở, phân vùng. Cảnh báo bộ phân tích (nếu có) nằm cuối sheet này |
| `2. Cột` | một cột: kiểu, NULL, mặc định, PK, UNIQUE, tham chiếu, ghi chú |
| `3. Ràng buộc & chỉ mục` | một ràng buộc, chỉ mục hoặc trigger, kèm định nghĩa |

- Ghi bằng bộ ghi xlsx thư viện chuẩn của `tools/xuat_tien_do.py`, không cần cài thêm gói nào.
- Bộ lọc service áp vào Excel; khi đó tên tệp mặc định có tên service.
- Bản lưu cũ thì **vẫn xuất**, dòng phụ đề ghi `⚠ BẢN LƯU CŨ`, và tool trả exit 3 như khi in.

## 2. Nguồn, bản lưu, và khi nào bản lưu cũ

- **Nguồn:** `service-*/migrations/*.sql`. Tool áp lần lượt `CREATE TABLE` / `ALTER TABLE` /
  `CREATE INDEX` / `CREATE TRIGGER` / `COMMENT ON` để ra cấu trúc **cuối cùng**. Nó không kết nối
  DB, nên không cần DSN hay mật khẩu (luật 8).
- **Chức năng của bảng** lấy từ khối chú thích ngay trên `CREATE TABLE`, các dấu `@entity` /
  `@scope` và `COMMENT ON`. Bảng không có những thứ đó in ra `chưa có mô tả trong migration`. **Đừng
  tự viết mô tả thay** — muốn có mô tả thì viết nó vào chính migration kế tiếp.
- **Bản lưu:** `tmp/db-tables/<service>.json`, đã được git bỏ qua. Đây là bản đệm, **không phải tài
  liệu**: không chép nội dung của nó vào `kb/` (luật 9 #2). Ai sở hữu entity nào thì đã có
  `kb/30-indexes/data-ownership.json`.
- **Bản lưu cũ:** mỗi bản lưu có dấu vân tay của các tệp migration và của phiên bản parser. Khi
  dấu ấy lệch, tool **vẫn in bản lưu** nhưng gắn nhãn `⚠ BẢN LƯU CŨ` và trả exit 3. Gặp exit 3 thì
  **nói cho người dùng biết**, và đề nghị chạy `--reload`. Đừng tự chạy `--reload` thay họ.

| Mã thoát | Nghĩa |
|---|---|
| 0 | In xong (vừa quét lại, hoặc đọc bản lưu còn khớp) |
| 2 | Không có service tên ấy — in danh sách tên đúng, hỏi lại người dùng |
| 3 | In / xuất từ bản lưu **cũ hơn migration** — báo người dùng, đề nghị `--reload` |
| 4 | `--excel`: đường dẫn nằm trong kho mà git không bỏ qua — tệp sinh ra không vào git. Để mặc định hoặc chọn đường ngoài kho |
| 5 | `--excel`: có ô trông như số điện thoại / CCCD thật — **không ghi tệp**, chỉ in toạ độ ô (luật 3). Sửa chú thích trong migration |
| 6 | `--excel`: tệp đích đang mở trong Excel — đóng rồi chạy lại |

## 3. Giới hạn — nói ra, đừng che

- Phần **`Cảnh báo bộ phân tích`** ở cuối mỗi service là những câu lệnh tool **chưa hiểu**. Có dòng
  nào thì đọc lại đúng `file:line` ấy trước khi khẳng định gì về bảng liên quan.
- DDL nằm **trong chuỗi** (`EXECUTE format('…')`) không được phân tích. Ngoại lệ duy nhất là vòng
  tạo phân vùng hash: nó được nhận ra và in thành `32 phân vùng hash (tạo động)`.
- Một thay đổi người ta làm tay trên DB thật, ngoài migration, thì tool **không thấy**.
- Sửa parser thì tăng `PARSER_VERSION` trong `tools/db_tables.py`. Nhờ vậy mọi bản lưu dựng bằng
  parser cũ sẽ hiện thành **cũ**, thay vì tiếp tục được tin.
