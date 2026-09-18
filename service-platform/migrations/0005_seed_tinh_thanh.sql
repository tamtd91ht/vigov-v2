-- platform — nạp 34 đơn vị hành chính cấp tỉnh sau sắp xếp 2025.
--
-- VÌ SAO LÀ MỘT TỆP RIÊNG chứ không sửa 0004: `core/migrate` đối chiếu checksum của mọi tệp đã
-- áp lúc khởi động. Một migration đã áp là bất biến. 0004 dựng bảng và cố ý để RỖNG; tệp này
-- đổ dữ liệu vào, và tách ra như vậy còn để sửa một cái tên viết sai là thêm một migration,
-- không phải đụng vào tệp đã chạy ở nơi khác.
--
-- ⚠ DỮ LIỆU NÀY CHƯA ĐƯỢC ĐỐI CHIẾU VỚI VĂN BẢN GỐC. ĐỌC HẾT ĐOẠN NÀY TRƯỚC KHI TIN NÓ.
--
-- Nguồn đúng là Nghị quyết 202/2025/QH15 ngày 12/06/2025 của Quốc hội về sắp xếp đơn vị hành
-- chính cấp tỉnh, hiệu lực từ 01/7/2025. Danh sách dưới đây dựng từ trí nhớ của agent, không
-- phải từ bản văn: đã thử ba nguồn công bố (`vanban.chinhphu.vn`, `xaydungchinhsach.chinhphu.vn`,
-- cổng thông tin tỉnh) và cả ba render bằng JavaScript nên không đọc trực tiếp được.
--
-- CƠ CẤU thì chắc: **34 đơn vị = 6 thành phố trực thuộc trung ương + 28 tỉnh**, và 11 đơn vị
-- KHÔNG bị sắp xếp (Cao Bằng · Điện Biên · Hà Tĩnh · Lai Châu · Lạng Sơn · Nghệ An · Quảng Ninh
-- · Sơn La · Thanh Hóa · Hà Nội · Huế).
--
-- CÁI CẦN NGƯỜI ĐỐI CHIẾU là 34 cái TÊN. Cột `ten` in thẳng ra màn hình công dân, nên dạng
-- viết LÀ nội dung, không phải nhãn nội bộ. Một cơ quan nhà nước hiển thị sai tên một tỉnh là
-- sự cố có người phải trả lời.
--
-- Cách sửa khi đối chiếu xong: thêm một migration mới với `UPDATE tinh_thanh SET ten = ...
-- WHERE id = ...`. **Giữ nguyên `id`** — nó là ULID vô nghĩa, và đổi tên hiển thị không được
-- phép đổi định danh (luật 1, bất biến 2). Đó chính là lý do khoá không mang nghĩa.
--
-- VỀ DẠNG VIẾT ĐÃ CHỌN, và đây là quyết định có thể bị bác:
--
--   * Dùng tên TRẦN — `Hà Nội`, `Đà Nẵng`, `Lai Châu` — không mang tiền tố `Tỉnh`/`Thành phố`.
--     Đây là nhãn trong một danh sách chọn trên điện thoại: người dân gõ "Tân Phú", không gõ
--     "xã Tân Phú"; thêm tiền tố vào cả 34 dòng làm cột dài ra mà không phân biệt thêm điều gì.
--   * MỘT ngoại lệ: `Thành phố Hồ Chí Minh` giữ nguyên tiền tố, vì `Hồ Chí Minh` trần là tên
--     một con người, không phải tên một địa phương. Đây là ngoại lệ của tiếng Việt chứ không
--     phải của hệ thống, nên nó nằm trong dữ liệu chứ không thành một cột `loai`.
--
-- VỀ `thu_tu`: xếp theo bảng chữ cái tiếng Việt, tính sẵn ở đây và ghi thành số. Sắp xếp theo
-- `ten` lúc đọc thì thứ tự phụ thuộc collation của từng bản cài PostgreSQL — cùng một danh sách
-- ra hai thứ tự khác nhau ở hai môi trường, và không ai coi đó là lỗi cho tới khi một công dân
-- nói "danh sách của tôi khác". Bước 10 để chèn được đơn vị mới vào giữa mà không đánh số lại.
--
-- NĂM CÂU HỎI CỦA MỘT MIGRATION:
--
--   1. BAO NHIÊU DÒNG MỖI XÃ: không dòng nào. Bảng này không có cột xã; 34 dòng cho cả nền tảng.
--   2. DỪNG GIỮA CHỪNG THÌ SAO: không thể. Một tệp, một giao dịch, dòng tiến độ nằm trong đó.
--   3. LÙI RA SAO: xem REVERSAL cuối tệp.
--   4. ĐƯỜNG ĐỌC NÀO ĐỔI NGHĨA KHI ÁP DỞ: không đường nào. `tenant.tinh_thanh` không bị đụng,
--      và chưa có mã nào đọc bảng `tinh_thanh`.
--   5. LƯU TRỮ: không xoá gì, không bỏ cột, không đổi kiểu.
--
-- ÁP LẠI ĐƯỢC NHIỀU LẦN: `ON CONFLICT DO NOTHING` trên khoá chính. Chạy lại không nhân đôi,
-- và cũng KHÔNG ghi đè tên — một `UPDATE` ở đây sẽ âm thầm nuốt mất bản sửa tên mà ai đó đã
-- làm bằng migration sau.

