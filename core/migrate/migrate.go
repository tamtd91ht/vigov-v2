// Package migrate applies a service's OWN schema migrations, once, at startup.
//
// WHY THIS EXISTS: sixteen migration files were committed and nothing in the repository
// applied any of them. The only two readers were integration suites, each hard-coding
// `0001_init.sql`, so `0002_audit_log_append_only.sql` — the file that turns "append-only"
// from a comment into a database constraint — had never run anywhere. A migration nothing
// applies is a migration that is not there.
//
// # WHAT THIS RUNNER IS, AND WHAT IT IS NOT
//
// It applies SCHEMA migrations: DDL that belongs to the whole database of one service. The
// schema is shared by every commune the process serves — one set of tables, partitioned by
// `tenant_id` (ADR 0010), not one set of tables per commune. So a run here has no commune in
// it, and deliberately takes none: there is no `tenant_id` on this path to get wrong.
//
// READ THIS BEFORE TREATING RULE 7 INVARIANT 5 AS SATISFIED. That invariant says migrations
// run "per commune, resumable, recording progress". This runner covers the last two for SCHEMA
// work and nothing at all of the first, because "per commune" does not apply to DDL. The part
// that genuinely needs it is DATA BACKFILL — rewriting existing rows of one commune at a time,
// resumable half-way through, so a backfill interrupted after commune 40 of 200 continues at 41
// instead of starting over and touching archival records twice.
//
// THAT MECHANISM DOES NOT EXIST YET. It is a different shape from this one: it needs a
// per-commune progress table, a batch size, and a way to be stopped and resumed, none of which
// a DDL runner has any use for. Do not read the presence of this package as the invariant being
// met; the backfill half is still open.
//
// # GUARANTEES
//
//   - Migrations are embedded in the binary (go:embed). The binary carries its own schema, so
//     "which version of the file is on the container" cannot be a question.
//   - Files are applied in filename order, each in ONE transaction together with its progress
//     row. A file that fails half-way leaves neither schema nor a row claiming it succeeded.
//   - An already-applied file whose bytes have changed STOPS the run. Two databases that both
//     say they are at 0002, with two different schemas, is the failure this prevents.
//   - One PostgreSQL advisory lock around the whole run, so two replicas starting together
//     cannot apply the same file twice.
//   - Re-running applies only what is missing.
//
// # WHAT IT DELIBERATELY DOES NOT DO
//
//   - No rollback, automatic or otherwise. Undoing DDL on archival records is an administrative
//     act, not something a process decides at 3am (rule 7). A failure stops the service and
//     waits for a person.
//   - No migration generation. A schema change is written by hand and reviewed.
//   - No support for statements that cannot run inside a transaction (CREATE INDEX
//     CONCURRENTLY). Such a statement would fail here, loudly; single-transaction-per-file is
//     the property that keeps a half-applied schema impossible, and it is not traded away
//     quietly.
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// BangTienDo is the table that records what has been applied.
//
// It carries no `tenant_id` on purpose, and this is the one table in the system where that is
// correct: it describes the shape of the DATABASE, which every commune in the process shares.
// A `tenant_id` here would claim communes can be at different schema versions, which they
// cannot be — they are rows in the same tables.
const BangTienDo = "schema_migration"

var (
	// ErrChecksumLech means a file that was already applied no longer matches what is in the
	// binary. Editing an applied migration leaves two databases claiming the same version with
	// different schemas, and nothing reports the difference afterwards.
	ErrChecksumLech = errors.New("migrate: tệp migration đã áp bị sửa")

	// ErrThieuTep means the database has applied a migration this binary does not carry — the
	// schema is AHEAD of the code. Refusing is the fail-closed answer: the running code would be
	// querying tables it does not know the shape of.
	ErrThieuTep = errors.New("migrate: tệp migration đã áp không có trong mã nguồn")
)

// KetQua reports what the run did, so the caller can log it at startup. An operator reading
// "applied 0002_audit_log_append_only.sql" is how a schema change becomes visible at all.
type KetQua struct {
	DaAp  []string // applied by THIS run, in the order applied
	BoQua []string // already present, skipped
}

type tepMigration struct {
	ten      string
	noiDung  []byte
	checksum string
}

