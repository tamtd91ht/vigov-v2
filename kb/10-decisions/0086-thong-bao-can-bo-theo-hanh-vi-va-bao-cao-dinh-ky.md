---
id: 0086-thong-bao-can-bo-theo-hanh-vi-va-bao-cao-dinh-ky
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d0b02227
expires: null
owns_facts:
  - "9 loại nhắn Zalo còn lại của ADR 0079 Q3 dựng chung một ADR: mã loại, nơi phát, hành vi phát, người nhận, khoá chống trùng (người dùng chốt 09/10/2026)"
  - "thông báo cán bộ theo hành vi đi qua outbox ghi trong giao dịch của hành vi; bộ chuyển trong tiến trình của service phát gọi gRPC CommsService.DeliverStaffNotifications — không broker (09/10/2026)"
  - "Việc chờ duyệt báo lãnh đạo giao việc của nhiệm vụ; không có thì báo người tạo (người dùng chốt 09/10/2026)"
  - "việc nền Gửi báo cáo định kỳ (scheduled_reports) theo prototype: thông báo chuông/Zalo kèm số chính, không tệp, không thư; mỗi service sở hữu sổ gửi phần của mình; kỳ HIỆN TẠI vừa bắt đầu theo giờ Việt Nam; nội dung dùng nhãn tiếng Việt — thay phần HOÃN của ADR 0053 B4 và ADR 0058 §4 (người dùng chốt 09/10/2026)"
  - "không thêm việc nền Tính lại số liệu Tổng quan: Tổng quan đếm trực tiếp (ADR 0053 §1, ADR 0058 §4) (người dùng chốt 09/10/2026)"
---

# 0086. Thông báo cán bộ theo hành vi (9 loại Zalo còn lại) và việc nền Gửi báo cáo định kỳ

**Trạng thái:** đã chốt · **Ngày:** 2026-10-09 · **Người quyết:** người dùng 09/10/2026, khi sửa
menu Cấu hình theo `tmp/web/updated/cap-nhat-menu-cau-hinh-theo-prototype.md`: "dựng cả 9, một ADR
chung, theo khuôn loại đang chạy"; Gửi báo cáo định kỳ "theo prototype". **Đóng** ADR 0079 §Còn mở
dòng 9. **Sửa** ADR 0053 B4 và ADR 0058 §4 (dòng `send_scheduled_reports`) cùng điều kiện dừng #5.
Hợp đồng: d0b02227.

## Bối cảnh

