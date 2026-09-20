package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// This file is the arithmetic behind AdvanceWorkingHours: move an instant forward through ONE
// commune's working calendar and say when that many WORKING hours have elapsed.
//
// IT LIVES IN domain/ AND NOT IN internal/grpc/ ON PURPOSE. The refusals below are the ones
// migration 0006 hands to the reader because no CHECK can enforce them, and ADR 0007 decisions 8
// and 9 are business rules about a commitment told to a citizen — not transport concerns. Keeping
// them here means the one place they can be changed is the one place a reviewer looks for them,
// and that a caller on the other side of the wire (petitions, documents) has no shape in which to
// re-implement them differently.
//
// NOTHING HERE READS A DATABASE, A CLOCK, OR AN ENVIRONMENT. The calendar arrives as arguments and
// as DocLichNam; `now` is never consulted (ADR 0007 decision 5: a document that arrived yesterday
// and is registered today counts from yesterday); and the zone is NAMED rather than taken from the
// process. That is what makes every case below testable without a PostgreSQL and without a TZ.

// MuiGioHanhChinh is the zone the commune's wall-clock calendar is interpreted in.
//
// NAMED EXPLICITLY, NEVER time.Local, AND THE DIFFERENCE IS THE WHOLE ANSWER. `lich_lam_viec.bat_dau`
// is a TIME WITHOUT TIME ZONE: "07:30" is an instruction to a commune's staff, not a moment in
// history (migration 0006:104). It becomes an instant only when combined with a DATE in this zone.
// A pod started with TZ=UTC must therefore compute the same deadline as one started without it —
// reading time.Local would make the commitment told to a citizen depend on a container's
// environment variable, and nothing on any screen would show it.
//
// ONE ZONE FOR EVERY COMMUNE, and it is a constant rather than a per-commune column because
// Vietnam has one zone. The day that stops being true, this constant is the single place to look,
// and the value would move to the commune's configuration rather than to a caller's argument.
const MuiGioHanhChinh = "Asia/Ho_Chi_Minh"

// ChanTroiNgay is how far after the starting instant an answer may fall: 366 days.
//
// THE REASON IS THE VIETNAMESE WORKING YEAR, not a wish to bound a loop. Tết and National Day
// arrangements — including làm bù — are announced ONE YEAR AT A TIME (migration 0006:238), so a
// commune's calendar is only ever complete for about a year ahead. A count that walks past the
// last holidays anybody entered does not fail: it returns a confident instant computed from a
// stretch of time with no holidays in it, which is the worst of the available answers. Bounding
// the walk is the smaller of the two reasons.
//
// IT DOES NOT MAKE THE CALENDAR COMPLETE, and no rule here can: a count starting in December and
// running into January is inside the horizon and still crosses a year whose holidays may not be
// entered yet. That gap belongs to whoever operates the configuration screen.
const ChanTroiNgay = 366

// MuiGio returns the commune's zone, or the reason the process cannot produce it.
//
// AN ERROR AND NOT A FALLBACK TO time.UTC OR time.Local. A calendar interpreted seven hours out
// puts every session boundary in the wrong place, and the answer would still look like an answer.
// Whoever calls this refuses — the gRPC server also calls it once at construction, so a deployment
// whose image lost its zone database fails to start rather than at the first intake.
//
// NOT MEMOISED, DELIBERATELY. Caching the result in a package variable would cost one file read
// per RECEIVED RECORD — this is not on any per-request path (rule 10, invariant 2: the deadline is
// computed once, at intake, and stored) — and it would buy a property that cannot be tested: with
// the answer captured at the first call, a test can no longer tell an implementation that names
// this zone from one that reads the process's. The zone database is read by time.LoadLocation,
// which the operating system's page cache already keeps in memory.
func MuiGio() (*time.Location, error) {
	mui, err := time.LoadLocation(MuiGioHanhChinh)
	if err != nil {
		return nil, fmt.Errorf("lịch làm việc: không nạp được múi giờ %q: %w", MuiGioHanhChinh, err)
	}
	return mui, nil
}

