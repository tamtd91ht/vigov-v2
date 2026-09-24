/**
 * Câu chữ, cách định dạng và phép dựng cây của màn "Thu - Chi ngân sách"
 * (`docs/ui-ux/07-thu-chi-ngan-sach.md`). Hàm thuần: không gọi mạng, không dựng DOM, không đọc
 * đồng hồ.
 *
 * KHÔNG MỘT PHÉP TÍNH NGHIỆP VỤ NÀO Ở ĐÂY. Tổng của một dòng cha, ba chỉ số của năm, và con số
 * tóm tắt đều do máy chủ suy ra trên từng lần đọc (`domain.BangDayDu.GiaTri`, `ChiSoDatDuToan`,
 * `CanDoiThuChi`) và về sẵn trong phản hồi. Tính lại ở đây là dựng câu trả lời thứ hai cho cùng
 * một câu hỏi — và bản chạy trên trình duyệt sẽ lệch bản của máy chủ mà không bài test nào đỏ.
 */

import type {
  finance_bangRa,
  finance_chiSoRa,
  finance_cotRa,
  finance_cotVao,
  finance_dongRa,
  finance_ghiDotVao,
  finance_soTienRa,
} from "@/lib/api/schema.gen";
import type { LoaiBang, SuaBangVao } from "@/lib/api/thu-chi";

