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
import type { finance_phanBoVao, finance_projectAllocationOut } from "@/lib/api/schema.gen";
import { compactDong } from "@/lib/compact-dong";

import { nhanTien } from "./nhan-du-an"; // vi-name-ok: existing formatter of nhan-du-an.ts (rule 12, invariant 3)

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
  /** `funding_source_id` of the `Rút từ nguồn vốn` select; `""` = none chosen. */
  readonly fundingSourceId: string;
};

export const FORM_CHUNG_TU_TRONG: GiaTriFormChungTu = {
  ngayChi: "",
  soTien: "",
  noiDung: "",
  doiTac: "",
  soChungTu: "",
  fundingSourceId: "",
};

/**
 * Asked when the project draws on named sources and the voucher names none — the client half of the
 * server's 409 `source_required` (decision 06/10/2026, `service-finance/internal/domain/voucher_source.go`).
 */
export const MISSING_FUNDING_SOURCE =
  "Chưa chọn nguồn vốn. Dự án này đã khai nguồn vốn, nên khoản chi phải ghi rõ rút từ nguồn nào.";

/**
 * One option of `Rút từ nguồn vốn`: "Ngân sách tỉnh — còn 1,2 tỷ đồng" (prototype
 * `DisbursementForm.tsx:167-171`). "Còn" is THIS PROJECT's line: allocated minus already drawn by its
 * vouchers, both the server's figures. Short form (`compactDong`) as on every funding tile; a negative
 * remainder stays negative — an overdrawn line must be visible where the next payment is chosen.
 */
export function allocationOptionLabel(line: finance_projectAllocationOut): string {
  return `${line.name} — còn ${compactDong(line.amount - line.disbursed_amount)}`;
}

