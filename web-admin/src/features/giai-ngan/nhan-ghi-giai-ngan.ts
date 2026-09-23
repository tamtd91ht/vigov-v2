/**
 * Câu chữ, phép đọc số tiền và phép dựng thân yêu cầu cho CHÍN TUYẾN GHI của phân hệ Giải ngân
 * (`docs/ui-ux/06-giai-ngan.md` §7 · §8 · §8.2 · §9).
 *
 * Hàm thuần: không gọi mạng, không dựng DOM, không đọc đồng hồ. Cùng khuôn `nhan-du-an.ts` ngay
 * bên cạnh, và cùng lý do: quyết định nào nằm trong một hàm thuần thì có bài kiểm đọc lại được.
 *
 * KHÔNG MỘT LUẬT NGHIỆP VỤ NÀO ĐƯỢC VIẾT LẠI Ở ĐÂY. Vòng đời chứng từ, ai mở khoá được, sửa xong
 * thì về trạng thái nào — tất cả do `service-finance` cưỡng chế, và câu từ chối của nó ra thẳng
 * màn hình NGUYÊN VĂN. Những gì ở đây chỉ trả lời một câu: **nút nào có nghĩa để vẽ ra**.
 */

import type { SuaChungTuVao, SuaDuAnVao, ThemChungTuVao, ThemDuAnVao } from "@/lib/api/giai-ngan";

/* ── Trần độ dài, chép từ máy chủ ──────────────────────────────────────────────────────────── */

/**
 * Trần độ dài các ô nhập, ĐÚNG BẰNG trần của `service-finance/internal/domain`.
 *
 * CHÉP Ở ĐÂY CHỈ ĐỂ Ô NHẬP DỪNG LẠI ĐÚNG CHỖ MÁY CHỦ SẼ DỪNG — máy chủ vẫn là nơi từ chối thật, và
 * nó từ chối bằng một câu nói rõ trần là bao nhiêu. Không có `maxLength` thì cán bộ gõ xong một
 * đoạn dài mới biết là không gửi được; có nó thì họ thấy giới hạn ngay lúc gõ.
 */
export const NOI_DUNG_CHUNG_TU_TOI_DA = 1000; // domain.NoiDungChungTuToiDa
export const DOI_TAC_TOI_DA = 300; // domain.DoiTacToiDa
export const SO_CHUNG_TU_TOI_DA = 100; // domain.SoChungTuToiDa
export const LY_DO_TOI_DA = 500; // domain.LyDoGoChungTuToiDa · LyDoMoKhoaToiDa · LyDoXoaDuAnToiDa
export const MA_DU_AN_TOI_DA = 100; // domain.MaDuAnToiDa
export const TEN_DU_AN_TOI_DA = 500; // domain.TenDuAnToiDa
export const MO_TA_DU_AN_TOI_DA = 5000; // domain.MoTaDuAnToiDa

/* ── Số tiền ───────────────────────────────────────────────────────────────────────────────── */

/**
 * Đọc một ô tiền người dùng gõ thành SỐ ĐỒNG.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * KHÔNG ĐỔI ĐƠN VỊ, KHÔNG LÀM TRÒN, KHÔNG NHẬN PHẦN THẬP PHÂN. Đơn vị của hợp đồng là **đồng**, và
 * cột `so_tien` là `bigint`. Một ô nhận "1,5" rồi nhân lên là một màn hình tự định nghĩa đơn vị của
 * riêng nó; một ô làm tròn là một con số ngân sách sai đi vài đồng ở mọi hàng.
 *
 * VÀ ĐÂY LÀ CA KHÔNG AI NGHĨ TỚI: trần của máy chủ là `10^17` đồng, trong khi số nguyên an toàn của
 * JavaScript dừng ở `2^53-1` ≈ `9,007 × 10^15`. Một chuỗi mười tám chữ số đi qua `Number(...)` cho
 * ra một con số KHÁC, im lặng, và `JSON.stringify` gửi con số khác ấy lên máy chủ — máy chủ nhận
 * một `bigint` hợp lệ, ghi nó vào hồ sơ lưu trữ, và không có gì đỏ ở bất kỳ đâu. Nên ca ấy bị TỪ
 * CHỐI ở đây bằng một câu riêng, không gộp vào "không phải số".
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */
export type SoTienDaDoc =
  | { loai: "trong" }
  | { loai: "khongPhaiSo" }
  | { loai: "vuotChinhXac" }
  | { loai: "so"; dong: number };