// MocDatDuoc is one requested amount of working time and the instant this commune reaches it.
type MocDatDuoc struct {
	// Gio is the amount asked for, in whole working hours. It is the KEY the caller maps by —
	// duplicates collapse, so a position is never a key.
	Gio int

	// DatLuc is the EARLIEST instant at which the commune has been open for Gio hours since the
	// starting instant. "Earliest" is arithmetic and not a policy: a count running out exactly at
	// 11:30 with the afternoon opening at 13:30 answers 11:30. Returning 13:30 would hand the
	// commune two hours nobody granted it, on every deadline landing on a session boundary — and
	// lunch is a session boundary every working day.
	DatLuc time.Time
}

// DocLichNam reads ONE calendar year of this commune's closures and swap-day sessions.
//
// A FUNCTION TYPE AND NOT AN INTERFACE, AND THE STORE STAYS ON THE OTHER SIDE OF IT: domain
// imports nothing but the standard library, so the two `TheoNam` reads cannot be called from here.
// The signature carries an error because the reads can fail — and because the CONFLICT refusal
// (`*LoiNgayVuaNghiVuaLamBu`, a date both closed and working) is detected inside those reads, in
// one SQL statement, and must travel out unchanged rather than be re-derived here.
//
// CALLED LAZILY, AT MOST ONCE PER YEAR THE WALK ACTUALLY ENTERS. A 16-hour deadline in March
// reads one year; it must not be charged for the year after, and a configuration fault in a year
// the count never reaches must not change this answer.
type DocLichNam func(nam int) ([]NgayNghiLe, []CaLamBu, error)

// LoaiLoiTinhHan names a reason a deadline cannot be computed. The values are stable identifiers a
// caller may branch on and a test may assert on, so the sentence can be rewritten without
// rewriting either.
type LoaiLoiTinhHan string

const (
	// LoiLichTrong: the commune has no working session at all, so a count in working hours never
	// arrives. THE REFUSAL IS THE ANSWER — "Mon–Fri 08:00–17:00" is not a fallback, it is a
	// commitment invented by software and told to a citizen (rule 10; migration 0006:56).
	//
	// THE SAME STRING VanDeCuaLich ALREADY PUBLISHES, deliberately: one fault, one spelling, so
	// the configuration screen and this refusal cannot describe one state with two words.
	LoiLichTrong = LoaiLoiTinhHan(VanDeLichTrong)

	// LoiCaChongNhau: two sessions overlap, so their overlap is counted twice and the deadline
	// comes out EARLIER than the commune's real hours — the direction that reports an authority
	// late when it was not. The database cannot refuse it (migration 0006:109).
	LoiCaChongNhau = LoaiLoiTinhHan(VanDeCaChongNhau)

	// LoiLamBuTrungNgayDaLam: a `ngay_lam_bu` row on a date whose weekday ALREADY has sessions.
	// ADR 0007 decision 9 refuses it rather than choosing between "the swap day REPLACES those
	// hours" and "it ADDS to them" — the two readings differ by exactly the overlap, nothing
	// decides between them, and loosening a refusal later is additive while un-inventing a
	// deadline computed from double-counted hours is not.
	LoiLamBuTrungNgayDaLam LoaiLoiTinhHan = "swap_day_on_working_day"

	// LoiVuotChanTroi: the answer would fall beyond ChanTroiNgay — see that constant.
	LoiVuotChanTroi LoaiLoiTinhHan = "beyond_horizon"
)

// LoiKhongTinhDuocHan is the ONE error type for "this commune's configuration cannot answer".
//
// ONE TYPE AND FOUR REASONS RATHER THAN FOUR TYPES, because every caller makes the SAME decision
// on all four: refuse, and send a human to the configuration screen (the gRPC boundary answers
// FAILED_PRECONDITION, never Internal — the difference is "fix the calling service" versus "open
// the configuration screen", and an operator sent to the wrong one reads a calendar that is fine
// for an afternoon). `Loai` is what a test and a log line branch on; `ChiTiet` is the sentence the
// person who has to fix it reads.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a weekday, a date and a row id say when an authority is
// open. That is what makes it safe for the sentence to cross the boundary, and a refusal that does
// not name the day to look at is a refusal nobody can act on.
type LoiKhongTinhDuocHan struct {
	Loai    LoaiLoiTinhHan
	ChiTiet string
}

