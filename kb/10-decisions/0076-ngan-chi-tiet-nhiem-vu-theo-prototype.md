---
id: 0076-ngan-chi-tiet-nhiem-vu-theo-prototype
tier: T1
source: CURATED
owner: domain
derived_from_commit: 49056d72
expires: null
owns_facts:
  - "ngăn chi tiết nhiệm vụ KHÔNG còn 3 tab thao tác Xem chi tiết · Chỉnh sửa · Xoá: sửa tại chỗ bằng '✎ Sửa' trong khối thông tin, Xoá (xoá mềm, có lý do) ở cột thao tác; ngăn phải + tab bản ghi giữ (chốt 07/10/2026)"
  - "ô tick + trọng số việc con vẽ như prototype nhưng là control vô hiệu dấu '?' tới khi máy chủ có trọng số (chốt 07/10/2026)"
  - "hai ô phê duyệt nhiệm vụ (lãnh đạo xã phê duyệt hoàn thành · cấp trên công nhận) khoá bằng task.update ở cả máy chủ và giao diện, tick thẳng ở chế độ xem, mỗi lần đổi có vết trước/sau + một dòng nhật ký (chốt 07/10/2026)"
  - "gỡ tệp đính kèm nhiệm vụ: người tải lên hoặc task.update; gỡ được cả tệp đã gắn nhật ký; lý do bắt buộc; xoá mềm; tệp giữ pháp lý → 409; dòng nhật ký 'đã gỡ' KHÔNG chép tên tệp (chốt 07/10/2026)"
  - "'Cập nhật và giao việc' trong một lần chuyển trạng thái: nhiệm vụ về ĐÚNG trạng thái đã chọn, không về moi-giao; người có task.assign HOẶC người đang thực hiện được giao — đảo quyết định 28/09/2026 CHỈ cho đường gộp; POST /tasks/{ma}/assignment giữ luật cũ (chốt 07/10/2026)"
  - "luật prototype 'phải có ≥1 tệp trước khi sang Chờ duyệt' KHÔNG áp dụng — chưa ai chốt (07/10/2026)"
---

# 0076. Ngăn chi tiết nhiệm vụ theo prototype và bốn thao tác máy chủ

**Trạng thái:** đã chốt · **Ngày:** 2026-10-07 · **Người quyết:** chủ dự án, 07/10/2026, trong lượt
`/fix-web-admin --menu=nhiem-vu` · **Bổ sung** ADR 0068 *Sửa đổi 06/10/2026 (lần 5)* cho riêng menu
Nhiệm vụ, **thay một phần** 0068 *Sửa đổi 05/10/2026* #3 — xem §*Quan hệ với ADR khác*.

## Bối cảnh

ADR 0068 lần 5 #3 đặt luật: điểm **sở thích trình bày** ghi trước đó mà prototype
(`../vigov-require/apps/admin`) nói khác thì bị prototype **thay**. Lần 5 #4 lại liệt kê ngoại lệ,
trong đó có *"hộp chi tiết nhiệm vụ dạng ngăn phải + tab bản ghi (05/10 #3/#6)"*. Hai câu này va nhau
ở ngăn chi tiết nhiệm vụ: 05/10 #3 đặt thanh **tab thao tác** Xem chi tiết · Chỉnh sửa · Xoá, còn
prototype sửa ngay trong khối thông tin và đặt Xoá ở cột thao tác.

Cùng lúc, prototype có bốn thứ cần **tuyến hoặc cột máy chủ** chưa tồn tại (lịch sử gia hạn, gỡ tệp,
hai ô phê duyệt có vết, giao việc gộp trong lần chuyển trạng thái), và một luật nghiệp vụ (≥1 tệp
trước khi Chờ duyệt) mà đặc tả không nói. Lần 5 #5 giới hạn ở front-end, nên mỗi thứ này cần chủ dự án
quyết. Chủ dự án quyết từng điểm ngày 07/10/2026.

## Quyết định