ADR 0079 Q3 để các loại nhắn Zalo cho cán bộ ở dấu "?" vì chưa có nơi phát, và chốt "mỗi loại một
ADR + outbox". Mười ba loại đang chạy đều do việc nền gọi gRPC `CommsService.DeliverStaffNotifications`;
comms ghi chuông và dòng chờ Zalo trong một giao dịch, rồi bộ phát Zalo gửi. Kho chưa có broker:
outbox `su_kien_di` của petitions không có bộ chuyển. Tiền lệ duy nhất phát theo hành vi là nhắc tên
Giải ngân (ADR 0081 #5): gọi sau commit, không outbox, chỉ chuông.

## Quyết định

### A1. Cùng một đường tới comms, thêm outbox phía bên phát

Hành vi ghi bản ghi, vết kiểm toán và MỘT dòng `staff_notice_outbox` trong cùng giao dịch. Bộ chuyển
chạy trong tiến trình của chính service ấy (khoá advisory, một pod chạy, như ADR 0058 §1) gửi các dòng
chưa giao qua `DeliverStaffNotifications`. Phía comms và Zalo không đổi gì so với loại đang chạy.
Ranh giới giao dịch: `thong_bao_can_bo_khi_giao_viec` trong `kb/30-indexes/transaction-boundaries.json`.

Vì sao không gọi comms ngay trong giao dịch: comms lỗi thì việc giao nhiệm vụ cũng lỗi theo — một
hồ sơ hành chính không được dừng vì kênh nhắn tin (luật 2 #5). Vì sao không gọi sau commit như ADR
0081: tiến trình chết giữa commit và lời gọi thì tin mất mà không ai biết.

### A2. Chín loại

| Loại | Mã lưu | Nơi phát · hành vi | Người nhận |
|---|---|---|---|
| Giao việc mới | `nhiem-vu.giao-moi` | petitions · tạo / giao lại nhiệm vụ | người thực hiện; chỉ giao bộ phận thì người có `task.assign` của bộ phận |
| Đề nghị lùi hạn chờ duyệt | `nhiem-vu.de-nghi-lui-han` | petitions · gửi đề nghị lùi hạn | lãnh đạo giao việc, không có thì người tạo |
| Việc chờ duyệt | `nhiem-vu.cho-duyet` | petitions · chuyển trạng thái sang chờ duyệt | lãnh đạo giao việc, không có thì người tạo |
| Được nhắc tên trong trao đổi | `nhiem-vu.nhac-ten` | petitions · ghi trao đổi có nhắc tên | người được nhắc |
| Văn bản, đơn thư chuyển tới mình | `van-ban.chuyen-toi` | documents · chuyển văn bản đến / đơn thư cho một cán bộ | cán bộ được chọn |
| Phản ánh được phân công | `phan-anh.phan-cong` | petitions · phân công cán bộ xử lý | cán bộ được phân công |
| Phản ánh bị mở lại | `phan-anh.mo-lai` | petitions · đánh giá thấp mở lại phiếu | cán bộ đang giữ phiếu |
| Thông báo mới gửi cho mình | `thong-bao.moi` | comms · phát hành thông báo nội bộ, ghi chuông trong chính giao dịch phát hành | người nhận của thông báo |
| Báo cáo điều hành đã sẵn sàng | `bao-cao.san-sang` | việc nền B1 | lãnh đạo |

Mã dây, nơi gọi chính xác và khoá: `proto/vigov/comms/v1/comms.proto` (giá trị 18–25).

### A3. Ba luật chung

1. Người vừa bấm nút không nhận tin.
2. Khoá chống trùng = `<mã lưu>:<id dòng hành vi>`. Mỗi lần giao là một quyết định; giao lại đúng
   người sau một vòng vẫn phải báo.
3. Tiêu đề và nội dung chỉ mang mã nghiệp vụ và tiêu đề do cán bộ viết. KHÔNG chép nội dung phản ánh,
   lời nhận xét của người dân, trích yếu đơn thư, lý do lùi hạn như prototype (luật 3; đơn tố cáo:
   ADR 0078 #4).

### B1. Gửi báo cáo định kỳ

Theo prototype (`vigov-require/apps/api/app/workers/reports.py`): báo cáo tuần vào thứ đã đặt, báo
cáo tháng vào mùng 1, cùng giờ; mùng 1 trùng thứ thì chỉ báo tháng. Gửi lãnh đạo một thông báo
chuông/Zalo `bao-cao.san-sang`, tiêu đề từ Lời hệ thống `report.notification.week|month`. Không tệp,
không thư.

Ba điểm người dùng chốt khác hoặc làm rõ prototype:

| Điểm | Chốt 09/10 |
|---|---|
| Nơi chạy | Mỗi service sở hữu sổ gửi phần của mình, như bản tin tuần (ADR 0058 §8): petitions (nhiệm vụ quá hạn; phản ánh trễ), documents (văn bản đến quá hạn). Mỗi loại việc một lượt nhận như bản tin tuần, nên lãnh đạo nhận 3 tin (Nhiệm vụ, Phản ánh, Văn bản đến). Không có nơi gộp, không RPC đếm mới |
| Kỳ | Kỳ HIỆN TẠI vừa bắt đầu, như prototype; ranh giới ngày theo giờ Việt Nam (ADR 0053 §3), không theo UTC |
| Nội dung | Nhãn tiếng Việt ("Nhiệm vụ quá hạn: 3"), không mã chỉ số thô. Số không có dữ liệu thì bỏ, không ghi 0 (ADR 0053 §6) — số Giải ngân bỏ vì finance chưa có bên chạy |

### B2. Lịch thuộc identity

`AUTOMATION_JOB_SCHEDULED_REPORTS`. Identity quyết kỳ tuần/tháng của lượt
(`AutomationRun.scheduled_report_period`), cùng lý do ADR 0058 §2 giữ lịch ở identity: hai bên chạy
tự đọc đồng hồ thì có thể quyết hai kỳ khác nhau cho cùng một ngày.

### C. Không thêm việc nền Tính lại số liệu Tổng quan

Prototype có, vì prototype dựng sẵn số liệu. Ở đây Tổng quan đếm trực tiếp mỗi lần mở (ADR 0053 §1,
ADR 0058 §4), nên việc ấy không có gì để tính; một thẻ việc chạy không làm gì là làm giả (ADR 0068 §14).

## Hệ quả

- Dễ: không broker; phía comms/Zalo chỉ thêm loại.
- Khó: petitions và documents mỗi bên một bộ chuyển giống nhau — phần chung nên vào `core/` khi có
  bên thứ ba.
- Hai khuôn cho "nhắc tên": nhiệm vụ qua outbox, Giải ngân sau commit (ADR 0081 #5 giữ nguyên).
- Kỳ hiện tại nghĩa là sáng thứ Hai số "phản ánh trễ trong kỳ" gần như bằng 0; các số quá hạn là tồn
  đọng, không theo kỳ. Đây là hành vi của prototype, người dùng giữ.
- Ngày có broker chỉ thay bộ chuyển.

## ĐIỀU KIỆN DỪNG

1. Gọi comms đồng bộ trong giao dịch của hành vi, hoặc gọi sau commit mà không ghi outbox
2. Khoá chống trùng theo bản ghi thay vì theo hành vi
3. Đưa nội dung phản ánh, lời nhận xét của dân, trích yếu đơn thư, lý do lùi hạn vào tin
4. Bên chạy tự quyết kỳ tuần/tháng từ đồng hồ của mình
5. Một service đếm sổ của service khác để gộp báo cáo