func (e *LoiKhongTinhDuocHan) Error() string { return e.ChiTiet }

// TienGioLamViec advances tuMoc through this commune's calendar and answers, for every amount in
// `gio`, the earliest instant at which the commune has been open that many hours.
//
// # ADR 0007 decision 8 is the shape of the loop, not a branch inside it
//
// A record received OUTSIDE every working session counts from the START OF THE NEXT WORKING
// SESSION. That is expressed by counting only the part of each session that falls AT OR AFTER
// tuMoc: a Friday 22:00 receipt is credited nothing for Friday night or the weekend, so its
// deadline equals that of a record received at Monday 07:30. "The next session" is the next one in
// the commune's REAL calendar — the ordinary week, minus its holidays, plus its swap days — never
// "the next working day" and never "08:00 tomorrow".
//
// Written as a clip rather than as an `if outside { jump }` because the two are the same rule and
// one of them has an edge case: a jump has to decide what "outside" means at 11:30 exactly, and
// gets it wrong in the direction that credits the commune a lunch break.
//
// # What it refuses, and none of these is a result
//
//	empty calendar            no working hours exist, so no count in working hours can arrive
//	overlapping sessions      those hours would be counted twice
//	holiday AND swap day      refused by the read itself; this walk only carries the error out
//	swap day on a working day ADR 0007 decision 9
//	an answer past the horizon see ChanTroiNgay
//
// # The starting instant is never compared with `now`
//
// tuMoc is the caller's mark, and the caller owns it (ADR 0007 decision 5). Clamping it forward to
// `now` would silently move a commitment the caller had already fixed.
func TienGioLamViec(tuMoc time.Time, tuan []CaLamViec, docNam DocLichNam, gio []int) ([]MocDatDuoc, error) {
	if docNam == nil {
		// A programming fault in this service's own wiring, not a commune's configuration: it
		// must not come back as FAILED_PRECONDITION and send an operator to a screen that is fine.
		return nil, fmt.Errorf("lịch làm việc: thiếu hàm đọc ngày nghỉ lễ và ngày làm bù")
	}
	moc, err := mocTheoGiay(gio)
	if err != nil {
		return nil, err
	}

	// THE WEEKLY CALENDAR IS CHECKED WHOLE, BEFORE THE WALK, and both of VanDeCuaLich's problems
	// are refusals here. It is the same function the configuration screen renders, so a week this
	// refuses to count is exactly the week that screen shows as broken.
	if vd := VanDeCuaLich(tuan); len(vd) > 0 {
		return nil, loiTuVanDeLich(vd[0])
	}

	mui, err := MuiGio()
	if err != nil {
		return nil, err
	}

	theoThu := map[int][]caTrongNgay{}
	for _, c := range tuan {
		theoThu[c.Thu] = append(theoThu[c.Thu], caTrongNgay{BatDau: c.BatDau, KetThuc: c.KetThuc})
	}
	for thu := range theoThu {
		// The store already orders by (thu, bat_dau); sorting a copy costs nothing and removes an
		// assumption this arithmetic would otherwise be silently wrong about if the read changed.
		sapXepCa(theoThu[thu])
	}

	nam := &lichTheoNam{doc: docNam, daNap: map[int]lichMotNam{}}

	// Everything below is instants. tuMoc keeps whatever zone it arrived in — comparisons between
	// instants do not care — while every boundary built from the calendar is built in mui.
	batDauVN := tuMoc.In(mui)
	chanTroi := batDauVN.AddDate(0, 0, ChanTroiNgay)
	ngayCuoi := nuaDem(chanTroi, mui)

	ra := make([]MocDatDuoc, 0, len(moc))
	var daCong time.Duration
	i := 0

	for ngay := nuaDem(batDauVN, mui); !ngay.After(ngayCuoi) && i < len(moc); ngay = ngay.AddDate(0, 0, 1) {
		cas, err := caCuaNgay(ngay, theoThu, nam)
		if err != nil {
			return nil, err
		}
		for _, ca := range cas {
			mo := ngay.Add(time.Duration(ca.BatDau) * time.Second)
			dong := ngay.Add(time.Duration(ca.KetThuc) * time.Second)
			if !dong.After(tuMoc) {
				// The whole session is behind the starting instant. Nothing is credited for it —
				// ADR 0007 decision 8: no session is ever counted in part for time the authority
				// was closed.
				continue
			}
			if mo.Before(tuMoc) {
				// Received DURING a session: the clock starts where the record arrived, not at the
				// session's opening.
				mo = tuMoc
			}
			dai := dong.Sub(mo)

			// `>=` AND NOT `>`, AND THIS IS THE OFF-BY-ONE-SESSION THE CONTRACT WRITES DOWN. When
			// the count runs out exactly at this session's end, the answer is that end — 11:30,
			// not the afternoon's 13:30.
			for i < len(moc) && daCong+dai >= moc[i] {
				ra = append(ra, MocDatDuoc{
					Gio:    int(moc[i] / time.Hour),
					DatLuc: mo.Add(moc[i] - daCong),
				})
				i++
			}
			daCong += dai
			if i == len(moc) {
				break
			}
		}
	}

	if i < len(moc) {
		return nil, &LoiKhongTinhDuocHan{
			Loai: LoiVuotChanTroi,
			ChiTiet: fmt.Sprintf(
				"lịch làm việc: %d giờ làm việc không đạt được trong %d ngày kể từ mốc bắt đầu — "+
					"lịch nghỉ lễ và ngày làm bù chỉ được khai trước khoảng một năm, nên hệ thống "+
					"từ chối thay vì đếm qua quãng thời gian chưa ai khai",
				int(moc[len(moc)-1]/time.Hour), ChanTroiNgay),
		}
	}
	// The horizon is an INSTANT, not a date: a session on the last day may end after it. The
	// amounts are ascending, so the last answer is the latest one.
	if muon := ra[len(ra)-1]; muon.DatLuc.After(chanTroi) {
		return nil, &LoiKhongTinhDuocHan{
			Loai: LoiVuotChanTroi,
			ChiTiet: fmt.Sprintf(
				"lịch làm việc: hạn %d giờ làm việc rơi quá %d ngày sau mốc bắt đầu — "+
					"lịch nghỉ lễ và ngày làm bù chỉ được khai trước khoảng một năm",
				muon.Gio, ChanTroiNgay),
		}
	}
	return ra, nil
}

