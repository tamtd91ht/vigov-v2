-- 0018 — the petition timeline learns two acts caused by the CITIZEN: `danh-gia` (a star rating) and
-- `mo-lai-theo-danh-gia` (the reopen a 1–2 star rating causes).
--
-- WHY THIS FILE EXISTS. ADR 0050 point 2 (round 2, 28/09/2026, following vigov-require
-- service.py:804-840): the citizen rates 1–5 stars at `da-xu-ly` / `cho-dan-xac-nhan`; 1 or 2 stars
-- reopen the petition to `dang-xu-ly`. The staff drawer's timeline must show both, or a petition
-- jumps back to "đang xử lý" with nothing on the page saying why. 0013 closed `hanh_vi` to seven
-- codes on purpose ("a new act is a migration"), so this is that migration. Without it every rating
-- rolls back on PostgreSQL, because the log row is written in the rating's transaction (rule 6,
-- invariant 3) — internal/app TestLogActionsAreAllowedByTheSchema reads the latest CHECK from
-- migrations.FS to catch exactly that.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0013: core/migrate compares the checksum of every applied file.
--
-- LOSSLESS: the list only WIDENS. Every existing row holds one of the seven old codes and still
-- passes. ADD CONSTRAINT validates every row of the table (and its partitions) under ACCESS
-- EXCLUSIVE; the timeline is young, so this is expected to be short — unmeasured.
--
-- REVERSAL: re-create the seven-code CHECK from 0013:153-155. That is lossless ONLY while no row
-- holds `danh-gia` or `mo-lai-theo-danh-gia`; once one does, reversing would require deleting timeline
-- rows, which rule 7 forbids — so from then on this migration is not reversed, and a reversal is a
-- stop condition (#2) for the user.

ALTER TABLE nhat_ky_phan_anh DROP CONSTRAINT IF EXISTS nhat_ky_phan_anh_hanh_vi_hop_le;
ALTER TABLE nhat_ky_phan_anh ADD CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le CHECK (hanh_vi IN (
    'phan-loai', 'phan-cong', 'chuyen-trang-thai', 'dong-phieu', 'khong-tiep-nhan',
    'chuyen-cap-tren', 'ghi-chu', 'danh-gia', 'mo-lai-theo-danh-gia'));