/**
 * Bỏ khoảng trắng và dấu chấm phân cách hàng nghìn — `100.000.000` là cách một cán bộ Việt Nam gõ
 * một trăm triệu, và từ chối nó là từ chối cách viết thông thường. **Dấu phẩy KHÔNG bị bỏ**: ở
 * `vi-VN` nó là dấu thập phân, nên "1,5" phải rơi vào nhánh "không phải số" thay vì lặng lẽ thành
 * mười lăm.
 */
export function docSoTien(nhap: string): SoTienDaDoc {
  const gon = nhap.trim().replace(/[\s.]/g, "");
  if (gon === "") return { loai: "trong" };
  if (!/^\d+$/.test(gon)) return { loai: "khongPhaiSo" };

  const so = Number(gon);
  if (!Number.isSafeInteger(so)) return { loai: "vuotChinhXac" };
  return { loai: "so", dong: so };
}

export const CAU_SO_TIEN_KHONG_DOC_DUOC =
  "Số tiền chỉ gồm chữ số, đơn vị là đồng. Được phép dùng dấu chấm phân cách hàng nghìn " +
  "(100.000.000), không nhận dấu phẩy và không nhận phần lẻ.";

export const CAU_SO_TIEN_VUOT_CHINH_XAC =
  "Số tiền này quá lớn để trình duyệt giữ chính xác từng đồng, nên màn hình không gửi đi. Hãy " +
  "kiểm tra lại con số — nếu nó đúng thật thì đây là việc phải báo, không phải việc gõ lại.";

/* ── Trạng thái chứng từ §8.2 ──────────────────────────────────────────────────────────────── */

/**
 * Ba trạng thái của vòng đời chứng từ, đúng ba chuỗi máy chủ phát ra (`domain.TrangThaiChungTu`).
 *
 * TIẾNG VIỆT KHÔNG DẤU LÀ GIÁ TRỊ TRÊN DÂY, tiếng Việt có dấu là thứ hiện ra — ADR 0011. Không
 * dịch ngược ở chỗ khác.
 */
export const CHUNG_TU_KE_TOAN_NHAP = "ke-toan-nhap";
export const CHUNG_TU_DA_XAC_NHAN = "da-xac-nhan";
export const CHUNG_TU_DA_KHOA = "da-khoa";

const NHAN_TRANG_THAI: Readonly<Record<string, string>> = {
  [CHUNG_TU_KE_TOAN_NHAP]: "Kế toán nhập",
  [CHUNG_TU_DA_XAC_NHAN]: "Đã xác nhận",
  [CHUNG_TU_DA_KHOA]: "Đã khoá",
};

/** Trạng thái lạ hiện NGUYÊN chuỗi máy chủ gửi: đoán một nhãn đẹp là giấu mất một lệch hợp đồng. */
export function nhanTrangThaiChungTu(ma: string): string {
  return NHAN_TRANG_THAI[ma] ?? ma;
}

/**
 * Lớp CSS theo trạng thái. MÀU KHÔNG PHẢI TÍN HIỆU DUY NHẤT — chữ trong chip đã nói đủ, và
 * `globals.css` nằm ngoài ranh giới ghi của lượt này nên không có lớp mới nào được thêm.
 */
export function lopTrangThaiChungTu(ma: string): string {
  switch (ma) {
    case CHUNG_TU_DA_XAC_NHAN:
      return "chip chip-hoat-dong";
    case CHUNG_TU_DA_KHOA:
      return "chip chip-lanh-dao";
    case CHUNG_TU_KE_TOAN_NHAP:
      return "chip chip-ngung";
    default:
      return "chip";
  }
}

/**
 * Thao tác nào CÓ NGHĨA ở trạng thái này — bảng chuyển trạng thái của `domain.ChoSua` · `ChoGo` ·
 * `ChoXacNhan` · `ChoKhoa` · `ChoMoKhoa`.
 *
 * ⚠ ĐÂY KHÔNG PHẢI LỚP CHẶN, VÀ KHÔNG ĐƯỢC ĐỌC NHƯ MỘT LỚP CHẶN. Máy chủ vẫn kiểm từng lần, và nó
 * trả **409** kèm nguyên câu vòng đời khi thao tác không hợp trạng thái. Bảng này chỉ để không vẽ
 * ra một cái nút chắc chắn 409 — ví dụ nút `Khoá` trên một chứng từ chưa ai xác nhận.
 *
 * TRẠNG THÁI LẠ THÌ ĐÓNG HẾT (fail closed, luật 1 cấm #1): một trạng thái thứ tư máy chủ thêm vào
 * mai này không được lặng lẽ thừa hưởng bộ nút của trạng thái nào cả.
 *
 * `moKhoa` KHÔNG XÉT "AI VỪA KHOÁ". Luật ấy là về NGƯỜI (`ErrTuMoKhoaChungTuMinhVuaKhoa`), và dựng
 * lại nó ở client cần so mã cán bộ của phiên với `locked_by` — một phép so mà nếu lệch kiểu định
 * danh thì KHÔNG BAO GIỜ đúng, và trông hệt như luật đã tắt. Để máy chủ từ chối, rồi hiện nguyên
 * câu của nó: câu ấy đã nói rõ phải nhờ một cán bộ khác.
 */
