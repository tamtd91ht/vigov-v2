-- 0034 — the task register learns a fifth source of work: `don-thu`, a task booked FROM a citizen
-- letter (POST /api/v1/citizen-letter-tasks, ADR 0085 A5; ADR 0084 #6). `nguon_id` then holds the
-- letter's id in service-documents (`citizen_letter.id`) — the link is the pair, one fact, one owner.
--
-- WHY THIS FILE EXISTS. 0006 closed `nguon_giao` to four codes on purpose ("a fifth is a new
-- integration rather than a new label"). This IS that integration. Without the widening every task
-- from a letter rolls back on PostgreSQL — task, timeline row and audit entry — while the fake-driver
-- suite stays green; internal/app TestTaskSourcesAreAllowedByTheSchema reads the latest CHECK from
-- migrations.FS to catch exactly that.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied file.
--
-- PARTITIONED TABLE. `nhiem_vu` is PARTITION BY HASH (tenant_id) (0006). A CHECK dropped and added on
-- the PARENT is dropped from and added to every partition by PostgreSQL itself — the shape 0023 used
-- for `nhat_ky_phan_anh_hanh_vi_hop_le`. Nothing is created per partition here.
--
-- LOSSLESS: the list only WIDENS (0006's four codes plus one). Every existing row still passes. ADD
-- CONSTRAINT validates every row of every partition under ACCESS EXCLUSIVE; expected to be short —
-- unmeasured on a production-sized register.
--
-- REVERSAL: re-create 0006's four-code CHECK. Lossless ONLY while no row holds `don-thu`; once one
-- does, reversing would require deleting or rewriting tasks, which rule 7 forbids — so from then on
-- this migration is not reversed, and a reversal is a stop condition (#2) for the user.

ALTER TABLE nhiem_vu DROP CONSTRAINT IF EXISTS nhiem_vu_nguon_giao_hop_le;
ALTER TABLE nhiem_vu ADD CONSTRAINT nhiem_vu_nguon_giao_hop_le CHECK (nguon_giao IN (
    'truc-tiep', 'ket-luan-hop', 'van-ban-den', 'phan-anh', 'don-thu'));
