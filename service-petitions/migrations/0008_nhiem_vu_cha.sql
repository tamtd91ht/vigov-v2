-- 0008 — CÂY NHIỆM VỤ CON: một cột, và ba luật KHÔNG nằm ở đây.
--
-- `0006_nhiem_vu.sql:40-49` cố ý KHÔNG tạo cột này và ghi rõ vì sao: đặc tả đòi việc con
-- (`docs/ui-ux/02-nhiem-vu.md:208`, `:306`) nhưng không trả lời câu nào trong bốn câu mà một cây
-- tự tham chiếu bắt phải quyết. Chủ dự án đã trả lời cả bốn ngày 23/09/2026 — **ADR 0037**.
--
-- VÌ SAO LÀ MỘT TỆP MỚI CHỨ KHÔNG SỬA 0006: `core/migrate` so checksum mỗi lần khởi động, và
-- 0006 đã áp. Sửa nó là dịch vụ không khởi động được.
--
-- ---------------------------------------------------------------------------
-- BỐN CÂU, VÀ CHỖ MỖI CÂU ĐƯỢC THI HÀNH
--
--   1. Sâu mấy tầng          → KHÔNG GIỚI HẠN.   Ở đây: cột nullable, không cột `cap`.
--   2. Con thừa hạn cha?     → KHÔNG, hạn riêng. KHÔNG ở đây — không DEFAULT, không trigger
--                              chép hạn. Tuyến GHI chỉ việc đừng chép, và `han_xu_ly` vốn đã
--                              nullable từ 0006.
--   3. Xoá cha còn con       → TỪ CHỐI.          KHÔNG ở đây — xem khối "BA LUẬT Ở ĐƯỜNG GHI".
--   4. Cha `hoan-thanh`      → MÁY CHỦ CHẶN.     KHÔNG ở đây — cùng lý do.
--
-- Tệp này vì thế chỉ mang ĐÚNG MỘT cột và MỘT ràng buộc. Ba luật còn lại là luật về QUAN HỆ GIỮA
-- NHIỀU DÒNG, và một `CHECK` chỉ nhìn thấy một dòng.
--
-- ---------------------------------------------------------------------------
-- BA LUẬT Ở ĐƯỜNG GHI — ghi ra đây vì đường ghi CHƯA TỒN TẠI, và một luật không có chỗ nào
-- nhắc tới là một luật lượt sau không biết mình đang nợ.
--
--   (a) XOÁ MỀM CHA CÒN CON → TỪ CHỐI, kèm câu nói rõ còn mấy việc con. Không xoá lan (một lần
--       bấm xoá hàng chục bản ghi trên sổ không xoá cứng được), không thả con thành việc gốc
--       (mất ngữ cảnh "việc này sinh ra từ đâu", đúng thứ cây sinh ra để giữ).
--
--   (b) CHA CHUYỂN `hoan-thanh` → chặn khi còn con chưa xong, DUYỆT ĐỆ QUY CẢ CÂY chứ không chỉ
--       một tầng con. Chỉ cảnh báo trên màn thì tỷ lệ hoàn thành đếm cả những cha còn con dở —
--       một con số đẹp hơn thực tế, và nó là con số đi lên lãnh đạo.
--
--   (c) CHU TRÌNH `A → B → C → A` — và đây là luật dễ quên nhất, vì nó không có trong đặc tả.
--       `CHECK` dưới đây chỉ chặn ca TỰ LÀM CHA CỦA MÌNH. Vòng qua hai bản ghi trở lên thì lược
--       đồ bất lực: một `CHECK` không đọc được dòng khác. Một chu trình làm MỌI phép duyệt cây —
--       (b) ở trên, đếm việc con, dựng cây trên màn — chạy vĩnh viễn.
--       Đường ghi phải đi ngược lên cha tìm chính mình trước khi nhận `nhiem_vu_cha_id`.
--
-- ---------------------------------------------------------------------------
-- BẢNG RỖNG Ở MỌI MÔI TRƯỜNG khi tệp này chạy lần đầu, nên đây KHÔNG phải migration trên hồ sơ
-- lưu trữ: không backfill, không cột NOT NULL cần giá trị mặc định, không có dòng nào để làm hỏng.
-- ---------------------------------------------------------------------------

ALTER TABLE nhiem_vu ADD COLUMN IF NOT EXISTS nhiem_vu_cha_id TEXT;

-- KHÔNG KHOÁ NGOẠI, dù cha và con ở CÙNG bảng — và lý do khác với chỗ 0006 bỏ khoá ngoại sang
-- `identity`. Ở đây bảng đã PHÂN MẢNH THEO HASH của `tenant_id`: một khoá ngoại tự tham chiếu
-- trên bảng phân mảnh đòi PostgreSQL bảo đảm cha nằm cùng mảnh, mà `nhiem_vu_cha_id` không tham
-- gia khoá phân mảnh nên nó không bảo đảm được. Cùng-xã do đường ghi giữ, cùng hình dạng
-- `nhiem_vu.nguon_id` ở 0006.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'nhiem_vu_khong_tu_lam_cha') THEN
        ALTER TABLE nhiem_vu ADD CONSTRAINT nhiem_vu_khong_tu_lam_cha
            CHECK (nhiem_vu_cha_id IS DISTINCT FROM id);
    END IF;
END $$;

-- `IS DISTINCT FROM` chứ không `<>`: cột nullable, và `NULL <> NULL` trả NULL — tức `CHECK` đi
-- qua ở đúng ca hay gặp nhất (nhiệm vụ gốc, không cha). Cùng cái bẫy `0007_nguon_von.sql` đã ghi.

-- Tìm con của một cha, và tìm cả cây bằng truy vấn đệ quy. KHÔNG duy nhất, nên mệnh đề
-- `WHERE deleted_at IS NULL` ở đây HỢP LỆ — nó chỉ quyết định đọc ít dòng hơn, không quyết định
-- một mã đã cấp có được cấp lại hay không (luật 7 bất biến 3, `tools/check_khoa_duy_nhat.py:175`).
CREATE INDEX IF NOT EXISTS nhiem_vu_cha
    ON nhiem_vu (tenant_id, nhiem_vu_cha_id)
    WHERE deleted_at IS NULL AND nhiem_vu_cha_id IS NOT NULL;