export type ThaoTacChungTu = {
  readonly sua: boolean;
  readonly go: boolean;
  readonly xacNhan: boolean;
  readonly khoa: boolean;
  readonly moKhoa: boolean;
};

const KHONG_THAO_TAC: ThaoTacChungTu = {
  sua: false,
  go: false,
  xacNhan: false,
  khoa: false,
  moKhoa: false,
};

export function thaoTacChungTu(trangThai: string): ThaoTacChungTu {
  switch (trangThai) {
    case CHUNG_TU_KE_TOAN_NHAP:
      // Chưa xác nhận thì chưa khoá được — vòng đời là một dây xích, không phải ba nút song song.
      return { sua: true, go: true, xacNhan: true, khoa: false, moKhoa: false };
    case CHUNG_TU_DA_XAC_NHAN:
      // Sửa VẪN ĐƯỢC, và chính vì thế phải cảnh báo: nó kéo chứng từ về `Kế toán nhập` (ADR 0036).
      return { sua: true, go: true, xacNhan: false, khoa: true, moKhoa: false };
    case CHUNG_TU_DA_KHOA:
      return { sua: false, go: false, xacNhan: false, khoa: false, moKhoa: true };
    default:
      return KHONG_THAO_TAC;
  }
}

/**
 * Câu BẮT BUỘC hiện ở chỗ bấm Sửa một chứng từ ĐÃ XÁC NHẬN — ADR 0036.
 *
 * KHÔNG ĐƯỢC NUỐT, KHÔNG ĐƯỢC ĐỂ NGƯỜI TA PHÁT HIỆN SAU. Máy chủ làm đúng việc ấy trong im lặng:
 * `PATCH` trả 200, và trạng thái trong phản hồi đã là `Kế toán nhập`. Một cán bộ sửa một dấu chấm
 * trong nội dung mà không được báo trước sẽ vừa xoá chữ xác nhận của lãnh đạo mà không hay — và
 * người phát hiện ra là người đi tìm chữ ký ấy lúc quyết toán.
 */
export const CANH_BAO_SUA_VE_NHAP =
  "Chứng từ này đang ở trạng thái Đã xác nhận. Lưu một thay đổi sẽ đưa nó VỀ Kế toán nhập và xoá " +
  "dấu người xác nhận — muốn xác nhận lại thì phải có người bấm lại.";

/** Câu ở chỗ bấm Khoá: nói trước khoá nghĩa là gì, vì sau đó không sửa và không gỡ được nữa. */
export const CANH_BAO_KHOA =
  "Khoá rồi thì chứng từ không sửa, không gỡ được nữa. Muốn sửa phải mở khoá, mở khoá cần lý do " +
  "và phải là một cán bộ khác — không phải chính người vừa khoá.";

/* ── Mốc thời gian ─────────────────────────────────────────────────────────────────────────── */

/** Ô rỗng. Chưa có gì để hiện thì **dấu gạch**. */
export const DAU_GACH = "—";

/**
 * Múi giờ GHIM cho mọi phép in MỐC của phân hệ này.
 *
 * HẰNG CỦA NỀN TẢNG, KHÔNG PHẢI GIÁ TRỊ CỦA MỘT XÃ (luật 8, bất biến 5): Việt Nam dùng một múi giờ
 * duy nhất, nên nó không khác nhau giữa 300 xã. Không ghim thì `locked_at` in ra giờ khác nhau
 * trên máy đặt múi giờ khác nhau — và `vitest.config.mts` ghim `TZ=UTC` đúng để một phép định dạng
 * quên `timeZone` phải đỏ ngay trên máy người viết.
 */
const MUI_GIO = "Asia/Ho_Chi_Minh";

