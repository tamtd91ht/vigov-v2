---
id: 0020-xac-thuc-so-dien-thoai-cong-dan
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d952aaf
expires: null
owns_facts:
  - "cách xác thực số điện thoại công dân ở kênh Zalo Mini App và cái giá của nó"
---

# 0020. Xác thực số điện thoại công dân bằng `getPhoneNumber` của Zalo

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17

## Bối cảnh

ADR 0002 trả lời câu **định danh công dân nằm ở đâu**: ở tầng nền tảng, một số điện thoại một
bản ghi, quan hệ với xã là nhiều-nhiều. Nó mô tả kèm một kho OTP khoá theo cặp *(số điện thoại,
xã)*.

Nó **không** trả lời câu kế tiếp, và câu kế tiếp mới là câu phải viết mã: *làm sao biết số điện
thoại này thật sự thuộc về người đang cầm máy.*

**ADR 0002 không bị thay thế.** Quyết định về *định danh* giữ nguyên từng chữ; tệp này chỉ chốt
*cách xác thực* — hai thứ khác nhau, và gộp chúng là lý do người ta hay sửa nhầm ADR cũ.

## Sự thật kỹ thuật đã tra

Luồng `getAccessToken()` + `getPhoneNumber()` ở client, backend đổi token tại `graph.zalo.me`
bằng **secret key của Mini App**, và **không đi qua OA nào**. Nguyên văn kết quả tra, URL nguồn
và mức chất lượng chứng cứ nằm ở `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`
§*Kết quả tra tài liệu*, câu 4 — đọc ở đó, không chép sang đây.

Điều đáng rút ra cho quyết định này: **đường đăng nhập độc lập với việc xã đã có OA hay chưa.**
Một xã vừa onboard, OA còn đang chờ duyệt, người dân của xã đó vẫn đăng nhập được.

## Ba hướng

| Hướng | Được | Mất |
|---|---|---|
| **`getPhoneNumber` là đường duy nhất** — đã chọn | Một chạm. Số đã được Zalo xác thực. Không màn nhập 6 số. Không tốn tiền SMS | Phụ thuộc một nhà cung cấp. Số nhận được là số của tài khoản Zalo, không nhất thiết là số xã đang giữ |
| Zalo là chính, **OTP SMS dự phòng** | Có đường lui khi Zalo hỏng | Phải nuôi **hai** đường xác thực và hai kho trạng thái cho một việc, trong đó đường thứ hai gần như không bao giờ chạy — tức là đường không ai kiểm, hỏng âm thầm, và hỏng đúng ngày cần tới |
| **Tự gửi OTP SMS** | Không phụ thuộc Zalo | Tốn tiền theo từng tin. Và bắt người dân nhập 6 số **ngay trong một app Zalo đã biết số của họ** — thêm một rào mà không thêm một chút chắc chắn nào |

## Quyết định

**`getPhoneNumber` là đường chính, và ở kênh Zalo Mini App là đường duy nhất.**

Lý do nặng nhất không phải chi phí SMS mà là **màn nhập 6 số**. Đó là rào thật với người cao
tuổi: chuyển ứng dụng để đọc tin nhắn, nhớ sáu chữ số, quay lại, gõ đúng trước khi hết hạn —
mỗi bước là một chỗ bỏ cuộc. Và kênh này người dân **không chọn**: họ buộc phải đi qua nó để
tiếp cận dịch vụ công. Một rào ở đây không làm mất một khách hàng, nó làm một người dân **không
dùng được dịch vụ của chính quyền mình**.

→ `.claude/skills/accessibility-elderly/SKILL.md`

## Cái phải trả

Đây là phần đắt nhất của ADR này. Ba khoản, không khoản nào có cách né.

### 1. Số Zalo có thể khác số công dân đã khai với xã

Zalo trả về **số đăng ký tài khoản Zalo**. Không có gì bảo đảm nó trùng số mà công dân đã khai
trong một hồ sơ nộp năm ngoái.

Hệ quả cụ thể: hồ sơ cũ khai số A, công dân đăng nhập bằng số B → **hệ thống thấy hai người**.
Công dân mở app, không thấy hồ sơ mình đã nộp, và kết luận là hệ thống làm mất hồ sơ của mình.

Đây là **vấn đề đối soát dữ liệu**, không phải lỗi đăng nhập, nên nó không tự hết khi luồng
đăng nhập chạy đúng. Cách xử lý khi lệch số **chưa ai chốt** → §CÒN MỞ.

### 2. Phụ thuộc một nhà cung cấp — rủi ro đã chấp nhận có ý thức

Chọn đường duy nhất nghĩa là: Zalo đổi chính sách, siết điều kiện cấp quyền, hoặc từ chối cấp
quyền cho app này, thì **không còn đường đăng nhập nào khác** cho công dân. Không phải suy giảm
chất lượng — là **đóng cửa kênh**.

Rủi ro này được chấp nhận vì hướng "dự phòng" ở bảng trên không thật sự mua được an toàn: một
đường dự phòng không bao giờ chạy là một đường không ai kiểm.

**Điều kiện phải mở lại quyết định này:**

| # | Khi nào |
|---|---|
| 1 | Zalo đổi điều kiện cấp quyền `getPhoneNumber`, hoặc gắn nó vào một ràng buộc mới |
| 2 | Có kênh công dân thứ hai không phải Mini App (web, tổng đài) — xem khoản 3 |
| 3 | Đo được một tỷ lệ công dân đáng kể **không có tài khoản Zalo** ở địa bàn đang triển khai |
| 4 | Yêu cầu pháp lý đòi một phương thức xác thực do phía nhà nước kiểm soát |

### 3. Kênh khác sau này không dùng lại được đường này

