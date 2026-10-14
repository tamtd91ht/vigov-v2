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
  // One line carries both the count and the cap (14/10/2026): a second "Tối đa N ký tự." line said the same
  // number twice to a screen reader.
  con_lai: (con: number, tran: number) => `Còn ${con} ký tự (tối đa ${tran} ký tự).`,
  qua_dai: (qua: number, tran: number) =>
    `Vượt quá ${qua} ký tự so với giới hạn ${tran} ký tự. Vui lòng rút gọn ghi chú.`,

  // NEUTRAL BEFORE THE FIRST TAP, RED ONLY AFTER IT (14/10/2026): an error on a form nobody has touched yet reads
  // as "you already did something wrong" to the person who just opened it.
  pick_one_hint: "Chọn ít nhất một giải pháp ở trên để gửi yêu cầu.",
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

  // ⚠ NO PROMISE THAT THE FORM IS KEPT (14/10/2026). Logging in again happens on Liên hệ, which unmounts this
  //   screen, and the form lives only in memory — drafts are deliberately never written to the device
  //   (`phase1-collects-nothing.test.ts`). The old sentence said "vẫn được giữ"; the person found out otherwise.
  session_expired_title: "Phiên đăng nhập đã hết hạn",
  phien_het_han:
    "Yêu cầu chưa được gửi vì phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại, sau đó chọn lại nội dung và gửi yêu cầu.",
  cau_lui_tu_choi:
    "Chưa gửi được yêu cầu. Vui lòng thử lại hoặc gọi hotline.",
  khong_goi_duoc:
    "Không kết nối được. Vui lòng kiểm tra mạng và thử lại.",
  // The server answered with a failure: the person's network is fine, so "kiểm tra mạng" would be wrong advice.
  server_error: "Máy chủ ViHAT đang gặp sự cố. Vui lòng thử lại sau ít phút.",
  // No answer in time, or an answer we could not read: the request may be recorded already. Resending blindly
  // makes a duplicate the specialist then has to sort out — so the list comes first.
  unconfirmed:
    "Chưa nhận được phản hồi từ máy chủ, nhưng yêu cầu có thể đã được ghi nhận. Vui lòng xem mục Yêu cầu của tôi trước khi gửi lại.",
  chua_khai_host:
    "Hệ thống đang gián đoạn. Vui lòng thử lại sau.",
} as const;

/**
 * PHOTOS ON "TƯ VẤN VÀ BÁO GIÁ" (owner 12/10/2026): up to 3, optional, sent to ViHAT Group's specialists with the
 * consultation request.
 *
 * ⚠ THE TWO QUESTIONS ARE SAID BEFORE ZALO'S OWN DIALOG (Mini App policy 3.3.4), and their words are quoted in the
 *   platform-call table (`zalo-api.ts`, rows `requestCameraPermission` and `openMediaPicker`, `intro_app`) — a
 *   reviewer reads the table and then looks for exactly these words on screen. `consult-photos.test.tsx` pins both.
 *
 * ⚠ AFTER THE REQUEST IS SENT, NO SENTENCE ASKS TO "CHOOSE ANOTHER PHOTO": the form is gone, so a photo that the
 *   server refused for good is sent another way — the floating "Chat Zalo" button (`NHAN_NUT_CHAT`), which every
 *   screen of the intro carries. A sentence pointing at a button that is not there is a dead end.
 */