const DINH_DANG_MOC = new Intl.DateTimeFormat("en-GB", {
  timeZone: MUI_GIO,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

function phanCua(
  phan: readonly Intl.DateTimeFormatPart[],
  loai: Intl.DateTimeFormatPartTypes,
): string {
  return phan.find((p) => p.type === loai)?.value ?? "";
}

/** `locked_at` (RFC 3339) → `14:05 22/09/2026`. Chuỗi không đọc được hiện NGUYÊN VĂN, không đoán. */
export function nhanMocKhoa(mocISO: string | undefined): string {
  if (mocISO === undefined || mocISO === "") return DAU_GACH;
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return mocISO;

  const phan = DINH_DANG_MOC.formatToParts(new Date(t));
  return (
    `${phanCua(phan, "hour")}:${phanCua(phan, "minute")} ` +
    `${phanCua(phan, "day")}/${phanCua(phan, "month")}/${phanCua(phan, "year")}`
  );
}

/* ── Thân yêu cầu ──────────────────────────────────────────────────────────────────────────── */

/**
 * Kết quả dựng một thân: hoặc thân gửi được, hoặc **một câu cho người dùng đọc**.
 *
 * KHÔNG TRẢ `null`: một `null` ở đây buộc mỗi chỗ gọi tự nghĩ ra câu giải thích, và câu ấy sẽ khác
 * nhau ở từng biểu mẫu.
 */
export type ThanDung<T> = { ok: true; than: T } | { ok: false; cau: string };

/** Giá trị các ô của biểu mẫu chứng từ §8.2 — CHUỖI hết, đúng như `<input>` cho ra. */
export type GiaTriFormChungTu = {
  readonly ngayChi: string; // YYYY-MM-DD, từ `<input type="date">`
  readonly soTien: string;
  readonly noiDung: string;
  readonly doiTac: string;
  readonly soChungTu: string;
};

export const FORM_CHUNG_TU_TRONG: GiaTriFormChungTu = {
  ngayChi: "",
  soTien: "",
  noiDung: "",
  doiTac: "",
  soChungTu: "",
};

export const CAU_THIEU_NGAY_CHI = "Chưa có ngày chi. Đây là ngày TIỀN RA, không phải ngày gõ vào sổ.";
export const CAU_THIEU_NOI_DUNG = "Chưa có nội dung chi.";
export const CAU_SO_TIEN_PHAI_DUONG = "Số tiền phải lớn hơn 0 đồng.";

/**
 * Dựng thân `POST /api/v1/disbursements` từ các ô của biểu mẫu.
 *
 * `project_id` ĐẾN TỪ ĐƯỜNG DẪN TRANG, KHÔNG TỪ MỘT Ô NHẬP: màn này luôn nằm trong một dự án cụ
 * thể. Một ô cho cán bộ gõ id dự án là một ô gõ nhầm được, và nhầm ở đây là ghi một khoản chi vào
 * dự án khác.
 *
 * KHÔNG CÓ `funding_source_id`: chưa có tuyến nào đọc danh mục nguồn vốn, nên không có ô chọn nào
 * dựng được — xem `PHAN_CHUA_DUNG_GHI`. Chứng từ chưa gắn nguồn là trạng thái §13 quy tắc 6 định
 * nghĩa và §6 báo cáo, không phải một lỗi.
 *
 * Ô TRỐNG KHÔNG ĐƯỢC GỬI THÀNH `""`: `counterparty` và `voucher_no` có `omitempty`, nên vắng mặt là
 * cách nói "chứng từ này không có số kho bạc". Gửi chuỗi rỗng cũng ra cùng kết quả ở máy chủ, nhưng
 * nó nói rằng client có một giá trị và giá trị ấy rỗng — hai điều khác nhau.
 */
export function thanThemChungTu(
  duAnID: string,
  gt: GiaTriFormChungTu,
): ThanDung<ThemChungTuVao> {
  if (gt.ngayChi === "") return { ok: false, cau: CAU_THIEU_NGAY_CHI };
  if (gt.noiDung.trim() === "") return { ok: false, cau: CAU_THIEU_NOI_DUNG };

  const tien = docSoTien(gt.soTien);
  switch (tien.loai) {
    case "trong":
    case "khongPhaiSo":
      return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
    case "vuotChinhXac":
      return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
    case "so":
      if (tien.dong <= 0) return { ok: false, cau: CAU_SO_TIEN_PHAI_DUONG };
      break;
  }

  const doiTac = gt.doiTac.trim();
  const soChungTu = gt.soChungTu.trim();

  return {
    ok: true,
    than: {
      project_id: duAnID,
      payment_date: gt.ngayChi,
      amount: tien.dong,
      description: gt.noiDung.trim(),
      counterparty: doiTac === "" ? undefined : doiTac,
      voucher_no: soChungTu === "" ? undefined : soChungTu,
    },
  };
}

export const CAU_KHONG_CO_GI_DOI =
  "Không có ô nào thay đổi, nên màn hình không gửi gì đi. Một lần `PATCH` rỗng vẫn là một lần ghi.";

/**
 * Dựng thân `PATCH /api/v1/disbursements/{id}` — **CHỈ những ô thật sự đổi**.
 *
 * VÌ SAO LÀ MỘT DELTA CHỨ KHÔNG GỬI CẢ BIỂU MẪU: mỗi trường là một con trỏ ở máy chủ, vắng nghĩa là
 * "để nguyên" còn `""` nghĩa là "xoá trắng". Gửi cả biểu mẫu thì không sai về giá trị, nhưng nó
 * biến một lần sửa nội dung thành một lần ghi đè mọi cột — và vết kiểm toán của máy chủ (trước/sau,
 * chỉ những trường đã đổi) sẽ ghi lại một danh sách trường mà không ai thật sự sửa.
 *
 * VÀ MỘT DELTA RỖNG BỊ CHẶN NGAY Ở ĐÂY: máy chủ có nhánh "không đổi gì thì không ghi gì", nhưng
 * trông cậy vào nhánh ấy là trông cậy vào một thứ không nhìn thấy được từ màn hình. Chặn ở đây thì
 * một cú bấm Lưu thừa không bao giờ đi tới chỗ có thể bóc chữ xác nhận của một chứng từ.
 */
export function thanSuaChungTu(
  dau: GiaTriFormChungTu,
  moi: GiaTriFormChungTu,
): ThanDung<SuaChungTuVao> {
  const than: {
    payment_date?: string;
    amount?: number;
    description?: string;
    counterparty?: string;
    voucher_no?: string;
  } = {};

  if (moi.ngayChi !== dau.ngayChi) {
    if (moi.ngayChi === "") return { ok: false, cau: CAU_THIEU_NGAY_CHI };
    than.payment_date = moi.ngayChi;
  }

  if (moi.soTien !== dau.soTien) {
    const tien = docSoTien(moi.soTien);
    switch (tien.loai) {
      case "trong":
      case "khongPhaiSo":
        return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
      case "vuotChinhXac":
        return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
      case "so":
        if (tien.dong <= 0) return { ok: false, cau: CAU_SO_TIEN_PHAI_DUONG };
        than.amount = tien.dong;
        break;
    }
  }

  if (moi.noiDung.trim() !== dau.noiDung.trim()) {
    if (moi.noiDung.trim() === "") return { ok: false, cau: CAU_THIEU_NOI_DUNG };
    than.description = moi.noiDung.trim();
  }

  // ⚠ Ở ĐÂY `""` LÀ MỘT GIÁ TRỊ, KHÔNG PHẢI MỘT Ô TRỐNG BỊ BỎ QUA: xoá trắng ô đối tác là một lần
  // sửa có thật ("khoản này không có đối tác"), và nó phải đi lên máy chủ.
  if (moi.doiTac.trim() !== dau.doiTac.trim()) than.counterparty = moi.doiTac.trim();
  if (moi.soChungTu.trim() !== dau.soChungTu.trim()) than.voucher_no = moi.soChungTu.trim();

  if (Object.keys(than).length === 0) return { ok: false, cau: CAU_KHONG_CO_GI_DOI };
  return { ok: true, than };
}

/* ── Dự án §9 · §8 ─────────────────────────────────────────────────────────────────────────── */

/** Giá trị các ô của biểu mẫu dự án §9. CHUỖI hết. */
export type GiaTriFormDuAn = {
  readonly ma: string;
  readonly hangMucID: string;
  readonly ten: string;
  readonly moTa: string;
  readonly keHoachVon: string;
  readonly tongMucDuyet: string;
  readonly ngayKhoiCong: string;
  readonly ngayHoanThanh: string;
  readonly thoiHanGiaiNgan: string;
};

export const FORM_DU_AN_TRONG: GiaTriFormDuAn = {
  ma: "",
  hangMucID: "",
  ten: "",
  moTa: "",
  keHoachVon: "",
  tongMucDuyet: "",
  ngayKhoiCong: "",
  ngayHoanThanh: "",
  thoiHanGiaiNgan: "",
};

export const CAU_THIEU_MA_DU_AN =
  "Chưa có mã dự án. Hệ thống CHƯA tự sinh mã (ô `Tự sinh mã` của bản thiết kế chưa dựng được), " +
  "nên mã phải nhập tay: chỉ chữ cái, chữ số và dấu gạch nối.";
export const CAU_THIEU_HANG_MUC =
  "Chưa chọn hạng mục. Báo cáo tiến độ cộng dồn theo hạng mục nên mỗi dự án thuộc đúng một hạng mục.";
export const CAU_THIEU_TEN_DU_AN = "Chưa có tên dự án.";
export const CAU_KE_HOACH_VON_AM = "Kế hoạch vốn năm không được âm.";

/**
 * Dựng thân `POST /api/v1/investment-projects` từ các ô của biểu mẫu §9.
 *
 * `year` ĐẾN TỪ Ô CHỌN NĂM NGÂN SÁCH CỦA MÀN, không từ đồng hồ máy: một dự án gõ vào ngày 02/01
 * thuộc năm ngân sách nào là chuyện cán bộ quyết, và mỗi năm là một tập dự án riêng (§13 quy tắc 8).
 *
 * `approved_amount` VẮNG MẶT KHI Ô TRỐNG, chứ không gửi 0: §9 nói *"để trống thì lấy bằng số tiền
 * bố trí năm nay"*, và máy chủ áp đúng quy tắc ấy (`0` nghĩa là "same as this year's plan"). Gửi 0
 * và để máy chủ suy là cùng kết quả, nhưng vắng mặt mới là điều biểu mẫu đang nói.
 *
 * KHÔNG CÓ `org_unit_id`, `assignee_id`, `funding_allocations` — xem `PHAN_CHUA_DUNG_GHI`.
 */
export function thanThemDuAn(nam: number, gt: GiaTriFormDuAn): ThanDung<ThemDuAnVao> {
  const ma = gt.ma.trim();
  if (ma === "") return { ok: false, cau: CAU_THIEU_MA_DU_AN };
  if (gt.hangMucID === "") return { ok: false, cau: CAU_THIEU_HANG_MUC };
  if (gt.ten.trim() === "") return { ok: false, cau: CAU_THIEU_TEN_DU_AN };

  const keHoach = docSoTien(gt.keHoachVon);
  let keHoachDong = 0;
  switch (keHoach.loai) {
    case "trong":
    case "khongPhaiSo":
      return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
    case "vuotChinhXac":
      return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
    case "so":
      keHoachDong = keHoach.dong;
      break;
  }

  const tongMuc = docSoTien(gt.tongMucDuyet);
  let tongMucDong: number | undefined;
  switch (tongMuc.loai) {
    case "trong":
      tongMucDong = undefined;
      break;
    case "khongPhaiSo":
      return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
    case "vuotChinhXac":
      return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
    case "so":
      tongMucDong = tongMuc.dong;
      break;
  }

  const moTa = gt.moTa.trim();

  return {
    ok: true,
    than: {
      code: ma,
      year: nam,
      category_id: gt.hangMucID,
      name: gt.ten.trim(),
      description: moTa === "" ? undefined : moTa,
      planned_amount: keHoachDong,
      approved_amount: tongMucDong,
      start_date: gt.ngayKhoiCong === "" ? undefined : gt.ngayKhoiCong,
      completion_date: gt.ngayHoanThanh === "" ? undefined : gt.ngayHoanThanh,
      disbursement_deadline: gt.thoiHanGiaiNgan === "" ? undefined : gt.thoiHanGiaiNgan,
    },
  };
}

/**
 * Dựng thân `PATCH /api/v1/investment-projects/{id}` — **CHỈ những ô thật sự đổi**.
 *
 * `ma` VÀ NĂM KHÔNG CÓ MẶT: cả hai là 400 ở máy chủ và đã bị `Omit` khỏi kiểu `SuaDuAnVao`. Biểu
 * mẫu sửa vẫn HIỆN mã dự án, nhưng hiện để đọc — mã đã cấp thì không đánh lại (luật 7, cấm #4).
 *
 * Ô NGÀY XOÁ TRẮNG GỬI `""`, và đó là một giá trị có nghĩa: ngày khởi công về NULL, còn THỜI HẠN
 * GIẢI NGÂN về mặc định 31/12 vì cột ấy NOT NULL — máy chủ sở hữu quy tắc đó, không phải màn hình.
 */
export function thanSuaDuAn(dau: GiaTriFormDuAn, moi: GiaTriFormDuAn): ThanDung<SuaDuAnVao> {
  const than: {
    category_id?: string;
    name?: string;
    description?: string;
    planned_amount?: number;
    approved_amount?: number;
    start_date?: string;
    completion_date?: string;
    disbursement_deadline?: string;
  } = {};

  if (moi.hangMucID !== dau.hangMucID) {
    if (moi.hangMucID === "") return { ok: false, cau: CAU_THIEU_HANG_MUC };
    than.category_id = moi.hangMucID;
  }
  if (moi.ten.trim() !== dau.ten.trim()) {
    if (moi.ten.trim() === "") return { ok: false, cau: CAU_THIEU_TEN_DU_AN };
    than.name = moi.ten.trim();
  }
  if (moi.moTa.trim() !== dau.moTa.trim()) than.description = moi.moTa.trim();

  if (moi.keHoachVon !== dau.keHoachVon) {
    const so = docSoTien(moi.keHoachVon);
    switch (so.loai) {
      case "trong":
      case "khongPhaiSo":
        return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
      case "vuotChinhXac":
        return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
      case "so":
        than.planned_amount = so.dong;
        break;
    }
  }

  if (moi.tongMucDuyet !== dau.tongMucDuyet) {
    const so = docSoTien(moi.tongMucDuyet);
    switch (so.loai) {
      case "trong":
        // Xoá trắng ô "tổng mức được duyệt" nghĩa là quay về §9: lấy bằng kế hoạch vốn năm. Máy chủ
        // đọc số 0 đúng như thế (`approved_amount_set` là cách nó nói ra sự khác nhau).
        than.approved_amount = 0;
        break;
      case "khongPhaiSo":
        return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
      case "vuotChinhXac":
        return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
      case "so":
        than.approved_amount = so.dong;
        break;
    }
  }

  if (moi.ngayKhoiCong !== dau.ngayKhoiCong) than.start_date = moi.ngayKhoiCong;
  if (moi.ngayHoanThanh !== dau.ngayHoanThanh) than.completion_date = moi.ngayHoanThanh;
  if (moi.thoiHanGiaiNgan !== dau.thoiHanGiaiNgan) {
    than.disbursement_deadline = moi.thoiHanGiaiNgan;
  }

  if (Object.keys(than).length === 0) return { ok: false, cau: CAU_KHONG_CO_GI_DOI };
  return { ok: true, than };
}

/* ── Lý do ─────────────────────────────────────────────────────────────────────────────────── */

export const CAU_THIEU_LY_DO =
  "Chưa có lý do. Lý do được lưu cùng bản ghi và cùng vết kiểm toán — đây là hồ sơ lưu trữ, không " +
  "phải một dòng dữ liệu xoá đi là xong.";

/** Lý do gõ vào có dùng được không. Máy chủ vẫn là nơi từ chối thật; đây chỉ để khỏi gửi đi thừa. */
export function lyDoDuDung(lyDo: string): boolean {
  return lyDo.trim() !== "";
}

/* ── Câu cho các cổng quyền ────────────────────────────────────────────────────────────────── */

/**
 * Câu hiện thay cho nhóm nút GHI khi tài khoản thiếu khoá.
 *
 * GỌI ĐÚNG TÊN KHOÁ, và gọi đúng khoá NÀO cho việc NÀO: "bạn không có quyền" trống trơn là câu
 * khiến cán bộ gọi lên huyện hỏi mình thiếu quyền gì, và ở màn này câu trả lời có hai khả năng
 * khác hẳn nhau.
 */
export const CAU_THIEU_QUYEN_GHI =
  "Tài khoản của bạn không có quyền nhập liệu ngân sách (budget.update), nên phần thêm và sửa " +
  "không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

export const CAU_THIEU_QUYEN_XAC_NHAN =
  "Tài khoản của bạn không có quyền xác nhận ngân sách (budget.confirm), nên các thao tác xác " +
  "nhận, khoá, mở khoá và gỡ không hiển thị. Đây là quyền của người chịu trách nhiệm, tách khỏi " +
  "quyền nhập liệu có chủ ý.";

/* ── Phần chưa dựng ────────────────────────────────────────────────────────────────────────── */

/**
 * ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA BẢN THIẾT KẾ MÀ LƯỢT NÀY KHÔNG DỰNG ĐƯỢC — HIỆN LÊN ĐẦU MÀN, không giấu trong
 * chú thích mã và không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của màn Nội dung
 * Mini App, màn Thông báo, màn Biên bản họp, màn Nhiệm vụ và màn Thu - Chi.
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */
export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG_GHI: readonly PhanChuaDung[] = [
  {
    ten: "Bảng chứng từ của một dự án (§8.2) — danh sách đầy đủ, mở màn là thấy",
    viSao:
      "ĐÂY LÀ MỤC QUAN TRỌNG NHẤT CỦA DANH SÁCH NÀY. Hợp đồng có SÁU tuyến ghi chứng từ và KHÔNG " +
      "có tuyến nào đọc danh sách chứng từ — máy chủ nói thẳng đó là chủ ý và nói vì sao " +
      "(`service-finance/internal/http/chung_tu_giai_ngan.go`, khối đầu tệp: một tuyến đọc mang " +
      "theo câu hỏi phân trang và câu hỏi 'xã có 4000 chứng từ một năm thì nhận được gì'). Hệ " +
      "quả thật thà: bảng dưới đây CHỈ giữ những chứng từ do chính phiên làm việc này vừa ghi " +
      "hoặc vừa đổi trạng thái, vì bốn tuyến ấy trả về nguyên hàng. Tải lại trang là bảng trống " +
      "— không phải vì xã không có chứng từ, mà vì không có đường nào hỏi.",
  },
  {
    ten: "Cột `NGUỒN VỐN` và ô chọn nguồn vốn ở biểu mẫu chứng từ (§8.2, §6)",
    viSao:
      "Không có tuyến nào đọc danh mục nguồn vốn (`nguon_von`) trong hợp đồng, nên không có ô " +
      "chọn nào dựng được. Một ô cho cán bộ dán một chuỗi ULID không phải là một ô chọn. Chứng " +
      "từ ghi mà chưa gắn nguồn là trạng thái §13 quy tắc 6 định nghĩa và §6 báo cáo ('đã chi " +
      "nhưng chưa ghi rút từ nguồn nào'), nên biểu mẫu bỏ trống trường ấy là một đường hợp lệ, " +
      "không phải một khiếm khuyết bị giấu. Tuyến `PATCH` đã sẵn sàng nhận nó ngày danh mục có.",
  },
  {
    ten: "Ô `☑ Tự sinh mã` của modal Thêm dự án (§9)",
    viSao:
      "Máy chủ CHƯA tự sinh mã và nói thẳng vì sao (`domain.ErrThieuMaDuAn`): đặc tả đưa ra hai " +
      "khuôn mã mâu thuẫn nhau (`DA01` ở §9 và `DA-2026-be-tong-hoa-duong-ngo-xo-2` ở §7.2) và " +
      "không nói dãy chạy trong phạm vi nào. Mã dự án là MÃ ĐÃ CẤP, thứ luật 7 cấm đánh lại, nên " +
      "một dãy tự chế ở màn hình là thứ không rút lại được. Câu để HỎI khách, không phải để đoán.",
  },
  {
    ten: "Hai ô chọn `Đơn vị thực hiện` và `Cán bộ phụ trách` của modal Thêm dự án (§9)",
    viSao:
      "Hợp đồng nhận `org_unit_id` và `assignee_id`, nhưng cả hai là id của bản ghi do " +
      "`service-identity` sở hữu; ghép chúng thành hai ô chọn là việc của tuyến khác dưới quyền " +
      "khác, và dán một chuỗi ULID vào ô nhập không phải một giao diện. Dự án tạo ra để trống hai " +
      "trường ấy, đúng như §9 vẽ (`— Chưa xác định —`, `— Chưa phân công —`).",
  },
  {
    ten: "Danh sách động `Nguồn vốn` trong modal Thêm dự án (§9) và phép sửa phân bổ ở §8",
    viSao:
      "Tuyến `POST` NHẬN `funding_allocations`, nên nửa này không thiếu ở máy chủ — thiếu là ô " +
      "chọn nguồn vốn (xem mục trên). Còn SỬA phân bổ thì hợp đồng cố ý không có: `PATCH` không " +
      "nhận trường ấy vì phải trả lời trước câu hỏi mở (b) của migration 0007 — một dự án có " +
      "được khai hai dòng cùng một nguồn hay không — và đó là quyết định của khách.",
  },
  {
    ten: "Modal `⬆ Nhập giải ngân` từ Excel (§10) và nút `☰ Hạng mục` (§5)",
    viSao:
      "Không tuyến nào trong hợp đồng nhận tệp Excel, và quy tắc all-or-nothing của §10 là quy " +
      "tắc của MÁY CHỦ (kiểm cả tệp, có lỗi thì không nhận dòng nào) — dựng một nửa ở trình " +
      "duyệt là hứa một điều không có gì bảo đảm. Quản lý hạng mục kế hoạch vốn có tuyến thật " +
      "nhưng đứng sau `admin.lookup` và thuộc màn Cấu hình §5, không thuộc lượt này.",
  },
];
