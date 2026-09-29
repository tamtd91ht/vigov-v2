package domain

// System messages — the sentences this service prints, which a commune may reword
// (14-cau-hinh §7 "Lời hệ thống", ADR 0024 §Phụ, *Bổ sung 29/09/2026*).
//
// A COPY OF service-finance/internal/domain/system_message.go, NOT AN IMPORT (rule 2, forbidden #1).
// Only the catalogue differs: these are the 38 `report.*` keys — the captions of the exported report
// and the titles of the report notifications, both of which this service emits.
//
// THE SHIPPED DEFAULT LIVES HERE, IN CODE, AND NOWHERE ELSE. The table `system_message_override`
// holds only a commune's own wording, and only once it has reworded a key (migration 0003). So a
// commune that never touched the screen reads exactly the sentence below, and a default improved in
// a release reaches every commune that has not reworded it.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ShippedMessage is one sentence the software ships. Key and DefaultText are the contract; the
// description is what the configuration card prints under the key.
type ShippedMessage struct {
	Key         string
	Description string
	DefaultText string
}

// shippedMessages is the closed set. It MUST agree with the CHECK `system_message_override_key_known`
// in migration 0003, in the same order (TestCatalogueAgreesWithMigrationCheck). Order is the order the
// configuration screen lists them in.
//
// WHERE EACH COLUMN COMES FROM (ADR 0024, Bổ sung 29/09/2026):
//
//   - KEYS: the 32 of docs/ui-ux/14-cau-hinh.md:226-261 plus the six fiscal keys of the requirement
//     repository (../vigov-require/apps/api/app/modules/admin/messages.py:278-316). Spelled as there;
//     a contract with the configuration screen and the database CHECK — never renamed.
//   - DEFAULT TEXT: the requirement's shipped `content`, verbatim (messages.py:98-316 for 35 keys,
//     :455-472 for report.notification.week, report.notification.month and
//     report.metric.economy.total). The specification shows one default (`Cần xử lý ngay`, §7 card)
//     and it agrees.
//   - DESCRIPTION: the specification's column where it has the key (its wording wins over the
//     requirement's "Nhãn chỉ số: …" phrasing); the requirement's for the six fiscal keys the
//     specification does not list, trailing full stop dropped to match.
//
// ORDER: the requirement's — title, the blocks (fiscal among them), then each block's metrics, then
// the two notification titles.
var shippedMessages = []ShippedMessage{
	{Key: "report.title", Description: "Tiêu đề trang đầu của báo cáo xuất ra PDF, Excel và PowerPoint", DefaultText: "Báo cáo điều hành"},

	{Key: "report.block.tasks", Description: "Tên khối nhiệm vụ", DefaultText: "Nhiệm vụ"},
	{Key: "report.block.register", Description: "Tên khối văn bản và đơn thư", DefaultText: "Văn bản và đơn thư"},
	{Key: "report.block.budget", Description: "Tên khối giải ngân", DefaultText: "Giải ngân ngân sách"},
	{Key: "report.block.feedback", Description: "Tên khối phản ánh", DefaultText: "Phản ánh của người dân"},
	{Key: "report.block.economy", Description: "Tên khối kinh tế và tài nguyên", DefaultText: "Kinh tế và tài nguyên"},
	{Key: "report.block.alerts", Description: "Tên khối cảnh báo trên báo cáo xuất ra", DefaultText: "Cần xử lý ngay"},
	{Key: "report.block.ranking", Description: "Tên bảng xếp hạng bộ phận", DefaultText: "Xếp hạng bộ phận"},
	{Key: "report.block.fiscal", Description: "Tên khối thu - chi ngân sách trên báo cáo xuất ra", DefaultText: "Thu - chi ngân sách xã"},

	{Key: "report.metric.tasks.open", Description: "Nhiệm vụ đang thực hiện", DefaultText: "Đang thực hiện"},
	{Key: "report.metric.tasks.overdue", Description: "Nhiệm vụ quá hạn", DefaultText: "Quá hạn"},
	{Key: "report.metric.tasks.done", Description: "Nhiệm vụ hoàn thành trong kỳ", DefaultText: "Hoàn thành trong kỳ"},
	{Key: "report.metric.tasks.on_time_percent", Description: "Tỷ lệ nhiệm vụ hoàn thành đúng hạn", DefaultText: "Tỷ lệ đúng hạn"},

	{Key: "report.metric.register.arrived", Description: "Văn bản, đơn thư đến trong kỳ", DefaultText: "Đến trong kỳ"},
	{Key: "report.metric.register.open", Description: "Văn bản, đơn thư chưa xử lý xong", DefaultText: "Chưa xử lý xong"},
	{Key: "report.metric.register.overdue", Description: "Văn bản, đơn thư quá hạn xử lý", DefaultText: "Quá hạn xử lý"},
	{Key: "report.metric.register.petition_arrived", Description: "Đơn thư công dân đến trong kỳ", DefaultText: "Đơn thư đến trong kỳ"},

	{Key: "report.metric.budget.disbursed_percent", Description: "Phần trăm giải ngân trên kế hoạch năm", DefaultText: "Tỷ lệ giải ngân"},
	{Key: "report.metric.budget.time_percent", Description: "Phần trăm thời gian năm ngân sách đã trôi qua", DefaultText: "Thời gian đã trôi qua"},
	{Key: "report.metric.budget.behind", Description: "Số dự án giải ngân chậm so với thời gian", DefaultText: "Dự án chậm tiến độ"},
	{Key: "report.metric.budget.open_issues", Description: "Số vướng mắc giải ngân chưa xử lý xong", DefaultText: "Vướng mắc chưa gỡ"},
	{Key: "report.metric.budget.disbursed_amount", Description: "Số tiền đã giải ngân trong năm", DefaultText: "Đã giải ngân"},

	{Key: "report.metric.feedback.received", Description: "Phản ánh tiếp nhận trong kỳ", DefaultText: "Tiếp nhận"},
	{Key: "report.metric.feedback.open", Description: "Phản ánh đang xử lý", DefaultText: "Đang xử lý"},
	{Key: "report.metric.feedback.on_time_percent", Description: "Tỷ lệ phản ánh xử lý đúng hạn", DefaultText: "Đúng hạn"},
	{Key: "report.metric.feedback.late", Description: "Số phản ánh xử lý trễ hạn", DefaultText: "Trễ hạn"},
	{Key: "report.metric.feedback.rating", Description: "Điểm hài lòng trung bình của người dân", DefaultText: "Điểm hài lòng"},

	{Key: "report.metric.economy.enterprises", Description: "Số doanh nghiệp trên địa bàn", DefaultText: "Doanh nghiệp"},
	{Key: "report.metric.economy.household_businesses", Description: "Số hộ kinh doanh cá thể", DefaultText: "Hộ kinh doanh"},
	{Key: "report.metric.economy.new_in_period", Description: "Cơ sở kinh doanh thành lập mới trong kỳ", DefaultText: "Thành lập mới trong kỳ"},
	{Key: "report.metric.economy.total", Description: "Tổng số đối tượng trong danh mục bản đồ", DefaultText: "Tổng tài nguyên trên bản đồ"},

	{Key: "report.metric.fiscal.revenue_percent", Description: "Nhãn chỉ số: tổng thu đã đạt bao nhiêu phần trăm dự toán năm", DefaultText: "Thu đạt so với dự toán"},
	{Key: "report.metric.fiscal.revenue_amount", Description: "Nhãn chỉ số: tổng thu ngân sách luỹ kế, quy về đồng", DefaultText: "Tổng thu ngân sách"},
	{Key: "report.metric.fiscal.expense_percent", Description: "Nhãn chỉ số: tổng chi đã đạt bao nhiêu phần trăm dự toán năm", DefaultText: "Chi đạt so với dự toán"},
	{Key: "report.metric.fiscal.expense_amount", Description: "Nhãn chỉ số: tổng chi ngân sách luỹ kế, quy về đồng", DefaultText: "Tổng chi ngân sách"},
	{Key: "report.metric.fiscal.balance", Description: "Nhãn chỉ số: tổng thu trừ tổng chi. Số âm là bội chi, không tự động là tin xấu", DefaultText: "Cân đối thu - chi"},

	{Key: "report.notification.week", Description: "Tiêu đề thông báo gửi lãnh đạo sáng thứ Hai, kèm các con số chính", DefaultText: "Báo cáo điều hành tuần đã sẵn sàng"},
	{Key: "report.notification.month", Description: "Tiêu đề thông báo gửi lãnh đạo đầu tháng", DefaultText: "Báo cáo điều hành tháng đã sẵn sàng"},
}