export const CONSULT_PHOTOS = {
  title: "Ảnh gửi kèm (không bắt buộc)",
  hint: "Tối đa 3 ảnh, mỗi ảnh không quá 5 MB, định dạng JPEG, PNG hoặc WebP — ví dụ ảnh thiết bị, tài liệu hoặc màn hình bạn muốn chuyên viên xem.",
  take: "Chụp ảnh",
  pick: "Chọn ảnh",
  picking: "Đang mở…",
  photo_alt: (n: number) => `Ảnh ${n} bạn đã chọn`,
  no_preview: (n: number) => `Ảnh ${n} (không xem trước được)`,
  remove: (n: number) => `Bỏ ảnh ${n}`,
  full: "Đã đủ 3 ảnh. Bỏ một ảnh nếu bạn muốn thay ảnh khác.",

  // The purpose, then "Cho phép" / "Không" — once per button per screen opening.
  camera_question: "Cho phép ứng dụng dùng máy ảnh để chụp ảnh gửi kèm yêu cầu tư vấn?",
  library_question: "Cho phép ứng dụng mở thư viện ảnh để bạn chọn ảnh gửi kèm yêu cầu tư vấn?",
  question_why:
    "Ảnh chỉ được gửi tới chuyên viên ViHAT Group khi bạn bấm “Gửi yêu cầu tư vấn”. Ứng dụng chỉ nhận ảnh bạn chụp hoặc chọn, và không lưu ảnh trên máy.",
  allow: "Cho phép",
  deny: "Không",

  // A pick that brought nothing — each says what to do next, and that the request can go without photos.
  camera_refused: "Bạn chưa cho phép dùng máy ảnh. Bạn vẫn gửi được yêu cầu không kèm ảnh, hoặc bấm “Chọn ảnh”.",
  library_refused: "Bạn chưa cho phép chọn ảnh. Bạn vẫn gửi được yêu cầu không kèm ảnh, hoặc bấm “Chụp ảnh”.",
  pick_failed: "Zalo chưa mở được máy ảnh hoặc thư viện ảnh. Vui lòng thử lại, hoặc gửi yêu cầu không kèm ảnh.",

  // A picked photo kept OUT of the request, said while the person can still choose another.
  rejected: {
    "file-unreadable":
      "Ứng dụng không đọc được ảnh này trên máy bạn. Vui lòng chụp lại hoặc chọn một ảnh khác.",
    "too-large": "Ảnh này lớn hơn 5 MB. Vui lòng chọn ảnh nhỏ hơn hoặc chụp lại.",
    unsupported: "Ảnh này không phải JPEG, PNG hoặc WebP. Vui lòng chọn một ảnh khác.",
  },

  // Photos are for a consultation request only (409 `photo_not_allowed` otherwise) — said before the tap.
  callback_no_photos: "Ảnh chỉ được gửi kèm “Gửi yêu cầu tư vấn”, không gửi kèm đề nghị gọi lại.",

  // After the request is sent.
  sending: (n: number, total: number) => `Đang gửi ảnh ${n}/${total} tới chuyên viên…`,
  all_arrived: (total: number) => (total === 1 ? "Chuyên viên đã nhận ảnh của bạn." : `Chuyên viên đã nhận đủ ${total} ảnh của bạn.`),
  some_arrived: (stored: number, total: number) =>
    `Chuyên viên đã nhận ${stored}/${total} ảnh. Yêu cầu của bạn vẫn đã được gửi; ảnh chưa gửi được ghi lý do bên dưới.`,
  row_failed: (n: number, why: string) => `Ảnh ${n} chưa gửi được. ${why}`,
  retry: (n: number) => `Tải lại ảnh ${n}`,
  /** Every server answer → what to do next. Keys: `RequestPhotoFailure` (`api/hop-dong-yeu-cau.ts`). */
  failures: {
    "invalid-upload": "Ảnh này không gửi được. Bạn có thể gửi ảnh cho chuyên viên qua nút “Chat Zalo”.",
    "not-found":
      "Không tìm thấy yêu cầu để đính ảnh. Vui lòng xem lại ở mục Yêu cầu của tôi, hoặc gửi ảnh qua nút “Chat Zalo”.",
    limit: "Yêu cầu này đã đủ 3 ảnh, nên ảnh này không được gửi thêm.",
    "not-allowed": "Chỉ yêu cầu tư vấn mới gửi kèm được ảnh, nên ảnh này không được gửi.",
    "too-large": "Ảnh lớn hơn 5 MB. Bạn có thể gửi ảnh nhỏ hơn qua nút “Chat Zalo”.",
    unsupported: "Ảnh không phải JPEG, PNG hoặc WebP. Bạn có thể gửi ảnh khác qua nút “Chat Zalo”.",
    unreadable: "Ảnh bị hỏng hoặc chưa lưu trọn. Bạn có thể chụp lại rồi gửi qua nút “Chat Zalo”.",
    server: "Hệ thống chưa nhận được ảnh. Vui lòng bấm “Tải lại”.",
    unavailable: "Chức năng nhận ảnh đang tạm ngưng. Vui lòng bấm “Tải lại” sau ít phút, hoặc gửi ảnh qua nút “Chat Zalo”.",
    session:
      "Phiên đăng nhập đã hết hạn. Yêu cầu của bạn đã được ghi nhận; vui lòng gửi ảnh qua nút “Chat Zalo”.",
    "rate-limited": "Bạn đã gửi nhiều lần trong ít phút. Vui lòng chờ khoảng 5 phút rồi bấm “Tải lại”.",
    network: "Không kết nối được. Vui lòng kiểm tra mạng rồi bấm “Tải lại”.",
    "not-configured": "Hệ thống đang gián đoạn. Vui lòng bấm “Tải lại” sau ít phút.",
  },
} as const;

