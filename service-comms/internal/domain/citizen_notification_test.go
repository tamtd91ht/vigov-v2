package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR. Two rules decide whether a notification is worth sending, both are pure,
// and both fail SILENTLY if they stop holding:
//
//  1. the message carries the lookup code, says what changed, and says what happens next
//     (rule 10, invariant 6). A message that fails this still sends, still looks fine in a log,
//     and is useless to the person who receives it;
//  2. two notifications about the same fact produce the same key, and two notifications about
//     DIFFERENT facts never do. A key that is too loose swallows a real second message; a key
//     that is too tight sends a duplicate. Neither shows up anywhere except on a citizen's phone.

// --- (1) what a message must contain --------------------------------------------------------

func fullParams() NotificationParams {
	return NotificationParams{
		LookupCode:  "PA-2026-7F3K9Q",
		StatusLabel: "Đã chuyển xử lý",
		NextStep:    "Cán bộ phụ trách sẽ liên hệ với ông/bà trong 2 ngày làm việc.",
	}
}

func TestFullParamsAreValid(t *testing.T) {
	if err := fullParams().Validate(); err != nil {
		t.Fatalf("tham số đầy đủ mà bị từ chối: %v", err)
	}
}

func TestParamsMissingAnyPartAreRefused(t *testing.T) {
	// EACH REFUSAL IS ASSERTED SEPARATELY, by its sentinel. A single "invalid" error would let
	// two of these three be dropped without a test turning red, and the one most likely to be
	// dropped is NextStep — it is the one that looks like a nicety.
	cases := []struct {
		name   string
		mutate func(*NotificationParams)
		want   error
	}{
		{"thiếu mã tra cứu", func(p *NotificationParams) { p.LookupCode = "" }, ErrMissingLookupCode},
		{"mã tra cứu chỉ có khoảng trắng", func(p *NotificationParams) { p.LookupCode = "   " }, ErrMissingLookupCode},
		{"thiếu mốc", func(p *NotificationParams) { p.StatusLabel = "" }, ErrMissingStatusLabel},
		{"thiếu việc tiếp theo", func(p *NotificationParams) { p.NextStep = "" }, ErrMissingNextStep},
		{"việc tiếp theo chỉ có khoảng trắng", func(p *NotificationParams) { p.NextStep = "\t \n" }, ErrMissingNextStep},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := fullParams()
			c.mutate(&p)
			if err := p.Validate(); !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
		})
	}
}

func TestParamsNextStepRepeatingLabelIsRefused(t *testing.T) {
	// THE LOOPHOLE IN THE PREVIOUS TEST, CLOSED. "Say what happens next" is trivially satisfied by
	// copying "what changed" into it, which produces "Đã xử lý. Đã xử lý." — worse than the bare
	// message rule 10 invariant 6 refuses, and it passes every emptiness check.
	//
	// The capitalisation differs on purpose: the copy somebody makes is the copy with the case
	// changed, and a case-sensitive comparison would wave it through.
	p := fullParams()
	p.StatusLabel = "Đã xử lý"
	p.NextStep = "đã XỬ LÝ"

	if err := p.Validate(); !errors.Is(err, ErrNextStepRepeatsLabel) {
		t.Fatalf("lỗi = %v, muốn ErrNextStepRepeatsLabel", err)
	}
}

func TestParamsJSONCarriesExactlyTheThreeContractKeys(t *testing.T) {
	// THE KEYS ARE THE CONTRACT WITH TWO THINGS AT ONCE: the CHECK constraint on `tham_so` in
	// migration 0004 names them, and the approved ZNS template is keyed by them. Renaming one
	// here breaks a constraint at the bottom of a transaction, which is the least readable place
	// for it to break.
	//
	// The ABSENT keys matter more than the present ones: an extra field added to
	// NotificationParams would ship in every message to every citizen, and personal data leaving for
	// an external service is rule 3's second stop condition — not something to discover from a
	// diff.
	b, err := fullParams().JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("không phải JSON: %q", string(b))
	}
	want := map[string]bool{"ma_tra_cuu": true, "moc_nhan": true, "viec_tiep_theo": true}
	for key := range raw {
		if !want[key] {
			t.Errorf("trường ngoài khai báo lọt vào thông điệp gửi ra ngoài: %q — %s", key, string(b))
		}
	}
	if len(raw) != len(want) {
		t.Errorf("số khoá = %d, muốn %d: %s", len(raw), len(want), string(b))
	}
}

func TestInvalidParamsYieldNoJSONToWrite(t *testing.T) {
	// A caller that swallowed the validation error upstream must not be handed a serialisable
	// value it can still write. Returning `{}` with the error would give it exactly that, and the
	// CHECK constraint would then be the only thing left standing — a database error at the bottom
	// of a transaction, with no explanation of which rule was broken.
	p := fullParams()
	p.NextStep = ""

	b, err := p.JSON()
	if !errors.Is(err, ErrMissingNextStep) {
		t.Fatalf("lỗi = %v, muốn ErrMissingNextStep", err)
	}
	if b != nil {
		t.Errorf("từ chối mà vẫn trả %d byte để ghi", len(b))
	}
}

// --- (2) the deduplication key ----------------------------------------------------------------

const (
	testReportCode   = "PA-2026-7F3K9Q"
	testCitizenCode  = "cd-01JCONGDANMAUTHUNGHIEM"
	otherCitizenCode = "cd-01JCONGDANKHACTHUNGHIE"
)

