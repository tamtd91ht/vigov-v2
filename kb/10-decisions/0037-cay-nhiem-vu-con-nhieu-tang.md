---
id: 0037-cay-nhiem-vu-con-nhieu-tang
tier: T1
source: CURATED
owner: domain
derived_from_commit: d688233
expires: null
owns_facts:
  - "nhiệm vụ con cho phép NHIỀU TẦNG, không giới hạn độ sâu"
  - "việc con có hạn RIÊNG, không thừa kế hạn của cha"
  - "xoá mềm một nhiệm vụ còn con chưa xoá bị TỪ CHỐI"
  - "cha chỉ chuyển `hoan-thanh` khi mọi việc con đã xong, cưỡng chế ở MÁY CHỦ"
  - "vì sao nhiều tầng kéo theo bài toán CHU TRÌNH mà một CHECK không giải được"
---

# 0037. Cây nhiệm vụ con — nhiều tầng, hạn riêng, không xoá cha còn con

**Trạng thái:** đã chốt · **Ngày:** 2026-09-23 · **Chủ dự án quyết**
**Đóng điều kiện dừng #1** mà `service-petitions/migrations/0006_nhiem_vu.sql:40` nêu và **cố ý
không tự trả lời**

## Bối cảnh

`docs/ui-ux/02-nhiem-vu.md:208` và `:306` đòi việc con, nhưng đặc tả **không nói** bốn điều mà
một cột `nhiem_vu_cha_id` buộc phải quyết: sâu mấy tầng · con có thừa hạn không · xoá cha thì
con ra sao · và §11.4 viết *"cha hoàn thành chỉ khi con xong hết"* — một luật về hình dạng chưa
ai chốt.

Agent dựng nền `0006` **dừng lại và không tạo cột nào**, ghi lý do tại chỗ: thêm một cột nullable
về sau rẻ vì bảng rỗng ở mọi môi trường, còn đoán bây giờ thì người viết tuyến GHI đầu tiên tự
đặt ngữ nghĩa — và lúc ấy đã có dữ liệu thật mang ngữ nghĩa ấy.

## Quyết định

| # | Câu | Chốt |
|---|---|---|
| 1 | Sâu mấy tầng | **Nhiều tầng, không giới hạn** |
| 2 | Con thừa hạn cha? | **KHÔNG.** Con có hạn RIÊNG, tự khai, được phép để trống |
| 3 | Xoá mềm cha còn con | **TỪ CHỐI**, kèm câu nói rõ còn mấy việc con |
| 4 | Cha `hoan-thanh` khi con chưa xong | **MÁY CHỦ CHẶN**, trả lỗi liệt kê việc con còn lại |

### Vì sao #2 là "hạn riêng" chứ không thừa kế

Đặc tả `02-nhiem-vu.md:208` viết thẳng: *"Việc con có hạn riêng và cũng lên Sổ tay lãnh đạo."*
Thừa kế sẽ mâu thuẫn với chính câu ấy — một việc con không có hạn của mình thì không có gì để
lên Sổ tay.

**Hệ quả phải nhận, không phải tác dụng phụ:** một việc con **quá hạn được trong khi cha chưa**.
Đó là đúng nghiệp vụ — một bước trung gian trễ không có nghĩa cả nhiệm vụ đã trễ — nhưng nó
nghĩa là màn hình phải nói rõ con nào trễ, chứ không gộp thành một chấm đỏ trên cha.

### Vì sao #3 là "từ chối" chứ không xoá lan hay thả con ra

Ba lối đều đứng được, và cái giá khác nhau:

- **Xoá lan cả cây** — với cây nhiều tầng, **một lần bấm xoá hàng chục bản ghi**. Sổ nhiệm vụ
  không xoá cứng được (luật 7), nên "hoàn tác" là khôi phục **từng dòng**, và người bấm nhầm
  không biết mình vừa chạm bao nhiêu dòng.
- **Thả con thành việc gốc** — không mất việc nào, nhưng mất ngữ cảnh *"việc này sinh ra từ
  đâu"*, mà đó đúng là thứ cây nhiệm vụ sinh ra để giữ.
- **Từ chối** — cán bộ phải xử lý con trước. Phiền hơn, và **không bao giờ sinh ra bản ghi mồ
  côi** trỏ tới một cha đã biến mất khỏi mọi danh sách.

Chốt lối thứ ba. Phiền là cái giá trả một lần cho mỗi lần xoá; hai lối kia trả giá vào ngày có
người đi tìm một việc không còn ngữ cảnh.

### Vì sao #4 cưỡng chế ở MÁY CHỦ

Chỉ cảnh báo trên màn thì **tỷ lệ hoàn thành đếm cả những cha còn con dở** — một con số đẹp hơn
thực tế, và nó là con số đi lên lãnh đạo. Luật 5 cấm #1 đã nói cùng một điều cho phân quyền:
kiểm ở giao diện là không kiểm.

Cái giá: kiểm phải **duyệt đệ quy cả cây**, không chỉ một tầng con.

## CHU TRÌNH — hệ quả của #1 mà một `CHECK` không giải được

Nhiều tầng nghĩa là `A → B → C → A` biểu diễn được.

`CHECK (nhiem_vu_cha_id IS DISTINCT FROM id)` chỉ chặn ca **tự làm cha của mình** — đúng tiền lệ
`khoan_muc_ngan_sach_khong_tu_lam_cha` (`0006_thu_chi_ngan_sach.sql`). Nó **không** chặn được
vòng qua hai bản ghi trở lên, vì một `CHECK` chỉ nhìn thấy một dòng.

Một chu trình làm mọi phép duyệt cây — kiểm hoàn thành ở #4, đếm việc con, dựng màn — **chạy
vĩnh viễn**. Nên nó không phải chuyện thẩm mỹ.

**Chặn ở đường GHI**, nơi có thể đi ngược lên cha để tìm chính mình, và ghi rõ tại chỗ vì sao
lược đồ một mình không đủ. Lượt thêm cột chỉ mang được `CHECK` tự-làm-cha; phần còn lại thuộc
lượt viết tuyến GHI, và **không được quên** — ghi vào sổ tiến độ ngay khi thêm cột.

## Hệ quả

| | |
|---|---|
| Lược đồ | `nhiem_vu_cha_id` nullable, khoá ngoại hợp thành với `tenant_id` về chính bảng, kèm `CHECK` tự-làm-cha |
| Dữ liệu cũ | Không có. Bảng rỗng ở mọi môi trường — đây **không** phải migration trên hồ sơ lưu trữ |
| Đường ĐỌC | Duyệt cây cần truy vấn đệ quy. Màn Sổ tay §3 còn đòi cột "quá hạn" **bao gồm cả việc con** |
| Đường GHI | Ba luật #2 #3 #4 cộng chặn chu trình đều nằm ở đây, và đường GHI **chưa dựng** |

## Cái giá, nói thẳng

Không giới hạn độ sâu là lựa chọn **dễ cho người dùng, đắt cho mọi truy vấn**: đếm việc con, kiểm
hoàn thành, dựng cây trên màn đều thành đệ quy. Giới hạn hai tầng sẽ rẻ hơn hẳn và vẫn phủ phần
lớn ca thật. Chủ dự án chọn không giới hạn ngày 2026-09-23, biết cái giá ấy.

→ Luật 7 (không xoá cứng) · luật 10 bất biến 3 (quá hạn là suy ra)
→ Nền: `service-petitions/migrations/0006_nhiem_vu.sql`
