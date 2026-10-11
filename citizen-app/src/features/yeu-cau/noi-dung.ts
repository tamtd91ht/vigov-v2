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
    "Cho chúng tôi biết nhu cầu của doanh nghiệp, chuyên viên ViHAT Group sẽ liên hệ tư vấn.",

  // MỜI ĐĂNG NHẬP — HIỆN THAY CHO BIỂU MẪU, KHÔNG HIỆN CẠNH NÓ.
  //
  // Để biểu mẫu ra rồi báo "chưa đăng nhập" lúc bấm gửi là một lần người dùng gõ xong bị mất
  // chữ. Với người vừa gõ một đoạn câu hỏi, đó là lần cuối họ gõ.
  can_dang_nhap_tieu_de: "Đăng nhập để gửi yêu cầu",
  can_dang_nhap:
    "Đăng nhập bằng số Zalo để chuyên viên liên hệ đúng số của bạn và để bạn theo dõi tình trạng yêu cầu.",
  nut_toi_dang_nhap: "Đăng nhập",
  van_goi_duoc:
    "Cần hỗ trợ ngay? Gọi hotline của chúng tôi.",

  cau_hoi_quan_tam: "Bạn đang quan tâm dòng giải pháp nào?",
  huong_dan_quan_tam: "Có thể chọn nhiều mục.",
  cau_hoi_quy_mo: "Doanh nghiệp của bạn có bao nhiêu nhân sự?",
  nhan_ghi_chu: "Ghi chú (không bắt buộc)",
  goi_y_ghi_chu: "Số máy nhánh cần dùng, thời gian thuận tiện để liên hệ…",

  // ĐẾM NGƯỢC THEO KÝ TỰ, KHÔNG THEO BYTE — xem `TRAN_GHI_CHU` trong hợp đồng.
  con_lai: (con: number) => `Còn ${con} ký tự`,
  qua_dai: (qua: number) => `Vượt quá ${qua} ký tự so với giới hạn.`,

  chua_chon_gi: "Vui lòng chọn ít nhất một giải pháp.",
  qua_nhieu: (toi_da: number) =>
    `Chọn tối đa ${toi_da} giải pháp cho mỗi yêu cầu.`,

  nut_gui: "Gửi yêu cầu tư vấn",
  nut_goi_lai: "Đề nghị gọi lại cho tôi",
  dang_gui: "Đang gửi…",

  // TRẦN GỌI LẠI NÓI TRƯỚC, NGAY TRÊN NÚT — không để người dùng gặp nó lần đầu dưới dạng một câu
  // từ chối. Con số 3 ở đây là con số chủ dự án đã chốt (22/09/2026) và máy chủ đang thi hành.
  tran_goi_lai: "Tối đa 3 lần đề nghị gọi lại trong 24 giờ.",

  xong_tieu_de: "Đã gửi yêu cầu thành công",
  nhan_ma: "Mã yêu cầu",
  // ⚠ CÂU NÀY ĐÚNG Ở CẢ HAI CA — có tin ZNS và không có. Xem khối đầu tệp.
  xong_xem_lai:
    "Theo dõi tình trạng xử lý tại mục Yêu cầu của tôi.",
  nut_xem_yeu_cau: "Xem yêu cầu của tôi",
  nut_gui_tiep: "Gửi yêu cầu khác",

  phien_het_han:
    "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại để gửi — nội dung bạn đã nhập vẫn được giữ.",
  cau_lui_tu_choi:
    "Chưa gửi được yêu cầu. Vui lòng thử lại hoặc gọi hotline.",
  khong_goi_duoc:
    "Không kết nối được. Vui lòng kiểm tra mạng và thử lại.",
  chua_khai_host:
    "Hệ thống đang gián đoạn. Vui lòng thử lại sau.",
} as const;

