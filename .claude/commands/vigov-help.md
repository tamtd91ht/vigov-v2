---
description: Hiển thị danh mục lệnh / của dự án ViGov — cú pháp và chức năng, gom theo nhóm; truyền tên lệnh để xem chi tiết
group: Bộ não
argument-hint: "[lệnh] — bỏ trống = toàn bộ danh mục, ví dụ: develop-feature"
allowed-tools: Bash, Read
---

# /vigov-help `[lệnh]`

Đối số: **$ARGUMENTS**

## 1. Chạy đúng một lệnh, in nguyên văn kết quả

- Bỏ trống → `python tools/vigov_help.py`
- Có tên lệnh → `python tools/vigov_help.py "<lệnh>"` (có hay không có `/` đều được)

Trên Windows khi chuyển hướng đầu ra, đặt `PYTHONIOENCODING=utf-8` trước lệnh.

**In lại đúng bảng công cụ trả về — không tóm tắt, không thêm, không bớt lệnh nào.** Danh mục
được SINH từ frontmatter của `.claude/commands/*.md` (`description`, `argument-hint`, `group`),
nên nó đúng tới lệnh vừa thêm hôm nay. Một bảng gõ lại từ trí nhớ là bản sao sẽ lệch (luật 9).

## 2. Khi công cụ báo lỗi

| Mã thoát | Nghĩa | Làm gì |
|---|---|---|
| 1 | Có lệnh thiếu `description` | Vẫn in bảng, rồi nói tên lệnh thiếu — đừng tự viết mô tả thay |
| 2 | Tên lệnh không khớp đúng một lệnh | In danh sách công cụ đưa ra, hỏi người dùng muốn lệnh nào |

Một lệnh nằm trong nhóm **"Chưa xếp nhóm"** nghĩa là tệp của nó thiếu khoá `group:` — nói ra để
có người bổ sung vào chính tệp ấy.

## 3. Sau bảng, một dòng gợi ý

Nếu người dùng chưa biết bắt đầu từ đâu: phát triển tiếp một menu dùng nhóm **Phát triển**
(`/develop-feature` khi chức năng chạm nhiều nền tảng); xem còn nợ gì dùng `/progress` hoặc
`python tools/tien_do.py --menu "<menu>"`.