// caTrongNgay is one session of ONE date, after the weekly calendar and the swap days have been
// reconciled. Two sources, one shape — the walk above must not branch on which table a session
// came from, because that is where a rule starts applying to one of the two and not the other.
type caTrongNgay struct {
	BatDau  GioTrongNgay
	KetThuc GioTrongNgay
}

func sapXepCa(cas []caTrongNgay) {
	sort.SliceStable(cas, func(i, j int) bool { return cas[i].BatDau < cas[j].BatDau })
}

// caCuaNgay answers what this commune's sessions are on one DATE.
//
// THE ORDER OF THE THREE QUESTIONS IS THE RULE ITSELF:
//
//  1. closed for a holiday → no sessions, whatever the week says;
//  2. a swap day → THOSE hours, because the weekday it falls on usually has none of its own;
//  3. otherwise → the ordinary week's sessions for that ISO weekday.
func caCuaNgay(ngay time.Time, theoThu map[int][]caTrongNgay, nam *lichTheoNam) ([]caTrongNgay, error) {
	lich, err := nam.cua(ngay.Year())
	if err != nil {
		return nil, err
	}
	chuoi := ngay.Format("2006-01-02")
	lamBu := lich.lamBu[chuoi]

	if lich.nghi[chuoi] {
		if len(lamBu) > 0 {
			// BACKSTOP, AND IT PICKS NO WINNER. Both reads detect this conflict in their own SQL
			// and refuse the whole year, so this branch is unreachable through the real stores —
			// it exists because a read path that ASSUMES a guarantee it does not enforce answers
			// confidently and wrongly the day that guarantee moves. The error type is the store's
			// own, so nothing downstream gains a second vocabulary for one fault.
			return nil, &LoiNgayVuaNghiVuaLamBu{Ngay: []string{chuoi}}
		}
		return nil, nil
	}

	thu := thuISO(ngay)
	if len(lamBu) > 0 {
		if len(theoThu[thu]) > 0 {
			// ADR 0007 decision 9. REPLACE or ADD are both defensible and they differ by exactly
			// the overlap; nothing decides between them, so neither is guessed.
			return nil, &LoiKhongTinhDuocHan{
				Loai: LoiLamBuTrungNgayDaLam,
				ChiTiet: fmt.Sprintf(
					"lịch làm việc: ngày %s được khai là ngày làm bù nhưng thứ %d vốn đã có ca làm "+
						"việc — hệ thống không tự chọn giữa THAY và CỘNG THÊM giờ, vui lòng sửa cấu hình",
					chuoi, thu),
			}
		}
		return lamBu, nil
	}
	return theoThu[thu], nil
}