| # | Điểm | Chốt | Phương án bị bác |
|---|---|---|---|
| 1 | Tab thao tác của ngăn chi tiết | **Bỏ** 3 tab Xem chi tiết · Chỉnh sửa · Xoá. Sửa **tại chỗ** trong khối thông tin bằng "✎ Sửa"; Xoá (xoá mềm, có lý do — luật 7) ở **cột thao tác**. **Giữ** ngăn phải và tab bản ghi (05/10 #6). Mã nhiệm vụ **vẫn không sửa được** dù prototype cho sửa | Giữ thanh tab của 05/10 #3 vì lần 5 #4 nêu tên nó |
| 2 | Việc con | Vẽ ô tick + trọng số **đúng vị trí prototype**, nhưng là control **vô hiệu dấu "?"** (0068 lần 5 #5, §14) tới khi máy chủ có trọng số. Danh sách việc con + "Thêm việc con" giữ nguyên | Bỏ hẳn hai ô · làm giả trọng số ở web |
| 3 | Hai ô phê duyệt (*lãnh đạo xã phê duyệt hoàn thành* · *cấp trên công nhận*) | Khoá **`task.update`** ở **cả máy chủ và giao diện**; tick thẳng ở chế độ xem; mỗi lần đổi ghi **vết kiểm toán trước/sau** và **một dòng nhật ký** cùng giao dịch | Giao diện khoá `task.assign` như prototype trong khi máy chủ khoá `task.update` |
| 4a | Lịch sử gia hạn | `GET /api/v1/tasks/{ma}/extensions`, `task.read`. Thêm cột `decision_note` (migration `0031`) cho ghi chú duyệt/từ chối; vết kiểm toán chỉ ghi **độ dài** ghi chú | — |
| 4b | Gỡ tệp đính kèm | Người **tải lên** hoặc người giữ **`task.update`**. Gỡ được cả tệp **đã gắn nhật ký**. **Lý do bắt buộc**. **Xoá mềm**. Tệp đang giữ pháp lý → **409**. Nhật ký thêm dòng "đã gỡ" **không chép tên tệp** | Chép tên tệp vào dòng nhật ký |
| 4c | "Cập nhật và giao việc" trong một lần chuyển trạng thái | **Theo prototype**: nhiệm vụ về **đúng trạng thái đã chọn**, **không** về `moi-giao`. Ai được làm: người giữ `task.assign` **hoặc** người đang thực hiện được giao. **Chỉ** đường gộp này; `POST /api/v1/tasks/{ma}/assignment` riêng **giữ luật 28/09** | Áp luật 28/09 ("đổi người giữ thì về `moi-giao`") cho cả đường gộp |
| 5 | Luật prototype "≥1 tệp trước khi sang Chờ duyệt" | **Không áp dụng** — chưa ai chốt | Dựng theo prototype |

**Vì sao #1 thắng chữ "05/10 #3" trong lần 5 #4:** điều chủ dự án muốn giữ ở lần 5 #4 là **hình thức
ngăn phải + tab bản ghi** — quyết định nghe được trực tiếp ngày 05/10. Thanh tab thao tác chỉ là cách
bày nút bên trong ngăn, tức là sở thích trình bày, nên rơi vào lần 5 #3. Chủ dự án xác nhận cách đọc
này ngày 07/10.

**Vì sao mã vẫn không sửa được (#1):** đó là luật cứng, không phải sở thích — ADR 0065 NV3 (luật 7 bất
biến 3: mã đã cấp là số trên hồ sơ lưu trữ). Lần 5 #4 đã liệt kê "mã đã cấp bất biến" trong các điểm
giữ bất kể prototype.

**Vì sao #3 khoá `task.update` mà không theo prototype:** máy chủ đã khoá `task.update`. Giao diện khoá
khác máy chủ thì người có `task.assign` thấy ô bấm được rồi nhận 403, còn người có `task.update` không
thấy ô — giao diện nói dối về quyền. Luật 5 đặt quyền ở tầng dịch vụ; giao diện chỉ phản chiếu nó.
Không thêm khoá mới (luật 5 bất biến 3c).

**Vì sao #4b không chép tên tệp:** tên tệp do cán bộ đặt, có thể chứa họ tên hoặc số định danh công dân
(luật 3). Nhật ký nhiệm vụ không sửa được (luật 7 cấm #5), nên một tên tệp chép vào đó **không ẩn danh
được** khi có yêu cầu theo NĐ 13/2023. Tên tệp chỉ ở trên dòng tệp đính kèm — một chỗ duy nhất phải xử
lý khi ẩn danh.

**Vì sao #4b trả 409 với tệp giữ pháp lý:** tệp đang giữ pháp lý là bằng chứng; gỡ nó, dù mềm, là đổi
hồ sơ đang bị niêm phong. 409 nói rõ "trạng thái bản ghi không cho", khác với 403 "bạn không có quyền".

**Vì sao #4c đảo quyết định 28/09 chỉ cho đường gộp:** luật 28/09 (`service-petitions/internal/app/task_assignment.go:5-10`)
trả về `moi-giao` vì người mới chưa nhận việc. Ở đường gộp, người giao **chọn trạng thái đích ngay trong
cùng hộp**, nên trạng thái ấy là ý định rõ ràng của người có thẩm quyền — ép về `moi-giao` là ghi đè ý
định ấy. Đường `/assignment` riêng không có ô trạng thái, nên luật 28/09 vẫn là đáp án đúng ở đó. Chủ dự
án chọn theo prototype (lần 5 #3); ngoài lời ấy chủ dự án **không nêu** lý do thêm — đừng suy thêm.

**Vì sao #5 không dựng:** đây là luật nghiệp vụ (chặn một bước chuyển trạng thái), không phải trình bày.
Lần 5 #3 chỉ cho prototype thay **sở thích trình bày**; luật nghiệp vụ chưa ai chốt thì không tự chọn.

## Quan hệ với ADR khác

ADR không bao giờ sửa; các ADR dưới đây giữ nguyên chữ, ADR này thắng ở đúng các chỗ nêu.

| ADR | Chỗ | Nay |
|---|---|---|
| 0068 *Sửa đổi 05/10/2026* #3 | Thanh **tab thao tác** Xem chi tiết · Chỉnh sửa · Xoá | **Bị thay** (#1). Phần còn lại của #3 (ngăn chi tiết, khối trạng thái, hai cột, thao tác chính có chữ) giữ |
| 0068 *Sửa đổi 05/10/2026* #4 | "Tab Xoá mở luồng xoá mềm kèm lý do" | Luồng giữ; chỗ đặt chuyển sang cột thao tác |
| 0068 *lần 5* #5 | "Chỉ front-end, không đổi API, migration" | Menu Nhiệm vụ đổi hợp đồng REST và thêm migration `0031` (#4a–#4c). Không thêm khoá quyền mới |
| 0065 NV3 | Mã nhiệm vụ không sửa | **Giữ**, kể cả khi prototype cho sửa |
| 0038 | Ai duyệt lùi hạn: người ghi trên bản ghi | Không đổi. #4a chỉ thêm đường **đọc** lịch sử và ghi chú quyết định |

## Hệ quả

- Ngăn chi tiết nhiệm vụ không còn khác bố cục prototype ở thanh tab; ADR 0068 05/10 #3 nay chỉ đúng
  một phần — người đọc 0068 phải đọc tiếp ADR này.
- Hai đường đổi người giữ việc cho **hai kết quả trạng thái khác nhau**: `/assignment` về `moi-giao`,
  đường gộp giữ trạng thái đã chọn. Đây là chủ ý; ai hợp nhất hai đường phải hỏi lại chủ dự án.
- Người đang thực hiện tự giao lại được ở đường gộp mà không cần `task.assign`. Vết kiểm toán của lần
  chuyển **phải** ghi mã cán bộ của người làm (luật 6 bất biến 8) — #4c chưa dựng xong, chưa kiểm.
- Ghi chú quyết định gia hạn (`decision_note`) chỉ ghi cùng lần duyệt/từ chối, đóng băng sau đó; vết
  kiểm toán không giữ nội dung (luật 6 cấm #4).
- Migration `0031` **chưa chạy trên PostgreSQL thật** lúc chốt — máy làm việc không có `VIGOV_TEST_DSN`.
- Commit: `ab8f9160` (web, #1–#2) · `07bfeef2` · `9a1a1644` (#4a) · `9d4b2684` (#4b) · `49056d72` (#3).
  #4c **đang dựng** lúc viết ADR này.

## Còn mở — chưa ai chốt, không tự chọn

| # | Câu | Mã hôm nay |
|---|---|---|
| 1 | Đề nghị lùi hạn **đang chờ duyệt** khi nhiệm vụ đổi người giữ việc (sổ `service-petitions` → `giao-lai-nhiem-vu` mục 6) — áp cho cả `/assignment` lẫn đường gộp #4c | Giữ nguyên; người duyệt vẫn là lãnh đạo ghi trên bản ghi (ADR 0038) |
| 2 | Luật bằng chứng: có bắt ≥1 tệp trước khi sang *Chờ duyệt* không (#5) | Không chặn |
| 3 | Trọng số việc con: có, lưu ở đâu, tính tiến độ việc cha thế nào (#2) | Control vô hiệu dấu "?" |

→ ADR 0068 (*Sửa đổi 05/10/2026* #3/#4/#6, *lần 5* #3–#5) · ADR 0065 NV3 · ADR 0038
→ Sổ tiến độ: `python tools/tien_do.py --menu nhiem-vu`
