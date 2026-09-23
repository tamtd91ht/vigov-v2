---
description: Đối chiếu kho yêu cầu ../vigov-require — BA/PM đã cập nhật gì từ lần review trước, ghi thành ngữ cảnh cho phiên sau
argument-hint: "[branch] — bỏ trống để xem mọi branch rồi chọn"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, ListAgents, SendMessage
---

# /doi-chieu-require

`../vigov-require` là nơi **BA và PM cập nhật nghiệp vụ và prototype**. Kho này viết mã theo
đó. Lệnh này trả lời đúng một câu: **từ lần đối chiếu trước tới giờ, bên kia đổi gì, và kho
này phải làm gì.**

Kết quả là một ghi chú `.md` đặt tên theo ngày, đủ để một phiên khác nạp vào làm ngữ cảnh mà
không phải đọc lại kho bên kia.

**Nền tảng, hình dạng tệp, và định nghĩa "đã tiếp nhận": `kb/50-doi-chieu/README.md`** — một
chủ, không chép sang đây.

---

## HAI PHÉP KIỂM KHÁC HẲN NHAU — đừng gộp

| | Hỏi gì | Giá | Chạy khi nào |
|---|---|---|---|
| **Đắt** | Bên kia có commit mới không | chạm mạng (`fetch`) | **chỉ lệnh này** |
| **Rẻ** | Có ghi chú nào chưa tiếp nhận không | đọc vài tệp nhỏ | mỗi phiên, im khi sạch |

Lệnh này là vế **đắt**. Nó không bao giờ tự chạy. Vế rẻ do
`hooks/require_sync_guard.py` lo.

---

## BƯỚC 0 — kho anh em phải có thật

```sh
ls ../vigov-require/.git
```

Không có → **DỪNG**, báo người dùng. Đừng suy nghiệp vụ bên kia từ tài liệu bên này: kết luận
ấy đọc xuôi tai, và sai. Cùng lý do `hooks/miniapp_sibling_guard.py` tồn tại cho
`vihat-miniapp`.

## BƯỚC 1 — nhìn, chưa động

```sh
git -C ../vigov-require fetch --all --prune
git -C ../vigov-require status --porcelain
git -C ../vigov-require branch --show-current
git -C ../vigov-require for-each-ref --sort=-committerdate \
    --format='%(refname:short) | %(committerdate:short) | %(authorname) | %(subject)' \
    refs/remotes/origin
```

**Cây bên kia KHÔNG sạch → DỪNG và báo.** Đó là cây làm việc của BA/PM; họ có thể đang sửa dở.
`checkout` đè lên thì hoặc hỏng, hoặc kéo thay đổi của họ sang branch khác — và cả hai đều là
thứ họ phát hiện ra muộn. Người dùng bảo cứ pull thì mới pull.

## BƯỚC 2 — hỏi branch, KHÔNG tự chọn

Bày bảng ở bước 1 kèm **neo hiện tại của từng branch** (`kb/50-doi-chieu/neo.json`), rồi hỏi
người dùng chọn.

Bên kia dùng nhiều branch, và branch đang đứng **không phải** branch đáng review. Tự chọn là
tự quyết xem bản yêu cầu nào là bản thật — điều kiện dừng, không phải mặc định.

Người dùng đưa branch sẵn trong `$ARGUMENTS` thì **vẫn xác nhận lại** branch ấy có trên
`origin` không, rồi đi tiếp.

## BƯỚC 3 — pull branch đã chọn

Người dùng đã chốt ngày 23/09/2026: **pull thật**, không chỉ fetch.

```sh
git -C ../vigov-require checkout <branch>
git -C ../vigov-require pull --ff-only
```

`--ff-only`: pull tạo merge commit **trong kho của BA/PM** là viết vào lịch sử của người khác.
Không fast-forward được → dừng, báo, để họ tự xử.

## BƯỚC 4 — đọc đúng phần mới

```sh
git -C ../vigov-require log --oneline <sha_tu>..<branch>
git -C ../vigov-require diff --stat <sha_tu>..<branch>
```