// thuISO converts Go's weekday to the ISO 8601 weekday the calendar is stored with: 1 = Monday …
// 7 = Sunday.
//
// THE ONE WEEKDAY CONVERSION IN THIS SERVICE, and it is written once because this is exactly where
// an off-by-one lives for a year before anybody notices the deadline is a day out (migration
// 0006:98). time.Weekday counts Sunday as 0; the column never does.
func thuISO(t time.Time) int {
	if t.Weekday() == time.Sunday {
		return 7
	}
	return int(t.Weekday())
}

// nuaDem is midnight of t's date IN THE COMMUNE'S ZONE.
//
// Built with time.Date rather than by truncating: Truncate works on the instant since the epoch
// and would land on a UTC boundary, which is 07:00 local — a session boundary computed from it
// would be seven hours out with nothing failing.
func nuaDem(t time.Time, mui *time.Location) time.Time {
	y, m, d := t.In(mui).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, mui)
}

// mocTheoGiay turns the requested amounts into ascending, distinct durations.
//
// SORTED AND DEDUPED HERE rather than trusted from the caller, so the single walk above can serve
// every amount in one pass: the answers come out in ascending order because the amounts do. The
// caller collapses duplicates too (the wire contract says so) — doing it again costs one map and
// removes an assumption this loop would otherwise be quietly wrong about.
//
// A ZERO OR NEGATIVE AMOUNT IS REFUSED AS A PLAIN ERROR, not as LoiKhongTinhDuocHan: the gRPC
// boundary already answers INVALID_ARGUMENT for it, so reaching here means this service's own two
// halves disagree — and that is Internal, never a commune's configuration.
func mocTheoGiay(gio []int) ([]time.Duration, error) {
	if len(gio) == 0 {
		return nil, fmt.Errorf("lịch làm việc: không có mốc giờ làm việc nào để tính")
	}
	thay := make(map[int]struct{}, len(gio))
	ra := make([]time.Duration, 0, len(gio))
	for _, g := range gio {
		if g <= 0 {
			return nil, fmt.Errorf("lịch làm việc: mốc %d giờ làm việc không hợp lệ", g)
		}
		if _, co := thay[g]; co {
			continue
		}
		thay[g] = struct{}{}
		ra = append(ra, time.Duration(g)*time.Hour)
	}
	sort.Slice(ra, func(i, j int) bool { return ra[i] < ra[j] })
	return ra, nil
}

