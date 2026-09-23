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
  finance_soTienRa,
} from "@/lib/api/schema.gen";
import type { LoaiBang } from "@/lib/api/thu-chi";

/**
 * Định dạng số theo `vi-VN`, GHIM chứ không theo cài đặt của máy: dấu phân cách hàng nghìn khác
 * nhau giữa các miền địa phương làm `3.794.740` đọc thành ba triệu ở máy này và thành ba phẩy ở
 * máy khác.
 */
const DINH_DANG_SO = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 });

/** Hai chữ số thập phân, đúng đơn vị đặc tả in ra cho phần trăm: `108,11%`. */
const DINH_DANG_PHAN_VAN = new Intl.NumberFormat("vi-VN", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

/** Ô rỗng, §9 quy tắc 4: giá trị trống hiện `—`, **không bao giờ** hiện `0`. */
export const O_TRONG = "—";

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * IN THẲNG SỐ MÁY CHỦ TRẢ, KHÔNG QUY ĐỔI — VÀ ĐÂY LÀ MỘT CÂU KHÁCH CHƯA CHỐT.
 *
 * `bang.unit` là **chuỗi tự do** mô tả thứ màn hình in ("Triệu đồng"). Hợp đồng không kèm hệ số
 * quy đổi nào, và không có ánh xạ nào từ chữ ấy sang một số chia. Nên hàm này in đúng con số máy
 * chủ gửi và màn hình hiện `unit` thành nhãn bên cạnh.
 *
 * Chia cho 1e6 mỗi khi chuỗi tình cờ đọc là "Triệu đồng" sẽ là một quyết định THẦM về cách in một
 * con số công: một xã ghi `unit` khác đi sẽ thấy số lệch **một triệu lần**, và không bài kiểm nào
 * đỏ. Đã ghi vào báo cáo để khách chốt.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function nhanSoTien(gia: number | null): string {
  if (gia === null) return O_TRONG;
  // Ca "có giá trị nhưng không phải số hữu hạn" là hợp đồng hỏng, không phải một trạng thái
  // nghiệp vụ, nên nó nói ra bằng một câu KHÁC thay vì hiện chữ "NaN".
  if (!Number.isFinite(gia)) return "Không đọc được";
  return DINH_DANG_SO.format(gia);
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
export function nhanSoTienChiSo(s: finance_soTienRa): string {
  if (s.amount !== null) return nhanSoTien(s.amount);
  return s.unavailable_reason !== undefined && s.unavailable_reason !== ""
    ? s.unavailable_reason
    : O_TRONG;
}

/**
 * `Cách tính` — HAI giá trị, không ba, và **không ai chọn nó**.
 *
 * §4.2 vẽ một ô chọn ba giá trị. Cả hai điều ấy đã sai ở máy chủ đang chạy:
 *
 *   - `entries` ("Cộng theo đợt") KHÔNG TỒN TẠI: bảng `dot_thu_chi` không có, tuyến đợt không có,
 *     nên một dòng đặt chế độ ấy sẽ báo 0 ở mọi cột trong khi trông như một tính năng đang chạy
 *     (`domain.CachTinh`, migration 0006).
 *   - Client gửi `method` lên nhận **400** (`ErrCachTinhDoTuClient`). Máy suy nó từ CÂY:
 *     `CachTinhTheoCay(coCon)` — có con thì cộng con, không con thì nhập tay.
 *
 * Nên cột này ở màn hình là CHỮ CHỈ ĐỌC, không phải ô chọn. Nhãn giữ nguyên văn của §4.2.
 */
export function nhanCachTinh(method: string): string {
  switch (method) {
    case "manual":
      return "Nhập trực tiếp";
    case "children":
      return "Cộng khoản mục con";
    default:
      // Một mã lạ là hợp đồng đã đổi mà màn hình chưa biết: hiện NGUYÊN mã thay vì đoán một nhãn.
      return method;
  }
}

/** Ô số có sửa được không: dòng có con thì con số là TỔNG của các con, gõ vào đó là 409. */
export function suaDuocOSo(method: string): boolean {
  return method === "manual";
}

/**
 * Một ô số vừa gõ đọc thành gì — BA kết quả, và gộp bất kỳ hai cái nào là mất dữ liệu.
 *
 *   `trong` → ô để trống, nghĩa là **XOÁ TRẮNG** ô ấy (`values[cot] = null`, §9 quy tắc 4). Đây là
 *             lý do nó không được gộp với `loi`: một lần gõ hỏng mà bị đọc thành "trống" sẽ lặng lẽ
 *             xoá một con số ngân sách đang có.
 *   `so`    → số NGUYÊN. Giá trị đi trên dây là `int64` đồng (`domain.Dong`); một số lẻ gửi lên sẽ
 *             bị phân giải thành thứ khác ở phía Go, nên nó bị chặn ở đây kèm câu giải thích thay
 *             vì làm tròn thầm.
 *   `loi`   → mọi thứ còn lại. Không đoán, không làm tròn, không bỏ qua.
 */
export type SoNhap = { loai: "trong" } | { loai: "so"; gia: number } | { loai: "loi" };

export function docSoNhap(tho: string): SoNhap {
  const sach = tho.trim();
  if (sach === "") return { loai: "trong" };
  const n = Number(sach);
  if (!Number.isFinite(n) || !Number.isInteger(n)) return { loai: "loi" };
  return { loai: "so", gia: n };
}

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
    ten: "⇄ Các đợt thu, chi (§5)",
    viSao:
      "Không có bảng `dot_thu_chi`, không có tuyến đợt, và chế độ tính `Cộng theo đợt` không tồn " +
      "tại ở máy chủ. Dựng hộp thoại ấy là dựng một tính năng luôn báo số 0.",
  },
  {
    ten: "Ô chọn `Cách tính` trên từng dòng (§4.2)",
    viSao:
      "Máy chủ suy cách tính từ cây — có khoản mục con thì cộng con, không có thì nhập tay — và " +
      "trả 400 cho client nào tự đặt. Cột `Cách tính` ở bảng dưới là chữ chỉ đọc.",
  },
  {
    ten: "Sửa `Luỹ kế đến` (§6)",
    viSao:
      "Mốc luỹ kế chỉ đặt được LÚC LẬP BẢNG. Hợp đồng không có tuyến sửa bảng, nên đổi mốc ấy hôm " +
      "nay nghĩa là gỡ cả bảng kèm lý do rồi lập lại.",
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
 * Câu nói ra rằng con số in nguyên, không quy đổi theo `unit` — xem khối chú thích trên
 * `nhanSoTien`. Nó hiện cạnh đơn vị tính chứ không nằm trong chú thích mã.
 */
export const CAU_KHONG_QUY_DOI =
  "Số hiện đúng như máy chủ trả, không quy đổi. Đơn vị tính là nhãn của bảng do đơn vị tự ghi.";

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
  const phan: string[] = [`Đơn vị tính: ${bang.unit}`];
  const luyKe = nhanNgayLuyKe(bang.cumulative_to ?? "");
  if (luyKe !== "") phan.push(`Luỹ kế đến ${luyKe}`);
  phan.push(`${soKhoanMuc} khoản mục`);
  return phan.join(" · ");
}
