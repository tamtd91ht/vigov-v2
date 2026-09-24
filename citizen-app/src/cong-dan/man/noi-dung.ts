/**
 * MỌI CÂU CHỮ CỦA HAI MÀN "GỬI PHẢN ÁNH" VÀ "TRA CỨU PHIẾU" — một tệp, kể cả bảng nhãn trạng thái.
 *
 * Người đọc là công dân, thường lớn tuổi, thường đang bực vì chính việc họ báo (`skills/
 * accessibility-elderly`): câu ngắn, một ý, không viết tắt, không mã lỗi, luôn nói việc cần làm tiếp.
 */

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CHÍN TRẠNG THÁI (ADR 0027) — nhãn lấy NGUYÊN VĂN bảng khách đã duyệt
 * (`kb/00-foundation/ubiquitous-language.md` §"Chín trạng thái"; cùng bảng `web-admin/src/features/
 * phan-anh/nhan-phieu.ts`). Xã KHÔNG đổi nhãn (ADR 0027, bổ sung 20/09 cuối ngày).
 *
 * ⚠ BẢN CHÉP TAY, VÌ HỢP ĐỒNG KHAI `status` LÀ `string` TRƠN — cùng lỗ hổng `nhan-phieu.ts` đã báo.
 * Mã lạ không hiện nguyên mã (một chuỗi `dang-xu-ly` vô nghĩa với người dân) mà hiện một câu trung
 * tính kèm việc cần làm.
 *
 * `giai_thich` là DÒNG PHỤ cho người dân, không thay nhãn. Bốn dòng có câu:
 *   `da-tiep-nhan`   "Đã gửi, đang chờ cán bộ xã xem." — theo góp ý nghiệp vụ của lượt này: phần mềm
 *                    tự sinh trạng thái ấy, chưa ai ở xã đọc phiếu, nên "đã tiếp nhận" trần dễ bị
 *                    hiểu là xã đã nhận việc.
 *   `dang-phan-loai` câu duy nhất đặc tả cho (`docs/ui-ux/09` §8.2).
 *   `khong-tiep-nhan`, `chuyen-cap-tren` — hai nhánh KẾT THÚC: phiếu dừng ở xã, và người dân không
 *                    có bước nào khác trên ứng dụng. Câu chỉ nói sự việc và chỉ xuống lý do / cơ quan
 *                    nhận mà thẻ phiếu hiện ngay bên dưới (migration 0011). Chưa qua khách duyệt câu chữ.
 * Năm dòng còn lại để `null` — không bịa câu chưa ai duyệt, đúng khuôn `cauGiaiThichTrangThai`.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */
export const TRANG_THAI: Readonly<Record<string, { nhan: string; giai_thich: string | null }>> = {
  "da-tiep-nhan": { nhan: "Đã tiếp nhận", giai_thich: "Đã gửi, đang chờ cán bộ xã xem." },
  "dang-phan-loai": {
    nhan: "Đang phân loại",
    giai_thich: "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không.",
  },
  "da-chuyen-xu-ly": { nhan: "Đã chuyển xử lý", giai_thich: null },
  "dang-xu-ly": { nhan: "Đang xử lý", giai_thich: null },
  "da-xu-ly": { nhan: "Đã xử lý", giai_thich: null },
  "cho-dan-xac-nhan": { nhan: "Chờ dân xác nhận", giai_thich: null },
  "da-dong": { nhan: "Đã đóng", giai_thich: null },
  "khong-tiep-nhan": {
    nhan: "Không tiếp nhận",
    giai_thich: "Ủy ban nhân dân xã không tiếp nhận phản ánh này. Lý do ghi ở dưới.",
  },
  "chuyen-cap-tren": {
    nhan: "Chuyển cấp trên",
    giai_thich:
      "Ủy ban nhân dân xã đã chuyển phản ánh tới cơ quan có thẩm quyền. Tên cơ quan ghi ở dưới.",
  },
};

export const TRANG_THAI_CHUA_CO_NHAN =
  "Trạng thái mới, ứng dụng chưa có tên gọi. Bạn hãy hỏi Ủy ban nhân dân xã và đọc mã tra cứu.";

export function nhanTrangThai(ma: string): string {
  return TRANG_THAI[ma]?.nhan ?? TRANG_THAI_CHUA_CO_NHAN;
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
    "Chỉ xã này nhận và xử lý phản ánh. Nếu việc xảy ra ở xã khác, hãy quay lại và đổi xã trước khi gửi.",
  nut_gui: (ten_xa: string) => `Gửi tới ${ten_xa}`,
  nut_sua: "Sửa lại",
  dang_gui: "Đang gửi phản ánh…",

  xong_tieu_de: "Đã gửi phản ánh",
  xong_ma: "Mã tra cứu của bạn",
  xong_giu_ma: "Hãy chép lại hoặc chụp màn hình mã này. Bạn cần mã để xem phiếu đi tới đâu.",
  se_xem_truoc: (moc: string) => `Phiếu sẽ được cán bộ xã xem trước ${moc} (giờ Việt Nam).`,
  gui_phieu_khac: "Gửi phản ánh khác",
  nut_gui_lai: "Gửi lại",
} as const;

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
 * MÀN "TRA CỨU PHIẾU"
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const TRA_CUU = {
  tieu_de: "Tra cứu phiếu của tôi",
  nhan_ma: "Mã tra cứu",
  goi_y_ma: "Mã được đưa cho bạn ngay khi gửi phản ánh.",
  nut_tra: "Tra cứu",
  dang_tra: "Đang tìm phiếu…",
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
