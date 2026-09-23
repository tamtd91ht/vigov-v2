-- 0009 — SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO: ba danh sách văn bản treo trên một nhiệm vụ.
--
-- `docs/ui-ux/02-nhiem-vu.md` §5.4 vẽ khối này trong ngăn chi tiết, §7.2 đặt nó trên biểu mẫu
-- "Giao việc mới", và §9 (`:314-323`) khai đúng bảng này với sáu cột. `0006_nhiem_vu.sql:50-52`
-- CỐ Ý không tạo nó và ghi rõ là để lượt sau — đây là lượt ấy.
--
-- VÌ SAO LÀ MỘT TỆP MỚI CHỨ KHÔNG SỬA 0006 HAY 0008: `core/migrate` so checksum mỗi lần khởi
-- động, và cả hai đã áp. Sửa một tệp đã áp thì hoặc dịch vụ không khởi động được
-- (ErrChecksumLech), hoặc hai cơ sở dữ liệu cùng khai một phiên bản lược đồ mà giữ hai lược đồ
-- khác nhau.
--
-- ENTITY OWNERSHIP khai bằng `-- @entity` / `-- @scope` ngay trên `CREATE TABLE` (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- ⚠ CÁI TÊN GỢI Ý MỘT LIÊN KẾT GIỮA HAI DỊCH VỤ, VÀ NÓ KHÔNG PHẢI — đọc đoạn này trước khi
-- "hoàn thiện" bảng bằng một `van_ban_id`.
--
-- KHÔNG CÓ CỘT NÀO TRỎ SANG `service-documents`, và đó không phải thiếu sót. §7.2 tả ba ô nhập
-- là "danh sách động textarea + nút `+ Thêm văn bản`", placeholder là VĂN XUÔI
-- (`Ví dụ: Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo…`): cán bộ GÕ TAY một chuỗi
-- tham chiếu văn bản. Văn bản được nhắc tới phần lớn là văn bản của CẤP TRÊN — thông báo của
-- Thành uỷ, công văn của Ban Tổ chức — thứ không bao giờ vào sổ văn bản đến của xã, nên không có
-- hàng nào bên `van_ban_den` để trỏ tới.
--
-- §9 cũng khai đúng sáu cột ấy và không khai cột liên kết nào. Vậy bảng này thuộc TRỌN
-- `service-petitions`, lưu văn bản tự do, và không đọc dữ liệu của dịch vụ khác — luật 2 không
-- có điều kiện dừng nào ở đây. Ngày nào khối này cần trỏ thật vào sổ văn bản thì đó là một
-- QUYẾT ĐỊNH của khách (một cột tuỳ chọn thêm sau, trên bảng rỗng), không phải một suy diễn từ
-- cái tên.
--
-- ---------------------------------------------------------------------------
-- NĂM CÂU HỎI CỦA MỘT MIGRATION, trả lời trước khi vào SQL.
--
--   1. BAO NHIÊU DÒNG MỖI XÃ: không dòng nào. Bảng mới, tệp này không ghi một hàng nào. Không
--      gieo mẫu, cùng lý do 0003/0006/0007 đã ghi: một hàng ở đây mang `tenant_id`, nên gieo là
--      gieo cho MỘT XÃ CÓ TÊN, và bước gieo hàng đầu tiên của một xã — onboarding — không tồn
--      tại trong kho này.
--   2. NẾU DỪNG GIỮA CHỪNG: không áp nửa vời được. Một tệp, một giao dịch, hàng tiến độ ghi bên
--      trong nó. Mọi câu đều `IF NOT EXISTS` hoặc `CREATE OR REPLACE`, nên chạy lại không tốn gì.
--   3. HOÀN NGUYÊN RA SAO: xem khối REVERSAL ở cuối.
--   4. ĐƯỜNG ĐỌC NÀO ĐỔI NGHĨA KHI MỚI ÁP MỘT NỬA: không đường nào. Tệp này chỉ THÊM một bảng,
--      một hàm trigger và một trigger. Không cột, không ràng buộc, không chỉ mục nào của bảng cũ
--      bị đụng tới. `nhiem_vu` chỉ nhận thêm một khoá ngoại ĐI VÀO, thứ ràng buộc cái được ghi
--      vào bảng MỚI và không đổi gì cách `nhiem_vu` được đọc.
--   5. LƯU TRỮ: xem khối ngay dưới — đây là quyết định lược đồ đắt nhất của tệp này.
--
-- ---------------------------------------------------------------------------
-- MỘT DÒNG VĂN BẢN LÀ GÌ: LÀ GIÁ TRỊ MỘT TRƯỜNG CỦA NHIỆM VỤ, KHÔNG PHẢI MỘT HỒ SƠ RIÊNG.
--
-- Câu này quyết định lược đồ, nên nó được trả lời bằng bằng chứng chứ không bằng cảm giác. Bốn
-- điểm, tất cả từ chính đặc tả:
--
--   * §5.4 vẽ ba danh sách này là các TRƯỜNG trong khối "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO", nằm cạnh
--     `Tóm tắt kết quả`, `Ghi chú` và hai ô tick — toàn cột thường của `nhiem_vu`. Cả khối có
--     ĐÚNG MỘT nút `✎ Sửa`, tức sửa cả khối là MỘT thao tác.
--   * §7.2 liệt chúng cùng hàng với `Ghi chú` trong danh sách TRƯỜNG của biểu mẫu tạo việc.
--   * §9 khai `id · nhiem_vu_id · nhom · so_ky_hieu · ngay_van_ban · trich_yeu · thu_tu` —
--     KHÔNG có `nguoi_tao`, KHÔNG có `thoi_diem`. Đối chiếu `nhat_ky_nhiem_vu` (§9 cùng trang)
--     có cả `nguoi_id` lẫn `thoi_diem`, vì nó LÀ một bản ghi riêng. Một hàng không biết ai tạo
--     và tạo lúc nào thì không phải hồ sơ hành chính; nó là giá trị một trường của hồ sơ nó treo
--     vào.
--   * Nó thành BẢNG chỉ vì là nhóm lặp (1..n), không vì có vòng đời riêng.
--
-- HAI HỆ QUẢ, VÀ CẢ HAI ĐỀU LÀ CHỦ Ý:
--
--   (a) XOÁ MỀM, KHÔNG BAO GIỜ XOÁ CỨNG. Nút `✕` của §7.2 gỡ một dòng khỏi danh sách, và một
--       dòng đã gỡ vẫn là chữ đã từng nằm trên một hồ sơ hành chính. Một câu xoá cứng ở đây là
--       luật 7 cấm #1, và trigger dưới đây từ chối nó chứ không phải một quy ước.
--
--   (b) KHÔNG CÓ `delete_reason`, VÀ ĐÓ LÀ KHÁC BIỆT DUY NHẤT SO VỚI MỌI BẢNG KHÁC CỦA DỊCH VỤ.
--       Ba cột của luật 7 bất biến 1 tồn tại để khi một HỒ SƠ rời sổ thì nói được ai gỡ và vì
--       sao. Gỡ một dòng ở đây không phải gỡ hồ sơ khỏi sổ — nó là một lần SỬA nhiệm vụ, y hệt
--       sửa `ghi_chu` hay `tom_tat_ket_qua`, và không lần sửa nào trong sổ này ghi lý do. Vết
--       kiểm toán của chính lần sửa ấy (ai · làm gì · lúc nào · từ IP nào · ở xã nào) là chỗ
--       "vì sao nhiệm vụ đổi" đang nằm, và nó được ghi trong CÙNG giao dịch (luật 6 bất biến 3).
--
--       CÁI GIÁ, NÓI THẲNG RA: nếu sau này khách muốn mỗi lần gỡ một dòng phải nêu lý do thì đó
--       là một cột thêm vào bảng rỗng, không phải một cột đã có mà không ai điền nổi. Lựa chọn
--       ngược lại — giữ cột rồi để máy chủ tự bịa một chuỗi vì §7.2 không có ô nhập lý do — tệ
--       hơn hẳn: một cột sinh ra để chứa lý do thật mà chứa chữ máy tự sinh là một cột không ai
--       đọc được nữa, và nó trông y như đang có lý do.
--
--       `deleted_by` VẪN CÓ. "Ai gỡ" trả lời được mà không phải bịa gì, và nó là MÃ NGHIỆP VỤ
--       CÁN BỘ (`CB-00123`) — luật 6 bất biến 8, thứ người xử lý khiếu nại đọc được sau nhiều
--       năm, trong khi một ULID thì không gọi tên ai.
--
-- ---------------------------------------------------------------------------
-- `thu_tu` — SỐ ĐÃ CẤP, KHÔNG PHẢI VỊ TRÍ TRONG MẢNG. Đây là bẫy dễ rơi nhất của bảng này.
--
-- Phản xạ khi cài "danh sách động" là: máy khách gửi cả mảng theo thứ tự hiển thị, máy chủ đánh
-- lại `thu_tu` bằng chỉ số mảng. Nó CHẠY ĐÚNG với mắt thường và sai theo hai đường cùng lúc:
--
--   * Nó đánh mất một sự thật đã ghi. `thu_tu` nói cán bộ đặt dòng ấy ở đâu; đánh lại theo mảng
--     biến nó thành hàm của lần gửi GẦN NHẤT. Một máy khách gửi mảng theo thứ tự khác là một lần
--     SẮP XẾP LẠI danh sách trên hồ sơ hành chính, âm thầm, không có gì đỏ.
--   * Nó đụng khoá duy nhất dưới đây, và đụng đúng ở ca hay gặp nhất. `UNIQUE (tenant_id,
--     nhiem_vu_id, nhom, thu_tu)` TÍNH CẢ DÒNG ĐÃ XOÁ MỀM (xem lý do tại chỗ khai). Gỡ dòng ②
--     rồi đánh lại 1,2 cho hai dòng còn lại là cấp lại số ② mà dòng đã xoá vẫn đang giữ.
--
-- ĐƯỜNG GHI VÌ THẾ: dòng đã có GIỮ NGUYÊN `thu_tu` đã lưu; dòng mới nhận `max(thu_tu) + 1`
-- TRONG CÙNG (nhiệm vụ, nhóm), đếm cả dòng đã xoá mềm. Đúng cách `ket_luan_hop` (0007) đánh số
-- kết luận và đúng câu §7.2 nói về nút `+ Thêm văn bản`: THÊM VÀO CUỐI, không đánh số lại.
-- Khoảng trống sau một lần gỡ là kết quả ĐÚNG.
--
-- ---------------------------------------------------------------------------
-- BA CỘT NỘI DUNG, VÀ MỘT CHỖ ĐẶC TẢ TỰ MÂU THUẪN — nói ra chứ không lặng lẽ chọn.
--
-- §9 khai ba cột `so_ky_hieu` · `ngay_van_ban` · `trich_yeu`, và §5.4 vẽ ra có cấu trúc
-- (`1742-CV/BTCTU · 9/6/2026` rồi `Công văn của Ban Tổ chức Thành uỷ`). Nhưng §7.2 chỉ cho MỘT
-- ô textarea với placeholder là một câu văn xuôi — biểu mẫu không tách ba ô ấy ra.
--
-- LƯỢC ĐỒ THEO §9 (ba cột), VÀ HAI CỘT CẤU TRÚC LÀ NULLABLE. Chuỗi cán bộ gõ vào textarea đi
-- thẳng vào `trich_yeu`; `so_ky_hieu` và `ngay_van_ban` để trống cho tới ngày biểu mẫu có ô cho
-- chúng, và ngày ấy không cần migration nào. Máy chủ TUYỆT ĐỐI KHÔNG tách câu văn xuôi ra thành
-- số hiệu và ngày: đó là đoán, và đoán sai trên một tham chiếu văn bản hành chính là in ra một
-- số hiệu không có thật. Hợp đồng REST nhận cả ba trường, nên máy khách là bên quyết định điền
-- gì.
--
-- `ngay_van_ban` LÀ `DATE`, KHÔNG PHẢI `TIMESTAMPTZ`, và đây là chiều NGƯỢC với lập luận của
-- 0006 về hai cột hạn. Hạn xử lý là một THỜI ĐIỂM đếm bằng giờ làm việc (ADR 0007), nên nó phải
-- là instant. Ngày văn bản là ngày IN TRÊN GIẤY — `9/6/2026`, không giờ, không múi — nên lưu
-- thành instant là buộc phải trả lời "nửa đêm theo múi nào" cho một giá trị vốn không có giờ.
-- Cùng kiểu và cùng tên cột `service-documents` đã dùng (`0004_so_van_ban.sql:324`), và cùng lập
-- luận `bien_ban_hop.ngay_hop` (0007) đã ghi.
--
-- ---------------------------------------------------------------------------
-- TỆP NÀY CỐ Ý KHÔNG TẠO GÌ, để chỗ trống không bị đọc là việc làm dở:
--
--   * KHÔNG CÓ BẢNG DANH MỤC CHO `nhom`. Ba giá trị là ba Ô CỐ ĐỊNH TRÊN MÀN HÌNH (§5.4, §7.2),
--     mỗi ô có nhãn riêng và placeholder riêng viết thẳng trong đặc tả. Xã không thêm được nhóm
--     thứ tư mà màn hình không có chỗ vẽ, nên danh sách là ĐÓNG và thuộc về một `CHECK` — cùng
--     hình dạng `nhiem_vu_trang_thai_hop_le` (0006) và cùng lý do ADR 0035 §C đưa ra.
--
--   * KHÔNG CÓ `nguoi_tao_ma` / `tao_luc` NGHIỆP VỤ. §9 không khai, và lý do sâu hơn là ở khối
--     "một dòng văn bản là gì" bên trên: hàng này không phải bản ghi riêng. `tao_luc` bên dưới
--     là mốc VÒNG ĐỜI HÀNG (mọi bảng trong kho đều có), không phải một sự thật nghiệp vụ nào
--     màn hình đọc.
--
--   * KHÔNG CÓ TRIGGER BẤT BIẾN TRÊN `trich_yeu` HAY `nhom`. Cả khối có nút `✎ Sửa`, nên sửa
--     chữ một dòng là việc đặc tả cho phép. Cái đường ghi KHÔNG cho là đổi `nhom` và `thu_tu`
--     của một dòng đã có — chuyển một dòng sang nhóm khác là một thao tác không đặc tả nào tả,
--     và §7.2 không có điều khiển kéo thả nào để đổi vị trí.
--
-- ---------------------------------------------------------------------------
-- PostgreSQL 13 là sàn, kiểm tường minh để lỗi đọc được.
--
-- BEFORE ... FOR EACH ROW trigger trên bảng PHÂN MẢNH chỉ được phép từ PostgreSQL 13. Trên 11 và
-- 12, câu CREATE TRIGGER dưới đây hỏng với một thông điệp đọc như lỗi cú pháp, và nó mời người
-- ta "sửa" bằng cách dời trigger xuống từng mảnh — nơi một mảnh thêm sau này đến mà không được
-- canh, lặng lẽ.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'nhiem_vu_van_ban needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the trigger below is what stops a line of an '
            'administrative record being destroyed with one statement.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- nhiem_vu_van_ban_cam_xoa_cung — chỉ từ chối xoá cứng, không đọc cột nào.
--
-- MỘT HÀM RIÊNG CHỨ KHÔNG DÙNG LẠI `ho_so_luu_tru_cam_xoa_cung` (0007), dù hai hàm cùng làm một
-- việc. Hàm bên ấy mang HINT nói về biên bản họp và về ba cột xoá mềm trong đó có
-- `delete_reason` — bảng này cố ý không có cột ấy (xem khối lý do bên trên), nên một thông điệp
-- bảo người đọc "đặt delete_reason" là một chỉ dẫn sai gửi đúng lúc họ đang bí. Cùng lập luận
-- 0007 đưa ra khi nó không dùng lại `nhiem_vu_bat_bien`.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nhiem_vu_van_ban_cam_xoa_cung() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'archival table %: hard delete refused', TG_TABLE_NAME
        USING HINT = 'A referenced-document line is part of a task, and a task is an '
                     'administrative record (rule 7, invariant 1). Remove it by setting '
                     'deleted_at and deleted_by; the reason for the change lives on the audit '
                     'entry of the edit that removed it.';
END $$;

-- ---------------------------------------------------------------------------
-- @entity: TaskDocument
-- @scope:  tenant
--
-- nhiem_vu_van_ban — MỘT dòng trong một trong ba danh sách văn bản của §5.4 / §7.2.
--
-- ⚠ NÓ KHÔNG GIỮ DỮ LIỆU CÁ NHÂN CÔNG DÂN, và không được bắt đầu giữ. Nhưng `trich_yeu` là chữ
-- của một VĂN BẢN HÀNH CHÍNH mà cán bộ gõ vào, và văn bản hành chính nhắc tới hồ sơ công dân là
-- chuyện thường ("về việc giải quyết đơn của hộ ông…"). Vậy nó là văn bản nghiệp vụ nhạy cảm:
-- KHÔNG vào dòng log, KHÔNG vào thông điệp lỗi trả về máy khách, KHÔNG vào tên tệp (luật 3).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nhiem_vu_van_ban (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    -- Nhiệm vụ mà dòng này là một phần của nó. CÓ KHOÁ NGOẠI THẬT, và chỗ này là nơi ba lập luận
    -- khác nhau trong kho gặp nhau — nên nói rõ vì sao lập luận thứ ba mới đúng ở đây:
    --
    --   `nhat_ky_nhiem_vu.nhiem_vu_id`   KHÔNG khoá ngoại (0006). Lý do ghi ở đó: một mục lịch
    --                                    sử phải SỐNG SÓT qua một lần định hình lại sổ. Đúng cho
    --                                    một bản ghi riêng, SAI ở đây — dòng này không có đời
    --                                    sống ngoài nhiệm vụ nó treo vào, và một dòng còn lại
    --                                    sau khi nhiệm vụ biến mất là rác không màn hình nào vẽ.
    --   `nhiem_vu.nhiem_vu_cha_id`       KHÔNG khoá ngoại (0008). Lý do ghi ở đó là TỰ THAM
    --                                    CHIẾU trên bảng phân mảnh: cha không tham gia khoá phân
    --                                    mảnh nên PostgreSQL không bảo đảm được cùng mảnh. Không
    --                                    áp vào đây — đây là hai bảng khác nhau.
    --   `ket_luan_hop.bien_ban_id`       CÓ khoá ngoại (0007), hợp thành với `tenant_id`, cả hai
    --                                    phía phân mảnh hash theo `tenant_id`, và cặp được trỏ
    --                                    tới đúng là khoá chính của bảng cha. ĐÓ LÀ HÌNH DẠNG
    --                                    NÀY, và `noi_dung_mini_app` -> `danh_muc_mini_app`
    --                                    (service-comms 0006) là bản thứ hai của cùng hình dạng.
    --
    -- Hợp thành với `tenant_id` nên một dòng không treo được vào nhiệm vụ của xã khác, kể cả khi
    -- hai id có ngày trùng nhau.
    nhiem_vu_id   TEXT        NOT NULL,

    -- Ba ô cố định của §5.4 / §7.2. DANH SÁCH ĐÓNG — xem khối "tệp này cố ý không tạo gì".
    --
    --   cap-tren-giao     "Văn bản cấp trên giao"
    --   chi-dao-dang-uy   "Văn bản chỉ đạo của Đảng uỷ"
    --   san-pham-dau-ra   "Văn bản sản phẩm đầu ra"
    --
    -- Mã là tiếng Việt không dấu, kebab-case: GIÁ TRỊ enum không bao giờ dịch sang tiếng Anh
    -- (ADR 0011). NHÃN cố ý không chép vào đây — nhãn ở trên màn hình, và bản sao thứ hai của
    -- một chuỗi hiển thị là bản sao sẽ lệch.
    nhom          TEXT        NOT NULL,

    -- `1742-CV/BTCTU` — số và ký hiệu văn bản. NULLABLE: §7.2 chỉ có một ô textarea và không
    -- tách trường này ra. Xem khối "ba cột nội dung" bên trên.
    so_ky_hieu    TEXT,

    -- Ngày ghi trên văn bản. `DATE` chứ không `TIMESTAMPTZ` — xem khối "ba cột nội dung".
    -- NULLABLE, cùng lý do `so_ky_hieu`.
    ngay_van_ban  DATE,

    -- Chữ của dòng: trích yếu văn bản, hoặc — hôm nay — cả chuỗi cán bộ gõ vào textarea của
    -- §7.2. BẮT BUỘC: một dòng rỗng trong danh sách động là một dòng không ai đọc được và không
    -- ai thao tác được, và nút `✕` mới là cách gỡ nó.
    trich_yeu     TEXT        NOT NULL,

    -- Vị trí trong NHÓM của nó. Số ĐÃ CẤP, không phải chỉ số mảng — xem khối `thu_tu` bên trên.
    thu_tu        INT         NOT NULL,

    -- HAI CỘT XOÁ MỀM, KHÔNG PHẢI BA. Xem khối "một dòng văn bản là gì", mục (b): lý do của lần
    -- sửa nằm trên vết kiểm toán của chính lần sửa ấy, không phải trên từng dòng mà nó chạm tới.
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- HỢP THÀNH VỚI tenant_id (luật 1 bất biến 6) VÀ TÍNH CẢ DÒNG ĐÃ XOÁ MỀM.
    --
    -- KHÔNG có `WHERE deleted_at IS NULL`, và chỗ thiếu ấy mới là điểm chính: khoá duy nhất TỪNG
    -- PHẦN sẽ cho gỡ dòng ② rồi cấp lại số ② cho một dòng khác, tức đường ghi được phép đánh số
    -- lại theo vị trí mảng mà không có gì đỏ. `tools/check_khoa_duy_nhat.py` từ chối đúng hình
    -- dạng ấy, và `ket_luan_hop` (0007) đã trả lời y hệt cho số kết luận.
    UNIQUE (tenant_id, nhiem_vu_id, nhom, thu_tu),

    -- CÙNG DỊCH VỤ, CÙNG LƯỢC ĐỒ — khoá không băng qua ranh giới nào (luật 2 cấm #2 không áp).
    FOREIGN KEY (tenant_id, nhiem_vu_id) REFERENCES nhiem_vu (tenant_id, id),

    CONSTRAINT nhiem_vu_van_ban_nhom_hop_le
        CHECK (nhom IN ('cap-tren-giao', 'chi-dao-dang-uy', 'san-pham-dau-ra')),

    CONSTRAINT nhiem_vu_van_ban_trich_yeu_khong_rong CHECK (btrim(trich_yeu) <> ''),

    -- SỐ ĐẦU TIÊN LÀ 1. Số không hoặc số âm là một vị trí không màn hình nào vẽ được, và nó phá
    -- luôn phép "max + 1" của đường ghi.
    CONSTRAINT nhiem_vu_van_ban_thu_tu_tu_mot CHECK (thu_tu >= 1),

    -- HAI CỘT XOÁ MỀM ĐI CÙNG NHAU HOẶC KHÔNG CÓ CỘT NÀO. Một dòng đã gỡ mà không nói ai gỡ là
    -- một dòng biến mất khỏi màn hình không quy được về ai (luật 6 bất biến 8).
    CONSTRAINT nhiem_vu_van_ban_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nhiem_vu_van_ban_p%s PARTITION OF nhiem_vu_van_ban '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- Ba danh sách của một nhiệm vụ, mỗi nhóm theo đúng thứ tự đã cấp — đúng thứ ngăn chi tiết vẽ.
--
-- CHỈ MỤC THƯỜNG, nên `WHERE deleted_at IS NULL` ở đây HỢP LỆ: nó chỉ quyết định đọc ít dòng
-- hơn, không quyết định một số đã cấp có được cấp lại hay không (luật 7 bất biến 3;
-- `tools/check_khoa_duy_nhat.py:274` chỉ từ chối hình dạng ấy cho chỉ mục DUY NHẤT). Khoá duy
-- nhất bên trên vẫn tính cả dòng đã xoá.
CREATE INDEX IF NOT EXISTS nhiem_vu_van_ban_theo_nhiem_vu
    ON nhiem_vu_van_ban (tenant_id, nhiem_vu_id, nhom, thu_tu)
    WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS nhiem_vu_van_ban_cam_xoa ON nhiem_vu_van_ban;
CREATE TRIGGER nhiem_vu_van_ban_cam_xoa
    BEFORE DELETE ON nhiem_vu_van_ban
    FOR EACH ROW EXECUTE FUNCTION nhiem_vu_van_ban_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- TỆP NÀY KHÔNG CHẶN ĐƯỢC GÌ, nói thẳng chứ không để người sau tự phát hiện: TRUNCATE trên bảng
-- này, và DDL của chủ bảng (ALTER TABLE ... DISABLE TRIGGER, dropping a table). Cùng lằn ranh
-- ADR 0013 vạch cho sổ kiểm toán — việc của trigger là làm cho cái VÔ Ý và cái TIỆN TAY thành
-- không làm được, không phải đánh bại một quản trị viên đã quyết phá dữ liệu và chấp nhận bị
-- nhìn thấy.
--
-- HOÀN NGUYÊN (câu hỏi 3). Mọi đối tượng ở đây đều mới, và trong khi bảng còn rỗng — đúng trạng
-- thái của mọi môi trường hôm nay — hoàn nguyên là trọn vẹn và không mất gì: bỏ bảng
-- `nhiem_vu_van_ban` (32 mảnh và trigger đi theo nó), rồi bỏ hàm
-- `nhiem_vu_van_ban_cam_xoa_cung()`.
--
-- và TRONG CÙNG giao dịch, xoá hàng của tệp này khỏi `schema_migration`, nếu không trình chạy
-- vẫn tin lược đồ đang có. Khoá ngoại đi vào `nhiem_vu` biến mất cùng bảng này, nên 0006 và 0008
-- không phải đặt lại gì.
--
-- KHI MỘT XÃ ĐÃ CÓ MỘT DÒNG Ở ĐÂY THÌ ĐÓ KHÔNG CÒN LÀ HOÀN NGUYÊN — đó là xoá một phần nội dung
-- của những hồ sơ hành chính đang sống, tức điều kiện dừng thứ nhất của luật 7, cần người dùng
-- chứ không cần một câu lệnh. Từ điểm ấy, đường lùi là một migration MỚI, và `core/migrate` cố ý
-- không có rollback tự động (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- CHẶN CUỐI: mọi bảng phân mảnh trong lược đồ này phải thật sự có mảnh.
--
-- Một bảng khai PARTITION BY mà không có mảnh nào TỪ CHỐI MỌI INSERT, lặng lẽ, cho tới lần ghi
-- thật đầu tiên — và vì vết kiểm toán dùng chung giao dịch nghiệp vụ (luật 6 bất biến 3), lần
-- ghi ấy quay lui toàn bộ. Phép kiểm này lặp lại ở cuối mọi migration khai bảng phân mảnh, vì nó
-- chỉ kiểm được trạng thái SAU một tệp có MANG nó (0002, §BACKSTOP).
-- ---------------------------------------------------------------------------
DO $$
DECLARE missing_partitions text;
BEGIN
    SELECT string_agg(c.relname, ', ' ORDER BY c.relname) INTO missing_partitions
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.relkind = 'p'
      AND n.nspname = current_schema()
      AND NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhparent = c.oid);

    IF missing_partitions IS NOT NULL THEN
        RAISE EXCEPTION
            'partitioned table(s) with no partitions in schema %: %',
            current_schema(), missing_partitions
            USING HINT = 'A partitioned table with no partitions rejects every INSERT. Add the '
                         'MODULUS 32 partition loop (ADR 0010) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