INSERT INTO tinh_thanh (id, ten, thu_tu) VALUES
    ('4E2Z38KRX62N3TA5E99BPZTASH', 'An Giang', 10),
    ('0M69BT6RD4HKV7TTCRKZ6N1RBS', 'Bắc Ninh', 20),
    ('5JMJSQNG8FDRBJSXXZMGWZK02Z', 'Cà Mau', 30),
    ('465D89MPGCHV3ZDYA0W4V3KA9Y', 'Cao Bằng', 40),
    ('4PMB643Z0RYWCCYV48A8WEYRQ7', 'Cần Thơ', 50),
    ('4WQ7A6C7CT838WS4AJGV7Z51Y5', 'Điện Biên', 60),
    ('66QW36RCJ7GVW79W8GJRYH3ZNR', 'Đà Nẵng', 70),
    ('45BRPM92S6HHHSM53RD7EW9RHX', 'Đắk Lắk', 80),
    ('4CSWH7R5654APQCQB2FM1CH6G4', 'Đồng Nai', 90),
    ('42T6NYNV8CX5VABT3JTSHQ6G3M', 'Đồng Tháp', 100),
    ('3T971EDQ8J1A91Y470F31MFV5W', 'Gia Lai', 110),
    ('5WK9RVKKQYFCJXTSX8A1YBVK0P', 'Hà Nội', 120),
    ('2VES8RX47C1YKHHRP3SQGA44AT', 'Hà Tĩnh', 130),
    ('7QKEMDFQRF3NCHMZJXD790Y8T0', 'Hải Phòng', 140),
    ('2WFZTMM9ZS68EJ63Q4SW3AB60G', 'Huế', 150),
    ('19TVJ093MHWXAYGVPYJGZY9KK9', 'Hưng Yên', 160),
    ('2RJFSVRQ10KCSNWHN30ZQH7E1R', 'Khánh Hòa', 170),
    ('1V8M685G8CW51SN95SJAP113MA', 'Lai Châu', 180),
    ('7GYJDFQ8417EP4G2ZRZBNX3DTB', 'Lâm Đồng', 190),
    ('0RRTE4K63JHQRC9RQK806WJQ8C', 'Lạng Sơn', 200),
    ('38G8XMBZ6KWXWYYPW8JY0Y13MF', 'Lào Cai', 210),
    ('46Q5AJ70364XZT15Z7JG0H0CPQ', 'Nghệ An', 220),
    ('5FQ187C59Q1YG7FY4FASAVXXNJ', 'Ninh Bình', 230),
    ('0686683P0HM4JZDNHCF266620K', 'Phú Thọ', 240),
    ('43FDKRC6SRKATEKXT7GXRE5C1S', 'Quảng Ngãi', 250),
    ('40F97VGRBRB3C8SX0ZZXDXZCW5', 'Quảng Ninh', 260),
    ('6GTKXJ34048W7G9XNGX18EGD3H', 'Quảng Trị', 270),
    ('2HJ7JPXG0T525Z01VQC4Y9AY1H', 'Sơn La', 280),
    ('7T4V5YNWG8GHVM7DQSV9RNE7VM', 'Tây Ninh', 290),
    ('06R46HKHV3BXBPYN8MPQT0V3A7', 'Thái Nguyên', 300),
    ('26V2DE35DWW4TGQCEDFJW8C0WM', 'Thanh Hóa', 310),
    ('58TDS6ME4FP9Z1515P07M605H4', 'Thành phố Hồ Chí Minh', 320),
    ('3BRHGM4590SNBYPEB0337ZPG94', 'Tuyên Quang', 330),
    ('7D595B7GR4PV30ED01QGR1XF89', 'Vĩnh Long', 340)
ON CONFLICT (id) DO NOTHING;

-- Chốt chặn: đúng 34 dòng đang hoạt động. Nếu ai đó thêm hoặc bớt một dòng mà không đọc phần
-- đầu tệp này, migration ĐỔ ngay tại đây thay vì để một danh mục sai lặng lẽ đi ra màn hình
-- công dân. Con số 34 là cơ cấu đã chốt của Nghị quyết 202/2025/QH15, không phải một con số đếm
-- được từ danh sách bên trên — nên nó kiểm được chính danh sách ấy.
DO $$
DECLARE n INT;
BEGIN
    SELECT count(*) INTO n FROM tinh_thanh WHERE dang_hoat_dong;
    IF n <> 34 THEN
        RAISE EXCEPTION 'tinh_thanh: co % don vi dang hoat dong, phai la 34 (Nghi quyet 202/2025/QH15)', n;
    END IF;
END $$;

-- REVERSAL
--
-- TRƯỚC KHI CÓ DỮ LIỆU TRỎ VÀO:
--     DELETE khỏi tinh_thanh 34 dòng theo đúng danh sách id ở trên, rồi gỡ dòng tiến độ
--     ten = '0005_seed_tinh_thanh.sql' khỏi schema_migration.
--     Mất 0 dữ liệu: chưa khoá ngoại nào trỏ vào bảng này.
--
-- SAU KHI `tenant.tinh_thanh_id` tồn tại và đã backfill:
--     KHÔNG có đường lùi đơn giản. Xoá một dòng danh mục là cắt mối nối của mọi xã trỏ vào nó,
--     và một xã không còn tỉnh là một xã biến mất khỏi màn hình chọn của công dân. Khi đó đây
--     là luật 7 điều kiện dừng #1: cần quyết định tường minh của người dùng và một bản sao lưu
--     đã kiểm.
--
-- Câu lệnh gỡ dòng tiến độ viết bằng văn xuôi, KHÔNG phải dòng chạy được — theo tiền lệ của
-- 0003: một dòng chạy được là một dòng sẽ có người chạy.