// Chay applies every not-yet-applied migration in nguon, in filename order.
//
// tenDichVu names the service and decides the advisory lock key. Two services sharing one
// PostgreSQL instance must not serialise behind each other, and — more importantly — two
// replicas of the SAME service must land on the same key, which is why it is derived from the
// name rather than chosen per process.
//
// It is the caller's job to abort startup when this returns an error. Serving on a schema whose
// shape is unknown is worse than not serving: every query is then written against a schema that
// may not be there.
func Chay(ctx context.Context, db *sql.DB, nguon fs.FS, tenDichVu string) (KetQua, error) {
	if db == nil {
		return KetQua{}, errors.New("migrate: thiếu kết nối cơ sở dữ liệu")
	}
	if strings.TrimSpace(tenDichVu) == "" {
		// No default here: a guessed name is a guessed lock key, and two services sharing a
		// guessed key would each think they hold the migration lock alone.
		return KetQua{}, errors.New("migrate: thiếu tên dịch vụ — không suy ra được khoá")
	}

	tep, err := docNguon(nguon)
	if err != nil {
		return KetQua{}, err
	}

	// ONE pinned connection for the whole run. pg_advisory_lock is SESSION-scoped, so taking it
	// on a pooled handle would let the unlock land on a different connection and leave the lock
	// held by a connection that goes back into the pool — a lock nobody can find and nobody can
	// release short of killing the backend.
	conn, err := db.Conn(ctx)
	if err != nil {
		return KetQua{}, fmt.Errorf("migrate: lấy kết nối riêng: %w", err)
	}
	defer conn.Close() //nolint:errcheck // returning the connection to the pool; the unlock below is what matters

	khoa := khoaTuTen(tenDichVu)
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", khoa); err != nil {
		return KetQua{}, fmt.Errorf("migrate: không lấy được khoá %d cho %s: %w", khoa, tenDichVu, err)
	}

	kq, loi := chayTrongKhoa(ctx, conn, tep)

	// The unlock runs on a context that cannot be cancelled: if the startup context has already
	// expired, an unlock skipped here is a lock held until the connection dies, and the NEXT
	// replica to start would block on it with no explanation.
	if _, err := conn.ExecContext(context.WithoutCancel(ctx),
		"SELECT pg_advisory_unlock($1)", khoa); err != nil && loi == nil {
		loi = fmt.Errorf("migrate: không nhả được khoá %d: %w", khoa, err)
	}
	return kq, loi
}

func chayTrongKhoa(ctx context.Context, conn *sql.Conn, tep []tepMigration) (KetQua, error) {
	if err := taoBangTienDo(ctx, conn); err != nil {
		return KetQua{}, err
	}
	daAp, err := docTienDo(ctx, conn)
	if err != nil {
		return KetQua{}, err
	}

	// PRE-FLIGHT, BEFORE ANYTHING IS APPLIED. Every already-applied file is verified first, so a
	// mismatch stops the run with NOTHING applied — not "stopped after two more files went in".
	// Verifying lazily inside the apply loop would let 0003 land on a database whose 0001 is not
	// the 0001 in this binary, which is exactly the divergence being detected.
	if err := kiemChecksum(tep, daAp); err != nil {
		return KetQua{}, err
	}

	var kq KetQua
	for _, t := range tep {
		if _, roi := daAp[t.ten]; roi {
			kq.BoQua = append(kq.BoQua, t.ten)
			continue
		}
		if err := apMot(ctx, conn, t); err != nil {
			// Stop at the first failure. Carrying on would apply 0003 to a database that never
			// got 0002, and the progress table would then be a record of a schema nobody has.
			return kq, err
		}
		kq.DaAp = append(kq.DaAp, t.ten)
	}
	return kq, nil
}

// apMot applies one file and records it — in ONE transaction.
//
// THE PROGRESS ROW SHARES THE TRANSACTION ON PURPOSE, the same reason rule 6 invariant 3 gives
// for audit entries: written afterwards, a crash in the gap leaves the schema changed and the
// table saying it was not, and the next start applies the file a second time.
func apMot(ctx context.Context, conn *sql.Conn, t tepMigration) error {
	batDau := time.Now()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate: mở giao dịch cho %s: %w", t.ten, err)
	}

	if _, err := tx.ExecContext(ctx, string(t.noiDung)); err != nil {
		if rb := tx.Rollback(); rb != nil {
			return fmt.Errorf("migrate: %s hỏng: %w (rollback cũng hỏng: %v)", t.ten, err, rb)
		}
		return fmt.Errorf("migrate: %s hỏng, đã rollback: %w", t.ten, err)
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO "+BangTienDo+" (ten, checksum, ap_dung_luc, thoi_luong_ms) VALUES ($1, $2, $3, $4)",
		t.ten, t.checksum, time.Now().UTC(), time.Since(batDau).Milliseconds())
	if err != nil {
		if rb := tx.Rollback(); rb != nil {
			return fmt.Errorf("migrate: ghi tiến độ %s hỏng: %w (rollback cũng hỏng: %v)", t.ten, err, rb)
		}
		return fmt.Errorf("migrate: ghi tiến độ %s hỏng, đã rollback: %w", t.ten, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate: commit %s: %w", t.ten, err)
	}
	return nil
}

