package http

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
)

// readImportSheet reads the first sheet of an uploaded workbook through core/xlsx.ReadSheet — the one
// hardened reader every import shares — or answers the refusal of the FILE and returns false.
//
// AN EMPTY SHEET IS NOT A FILE REFUSAL: it returns (nil, true), and the caller's domain reader reports
// "Tệp không có dòng nào." as a content error, the way parseResidentialUnitFile does.
//
// EVERY SENTINEL HAS ONE FIXED SENTENCE, the residential-unit import's. err.Error() is never written
// anywhere a client or a log sees it: nothing core/xlsx returns may carry file content, and this layer
// does not lean on that promise either (rule 3, forbidden #3).
func (h *Handler) readImportSheet(w http.ResponseWriter, r *http.Request, data []byte, what string) ([][]string, bool) {
	cells, err := xlsx.ReadSheet(bytes.NewReader(data), int64(len(data)), xlsx.DefaultLimits)
	switch {
	case err == nil:
		return cells, true
	case errors.Is(err, xlsx.ErrEmptySheet):
		return nil, true
	case errors.Is(err, xlsx.ErrTooLarge), errors.Is(err, xlsx.ErrTooManyRows):
		writeFileTooLarge(w)
	case errors.Is(err, xlsx.ErrMacroEnabled):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp có macro (.xlsm) không được nhận. Hãy lưu lại dưới dạng Excel Workbook (.xlsx) không macro.", "")
	case errors.Is(err, xlsx.ErrNotXLSX):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp không phải bảng tính Excel .xlsx hợp lệ (hoặc đang đặt mật khẩu). Hãy tải tệp mẫu và nhập vào đó.", "")
	case errors.Is(err, xlsx.ErrMalformed):
		httpx.WriteError(w, http.StatusBadRequest, "malformed_file",
			"Tệp Excel bị hỏng hoặc không đọc trọn được. Hãy mở tệp bằng Excel, lưu lại rồi gửi lại.", "")
	default:
		// No `err` in the log line: it may wrap a read of the request body, and nothing more is known.
		h.d.Log.Error(what+": đọc tệp lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return nil, false
}