/** The select's starting value on ADD: the only source when there is exactly one, else none. */
export function initialFundingSource(lines: readonly finance_projectAllocationOut[]): string {
  return lines.length === 1 ? lines[0]!.funding_source_id : "";
}

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
 * `funding_source_id` FOLLOWS THE PROJECT (decision 06/10/2026): `allocatedSourceIds` non-empty → the
 * source is REQUIRED (refused here before the server's 409 `source_required`); empty → NOTHING is sent,
 * because the server refuses any source on a project with no allocation line (409
 * `source_not_allocated`). A voucher with no source on such a project is §13 rule 6's legal state.
 *
 * Ô TRỐNG KHÔNG ĐƯỢC GỬI THÀNH `""`: `counterparty` và `voucher_no` có `omitempty`, nên vắng mặt là
 * cách nói "chứng từ này không có số kho bạc". Gửi chuỗi rỗng cũng ra cùng kết quả ở máy chủ, nhưng
 * nó nói rằng client có một giá trị và giá trị ấy rỗng — hai điều khác nhau.
 */
export function thanThemChungTu(
  duAnID: string,
  gt: GiaTriFormChungTu,
  allocatedSourceIds: readonly string[],
): ThanDung<ThemChungTuVao> {
  const drawsOnSources = allocatedSourceIds.length > 0;
  if (drawsOnSources && gt.fundingSourceId === "") return { ok: false, cau: MISSING_FUNDING_SOURCE };
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
      funding_source_id: drawsOnSources ? gt.fundingSourceId : undefined,
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
  allocatedSourceIds: readonly string[],
): ThanDung<SuaChungTuVao> {
  const than: {
    payment_date?: string;
    amount?: number;
    description?: string;
    counterparty?: string;
    voucher_no?: string;
    funding_source_id?: string;
  } = {};

  // SENT ONLY WHEN CHANGED. Absent means "leave the source alone" and the server checks nothing — so
  // a voucher entered before the project declared its sources can still have its description fixed
  // without being forced onto a source. Changed to `""` on a project with sources is the server's 409
  // `source_required`, refused here first. No allocation line → no select → the values never differ.
  if (moi.fundingSourceId !== dau.fundingSourceId) {
    if (allocatedSourceIds.length > 0 && moi.fundingSourceId === "") {
      return { ok: false, cau: MISSING_FUNDING_SOURCE };
    }
    than.funding_source_id = moi.fundingSourceId;
  }

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

/**
 * One line of the §9 `Nguồn vốn` list: a source and the amount drawn from it, as typed.
 * `key` only identifies the row on screen (React) and is never sent.
 */
export type AllocationRow = {
  readonly key: number;
  readonly sourceId: string;
  readonly amount: string;
};

/** Giá trị các ô của biểu mẫu dự án §9. CHUỖI hết, trừ ô đánh dấu `Tự sinh mã`. */
export type GiaTriFormDuAn = {
  /**
   * §9 `☑ Tự sinh mã`. Checked: no `code` is sent and the server issues the next code of the commune's
   * DA01, DA02… series (9f0a0187). Only the add form reads it; an issued code is never re-chosen.
   */
  readonly autoCode: boolean;
  readonly ma: string;
  readonly hangMucID: string;
  readonly ten: string;
  readonly moTa: string;
  readonly keHoachVon: string;
  readonly tongMucDuyet: string;
  readonly ngayKhoiCong: string;
  readonly ngayHoanThanh: string;
  readonly thoiHanGiaiNgan: string;
  readonly allocations: readonly AllocationRow[];
};

export const FORM_DU_AN_TRONG: GiaTriFormDuAn = {
  // Checked by default, as §9 and the prototype draw it (`BudgetItemForm.tsx:119-121`).
  autoCode: true,
  ma: "",
  hangMucID: "",
  ten: "",
  moTa: "",
  keHoachVon: "",
  tongMucDuyet: "",
  ngayKhoiCong: "",
  ngayHoanThanh: "",
  thoiHanGiaiNgan: "",
  allocations: [],
};

/* ── Phân bổ nguồn vốn §9 ─────────────────────────────────────────────────────────────────── */

export const ALLOCATION_NO_SOURCE =
  "Có dòng nguồn vốn chưa chọn nguồn. Chọn nguồn cho từng dòng, hoặc bỏ dòng không dùng.";
export const ALLOCATION_NO_AMOUNT =
  "Có dòng nguồn vốn chưa có số tiền. Nhập số tiền cho từng dòng, hoặc bỏ dòng không dùng.";

/**
 * The live comparison under the funding list: what the rows add up to against the year plan typed
 * above. A figure of what the clerk is TYPING, not a report figure — the server re-checks on save and
 * answers 409 `allocation_exceeds_plan` (8245698b); under-allocation is a normal project (its chip
 * shows the shortfall), so only `over` blocks.
 *
 * A row whose amount is empty or unreadable counts as 0 here; the submit check names it.
 * `unsafe`: the total left the range where a browser number holds every đồng — refused, never rounded.
 */
export type AllocationSummary =
  | { readonly state: "none" }
  | { readonly state: "noPlan"; readonly allocated: number }
  | { readonly state: "unsafe" }
  | { readonly state: "short"; readonly allocated: number; readonly planned: number; readonly gap: number }
  | { readonly state: "match"; readonly allocated: number; readonly planned: number }
  | { readonly state: "over"; readonly allocated: number; readonly planned: number; readonly gap: number };

export function summarizeAllocations(plannedTyped: string, rows: readonly AllocationRow[]): AllocationSummary {
  if (rows.length === 0) return { state: "none" };
  let allocated = 0;
  for (const row of rows) {
    const read = docSoTien(row.amount);
    if (read.loai === "vuotChinhXac") return { state: "unsafe" };
    if (read.loai === "so") allocated += read.dong;
  }
  if (!Number.isSafeInteger(allocated)) return { state: "unsafe" };

  const plan = docSoTien(plannedTyped);
  if (plan.loai !== "so") return { state: "noPlan", allocated };
  const planned = plan.dong;
  if (allocated > planned) return { state: "over", allocated, planned, gap: allocated - planned };
  if (allocated < planned) return { state: "short", allocated, planned, gap: planned - allocated };
  return { state: "match", allocated, planned };
}

/** Whether the summary forbids saving: over the plan, or a total the browser cannot hold exactly. */
export function allocationBlocksSave(summary: AllocationSummary): boolean {
  return summary.state === "over" || summary.state === "unsafe";
}

/** The sentence for a blocking summary — the same words the live line shows. */
function allocationBlockSentence(summary: AllocationSummary): string | null {
  if (summary.state === "unsafe") return CAU_SO_TIEN_VUOT_CHINH_XAC;
  if (summary.state === "over") {
    return (
      `Đã phân bổ ${nhanTien(summary.allocated)}, vượt số tiền bố trí ${nhanTien(summary.planned)} là ` +
      `${nhanTien(summary.gap)}, không lưu được. Hãy giảm số phân bổ hoặc tăng số tiền bố trí.`
    );
  }
  return null;
}

/**
 * The rows as the contract's lines, or the sentence naming what is missing. Field by field, never the
 * row itself (`key` must not reach the server). Amount 0 is a line the server accepts and is kept.
 */
export function allocationLines(rows: readonly AllocationRow[]): ThanDung<finance_phanBoVao[]> {
  const lines: finance_phanBoVao[] = [];
  for (const row of rows) {
    if (row.sourceId === "") return { ok: false, cau: ALLOCATION_NO_SOURCE };
    const read = docSoTien(row.amount);
    switch (read.loai) {
      case "trong":
        return { ok: false, cau: ALLOCATION_NO_AMOUNT };
      case "khongPhaiSo":
        return { ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC };
      case "vuotChinhXac":
        return { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC };
      case "so":
        lines.push({ funding_source_id: row.sourceId, amount: read.dong });
        break;
    }
  }
  return { ok: true, than: lines };
}

/**
 * Whether two lists say the same thing: the same sources with the same amounts, in any order. Amounts
 * compare by VALUE (`100.000.000` = `100000000`); an unreadable amount compares as typed, so editing
 * one is a change. Re-adding a removed source with its old amount is no change — and sends nothing.
 */
export function sameAllocations(a: readonly AllocationRow[], b: readonly AllocationRow[]): boolean {
  if (a.length !== b.length) return false;
  const canon = (rows: readonly AllocationRow[]) =>
    rows
      .map((r) => {
        const read = docSoTien(r.amount);
        return `${r.sourceId}\u0000${read.loai === "so" ? String(read.dong) : r.amount}`;
      })
      .sort();
  const left = canon(a);
  const right = canon(b);
  return left.every((v, i) => v === right[i]);
}

export const CAU_THIEU_MA_DU_AN =
  "Chưa có mã dự án. Nhập mã (chỉ chữ cái, chữ số và dấu gạch nối), hoặc đánh dấu “Tự sinh mã” " +
  "để hệ thống cấp mã tiếp theo trong dãy DA01, DA02…";
export const CAU_THIEU_HANG_MUC =
  "Chưa chọn hạng mục. Báo cáo tiến độ cộng dồn theo hạng mục nên mỗi dự án thuộc đúng một hạng mục.";
export const CAU_THIEU_TEN_DU_AN = "Chưa có tên dự án.";
export const CAU_KE_HOACH_VON_AM = "Kế hoạch vốn năm không được âm.";
export const CAU_KHOI_CONG_SAU_HOAN_THANH = "Ngày khởi công không được sau ngày hoàn thành.";

/**
 * Local mirror of `domain.ErrProjectStartAfterCompletion` (service-finance `du_an_ghi.go`, 0fa67247):
 * said at the form instead of after a round trip (GN-03). Same day is allowed, an empty box is no
 * date. The server still checks, and its sentence wins if the two ever disagree. Both values are the
 * `<input type="date">` `YYYY-MM-DD` form, so string order is date order.
 */
function startAfterCompletion(start: string, completion: string): boolean {
  return start !== "" && completion !== "" && start > completion;
}

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
 * KHÔNG CÓ `org_unit_id`, `assignee_id` — xem `PHAN_CHUA_DUNG_GHI`. `funding_allocations` is sent only
 * when the list has rows: absent and `[]` both create a project with no source on POST, and absent is
 * what an untouched list says.
 */
export function thanThemDuAn(nam: number, gt: GiaTriFormDuAn): ThanDung<ThemDuAnVao> {
  // `Tự sinh mã` checked → `code` ABSENT and the server issues the next DA-number. Unchecked with a
  // blank box is REFUSED here, never sent blank: the server reads a blank code as "issue one", which
  // is not what a clerk who unticked the box chose.
  const ma = gt.autoCode ? undefined : gt.ma.trim();
  if (ma === "") return { ok: false, cau: CAU_THIEU_MA_DU_AN };
  if (gt.hangMucID === "") return { ok: false, cau: CAU_THIEU_HANG_MUC };
  if (gt.ten.trim() === "") return { ok: false, cau: CAU_THIEU_TEN_DU_AN };
  if (startAfterCompletion(gt.ngayKhoiCong, gt.ngayHoanThanh)) {
    return { ok: false, cau: CAU_KHOI_CONG_SAU_HOAN_THANH };
  }

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

  let lines: finance_phanBoVao[] | undefined;
  if (gt.allocations.length > 0) {
    const read = allocationLines(gt.allocations);
    if (!read.ok) return read;
    const blocked = allocationBlockSentence(summarizeAllocations(gt.keHoachVon, gt.allocations));
    if (blocked !== null) return { ok: false, cau: blocked };
    lines = read.than;
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
      funding_allocations: lines,
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
    funding_allocations?: finance_phanBoVao[];
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

  // The pair AS IT WILL STAND after the PATCH: the form holds both dates, changed or not.
  if (startAfterCompletion(moi.ngayKhoiCong, moi.ngayHoanThanh)) {
    return { ok: false, cau: CAU_KHOI_CONG_SAU_HOAN_THANH };
  }
  if (moi.ngayKhoiCong !== dau.ngayKhoiCong) than.start_date = moi.ngayKhoiCong;
  if (moi.ngayHoanThanh !== dau.ngayHoanThanh) than.completion_date = moi.ngayHoanThanh;
  if (moi.thoiHanGiaiNgan !== dau.thoiHanGiaiNgan) {
    than.disbursement_deadline = moi.thoiHanGiaiNgan;
  }

  // FUNDING: ABSENT = unchanged, `[]` = remove all, a list = full replacement (8245698b). So it is
  // sent ONLY when the set changed — an unchanged list sent back would be a write nobody asked for.
  // The plan-vs-allocation check runs on the list AS IT WILL STAND, changed or not: lowering the plan
  // below what is already allocated is refused by the server too.
  if (!sameAllocations(dau.allocations, moi.allocations)) {
    const read = allocationLines(moi.allocations);
    if (!read.ok) return read;
    than.funding_allocations = read.than;
  }
  const blocked = allocationBlockSentence(summarizeAllocations(moi.keHoachVon, moi.allocations));
  if (blocked !== null) return { ok: false, cau: blocked };

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
 * GỌI ĐÚNG TÊN QUYỀN, và gọi đúng quyền NÀO cho việc NÀO: "bạn không có quyền" trống trơn là câu
 * khiến cán bộ gọi lên huyện hỏi mình thiếu quyền gì, và ở màn này câu trả lời có hai khả năng
 * khác hẳn nhau. Tên là tên màn Phân quyền hiện (`quyen.ten`, migration 0001 của identity), không
 * phải khoá máy `budget.*` (tester report GN-07).
 */
export const CAU_THIEU_QUYEN_GHI =
  "Tài khoản của bạn chưa được cấp quyền “Cập nhật giải ngân”, nên phần thêm và sửa " +
  "không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

export const CAU_THIEU_QUYEN_XAC_NHAN =
  "Tài khoản của bạn chưa được cấp quyền “Xác nhận, khoá khoản giải ngân”, nên các thao tác xác " +
  "nhận, khoá, mở khoá và gỡ không hiển thị. Đây là quyền của người chịu trách nhiệm, tách khỏi " +
  "quyền nhập liệu có chủ ý.";

/* ── Phần chưa dựng ────────────────────────────────────────────────────────────────────────── */

/**
 * ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA BẢN THIẾT KẾ MÀ LƯỢT NÀY KHÔNG DỰNG ĐƯỢC — không giấu trong chú thích mã và không
 * vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của các màn khác.
 *
 * Each entry is the description behind a disabled "?" placeholder AT ITS SPEC POSITION (ADR 0068
 * §14, `components/ui/pending-feature.tsx`), looked up with `pendingPart`.
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */
export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG_GHI: readonly PhanChuaDung[] = [
  // §8.2 voucher table + its `NGUỒN VỐN` column + the form's `Rút từ nguồn vốn` select: BUILT
  // (db94b35c, `GET /api/v1/investment-projects/{id}/disbursements`; `chung-tu-du-an.tsx`).
  // §9 `☑ Tự sinh mã`: BUILT (9f0a0187 — the server issues DA01, DA02… per commune; `ghi-du-an.tsx`).
  // §9 `Đơn vị thực hiện` / `Cán bộ phụ trách`. The contract takes `org_unit_id` and `assignee_id`,
  // but both are ids of records `service-identity` owns; turning them into two selects needs another
  // route under another permission, and a text box for a ULID is not an interface. Projects are
  // created with both empty, as §9 draws (`— Chưa xác định —`, `— Chưa phân công —`).
  {
    ten: "Đơn vị thực hiện và Cán bộ phụ trách",
    viSao:
      "Chưa chọn được đơn vị thực hiện và cán bộ phụ trách khi thêm dự án. Dự án được tạo với hai " +
      "mục này để trống.",
  },
  // §9 dynamic `Nguồn vốn` list + editing allocations: BUILT (8245698b, `FundingAllocationList` in
  // `ghi-du-an.tsx`).
  // §10 Excel import modal. No contract route takes an Excel file, and §10's all-or-nothing rule is a
  // SERVER rule (check the whole file; any error, accept no row) — half of it in the browser would
  // promise something nothing guarantees.
  {
    ten: "Nhập giải ngân từ Excel",
    viSao:
      "Chưa nhập được giải ngân từ tệp Excel. Hãy ghi từng khoản chi ở trang chi tiết của dự án.",
  },
  // §5 `☰ Hạng mục`: BUILT 06/10/2026 as the prototype's dialog (`category-manager-dialog.tsx`) —
  // list, add, rename, turn off / on, soft delete. A per-category deadline and yearly capital plan
  // are in neither the prototype dialog nor the contract.
  // §3 four KPI cards, §4 cumulative chart, §5 per-category table, §7.1 `Chỉ dự án chậm` and `Gộp theo
  // hạng mục`: BUILT 06/10/2026 on `GET /api/v1/investment-project-summary` and the list's
  // `delayed_only` (a3fdcac2; `disbursement-overview.tsx`, `project-groups.ts`). §6 per-funding-source
  // block + `Quản lý nguồn vốn`: BUILT (migration 0013, `funding-source-progress.tsx`).
  // §3 fourth card, its sub-line "N vướng mắc đang theo dõi · N nguy cơ không giải ngân hết". `vuong_mac`
  // (§8.1) does not exist in `finance` and the at-risk rule is undefined; the summary route leaves both
  // counts out on purpose (`disbursement_summary.go`), because "0 vướng mắc" would read as "none".
  {
    ten: "Vướng mắc và nguy cơ không giải ngân hết",
    viSao:
      "Hệ thống chưa ghi nhận vướng mắc của dự án và chưa có quy tắc xác định dự án có nguy cơ không " +
      "giải ngân hết, nên chưa có số liệu để hiện.",
  },
  // §7.2 funding chip: BUILT (`funding_status` on the list, `FundingChip` in `bang-du-an.tsx`).
  // Prototype list column "Đơn vị / phụ trách" + the detail figure "Đơn vị thực hiện" (ADR 0068 lần
  // 5). The contract returns `org_unit_id` / `assignee_id` as internal ids only; turning them into
  // names is another service's route under another permission, and an id is not a name.
  {
    ten: "Đơn vị và cán bộ phụ trách của dự án",
    viSao:
      "Hệ thống chỉ lưu mã nội bộ của đơn vị thực hiện và cán bộ phụ trách, chưa tra được thành " +
      "tên để hiện.",
  },
  // §7.2 column: no issue-tracking data exists anywhere yet.
  {
    ten: "Vướng mắc mới nhất",
    viSao: "Hệ thống chưa ghi nhận vướng mắc của dự án, nên chưa có nội dung để hiện.",
  },
  // §8 `GIẢI NGÂN THEO NGUỒN VỐN`: BUILT (`funding_allocations` on the detail, `ProjectFundingBlock`).
  // §8.1 tab: no issue-tracking routes.
  {
    ten: "Vướng mắc",
    viSao: "Hệ thống chưa có chức năng ghi nhận và theo dõi vướng mắc của dự án.",
  },
  // §8.3 tab `Biểu đồ`: BUILT 06/10/2026 on `GET /api/v1/investment-projects/{id}/disbursement-curve`
  // (`project-curve.tsx`).
  // §8.4 tab: no discussion storage for a project.
  {
    ten: "Trao đổi",
    viSao: "Hệ thống chưa có chức năng lưu trao đổi giữa các cán bộ về một dự án.",
  },
];

/**
 * One `PHAN_CHUA_DUNG_GHI` entry by its exact `ten`, for the "?" placeholders (ADR 0068 §14).
 *
 * LOOKED UP, NOT COPIED: the entries must stay literal inside the array (`tools/tien_do_san_pham.py`
 * counts `ten: "` lines inside its block), and a second copy of a sentence here would drift.
 * THROWS on a missing name rather than drawing a placeholder with no description — `ghi-giai-ngan
 * .test.tsx` renders every placeholder, so a renamed entry fails there instead of on a staff screen.
 */
export function pendingPart(ten: string): PhanChuaDung {
  const part = PHAN_CHUA_DUNG_GHI.find((p) => p.ten === ten);
  if (part === undefined) throw new Error(`PHAN_CHUA_DUNG_GHI has no entry "${ten}"`);
  return part;
}