export const YEU_CAU_CUA_TOI = {
  tieu_de: "Yêu cầu của tôi",
  dan_nhap: "Các yêu cầu tư vấn bạn đã gửi.",

  can_dang_nhap_tieu_de: "Đăng nhập để xem yêu cầu",
  can_dang_nhap:
    "Đăng nhập bằng số Zalo để xem các yêu cầu bạn đã gửi.",
  nut_toi_dang_nhap: "Đăng nhập",

  dang_doc: "Đang tải…",
  // DANH SÁCH RỖNG CÓ MỘT CÂU, KHÔNG PHẢI MỘT MÀN TRẮNG. Một màn trắng và một màn hỏng trông
  // giống hệt nhau, và người lớn tuổi đọc cả hai thành "app hỏng".
  rong: "Bạn chưa có yêu cầu nào.",
  nut_gui_moi: "Gửi yêu cầu tư vấn",
  nut_doc_lai: "Tải lại",

  phien_het_han:
    "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
  khong_doc_duoc:
    "Không tải được danh sách. Vui lòng kiểm tra mạng và thử lại.",
  chua_khai_host:
    "Hệ thống đang gián đoạn. Vui lòng thử lại sau.",

  nhan_ma: "Mã yêu cầu",
  nhan_gui_luc: "Gửi lúc",
} as const;

/**
 * "CHAT VỚI CHUYÊN VIÊN" — the block on Liên hệ (07/10/2026).
 *
 * The explanation says WHY before Zalo's dialog does (Mini App policy 3.3.4): the specialist knows who is
 * writing and can call back if the chat drops. It names the floating "Chat Zalo" button (`NHAN_NUT_CHAT`,
 * pinned equal by the test) as the way to chat while sharing nothing — the refusal path, not an error.
 */
export const CHAT_SPECIALIST = {
  title: "Chat với chuyên viên",
  why: "Trò chuyện trực tiếp với chuyên viên ViHAT Group. Chuyên viên nhận số điện thoại và tên Zalo của bạn để hỗ trợ và liên hệ lại khi cần.",
  share_nothing: "Bạn cũng có thể nhắn tin qua nút “Chat Zalo”.",
  button: "Chat với chuyên viên",
  busy: "Đang kết nối…",
  refused:
    "Bạn có thể nhắn tin qua nút “Chat Zalo” hoặc gọi hotline.",
  opened_delivered: "Đã kết nối với chuyên viên.",
  // The POST failed but the chat opened (owner: the chat is the goal). Says what to do so it still works.
  opened_undelivered:
    "Đã mở cuộc trò chuyện. Vui lòng cho chuyên viên biết tên của bạn trong tin nhắn đầu tiên.",
  not_opened:
    "Không mở được cuộc trò chuyện. Vui lòng thử lại hoặc dùng nút “Chat Zalo”.",
} as const;

/**
 * "NHẬN ƯU ĐÃI QUA SMS" — the block on the home screen (07/10/2026).
 *
 * ⚠ THE CONSENT SENTENCE NAMES ALL THREE THINGS, AND NONE MAY BE DROPPED: which number (the Zalo phone number),
 *   what arrives (ViHAT's offers and promotions — it is ADVERTISING, said as such), and how to stop (the
 *   "Huỷ nhận ưu đãi SMS" button, here, any time). No frequency and no "you will receive" promise: nothing in
 *   this build fixes either.
 */
export const SMS_OFFERS = {
  title: "Nhận ưu đãi qua SMS",
  consent:
    "Nhận tin nhắn SMS về ưu đãi và chương trình khuyến mại của ViHAT Group qua số điện thoại Zalo của bạn. Bạn có thể huỷ bất cứ lúc nào bằng nút “Huỷ nhận ưu đãi SMS” tại đây.",
  subscribe: "Đăng ký nhận",
  unsubscribe: "Huỷ nhận ưu đãi SMS",
  busy: "Đang gửi…",
  status_subscribed: "Bạn đang nhận ưu đãi qua SMS.",
  done_subscribe:
    "Đăng ký nhận ưu đãi thành công.",
  done_unsubscribe: "Đã huỷ nhận ưu đãi qua SMS.",
  refused_subscribe:
    "Cần chia sẻ số điện thoại để đăng ký nhận ưu đãi.",
  refused_unsubscribe:
    "Cần chia sẻ số điện thoại để huỷ đăng ký. Bạn cũng có thể gọi hotline để được hỗ trợ.",
  session_failed: "Đăng nhập không thành công. Vui lòng thử lại.",
  expired:
    "Phiên đăng nhập đã hết hạn. Vui lòng thử lại.",
  network: "Không kết nối được. Vui lòng kiểm tra mạng và thử lại.",
} as const;