// loiTuVanDeLich turns the problem the configuration screen shows into the refusal the deadline
// path answers with. ONE DETECTION, TWO PRESENTATIONS — a second detector written here would drift
// from the one the screen renders, and the commune would be told its calendar is fine.
func loiTuVanDeLich(vd VanDeLich) error {
	switch vd.Loai {
	case VanDeLichTrong:
		return &LoiKhongTinhDuocHan{
			Loai: LoiLichTrong,
			ChiTiet: "lịch làm việc: xã chưa khai ca làm việc nào, nên không tính được hạn bằng " +
				"giờ làm việc — vui lòng khai lịch làm việc trước",
		}
	case VanDeCaChongNhau:
		return &LoiKhongTinhDuocHan{
			Loai: LoiCaChongNhau,
			ChiTiet: fmt.Sprintf(
				"lịch làm việc: thứ %d có hai ca chồng giờ nhau (%s) — giờ chồng nhau sẽ bị đếm "+
					"hai lần, vui lòng sửa cấu hình",
				vd.Thu, strings.Join(vd.CaID, ", ")),
		}
	default:
		// VanDeCuaLich gains a third kind only by an edit to this package, and an unknown kind must
		// not fall through to an answer: it would be a fault nobody refuses.
		return &LoiKhongTinhDuocHan{
			Loai:    LoaiLoiTinhHan(vd.Loai),
			ChiTiet: fmt.Sprintf("lịch làm việc: lịch của xã không dùng để tính hạn được (%s)", vd.Loai),
		}
	}
}

// lichMotNam is one calendar year of closures and swap-day sessions, indexed by date.
type lichMotNam struct {
	nghi  map[string]bool
	lamBu map[string][]caTrongNgay
}

// lichTheoNam reads a year AT MOST ONCE PER CALL and remembers it for the rest of THAT call.
//
// THIS IS NOT A CACHE, AND THE DISTINCTION IS THE ONE THE CONTRACT ARGUES AT LENGTH. It lives for
// the duration of one RPC and dies with it, so the next intake reads the calendar as it stands
// then. A cache that OUTLIVED the call would invert ADR 0007 decision 6 — a configuration change
// is not retroactive, it applies to records received after it — by computing a deadline from the
// calendar as it was before, on whichever process still held the stale copy.
type lichTheoNam struct {
	doc   DocLichNam
	daNap map[int]lichMotNam
}

func (l *lichTheoNam) cua(nam int) (lichMotNam, error) {
	if co, roi := l.daNap[nam]; roi {
		return co, nil
	}
	nghi, lamBu, err := l.doc(nam)
	if err != nil {
		return lichMotNam{}, err
	}

	// THE SWAP DAYS OF THIS YEAR ARE CHECKED FOR OVERLAP THE SAME WAY THE WEEK IS, and with the
	// same function: `ngay_lam_bu` carries UNIQUE (tenant_id, ngay, bat_dau), which stops two
	// sessions STARTING at the same minute and nothing more (migration 0006:283).
	if vd := CaLamBuChongNhau(lamBu); len(vd) > 0 {
		return lichMotNam{}, &LoiKhongTinhDuocHan{
			Loai: LoiCaChongNhau,
			ChiTiet: fmt.Sprintf(
				"lịch làm việc: ngày làm bù %s có hai ca chồng giờ nhau (%s) — giờ chồng nhau sẽ "+
					"bị đếm hai lần, vui lòng sửa cấu hình",
				vd[0].Ngay, strings.Join(vd[0].CaID, ", ")),
		}
	}

	ra := lichMotNam{
		nghi:  make(map[string]bool, len(nghi)),
		lamBu: make(map[string][]caTrongNgay, len(lamBu)),
	}
	for _, n := range nghi {
		ra.nghi[n.Ngay] = true
	}
	for _, c := range lamBu {
		ra.lamBu[c.Ngay] = append(ra.lamBu[c.Ngay], caTrongNgay{BatDau: c.BatDau, KetThuc: c.KetThuc})
	}
	for ngay := range ra.lamBu {
		sapXepCa(ra.lamBu[ngay])
	}
	l.daNap[nam] = ra
	return ra, nil
}
