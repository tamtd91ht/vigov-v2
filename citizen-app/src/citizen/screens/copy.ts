/**
 * MỌI CÂU CHỮ CỦA HAI MÀN "GỬI PHẢN ÁNH" VÀ "TRA CỨU PHIẾU" — một tệp, kể cả bảng nhãn trạng thái.
 *
 * Người đọc là công dân, thường lớn tuổi, thường đang bực vì chính việc họ báo (`skills/
 * accessibility-elderly`): câu ngắn, một ý, không viết tắt, không mã lỗi, luôn nói việc cần làm tiếp.
 */
import type { SceneLocationWords } from "./scene-location";
import { groupOf, STATUS_GROUP_LABEL } from "./status-groups";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CHÍN TRẠNG THÁI (ADR 0027) — người dân KHÔNG thấy chín nhãn ấy nữa mà thấy BỐN NHÓM (ADR 0050 #5,
 * chủ dự án 28/09/2026 mở phạm vi sang cả app chung: "3 điểm còn lại cũng theo require nhé"). Nhãn
 * nhóm nằm ở `status-groups.ts`; chín nhãn của cán bộ vẫn ở `web-admin/src/features/phan-anh/
 * nhan-phieu.ts`, không đổi. Bảng dưới chỉ còn là danh sách chín mã + câu giải thích cho dân.
 *
 * ⚠ HỢP ĐỒNG KHAI `status` LÀ `string` TRƠN — cùng lỗ hổng `nhan-phieu.ts` đã báo. Mã lạ không hiện
 * nguyên mã (một chuỗi `dang-xu-ly` vô nghĩa với người dân) và KHÔNG bị đoán vào một nhóm (một phiếu
 * còn mở mà hiện "Đã đóng" là nói sai với dân): nó hiện một câu trung tính kèm việc cần làm.
 *
 * `explanation` là DÒNG PHỤ cho người dân, không thay nhãn. Với bốn nhóm, dòng phụ là chỗ DUY NHẤT phân
 * biệt "Không tiếp nhận" / "Chuyển cấp trên" với "Đã đóng" thường — đừng gỡ nó. Bốn dòng có câu:
 *   `da-tiep-nhan`   "Đã gửi, đang chờ cán bộ xã xem." — theo góp ý nghiệp vụ của lượt này: phần mềm
 *                    tự sinh trạng thái ấy, chưa ai ở xã đọc phiếu, nên "đã tiếp nhận" trần dễ bị
 *                    hiểu là xã đã nhận việc.
 *   `dang-phan-loai` câu duy nhất đặc tả cho (`docs/ui-ux/09` §8.2).
 *   `khong-tiep-nhan`, `chuyen-cap-tren` — hai nhánh KẾT THÚC: phiếu dừng ở xã, và người dân không
 *                    có bước nào khác trên ứng dụng. Câu chỉ nói sự việc và chỉ xuống lý do / cơ quan
 *                    nhận mà thẻ phiếu hiện ngay bên dưới (migration 0011). Chưa qua khách duyệt câu chữ.
 * Năm dòng còn lại để `null` — không bịa câu chưa ai duyệt, đúng khuôn `cauGiaiThichTrangThai`.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */
export const STATUS: Readonly<Record<string, { explanation: string | null }>> = {
  "da-tiep-nhan": { explanation: "Đã gửi, đang chờ cán bộ xã xem." },
  "dang-phan-loai": { explanation: "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không." },
  "da-chuyen-xu-ly": { explanation: null },
  "dang-xu-ly": { explanation: null },
  "da-xu-ly": { explanation: null },
  "cho-dan-xac-nhan": { explanation: null },
  "da-dong": { explanation: null },
  "khong-tiep-nhan": {
    explanation: "Ủy ban nhân dân xã không tiếp nhận phản ánh này. Lý do ghi ở dưới.",
  },
  "chuyen-cap-tren": {
    explanation:
      "Ủy ban nhân dân xã đã chuyển phản ánh tới cơ quan có thẩm quyền. Tên cơ quan ghi ở dưới.",
  },
};

export const STATUS_UNLABELLED =
  "Trạng thái mới, ứng dụng chưa có tên gọi. Bạn hãy hỏi Ủy ban nhân dân xã và đọc mã tra cứu.";

/** Nhãn người dân thấy: một trong bốn nhóm, hoặc câu trung tính cho mã lạ (không đoán nhóm). */
export function statusLabel(code: string): string {
  const group = groupOf(code);
  return group === null ? STATUS_UNLABELLED : STATUS_GROUP_LABEL[group];
}