// ShippedMessages returns a copy of the catalogue, so no caller can edit the defaults in place.
func ShippedMessages() []ShippedMessage {
	out := make([]ShippedMessage, len(shippedMessages))
	copy(out, shippedMessages)
	return out
}

// LookupShippedMessage finds one key. A key outside the catalogue answers false — the caller turns
// that into 404, never into a new row.
func LookupShippedMessage(key string) (ShippedMessage, bool) {
	for _, m := range shippedMessages {
		if m.Key == key {
			return m, true
		}
	}
	return ShippedMessage{}, false
}

// MessageOverride is a commune's live wording of one key, as stored.
type MessageOverride struct {
	ID        string
	Key       string
	Text      string
	UpdatedAt time.Time
	UpdatedBy string // staff business code, never the internal id (rule 6, invariant 8)
}

// SystemMessage is what a reader gets: the default, the text in force, and whether the commune
// has reworded it.
type SystemMessage struct {
	Key         string
	Description string
	DefaultText string
	CurrentText string
	Overridden  bool
	UpdatedAt   *time.Time
	UpdatedBy   string
}

// ResolveMessage is THE fallback rule, in one place: no live override means the default, and an
// override for another key is a programming error, not a wording. The configuration list today, and
// the report export and notifications when they are built, go through here, so "what does this
// commune see" has exactly one answer.
func ResolveMessage(m ShippedMessage, o *MessageOverride) SystemMessage {
	out := SystemMessage{
		Key:         m.Key,
		Description: m.Description,
		DefaultText: m.DefaultText,
		CurrentText: m.DefaultText,
	}
	if o == nil || o.Key != m.Key {
		return out
	}
	at := o.UpdatedAt
	out.CurrentText = o.Text
	out.Overridden = true
	out.UpdatedAt = &at
	out.UpdatedBy = o.UpdatedBy
	return out
}