func testKey(t *testing.T, milestone string, round int, recipient string) string {
	t.Helper()
	k, err := SendKey(SubjectCitizenReport, testReportCode, milestone, round, recipient, ChannelZaloZNS)
	if err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	return k
}

func TestSendKeyRecipeIsPinned(t *testing.T) {
	// THE EXACT BYTES, NOT A PROPERTY OF THEM. Every key already stored in `khoa_lan_gui` was produced
	// by this recipe, and the UNIQUE (tenant_id, khoa_lan_gui) constraint is what turns a redelivery of
	// an OLD fact into a no-op. A refactor that reorders the components, swaps the separator, renders
	// `lan` differently or spells a value set in English produces a key that matches nothing on disk —
	// and the citizen is told the same thing twice. The properties below (same fact → same key,
	// different fact → different key) all still pass after such a change; only this does not.
	got := testKey(t, "da-dong", 2, testCitizenCode)
	const want = "phieu-phan-anh|PA-2026-7F3K9Q|da-dong|2|cd-01JCONGDANMAUTHUNGHIEM|zalo-zns"
	if got != want {
		t.Fatalf("SendKey = %q, muốn %q — công thức khoá đã lưu bị đổi", got, want)
	}
}

func TestSendKeySameFactSameKey(t *testing.T) {
	// THE PROPERTY THE WHOLE IDEMPOTENCY RESTS ON. Queues deliver at least once, so the same fact
	// arrives twice; the second arrival must land on the same key and become a no-op at the
	// UNIQUE (tenant_id, khoa_lan_gui) constraint.
	//
	// It is NOT a test of determinism-in-general: it is a test that the key is built from the
	// FACT. A key that mixed in a message id, a timestamp or a random value would differ between
	// these two calls and send the citizen the same message twice.
	if a, b := testKey(t, "da-dong", 1, testCitizenCode), testKey(t, "da-dong", 1, testCitizenCode); a != b {
		t.Fatalf("hai lần giao cùng một sự kiện cho ra hai khoá: %q vs %q", a, b)
	}
}

func TestSendKeyDistinguishesEachComponent(t *testing.T) {
	// EACH COMPONENT IS ASSERTED TO CHANGE THE KEY, because dropping any one of them is a
	// plausible-looking simplification with a specific victim:
	//
	//	moc      two different transitions on one record collapse into one message
	//	lan      a petition closed, reopened and closed AGAIN (ADR 0008) owes a second
	//	         notification; without `lan` it is swallowed as a duplicate, and swallowing looks
	//	         exactly like correct deduplication from the inside
	//	người    two recipients of one transition collapse into one message
	base := testKey(t, "da-dong", 1, testCitizenCode)

	others := map[string]string{
		"mốc khác":        testKey(t, "dang-xu-ly", 1, testCitizenCode),
		"lần khác":        testKey(t, "da-dong", 2, testCitizenCode),
		"người nhận khác": testKey(t, "da-dong", 1, otherCitizenCode),
	}
	for name, k := range others {
		if k == base {
			t.Errorf("%s vẫn cho ra cùng một khoá %q — một thông báo thật sẽ bị nuốt", name, k)
		}
	}
}

func TestSendKeyDistinguishesChannel(t *testing.T) {
	// A second channel is a second delivery, not a repeat of the first. Asserted separately
	// because there is only one channel today, which makes this the component most likely to be
	// dropped as "unused" — and the day a second channel exists, the mistake is invisible.
	a, err := SendKey(SubjectCitizenReport, testReportCode, "da-dong", 1, testCitizenCode, ChannelZaloZNS)
	if err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	b, err := SendKey(SubjectCitizenReport, testReportCode, "da-dong", 1, testCitizenCode, Channel("kenh-khac"))
	if err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	if a == b {
		t.Fatalf("hai kênh cho ra cùng một khoá %q", a)
	}
}

func TestSendKeyComponentWithSeparatorIsRefused(t *testing.T) {
	// REFUSE, DO NOT ESCAPE. Two different escaping conventions produce two different keys for one
	// fact, and the drift surfaces as duplicate messages to real people months later, with the
	// escaping function nowhere near the symptom.
	//
	// The refusal is also the only thing standing between a component and key COLLISION: with a
	// separator inside a value, ("a|b", "c") and ("a", "b|c") join to the same string — two
	// different facts, one key, and the second notification silently never sent.
	_, err := SendKey(SubjectCitizenReport, "PA|2026", "da-dong", 1, testCitizenCode, ChannelZaloZNS)
	if !errors.Is(err, ErrKeyHasSeparator) {
		t.Fatalf("lỗi = %v, muốn ErrKeyHasSeparator", err)
	}
}

func TestSendKeyCarriesNoPersonalData(t *testing.T) {
	// THE KEY IS STORED, INDEXED, AND READ BY WHOEVER DEBUGS THE SEND PATH. Every component of it
	// is either a code or an opaque id (rule 3, invariant 2 allows a business code), and this
	// pins that: the only inputs are the ones passed in, so a future version that reached for the
	// recipient's number "to make the key more precise" has to change this signature first.
	k := testKey(t, "da-dong", 1, testCitizenCode)
	for _, part := range strings.Split(k, keySeparator) {
		if part == "" {
			t.Fatalf("khoá có thành phần rỗng: %q", k)
		}
	}
	if n := len(strings.Split(k, keySeparator)); n != 6 {
		t.Fatalf("khoá có %d thành phần, muốn 6: %q", n, k)
	}
}