export function statusExplanation(code: string): string | null {
  return STATUS[code]?.explanation ?? null;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CHUNG CHO HAI MÀN
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Kênh chưa mở — câu hiện khi CHƯA CÓ PHIÊN ViGov (hôm nay: luôn luôn). Nói thật, và nói việc làm
 * được ngay bây giờ. Không mời "thử lại sau": bấm lại không đổi được gì.
 */
export const CHANNEL_NOT_OPEN = {
  title: "Kênh phản ánh của xã chưa mở trên ứng dụng này",
  text: "Ứng dụng chưa gửi được phản ánh tới xã. Bạn hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã để phản ánh.",
} as const;

export const SENDING_TO_COMMUNE_LABEL = "Đang làm việc với";

export const EMERGENCY =
  "Việc khẩn cấp, cần giúp ngay: gọi 113 (Công an), 114 (Cứu hỏa), 115 (Cấp cứu). Phản ánh trên ứng dụng không được xử lý ngay lập tức.";

export const BACK = "Quay lại";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * MÀN "GỬI PHẢN ÁNH"
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const SEND = {
  title: "Gửi phản ánh",
  content_label: "Nội dung phản ánh (bắt buộc)",
  hint_content: "Việc gì đang xảy ra? Bạn thấy từ khi nào?",
  address_label: "Nơi xảy ra (không bắt buộc)",
  hint_address: "Ví dụ: đầu ngõ, gần chợ, tên thôn.",
  name_label: "Họ và tên (không bắt buộc)",
  phone_label: "Số điện thoại để xã liên hệ lại (không bắt buộc)",
  anonymous: "Gửi ẩn danh",
  anonymous_on: "Đang bật",
  anonymous_off: "Đang tắt",
  anonymous_explanation:
    "Khi gửi ẩn danh, ứng dụng không gửi họ tên và số điện thoại. Cán bộ xử lý không thấy bạn là ai.",
  photos_not_supported: "Ứng dụng chưa hỗ trợ đính kèm ảnh.",
  next_button: "Tiếp tục",
  missing_content: "Bạn chưa viết nội dung phản ánh. Hãy viết vài câu về việc đang xảy ra.",
  too_long: (name: string, max: number) =>
    `Ô "${name}" dài quá. Hãy viết gọn lại, tối đa ${max.toLocaleString("vi-VN")} ký tự.`,

  confirm_title: "Kiểm tra trước khi gửi",
  confirm_text: "Phản ánh sẽ được gửi tới:",
  confirm_consequence:
    "Chỉ xã này nhận và xử lý phản ánh. Nếu việc xảy ra ở xã khác, xin đừng gửi tại đây mà liên hệ Ủy ban nhân dân xã nơi xảy ra sự việc.",
  send_button: (commune_name: string) => `Gửi tới ${commune_name}`,
  edit_button: "Sửa lại",
  sending: "Đang gửi phản ánh…",

  done_title: "Đã gửi phản ánh",
  done_code: "Mã tra cứu của bạn",
  done_keep_code: "Hãy chép lại hoặc chụp màn hình mã này. Bạn cần mã để xem phiếu đi tới đâu.",
  will_view_by: (time: string) => `Phiếu sẽ được cán bộ xã xem trước ${time} (giờ Việt Nam).`,
  send_another: "Gửi phản ánh khác",
  resend_button: "Gửi lại",
  /** Shown on the confirmation step when a location is attached — the citizen sees what goes with it. */
  confirm_location: (coordinates: string) => `Kèm vị trí hiện tại: ${coordinates}.`,
} as const;

/**
 * "Lấy vị trí hiện tại" on the live form — "bạn", like every sentence of the shared app's citizen screens.
 * Every failure says what to do next and names the address box as the way that always works (the button
 * sits under it, hence "ô trên"). The box stays optional, as it is on this form and in the prototype.
 */
export const SEND_LOCATION_WORDS: SceneLocationWords = {
  button: "Lấy vị trí hiện tại",
  button_again: "Lấy lại vị trí hiện tại",
  locating: "Đang lấy vị trí…",
  why: "Để cán bộ xã tìm đúng nơi xảy ra sự việc. Zalo sẽ hỏi bạn có đồng ý chia sẻ vị trí không. Vị trí chỉ tới xã khi bạn gửi phản ánh.",
  found: (coordinates) => `Đã lấy vị trí hiện tại (${coordinates}). Bạn vẫn nên ghi rõ nơi xảy ra ở ô trên.`,
  failures: {
    "tu-choi": "Bạn chưa đồng ý chia sẻ vị trí. Bạn vẫn gửi được phản ánh — hãy ghi rõ nơi xảy ra ở ô trên.",
    "ngoai-zalo": "Chỉ lấy được vị trí khi mở ứng dụng trong Zalo. Hãy ghi rõ nơi xảy ra ở ô trên.",
    "qua-nhieu-lan": "Bạn đã thử lấy vị trí nhiều lần. Hãy chờ vài phút rồi bấm lại, hoặc ghi rõ nơi xảy ra ở ô trên.",
    "thu-lai": "Chưa lấy được vị trí. Hãy bấm lại nút, hoặc ghi rõ nơi xảy ra ở ô trên.",
    "tam-ngung": "Ứng dụng tạm thời chưa lấy được vị trí. Hãy ghi rõ nơi xảy ra ở ô trên.",
  },
};

/**
 * Câu cho từng nhánh không thành của lần gửi. `can_resend` quyết định nút "Gửi lại" (CÙNG khoá
 * chống trùng — xem `api/send-attempt.ts`) có hiện hay không.
 */
export const SEND_ERROR: Readonly<
  Record<
    | "het-phien"
    | "dang-xu-ly-truoc"
    | "field-not-offered"
    | "khong-hop-le"
    | "field-catalogue-unavailable"
    | "kenh-chua-mo"
    | "loi-may-chu"
    | "loi-mang"
    | "khong-tao-duoc-khoa",
    { text: string; can_resend: boolean }
  >
> = {
  "het-phien": {
    text: "Phiên làm việc đã hết hạn nên phản ánh chưa được gửi. Hãy đóng ứng dụng, mở lại rồi gửi lại.",
    can_resend: false,
  },
  "dang-xu-ly-truoc": {
    text: "Lần gửi trước của bạn đang được xử lý. Hãy chờ một phút rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    can_resend: true,
  },
  "khong-hop-le": {
    text: "Phản ánh chưa được gửi vì có ô chưa đúng. Hãy bấm Sửa lại, viết gọn nội dung rồi gửi lại.",
    can_resend: false,
  },
  // 400 `field_not_offered`: the commune changed its list since it was loaded. The screen reloads the list
  // and returns to the field step; sending again unchanged would only meet the same answer.
  "field-not-offered": {
    text: "Lĩnh vực đã chọn hiện không còn trong danh sách xã đang nhận, nên phản ánh chưa được gửi. Hãy chọn lại lĩnh vực. Nội dung đã viết vẫn còn nguyên.",
    can_resend: false,
  },
  // 503 `field_catalogue_unavailable`: nothing was written, and it clears by itself (ADR 0060 §3).
  "field-catalogue-unavailable": {
    text: "Chưa kiểm tra được lĩnh vực nên phản ánh CHƯA được ghi nhận. Hãy chờ vài phút rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    can_resend: true,
  },
  "kenh-chua-mo": {
    text: "Ủy ban nhân dân xã chưa mở kênh nhận phản ánh trực tuyến. Phản ánh của bạn CHƯA được ghi nhận. Hãy liên hệ trực tiếp Ủy ban nhân dân xã.",
    can_resend: false,
  },
  "loi-may-chu": {
    text: "Hệ thống của xã đang gặp sự cố nên chưa nhận được phản ánh. Hãy chờ vài phút rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    can_resend: true,
  },
  "loi-mang": {
    text: "Không gửi được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    can_resend: true,
  },
  "khong-tao-duoc-khoa": {
    text: "Điện thoại này chưa gửi được phản ánh an toàn. Hãy cập nhật ứng dụng Zalo rồi thử lại, hoặc liên hệ trực tiếp Ủy ban nhân dân xã.",
    can_resend: false,
  },
};

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * XÁC NHẬN SỐ ĐIỆN THOẠI — khi xã cần số của công dân mới gửi / xem được phản ánh
 *
 * Hiện ở cả ba màn phản ánh (`phone-verification.tsx`). Câu nói VÌ SAO trước, rồi nói Zalo sẽ hỏi gì,
 * rồi mới tới nút: người lớn tuổi bấm một nút mà không biết nó dẫn tới hộp thoại nào là người sẽ bấm
 * "Từ chối" ở hộp thoại ấy. Mỗi câu kết quả nói việc làm tiếp; không câu nào nhắc mã lỗi.
 *
 * `task` là vế "việc gì chưa làm được", để một câu dùng chung cho cả ba màn mà vẫn nói đúng màn ấy.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const PHONE_VERIFICATION_TASK = {
  submit: "phản ánh chưa được gửi",
  lookup: "chưa tra cứu được phiếu",
  mine: "chưa xem được phản ánh của bạn",
  rate: "đánh giá chưa được gửi",
} as const;

export type PhoneVerificationTask = keyof typeof PHONE_VERIFICATION_TASK;

export const PHONE_VERIFICATION = {
  title: "Cần xác nhận số điện thoại của bạn",
  why: "Để gửi phản ánh và xem phản ánh của chính mình, xã cần biết số điện thoại Zalo của bạn. Nhờ số này, xã biết phản ánh là của ai, và chỉ bạn xem được phản ánh của bạn.",
  zalo_asks:
    "Khi bạn bấm nút dưới đây, Zalo sẽ hỏi bạn có đồng ý chia sẻ số điện thoại không. Số được gửi qua máy chủ của Tập đoàn ViHAT Group tới hệ thống của xã. Ứng dụng không lưu số này trên điện thoại.",
  allow: "Đồng ý chia sẻ số điện thoại",
  decline: "Không chia sẻ",
  working: "Đang xác nhận số điện thoại…",
  refused: (task: string) =>
    `Bạn chưa chia sẻ số điện thoại, nên ${task}. Bạn vẫn có thể đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã.`,
  retry: (task: string) =>
    `Chưa xác nhận được số điện thoại vì mạng yếu hoặc hệ thống đang bận, nên ${task}. Hãy kiểm tra mạng rồi bấm “Đồng ý chia sẻ số điện thoại” lần nữa.`,
  outside_zalo: (task: string) =>
    `Chỉ xác nhận được số điện thoại khi mở ứng dụng trong Zalo, nên ${task}. Hãy mở ứng dụng trong Zalo rồi làm lại.`,
  unavailable: (task: string) =>
    `Hệ thống của xã chưa xác nhận được số điện thoại qua ứng dụng lúc này, nên ${task}. Hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã.`,
  still_unverified: (task: string) =>
    `Xã chưa xác nhận được số điện thoại của bạn, nên ${task}. Hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã.`,
  other_commune: (task: string) =>
    `Ứng dụng chưa mở lại được phiên làm việc với đúng xã ghi ở đầu màn hình, nên ${task}. Hãy đóng ứng dụng rồi quét lại mã QR của xã.`,
  draft_kept: "Nội dung bạn đã viết vẫn còn nguyên.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * APP RIÊNG CỦA XÃ — MỞ PHIÊN Ở VIỆC CÁ NHÂN ĐẦU TIÊN (`commune-session.ts`)
 *
 * Lời giải thích trước hộp thoại của Zalo DÙNG LẠI `PHONE_VERIFICATION` (title · why · zalo_asks · allow ·
 * decline): cùng một việc — chia sẻ số để xã biết phản ánh là của ai — và hai bản câu chữ cho một việc là
 * hai bản sẽ lệch. Ở đây chỉ có các câu kết quả RIÊNG của app xã; câu nào trùng nghĩa thì dùng lại câu cũ.
 * Không câu nào nhắc mã lỗi; câu nào cũng nói việc làm tiếp.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const COMMUNE_APP_SESSION = {
  working: "Đang kết nối với hệ thống của xã…",
  not_connected: (task: string) =>
    `Ứng dụng của xã chưa được kết nối với hệ thống tiếp nhận phản ánh, nên ${task}. Hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã.`,
  paused: (task: string) =>
    `Hệ thống tiếp nhận phản ánh qua ứng dụng đang tạm ngưng, nên ${task}. Hãy thử lại sau ít phút, hoặc gọi điện thoại cho xã.`,
  wait: (task: string) =>
    `Zalo hoặc hệ thống của xã đang bận, nên ${task}. Hãy chờ một lát rồi bấm “Đồng ý chia sẻ số điện thoại” lần nữa.`,
  other_commune: (task: string) =>
    `Ứng dụng chưa xác nhận được bạn đang làm việc với đúng xã ghi ở đầu màn hình, nên ${task}. Hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã.`,
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * MÀN "TRA CỨU PHIẾU"
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const LOOKUP = {
  title: "Tra cứu phiếu của tôi",
  code_label: "Mã tra cứu",
  hint_code: "Mã được đưa cho bạn ngay khi gửi phản ánh.",
  lookup_button: "Tra cứu",
  looking_up: "Đang tìm phiếu…",
  found: "Đã tìm thấy phiếu. Thông tin phiếu ở ngay bên dưới.",
  missing_code: "Bạn chưa nhập mã tra cứu. Hãy nhập mã được đưa khi gửi phản ánh.",
  /**
   * MỘT CÂU cho "không có mã này", "phiếu của người khác", "phiếu của xã khác" — máy chủ trả cùng
   * một 404, và màn hình không được tách chúng ra (luật 4, cấm #2).
   */
  not_found:
    "Không tìm thấy phiếu với mã này. Hãy kiểm tra lại từng ký tự của mã. Nếu vẫn không thấy, hãy liên hệ Ủy ban nhân dân xã.",
  session_expired: "Phiên làm việc đã hết hạn. Hãy đóng ứng dụng, mở lại rồi tra cứu lại.",
  server_error: "Hệ thống của xã đang gặp sự cố. Hãy chờ vài phút rồi tra cứu lại.",
  network_error: "Không tra được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi tra cứu lại.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐÁNH GIÁ KẾT QUẢ XỬ LÝ — trên phiếu mở ở màn tra cứu (ADR 0050 điểm 2, `CitizenReportRating.tsx`)
 *
 * KHÔNG CÂU NÀO NÓI NGƯỠNG SAO MỞ LẠI PHIẾU (ADR 0050: "Giao diện dân không nói ngưỡng"): người dân chấm
 * theo điều họ thấy, không theo hệ quả họ được báo trước. Khi máy chủ đã mở lại, câu `reopened` chỉ nói
 * SỰ VIỆC đã xảy ra — tình trạng mới đã hiện trên thẻ phiếu.
 *
 * Nhãn năm mức sao và câu "chạm vào sao" nằm ở `star-picker.tsx`, dùng chung cho cả hai app, không chép.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const RATING = {
  title: "Đánh giá kết quả xử lý",
  your_rating: "Đánh giá của bạn",
  why: "Đánh giá của bạn giúp Ủy ban nhân dân xã biết việc đã được giải quyết đúng mong muốn chưa.",
  comment_label: "Nhận xét thêm (không bắt buộc)",
  submit: "Gửi đánh giá",
  retry: "Gửi lại",
  reload: "Tải lại phiếu",
  rate_again: "Đánh giá lại",
  sending: "Đang gửi đánh giá…",
  sent: "Đã gửi đánh giá. Cảm ơn bạn.",
  rated: (n: number) => `Bạn đã đánh giá ${n} sao.`,
  reopened: "Phản ánh đã được chuyển lại cho Ủy ban nhân dân xã xử lý tiếp. Tình trạng mới ghi ở trên.",
} as const;

/**
 * Câu cho từng nhánh không thành của lần gửi đánh giá. `can_retry`: nút "Gửi lại" dùng lại CÙNG lần gửi
 * (cùng khoá chống trùng). `can_reload`: nút "Tải lại phiếu" — khi việc cần làm là xem tình trạng mới.
 */
export const RATING_ERROR: Readonly<
  Record<
    "session-expired" | "not-found" | "state-changed" | "invalid" | "server-fault" | "network" | "no-key",
    { text: string; can_retry: boolean; can_reload: boolean }
  >
> = {
  "session-expired": {
    text: "Phiên làm việc đã hết hạn nên đánh giá chưa được gửi. Hãy đóng ứng dụng, mở lại rồi đánh giá lại.",
    can_retry: false,
    can_reload: false,
  },
  "not-found": {
    text: "Không tìm thấy phiếu này nữa nên đánh giá chưa được gửi. Hãy liên hệ Ủy ban nhân dân xã để hỏi.",
    can_retry: false,
    can_reload: false,
  },
  "state-changed": {
    text: "Tình trạng phiếu vừa thay đổi, hoặc lần gửi trước còn đang được xử lý, nên đánh giá chưa được ghi nhận. Hãy bấm “Tải lại phiếu” để xem tình trạng hiện tại.",
    can_retry: false,
    can_reload: true,
  },
  invalid: {
    text: "Đánh giá chưa được gửi vì có ô chưa đúng. Hãy chọn từ 1 đến 5 sao, viết nhận xét ngắn lại rồi bấm “Gửi đánh giá”.",
    can_retry: false,
    can_reload: false,
  },
  "server-fault": {
    text: "Hệ thống của xã đang gặp sự cố nên chưa nhận được đánh giá. Hãy chờ vài phút rồi bấm “Gửi lại”.",
    can_retry: true,
    can_reload: false,
  },
  network: {
    text: "Không gửi được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm “Gửi lại”.",
    can_retry: true,
    can_reload: false,
  },
  "no-key": {
    text: "Điện thoại này chưa gửi được đánh giá an toàn. Hãy cập nhật ứng dụng Zalo rồi thử lại, hoặc liên hệ trực tiếp Ủy ban nhân dân xã.",
    can_retry: false,
    can_reload: false,
  },
};

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * MÀN "PHẢN ÁNH CỦA TÔI" — danh sách phiếu của chính người dân
 *
 * Nhãn trạng thái KHÔNG viết lại ở đây: thẻ dùng `statusLabel`, cùng bảng màn tra cứu dùng. Nhãn
 * "Gửi lúc" / "Hạn xử lý xong" lấy từ `REPORT_CARD` để hai màn nói cùng một chữ cho cùng một mốc.
 *
 * KHÔNG CÓ CÂU "QUÁ HẠN": quá hạn đếm bằng GIỜ LÀM VIỆC (ADR 0007), và chỉ `identity` đếm được.
 * Màn này chỉ hiện mốc hạn cố định máy chủ đã ghi.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const MY_REPORTS = {
  title: "Phản ánh của tôi",
  loading: "Đang tải danh sách phản ánh…",
  empty: "Bạn chưa gửi phản ánh nào.",
  view_button: "Xem chi tiết",
  load_more_button: "Xem thêm",
  loading_more: "Đang tải thêm…",
  retry_button: "Thử lại",
  end_of_list: "Đã hiện hết phản ánh của bạn.",
  session_expired: "Phiên làm việc đã hết hạn. Hãy đóng ứng dụng, mở lại rồi xem lại danh sách.",
  server_error: "Hệ thống của xã đang gặp sự cố. Hãy chờ vài phút rồi bấm Thử lại.",
  network_error: "Không tải được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Thử lại.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * THẺ PHIẾU — dùng chung cho màn kết quả gửi và màn tra cứu
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const REPORT_CARD = {
  status: "Tình trạng",
  field: "Lĩnh vực",
  /** Lĩnh vực do CÁN BỘ chốt (ADR 0028). Chưa chốt là bình thường, không phải lỗi. */
  not_classified: "Cán bộ xã chưa phân loại",
  /** Có mã mà xã chưa đặt tên: KHÔNG hiện mã thô cho người dân. */
  classified: "Đã được cán bộ xã phân loại",
  sent_at: "Gửi lúc",
  view_due: "Hạn cán bộ xem phiếu",
  due_at: "Hạn xử lý xong",
  /** `resolve_due = null`: chưa có cam kết nào — không bịa một ngày. */
  due_at_not_yet: "Chưa có. Hạn được ấn định sau khi cán bộ phân loại phiếu.",
  /** `acknowledge_due = null`: khoảng ấy không áp dụng cho phiếu này. */
  view_due_not_applicable: "Không áp dụng",
  content: "Nội dung",
  address: "Nơi xảy ra",
  address_empty: "Chưa rõ vị trí",
  reporter: "Người gửi",
  anonymous: "Gửi ẩn danh",
  name_not_given: "Không ghi họ tên",
  result: "Kết quả xử lý của xã",
  vn_time: "(giờ Việt Nam)",
  /* Hai nhánh kết thúc (`khong-tiep-nhan`, `chuyen-cap-tren`) — chữ do cán bộ viết CHO người dân. */
  rejection_reason: "Lý do xã không tiếp nhận",
  receiving_body_label: "Cơ quan tiếp nhận",
  referral_reason: "Lý do chuyển",
  /** Việc làm tiếp: phiếu đã rời xã, ứng dụng không theo được nó tới cơ quan kia. */
  contact_body:
    "Bạn có thể liên hệ trực tiếp cơ quan tiếp nhận ở trên để hỏi tiếp về phản ánh này.",
  /**
   * Máy chủ không gửi lý do / tên cơ quan dù phiếu ở nhánh kết thúc — lẽ ra không xảy ra (máy chủ bắt
   * buộc hai ô ấy khi cán bộ bấm). Không để ô trống im lặng: nói việc người dân làm được.
   */
  not_recorded: "Ứng dụng chưa nhận được thông tin này. Bạn hãy liên hệ Ủy ban nhân dân xã để hỏi.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * XÁC NHẬN XÃ TỪ MÃ QR (ADR 0047 §Trả lời mục 4) — tra tên xã, rồi mở phiên sau khi xác nhận
 *
 * Mọi nhánh không thành đều về PHẦN GIỚI THIỆU kèm một câu — trừ `retry`, nơi bấm lại có thể được.
 * Không câu nào nhắc tên miền, mã lỗi hay tên dịch vụ: người dân không làm gì được với chúng.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const COMMUNE_CONFIRMATION = {
  looking_up: "Đang tìm xã theo mã QR bạn vừa quét…",
  /** Máy chủ nói "không xã nào" (hoặc từ chối tên miền trên mã). */
  not_found:
    "Mã QR này chưa dẫn tới xã nào trên ứng dụng. Bạn vẫn xem được phần giới thiệu. Nếu cần làm việc với xã, hãy quét mã QR dán tại trụ sở Ủy ban nhân dân xã.",
  /** Nền tảng tạm ngưng, lỗi máy chủ, mất mạng. */
  not_connected:
    "Chưa kết nối được tới hệ thống của xã. Bạn vẫn xem được phần giới thiệu. Hãy kiểm tra mạng, chờ ít phút rồi quét lại mã QR.",
  opening: "Đang mở kênh làm việc với xã…",
  // Hai câu `chua_mo` · `ngoai_zalo` cũ đã bị gỡ 27/09/2026: hai nhánh ấy không còn về phần giới thiệu
  // mà mở tin tức và danh bạ của xã vừa xác nhận; câu thay thế là `COMMUNE_NOT_LOGGED_IN` ở dưới.
  retry:
    "Chưa mở được kênh làm việc với xã vì mạng yếu hoặc hệ thống đang bận. Hãy kiểm tra mạng rồi bấm “Đúng, tiếp tục” lần nữa.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * APP RIÊNG CỦA XÃ (`--vao-thang`, ADR 0047 §6) — không màn giới thiệu, không QR, không nút xác nhận
 *
 * Câu của đường QR ở trên nói "phần giới thiệu" và "quét lại mã QR" — cả hai đều không có trong app
 * riêng. Người dân ở đây chỉ làm được một việc: thử lại.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const COMMUNE_APP = {
  opening: "Đang mở ứng dụng của xã…",
  /** Máy chủ nói tên miền của bản dựng không thuộc xã nào đang hoạt động. */
  not_found: "Ứng dụng này chưa được gắn với xã nào đang hoạt động. Vui lòng liên hệ Ủy ban nhân dân xã.",
  /** Nền tảng tạm ngưng, lỗi máy chủ, mất mạng. */
  not_connected: "Chưa kết nối được tới hệ thống của xã. Hãy kiểm tra mạng rồi bấm “Thử lại”.",
  retry: "Thử lại",
} as const;

/**
 * ĐÃ XÁC NHẬN XÃ NHƯNG CHƯA CÓ PHIÊN — câu trên màn chọn việc, thay vì để ba lối phản ánh im lặng.
 *
 * Đứng NGAY TRÊN ba lối ấy: người dân đọc nó trước khi bấm. Ba lối vẫn còn (mỗi màn nói lại "kênh chưa
 * mở" và chỉ đường tới trụ sở), vì một nút biến mất không báo là thứ người lớn tuổi không tìm lại được.
 * `remaining` chỉ hiện khi có tên miền — tức khi hai màn công khai thật sự có mặt ở dưới.
 */
export const COMMUNE_NOT_LOGGED_IN = {
  text: "Ứng dụng chưa đăng nhập được với xã trên điện thoại này, nên chưa gửi phản ánh, xem phản ánh của bạn hay tra cứu phiếu được. Để phản ánh, hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã hoặc gọi điện thoại cho xã.",
  remaining: "Bạn vẫn đọc được tin tức và danh bạ cán bộ của xã ở dưới.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TIN TỨC CỦA XÃ — công khai, văn bản thuần
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const COMMUNE_NEWS = {
  title: "Tin tức của xã",
  loading: "Đang tải tin của xã…",
  loading_more: "Đang tải thêm tin…",
  empty: "Hiện chưa có tin nào được đăng trên ứng dụng.",
  read_button: "Đọc tin",
  load_more_button: "Xem thêm tin",
  retry_button: "Thử lại",
  end_of_list: "Đã hiện hết tin của xã.",
  published_on: "Ngày đăng",
  category: "Chuyên mục",
  loading_article: "Đang mở tin…",
  not_found: "Tin này không còn trên ứng dụng. Hãy bấm Quay lại để xem các tin khác.",
  network_error: "Không tải được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Thử lại.",
  server_error: "Hệ thống của xã đang bận nên chưa tải được tin. Hãy chờ vài phút rồi bấm Thử lại.",
  /** Tên miền trên mã bị từ chối giữa chừng — thử lại không đổi được gì. */
  invalid: "Chưa tải được tin của xã. Hãy đóng ứng dụng rồi quét lại mã QR của xã.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * DANH BẠ CÁN BỘ XÃ — công khai; chỉ người đã đồng ý công khai
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const DIRECTORY = {
  title: "Danh bạ cán bộ xã",
  intro: "Bấm vào một số điện thoại để gọi cho cán bộ ấy.",
  loading: "Đang tải danh bạ cán bộ…",
  empty: "Hiện chưa có cán bộ nào được công khai số điện thoại trên ứng dụng. Bạn hãy đến trụ sở Ủy ban nhân dân xã để được hướng dẫn.",
  position: "Chức vụ",
  org_unit: "Bộ phận",
  office_phone: "Điện thoại cơ quan",
  mobile: "Điện thoại di động",
  call: (number: string) => `Gọi ${number}`,
  has_zalo: "Có dùng Zalo",
  retry_button: "Thử lại",
  network_error: "Không tải được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Thử lại.",
  server_error: "Hệ thống của xã đang bận nên chưa tải được danh bạ. Hãy chờ vài phút rồi bấm Thử lại.",
  invalid: "Chưa tải được danh bạ của xã. Hãy đóng ứng dụng rồi quét lại mã QR của xã.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * GIAO DIỆN APP RIÊNG CỦA XÃ — theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án chọn, 28/09/2026)
 *
 * Không câu nào nhắc "phần giới thiệu" hay "mã QR": app riêng không có cả hai.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const COMMUNE_APP_UI = {
  greeting: "Xin chào",
  tab_bar: "Các mục chính của ứng dụng",
  tab_home: "Trang chủ",
  tab_reports: "Phản ánh",
  tab_news: "Tin tức",
  tab_directory: "Danh bạ",
  send_prominent_button: "Gửi phản ánh",
  tile_send: "Gửi phản ánh",
  tile_lookup: "Tra cứu phiếu",
  tile_directory: "Danh bạ",
  section_reports: "Phản ánh của tôi",
  section_latest_news: "Tin tức mới",
  view_all: "Xem tất cả",
  latest_news_empty: "Chưa có tin nào được đăng.",
  // App riêng ĐÃ ở đúng xã; thứ còn thiếu là xác nhận NGƯỜI DÂN (tài khoản Zalo) — câu cũ "chưa đăng
  // nhập được với xã" làm người đọc tưởng app vào nhầm xã (chủ dự án hỏi đúng câu ấy, 28/09/2026).
  not_logged_in_title: "Tính năng đang được hoàn thiện",
  not_logged_in_short: "Gửi và theo dõi phản ánh cần xác nhận tài khoản Zalo của bạn. Tính năng này đang được hoàn thiện.",
  not_logged_in_full:
    "Gửi và theo dõi phản ánh cần xác nhận tài khoản Zalo của bạn. Tính năng này đang được hoàn thiện. Trong lúc chờ, bạn có thể đến Bộ phận tiếp nhận của Ủy ban nhân dân xã hoặc gọi điện cho cán bộ trong mục Danh bạ.",
  search_directory: "Tìm theo tên, chức vụ, bộ phận, thôn",
  staff_not_found: "Không tìm thấy cán bộ phù hợp.",
  call: "Gọi",
  call_person: (name: string) => `Gọi ${name}`,
} as const;

/** Directory lines of the commune app for the units a person heads (`CommuneDirectory.tsx` `unitHeadLine`). */
export const DIRECTORY_UNIT_HEAD = {
  head_of_village: "Trưởng thôn",
  /** Used when the unit name already says its kind: "Trưởng" + "thôn Hà Lam" / "tổ dân phố 3". */
  head_of: "Trưởng",
} as const;

/**
 * THE COMMUNE'S OFFICE — what the commune declared (`/commune-profiles`). A row appears only when the
 * commune filled it; nothing is shown in its place ("" is "not declared", never a default).
 */
export const COMMUNE_OFFICE = {
  title: "Ủy ban nhân dân xã",
  address: "Trụ sở",
  hours: "Giờ làm việc",
  hotline: "Đường dây nóng",
  call_hotline: (number: string) => `Gọi đường dây nóng ${number}`,
} as const;

/**
 * News type chips and the Sự kiện screen — the staff register's words for the same codes
 * (`web-admin/src/features/noi-dung/nhan-noi-dung.ts`), so a commune and its citizens name a type alike.
 */
export const NEWS_TYPE_LABEL = {
  "tin-tuc": "Tin tức",
  "su-kien": "Sự kiện",
  "thong-bao": "Thông báo",
  "truyen-thanh": "Truyền thanh",
  video: "Video",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * APP RIÊNG — ĐỦ MÀN theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án, 28/09/2026). Từ 29/09/2026 phiếu
 * phản ánh đi vào sổ thật của xã; các câu "phiếu chỉ trong máy" đã bị gỡ cùng phần ấy.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const COMMUNE_APP_SCREENS = {
  // Định danh
  identity_title: "Xác nhận tài khoản",
  identity_description: "Liên kết tài khoản Zalo để dùng đầy đủ dịch vụ của xã:",
  benefit_send: "Gửi phản ánh tới Ủy ban nhân dân xã và theo dõi kết quả",
  benefit_lookup: "Tra cứu tiến độ hồ sơ của bà con",
  benefit_news: "Nhận tin tức, thông báo của xã",
  link_button: "Tiếp tục với tài khoản Zalo",
  linking: "Đang liên kết…",
  link_failed: "Chưa liên kết được. Hãy kiểm tra mạng rồi bấm lại.",
  phone_commitment: "Số điện thoại chỉ dùng để tiếp nhận và phản hồi phản ánh của bà con.",
  // Trang chủ
  notices: "Thông báo",
  tile_record_lookup: "Tra cứu hồ sơ",
  tile_broadcast: "Truyền thanh",
  tile_video: "Video",
  tile_map: "Bản đồ tiện ích",
  no_reports_yet: "Bà con chưa gửi phản ánh nào.",
  // Phản ánh
  filter_all: "Tất cả",
  filter_empty: "Chưa có phản ánh nào ở trạng thái này.",
  filter_group: "Lọc phản ánh theo tình trạng",
  step_field: "Lĩnh vực",
  change: "Đổi",
  events_title: "Sự kiện",
  events_empty: "Chưa có sự kiện nào được đăng.",
  group_government: "Chính quyền số",
  group_information: "Thông tin – Truyền thông",
  view_more: "Xem thêm",
  tile_lookup_short: "Tra cứu",
  tile_news: "Tin tức",
  tile_events: "Sự kiện",
  greeting_name: (name: string) => `Xin chào, ${name}`,
  // The entry card (`CommuneHome`, 29/09/2026): the ONLY place the app asks Zalo for the name. It must say
  // what the name is for before Zalo's own dialog appears — policy 3.3.4 (`features/tinh-nang/khung.tsx`).
  name_card_title: "Điền sẵn họ tên khi gửi phản ánh",
  name_card_why:
    "Để bà con không phải gõ lại họ tên mỗi lần gửi phản ánh tới xã, ứng dụng xin phép dùng tên Zalo của bà con. Tên chỉ nằm trên điện thoại này và chỉ đi tới xã khi bà con tự bấm gửi phản ánh.",
  name_card_zalo_asks:
    "Bấm “Đồng ý” thì Zalo sẽ hỏi bà con thêm một lần. Không đồng ý thì bà con vẫn dùng ứng dụng bình thường và tự gõ họ tên khi gửi phản ánh.",
  name_card_agree: "Đồng ý",
  name_card_decline: "Không, tôi sẽ tự gõ tên",
  name_card_asking: "Đang chờ bà con trả lời Zalo…",
  no_name_yet: "Chưa xác định",
  filter_news_type: "Lọc tin theo loại",
  /** A type chip, or the Sự kiện tile, with nothing published of that type. */
  news_type_empty: (type: string) => `Xã chưa đăng tin nào thuộc loại “${type}”.`,
  related_news: "Tin liên quan",
  group_other: "Cán bộ khác",
  this_tile: "này",
  location_button: "Lấy vị trí hiện tại",
  location_again: "Lấy lại vị trí hiện tại",
  location_locating: "Đang lấy vị trí…",
  location_why: "Để cán bộ xã tìm đúng nơi xảy ra sự việc. Zalo sẽ hỏi bà con có đồng ý chia sẻ vị trí không.",
  // Prototype `AddressBlock.tsx:112`: "Đã ghi nhận vị trí (lat, lng)". Coordinates, never a guessed address.
  location_found: (coordinates: string) =>
    `Đã lấy vị trí hiện tại (${coordinates}). Bà con vẫn ghi rõ nơi xảy ra ở ô trên.`,
  location_denied: "Bà con chưa đồng ý chia sẻ vị trí. Bà con vẫn gửi được phản ánh — hãy ghi rõ nơi xảy ra ở ô trên.",
  location_outside_zalo: "Chỉ lấy được vị trí khi mở ứng dụng trong Zalo. Bà con hãy ghi rõ nơi xảy ra ở ô trên.",
  location_rate_limited:
    "Bà con đã thử lấy vị trí nhiều lần. Bà con chờ vài phút rồi bấm lại, hoặc ghi rõ nơi xảy ra ở ô trên.",
  location_retry: "Chưa lấy được vị trí. Bà con bấm lại nút, hoặc ghi rõ nơi xảy ra ở ô trên.",
  location_unavailable: "Ứng dụng tạm thời chưa lấy được vị trí. Bà con hãy ghi rõ nơi xảy ra ở ô trên.",
  cancel_title: "Huỷ gửi phản ánh?",
  cancel_question: "Nội dung bà con đã nhập sẽ không được lưu lại.",
  continue_editing: "Tiếp tục nhập",
  cancel: "Huỷ bỏ",
  sent_at: "Gửi lúc",
  incident_place: "Nơi xảy ra",
  description: "Mô tả",
  progress: "Tiến trình xử lý",
  step_sent: "Đã gửi phản ánh",
  step_awaiting_acknowledge: "Chờ Ủy ban nhân dân xã tiếp nhận",
  detail_title: "Chi tiết phản ánh",
  // Gửi phản ánh
  step: (number: number, total: number) => `Bước ${number}/${total}`,
  step_content: "Nội dung",
  step_confirm: "Xác nhận",
  field_title: "Tiêu đề",
  hint_title: "Ví dụ: Rác tồn đọng tại đầu ngõ 12",
  field_content: "Mô tả chi tiết",
  hint_content: "Mô tả sự việc, thời điểm xảy ra và mức độ ảnh hưởng.",
  field_address: "Nơi xảy ra sự việc",
  hint_address: "Ví dụ: Ngõ 12, thôn Đông",
  review: "Kiểm tra lại thông tin",
  next_button: "Tiếp tục",
  back_link: "Quay lại",
  send_button: "Gửi phản ánh",
  home_button: "Về trang chủ",
  // Tra cứu hồ sơ
  lookup_title: "Tra cứu hồ sơ một cửa",
  // Hai ô, cả hai bắt buộc (spec 05-nghiep-vu.md:148, chủ dự án 28/09/2026 "theo require"): số điện thoại
  // một mình thì cầm danh bạ là tra ra hàng xóm; bốn số cuối nằm trên giấy biên nhận của chính người nộp.
  field_profile_phone: "Số điện thoại đã khai khi nộp hồ sơ",
  hint_profile_phone: "Nhập đủ số, ví dụ 0900000000",
  field_last_four: "4 số cuối của số hồ sơ",
  hint_last_four: "In trên giấy biên nhận",
  lookup_tile_button: "Tra cứu",
  lookup_not_connected:
    "Ứng dụng chưa kết nối với hệ thống một cửa của xã, nên chưa tra được hồ sơ. Bà con hãy liên hệ Bộ phận một cửa của Ủy ban nhân dân xã.",
  lookup_needs_code: "Bà con nhập đủ số điện thoại và 4 số cuối của số hồ sơ.",
  // Màn chưa có dữ liệu
  broadcast_title: "Truyền thanh",
  broadcast_empty: "Chưa có bản tin truyền thanh nào trên ứng dụng.",
  video_title: "Video tuyên truyền",
  video_empty: "Chưa có video tuyên truyền nào trên ứng dụng.",
  // SRS M6.1.12 gọi màn phía dân là "Bản đồ tiện ích"; "bản đồ kinh tế số" là màn M5 của cán bộ.
  map_title: "Bản đồ tiện ích",
  map_empty: "Chưa có dữ liệu bản đồ tiện ích (chợ, trường, trạm y tế, di tích…) trên ứng dụng.",
  notices_none_yet:
    "Ứng dụng chưa gửi thông báo. Khi có, kết quả phản ánh và tin khẩn của xã sẽ gửi qua tin nhắn Zalo.",
  notices_empty: "Chưa có thông báo nào.",
  // Cá nhân
  tab_personal: "Cá nhân",
  personal_title: "Cá nhân",
  sample_account_label: "Tài khoản mẫu",
  utilities: "Tiện ích của tôi",
  report_count: (n: number) => `${n} phiếu đã gửi`,
  /** The list has more pages than loaded: the count is a floor, said as one. */
  report_count_more: (n: number) => `Hơn ${n} phiếu đã gửi`,
  /** Not loaded (no session in this open): no number is invented. */
  report_count_unknown: "Bấm để xem phản ánh đã gửi",
  lookup_history: "Tra cứu hồ sơ một cửa",
  lookup_history_note: "Tra cứu tiến độ hồ sơ của bà con",
  display: "Cài đặt hiển thị",
  font_size: "Cỡ chữ",
  font_size_medium: "Vừa",
  font_size_large: "Lớn",
  font_size_extra_large: "Rất lớn",
  font_size_preview: "Xem trước: kích thước chữ hiện tại",
  section_notices: "Thông báo",
  about_app: "Về ứng dụng",
  unit: "Đơn vị",
  log_out: "Đăng xuất",
  confirm_log_out: "Đăng xuất",
  decline: "Không",
} as const;

/**
 * PHẢN ÁNH TRONG APP RIÊNG — theo ADR 0050: chữ của prototype khách, xưng "bà con" (#6). Không có con số
 * hạn nào ở đây: hạn là việc của máy chủ (luật 10).
 */
export const COMMUNE_APP_REPORTS = {
  // Phản ánh của tôi khi CHƯA có phiên (29/09/2026): xem phiếu là việc cá nhân, nên chỉ mở sau lời giải
  // thích và cú bấm của bà con (`commune-session.ts`) — không bao giờ tự mở lúc vào app (ADR 0047:251).
  need_session_title: "Phản ánh bà con đã gửi",
  need_session_body:
    "Để xem các phản ánh đã gửi, xã cần xác nhận số điện thoại Zalo của bà con. Bấm nút dưới đây, ứng dụng sẽ nói rõ trước khi Zalo hỏi.",
  need_session_button: "Xem phản ánh của tôi",
  session_expired:
    "Phiên làm việc với xã đã hết hạn. Bà con bấm “Thử lại” để xác nhận lại số điện thoại rồi làm tiếp.",
  loading_ticket: "Đang tải phiếu…",
  // Step 1 of the send screen — the commune's field catalogue (`my-citizen-report-fields`). No fallback list.
  fields_loading: "Đang tải danh sách lĩnh vực của xã…",
  fields_unavailable:
    "Chưa tải được danh sách lĩnh vực của xã. Bà con chờ vài phút rồi bấm “Thử lại”. Nội dung bà con viết chưa bị mất.",
  fields_network: "Không tải được danh sách lĩnh vực vì mạng yếu hoặc mất kết nối. Bà con kiểm tra mạng rồi bấm “Thử lại”.",
  fields_server: "Hệ thống của xã đang gặp sự cố nên chưa tải được danh sách lĩnh vực. Bà con chờ vài phút rồi bấm “Thử lại”.",
  fields_empty:
    "Hiện xã chưa mở lĩnh vực nào để nhận phản ánh qua ứng dụng. Bà con hãy gọi điện cho xã hoặc đến Bộ phận tiếp nhận của Ủy ban nhân dân xã.",
  sending: "Đang gửi phản ánh tới xã…",
  acknowledge_by: (time: string) => `Cán bộ xã sẽ xem phiếu trước ${time} (giờ Việt Nam).`,
  lookup_title: "Tra cứu phiếu",
  lookup_hint: "Nhập mã phiếu bà con đã nhận",
  no_reports_yet: "Bà con chưa gửi phản ánh nào.",
  handling_unit: "Cán bộ tiếp nhận phản ánh",
  reopened: (n: number) => `Đã mở lại để xử lý tiếp (lần ${n})`,
  rated: "Đánh giá của bà con",
  rating_title: "Bà con đánh giá kết quả xử lý",
  rating_why: "Đánh giá giúp chính quyền biết việc đã được giải quyết đúng mong muốn chưa.",
  rated_stars: (n: number) => `Đã chấm ${n} trên 5 sao`,
  rate_stars: "Chấm điểm từ 1 đến 5 sao",
  tap_star: "Chạm vào sao để chấm điểm",
  comment: "Nhận xét thêm (không bắt buộc)",
  send_rating: "Gửi đánh giá",
  // Không nói ngưỡng sao mở lại: ngưỡng và số lần mở lại là cấu hình TỪNG XÃ (ADR 0008), không phải hằng.
  not_recorded: "Ứng dụng chưa nhận được thông tin này. Bà con hãy liên hệ Ủy ban nhân dân xã để hỏi.",
  // App riêng không có mã QR để quét lại (khác câu chung `DIRECTORY`).
  directory_empty:
    "Hiện chưa có cán bộ nào được công khai số điện thoại trên ứng dụng. Bà con hãy đến trụ sở Ủy ban nhân dân xã để được hướng dẫn.",
  directory_invalid: "Chưa tải được danh bạ của xã. Bà con hãy đóng ứng dụng rồi mở lại.",
  hide_name: "Giấu tên",
  report_code: "Mã phiếu",
  expected_done: "Dự kiến xử lý xong",
  progress: "Tiến trình xử lý",
  detail_title: "Chi tiết phản ánh",
  // ONE sentence for "no such code", "someone else's", "another commune's" — the server answers the same
  // 404 for all three, and the screen must not tell them apart (rule 4, forbidden #2).
  report_not_found:
    "Không tìm thấy phiếu với mã này. Bà con kiểm tra lại từng ký tự của mã. Nếu vẫn không thấy, hãy liên hệ Ủy ban nhân dân xã.",
  missing_code: "Bà con nhập mã phiếu để tra cứu.",
  step_description: "Mô tả",
  step_done: "Xong",
  location_why: "Giúp cán bộ tìm đúng nơi xảy ra sự việc.",
  missing_description: "Bà con mô tả sự việc trước khi gửi.",
  cancel_question: "Nội dung bà con đang nhập sẽ mất.",
  choose_field: "Bà con chọn lĩnh vực gần đúng nhất với sự việc.",
  incident: "Mô tả sự việc",
  hint_incident: "Sự việc gì, ở đâu, từ khi nào…",
  field: "Lĩnh vực",
  address: "Nơi xảy ra",
  hint_address: "Thôn, tổ, đường, số nhà…",
  reporter_name: "Họ và tên",
  hint_name: "Họ tên người gửi phản ánh",
  missing_reporter: "Bà con nhập họ tên người gửi, hoặc bật “Gửi ẩn danh”.",
  anonymous: "Gửi ẩn danh",
  anonymous_explanation: "Bật lên thì cán bộ không thấy họ tên và số điện thoại của bà con.",
  photo_required: "Ảnh hoặc video (bắt buộc, tối đa 5 tệp)",
  photo_coming_soon: "Ứng dụng chưa gửi được ảnh, video — tính năng sắp có. Trong lúc chờ, bà con mô tả thật rõ sự việc.",
  location_required: "Vị trí trên bản đồ (bắt buộc)",
  // 29/09/2026: the app now takes the current location (button under the address box); a MAP pin is still
  // not there, and a location is still not enforced (ADR 0050 #9 — the server keeps it optional, b5d17bb).
  location_coming_soon: "Bà con bấm “Lấy vị trí hiện tại” ở dưới để gửi kèm vị trí, và ghi rõ nơi xảy ra ở ô dưới.",
  phone_number: "Số điện thoại",
  hint_phone: "Để cán bộ liên hệ khi cần",
  required: "Bắt buộc: lĩnh vực, mô tả, ảnh hoặc video, vị trí, họ tên người gửi.",
  required_anonymous: "Bắt buộc: lĩnh vực, mô tả, ảnh hoặc video, vị trí.",
  send_to: (commune: string) => `Phản ánh sẽ gửi tới: ${commune}`,
  // Said only after a 201: the petition is in the commune's register and has its lookup code (rule 10 #1).
  done_title: "Đã gửi phản ánh",
  done_description: "Ủy ban nhân dân xã đã nhận phản ánh của bà con. Bà con giữ mã phiếu dưới đây để theo dõi.",
  your_report_code: "Mã phiếu của bà con",
  follow_up: "Theo dõi phiếu này",
  // Nháp đang soạn (ADR 0050 #7, prototype `NewFeedbackPage.tsx:426-453`) — chỉ app riêng của xã có nháp.
  draft_title: "Bà con có một phản ánh đang soạn dở. Tiếp tục?",
  draft_body: "Nội dung bà con đã nhập vẫn được giữ nguyên.",
  draft_resume: "Tiếp tục",
  draft_discard: "Bỏ nháp",
  draft_kept_on_phone:
    "Phản ánh đang soạn được giữ trên điện thoại này cho tới khi bà con gửi đi hoặc bỏ nháp.",
} as const;
