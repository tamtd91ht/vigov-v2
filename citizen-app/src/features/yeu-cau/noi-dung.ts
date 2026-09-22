/**
 * CHỮ CỦA BỀ MẶT YÊU CẦU — một tệp, để người duyệt câu chữ đọc được mà không phải mở JSX.
 *
 * ⚠ BA THỨ KHÔNG CÂU NÀO Ở ĐÂY ĐƯỢC NÓI, VÀ MỖI THỨ MỘT LÝ DO ĐO ĐƯỢC:
 *
 *   1. **KHÔNG MỘT CAM KẾT THỜI GIAN NÀO.** Không "gọi lại trong 30 giây", không "trả lời trong
 *      24 giờ". Không ai cam kết những con số ấy, và người phát hiện ra là người đang chờ. Đây là
 *      cùng một luật với `rules/critical/10-citizen-commitment.md`: một thời hạn in ra màn hình
 *      là một cam kết của bên phát hành, không phải một câu trấn an.
 *
 *   2. **KHÔNG HỨA CHẮC CÓ TIN ZNS.** Máy chủ CÓ gửi ZNS xác nhận, nhưng nó có trần và có thể
 *      tắt (`zns_da_gui.ket_qua` có hẳn một giá trị `bo_qua_vuot_tran`). Nên câu chữ phải đúng ở
 *      CẢ HAI ca: có tin và không có tin. "Nếu bạn nhận được…" chứ không "Bạn sẽ nhận được…".
 *
 *   3. **KHÔNG VIẾT LẠI CÂU LỖI CỦA MÁY CHỦ.** 400 · 429 · 503 đều kèm một câu tiếng Việt đã nói
 *      việc cần làm tiếp, và câu ấy mang những con số của máy chủ (trần 3 lượt/24 giờ). Viết lại
 *      ở đây là dựng chỗ thứ hai giữ cùng một câu, và ngày trần đổi thì chỗ thứ hai vẫn nói số
 *      cũ. `CAU_LUI_TU_CHOI` chỉ dùng khi máy chủ KHÔNG nói gì.
 */

export const TU_VAN = {
  tieu_de: "Tư vấn và báo giá",
  dan_nhap:
    "Chọn dòng giải pháp bạn quan tâm và quy mô doanh nghiệp, rồi gửi cho chúng tôi. Bạn có thể ghi thêm câu hỏi của mình ở ô bên dưới.",

  // MỜI ĐĂNG NHẬP — HIỆN THAY CHO BIỂU MẪU, KHÔNG HIỆN CẠNH NÓ.
  //
  // Để biểu mẫu ra rồi báo "chưa đăng nhập" lúc bấm gửi là một lần người dùng gõ xong bị mất
  // chữ. Với người vừa gõ một đoạn câu hỏi, đó là lần cuối họ gõ.
  can_dang_nhap_tieu_de: "Bạn cần đăng nhập trước",
  can_dang_nhap:
    "Để gửi yêu cầu, chúng tôi cần biết gọi lại cho ai. Bạn đăng nhập bằng một lần chạm với chính số Zalo đang dùng — không có ô nhập số, không có mã sáu số nào phải gõ.",
  nut_toi_dang_nhap: "Tới màn đăng nhập",
  van_goi_duoc:
    "Hoặc bạn gọi thẳng hotline, không cần đăng nhập gì cả — đây là đường nhanh nhất nếu bạn đang cần gấp.",

  cau_hoi_quan_tam: "Bạn đang quan tâm dòng giải pháp nào?",
  huong_dan_quan_tam: "Chọn một hoặc nhiều mục. Bấm lại một mục để bỏ chọn.",
  cau_hoi_quy_mo: "Doanh nghiệp của bạn có bao nhiêu nhân sự?",
  nhan_ghi_chu: "Bạn muốn hỏi thêm điều gì? (không bắt buộc)",
  goi_y_ghi_chu: "Ví dụ: số máy nhánh cần dùng, thời điểm bạn tiện nghe máy…",

  // ĐẾM NGƯỢC THEO KÝ TỰ, KHÔNG THEO BYTE — xem `TRAN_GHI_CHU` trong hợp đồng.
  con_lai: (con: number) => `Còn ${con} ký tự`,
  qua_dai: (qua: number) => `Bạn đang viết dài hơn ${qua} ký tự so với mức cho phép. Rút ngắn giúp chúng tôi nhé.`,

  chua_chon_gi: "Bạn hãy chọn ít nhất một dòng giải pháp để chúng tôi biết nên chuẩn bị gì.",
  qua_nhieu: (toi_da: number) =>
    `Bạn chỉ chọn được tối đa ${toi_da} dòng giải pháp trong một lần gửi. Bỏ bớt một mục rồi gửi tiếp nhé.`,

  nut_gui: "Gửi yêu cầu tư vấn",
  nut_goi_lai: "Đề nghị gọi lại cho tôi",
  dang_gui: "Đang gửi…",

  // TRẦN GỌI LẠI NÓI TRƯỚC, NGAY TRÊN NÚT — không để người dùng gặp nó lần đầu dưới dạng một câu
  // từ chối. Con số 3 ở đây là con số chủ dự án đã chốt (22/09/2026) và máy chủ đang thi hành.
  tran_goi_lai: "Bạn đề nghị gọi lại được tối đa 3 lần trong 24 giờ.",

  xong_tieu_de: "Chúng tôi đã nhận yêu cầu của bạn",
  nhan_ma: "Mã yêu cầu",
  // ⚠ CÂU NÀY ĐÚNG Ở CẢ HAI CA — có tin ZNS và không có. Xem khối đầu tệp.
  xong_xem_lai:
    "Bạn xem lại yêu cầu này bất cứ lúc nào ở màn Yêu cầu. Nếu bạn nhận được một tin ZNS xác nhận từ chúng tôi thì đó cũng là yêu cầu này; không nhận được tin cũng không sao, yêu cầu vẫn đã vào hệ thống.",
  nut_xem_yeu_cau: "Xem yêu cầu của tôi",
  nut_gui_tiep: "Gửi một yêu cầu khác",

  phien_het_han:
    "Phiên đăng nhập của bạn đã hết hạn nên yêu cầu chưa gửi được. Bạn hãy đăng nhập lại rồi gửi lại — phần bạn vừa chọn và vừa gõ vẫn còn ở đây.",
  cau_lui_tu_choi:
    "Yêu cầu chưa gửi được. Bạn hãy thử lại, hoặc gọi hotline để chúng tôi hỗ trợ ngay.",
  khong_goi_duoc:
    "Yêu cầu chưa gửi được. Bạn hãy kiểm tra kết nối mạng rồi bấm gửi lại — phần bạn vừa gõ vẫn còn ở đây.",
  chua_khai_host:
    "Bản dựng này chưa được khai địa chỉ máy chủ, nên chưa gửi được yêu cầu. Người dựng bản cần đặt biến VIGOV_API_HOST rồi dựng lại.",
} as const;

