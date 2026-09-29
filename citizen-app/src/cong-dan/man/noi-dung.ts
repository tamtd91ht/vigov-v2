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
 * `giai_thich` là DÒNG PHỤ cho người dân, không thay nhãn. Với bốn nhóm, dòng phụ là chỗ DUY NHẤT phân
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
export const TRANG_THAI: Readonly<Record<string, { giai_thich: string | null }>> = {
  "da-tiep-nhan": { giai_thich: "Đã gửi, đang chờ cán bộ xã xem." },
  "dang-phan-loai": { giai_thich: "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không." },
  "da-chuyen-xu-ly": { giai_thich: null },
  "dang-xu-ly": { giai_thich: null },
  "da-xu-ly": { giai_thich: null },
  "cho-dan-xac-nhan": { giai_thich: null },
  "da-dong": { giai_thich: null },
  "khong-tiep-nhan": {
    giai_thich: "Ủy ban nhân dân xã không tiếp nhận phản ánh này. Lý do ghi ở dưới.",
  },
  "chuyen-cap-tren": {
    giai_thich:
      "Ủy ban nhân dân xã đã chuyển phản ánh tới cơ quan có thẩm quyền. Tên cơ quan ghi ở dưới.",
  },
};

export const TRANG_THAI_CHUA_CO_NHAN =
  "Trạng thái mới, ứng dụng chưa có tên gọi. Bạn hãy hỏi Ủy ban nhân dân xã và đọc mã tra cứu.";

/** Nhãn người dân thấy: một trong bốn nhóm, hoặc câu trung tính cho mã lạ (không đoán nhóm). */
export function nhanTrangThai(ma: string): string {
  const nhom = groupOf(ma);
  return nhom === null ? TRANG_THAI_CHUA_CO_NHAN : STATUS_GROUP_LABEL[nhom];
}