func taoBangTienDo(ctx context.Context, conn *sql.Conn) error {
	// IF NOT EXISTS rather than a bootstrap migration: this table has to exist before the first
	// migration can be recorded, so it cannot itself be one.
	_, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS `+BangTienDo+` (
    ten           text PRIMARY KEY,
    checksum      text NOT NULL,
    ap_dung_luc   timestamptz NOT NULL,
    thoi_luong_ms bigint NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("migrate: tạo bảng %s: %w", BangTienDo, err)
	}
	return nil
}

func docTienDo(ctx context.Context, conn *sql.Conn) (map[string]string, error) {
	rows, err := conn.QueryContext(ctx, "SELECT ten, checksum FROM "+BangTienDo)
	if err != nil {
		return nil, fmt.Errorf("migrate: đọc bảng %s: %w", BangTienDo, err)
	}
	defer rows.Close() //nolint:errcheck // the error that matters is rows.Err() below

	ra := map[string]string{}
	for rows.Next() {
		var ten, sum string
		if err := rows.Scan(&ten, &sum); err != nil {
			return nil, fmt.Errorf("migrate: đọc dòng tiến độ: %w", err)
		}
		ra[ten] = sum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: duyệt bảng %s: %w", BangTienDo, err)
	}
	return ra, nil
}

// kiemChecksum compares what the database has applied against what this binary carries.
//
// Both directions are faults, and both name the file: an operator who is told only "checksum
// mismatch" has sixteen files to open.
func kiemChecksum(tep []tepMigration, daAp map[string]string) error {
	coTrongNguon := make(map[string]struct{}, len(tep))
	for _, t := range tep {
		coTrongNguon[t.ten] = struct{}{}
		sum, roi := daAp[t.ten]
		if !roi {
			continue
		}
		if sum != t.checksum {
			return fmt.Errorf("%w: %s (đã áp checksum %s, mã nguồn %s). "+
				"Không chạy tiếp tệp nào. Sửa một migration đã chạy là để hai cơ sở dữ liệu "+
				"cùng khai một phiên bản với hai schema khác nhau — hãy thêm tệp mới thay vì sửa tệp cũ",
				ErrChecksumLech, t.ten, rutGon(sum), rutGon(t.checksum))
		}
	}

	// Deterministic order: the same database must report the same file every time, or two
	// operators comparing notes are looking at two different messages for one fault.
	var thieu []string
	for ten := range daAp {
		if _, roi := coTrongNguon[ten]; !roi {
			thieu = append(thieu, ten)
		}
	}
	if len(thieu) > 0 {
		sort.Strings(thieu)
		return fmt.Errorf("%w: %s. Không chạy tiếp tệp nào. "+
			"Cơ sở dữ liệu đang ở phiên bản MỚI HƠN mã đang chạy — dừng lại là câu trả lời đóng, "+
			"vì mã này sẽ truy vấn những bảng nó không biết hình dạng",
			ErrThieuTep, strings.Join(thieu, ", "))
	}
	return nil
}

// docNguon reads every *.sql in the root of nguon, sorted by filename.
//
// FILENAME ORDER IS THE CONTRACT, and the sort here is what enforces it: fs.ReadDir is only
// guaranteed to sort on the generic path, and an embed.FS or any other ReadDirFS returns
// whatever it returns. Applying 0002 before 0001 fails in a way that looks like a broken
// migration rather than a broken runner.
func docNguon(nguon fs.FS) ([]tepMigration, error) {
	if nguon == nil {
		return nil, errors.New("migrate: thiếu nguồn migration")
	}
	muc, err := fs.ReadDir(nguon, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: đọc thư mục migration: %w", err)
	}

	var ten []string
	for _, m := range muc {
		if m.IsDir() || !strings.HasSuffix(m.Name(), ".sql") {
			continue
		}
		ten = append(ten, m.Name())
	}
	sort.Strings(ten)

	tep := make([]tepMigration, 0, len(ten))
	for _, t := range ten {
		b, err := fs.ReadFile(nguon, t)
		if err != nil {
			return nil, fmt.Errorf("migrate: đọc %s: %w", t, err)
		}
		tong := sha256.Sum256(b)
		tep = append(tep, tepMigration{ten: t, noiDung: b, checksum: hex.EncodeToString(tong[:])})
	}
	return tep, nil
}

// khoaTuTen derives the advisory lock key from the service name.
//
// Deterministic across processes and across restarts — that is the whole requirement. Two
// replicas of one service must compute the same number without talking to each other, and two
// different services must not collide on it merely because they start at the same time.
func khoaTuTen(ten string) int64 {
	h := fnv.New64a()
	// Namespaced so the key cannot collide with an advisory lock some other part of the system
	// takes on the same database for an unrelated reason.
	_, _ = h.Write([]byte("vigov.migrate:" + ten))
	return int64(h.Sum64())
}

// rutGon shortens a checksum for an error message. The full 64 characters say nothing more to
// the reader than the first twelve, and a readable message is a message that gets acted on.
func rutGon(sum string) string {
	if len(sum) <= 12 {
		return sum
	}
	return sum[:12]
}
