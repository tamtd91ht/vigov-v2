-- 0023 — the petition timeline learns one more act: `tao-nhiem-vu`, a task booked FROM this petition
-- (POST /api/v1/citizen-reports/{maTraCuu}/tasks, user decision 30/09/2026).
--
-- WHY THIS FILE EXISTS. The task and the petition's timeline row are written in ONE transaction
-- (rule 2, invariant 6; rule 6, invariant 3), and 0013 closed `hanh_vi` on purpose ("a new act is a
-- migration"). Without this widening every task-from-petition rolls back on PostgreSQL — task, both
-- audit entries and all — while the fake-driver suite stays green; internal/app
-- TestLogActionsAreAllowedByTheSchema reads the latest CHECK from migrations.FS to catch exactly that.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0018: core/migrate compares the checksum of every applied file.
--
-- LOSSLESS: the list only WIDENS (0018's nine codes plus one). Every existing row still passes. ADD
-- CONSTRAINT validates every row of the table under ACCESS EXCLUSIVE; the timeline is young, so this is
-- expected to be short — unmeasured. The same shape as 0018.
--
-- REVERSAL: re-create 0018's nine-code CHECK. Lossless ONLY while no row holds `tao-nhiem-vu`; once
-- one does, reversing would require deleting timeline rows, which rule 7 forbids — so from then on
-- this migration is not reversed, and a reversal is a stop condition (#2) for the user.

ALTER TABLE nhat_ky_phan_anh DROP CONSTRAINT IF EXISTS nhat_ky_phan_anh_hanh_vi_hop_le;
ALTER TABLE nhat_ky_phan_anh ADD CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le CHECK (hanh_vi IN (
    'phan-loai', 'phan-cong', 'chuyen-trang-thai', 'dong-phieu', 'khong-tiep-nhan',
    'chuyen-cap-tren', 'ghi-chu', 'danh-gia', 'mo-lai-theo-danh-gia', 'tao-nhiem-vu'));
