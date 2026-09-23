---
id: doi-chieu-module
tier: T2
source: CURATED
owner: architecture
derived_from_commit: c7268e5
expires: null
owns_facts:
  - "tầng kb/50-doi-chieu/ là gì, giữ cái gì, và đọc theo thứ tự nào"
  - "hình dạng bắt buộc của một ghi chú đối chiếu và của sổ neo"
  - "định nghĩa ĐÃ TIẾP NHẬN một ghi chú đối chiếu, và vì sao nó được SUY RA"
---

# Tầng đối chiếu kho yêu cầu

**Kho `../vigov-require` là nơi BA và PM cập nhật nghiệp vụ và prototype. Kho này viết mã theo
đó.** Tầng này giữ vết: mỗi lần đối chiếu đọc branch nào, từ commit nào tới commit nào, thấy
gì, và phân hệ nào của kho này phải đổi theo.

Quan hệ giữa hai kho — vì sao chúng cùng một sản phẩm mà kiến trúc lệch toàn phần:
`kb/90-ephemeral/doi-chieu-require.md`.

## Vì sao là T2 chứ không T5

Một ghi chú đối chiếu ghi **sự việc đã xảy ra vào một ngày**: branch `main` đi từ `abc1234`
tới `def5678`, và trong khoảng ấy BA đổi ba luật nghiệp vụ. Câu ấy **không bao giờ trở thành
sai**. Nó không hết hạn, nên không phải T5.

Nó cũng không phải T1: T1 là quyết định của **kho này**, kèm phương án đã cân nhắc. Ghi chú
đối chiếu chỉ tường thuật thứ **kho kia** đã làm.

## Thư mục

| Tệp | Nội dung |
|---|---|
| `neo.json` | Mỗi branch đã review tới commit nào. Lần sau đọc tiếp **từ đó** |
| `<ngày>-<branch>.md` | Một ghi chú cho một lần review một branch |

## Sổ neo

```json
{"branches": {"main": {"sha_den": "<sha>", "ngay_review": "2026-09-23"}}}
```

`sha_den` là commit **cuối cùng đã được đọc**. Lần review sau chạy
`git log <sha_den>..origin/<branch>`, nên nó chỉ thấy phần mới.

**Neo theo SHA, không theo ngày.** Ngày trả lời "review lúc nào", không trả lời "đã đọc tới
đâu" — và hai câu ấy lệch nhau ngay lần đầu có người đẩy commit lùi ngày, rebase, hay merge
một nhánh cũ. SHA thì không lệch được.

## Hình dạng một ghi chú

```yaml
---
id: doi-chieu-2026-09-23-main
tier: T2
source: CURATED
owner: architecture
derived_from_commit: <sha của KHO NÀY lúc review>
expires: null
kho_nguon: vigov-require
branch: main
sha_tu: abc1234
sha_den: def5678
ngay_review: 2026-09-23
anh_huong: [service-petitions, web-admin]
owns_facts:
  - "..."
---
```

`hooks/require_sync_guard.py` **chặn** khi thiếu `branch` · `sha_tu` · `sha_den` ·
`ngay_review` · `anh_huong`, và khi hai SHA không đúng dạng hex.

**Bốn khoá đầu là bằng chứng.** Một ghi chú không nói nó đọc từ đâu là lời kể — không ai kiểm
lại được, và lần sau không biết nối tiếp từ chỗ nào.

**`anh_huong` là khoá máy đọc, và là khoá làm cả cơ chế chạy được.** Nó liệt kê module của
**kho này** bị ảnh hưởng. Rào đọc nó để biết chặn ai. Không có nó thì rào phải đoán module từ
văn xuôi, tức đoán sai. Không ảnh hưởng ai thật thì ghi `anh_huong: []`.

## ĐÃ TIẾP NHẬN — suy ra, không phải một cờ

Một ghi chú được coi là **đã tiếp nhận** khi: với **mọi** module trong `anh_huong`, sổ tiến độ
`kb/90-ephemeral/tien-do/<module>.json` có **trích dẫn tên tệp ghi chú**.

Tức là đã có người mở việc thật cho nó. Không phải "đã đọc", không phải "đã code xong".

**Vì sao không nuôi một danh sách `da_tieu_thu` trong `neo.json`.** Một cột trạng thái do
người tự đặt sẽ lệch khỏi sự thật, và bản lệch là bản còn lại. Ai đó đánh dấu "đã tiếp nhận"
cho xong việc thì cờ tắt mà không việc nào được mở — rào vẫn xanh, vẫn chạy, và đã mù. Đây là
lớp lỗi kho này đã ghi nhận năm lần (`kb/90-ephemeral/ban-giao-phien.md` §5).

Hỏi ngược lại vào thứ không giả được — sổ tiến độ có trích không — thì cờ không thể sai. Cùng
hình dạng luật 10 bất biến 3 dùng cho `quá hạn`: **suy ra từ dữ kiện, đừng nuôi một cột song
song.**

## Ba lớp tín hiệu

| Lớp | Khi nào | Sức |
|---|---|---|
| `SendMessage` tới agent đang chạy | ngay sau khi command chạy xong | yếu — agent đã kết thúc thì không nhận được |
| Banner `SessionStart` | mở phiên mới | vừa — có thể bị lướt qua |
| **Chặn ở `PreToolUse`** | đúng lúc sắp ghi vào module bị ảnh hưởng | **mạnh — đây là lớp giữ lời hứa** |

**Phiên sạch thì cả ba lớp im lặng tuyệt đối.** Hook đọc vài tệp nhỏ, thấy không có gì chưa
tiếp nhận, và không in một chữ nào — không nạp ngữ cảnh, không tốn token. Đó là ràng buộc
người dùng đặt ra ngày 23/09/2026, không phải tối ưu thêm.

→ Thủ tục: `/doi-chieu-require` · Kỹ năng: `skills/doi-chieu-require`
→ Agent: `.claude/agents/require-watcher.md`