export function giaiThichTrangThai(ma: string): string | null {
  return TRANG_THAI[ma]?.giai_thich ?? null;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CHUNG CHO HAI MÀN
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Kênh chưa mở — câu hiện khi CHƯA CÓ PHIÊN ViGov (hôm nay: luôn luôn). Nói thật, và nói việc làm
 * được ngay bây giờ. Không mời "thử lại sau": bấm lại không đổi được gì.
 */
export const KENH_CHUA_MO = {
  tieu_de: "Kênh phản ánh của xã chưa mở trên ứng dụng này",
  cau: "Ứng dụng chưa gửi được phản ánh tới xã. Bạn hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã, hoặc gọi điện thoại cho xã để phản ánh.",
} as const;

export const NHAN_XA_DANG_GUI = "Đang làm việc với";

export const KHAN_CAP =
  "Việc khẩn cấp, cần giúp ngay: gọi 113 (Công an), 114 (Cứu hỏa), 115 (Cấp cứu). Phản ánh trên ứng dụng không được xử lý ngay lập tức.";

export const QUAY_LAI = "Quay lại";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * MÀN "GỬI PHẢN ÁNH"
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const GUI = {
  tieu_de: "Gửi phản ánh",
  nhan_noi_dung: "Nội dung phản ánh (bắt buộc)",
  goi_y_noi_dung: "Việc gì đang xảy ra? Bạn thấy từ khi nào?",
  nhan_dia_chi: "Nơi xảy ra (không bắt buộc)",
  goi_y_dia_chi: "Ví dụ: đầu ngõ, gần chợ, tên thôn.",
  nhan_ho_ten: "Họ và tên (không bắt buộc)",
  nhan_dien_thoai: "Số điện thoại để xã liên hệ lại (không bắt buộc)",
  an_danh: "Gửi ẩn danh",
  an_danh_bat: "Đang bật",
  an_danh_tat: "Đang tắt",
  an_danh_giai_thich:
    "Khi gửi ẩn danh, ứng dụng không gửi họ tên và số điện thoại. Cán bộ xử lý không thấy bạn là ai.",
  chua_ho_tro_anh: "Ứng dụng chưa hỗ trợ đính kèm ảnh.",
  nut_tiep: "Tiếp tục",
  thieu_noi_dung: "Bạn chưa viết nội dung phản ánh. Hãy viết vài câu về việc đang xảy ra.",
  qua_dai: (ten: string, toi_da: number) =>
    `Ô "${ten}" dài quá. Hãy viết gọn lại, tối đa ${toi_da.toLocaleString("vi-VN")} ký tự.`,

  xac_nhan_tieu_de: "Kiểm tra trước khi gửi",
  xac_nhan_cau: "Phản ánh sẽ được gửi tới:",
  xac_nhan_hau_qua:
    "Chỉ xã này nhận và xử lý phản ánh. Nếu việc xảy ra ở xã khác, xin đừng gửi tại đây mà liên hệ Ủy ban nhân dân xã nơi xảy ra sự việc.",
  nut_gui: (ten_xa: string) => `Gửi tới ${ten_xa}`,
  nut_sua: "Sửa lại",
  dang_gui: "Đang gửi phản ánh…",

  xong_tieu_de: "Đã gửi phản ánh",
  xong_ma: "Mã tra cứu của bạn",
  xong_giu_ma: "Hãy chép lại hoặc chụp màn hình mã này. Bạn cần mã để xem phiếu đi tới đâu.",
  se_xem_truoc: (moc: string) => `Phiếu sẽ được cán bộ xã xem trước ${moc} (giờ Việt Nam).`,
  gui_phieu_khac: "Gửi phản ánh khác",
  nut_gui_lai: "Gửi lại",
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
 * Câu cho từng nhánh không thành của lần gửi. `co_the_gui_lai` quyết định nút "Gửi lại" (CÙNG khoá
 * chống trùng — xem `api/lan-gui.ts`) có hiện hay không.
 */
export const LOI_GUI: Readonly<
  Record<
    | "het-phien"
    | "dang-xu-ly-truoc"
    | "khong-hop-le"
    | "kenh-chua-mo"
    | "loi-may-chu"
    | "loi-mang"
    | "khong-tao-duoc-khoa",
    { cau: string; co_the_gui_lai: boolean }
  >
> = {
  "het-phien": {
    cau: "Phiên làm việc đã hết hạn nên phản ánh chưa được gửi. Hãy đóng ứng dụng, mở lại rồi gửi lại.",
    co_the_gui_lai: false,
  },
  "dang-xu-ly-truoc": {
    cau: "Lần gửi trước của bạn đang được xử lý. Hãy chờ một phút rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    co_the_gui_lai: true,
  },
  "khong-hop-le": {
    cau: "Phản ánh chưa được gửi vì có ô chưa đúng. Hãy bấm Sửa lại, viết gọn nội dung rồi gửi lại.",
    co_the_gui_lai: false,
  },
  "kenh-chua-mo": {
    cau: "Ủy ban nhân dân xã chưa mở kênh nhận phản ánh trực tuyến. Phản ánh của bạn CHƯA được ghi nhận. Hãy liên hệ trực tiếp Ủy ban nhân dân xã.",
    co_the_gui_lai: false,
  },
  "loi-may-chu": {
    cau: "Hệ thống của xã đang gặp sự cố nên chưa nhận được phản ánh. Hãy chờ vài phút rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    co_the_gui_lai: true,
  },
  "loi-mang": {
    cau: "Không gửi được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Gửi lại. Phản ánh sẽ không bị gửi hai lần.",
    co_the_gui_lai: true,
  },
  "khong-tao-duoc-khoa": {
    cau: "Điện thoại này chưa gửi được phản ánh an toàn. Hãy cập nhật ứng dụng Zalo rồi thử lại, hoặc liên hệ trực tiếp Ủy ban nhân dân xã.",
    co_the_gui_lai: false,
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
 * MÀN "TRA CỨU PHIẾU"
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const TRA_CUU = {
  tieu_de: "Tra cứu phiếu của tôi",
  nhan_ma: "Mã tra cứu",
  goi_y_ma: "Mã được đưa cho bạn ngay khi gửi phản ánh.",
  nut_tra: "Tra cứu",
  dang_tra: "Đang tìm phiếu…",
  tim_thay: "Đã tìm thấy phiếu. Thông tin phiếu ở ngay bên dưới.",
  thieu_ma: "Bạn chưa nhập mã tra cứu. Hãy nhập mã được đưa khi gửi phản ánh.",
  /**
   * MỘT CÂU cho "không có mã này", "phiếu của người khác", "phiếu của xã khác" — máy chủ trả cùng
   * một 404, và màn hình không được tách chúng ra (luật 4, cấm #2).
   */
  khong_thay:
    "Không tìm thấy phiếu với mã này. Hãy kiểm tra lại từng ký tự của mã. Nếu vẫn không thấy, hãy liên hệ Ủy ban nhân dân xã.",
  het_phien: "Phiên làm việc đã hết hạn. Hãy đóng ứng dụng, mở lại rồi tra cứu lại.",
  loi_may_chu: "Hệ thống của xã đang gặp sự cố. Hãy chờ vài phút rồi tra cứu lại.",
  loi_mang: "Không tra được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi tra cứu lại.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐÁNH GIÁ KẾT QUẢ XỬ LÝ — trên phiếu mở ở màn tra cứu (ADR 0050 điểm 2, `PetitionRating.tsx`)
 *
 * KHÔNG CÂU NÀO NÓI NGƯỠNG SAO MỞ LẠI PHIẾU (ADR 0050: "Giao diện dân không nói ngưỡng"): người dân chấm
 * theo điều họ thấy, không theo hệ quả họ được báo trước. Khi máy chủ đã mở lại, câu `reopened` chỉ nói
 * SỰ VIỆC đã xảy ra — tình trạng mới đã hiện trên thẻ phiếu.
 *
 * Nhãn năm mức sao và câu "chạm vào sao" dùng chung với bản trải nghiệm (`star-picker.tsx`), không chép.
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
 * Nhãn trạng thái KHÔNG viết lại ở đây: thẻ dùng `nhanTrangThai`, cùng bảng màn tra cứu dùng. Nhãn
 * "Gửi lúc" / "Hạn xử lý xong" lấy từ `THE_PHIEU` để hai màn nói cùng một chữ cho cùng một mốc.
 *
 * KHÔNG CÓ CÂU "QUÁ HẠN": quá hạn đếm bằng GIỜ LÀM VIỆC (ADR 0007), và chỉ `identity` đếm được.
 * Màn này chỉ hiện mốc hạn cố định máy chủ đã ghi.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const CUA_TOI = {
  tieu_de: "Phản ánh của tôi",
  dang_tai: "Đang tải danh sách phản ánh…",
  trong: "Bạn chưa gửi phản ánh nào.",
  nut_xem: "Xem chi tiết",
  nut_xem_them: "Xem thêm",
  dang_tai_them: "Đang tải thêm…",
  nut_thu_lai: "Thử lại",
  het_danh_sach: "Đã hiện hết phản ánh của bạn.",
  het_phien: "Phiên làm việc đã hết hạn. Hãy đóng ứng dụng, mở lại rồi xem lại danh sách.",
  loi_may_chu: "Hệ thống của xã đang gặp sự cố. Hãy chờ vài phút rồi bấm Thử lại.",
  loi_mang: "Không tải được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Thử lại.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * THẺ PHIẾU — dùng chung cho màn kết quả gửi và màn tra cứu
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const THE_PHIEU = {
  trang_thai: "Tình trạng",
  linh_vuc: "Lĩnh vực",
  /** Lĩnh vực do CÁN BỘ chốt (ADR 0028). Chưa chốt là bình thường, không phải lỗi. */
  chua_phan_loai: "Cán bộ xã chưa phân loại",
  /** Có mã mà xã chưa đặt tên: KHÔNG hiện mã thô cho người dân. */
  da_phan_loai: "Đã được cán bộ xã phân loại",
  gui_luc: "Gửi lúc",
  han_xem: "Hạn cán bộ xem phiếu",
  han_xu_ly: "Hạn xử lý xong",
  /** `resolve_due = null`: chưa có cam kết nào — không bịa một ngày. */
  han_xu_ly_chua_co: "Chưa có. Hạn được ấn định sau khi cán bộ phân loại phiếu.",
  /** `acknowledge_due = null`: khoảng ấy không áp dụng cho phiếu này. */
  han_xem_khong_ap_dung: "Không áp dụng",
  noi_dung: "Nội dung",
  dia_chi: "Nơi xảy ra",
  dia_chi_trong: "Chưa rõ vị trí",
  nguoi_gui: "Người gửi",
  an_danh: "Gửi ẩn danh",
  khong_ghi_ten: "Không ghi họ tên",
  ket_qua: "Kết quả xử lý của xã",
  gio_vn: "(giờ Việt Nam)",
  /* Hai nhánh kết thúc (`khong-tiep-nhan`, `chuyen-cap-tren`) — chữ do cán bộ viết CHO người dân. */
  ly_do_khong_tiep_nhan: "Lý do xã không tiếp nhận",
  co_quan_tiep_nhan: "Cơ quan tiếp nhận",
  ly_do_chuyen: "Lý do chuyển",
  /** Việc làm tiếp: phiếu đã rời xã, ứng dụng không theo được nó tới cơ quan kia. */
  lien_he_co_quan:
    "Bạn có thể liên hệ trực tiếp cơ quan tiếp nhận ở trên để hỏi tiếp về phản ánh này.",
  /**
   * Máy chủ không gửi lý do / tên cơ quan dù phiếu ở nhánh kết thúc — lẽ ra không xảy ra (máy chủ bắt
   * buộc hai ô ấy khi cán bộ bấm). Không để ô trống im lặng: nói việc người dân làm được.
   */
  chua_ghi: "Ứng dụng chưa nhận được thông tin này. Bạn hãy liên hệ Ủy ban nhân dân xã để hỏi.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * XÁC NHẬN XÃ TỪ MÃ QR (ADR 0047 §Trả lời mục 4) — tra tên xã, rồi mở phiên sau khi xác nhận
 *
 * Mọi nhánh không thành đều về PHẦN GIỚI THIỆU kèm một câu — trừ `thu_lai`, nơi bấm lại có thể được.
 * Không câu nào nhắc tên miền, mã lỗi hay tên dịch vụ: người dân không làm gì được với chúng.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const XAC_NHAN_XA = {
  dang_tra: "Đang tìm xã theo mã QR bạn vừa quét…",
  /** Máy chủ nói "không xã nào" (hoặc từ chối tên miền trên mã). */
  khong_thay:
    "Mã QR này chưa dẫn tới xã nào trên ứng dụng. Bạn vẫn xem được phần giới thiệu. Nếu cần làm việc với xã, hãy quét mã QR dán tại trụ sở Ủy ban nhân dân xã.",
  /** Nền tảng tạm ngưng, lỗi máy chủ, mất mạng. */
  chua_ket_noi:
    "Chưa kết nối được tới hệ thống của xã. Bạn vẫn xem được phần giới thiệu. Hãy kiểm tra mạng, chờ ít phút rồi quét lại mã QR.",
  dang_mo: "Đang mở kênh làm việc với xã…",
  // Hai câu `chua_mo` · `ngoai_zalo` cũ đã bị gỡ 27/09/2026: hai nhánh ấy không còn về phần giới thiệu
  // mà mở tin tức và danh bạ của xã vừa xác nhận; câu thay thế là `CHUA_DANG_NHAP_XA` ở dưới.
  thu_lai:
    "Chưa mở được kênh làm việc với xã vì mạng yếu hoặc hệ thống đang bận. Hãy kiểm tra mạng rồi bấm “Đúng, tiếp tục” lần nữa.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * APP RIÊNG CỦA XÃ (`--vao-thang`, ADR 0047 §6) — không màn giới thiệu, không QR, không nút xác nhận
 *
 * Câu của đường QR ở trên nói "phần giới thiệu" và "quét lại mã QR" — cả hai đều không có trong app
 * riêng. Người dân ở đây chỉ làm được một việc: thử lại.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const APP_RIENG = {
  dang_mo: "Đang mở ứng dụng của xã…",
  /** Máy chủ nói tên miền của bản dựng không thuộc xã nào đang hoạt động. */
  khong_thay: "Ứng dụng này chưa được gắn với xã nào đang hoạt động. Vui lòng liên hệ Ủy ban nhân dân xã.",
  /** Nền tảng tạm ngưng, lỗi máy chủ, mất mạng. */
  chua_ket_noi: "Chưa kết nối được tới hệ thống của xã. Hãy kiểm tra mạng rồi bấm “Thử lại”.",
  thu_lai: "Thử lại",
} as const;

/**
 * ĐÃ XÁC NHẬN XÃ NHƯNG CHƯA CÓ PHIÊN — câu trên màn chọn việc, thay vì để ba lối phản ánh im lặng.
 *
 * Đứng NGAY TRÊN ba lối ấy: người dân đọc nó trước khi bấm. Ba lối vẫn còn (mỗi màn nói lại "kênh chưa
 * mở" và chỉ đường tới trụ sở), vì một nút biến mất không báo là thứ người lớn tuổi không tìm lại được.
 * `con_lai` chỉ hiện khi có tên miền — tức khi hai màn công khai thật sự có mặt ở dưới.
 */
export const CHUA_DANG_NHAP_XA = {
  cau: "Ứng dụng chưa đăng nhập được với xã trên điện thoại này, nên chưa gửi phản ánh, xem phản ánh của bạn hay tra cứu phiếu được. Để phản ánh, hãy đến Bộ phận tiếp nhận của Ủy ban nhân dân xã hoặc gọi điện thoại cho xã.",
  con_lai: "Bạn vẫn đọc được tin tức và danh bạ cán bộ của xã ở dưới.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TIN TỨC CỦA XÃ — công khai, văn bản thuần
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const TIN_XA = {
  tieu_de: "Tin tức của xã",
  dang_tai: "Đang tải tin của xã…",
  dang_tai_them: "Đang tải thêm tin…",
  trong: "Hiện chưa có tin nào được đăng trên ứng dụng.",
  nut_doc: "Đọc tin",
  nut_xem_them: "Xem thêm tin",
  nut_thu_lai: "Thử lại",
  het_danh_sach: "Đã hiện hết tin của xã.",
  ngay_dang: "Ngày đăng",
  chuyen_muc: "Chuyên mục",
  dang_tai_bai: "Đang mở tin…",
  khong_thay: "Tin này không còn trên ứng dụng. Hãy bấm Quay lại để xem các tin khác.",
  loi_mang: "Không tải được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Thử lại.",
  loi_may_chu: "Hệ thống của xã đang bận nên chưa tải được tin. Hãy chờ vài phút rồi bấm Thử lại.",
  /** Tên miền trên mã bị từ chối giữa chừng — thử lại không đổi được gì. */
  khong_hop_le: "Chưa tải được tin của xã. Hãy đóng ứng dụng rồi quét lại mã QR của xã.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * DANH BẠ CÁN BỘ XÃ — công khai; chỉ người đã đồng ý công khai
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const DANH_BA = {
  tieu_de: "Danh bạ cán bộ xã",
  gioi_thieu: "Bấm vào một số điện thoại để gọi cho cán bộ ấy.",
  dang_tai: "Đang tải danh bạ cán bộ…",
  trong: "Hiện chưa có cán bộ nào được công khai số điện thoại trên ứng dụng. Bạn hãy đến trụ sở Ủy ban nhân dân xã để được hướng dẫn.",
  chuc_vu: "Chức vụ",
  bo_phan: "Bộ phận",
  so_co_quan: "Điện thoại cơ quan",
  di_dong: "Điện thoại di động",
  goi: (so: string) => `Gọi ${so}`,
  co_zalo: "Có dùng Zalo",
  nut_thu_lai: "Thử lại",
  loi_mang: "Không tải được vì mạng yếu hoặc mất kết nối. Hãy kiểm tra mạng rồi bấm Thử lại.",
  loi_may_chu: "Hệ thống của xã đang bận nên chưa tải được danh bạ. Hãy chờ vài phút rồi bấm Thử lại.",
  khong_hop_le: "Chưa tải được danh bạ của xã. Hãy đóng ứng dụng rồi quét lại mã QR của xã.",
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * GIAO DIỆN APP RIÊNG CỦA XÃ — theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án chọn, 28/09/2026)
 *
 * Không câu nào nhắc "phần giới thiệu" hay "mã QR": app riêng không có cả hai.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const XA_GIAO_DIEN = {
  chao: "Xin chào",
  thanh_tab: "Các mục chính của ứng dụng",
  tab_trang_chu: "Trang chủ",
  tab_phan_anh: "Phản ánh",
  tab_tin_tuc: "Tin tức",
  tab_danh_ba: "Danh bạ",
  nut_gui_noi: "Gửi phản ánh",
  o_gui: "Gửi phản ánh",
  o_tra_cuu: "Tra cứu phiếu",
  o_danh_ba: "Danh bạ",
  muc_phan_anh: "Phản ánh của tôi",
  muc_tin_moi: "Tin tức mới",
  xem_tat_ca: "Xem tất cả",
  tin_moi_trong: "Chưa có tin nào được đăng.",
  // App riêng ĐÃ ở đúng xã; thứ còn thiếu là xác nhận NGƯỜI DÂN (tài khoản Zalo) — câu cũ "chưa đăng
  // nhập được với xã" làm người đọc tưởng app vào nhầm xã (chủ dự án hỏi đúng câu ấy, 28/09/2026).
  chua_dang_nhap_tieu_de: "Tính năng đang được hoàn thiện",
  chua_dang_nhap_ngan: "Gửi và theo dõi phản ánh cần xác nhận tài khoản Zalo của bạn. Tính năng này đang được hoàn thiện.",
  chua_dang_nhap_day_du:
    "Gửi và theo dõi phản ánh cần xác nhận tài khoản Zalo của bạn. Tính năng này đang được hoàn thiện. Trong lúc chờ, bạn có thể đến Bộ phận tiếp nhận của Ủy ban nhân dân xã hoặc gọi điện cho cán bộ trong mục Danh bạ.",
  tim_danh_ba: "Tìm theo tên, chức vụ, bộ phận",
  khong_thay_can_bo: "Không tìm thấy cán bộ phù hợp.",
  goi: "Gọi",
  goi_ai: (ten: string) => `Gọi ${ten}`,
} as const;

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * APP RIÊNG — BẢN TRẢI NGHIỆM ĐỦ MÀN theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án, 28/09/2026)
 *
 * Người dùng giả lập và phiếu chỉ trong máy (`trai-nghiem.ts`). Mọi chỗ ấy nói thẳng "bản trải
 * nghiệm": người dân không được tưởng một phiếu chưa gửi đã tới xã.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const XA_TN = {
  nhan_trai_nghiem: "BẢN TRẢI NGHIỆM",
  // Định danh
  dinh_danh_tieu_de: "Xác nhận tài khoản",
  dinh_danh_mo_ta: "Liên kết tài khoản Zalo để dùng đầy đủ dịch vụ của xã:",
  loi_ich_gui: "Gửi phản ánh tới Ủy ban nhân dân xã và theo dõi kết quả",
  loi_ich_tra_cuu: "Tra cứu tiến độ hồ sơ của bà con",
  loi_ich_tin: "Nhận tin tức, thông báo của xã",
  nut_lien_ket: "Tiếp tục với tài khoản Zalo",
  dang_lien_ket: "Đang liên kết…",
  lien_ket_loi: "Chưa liên kết được. Hãy kiểm tra mạng rồi bấm lại.",
  ghi_chu_dinh_danh:
    "Bản trải nghiệm: ứng dụng dùng một tên và số điện thoại mẫu, chưa lấy từ Zalo. Không có thông tin nào của bà con được gửi đi.",
  cam_ket_so: "Số điện thoại chỉ dùng để tiếp nhận và phản hồi phản ánh của bà con.",
  // Trang chủ
  thong_bao: "Thông báo",
  o_tra_cuu_ho_so: "Tra cứu hồ sơ",
  o_truyen_thanh: "Truyền thanh",
  o_video: "Video",
  o_ban_do: "Bản đồ tiện ích",
  chua_co_phieu: "Bà con chưa gửi phản ánh nào.",
  // Phản ánh
  loc_tat_ca: "Tất cả",
  loc_trong: "Chưa có phản ánh nào ở trạng thái này.",
  loc_nhom: "Lọc phản ánh theo tình trạng",
  buoc_linh_vuc: "Lĩnh vực",
  doi: "Đổi",
  su_kien_tieu_de: "Sự kiện",
  su_kien_trong: "Chưa có sự kiện nào được đăng.",
  nhom_chinh_quyen: "Chính quyền số",
  nhom_thong_tin: "Thông tin – Truyền thông",
  xem_them: "Xem thêm",
  o_tra_cuu_ngan: "Tra cứu",
  o_tin_tuc: "Tin tức",
  o_su_kien: "Sự kiện",
  xin_chao_ten: (ten: string) => `Xin chào, ${ten}`,
  // The entry card (`TrangXa`, 29/09/2026): the ONLY place the app asks Zalo for the name. It must say
  // what the name is for before Zalo's own dialog appears — policy 3.3.4 (`features/tinh-nang/khung.tsx`).
  name_card_title: "Điền sẵn họ tên khi gửi phản ánh",
  name_card_why:
    "Để bà con không phải gõ lại họ tên mỗi lần gửi phản ánh tới xã, ứng dụng xin phép dùng tên Zalo của bà con. Tên chỉ nằm trên điện thoại này và chỉ đi tới xã khi bà con tự bấm gửi phản ánh.",
  name_card_zalo_asks:
    "Bấm “Đồng ý” thì Zalo sẽ hỏi bà con thêm một lần. Không đồng ý thì bà con vẫn dùng ứng dụng bình thường và tự gõ họ tên khi gửi phản ánh.",
  name_card_agree: "Đồng ý",
  name_card_decline: "Không, tôi sẽ tự gõ tên",
  name_card_asking: "Đang chờ bà con trả lời Zalo…",
  chua_co_ten: "Chưa xác định",
  so_tu_go: "Ứng dụng không lấy số điện thoại từ Zalo. Nếu muốn xã gọi lại, bà con hãy tự nhập số.",
  loc_chuyen_muc: "Lọc tin theo chuyên mục",
  tin_lien_quan: "Tin liên quan",
  nhom_khac: "Cán bộ khác",
  chua_tinh_han: "Bản trải nghiệm chưa tính hạn. Khi gửi thật, hạn được tính theo giờ làm việc của xã.",
  ghi_chu_tra_cuu_tn: "Bản trải nghiệm: chỉ tra được các phiếu bà con đã gửi trên điện thoại này trong lần mở ứng dụng này.",
  o_nay: "này",
  vi_tri_nut: "Lấy vị trí hiện tại",
  location_again: "Lấy lại vị trí hiện tại",
  vi_tri_dang_lay: "Đang lấy vị trí…",
  vi_tri_vi_sao: "Để cán bộ xã tìm đúng nơi xảy ra sự việc. Zalo sẽ hỏi bà con có đồng ý chia sẻ vị trí không.",
  // Prototype `AddressBlock.tsx:112`: "Đã ghi nhận vị trí (lat, lng)". Coordinates, never a guessed address.
  location_found: (coordinates: string) =>
    `Đã lấy vị trí hiện tại (${coordinates}). Bà con vẫn ghi rõ nơi xảy ra ở ô trên.`,
  vi_tri_tu_choi: "Bà con chưa đồng ý chia sẻ vị trí. Bà con vẫn gửi được phản ánh — hãy ghi rõ nơi xảy ra ở ô trên.",
  vi_tri_ngoai_zalo: "Chỉ lấy được vị trí khi mở ứng dụng trong Zalo. Bà con hãy ghi rõ nơi xảy ra ở ô trên.",
  location_rate_limited:
    "Bà con đã thử lấy vị trí nhiều lần. Bà con chờ vài phút rồi bấm lại, hoặc ghi rõ nơi xảy ra ở ô trên.",
  vi_tri_khong_lay_duoc: "Chưa lấy được vị trí. Bà con bấm lại nút, hoặc ghi rõ nơi xảy ra ở ô trên.",
  location_unavailable: "Ứng dụng tạm thời chưa lấy được vị trí. Bà con hãy ghi rõ nơi xảy ra ở ô trên.",
  hoi_huy_tieu_de: "Huỷ gửi phản ánh?",
  hoi_huy_cau: "Nội dung bà con đã nhập sẽ không được lưu lại.",
  tiep_tuc_nhap: "Tiếp tục nhập",
  huy_bo: "Huỷ bỏ",
  gui_luc: "Gửi lúc",
  noi_xay_ra: "Nơi xảy ra",
  mo_ta: "Mô tả",
  tien_trinh: "Tiến trình xử lý",
  buoc_da_gui: "Đã gửi phản ánh",
  buoc_cho_tiep_nhan: "Chờ Ủy ban nhân dân xã tiếp nhận",
  ghi_chu_phieu_tn:
    "Đây là phiếu của bản trải nghiệm: phiếu chỉ lưu trên điện thoại này, CHƯA được gửi tới Ủy ban nhân dân xã, và sẽ mất khi đóng ứng dụng.",
  chi_tiet_tieu_de: "Chi tiết phản ánh",
  // Gửi phản ánh
  buoc: (so: number, tong: number) => `Bước ${so}/${tong}`,
  buoc_noi_dung: "Nội dung",
  buoc_xac_nhan: "Xác nhận",
  o_tieu_de: "Tiêu đề",
  goi_y_tieu_de: "Ví dụ: Rác tồn đọng tại đầu ngõ 12",
  o_noi_dung: "Mô tả chi tiết",
  goi_y_noi_dung: "Mô tả sự việc, thời điểm xảy ra và mức độ ảnh hưởng.",
  o_dia_chi: "Nơi xảy ra sự việc",
  goi_y_dia_chi: "Ví dụ: Ngõ 12, thôn Đông",
  kiem_tra_lai: "Kiểm tra lại thông tin",
  nut_tiep: "Tiếp tục",
  nut_lui: "Quay lại",
  nut_gui: "Gửi phản ánh",
  nut_ve_trang_chu: "Về trang chủ",
  // Tra cứu hồ sơ
  tra_cuu_tieu_de: "Tra cứu hồ sơ một cửa",
  // Hai ô, cả hai bắt buộc (spec 05-nghiep-vu.md:148, chủ dự án 28/09/2026 "theo require"): số điện thoại
  // một mình thì cầm danh bạ là tra ra hàng xóm; bốn số cuối nằm trên giấy biên nhận của chính người nộp.
  o_so_dien_thoai_ho_so: "Số điện thoại đã khai khi nộp hồ sơ",
  goi_y_so_dien_thoai_ho_so: "Nhập đủ số, ví dụ 0900000000",
  o_bon_so_cuoi: "4 số cuối của số hồ sơ",
  goi_y_bon_so_cuoi: "In trên giấy biên nhận",
  nut_tra_cuu: "Tra cứu",
  tra_cuu_chua_ket_noi:
    "Ứng dụng chưa kết nối với hệ thống một cửa của xã, nên chưa tra được hồ sơ. Bà con hãy liên hệ Bộ phận một cửa của Ủy ban nhân dân xã.",
  tra_cuu_can_ma: "Bà con nhập đủ số điện thoại và 4 số cuối của số hồ sơ.",
  // Màn chưa có dữ liệu
  truyen_thanh_tieu_de: "Truyền thanh",
  truyen_thanh_trong: "Chưa có bản tin truyền thanh nào trên ứng dụng.",
  video_tieu_de: "Video tuyên truyền",
  video_trong: "Chưa có video tuyên truyền nào trên ứng dụng.",
  // SRS M6.1.12 gọi màn phía dân là "Bản đồ tiện ích"; "bản đồ kinh tế số" là màn M5 của cán bộ.
  ban_do_tieu_de: "Bản đồ tiện ích",
  ban_do_trong: "Chưa có dữ liệu bản đồ tiện ích (chợ, trường, trạm y tế, di tích…) trên ứng dụng.",
  thong_bao_chua_co:
    "Ứng dụng chưa gửi thông báo. Khi có, kết quả phản ánh và tin khẩn của xã sẽ gửi qua tin nhắn Zalo.",
  thong_bao_trong: "Chưa có thông báo nào.",
  // Cá nhân
  tab_ca_nhan: "Cá nhân",
  ca_nhan_tieu_de: "Cá nhân",
  nhan_tai_khoan_mau: "Tài khoản mẫu",
  tien_ich: "Tiện ích của tôi",
  so_phieu: (n: number) => `${n} phiếu đã gửi`,
  lich_su_tra_cuu: "Tra cứu hồ sơ một cửa",
  lich_su_tra_cuu_phu: "Tra cứu tiến độ hồ sơ của bà con",
  hien_thi: "Cài đặt hiển thị",
  co_chu: "Cỡ chữ",
  co_chu_vua: "Vừa",
  co_chu_lon: "Lớn",
  co_chu_rat_lon: "Rất lớn",
  xem_truoc_co_chu: "Xem trước: kích thước chữ hiện tại",
  muc_thong_bao: "Thông báo",
  ve_ung_dung: "Về ứng dụng",
  don_vi: "Đơn vị",
  dang_xuat: "Đăng xuất",
  hoi_dang_xuat: "Đăng xuất khỏi ứng dụng? Phiếu của bản trải nghiệm sẽ mất.",
  dong_y_dang_xuat: "Đăng xuất",
  huy: "Không",
} as const;

/**
 * PHẢN ÁNH TRONG APP RIÊNG — theo ADR 0050: chữ của prototype khách, xưng "bà con" (#6). Không có con số
 * hạn nào ở đây: hạn là việc của máy chủ (luật 10).
 */
export const XA_PA = {
  ghi_chu_phieu: "Phiếu của bản trải nghiệm chỉ nằm trên máy này, chưa gửi tới cán bộ. Đóng ứng dụng là mất.",
  ghi_chu_gui: "Bản trải nghiệm: phiếu chỉ lưu trong máy, chưa gửi tới cán bộ.",
  tra_cuu_tieu_de: "Tra cứu phiếu",
  tra_cuu_goi_y: "Nhập mã phiếu bà con đã nhận",
  chua_co_phieu: "Bà con chưa gửi phản ánh nào.",
  don_vi_xu_ly: "Cán bộ tiếp nhận phản ánh",
  da_mo_lai: (n: number) => `Đã mở lại để xử lý tiếp (lần ${n})`,
  da_danh_gia: "Đánh giá của bà con",
  danh_gia_tieu_de: "Bà con đánh giá kết quả xử lý",
  danh_gia_vi_sao: "Đánh giá giúp chính quyền biết việc đã được giải quyết đúng mong muốn chưa.",
  da_cham: (n: number) => `Đã chấm ${n} trên 5 sao`,
  cham_diem: "Chấm điểm từ 1 đến 5 sao",
  cham_vao_sao: "Chạm vào sao để chấm điểm",
  nhan_xet: "Nhận xét thêm (không bắt buộc)",
  gui_danh_gia: "Gửi đánh giá",
  // Không nói ngưỡng sao mở lại: ngưỡng và số lần mở lại là cấu hình TỪNG XÃ (ADR 0008), không phải hằng.
  chua_ghi: "Ứng dụng chưa nhận được thông tin này. Bà con hãy liên hệ Ủy ban nhân dân xã để hỏi.",
  // App riêng không có mã QR để quét lại (khác câu chung `DANH_BA`).
  danh_ba_trong:
    "Hiện chưa có cán bộ nào được công khai số điện thoại trên ứng dụng. Bà con hãy đến trụ sở Ủy ban nhân dân xã để được hướng dẫn.",
  danh_ba_khong_hop_le: "Chưa tải được danh bạ của xã. Bà con hãy đóng ứng dụng rồi mở lại.",
  giau_ten: "Giấu tên",
  ma_phieu: "Mã phiếu",
  du_kien_xong: "Dự kiến xử lý xong",
  tien_trinh: "Tiến trình xử lý",
  chi_tiet_tieu_de: "Chi tiết phản ánh",
  khong_thay_phieu: "Không tìm thấy phiếu với mã này trên máy.",
  thieu_ma: "Bà con nhập mã phiếu để tra cứu.",
  buoc_mo_ta: "Mô tả",
  buoc_xong: "Xong",
  vi_tri_vi_sao: "Giúp cán bộ tìm đúng nơi xảy ra sự việc.",
  thieu_mo_ta: "Bà con mô tả sự việc trước khi gửi.",
  hoi_huy_cau: "Nội dung bà con đang nhập sẽ mất.",
  chon_linh_vuc: "Bà con chọn lĩnh vực gần đúng nhất với sự việc.",
  su_viec: "Mô tả sự việc",
  goi_y_su_viec: "Sự việc gì, ở đâu, từ khi nào…",
  linh_vuc: "Lĩnh vực",
  dia_chi: "Nơi xảy ra",
  goi_y_dia_chi: "Thôn, tổ, đường, số nhà…",
  ten_nguoi_pa: "Họ và tên",
  goi_y_ten: "Họ tên người gửi phản ánh",
  thieu_nguoi_gui: "Bà con nhập họ tên người gửi, hoặc bật “Gửi ẩn danh”.",
  an_danh: "Gửi ẩn danh",
  an_danh_giai_thich: "Bật lên thì cán bộ không thấy họ tên và số điện thoại của bà con.",
  tac_phong_rieng:
    "Phản ánh về thái độ, tác phong cán bộ chỉ lãnh đạo xã được xem và không bao giờ hiện công khai.",
  anh_bat_buoc: "Ảnh hoặc video (bắt buộc, tối đa 5 tệp)",
  anh_sap_co: "Ứng dụng chưa gửi được ảnh, video — tính năng sắp có. Trong lúc chờ, bà con mô tả thật rõ sự việc.",
  vi_tri_bat_buoc: "Vị trí trên bản đồ (bắt buộc)",
  // 29/09/2026: the app now takes the current location (button under the address box); a MAP pin is still
  // not there, and a location is still not enforced (ADR 0050 #9 — the server keeps it optional, b5d17bb).
  vi_tri_sap_co: "Bà con bấm “Lấy vị trí hiện tại” ở dưới để gửi kèm vị trí, và ghi rõ nơi xảy ra ở ô dưới.",
  so_dien_thoai: "Số điện thoại",
  goi_y_so: "Để cán bộ liên hệ khi cần",
  bat_buoc: "Bắt buộc: lĩnh vực, mô tả, ảnh hoặc video, vị trí, họ tên người gửi.",
  bat_buoc_an_danh: "Bắt buộc: lĩnh vực, mô tả, ảnh hoặc video, vị trí.",
  gui_toi: (xa: string) => `Phản ánh sẽ gửi tới: ${xa}`,
  // Bản trải nghiệm KHÔNG gửi gì: câu này không được nói "đã gửi" hay hứa cán bộ phản hồi.
  xong_tieu_de: "Đã lưu phản ánh (bản trải nghiệm)",
  xong_mo_ta: "Phản ánh chỉ nằm trên máy này, cán bộ chưa nhận được. Khi ứng dụng kết nối xong, phản ánh sẽ gửi thẳng tới cán bộ.",
  ma_phieu_cua_ba_con: "Mã phiếu của bà con",
  theo_doi: "Theo dõi phiếu này",
  // Nháp đang soạn (ADR 0050 #7, prototype `NewFeedbackPage.tsx:426-453`) — chỉ app riêng của xã có nháp.
  draft_title: "Bà con có một phản ánh đang soạn dở. Tiếp tục?",
  draft_body: "Nội dung bà con đã nhập vẫn được giữ nguyên.",
  draft_resume: "Tiếp tục",
  draft_discard: "Bỏ nháp",
  draft_kept_on_phone:
    "Phản ánh đang soạn được giữ trên điện thoại này cho tới khi bà con gửi đi hoặc bỏ nháp.",
} as const;