export const YEU_CAU_CUA_TOI = {
  tieu_de: "Yêu cầu của tôi",
  dan_nhap: "Các yêu cầu tư vấn bạn đã gửi.",

  can_dang_nhap_tieu_de: "Đăng nhập để xem yêu cầu",
  can_dang_nhap:
    "Đăng nhập bằng số Zalo để xem các yêu cầu bạn đã gửi.",
  nut_toi_dang_nhap: "Đăng nhập",

  dang_doc: "Đang tải danh sách yêu cầu…",
  // DANH SÁCH RỖNG CÓ MỘT CÂU, KHÔNG PHẢI MỘT MÀN TRẮNG. Một màn trắng và một màn hỏng trông
  // giống hệt nhau, và người lớn tuổi đọc cả hai thành "app hỏng".
  rong: "Bạn chưa có yêu cầu nào.",
  // The next step, said under the empty sentence (12/10/2026): what sending one gets you.
  rong_goi_y: "Gửi yêu cầu tư vấn, chuyên viên ViHAT Group sẽ liên hệ lại và bạn theo dõi tình trạng tại đây.",
  nut_gui_moi: "Gửi yêu cầu tư vấn",
  nut_doc_lai: "Tải lại",
  // Under every failure that a second try can fix (12/10/2026): one button, the word the commune app uses too.
  nut_thu_lai: "Thử lại",
  tieu_de_loi: "Chưa tải được danh sách",

  // PULL TO REFRESH (12/10/2026). A failed refresh KEEPS the list on screen and says so above it.
  keo_tai_lai: "Kéo xuống để tải lại",
  tha_tai_lai: "Thả tay để tải lại",
  dang_tai_lai: "Đang tải lại…",
  tai_lai_hong: "Chưa tải được dữ liệu mới. Danh sách dưới đây là lần tải trước — kéo xuống để thử lại.",

  session_expired_title: "Phiên đăng nhập đã hết hạn",
  phien_het_han:
    "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại để xem các yêu cầu bạn đã gửi.",
  khong_doc_duoc:
    "Không tải được danh sách. Vui lòng kiểm tra mạng và thử lại.",
  server_error: "Máy chủ ViHAT đang gặp sự cố. Vui lòng thử lại sau ít phút.",
  chua_khai_host:
    "Hệ thống đang gián đoạn. Vui lòng thử lại sau.",

  nhan_ma: "Mã yêu cầu",
  nhan_gui_luc: "Gửi lúc",
  // 12/10/2026: how many photos the server holds for the request (`photoCount`); the row is absent at 0.
  photos_label: "Ảnh đính kèm",
  photo_count: (n: number) => `${n} ảnh`,
} as const;

/**
 * "KẾT NỐI VỚI ViHAT" — the one card on Liên hệ holding every ask of the phone number (owner 12/10/2026, Zalo
 * resubmission): log in, chat with a specialist, SMS offers. Each action keeps its own purpose sentence, shown before
 * its button and so before Zalo's dialog (Mini App policy 3.3.4).
 *
 * ⚠ `connected` NAMES NO NUMBER, NOT EVEN MASKED: the app never receives the number — `getPhoneNumber` returns a
 *   code only the server can exchange (`dang-nhap/hop-dong.ts`, type `Phien`: "KHÔNG CÓ `so_dien_thoai`"). A masked
 *   number on screen would need the server to send it back, a new data flow nobody decided.
 */
export const CONNECT_VIHAT = {
  title: "Kết nối với ViHAT",
  lead: "Ba việc riêng, mỗi việc nói rõ để làm gì. Zalo chỉ hỏi quyền khi chính bạn bấm.",
  connected: "Đã kết nối với ViHAT bằng số Zalo của bạn.",
  // A session can expire on the server while the app still holds it, so a way to log in again always stays; it
  // opens the login action with its purpose sentence, it does not call Zalo by itself. (Since 14/10/2026 a 401 on
  // Tư vấn or Yêu cầu của tôi forgets the session, so this card no longer says "connected" after one.)
  relogin: "Đăng nhập lại",
  // The way back after "Đăng nhập" was pressed on one of the two request screens (14/10/2026).
  back_to_consult: "Tiếp tục gửi yêu cầu",
  back_to_requests: "Quay lại Yêu cầu của tôi",
} as const;

/**
 * "CHAT VỚI CHUYÊN VIÊN" — an action of the "Kết nối với ViHAT" card on Liên hệ (07/10/2026).
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
 * "NHẬN ƯU ĐÃI QUA SMS" — an action of the "Kết nối với ViHAT" card on Liên hệ (07/10/2026; on the home screen
 * until 12/10/2026).
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
  server_error: "Máy chủ ViHAT đang gặp sự cố. Vui lòng thử lại sau ít phút.",
  // No answer in time: the choice may be recorded. Opening Liên hệ again re-reads the server's own list
  // (`SmsOffers`, `deriveSmsSubscription`), so the buttons then show what the server holds before a second tap.
  unconfirmed:
    "Chưa nhận được phản hồi từ máy chủ, nhưng lựa chọn của bạn có thể đã được ghi nhận. Vui lòng mở lại mục này sau ít phút để xem trạng thái trước khi bấm lại.",
} as const;
