---
id: 0076-ngan-chi-tiet-nhiem-vu-theo-prototype
tier: T1
source: CURATED
owner: domain
derived_from_commit: cca90b98
expires: null
owns_facts:
  - "ngăn chi tiết nhiệm vụ KHÔNG còn 3 tab thao tác Xem chi tiết · Chỉnh sửa · Xoá: sửa tại chỗ bằng '✎ Sửa' trong khối thông tin, Xoá (xoá mềm, có lý do) ở cột thao tác; ngăn phải + tab bản ghi giữ (chốt 07/10/2026) — vế 'Xoá ở cột thao tác' THAY bởi §Sửa đổi 07/10/2026 (lần 2) #15: không còn cột thao tác, Xoá ở đầu ngăn"
  - "ô tick + trọng số việc con vẽ như prototype nhưng là control vô hiệu dấu '?' tới khi máy chủ có trọng số (chốt 07/10/2026)"
  - "hai ô phê duyệt nhiệm vụ (lãnh đạo xã phê duyệt hoàn thành · cấp trên công nhận) khoá bằng task.update ở cả máy chủ và giao diện, tick thẳng ở chế độ xem, mỗi lần đổi có vết trước/sau + một dòng nhật ký (chốt 07/10/2026)"
  - "gỡ tệp đính kèm nhiệm vụ: người tải lên hoặc task.update; gỡ được cả tệp đã gắn nhật ký; lý do bắt buộc; xoá mềm; tệp giữ pháp lý → 409; dòng nhật ký 'đã gỡ' KHÔNG chép tên tệp (chốt 07/10/2026)"
  - "'Cập nhật và giao việc' trong một lần chuyển trạng thái: nhiệm vụ về ĐÚNG trạng thái đã chọn, không về moi-giao; người có task.assign HOẶC người đang thực hiện được giao — đảo quyết định 28/09/2026 CHỈ cho đường gộp; POST /tasks/{ma}/assignment giữ luật cũ (chốt 07/10/2026)"
  - "luật prototype 'phải có ≥1 tệp trước khi sang Chờ duyệt' KHÔNG áp dụng — chưa ai chốt (07/10/2026)"
  - "từ 07/10/2026 (lần 2) menu Nhiệm vụ web-admin theo spec Nhiệm vụ 02–10 của chủ dự án (tmp/web/nhiem-vu/vigov-nhiem-vu-spec/vigov-nhiem-vu-spec/, ngoài git); ngăn chi tiết theo spec 07 TRỪ thanh tab nhiều hồ sơ + 'Đóng tất cả' (giữ); cột icon dọc bên phải bỏ"
  - "menu Nhiệm vụ theo chữ spec ở bốn chỗ (chốt 07/10/2026 lần 2 #2): nguồn 'Giao trực tiếp' (đảo NV-11), nhãn trạng thái CỐ ĐỊNH của spec (nhãn riêng của xã không hiện ở menu này — đảo câu #21 trên web, cần xác nhận với khách hàng), ô trống Lãnh đạo giao việc '— Người đang tạo nhiệm vụ —' (máy chủ vẫn không lưu người giao), loại mặc định = is_default hoặc dòng đầu (đảo a1e5e64f)"
  - "câu gợi ý trạng thái nhiệm vụ hiện NGUYÊN VĂN spec 10 dù nói luật hệ thống không áp (hoàn thành không cần duyệt, chờ duyệt không bắt minh chứng) — luật máy chủ không đổi (chốt 07/10/2026 lần 2 #3)"
  - "bộ lọc phạm vi trang Nhiệm vụ = 3 lựa chọn của spec 02; 'Tôi đã giao' bỏ khỏi trang này, Sổ tay lãnh đạo giữ cột của nó (chốt 07/10/2026 lần 2 #4)"
  - "phần spec Nhiệm vụ cần máy chủ (extension_count/pending_extension trên dòng, collaborator_ids, trọng số việc con, tổng danh sách, sắp xếp máy chủ theo người/bộ phận/trạng thái, nguồn recurring, tệp chưa gắn, email trong ô chọn cán bộ, requires_directive) = control '?' + BACKEND DEPENDENCY; không thêm react-hook-form/zod/@tanstack/react-table/@dnd-kit/sortable (07/10/2026 lần 2)"
  - "menu Nhiệm vụ GIỮ ba thứ prototype không có — menu 'Chuyển sang cột…' trên thẻ Kanban, nút 'Xuất Excel' Sổ theo dõi, phân trang máy chủ + 'Xem thêm' — vẽ lại theo UI chung (chốt 07/10/2026 lần 2 #13)"
  - "ngăn chi tiết nhiệm vụ KHÔNG có ô 'Việc cha' và không có liên kết lên việc cha, như prototype; API PATCH {parent}, luật cây ADR 0037 và '+ Thêm việc con' trong ngăn của việc cha giữ (chốt 07/10/2026 lần 2 #14)"
  - "ngăn chi tiết nhiệm vụ không có cột thao tác bên phải; Xoá (xoá mềm, có lý do) ở đầu ngăn; dòng 'Tạo bởi …' bỏ nếu prototype không có (chốt 07/10/2026 lần 2 #15)"
  - "Sổ theo dõi nhiệm vụ vẽ đủ 13 cột prototype, ngăn chi tiết có ô 'Cơ quan chủ trì tham mưu' và 'Chuyên viên VP tham mưu/theo dõi' — hiện CÙNG dữ liệu với Đơn vị thực hiện / Người thực hiện; chỉ trình bày, mô hình dữ liệu ADR 0065 NV5 giữ (chốt 07/10/2026 lần 2 #16)"
  - "hộp 'Giao việc mới' rộng 500px, 800px khi loại 'Theo văn bản' (spec 06) — thay chốt 07/10/2026 'luôn 800px' vốn chỉ nằm ở sổ tiến độ và mã (chốt 07/10/2026 lần 2 #17)"
  - "menu Nhiệm vụ: trạng thái trống/lỗi của danh sách giữ cách xử lý hiện có; số liệu chỉ đọc cần máy chủ (vd 'đã gia hạn n lần') = control vô hiệu '?' đúng vị trí prototype (mặc định phiên 07/10/2026 lần 2 #18–#19)"