// MessageTextMax is the bound in characters (runes), mirrored by the CHECK in migration 0003.
const MessageTextMax = 1000

var (
	ErrUnknownMessageKey = errors.New("system_message: khoá câu không thuộc phân hệ này")

	// ErrMessageTextEmpty — an empty wording is refused rather than stored: "no sentence" is not a
	// state this screen offers, and an empty caption prints a block with no name on a report the
	// leadership signs. Going back to the software's sentence is the revert route.
	ErrMessageTextEmpty    = errors.New("system_message: nội dung câu trống — muốn dùng lại câu mặc định thì bấm Khôi phục câu mặc định")
	ErrMessageTextTooLong  = errors.New("system_message: nội dung câu quá dài")
	ErrMessageTextControl  = errors.New("system_message: nội dung câu chứa ký tự điều khiển hoặc ký tự vô hình")
	ErrMessageTextMarkup   = errors.New("system_message: nội dung câu không được chứa dấu < hoặc >")
	ErrMessageActorMissing = errors.New("system_message: thiếu mã cán bộ thực hiện")
)

// NormalizeMessageText trims and validates one wording.
//
// CONTROL (Cc) AND FORMAT (Cf) CHARACTERS ARE BOTH REFUSED. Cc covers line breaks and escape
// sequences; Cf covers zero-width and bidirectional-override characters, which make a caption read
// differently from what it contains — on a page a public authority sends upward under its name.
//
// `<` AND `>` ARE REFUSED so the stored sentence is text by construction; no renderer has to be the
// only defence (rule 13, invariant 3).
func NormalizeMessageText(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrMessageTextEmpty
	}
	if n := len([]rune(s)); n > MessageTextMax {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMessageTextTooLong, MessageTextMax)
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrMessageTextControl
		}
		if r == '<' || r == '>' {
			return "", ErrMessageTextMarkup
		}
	}
	return s, nil
}

// IsMessageInputError reports whether err is a refusal of what the client sent (400), as opposed to
// a failure. Listed explicitly: a default of "unknown means client error" turns an outage into a 400
// that a client retries with different input forever.
func IsMessageInputError(err error) bool {
	for _, e := range []error{ErrMessageTextEmpty, ErrMessageTextTooLong, ErrMessageTextControl, ErrMessageTextMarkup} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