/** Hai chữ số thập phân, đúng đơn vị đặc tả in ra cho phần trăm: `108,11%`. */
const DINH_DANG_PHAN_VAN = new Intl.NumberFormat("vi-VN", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

/** Ô rỗng, §9 quy tắc 4: giá trị trống hiện `—`, **không bao giờ** hiện `0`. */
export const O_TRONG = "—";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐƠN VỊ TÍNH — MÁY CHỦ GIỮ ĐỒNG, MÀN HÌNH CHIA KHI VẼ VÀ NHÂN KHI NHẬN
 *
 * Quyết định của khách 25/09/2026: tiền trên dây là SỐ NGUYÊN ĐỒNG (`int64`), và bảng mang một
 * đơn vị hiển thị đóng — `dong` | `nghin-dong` | `trieu-dong`. Trước đó màn hình in số đồng thô
 * cạnh nhãn "Triệu đồng": lệch 10⁶ lần.
 *
 * KHÔNG MỘT PHÉP NHÂN HAY CHIA SỐ THỰC NÀO. `1,005 × 1e6` trong JS là `1004999.9999999999`: một
 * phép quy đổi bằng số thực làm mất một đồng ở đúng những con số trông tròn nhất. Cả hai chiều ở
 * đây là phép DỜI DẤU PHẨY trên chuỗi chữ số (và `BigInt` cho phép dựng lại số nguyên), nên một
 * giá trị đồng đi ra màn hình rồi quay lại là ĐÚNG giá trị ấy.
 *
 * SỐ CHỮ SỐ LẺ = log10(hệ số): triệu nhận tới 6, nghìn tới 3, đồng 0. Đủ để MỌI giá trị đồng viết
 * được trong đơn vị của bảng, và không hơn — chữ số thứ 7 sau dấu phẩy của "triệu" là phần lẻ của
 * một đồng, thứ không có. HIỂN THỊ cũng CHÍNH XÁC (bỏ số 0 cuối), không làm tròn về 2 chữ số: một
 * con số công bị làm tròn trên màn hình là một con số khác con số đang lưu, và ô sửa điền sẵn phải
 * đọc lại đúng giá trị ấy để "Lưu" mà không sửa gì không đổi con số nào.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Ba mã đơn vị máy chủ nhận (`domain.KiemTraDonViTinh`). */
export type MaDonVi = "dong" | "nghin-dong" | "trieu-dong";

/**
 * Ba đơn vị kèm nhãn — cho Ô CHỌN của biểu mẫu lập và sửa bảng.
 *
 * Nhãn chép đúng `domain.nhanDonViTinh`. Đây là hằng của NỀN TẢNG, không phải của một xã; bảng đã
 * có thì màn hình in `unit_label` máy chủ gửi, không in nhãn này.
 */
export const DON_VI_TINH: readonly { ma: MaDonVi; nhan: string }[] = [
  { ma: "dong", nhan: "Đồng" },
  { ma: "nghin-dong", nhan: "Nghìn đồng" },
  { ma: "trieu-dong", nhan: "Triệu đồng" },
];

/** Đơn vị điền sẵn khi lập bảng: biểu của Phòng Tài chính in triệu đồng (§2). */
export const DON_VI_KHOI_DIEM: MaDonVi = "trieu-dong";

const SO_CHU_SO_LE: Readonly<Record<MaDonVi, number>> = {
  dong: 0,
  "nghin-dong": 3,
  "trieu-dong": 6,
};

/** Chuỗi có phải một trong ba mã không. Không đoán từ nhãn ("Triệu đồng" KHÔNG phải mã). */
export function laMaDonVi(tho: string): tho is MaDonVi {
  return tho === "dong" || tho === "nghin-dong" || tho === "trieu-dong";
}

/** Đơn vị màn hình dùng để vẽ và nhận số của MỘT bảng. */
export type DonViHien = {
  readonly ma: MaDonVi;
  readonly nhan: string;
  /** Có câu này thì màn hình KHÔNG quy đổi, và phải hiện câu ấy nổi bật. */
  readonly canhBao: string | null;
  /** Chữ đơn vị cũ đang lưu, khi máy chủ không ánh xạ được nó. */
  readonly nhanCu: string | null;
};

/**
 * Đơn vị của một bảng.
 *
 * FAIL CLOSED VỀ ĐỒNG, KHÔNG VỀ MỘT HỆ SỐ ĐOÁN: bảng lập trước khi danh sách đóng lại có thể giữ
 * chữ tự do; máy chủ khi ấy gửi `unit` rỗng, `unit_label` là chữ cũ và `unit_warning`. Chia cho
 * một hệ số đoán từ chữ ấy là hiện mọi con số lệch một nghìn hay một triệu lần. In đồng thô kèm
 * nhãn "đồng" thì đúng — chỉ khó đọc — và câu cảnh báo nói việc phải làm.
 */
export function donViCuaBang(bang: finance_bangRa): DonViHien {
  const canhBao = (bang.unit_warning ?? "").trim();
  if (canhBao === "" && laMaDonVi(bang.unit)) {
    const nhan = bang.unit_label.trim() !== "" ? bang.unit_label : nhanMaDonVi(bang.unit);
    return { ma: bang.unit, nhan, canhBao: null, nhanCu: null };
  }
  return {
    ma: "dong",
    nhan: "đồng",
    canhBao:
      canhBao !== ""
        ? canhBao
        : `Mã đơn vị tính "${bang.unit}" không thuộc danh sách đồng / nghìn đồng / triệu đồng — ` +
          "số hiện theo đồng, không quy đổi.",
    nhanCu: bang.unit_label.trim() !== "" ? bang.unit_label : null,
  };
}

function nhanMaDonVi(ma: MaDonVi): string {
  return DON_VI_TINH.find((d) => d.ma === ma)?.nhan ?? ma;
}

/**
 * Số nguyên đồng → chữ theo đơn vị, kiểu `vi-VN`: `3463459200000` (triệu) ⇒ `3.463.459,2`.
 *
 * Dời dấu phẩy trên chuỗi chữ số; không chia. `String()` của một số nguyên an toàn không bao giờ
 * ra dạng mũ (chỉ từ 1e21 trở lên), nên chuỗi chữ số là chính xác.
 */
export function dongSangChuoi(dong: number, donVi: MaDonVi): string {
  const soLe = SO_CHU_SO_LE[donVi];
  const am = dong < 0;
  const chu = String(Math.abs(dong)).padStart(soLe + 1, "0");
  const nguyen = chu.slice(0, chu.length - soLe);
  const le = chu.slice(chu.length - soLe).replace(/0+$/, "");
  const nhom = nguyen.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${am ? "-" : ""}${nhom}${le === "" ? "" : `,${le}`}`;
}

/**
 * Một ô tiền. `null` là ô trống (`—`), KHÔNG PHẢI 0 (§9 quy tắc 4).
 *
 * Giá trị không phải số nguyên an toàn là hợp đồng hỏng (máy chủ gửi `int64` đồng), không phải
 * một trạng thái nghiệp vụ: nó nói ra bằng một câu KHÁC thay vì làm tròn hay in "NaN".
 */
export function nhanSoTien(gia: number | null, donVi: MaDonVi): string {
  if (gia === null) return O_TRONG;
  if (!Number.isSafeInteger(gia)) return "Không đọc được";
  return dongSangChuoi(gia, donVi);
}

/** Câu nói ra cách con số được quy đổi — hiện cạnh đơn vị tính, không nằm trong chú thích mã. */
export function cauQuyDoi(donVi: DonViHien): string {
  if (donVi.ma === "dong") {
    return "Số liệu lưu bằng đồng và hiện bằng đồng, không quy đổi.";
  }
  const heSo = donVi.ma === "nghin-dong" ? "1.000" : "1.000.000";
  return (
    `Số liệu lưu bằng đồng; màn hình chia cho ${heSo} để hiện theo ${donVi.nhan.toLowerCase()}. ` +
    `Số gõ vào ô cũng được hiểu theo ${donVi.nhan.toLowerCase()}.`
  );
}

/**
 * `basis_points` là **phần vạn**: `10811` ⇒ `108,11%`.
 *
 * `null` KHÔNG PHẢI `0%`. Máy chủ gửi `null` kèm `unavailable_reason` khi không có mẫu số, khi xã
 * chưa gắn vai trò cho cột, hoặc khi chưa ai đánh dấu dòng tổng — ba trạng thái khác hẳn nhau và
 * không trạng thái nào là "đạt 0%". Hiện `0%` ở đó là báo cáo xã ấy như xã tệ nhất tỉnh.
 */
export function nhanPhanVan(phanVan: number | null): string {
  if (phanVan === null) return O_TRONG;
  if (!Number.isFinite(phanVan)) return "Không đọc được";
  return `${DINH_DANG_PHAN_VAN.format(phanVan / 100)}%`;
}

/** Một chỉ số đọc ra thành chữ: hoặc phần trăm, hoặc NGUYÊN câu máy chủ nói vì sao không có. */
export function nhanChiSo(c: finance_chiSoRa): string {
  if (c.basis_points !== null) return nhanPhanVan(c.basis_points);
  // Câu của máy chủ, nguyên văn: nó nói ra việc xã còn phải làm ("chưa đánh dấu dòng tổng",
  // "chưa có bảng"), thứ một câu do web đoán không nói được.
  return c.unavailable_reason !== undefined && c.unavailable_reason !== ""
    ? c.unavailable_reason
    : O_TRONG;
}

/** Một số tiền của thẻ chỉ số: hoặc con số, hoặc NGUYÊN câu máy chủ nói vì sao không có. */
export function nhanSoTienChiSo(s: finance_soTienRa, donVi: MaDonVi): string {
  if (s.amount !== null) return nhanSoTien(s.amount, donVi);
  return s.unavailable_reason !== undefined && s.unavailable_reason !== ""
    ? s.unavailable_reason
    : O_TRONG;
}

/**
 * `Cách tính` — ba chế độ của §4.2, nhưng CHỈ HAI là lựa chọn.
 *
 * `children` do CÂY quyết định (có con thì cộng con), gửi lên là 400. Dòng LÁ chọn giữa `manual`
 * và `entries` (quyết định của khách 25/09/2026). Nhãn giữ nguyên văn của §4.2.
 */
export function nhanCachTinh(method: string): string {
  switch (method) {
    case "manual":
      return "Nhập trực tiếp";
    case "entries":
      return "Cộng theo đợt";
    case "children":
      return "Cộng khoản mục con";
    default:
      // Một mã lạ là hợp đồng đã đổi mà màn hình chưa biết: hiện NGUYÊN mã thay vì đoán một nhãn.
      return method;
  }
}

/** Hai lựa chọn của ô chọn `Cách tính` trên một dòng lá. */
export const CACH_TINH_CHON: readonly { ma: "manual" | "entries"; nhan: string }[] = [
  { ma: "manual", nhan: "Nhập trực tiếp" },
  { ma: "entries", nhan: "Cộng theo đợt" },
];

/**
 * Dòng này có ĐƯỢC CHỌN cách tính không: dòng lá đang ở một trong hai chế độ chọn được.
 *
 * Xét CẢ cây lẫn mã máy chủ gửi: một dòng có con trên cây đang vẽ, hay một dòng máy chủ nói là
 * `children`, đều không có ô chọn — gửi `method` cho nó là 409 hoặc 400.
 */
export function chonDuocCachTinh(method: string, coCon: boolean): method is "manual" | "entries" {
  return !coCon && (method === "manual" || method === "entries");
}

/**
 * Câu cảnh báo TRƯỚC khi đổi cách tính — nói đúng điều máy chủ sẽ làm với con số.
 *
 * `entries → manual`: máy chủ CHÉP tổng các đợt vào ô số trong cùng giao dịch. `manual →
 * entries`: số gõ tay được GIỮ nhưng không hiện nữa. Cả hai đổi con số đang hiện trên bảng và
 * trong hai chỉ số của năm, nên không đổi bằng một lần chọn im lặng.
 */
export function canhBaoDoiCachTinh(den: "manual" | "entries"): string {
  if (den === "manual") {
    return (
      "Chuyển về Nhập trực tiếp: hệ thống chép tổng các đợt hiện có vào các ô số của khoản mục, " +
      "rồi từ đó con số được gõ tay. Các đợt đã ghi vẫn được giữ nhưng không còn được cộng."
    );
  }
  return (
    "Chuyển sang Cộng theo đợt: con số của khoản mục sẽ là tổng các đợt ghi ở hộp ⇄. Số đang gõ " +
    "tay được giữ lại nhưng không hiện nữa, và ô số không sửa được cho tới khi chuyển về Nhập " +
    "trực tiếp."
  );
}

/** Ô số có sửa được không: chỉ dòng `manual`. Dòng có con cộng con; dòng `entries` cộng đợt. */
export function suaDuocOSo(method: string): boolean {
  return method === "manual";
}

/**
 * Một ô tiền vừa gõ đọc thành gì — BA kết quả, và gộp bất kỳ hai cái nào là mất dữ liệu.
 *
 *   `trong` → ô để trống, nghĩa là **XOÁ TRẮNG** ô ấy (`values[cot] = null`, §9 quy tắc 4). Đây là
 *             lý do nó không được gộp với `loi`: một lần gõ hỏng mà bị đọc thành "trống" sẽ lặng lẽ
 *             xoá một con số ngân sách đang có.
 *   `so`    → số NGUYÊN ĐỒNG, đã quy đổi từ đơn vị của bảng — đúng thứ đi trên dây.
 *   `loi`   → mọi thứ còn lại, kèm câu nói vì sao. Không đoán, không làm tròn, không bỏ qua.
 */
export type SoNhap = { loai: "trong" } | { loai: "so"; gia: number } | { loai: "loi"; viSao: string };

/** `3463459,2` — không nhóm hàng nghìn. */
const KHUON_KHONG_NHOM = /^(-?)(\d+)(?:,(\d+))?$/;
/** `3.463.459,2` — dấu chấm ngăn ĐÚNG từng nhóm ba chữ số. */
const KHUON_CO_NHOM = /^(-?)(\d{1,3}(?:\.\d{3})+)(?:,(\d+))?$/;

const CAU_SAI_KHUON =
  "viết số theo kiểu Việt Nam: dấu chấm ngăn hàng nghìn, dấu phẩy trước phần lẻ (ví dụ 3.463.459,2).";

/**
 * Chữ gõ theo đơn vị của bảng → số nguyên đồng.
 *
 * `1.5` BỊ TỪ CHỐI, không đoán: ở `vi-VN` dấu chấm là dấu nhóm hàng nghìn, nên `1.500` là một nghìn
 * năm trăm; `1.5` không khớp khuôn nhóm nào và đọc nó thành một phẩy năm là đoán theo thói quen
 * của một bàn phím khác — lệch một nghìn lần.
 *
 * Quy đổi bằng NỐI CHUỖI: phần nguyên + phần lẻ đệm đủ số chữ số của đơn vị, rồi `BigInt`. Không
 * số thực nào đi qua, nên `1,005` triệu là đúng `1005000` đồng.
 */
export function docSoNhap(tho: string, donVi: MaDonVi): SoNhap {
  const sach = tho.trim();
  if (sach === "") return { loai: "trong" };

  const khop = KHUON_KHONG_NHOM.exec(sach) ?? KHUON_CO_NHOM.exec(sach);
  if (khop === null) return { loai: "loi", viSao: `Hãy ${CAU_SAI_KHUON}` };

  const dau = khop[1] ?? "";
  const nguyen = (khop[2] ?? "").replaceAll(".", "");
  const le = khop[3] ?? "";
  const soLe = SO_CHU_SO_LE[donVi];

  if (le.length > soLe) {
    return {
      loai: "loi",
      viSao:
        soLe === 0
          ? "Đơn vị đồng không có phần lẻ sau dấu phẩy."
          : `Đơn vị ${nhanMaDonVi(donVi).toLowerCase()} nhận tối đa ${soLe} chữ số sau dấu phẩy ` +
            "(đủ để ghi tới từng đồng).",
    };
  }

  let dong = BigInt(nguyen + le.padEnd(soLe, "0"));
  if (dau === "-") dong = -dong;

  const tran = BigInt(Number.MAX_SAFE_INTEGER);
  if (dong > tran || dong < -tran) {
    return { loai: "loi", viSao: "Số quá lớn để ghi." };
  }
  // `+ 0` gộp `-0` về `0`: "-0" là một cách gõ số 0, không phải một giá trị khác.
  return { loai: "so", gia: Number(dong) + 0 };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HỘP "CÁC ĐỢT THU, CHI" (§5) VÀ BIỂU MẪU SỬA BẢNG (§6)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Nhãn a11y của nút `⇄`, nguyên văn §4.1. */
export function nhanNutDot(ten: string): string {
  return `Các đợt thu, chi của ${ten}`;
}

/** Tiêu đề hộp: tên khoản mục VIẾT HOA (§5). `vi-VN` để `đ` thành `Đ`. */
export function tieuDeHopDot(ten: string): string {
  return ten.toLocaleUpperCase("vi-VN");
}

/** Câu mô tả của §5, nguyên văn. */
export const MO_TA_HOP_DOT =
  "Ghi từng đợt thu, chi rồi hệ thống cộng lại. Con số của khoản mục này lấy từ tổng các đợt bên " +
  "dưới, không gõ thẳng nữa.";

export const DOT_TRONG = "Chưa ghi đợt nào.";

/**
 * Câu nói ra ĐIỀU KIỆN để đợt được cộng. Ghi đợt KHÔNG tự chuyển dòng sang `entries`
 * (`routes.go:1204`), nên một cán bộ ghi đợt vào dòng đang `manual` sẽ không thấy con số đổi — và
 * phải được biết vì sao trước khi nghĩ rằng hệ thống hỏng.
 */
export function cauDieuKienDot(method: string): string {
  const dang = `Khoản mục đang tính theo: ${nhanCachTinh(method)}.`;
  return method === "entries"
    ? `Các đợt chỉ được cộng vào khoản mục khi Cách tính là "Cộng theo đợt". ${dang}`
    : `Các đợt chỉ được cộng vào khoản mục khi Cách tính là "Cộng theo đợt". ${dang} ` +
        "Các đợt ghi lúc này được lưu nhưng chưa được cộng.";
}

/** Giới hạn độ dài máy chủ đặt cho ba trường chữ của một đợt. */
export const DO_DAI_TOI_DA_DOT = { content: 1000, counterparty: 300, document_no: 100 } as const;

/** Thứ biểu mẫu ghi đợt đọc được, còn là chữ thô. */
export type NhapDot = {
  ngay: string;
  noiDung: string;
  doiTac: string;
  soChungTu: string;
  /** mã cột → chữ gõ trong ô, theo đơn vị của bảng */
  gia: Readonly<Record<string, string>>;
};

export type KetQuaDung<T> = { ok: true; than: T } | { ok: false; thongBao: string };

/** Độ dài theo KÝ TỰ, không theo đơn vị UTF-16: chữ Việt tổ hợp không được tính gấp đôi. */
function doDai(s: string): number {
  return [...s].length;
}

/**
 * Dựng thân `POST …/entries` từ biểu mẫu.
 *
 * CHỈ CỘT SỐ: cột phần trăm không lưu giá trị (§9 quy tắc 3) — lọc lại ở đây dù phía gọi đã lọc,
 * vì một cột `%` lọt vào thân là 400 ở mọi lần ghi. Cột để trống đi lên thành `null` (ô trống),
 * không phải 0. Phải có ÍT NHẤT một số tiền: một đợt không có số nào là một dòng vô nghĩa trong
 * sổ, và máy chủ từ chối nó.
 */
export function dungThanDot(
  nhap: NhapDot,
  cot: readonly finance_cotRa[],
  donVi: MaDonVi,
): KetQuaDung<finance_ghiDotVao> {
  const ngay = nhap.ngay.trim();
  if (!/^\d{4}-\d{2}-\d{2}$/.test(ngay)) {
    return { ok: false, thongBao: "Chọn ngày của đợt." };
  }
  const noiDung = nhap.noiDung.trim();
  if (noiDung === "") return { ok: false, thongBao: "Nhập nội dung của đợt." };
  if (doDai(noiDung) > DO_DAI_TOI_DA_DOT.content) {
    return { ok: false, thongBao: `Nội dung dài quá ${DO_DAI_TOI_DA_DOT.content} ký tự.` };
  }
  const doiTac = nhap.doiTac.trim();
  if (doDai(doiTac) > DO_DAI_TOI_DA_DOT.counterparty) {
    return {
      ok: false,
      thongBao: `Đơn vị, cá nhân dài quá ${DO_DAI_TOI_DA_DOT.counterparty} ký tự.`,
    };
  }
  const soChungTu = nhap.soChungTu.trim();
  if (doDai(soChungTu) > DO_DAI_TOI_DA_DOT.document_no) {
    return {
      ok: false,
      thongBao: `Số chứng từ dài quá ${DO_DAI_TOI_DA_DOT.document_no} ký tự.`,
    };
  }

  const values: Record<string, number | null> = {};
  let coSo = false;
  for (const c of cot) {
    if (c.type !== "so") continue;
    const doc = docSoNhap(nhap.gia[c.id] ?? "", donVi);
    if (doc.loai === "loi") return { ok: false, thongBao: `Ô "${c.name}": ${doc.viSao}` };
    if (doc.loai === "so") {
      values[c.id] = doc.gia;
      coSo = true;
    } else {
      values[c.id] = null;
    }
  }
  if (!coSo) return { ok: false, thongBao: "Nhập ít nhất một số tiền cho đợt." };

  const than: finance_ghiDotVao = { date: ngay, content: noiDung, values };
  if (doiTac !== "") than.counterparty = doiTac;
  if (soChungTu !== "") than.document_no = soChungTu;
  return { ok: true, than };
}

/**
 * Khoá chống trùng cho lần gửi KẾ TIẾP.
 *
 * Thành công → khoá MỚI (đợt sau là một đợt khác). Thất bại → GIỮ khoá: một lần gửi lại sau lỗi
 * mạng mà mang khoá mới là một đợt thứ hai nếu lần đầu thực ra đã tới máy chủ — đúng cái cộng đôi
 * `Idempotency-Key` sinh ra để chặn.
 */
export function khoaSauLanGhi(khoaHienTai: string, thanhCong: boolean, sinh: () => string): string {
  return thanhCong ? sinh() : khoaHienTai;
}

/** Thứ biểu mẫu sửa bảng đọc được. */
export type NhapSuaBang = { tieuDe: string; luyKe: string; donVi: string };

/**
 * Dựng thân `PATCH /budget-sheets/{id}` — CHỈ những trường thật sự đổi.
 *
 * Gửi cả ba mỗi lần thì một lần sửa tiêu đề cũng "đặt lại" mốc luỹ kế và đơn vị bằng đúng giá trị
 * cũ — vô hại với máy chủ, nhưng một vết ghi không nói được người ta đã đổi CÁI GÌ. Luỹ kế để
 * trống khi bảng đang có mốc là `""` — BỎ mốc, hợp đồng nói thế.
 */
export function dungThanSuaBang(bang: finance_bangRa, nhap: NhapSuaBang): KetQuaDung<SuaBangVao> {
  const tieuDe = nhap.tieuDe.trim();
  if (tieuDe === "") return { ok: false, thongBao: "Tiêu đề bảng không được để trống." };
  if (!laMaDonVi(nhap.donVi)) return { ok: false, thongBao: "Chọn đơn vị tính của bảng." };
  const luyKe = nhap.luyKe.trim();
  if (luyKe !== "" && !/^\d{4}-\d{2}-\d{2}$/.test(luyKe)) {
    return { ok: false, thongBao: "Ngày luỹ kế không đúng khuôn năm-tháng-ngày." };
  }

  const than: SuaBangVao = {};
  if (tieuDe !== bang.title) than.title = tieuDe;
  if (luyKe !== (bang.cumulative_to ?? "")) than.cumulative_to = luyKe;
  if (nhap.donVi !== bang.unit) than.unit = nhap.donVi;

  if (Object.keys(than).length === 0) {
    return { ok: false, thongBao: "Chưa có gì thay đổi." };
  }
  return { ok: true, than };
}

/** Câu gợi ý cạnh ô đơn vị tính — nói ra rằng đổi đơn vị không đổi con số nào. */
export const GOI_Y_DOI_DON_VI =
  "Đổi đơn vị tính chỉ đổi cách hiển thị: số liệu vẫn lưu bằng đồng, không con số nào bị quy đổi " +
  "hay làm tròn.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * THẺ CHỈ SỐ — `Chênh lệch thu – chi luỹ kế`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Tên con số chênh lệch, NGUYÊN VĂN quyết định của khách 25/09/2026.
 *
 * KHÔNG gọi nó là cân đối, bội chi hay thâm hụt: đó là những thuật ngữ ngân sách có định nghĩa
 * pháp lý, và con số này chỉ là một phép trừ giữa hai cột đã đánh dấu. Gọi nó bằng tên của một
 * chỉ tiêu pháp định là báo cáo lên trên một chỉ tiêu xã chưa từng tính.
 */
export const NHAN_CHENH_LECH = "Chênh lệch thu – chi luỹ kế";

export const GHI_CHU_CHENH_LECH =
  'Tính bằng cột "Thu xã hưởng" trên dòng được đánh sao của bảng thu trừ cột "Chi ngân sách" ' +
  "trên dòng được đánh sao của bảng chi. Tên gọi và cách tính này đang chờ khách hàng xác nhận.";

/**
 * Ngày `YYYY-MM-DD` của hợp đồng → `25/8/2026`.
 *
 * CẮT CHUỖI, KHÔNG DỰNG `Date`: `luy_ke_den` là một NGÀY, không mang giờ và không mang múi giờ;
 * cho nó qua `new Date("2026-08-25")` là gán cho nó nửa đêm UTC, và ở mọi máy đặt múi giờ phía tây
 * London nó hiện thành ngày 24 — một mốc luỹ kế lùi một ngày.
 *
 * ⚠ KHÔNG DÙNG LẠI `nhanNgay` CỦA `features/giai-ngan/nhan-du-an.ts`, và đó là một lựa chọn chứ
 * không phải một lần bỏ sót: hàm bên ấy đệm số 0 (`25/08/2026`) vì cột thời hạn giải ngân in như
 * thế, còn §2 của đặc tả này in `Luỹ kế đến 25/8/2026` — không đệm. Gọi sang hàm kia rồi cắt số 0
 * đi là hai phép biến đổi ngược nhau trên cùng một chuỗi.
 */
export function nhanNgayLuyKe(ngayISO: string): string {
  if (ngayISO === "") return "";
  const phan = ngayISO.split("-");
  const [nam, thang, ngay] = phan;
  if (phan.length !== 3 || nam === undefined || thang === undefined || ngay === undefined) {
    // Không đúng khuôn hợp đồng: hiện NGUYÊN chuỗi máy chủ gửi, đừng đoán. Một ngày đoán sai
    // trông y hệt một ngày đúng.
    return ngayISO;
  }
  return `${Number(ngay)}/${Number(thang)}/${nam}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CÂY KHOẢN MỤC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type NutCay = {
  readonly dong: finance_dongRa;
  readonly con: readonly NutCay[];
};

/**
 * Dựng cây từ danh sách phẳng máy chủ trả về, theo `parent_id`.
 *
 * KHÔNG DÒNG NÀO ĐƯỢC BIẾN MẤT, và đó là bất biến của hàm này. Một dòng trỏ tới cha không có
 * trong tập (bị lọc, bị xoá mềm giữa chừng, hay dữ liệu lệch) sẽ được treo ở **gốc** chứ không bị
 * bỏ qua: một khoản mục ngân sách rơi khỏi màn hình là một con số biến mất khỏi một báo cáo, và
 * không có gì trên màn hình nói ra rằng nó đã biến mất.
 *
 * `daQua` chặn vòng lặp vô hạn. Máy chủ đã từ chối vòng lặp (`ErrChaTaoVongLap`), nhưng một vòng
 * lặp lọt qua ở đây không phải một bảng vẽ sai — nó là một tab trình duyệt treo cứng.
 */
export function dungCay(dong: readonly finance_dongRa[]): readonly NutCay[] {
  const coTrongTap = new Set(dong.map((d) => d.id));
  const conCua = new Map<string, finance_dongRa[]>();
  const goc: finance_dongRa[] = [];

  for (const d of dong) {
    const cha = d.parent_id ?? "";
    if (cha === "" || cha === d.id || !coTrongTap.has(cha)) {
      goc.push(d);
      continue;
    }
    const nhom = conCua.get(cha);
    if (nhom === undefined) conCua.set(cha, [d]);
    else nhom.push(d);
  }

  const daQua = new Set<string>();

  function nhanh(chaID: string): NutCay[] {
    const ra: NutCay[] = [];
    for (const d of conCua.get(chaID) ?? []) {
      if (daQua.has(d.id)) continue;
      daQua.add(d.id);
      ra.push({ dong: d, con: nhanh(d.id) });
    }
    return ra;
  }

  const cay: NutCay[] = [];
  for (const d of goc) {
    if (daQua.has(d.id)) continue;
    daQua.add(d.id);
    cay.push({ dong: d, con: nhanh(d.id) });
  }

  // Dòng nào chưa xuất hiện (nhánh mồ côi bị vòng lặp cắt rời) vẫn phải ra màn hình.
  for (const d of dong) {
    if (!daQua.has(d.id)) {
      daQua.add(d.id);
      cay.push({ dong: d, con: [] });
    }
  }

  return cay;
}

export type DongHien = {
  readonly dong: finance_dongRa;
  /**
   * Độ sâu ĐÃ VẼ, đếm từ cây dựng ngay trên, không lấy `level` của máy chủ.
   *
   * Hai con số ấy khớp nhau khi dữ liệu nhất quán; khi chúng lệch, cái đúng để thụt lề là cái
   * khớp với hình cây đang vẽ — thụt lề theo `level` trên một dòng mồ côi treo ở gốc sẽ vẽ nó
   * lùi vào dưới một dòng không phải cha nó.
   */
  readonly cap: number;
  readonly coCon: boolean;
  readonly moRong: boolean;
};

/**
 * Trải cây thành danh sách dòng ĐANG HIỆN, theo tập id đang thu gọn.
 *
 * Con của một dòng thu gọn không có mặt trong kết quả — đó chính là ý nghĩa của "thu gọn" — nên
 * bộ đếm `đang hiện {x}/{y} khoản mục` của §4.3 đếm đúng độ dài danh sách này trên tổng số dòng.
 */
export function phangCay(
  cay: readonly NutCay[],
  thuGon: ReadonlySet<string>,
  cap = 0,
): readonly DongHien[] {
  const ra: DongHien[] = [];
  for (const nut of cay) {
    const coCon = nut.con.length > 0;
    const moRong = coCon && !thuGon.has(nut.dong.id);
    ra.push({ dong: nut.dong, cap, coCon, moRong });
    if (moRong) ra.push(...phangCay(nut.con, thuGon, cap + 1));
  }
  return ra;
}

/** Mọi dòng CÓ CON — tập thu gọn của nút `› Chỉ xem mục lớn` (§4.3). */
export function moiDongCoCon(dong: readonly finance_dongRa[]): ReadonlySet<string> {
  const coCha = new Set<string>();
  for (const d of dong) {
    const cha = d.parent_id ?? "";
    if (cha !== "") coCha.add(cha);
  }
  return coCha;
}

/** Bộ đếm của §4.3, nguyên văn. */
export function nhanBoDem(dangHien: number, tong: number): string {
  return `đang hiện ${dangHien}/${tong} khoản mục`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHÃN A11Y — §4.1 và `15-phu-luc §8` yêu cầu GIỮ NGUYÊN từng chữ
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export function nhanDatDongTong(ten: string): string {
  return `Đặt "${ten}" làm con số tổng`;
}

export const NHAN_SUA_TEN = "Bấm để sửa tên khoản mục";

export function nhanThemCon(ten: string): string {
  return `Thêm khoản mục con dưới ${ten}`;
}

export function nhanGoKhoanMuc(ten: string): string {
  return `Gỡ khoản mục ${ten}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Đây là khuôn màn Danh bạ và thanh
 * bên vừa dùng: đưa phần KHÔNG dùng được ra màn hình kèm lý do.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "⬆ Nạp từ Excel (§6)",
    viSao:
      "Hợp đồng REST không có tuyến nhận tệp: không có `POST /api/v1/budget-sheets/import` và " +
      "không có tuyến multipart nào trong `service-finance`. Bảng lập bằng biểu mẫu bên dưới, " +
      "khoản mục nhập từng dòng.",
  },
  {
    ten: "Cột phần trăm trên từng dòng (§3, §9 quy tắc 3)",
    viSao:
      "Cột `phan_tram` mang một chuỗi `formula` mà máy chủ CỐ Ý không diễn giải, và chuỗi ấy trỏ " +
      "tới cột bằng `col_4`, `col_2` — không ánh xạ được sang mã cột hợp đồng trả về. Tự đoán ánh " +
      "xạ là in một tỷ lệ sai trông y hệt một tỷ lệ đúng, nên các ô ấy hiện dấu gạch. Tỷ lệ của " +
      "DÒNG TỔNG vẫn có thật: máy chủ tính và gửi trong `summary.indicator`.",
  },
];

/**
 * Câu cảnh báo trước khi gỡ cả bảng (§6: "cảnh báo không hoàn tác"), nói đúng hậu quả có thật.
 *
 * KHÔNG VIẾT "không hoàn tác được" TRỐNG KHÔNG: dữ liệu vẫn còn (xoá mềm, luật 7), thứ không lấy
 * lại được là **mã bảng** và trạng thái "bảng đang sống của năm ấy". Nói sai về hậu quả theo chiều
 * nào cũng là nói sai.
 */
export const CANH_BAO_GO_BANG =
  "Gỡ bảng lấy cả một năm số liệu ra khỏi màn hình này và khỏi ba chỉ số của năm, kể cả những " +
  "con số đã báo cáo lên trên. Dữ liệu vẫn được giữ kèm người gỡ và lý do; mã bảng thì không cấp " +
  "lại, lần lập lại của năm này là một lần mới.";

export const CANH_BAO_GO_KHOAN_MUC =
  "Gỡ một khoản mục đổi ngay con số tổng của dòng cha và cả hai chỉ số của năm. Khoản mục còn " +
  "khoản mục con thì máy chủ từ chối — gỡ từ dòng con lên.";

/**
 * Câu hiện thay cho cả màn khi tài khoản thiếu `budget.read`.
 *
 * NÓ NẰM Ở ĐÂY CHỨ KHÔNG VIẾT THẲNG TRONG `page.tsx` vì nhánh "thiếu quyền" là nhánh người viết
 * mã KHÔNG BAO GIỜ nhìn thấy — tài khoản của họ luôn đủ quyền — nên nó phải kiểm được bằng một
 * bài test thay vì bằng mắt. Câu gọi ĐÚNG TÊN việc không làm được và đúng khoá quyền: "bạn không
 * có quyền" trống trơn là câu khiến cán bộ gọi lên tỉnh hỏi mình thiếu quyền gì.
 */
export const CAU_THIEU_QUYEN_XEM =
  "Tài khoản của bạn không có quyền xem thu - chi ngân sách (budget.read), nên phần này không " +
  "hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/** Trạng thái rỗng: xã chưa lập bảng cho năm và tab này. Bình thường, không phải lỗi. */
export function nhanChuaCoBang(nam: number, loai: LoaiBang): string {
  return (
    `Chưa đọc được bảng ${nhanLoaiBang(loai).toLowerCase()} năm ${nam}. Nếu đơn vị chưa lập bảng ` +
    "cho năm này, dùng biểu mẫu Lập bảng ngân sách bên dưới."
  );
}

export function nhanLoaiBang(loai: LoaiBang): string {
  return loai === "chi" ? "Chi ngân sách" : "Thu ngân sách";
}

/** Nhãn tab của §2, kèm năm: `Chi ngân sách 2026` · `Thu ngân sách 2026`. */
export function nhanTab(loai: LoaiBang, nam: number): string {
  return `${nhanLoaiBang(loai)} ${nam}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỘ CỘT KHỞI ĐIỂM CỦA BIỂU MẪU LẬP BẢNG
 *
 * CỘT LÀ DỮ LIỆU, KHÔNG PHẢI LƯỢC ĐỒ (§3, kết luận thiết kế) — nên đây là **giá trị điền sẵn của
 * một biểu mẫu sửa được**, không phải một bộ cột đóng cứng. Xã nào dùng biểu khác thì sửa, thêm,
 * bớt ngay trên biểu mẫu.
 *
 * TÊN CỘT CỦA TAB THU MANG NĂM (`Dự toán 2026 TP giao`) nên nó dựng TỪ năm đang chọn, không phải
 * một chuỗi có sẵn số 2026.
 *
 * `role` KHÔNG PHẢI TRANG TRÍ: hai chỉ số rời màn hình này đi vào báo cáo được đọc từ vai trò cột
 * chứ không từ tên cột (ADR 0035 §A). Một bảng lập thiếu vai trò là một bảng mà `Thu đạt dự toán`
 * và `Chi đạt dự toán` trống suốt năm.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export function boCotKhoiDiem(loai: LoaiBang, nam: number): finance_cotVao[] {
  if (loai === "chi") {
    return [
      { name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam" },
      { name: "Chi ngân sách", order: 2, type: "so", role: "chi-ngan-sach" },
      // Cột `phan_tram` BẮT BUỘC có `formula`, và cột `so` bắt buộc KHÔNG có (`KiemTraCot`).
      // Chuỗi này được LƯU chứ không được diễn giải ở bất kỳ đâu — xem `PHAN_CHUA_DUNG`.
      { name: "So sánh TH/DT (%)", order: 3, type: "phan_tram", formula: "col_2 / col_1 * 100" },
    ];
  }
  return [
    { name: `Dự toán ${nam} TP giao`, order: 1, type: "so", role: "du-toan-tp-giao" },
    { name: `Dự toán ${nam} Xã giao`, order: 2, type: "so", role: "du-toan-xa-giao" },
    { name: "Thu ngân sách NSNN", order: 3, type: "so", role: "thu-nsnn" },
    { name: "Thu ngân sách Thu xã hưởng", order: 4, type: "so", role: "thu-xa-huong" },
    { name: "Tỷ lệ % thu", order: 5, type: "phan_tram", formula: "col_3 / col_1 * 100" },
  ];
}

/** Sáu vai trò của `domain.VaiTroCot`, tách theo loại bảng đúng như `vaiTroCuaLoai` cho phép. */
export function vaiTroChoLoai(loai: LoaiBang): readonly { ma: string; nhan: string }[] {
  if (loai === "chi") {
    return [
      { ma: "du-toan-nam", nhan: "Dự toán năm" },
      { ma: "chi-ngan-sach", nhan: "Chi ngân sách" },
    ];
  }
  return [
    { ma: "du-toan-tp-giao", nhan: "Dự toán TP giao" },
    { ma: "du-toan-xa-giao", nhan: "Dự toán Xã giao" },
    { ma: "thu-nsnn", nhan: "Thu ngân sách NSNN" },
    { ma: "thu-xa-huong", nhan: "Thu xã hưởng" },
  ];
}

/** Chỉ cột `so` mới có ô nhập số; cột `phan_tram` không lưu giá trị nào (§9 quy tắc 3). */
export function cotSo(cot: readonly finance_cotRa[]): readonly finance_cotRa[] {
  return cot.filter((c) => c.type === "so");
}

/**
 * Tiêu đề thẻ báo cáo của §2: tiêu đề bảng, rồi dòng phụ `Đơn vị tính · Luỹ kế đến · N khoản mục`.
 *
 * TIÊU ĐỀ LẤY TỪ BẢNG, KHÔNG DỰNG TỪ TÊN XÃ TRONG BUNDLE: `BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ
 * THĂNG BÌNH NĂM 2026` là chữ của một xã cụ thể, và một chuỗi như thế nung vào bundle là đúng thứ
 * luật 1 bất biến 10 cấm. Nó do người lập bảng gõ và máy chủ lưu.
 */
export function dongPhuTieuDe(bang: finance_bangRa, soKhoanMuc: number): string {
  // Nhãn của đơn vị ĐANG DÙNG ĐỂ VẼ, không phải `unit` (một mã máy) và không phải chữ cũ chưa ánh
  // xạ được — với bảng ấy số đang hiện là đồng, và dòng phụ phải nói đúng điều đó.
  const phan: string[] = [`Đơn vị tính: ${donViCuaBang(bang).nhan}`];
  const luyKe = nhanNgayLuyKe(bang.cumulative_to ?? "");
  if (luyKe !== "") phan.push(`Luỹ kế đến ${luyKe}`);
  phan.push(`${soKhoanMuc} khoản mục`);
  return phan.join(" · ");
}