`getPhoneNumber` chỉ tồn tại **bên trong** Mini App. Một web công dân hay một tổng đài sẽ phải
có đường xác thực riêng của nó — và ngày đó phải **quay lại ADR này**, vì lúc ấy hệ thống có
hai cách xác thực cho cùng một định danh, và câu "hai đường có tin nhau ở mức ngang nhau không"
là câu phải trả lời trước khi viết mã, không phải sau.

## Bất biến

| # | Bất biến | Neo vào |
|---|---|---|
| 1 | Số điện thoại lấy được là **dữ liệu cá nhân**: không vào log, ra API phải che (`MaskPhone`), không nằm trong URL, tên tệp, khoá cache, tên phòng realtime | Luật 3 |
| 2 | Token từ `getPhoneNumber` đổi **ở backend**, không bao giờ ở client — secret key của Mini App là bí mật | Luật 8 |
| 3 | Xác thực xong **vẫn là danh tính yếu**: không đủ một mình cho hành vi có hậu quả pháp lý | Luật 4 |
| 4 | Phiên công dân phát hành sau đó vẫn mang **đúng một xã đang chọn**, và đổi xã vẫn **ghi vết** | ADR 0002 |
| 5 | Mỗi lần đăng nhập ghi vết: **lúc nào · từ IP nào · xã nào**. Số điện thoại trong bản ghi vết phải **che** | Luật 6, cấm #4 |

**Vì sao bất biến 3 không được nới dù Zalo đã xác thực:** Zalo khẳng định *"số này thuộc tài
khoản Zalo này"*. Nó **không** khẳng định *"người đang cầm máy là chủ số"*. Máy mở sẵn, máy cho
mượn, máy của con cháu — cả ba đều qua cửa. Đổi từ OTP sang Zalo làm luồng **dễ hơn**, không
làm danh tính **mạnh hơn**.

**Vì sao bất biến 5 dễ vi phạm nhất:** bản ghi vết là chỗ người ta thấy hợp lý khi lưu số điện
thoại — "để còn biết ai đăng nhập". Nhưng vết kiểm toán lưu tối thiểu 12 tháng và không xoá
được, nên một số lưu thô ở đó là một kho dữ liệu cá nhân vĩnh viễn, đúng thứ luật 6 cấm #4 nói
tới. Ghi mã định danh công dân, không ghi số.

## Hệ quả cho ADR 0002

Kho OTP khoá theo *(số điện thoại, xã)* mà ADR 0002 mô tả **không còn đường nào dùng tới** ở
kênh Zalo Mini App. Điều đó ghi ở đây, **không sửa ADR 0002** — tệp đó ghi đúng suy nghĩ của
ngày 15/09, và suy nghĩ đó không sai, nó chỉ mất chỗ dùng.

**Đừng xoá khái niệm ấy khỏi mô hình.** Ngày có kênh web hoặc tổng đài (khoản 3 ở trên), OTP
quay lại, và quay lại với đúng hình dạng đó: khoá theo cặp *(số điện thoại, xã)*, để xin OTP ở
xã A không chặn xã B.

## CÒN MỞ — khách phải chốt

| # | Câu hỏi | Vì sao không tự quyết được |
|---|---|---|
| 1 | **Công dân đăng nhập bằng số khác số đã khai trong hồ sơ cũ thì xử lý thế nào?** Gộp hai bản ghi? Cho công dân tự nhận hồ sơ bằng mã tra cứu? Để cán bộ đối soát thủ công? Hay chấp nhận là hai người? | Mỗi đáp án là một mô hình dữ liệu khác nhau **và** một quy trình hành chính khác nhau. Gộp nhầm hai người là để người này đọc hồ sơ người kia — hỏng đúng luật 4. Không gộp thì công dân mất dấu hồ sơ của chính mình. Không có đáp án nào an toàn mặc định, nên không được chọn hộ |
| 2 | **Ai chịu trách nhiệm đối soát khi lệch số** — công dân tự làm trong app, hay cán bộ xã làm? | Đây là phân công công việc trong xã, không phải thiết kế phần mềm |

## ĐIỀU KIỆN DỪNG

1. Bất kỳ đề xuất nào **đổi token ở phía client** để "cho gọn" — đó là đưa secret key của Mini
   App ra thiết bị người dùng
2. Một hành vi mới cho công dân mà **chưa rõ có hậu quả pháp lý hay không** — bất biến 3 không
   tự phân loại hộ
3. Chạm tới câu 1 của §CÒN MỞ bằng bất kỳ dòng mã nào (gộp bản ghi, nhận hồ sơ cũ theo số mới)
4. Xuất hiện kênh công dân thứ hai — quay lại tệp này trước khi thiết kế xác thực cho nó

→ ADR 0002 (định danh công dân, phiên mang một xã, kho OTP): `kb/10-decisions/0002-citizen-identity-platform-level.md`
→ ADR 0018 (kết quả tra Zalo, câu 4 — nguồn của sự thật kỹ thuật ở đây): `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`
→ ADR 0005 (xã của phiên được xác định thế nào sau khi đăng nhập): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ Luật 3 (dữ liệu cá nhân, che khi ra API): `.claude/rules/critical/3-personal-data.md`
→ Luật 4 (cách ly công dân, danh tính yếu): `.claude/rules/critical/4-citizen-isolation.md`
→ Luật 6 (ghi vết, cấm lưu thô dữ liệu cá nhân): `.claude/rules/critical/6-audit-log.md`
→ Luật 8 (secret key không ra khỏi backend): `.claude/rules/critical/8-secrets-config.md`
→ Kỹ năng: `.claude/skills/citizen-identity-multi-tenant/SKILL.md` · `.claude/skills/accessibility-elderly/SKILL.md` · `.claude/skills/mask-personal-data/SKILL.md`