---

# 0076. Ngăn chi tiết nhiệm vụ theo prototype và bốn thao tác máy chủ

**Trạng thái:** đã chốt · **Ngày:** 2026-10-07 · **Người quyết:** chủ dự án, 07/10/2026, trong lượt
`/fix-web-admin --menu=nhiem-vu` · **Bổ sung** ADR 0068 *Sửa đổi 06/10/2026 (lần 5)* cho riêng menu
Nhiệm vụ, **thay một phần** 0068 *Sửa đổi 05/10/2026* #3 — xem §*Quan hệ với ADR khác* ·
**Sửa đổi 07/10/2026 (lần 2)** (menu Nhiệm vụ theo spec 02–10; giữ tab nhiều hồ sơ; bốn chỗ chữ theo
spec đảo NV-11, câu #21 trên web, `a1e5e64f`; lô hai #13–#19: bỏ ô Việc cha, Xoá lên đầu ngăn, Sổ theo dõi
13 cột, hộp giao việc 500/800 — §*Sửa đổi 07/10/2026 (lần 2)*).

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

## Sửa đổi 07/10/2026 (lần 2) — Nhiệm vụ theo spec

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác. **Người quyết:** chủ dự án,
07/10/2026, trong phiên chính (lệnh `/fix-web-admin --menu=nhiem-vu`, mô tả *"cập nhật UI UX theo
prototype, tham khảo thêm nội dung hướng dẫn trong ./tmp/web/giai-ngan/vigov-nhiem-vu-spec/*.md"*).
**Chưa dựng** — mỗi dòng là điều phải đúng khi dựng.

Nguồn: bản spec Nhiệm vụ của chủ dự án, thư mục thật
`tmp/web/nhiem-vu/vigov-nhiem-vu-spec/vigov-nhiem-vu-spec/` (ngoài git; lệnh ghi đường dẫn khác),
tệp **02–10**. Tệp 00/01 trùng spec Giải ngân, đã áp bởi ADR 0068 *lần 6*. Nội dung spec **không chép
vào đây** (luật 9) — spec là nguồn lúc dựng; thứ tự nguồn giữ như ADR 0068 *lần 6* (luật cứng lần 5 #4
thắng, trừ điểm nói rõ thay).

Câu trả lời của chủ dự án (chọn trong câu hỏi của phiên, nguyên văn lựa chọn):

| # | Câu hỏi của phiên | Lựa chọn nguyên văn | Chốt |
|---|---|---|---|
| 1 | Spec 07 dùng **một** ngăn chi tiết, không thanh tab nhiều hồ sơ / "Đóng tất cả"; ADR này (#1) giữ ngăn phải + tab nhiều hồ sơ | **"Giữ tab nhiều hồ sơ"** | Thanh tab nhiều hồ sơ + "Đóng tất cả" **giữ** (ADR 0068 *05/10* dòng tab nhiều bản ghi vẫn đúng). **Phần còn lại** của ngăn theo spec 07 — gồm việc bỏ cột icon dọc bên phải mà spec 07 dòng 3 nêu tên |
| 2 | Bốn chỗ spec đặt chữ khác chỗ đã chốt: nguồn "Giao trực tiếp" (đã sửa thành "Tạo trên sổ nhiệm vụ" theo báo cáo kiểm thử NV-11) · nhãn trạng thái cố định (xã tự đặt nhãn, câu mở #21) · ô trống "Lãnh đạo giao việc" = "— Người đang tạo nhiệm vụ —" (máy chủ không tự lấy người tạo) · loại mặc định = dòng đầu (chốt 07/10: để trống) | **"Theo spec toàn bộ"** | Đổi **cả bốn** theo spec 02/04/06/10, kể cả câu nói sai hành vi máy chủ — hệ quả (a), (b), (d), (e) |
| 3 | Câu gợi ý trạng thái của spec nói sai luật của ta: "Hoàn thành: Lãnh đạo đã duyệt" (ADR 0065 cho người thực hiện tự hoàn thành) · "Chờ duyệt: Phải có minh chứng" (#5 trên chưa áp luật ≥1 tệp) · "Chuyển tiếp: chờ người mới tiếp nhận" | **"Nguyên văn spec"** | Câu gợi ý hiện **đúng chữ spec 10**. Luật máy chủ **không đổi** — hệ quả (c) |
| 4 | Bộ lọc phạm vi: spec 3 lựa chọn (Tất cả / Được giao / Liên quan), web 4 (thêm "Tôi đã giao") | **"3 như spec"** | Trang Nhiệm vụ còn 3 lựa chọn theo spec 02; "Tôi đã giao" **bỏ khỏi trang này** — hệ quả (f) |

**Mặc định phiên nêu, chủ dự án không phản đối** — giữ nguyên vì luật hoặc ADR khác sở hữu:

| # | Giữ | Chủ sở hữu |
|---|---|---|
| 5 | Không thêm ô bộ phận chủ trì / người theo dõi; Sổ theo dõi giữ 11 cột | ADR 0065 NV5 |
| 6 | Mã nhiệm vụ không sửa được | ADR 0065 NV3 · #1 trên |
| 7 | Lùi hạn do người ghi trên bản ghi quyết, khoá `task.extend` | ADR 0038 |
| 8 | Gỡ tệp đính kèm phải có lý do | #4b trên |
| 9 | Xoá hàng loạt cần **một** lý do chung | Luật 7 |
| 10 | Mức ưu tiên lấy từ danh mục của xã; màu vạch theo hạng | Luật 1 bất biến 10 |
| 11 | Không thêm thư viện (`react-hook-form`, `zod`, `@tanstack/react-table`, `@dnd-kit/sortable`): hành vi đã dựng; sắp xếp **phải ở máy chủ** vì danh sách phân trang bằng con trỏ | — (lựa chọn kỹ thuật của phiên) |
| 12 | Phần spec cần máy chủ → control vô hiệu dấu "?" + **BACKEND DEPENDENCY** (ADR 0068 *lần 6* #9): `extension_count` / `pending_extension` trên dòng danh sách · `collaborator_ids` · trọng số việc con · tổng của danh sách (số đếm vẫn lấy từ `/task-counts`) · sắp xếp ở máy chủ theo người thực hiện / bộ phận / trạng thái · nguồn `recurring` · danh sách tệp chưa gắn · email trong ô chọn cán bộ · thuộc tính `requires_directive` của loại nhiệm vụ | ADR 0068 *lần 5* #5 · *lần 6* #9 |

**Còn mở / hệ quả** (chủ dự án **không nêu** lý do cho bốn lựa chọn trên — đừng suy thêm):

| # | Điểm | Thực tế |
|---|---|---|
| a | Nhãn trạng thái của xã | Câu mở **#21** đã **DECIDED**: xã tự đặt nhãn qua `/task-statuses`. Lựa chọn #2 làm web hiện **nhãn cố định của spec** trên các màn menu Nhiệm vụ, nên nhãn riêng của xã **không còn hiện ở menu này**; API và danh mục giữ nguyên. Đây là **đảo một câu khách hàng đã chốt** — phải nêu lại với khách hàng để xác nhận. `open-questions.json` **chưa sửa** |
| b | Ô trống "Lãnh đạo giao việc" | Chữ "— Người đang tạo nhiệm vụ —" nói người tạo là lãnh đạo giao việc, nhưng để trống thì máy chủ **không lưu** người giao việc — **bản ghi và màn hình nói khác nhau**. Ai duyệt lùi hạn khi ô này trống: ADR 0038 |
| c | Câu gợi ý trạng thái | Hiện nguyên văn luật hệ thống **không áp**: hoàn thành **không** cần duyệt (ADR 0065), Chờ duyệt **không** bắt minh chứng (#5). Cán bộ có thể đọc một nhiệm vụ "Hoàn thành" là **đã được lãnh đạo duyệt** khi thực tế không |
| d | Nhãn nguồn `truc-tiep` | Sửa theo báo cáo kiểm thử **NV-11** ("Tạo trên sổ nhiệm vụ") **bị đảo** — về "Giao trực tiếp" |
| e | Loại nhiệm vụ mặc định | Chốt 07/10 "không có loại mặc định thì để trống" (`a1e5e64f`, sổ tiến độ NV-01) **bị đảo**: mặc định = loại `is_default`, không có thì **dòng đầu** (spec 06) |
| f | "Tôi đã giao" | Bỏ khỏi trang Nhiệm vụ. Cột *Việc tôi đã giao* của Sổ tay lãnh đạo (ADR 0071) **giữ**; tham số `scope=assigned-by-me` của API giữ |
| g | Gợi ý ô *Lãnh đạo giao việc* "…qua chuông và qua thư" (spec 06) | Phiên đưa lại câu này theo spec, **cách đọc của phiên** từ hai câu trả lời "Theo spec toàn bộ" và "Nguyên văn spec" (chủ dự án không nêu riêng câu này). Câu ấy từng bị bỏ có chủ ý (bbcaf3c7): gửi đề nghị lùi hạn hiện **không** phát thông báo chuông hay thư — màn hình hứa một việc hệ thống không làm. Chủ dự án phủ quyết thì bỏ lại |

### Lô câu trả lời thứ hai (cùng phiên, 07/10/2026)

Cùng người quyết, cùng phiên. Nguyên văn lựa chọn của chủ dự án:

| # | Câu hỏi của phiên | Lựa chọn nguyên văn | Chốt |
|---|---|---|---|
| 13 | Prototype **không có** bốn thứ web đang chạy: menu "Chuyển sang cột…" trên thẻ Kanban (đường dùng bàn phím/cảm ứng), nút "Xuất Excel" Sổ theo dõi, ô "Việc cha" (ADR 0037), phân trang máy chủ + "Xem thêm" | Trả lời tự do: **"tôi lăn tăn mục ô Việc cha, hãy đánh giá thêm chỗ này, ngoài ra 3 cái kia cho phép giữ lại nhưng phải chuẩn UI UX chung nhé"** | Menu chuyển cột Kanban, "Xuất Excel", phân trang máy chủ + "Xem thêm" **giữ**, vẽ lại theo UI chung. Ô Việc cha → #14 |
| 14 | Sau khi phiên đánh giá (prototype có dữ liệu `parent_id` nhưng không có ô đặt nó, form tạo luôn gửi `parent_id` null; spec khách hàng `docs/ui-ux/02-nhiem-vu.md` §5.10 chỉ nêu danh sách việc con + thêm, không nêu ô cha): "Ô 'Việc cha' trong ngăn chi tiết xử lý thế nào?" | **"Bỏ hẳn như prototype"** | Ngăn chi tiết **không có** ô Việc cha và **không có** liên kết lên việc cha. API `PATCH {parent}`, luật cây ADR 0037 và "+ Thêm việc con" trong ngăn của việc cha **giữ**. Phiên đã đề xuất giữ một liên kết chỉ đọc — **bị bác**. Hệ quả (h), (j) |
| 15 | Spec 07 không có cột nút tròn bên phải; #1 trên đặt Xoá ở "cột thao tác" | **"Bỏ cột, Xoá vào đầu ngăn"** | Không còn cột thao tác; Xoá ở **đầu ngăn**, vẫn xoá mềm có lý do (luật 7). Dòng "Tạo bởi …" bỏ nếu prototype không có. **Thay** vế "Xoá ở cột thao tác" của #1 |
| 16 | Sổ theo dõi và ngăn chi tiết prototype có "Cơ quan chủ trì tham mưu" và "Chuyên viên VP tham mưu/theo dõi"; ADR 0065 NV5 đã gộp hai vai này vào "Đơn vị thực hiện" / "Người thực hiện" (dữ liệu như nhau) | **"Theo đúng prototype"** | Sổ theo dõi vẽ đủ **13 cột**, ngăn có hai ô ấy; hai ô thêm hiện **CÙNG dữ liệu** với Đơn vị thực hiện / Người thực hiện. **Chỉ trình bày** — mô hình dữ liệu NV5 không đổi. **Thay** mặc định #5 trên ("Sổ theo dõi giữ 11 cột"). Hệ quả (i) |
| 17 | Hộp "Giao việc mới" hiện luôn rộng 800px (chốt 07/10, chỉ ghi ở sổ tiến độ và mã, chưa có ADR); spec 06: 500px bình thường, 800px khi loại "Theo văn bản" | **"Theo spec 500/800"** | Rộng **500px**; **800px** khi loại nhiệm vụ là "Theo văn bản". Chốt "luôn 800px" **hết hiệu lực** |

**Mặc định phiên nêu, chủ dự án không phản đối:**

| # | Giữ | Chủ sở hữu |
|---|---|---|
| 18 | Trạng thái trống / lỗi của danh sách giữ cách xử lý hiện có | Như ADR 0068 *lần 6* #11 cho Giải ngân |
| 19 | Số liệu chỉ đọc cần máy chủ (vd "đã gia hạn n lần") → control vô hiệu dấu "?" **đúng vị trí prototype** | ADR 0068 *lần 5* #5 · #12 trên |

**Hệ quả lô hai** (chủ dự án **không nêu** lý do cho #14–#17 — đừng suy thêm):

| # | Điểm | Thực tế |
|---|---|---|
| h | Từ việc con | **Không có** đường trên màn hình để tới việc cha. Cây vẫn đúng ở máy chủ (ADR 0037); việc gắn/đổi cha chỉ còn qua API hoặc "+ Thêm việc con" ở ngăn của việc cha |
| i | Hai ô chủ trì / theo dõi | Người đọc thấy **cùng một giá trị hai lần** và có thể nghĩ hệ thống có hai vai riêng — trong khi ADR 0065 NV5 nói là **một** vai |
| j | ADR 0068 *05/10* dòng "URL theo hộp" | Vế "đi tới việc **cha** trong hộp thay mục lịch sử" không còn chỗ dùng (#14); vế đi tới việc **con** giữ |
| k | Mặc định #5 lô một | "Không thêm ô bộ phận chủ trì / người theo dõi; Sổ theo dõi giữ 11 cột" **bị thay** bởi #16 |
