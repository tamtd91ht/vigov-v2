-- 0007 — hai khoá quyền cho phân hệ phản ánh: `feedback.classify` và `feedback.unmask`.
--
-- NGƯỜI DÙNG CHỐT 2026-09-20. Hai hành vi này đang KHÔNG có khoá nào canh, và luật 5 nói rõ
-- hậu quả: guard toàn cục chỉ chặn người CHƯA ĐĂNG NHẬP, nên một tuyến không khai quyền là
-- tuyến MỌI vai trò cán bộ gọi được — kể cả vai trò không liên quan gì tới phản ánh. Việc ấy
-- hỏng trong im lặng: không test nào đỏ, không dòng log nào.
--
-- VÌ SAO KHÔNG DÙNG LẠI KHOÁ ĐANG CÓ. Luật 5 bất biến 3b: các quyền này KHÔNG phải tích Đề-các
-- để suy ra, và `task.approve` cố ý không phải `task.extend`. Cụ thể ở đây:
--
--   `feedback.assign`     = giao phiếu cho ai xử lý.
--   `feedback.classify`   = XÁC ĐỊNH LĨNH VỰC, và vì lĩnh vực quyết số giờ SLA, hành vi này
--                           ẤN ĐỊNH HẠN XỬ LÝ — tức phát ra một cam kết của chính quyền với một
--                           người dân đang cầm mã tra cứu (ADR 0028 quyết định E). Người được
--                           giao việc không đương nhiên là người được hứa thay cơ quan.
--
--   `feedback.restricted` = xem được phiếu thuộc lĩnh vực `can-bo` (tố cáo tác phong cán bộ).
--   `feedback.unmask`  = xem được HỌ TÊN và SỐ ĐIỆN THOẠI người gửi, ở MỌI lĩnh vực.
--
-- Hai nghĩa ấy khác nhau ở chiều nào cũng được: một người xác minh tố cáo cán bộ chưa chắc
-- cần số điện thoại của mọi người phản ánh rác thải, và ngược lại. Gộp vào một khoá là cấp
-- quyền ngoài ý định cho một trong hai nhóm, và không ai thấy vì màn Phân quyền chỉ hiện MỘT ô.
--
-- `feedback.unmask` MỞ ĐÚNG MỘT THỨ ĐANG HỎNG TRONG VẬN HÀNH: tuyến đọc phiếu hôm nay che
-- cả họ tên lẫn số điện thoại (luật 3 bất biến 3 — che trừ khi có quyền xem đầy đủ, mà khoá ấy
-- chưa tồn tại), tức CÁN BỘ KHÔNG GỌI LẠI ĐƯỢC CHO NGƯỜI PHẢN ÁNH. Đó là cái giá thật của
-- nguyên tắc hỏng-thì-đóng, và đây là chỗ trả nó.
--
-- MỖI LẦN ĐỌC ĐẦY ĐỦ PHẢI GHI VẾT — luật 6 bất biến 7. Lược đồ không cưỡng chế được điều đó;
-- nó thuộc về tầng use case, và là điều kiện để khoá này không biến thành một cửa hậu im lặng
-- vào dữ liệu cá nhân của cả xã.
--
-- VÌ SAO `unmask` CHỨ KHÔNG `view_full`. Đã đếm: KHÔNG khoá nào trong 33 khoá của 0001 có
-- gạch dưới ở vế sau — tất cả là `nhóm.mộttừ` (`task.extend`, `report.export`, `admin.audit`).
-- Luật 5 bất biến 3b nói khoá là CHÍNH CHUỖI màn Phân quyền hiện ra và `quyen` lưu, nên một
-- chuỗi lệch quy ước không sửa lại được sau khi một xã đã gán nó cho một vai trò.
-- `unmask` còn đúng việc hơn: nó GỠ CHE, đúng cặp với MaskPhone/MaskCccd của luật 3, chứ
-- không phải một quyền xem chung chung.
--
-- MIGRATION RIÊNG, KHÔNG SỬA 0001. ADR 0013: migration chạy lúc khởi động và đã chạy thì không
-- sửa lại. Sửa 0001 thì môi trường nào đã chạy nó sẽ KHÔNG BAO GIỜ nhận hai khoá này, và sự
-- lệch ấy chỉ lộ ra khi một cán bộ bấm nút và nhận 403 không giải thích được.
--
-- `thu_tu` 34 VÀ 35, KHÔNG PHẢI 23 VÀ 24. Hai số ấy đã thuộc `petition.create` và
-- `petition.read` trong 0001; cột này không có ràng buộc duy nhất, nên trùng số không báo lỗi
-- gì — nó chỉ làm thứ tự trên màn Phân quyền phụ thuộc vào thứ tự PostgreSQL trả dòng, tức
-- đổi giữa hai lần tải mà không ai biết vì sao.
--
-- KHÔNG GÁN KHOÁ CHO VAI TRÒ NÀO. Khoá tồn tại để quản trị xã tick trên màn Phân quyền; tự gán
-- là tự quyết ai trong một cơ quan nhà nước được hứa thay cơ quan ấy.
--
-- CÒN THIẾU, ĐÃ ĐO, KHÔNG ĐOÁN: docs/ui-ux/14-cau-hinh.md:104 ghi 43 quyền / 11 nhóm; 0001 nạp
-- 33 khoá / 10 nhóm; hai khoá này thành 35. MƯỜI KHOÁ VÀ TRỌN MỘT NHÓM vẫn thiếu, và người
-- dùng cố ý chỉ chốt hai khoá cần ngay. Đừng bịa tám khoá còn lại cho đủ số.
-- ---------------------------------------------------------------------------

INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('feedback.classify',  'PHẢN ÁNH', 'Phân loại phản ánh, ấn định hạn xử lý', 34),
    ('feedback.unmask', 'PHẢN ÁNH', 'Xem đầy đủ họ tên và số điện thoại người gửi', 35)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;