export const YEU_CAU_CUA_TOI = {
  tieu_de: "Yêu cầu của tôi",
  dan_nhap: "Những yêu cầu bạn đã gửi, mới nhất trước.",

  can_dang_nhap_tieu_de: "Bạn cần đăng nhập để xem",
  can_dang_nhap:
    "Danh sách này chỉ hiện yêu cầu của chính bạn, nên chúng tôi cần biết bạn là ai. Bạn đăng nhập bằng một lần chạm với số Zalo đang dùng.",
  nut_toi_dang_nhap: "Tới màn đăng nhập",

  dang_doc: "Đang đọc danh sách…",
  // DANH SÁCH RỖNG CÓ MỘT CÂU, KHÔNG PHẢI MỘT MÀN TRẮNG. Một màn trắng và một màn hỏng trông
  // giống hệt nhau, và người lớn tuổi đọc cả hai thành "app hỏng".
  rong: "Bạn chưa gửi yêu cầu nào. Khi bạn gửi, yêu cầu sẽ hiện ở đây kèm tình trạng xử lý.",
  nut_gui_moi: "Gửi yêu cầu tư vấn",
  nut_doc_lai: "Đọc lại danh sách",

  phien_het_han:
    "Phiên đăng nhập của bạn đã hết hạn. Bạn hãy đăng nhập lại để xem yêu cầu của mình.",
  khong_doc_duoc:
    "Chưa đọc được danh sách. Bạn hãy kiểm tra kết nối mạng rồi bấm đọc lại.",
  chua_khai_host:
    "Bản dựng này chưa được khai địa chỉ máy chủ, nên chưa đọc được danh sách. Người dựng bản cần đặt biến VIGOV_API_HOST rồi dựng lại.",

  nhan_ma: "Mã yêu cầu",
  nhan_gui_luc: "Gửi lúc",
} as const;

/**
 * CÂU NÓI RA RẰNG PHIÊN KHÔNG SỐNG QUA MỘT LẦN ĐÓNG APP.
 *
 * Nói ra là bắt buộc, không phải lịch sự: người dùng mở lại app hôm sau, thấy mình đã đăng xuất
 * và kết luận rằng app quên mất họ — hoặc tệ hơn, rằng yêu cầu họ gửi cũng mất theo. Câu này nói
 * cả hai vế: phiên mất, yêu cầu thì không.
 */
export const PHIEN_KHONG_LUU =
  "Ứng dụng không ghi phiên đăng nhập xuống máy bạn, nên khi bạn đóng ứng dụng thì phiên mất đi và lần sau bạn đăng nhập lại bằng một lần chạm. Những yêu cầu bạn đã gửi thì vẫn còn — chúng nằm ở máy chủ, không nằm trên máy bạn.";