`<sha_tu>` = `neo.json` → `branches.<branch>.sha_den`. **Chưa có neo** (lần đầu) → nói rõ với
người dùng đây là lần đầu, và hỏi mốc bắt đầu; đừng lặng lẽ đọc từ commit gốc.

**Phạm vi đọc** (người dùng chốt 23/09/2026):

| Đọc kỹ | Đọc có chọn lọc |
|---|---|
| `docs/spec/**` · `docs/SRS.md` · `docs/adr/**` · `docs/open-questions.md` | `apps/api/migrations/**` khi lược đồ đổi |
| | `apps/api/app/modules/*/router.py` khi có API mới |
| | `apps/admin/src/app/**` khi có màn hình mới |

Mã chỉ mở **khi một commit chạm nghiệp vụ mà `docs/` chưa theo kịp**. Phần lớn diff bên kia là
Python/FastAPI, không áp được sang Go — đọc hết là tốn ngữ cảnh cho thứ không dùng.

## BƯỚC 5 — dựng ghi chú

Giao `require-watcher` (`.claude/agents/require-watcher.md`). Agent ấy sở hữu
`kb/50-doi-chieu/**` và không ai khác ghi vào đó.

Tên tệp: `kb/50-doi-chieu/<YYYY-MM-DD>-<branch>.md`.

**Cùng ngày, cùng branch, chạy lại** → **ghi đè** tệp ấy và nới `sha_tu` về mốc cũ nhất. Hai
tệp cùng ngày cùng branch là hai tệp mâu thuẫn, và người đọc không phân biệt được cái nào còn
đúng — cùng lý do `ban-giao-phien.md` là MỘT tệp.

## BƯỚC 6 — dời neo

Ghi `sha_den` mới + `ngay_review` vào `neo.json`. **Chỉ sau khi ghi chú đã nằm trên đĩa.** Dời
neo trước mà ghi chú hỏng giữa chừng thì lần sau bắt đầu từ sau khoảng vừa mất — và không ai
biết là đã mất.

## BƯỚC 7 — báo cho agent đang chạy

```
ListAgents  →  agent nào đang sống?
```

Có → `SendMessage` tới từng agent, ngắn gọn: tệp ghi chú nào, `anh_huong` gồm module nào.

**Lớp này yếu, phải nói thẳng khi báo cáo:** agent đã kết thúc thì không nhận được gì, agent
đang chạy giữa chừng thì nhận được nhưng không chắc đổi hướng. Lớp giữ lời hứa là **rào chặn ở
`PreToolUse`**, không phải tin nhắn này.

Không có agent nào đang chạy → **không làm gì.** Banner `SessionStart` và rào chặn đã lo phần
còn lại.

## BƯỚC 8 — báo cáo, rồi DỪNG

Báo: branch đã review · khoảng commit · số commit · module bị ảnh hưởng · đường dẫn ghi chú.

**Lệnh này KHÔNG tự mở việc vào `tien-do`.** Nó tường thuật; quyết định phạm vi là của người
dùng hoặc của phiên sau. Tự mở việc là tự quyết hộ kho này phải làm gì với yêu cầu của khách.

---

## ĐIỀU KIỆN DỪNG — hỏi người dùng, đừng tự quyết

| # | Tình huống |
|---|---|
| 1 | `../vigov-require` không có, hoặc không nằm cạnh kho này |
| 2 | Cây làm việc bên kia **không sạch** |
| 3 | Lần đầu review một branch — **chưa có neo**, chưa biết bắt đầu từ đâu |
| 4 | `pull --ff-only` không đi được |
| 5 | Một commit bên kia **đổi thứ khách đã chốt ở kho này** (ADR, hay một câu `DECIDED` trong `kb/00-foundation/open-questions.json`) — đây là **mâu thuẫn giữa hai nguồn**, không phải một mục để ghi chú |

Điều kiện 5 là cái đắt nhất. Ghi chú nó như một mục thường thì kho này lặng lẽ đi theo bản mới
và **bỏ quyết định khách đã ký** — luật 7 gọi đó là sửa hồ sơ lưu trữ khi nó đã chạm dữ liệu.
