/**
 * Câu chữ, vòng đời và các phép QUYẾT ĐỊNH của màn "Quản lý nhiệm vụ"
 * (`docs/ui-ux/02-nhiem-vu.md`). Hàm thuần: không gọi mạng, không dựng DOM, không đọc đồng hồ —
 * thời điểm "bây giờ" luôn là một tham số, để ca "trễ 87 ngày" kiểm được mà không cần giả lập
 * đồng hồ hệ thống.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO TỆP NÀY ĐỨNG MỘT MÌNH, KHÔNG KÈM `lib/api/nhiem-vu.ts` VÀ KHÔNG KÈM MÀN HÌNH
 *
 * Tám tuyến Nhiệm vụ ĐÃ CÓ trong `kb/20-contracts/openapi.json`, nhưng `src/lib/api/schema.gen.ts`
 * chưa được sinh lại kể từ khi chúng vào hợp đồng: mười một kiểu mà tầng gọi cần —
 * `petitions_nhiemVuRa`, `petitions_taoNhiemVuVao`, `petitions_doiTrangThaiVao`,
 * `petitions_deNghiLuiHanVao`, `petitions_quyetDinhLuiHanVao`, `page_Result_petitions_nhiemVuRa`
 * và năm kiểu tuyến — KHÔNG TỒN TẠI trong tệp sinh hiện tại. `node scripts/gen-api-types.mjs
 * --check` thoát mã 1.
 *
 * Gõ tay các hình dạng ấy ở đây là dựng đúng cái hỏng mà `scripts/gen-api-types.mjs` được viết ra
 * để chặn — ba bản chép tay của một hình dạng, trôi dần, và bản sai là bản chạy thật (luật 9,
 * cấm #2; bất biến 6 của agent này). Nên mọi thứ trong tệp này CỐ Ý chỉ nhận **giá trị vô hướng**
 * (chuỗi, số, `Date`), không nhận và không mô tả lại một bản ghi nhiệm vụ nào. Khi tệp sinh có
 * các kiểu ấy, tầng gọi và màn hình lắp lên trên tệp này mà không phải sửa một dòng nào ở đây.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** Ô rỗng. Chưa có gì để tính thì **dấu gạch**, không bao giờ `0` và không bao giờ `0%`. */
export const O_TRONG = "—";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢY TRẠNG THÁI — §6
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Bảy mã trạng thái của §6. **DANH SÁCH ĐÓNG** — câu hỏi mở #21 đã chốt ngày 22/09/2026
 * (ADR 0035 §C): xã đổi được NHÃN và THỨ TỰ, không đổi được DANH SÁCH MÃ.
 *
 * Chính vì danh sách đóng mà gõ thẳng bảy mã ấy vào mã nguồn trình duyệt là hợp lệ. Với một danh
 * sách xã sửa được thì nó sẽ là một giá trị riêng của xã bị nung vào bundle (luật 1, bất biến 10)
 * — điều tệp này không được phép làm.
 *
 * Cùng bảy chuỗi mà lược đồ cưỡng chế bằng `CHECK`
 * (`service-petitions/migrations/0006_nhiem_vu.sql:341`) và miền nghiệp vụ khai bằng hằng
 * (`service-petitions/internal/domain/nhiem_vu.go:40-47`).
 */
export type TrangThaiNhiemVu =
  | "moi-giao"
  | "da-tiep-nhan"
  | "dang-thuc-hien"
  | "cho-duyet"
  | "hoan-thanh"
  | "tam-dung"
  | "chuyen-tiep";

/**
 * Năm trạng thái CHÍNH — đúng năm cột Kanban của §4.1, đúng thứ tự dải bước của §5.2.
 *
 * Hai trạng thái rẽ nhánh KHÔNG có cột riêng (§4.1), nên chúng không nằm trong mảng này.
 */
export const TRANG_THAI_CHINH: readonly TrangThaiNhiemVu[] = [
  "moi-giao",
  "da-tiep-nhan",
  "dang-thuc-hien",
  "cho-duyet",
  "hoan-thanh",
];

/** Hai trạng thái rẽ nhánh — hàng `Rẽ nhánh:` dưới dải bước (§5.2, phụ lục 15 §5.2). */
export const TRANG_THAI_RE_NHANH: readonly TrangThaiNhiemVu[] = ["tam-dung", "chuyen-tiep"];

/** Bảy mã, dùng cho ô chọn và cho phép kiểm `laTrangThaiNhiemVu`. */
export const MOI_TRANG_THAI: readonly TrangThaiNhiemVu[] = [
  ...TRANG_THAI_CHINH,
  ...TRANG_THAI_RE_NHANH,
];

/**
 * Nhãn của §6 — chữ được giao hàng.
 *
 * ⚠ ĐÂY LÀ CHỖ SẼ PHẢI ĐỔI KHI XÃ SỬA ĐƯỢC NHÃN. ADR 0035 §C cho phép xã đổi nhãn, nhưng hợp
 * đồng KHÔNG có tuyến nào phát ra danh mục `Trạng thái nhiệm vụ`: có `GET /api/v1/task-types`,
 * `GET /api/v1/task-priorities` và `GET /api/v1/task-blocs`, không có `task-statuses`. Nên hôm
 * nay bảy nhãn này là chữ của đặc tả, và sự vắng mặt ấy được nói ra ở `PHAN_CHUA_DUNG` chứ
 * không giấu trong chú thích.
 */
const NHAN_TRANG_THAI: Readonly<Record<TrangThaiNhiemVu, string>> = {
  "moi-giao": "Mới giao",
  "da-tiep-nhan": "Đã tiếp nhận",
  "dang-thuc-hien": "Đang thực hiện",
  "cho-duyet": "Chờ duyệt",
  "hoan-thanh": "Hoàn thành",
  "tam-dung": "Tạm dừng",
  "chuyen-tiep": "Chuyển tiếp",
};

/**
 * Nhãn cột Kanban — KHÁC nhãn §6 ở đúng một ô: `moi-giao` lên bảng là **"Chưa thực hiện"**.
 *
 * §6 ghi rõ sự lệch ấy (`Mới giao *(Kanban gọi "Chưa thực hiện")*`), và §4.1 lẫn §12 đều đếm
 * "Chưa thực hiện 7". Dùng một nhãn cho cả hai chỗ là làm sai một trong hai màn.
 */
const NHAN_COT_KANBAN: Readonly<Record<TrangThaiNhiemVu, string>> = {
  ...NHAN_TRANG_THAI,
  "moi-giao": "Chưa thực hiện",
};

/** Mã có phải một trong bảy hay không. Dùng để đọc `status` — hợp đồng khai nó là `string` trơn. */
export function laTrangThaiNhiemVu(ma: string): ma is TrangThaiNhiemVu {
  return (MOI_TRANG_THAI as readonly string[]).includes(ma);
}

/**
 * Nhãn để hiện, kể cả khi máy chủ gửi một mã màn hình chưa biết.
 *
 * MÃ LẠ HIỆN NGUYÊN VĂN, KHÔNG HIỆN DẤU GẠCH và không im lặng bỏ qua: một trạng thái mới ở máy
 * chủ mà màn hình vẽ thành `—` là một hồ sơ trông như chưa có trạng thái.
 */
export function nhanTrangThai(ma: string): string {
  return laTrangThaiNhiemVu(ma) ? NHAN_TRANG_THAI[ma] : ma;
}

/** Nhãn cột Kanban (§4.1). Cùng quy tắc "mã lạ hiện nguyên văn". */
export function nhanCotKanban(ma: string): string {
  return laTrangThaiNhiemVu(ma) ? NHAN_COT_KANBAN[ma] : ma;
}

/**
 * Vòng đời §6 — **BẢN THỨ HAI** của `chuyenDuocSangNhiemVu`
 * (`service-petitions/internal/domain/nhiem_vu.go:77-85`), và cái giá của nó được nói ra ở đây
 * chứ không để người sau tự phát hiện.
 *
 * VÌ SAO VẪN CHẤP NHẬN ĐƯỢC: hợp đồng không có tuyến nào phát ra vòng đời, mà §5.2 đòi dải bước
 * biết ô nào **bấm được**. Bản sao này chỉ quyết định MỘT NÚT CÓ HIỆN HAY KHÔNG; nó không quyết
 * định lần ghi nào thành công. `POST /api/v1/tasks/{ma}/status` kiểm lại toàn bộ bằng
 * `ChuyenTrangThaiDuoc` và trả `ErrChuyenTrangThaiNhiemVuSaiLuc` cho bước nó không có.
 *
 * Nên bản sao này trôi theo hai chiều, cả hai đều HỎNG AN TOÀN:
 *   - rộng hơn máy chủ ⇒ một nút hiện ra rồi nhận câu từ chối nguyên văn của máy chủ;
 *   - hẹp hơn máy chủ ⇒ một nút thiếu, cán bộ báo ngay vì họ đang cần bấm nó.
 * Không chiều nào cho ra một lần GHI SAI, và đó là điều kiện để chấp nhận một bản sao.
 */
const CHUYEN_DUOC: Readonly<Record<TrangThaiNhiemVu, readonly TrangThaiNhiemVu[]>> = {
  "moi-giao": ["da-tiep-nhan", "tam-dung", "chuyen-tiep"],
  "da-tiep-nhan": ["dang-thuc-hien", "tam-dung", "chuyen-tiep"],
  "dang-thuc-hien": ["cho-duyet", "tam-dung", "chuyen-tiep"],
  "cho-duyet": ["hoan-thanh", "tam-dung", "chuyen-tiep"],
  "tam-dung": ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet"],
  "hoan-thanh": [],
  "chuyen-tiep": [],
};

/**
 * Vòng đời CÓ bước này hay không — câu hỏi về HÌNH DẠNG, không phải về quyền.
 *
 * ⚠ LỎNG CÓ CHỦ Ý Ở `tam-dung`, đúng như miền nghiệp vụ: từ `tam-dung` thì bốn trạng thái chính
 * đều là hình dạng hợp lệ, nhưng chỉ ĐÚNG MỘT trong bốn là hợp lệ thật — trạng thái ngay trước
 * lúc tạm dừng. Máy chủ tìm nó trong nhật ký (`TrangThaiTruocTamDung`); màn hình **không tìm
 * được**, vì hợp đồng không có tuyến nhật ký nào. Xem `PHAN_CHUA_DUNG`.
 */
export function chuyenSangDuoc(hienTai: string, moi: string): boolean {
  if (!laTrangThaiNhiemVu(hienTai) || !laTrangThaiNhiemVu(moi)) return false;
  return CHUYEN_DUOC[hienTai].includes(moi);
}

/** Vòng đời có lối ra khỏi trạng thái này không (`hoan-thanh` và `chuyen-tiep` là hai ngõ cụt). */
export function ketThuc(ma: string): boolean {
  return laTrangThaiNhiemVu(ma) && CHUYEN_DUOC[ma].length === 0;
}

/**
 * Câu giải thích dưới dải bước (§5.2: *"Dưới stepper là câu giải thích trạng thái hiện tại"*).
 *
 * Câu của `moi-giao` lấy NGUYÊN VĂN ví dụ đặc tả đưa ra. Sáu câu còn lại viết theo cùng một
 * khuôn: nói ai đang giữ việc và điều gì đang được chờ — không nói lại tên trạng thái.
 */
export function cauGiaiThichTrangThai(ma: string): string {
  switch (ma) {
    case "moi-giao":
      return "Đã giao nhưng người nhận chưa bấm tiếp nhận.";
    case "da-tiep-nhan":
      return "Người nhận đã tiếp nhận nhưng chưa bắt tay vào làm.";
    case "dang-thuc-hien":
      return "Đang làm, chưa báo xong.";
    case "cho-duyet":
      return "Đã báo xong, đang chờ lãnh đạo duyệt hoàn thành.";
    case "hoan-thanh":
      return "Đã duyệt hoàn thành. Nhiệm vụ khép lại ở đây.";
    case "tam-dung":
      return "Đang tạm dừng. Tiếp tục thì việc quay về đúng trạng thái trước lúc dừng.";
    case "chuyen-tiep":
      return "Đã chuyển cho bộ phận khác. Nhiệm vụ này khép lại tại đây.";
    default:
      return "";
  }
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỐN NGUỒN GIAO VIỆC — §3, §4.2
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Bốn mã nguồn giao, danh sách ĐÓNG: mỗi mã trỏ về một quyển sổ gốc KHÁC nhau, nên mã thứ năm là
 * một tích hợp mới chứ không phải một dòng danh mục (`domain.NguonGiao`). Máy chủ TỪ CHỐI — không
 * bỏ qua — một `source` ngoài bốn mã này (`nhiem_vu.go:352`).
 */
export type NguonGiao = "truc-tiep" | "ket-luan-hop" | "van-ban-den" | "phan-anh";

const NHAN_NGUON_GIAO: Readonly<Record<NguonGiao, string>> = {
  "truc-tiep": "Giao trực tiếp",
  "ket-luan-hop": "Từ kết luận họp",
  "van-ban-den": "Từ văn bản đến",
  "phan-anh": "Từ phản ánh",
};

export const MOI_NGUON_GIAO: readonly NguonGiao[] = [
  "truc-tiep",
  "ket-luan-hop",
  "van-ban-den",
  "phan-anh",
];

export function laNguonGiao(ma: string): ma is NguonGiao {
  return (MOI_NGUON_GIAO as readonly string[]).includes(ma);
}

/** Dòng phụ nhỏ dưới tên việc ở bảng Danh sách (§4.2). Mã lạ hiện nguyên văn. */
export function nhanNguonGiao(ma: string): string {
  return laNguonGiao(ma) ? NHAN_NGUON_GIAO[ma] : ma;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HẠN XỬ LÝ — và vì sao "quá hạn" KHÔNG BAO GIỜ là một cột
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Múi giờ GHIM cho mọi phép in ngày của màn này.
 *
 * ĐÂY LÀ HẰNG CỦA NỀN TẢNG, KHÔNG PHẢI GIÁ TRỊ CỦA MỘT XÃ (luật 8, bất biến 5): Việt Nam dùng
 * một múi giờ duy nhất trên toàn quốc, nên nó không khác nhau giữa 300 xã. Không ghim thì
 * `due_at` — một mốc `date-time` — in ra ngày khác nhau trên máy đặt múi giờ khác nhau, và một
 * hạn xử lý lệch một ngày là một cam kết với dân bị đọc sai.
 */
const MUI_GIO = "Asia/Ho_Chi_Minh";

/** `20/6/2026` — không đệm số 0, đúng như §5.3, §5.4 và §5.6 in ra. */
const DINH_DANG_NGAY = new Intl.DateTimeFormat("vi-VN", {
  timeZone: MUI_GIO,
  day: "numeric",
  month: "numeric",
  year: "numeric",
});

/**
 * Mốc `date-time` của hợp đồng → `20/6/2026`. `null` → `—`.
 *
 * CHUỖI KHÔNG ĐỌC ĐƯỢC HIỆN NGUYÊN VĂN, không hiện `Invalid Date` và không hiện dấu gạch: một
 * ngày đoán sai trông y hệt một ngày đúng, còn dấu gạch nói dối rằng máy chủ không gửi gì.
 */
export function nhanNgay(mocISO: string | null): string {
  if (mocISO === null || mocISO === "") return O_TRONG;
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return mocISO;
  return DINH_DANG_NGAY.format(new Date(t));
}

/**
 * Tình trạng hạn — **SUY RA**, không đọc từ một cột.
 *
 * Luật 10, bất biến 3: quá hạn là phép so `han_xu_ly` với BÂY GIỜ, không bao giờ là một cột hay
 * một cờ được ghi. Một cột `is_overdue` do việc chạy đêm đặt sẽ sai ngay khi việc ấy chạy trễ,
 * khi đồng hồ lệch, hoặc khi thêm một ngày nghỉ lễ — và bản sai là bản đi lên báo cáo.
 *
 * ⚠ `soNgay` LÀ THỜI GIAN TRÔI QUA THEO LỊCH, KHÔNG PHẢI GIỜ LÀM VIỆC. §4.1 và §4.2 in đúng như
 * thế (`Trễ 87 ngày`, `10/9/2026 (trễ 6 ngày)`). Nó KHÔNG được dùng để tính ra một cái hạn: hạn
 * đếm bằng GIỜ LÀM VIỆC và chỉ `identity.AdvanceWorkingHours` tính được, vì chỉ dịch vụ ấy giữ
 * lịch làm việc, `ngay_nghi_le` và `ngay_lam_bu` (luật 10, bất biến 4 và cấm #2, ADR 0007).
 */
export type TinhTrangHan =
  /** Nhiệm vụ không có hạn. Hai cột hạn có hoặc không CÙNG NHAU, nên đây là một trạng thái thật. */
  | { readonly loai: "khong-han" }
  | { readonly loai: "tre"; readonly soNgay: number }
  | { readonly loai: "con-han"; readonly soNgay: number };

const MOT_NGAY_MS = 86_400_000;

/**
 * So hạn với bây giờ. `bayGio` là THAM SỐ chứ không phải `new Date()` đọc trong hàm — một hàm
 * đọc đồng hồ hệ thống là một hàm chỉ kiểm được bằng cách giả lập đồng hồ.
 *
 * KHÔNG CÓ NHÁNH "SẮP ĐẾN HẠN" Ở ĐÂY, có chủ ý. Ngưỡng ấy là `sla.gio_sap_den_han` — **số giờ
 * của TỪNG XÃ** (`service-identity/migrations/0008_sla.sql:178`) — nên nung số 72 vào đây là
 * đúng thứ luật 1 bất biến 10 cấm, và là nguồn thứ hai cho một con số mà mọi màn "sắp đến hạn"
 * phải đọc từ một cột duy nhất. Máy chủ cũng TỪ CHỐI `?soon=` vì đúng lý do ấy
 * (`nhiem_vu.go:386`). Xem `PHAN_CHUA_DUNG`.
 */
export function tinhTrangHan(hanISO: string | null, bayGio: Date): TinhTrangHan {
  if (hanISO === null || hanISO === "") return { loai: "khong-han" };
  const han = Date.parse(hanISO);
  if (Number.isNaN(han)) return { loai: "khong-han" };

  const lech = bayGio.getTime() - han;
  if (lech > 0) return { loai: "tre", soNgay: Math.floor(lech / MOT_NGAY_MS) };
  return { loai: "con-han", soNgay: Math.floor(-lech / MOT_NGAY_MS) };
}

/**
 * Dòng hạn trên thẻ Kanban (§4.1): `Trễ 87 ngày` · `Hạn 20/12/2026` · `Hạn —`.
 *
 * "TRỄ 0 NGÀY" KHÔNG ĐƯỢC IN RA. Một việc vừa quá hạn nửa tiếng vẫn là việc đã trễ, và dòng chữ
 * `Trễ 0 ngày` đọc ra là "chưa trễ" — đúng điều ngược lại.
 */
export function nhanHanThe(hanISO: string | null, bayGio: Date): string {
  const tt = tinhTrangHan(hanISO, bayGio);
  if (tt.loai === "khong-han") return `Hạn ${O_TRONG}`;
  if (tt.loai === "tre") {
    return tt.soNgay < 1 ? "Trễ dưới 1 ngày" : `Trễ ${tt.soNgay} ngày`;
  }
  return `Hạn ${nhanNgay(hanISO)}`;
}

/**
 * Ô hạn ở bảng Danh sách (§4.2): `10/9/2026 (trễ 6 ngày)`, phần trong ngoặc hiện đỏ; hoặc `—`.
 *
 * Trả về HAI MẢNH thay vì một chuỗi ghép sẵn, vì đặc tả tô đỏ riêng phần trễ. Ghép sẵn rồi cắt
 * lại bằng biểu thức chính quy ở tầng vẽ là dựng một phép phân tích trên chính chuỗi mình vừa tạo.
 */
export type OHan = {
  readonly ngay: string;
  /** Rỗng khi việc chưa trễ — tầng vẽ không phải đoán. */
  readonly phanTre: string;
};

export function oHan(hanISO: string | null, bayGio: Date): OHan {
  const tt = tinhTrangHan(hanISO, bayGio);
  if (tt.loai === "khong-han") return { ngay: O_TRONG, phanTre: "" };
  if (tt.loai === "tre") {
    return {
      ngay: nhanNgay(hanISO),
      phanTre: tt.soNgay < 1 ? "(trễ dưới 1 ngày)" : `(trễ ${tt.soNgay} ngày)`,
    };
  }
  return { ngay: nhanNgay(hanISO), phanTre: "" };
}

/**
 * Chip xanh lá `Hoàn thành trễ hạn` (§4.1, §6).
 *
 * SO VỚI `original_due_at`, KHÔNG SO VỚI `due_at` — và đó là toàn bộ điểm của hai cột. §5.6 và
 * §11 quy tắc 3 nói thẳng: tỷ lệ đúng hạn tính trên `ngay_hoan_thanh ≤ han_ban_dau`, và
 * `HẠN BAN ĐẦU` **không đổi khi gia hạn**. So với `due_at` thì mọi việc được duyệt lùi hạn đều
 * thành "đúng hạn", tức là một lần lùi hạn tự xoá dấu vết của chính nó khỏi báo cáo.
 */
export function hoanThanhTreHan(hoanThanhLucISO: string | null, hanBanDauISO: string | null): boolean {
  if (!hoanThanhLucISO || !hanBanDauISO) return false;
  const xong = Date.parse(hoanThanhLucISO);
  const han = Date.parse(hanBanDauISO);
  if (Number.isNaN(xong) || Number.isNaN(han)) return false;
  return xong > han;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ADR 0038 — AI DUYỆT ĐƯỢC ĐỀ NGHỊ LÙI HẠN
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Chưa phân công người thực hiện (§4.1, §4.2). Không phải dấu gạch: đây là một trạng thái thật. */
export const CHUA_PHAN_CONG = "Chưa phân công";

/**
 * Nút `Duyệt` / `Từ chối` của §5.8 có hiện hay không — **HAI LỚP, KHÔNG THAY NHAU** (ADR 0038).
 *
 *   task.extend                    tài khoản này có được đụng tới gia hạn không   — CỔNG CỦA TUYẾN,
 *                                  `authz.RequirePermission` ở `routes.go`
 *   ma === maLanhDaoGiaoViec       đây có phải việc của CHÍNH NGƯỜI ẤY không      — ở đây
 *
 * Bỏ lớp một thì ai gọi tuyến cũng được. Bỏ lớp hai thì **mọi lãnh đạo cầm khoá duyệt được mọi
 * nhiệm vụ của cả xã**, kể cả của bộ phận họ không liên quan — vì luật 5 kiểm
 * `(tenant_id, role, permission)` và không có chiều "bản ghi nào". Máy chủ giữ cả hai
 * (`DuocDuyetLuiHan`, `nhiem_vu_ghi.go:676`); màn hình chỉ soi lại cùng hai câu hỏi ấy để cán bộ
 * không bấm vào một thứ chắc chắn bị từ chối.
 *
 * ẨN MỘT NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP (luật 5, cấm #1). Nếu hàm này có ngày trả `hien:
 * true` sai, hậu quả là một màn hình hiện câu từ chối của máy chủ — không phải một lần duyệt lọt.
 *
 * SO SÁNH HAI **MÃ NGHIỆP VỤ CÁN BỘ** (`CB-…`), và đó là toàn bộ tính đúng đắn của nó.
 * `lanh_dao_giao_viec_ma` giữ `CB-…` (migration 0006 đặt tên cột như vậy) và
 * `phien.staff.code` giữ đúng loại giá trị ấy. Đem một id nội bộ ULID ra so với cột này là so
 * HAI LOẠI ĐỊNH DANH KHÁC NHAU: không bao giờ khớp, mọi lãnh đạo rơi vào nhánh "không phải người
 * được ghi", tính năng đơn giản là không chạy — mà không có gì đỏ ở đâu cả, vì cả hai đều là
 * chuỗi khác rỗng trông rất hợp lý (luật 6, bất biến 8; đã đo trong chính kho này 22/09/2026).
 *
 * HAI CHUỖI RỖNG KHÔNG ĐƯỢC PHÉP KHỚP NHAU, và đây là chỗ một dòng thiếu gây hỏng rộng nhất:
 * không có hai phép kiểm rỗng bên dưới thì `"" === ""` là đúng, và **mọi** tài khoản cầm
 * `task.extend` duyệt được **mọi** đề nghị trên **mọi** nhiệm vụ chưa ghi lãnh đạo giao việc.
 */
export type QuyetDinhDuyetLuiHan =
  | { readonly hien: true }
  /** Tài khoản không có `task.extend`. Tuyến sẽ trả 403 trước khi chạm tới quy tắc ADR 0038. */
  | { readonly hien: false; readonly vi: "thieu-quyen" }
  /** Nhiệm vụ chưa ghi lãnh đạo giao việc — ADR 0038 để ngỏ, và câu trả lời là TỪ CHỐI. */
  | { readonly hien: false; readonly vi: "chua-ghi-lanh-dao-giao-viec"; readonly thongBao: string }
  /** Có khoá, nhưng không phải người được ghi trên bản ghi này. */
  | { readonly hien: false; readonly vi: "khong-phai-lanh-dao-giao-viec"; readonly thongBao: string };

export const CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC =
  "Nhiệm vụ này chưa ghi lãnh đạo giao việc nên chưa ai duyệt được đề nghị lùi hạn — hãy bổ sung " +
  "lãnh đạo giao việc cho nhiệm vụ.";

export const CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC =
  "Chỉ lãnh đạo giao việc ghi trên nhiệm vụ này mới duyệt được đề nghị lùi hạn.";

export function quyetDinhDuyetLuiHan(
  /** `phien.staff.code` — mã nghiệp vụ của người đang đăng nhập (`CB-…`). */
  maNguoiDangNhap: string,
  /** `assigner` của nhiệm vụ — chính là `lanh_dao_giao_viec_ma`. */
  maLanhDaoGiaoViec: string,
  /** Tài khoản có khoá `task.extend` hay không — lớp CỔNG, do `lib/quyen.ts` trả lời. */
  coQuyenDuyetGiaHan: boolean,
): QuyetDinhDuyetLuiHan {
  // LỚP MỘT TRƯỚC. Thiếu khoá thì tuyến trả 403 bất kể người ấy có tên trên bản ghi hay không,
  // nên đây là câu trả lời đúng và cũng là câu trả lời rẻ nhất.
  if (!coQuyenDuyetGiaHan) return { hien: false, vi: "thieu-quyen" };

  if (maLanhDaoGiaoViec === "") {
    return {
      hien: false,
      vi: "chua-ghi-lanh-dao-giao-viec",
      thongBao: CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
    };
  }
  // Phiên không có mã cán bộ thì không so được với một cột giữ mã cán bộ. FAIL CLOSED.
  if (maNguoiDangNhap === "" || maNguoiDangNhap !== maLanhDaoGiaoViec) {
    return {
      hien: false,
      vi: "khong-phai-lanh-dao-giao-viec",
      thongBao: CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
    };
  }
  return { hien: true };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐẾM VÀ CÂU CHỮ CHUNG
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Chân trang §2: `Hiển thị {N} nhiệm vụ.` */
export function nhanBoDem(n: number): string {
  return `Hiển thị ${n} nhiệm vụ.`;
}

/** Hàng lọc 2 khi có dòng được tick (§2): `Đã chọn {N} nhiệm vụ`. */
export function nhanDaChon(n: number): string {
  return `Đã chọn ${n} nhiệm vụ`;
}

/** Cột Kanban rỗng (§4.1, phụ lục 15 §6). */
export const COT_RONG = "Không có nhiệm vụ";

/** Nhật ký rỗng (§5.9, phụ lục 15 §6). */
export const NHAT_KY_RONG = "Chưa có ghi chép nào.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CÂU CHỮ CỦA MÀN HÌNH — §2, §3, §5, §7
 *
 * Đặt ở đây chứ không rải trong `so-nhiem-vu.tsx` vì cùng một lý do `nhan-phieu.ts` làm thế: một
 * chuỗi giao diện nằm trong JSX là một chuỗi không bài kiểm nào so lại được với đặc tả mà không
 * phải kết xuất cả cây.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Quyển sổ rỗng. */
export const SO_RONG = "Chưa có nhiệm vụ nào khớp bộ lọc đang chọn.";

/** Đang đọc trang. `role="status"`, không phải `alert`. */
export const DANG_TAI_SO = "Đang tải sổ nhiệm vụ…";

/** Ô tìm §3 — nguyên văn placeholder đặc tả. */
export const TIM_PLACEHOLDER = "Tìm theo tên nhiệm vụ…";

/** Bảy nhãn "tất cả" của §3, nguyên văn. */
export const MOI_BO_PHAN_NHAN = "Tất cả bộ phận";
export const MOI_NGUOI_THUC_HIEN_NHAN = "Tất cả người thực hiện";
export const MOI_MUC_UU_TIEN_NHAN = "Mọi mức ưu tiên";
export const MOI_LOAI_NHAN = "Mọi loại nhiệm vụ";
export const MOI_KHOI_NHAN = "Mọi khối";
export const MOI_NGUON_GIAO_NHAN = "Mọi nguồn giao";
export const MOI_TRANG_THAI_NHAN = "Mọi trạng thái";
export const CHI_QUA_HAN_NHAN = "Chỉ việc quá hạn";

/** Hai tab phạm vi DỰNG ĐƯỢC. Tab thứ ba (`Liên quan đến tôi`) — xem `PHAN_CHUA_DUNG`. */
export const PHAM_VI_TOAN_XA = "Toàn xã";
export const PHAM_VI_CUA_TOI = "Giao cho tôi";

/** §5.3 — ô `ĐANG GIAO CHO` khi nhiệm vụ chưa về bộ phận nào. Không phải dấu gạch: một trạng thái thật. */
export const CHUA_GIAO_BO_PHAN = "Chưa giao bộ phận nào";

/** Lựa chọn mặc định của hai ô chọn ở §5.7 và §7.1 — nguyên văn đặc tả. */
export const CHUA_XAC_DINH = "— Chưa xác định —";

/** §5.8 — chú thích bắt buộc dưới ô đề nghị lùi hạn. */
export const GHI_CHU_LUI_HAN =
  "Hạn gốc vẫn được giữ lại để báo cáo đúng hạn không bị lùi theo.";

/** §7.1 — chú thích dưới ô `Lãnh đạo giao việc`. */
export const GHI_CHU_LANH_DAO_GIAO_VIEC =
  "Đề nghị lùi hạn sẽ gửi tới người này, qua chuông và qua thư.";

/** §7.1 — chú thích dưới ô `Tự sinh mã`. */
export const GHI_CHU_TU_SINH_MA =
  "Tự sinh sẽ cấp số tiếp theo trong dãy NV01, NV02… Nhập từ Excel cũng được đánh số tự động " +
  "theo dãy này.";

/** §5.4 — chú thích BẮT BUỘC hiển thị dưới hai ô tick phê duyệt. */
export const CHU_THICH_HAI_O_TICK =
  "Hai ô này đánh dấu bằng tay và không làm đổi trạng thái nhiệm vụ.";

/** §7 — mô tả dưới tiêu đề form `Giao việc mới`, nguyên văn. */
export const MO_TA_FORM_GIAO_VIEC =
  "Giao cho một bộ phận hoặc trực tiếp cho cán bộ. Giao cho bộ phận mà quá lâu chưa phân công " +
  "người thì hệ thống báo lên lãnh đạo.";

/**
 * Câu nói ra rằng hạn CHỈ ĐẶT ĐƯỢC MỘT LẦN, đặt ngay cạnh ô ngày ở form tạo.
 *
 * Không phải một lời nhắc lịch sự: `han_ban_dau` lấy cùng mốc với `han_xu_ly` lúc INSERT và
 * trigger `nhiem_vu_bat_bien` từ chối mọi lần ghi lại. Một nhiệm vụ tạo ra không có hạn thì
 * **không bao giờ** có hạn nữa — kể cả qua đường đề nghị lùi hạn, vì không có gì để lùi.
 */
export const CANH_BAO_HAN_MOT_LAN =
  "Hạn hoàn thành chỉ đặt được một lần, ngay lúc tạo. Bỏ trống thì nhiệm vụ này sẽ không có hạn " +
  "và cũng không đặt được về sau — kể cả qua đề nghị lùi hạn.";

/** §5.10 — việc con có hạn RIÊNG (ADR 0037 quyết định 2), không thừa kế hạn cha. */
export const GHI_CHU_HAN_VIEC_CON =
  "Việc con có hạn riêng, không lấy theo hạn của việc cha.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * Ô NGÀY ↔ MỐC CỦA HỢP ĐỒNG
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * `<input type="date">` phát ra `YYYY-MM-DD`; hợp đồng đòi một **mốc `date-time`** (`due_at`,
 * `new_due_at` là `time.Time` ở máy chủ, nên `"2026-12-20"` trần là **400**).
 *
 * ⚠ GIỜ TRONG NGÀY LÀ MỘT GIẢ ĐỊNH ĐƯỢC NÓI RA, KHÔNG PHẢI MỘT CHI TIẾT KỸ THUẬT. Đặc tả chỉ cho
 * một ô NGÀY (§7.1, §5.6), nên phải chọn một mốc trong ngày ấy, và hai lựa chọn không tương đương:
 *
 *   00:00  ⇒ nhiệm vụ "hạn 20/6" ĐÃ QUÁ HẠN suốt cả ngày 20/6. Đọc lên là sai.
 *   23:59  ⇒ đúng nghĩa thông thường của "hạn ngày 20/6", và là mốc dùng ở đây.
 *
 * KHÔNG DÙNG GIỜ TAN LÀM VIỆC (17:00 hay bất kỳ số nào): đó là `ca_lam_viec` của TỪNG XÃ, và một
 * con số như thế nung vào bundle là đúng thứ luật 1 bất biến 10 cấm. Nếu khách muốn hạn rơi vào
 * cuối giờ làm việc thì đó là một phép tính của `identity` (nơi giữ lịch làm việc, `ngay_nghi_le`
 * và `ngay_lam_bu`), không phải một hằng ở trình duyệt. Đã báo về như một giả định chờ khách chốt.
 *
 * MÚI GIỜ GHIM `+07:00`, không lấy múi giờ máy: một cán bộ mở màn hình trên máy đặt sai múi giờ
 * sẽ gửi lên một hạn lệch một ngày, và đó là một cam kết bị đọc sai.
 */
export function mocCuoiNgay(ngay: string): string {
  return `${ngay}T23:59:59+07:00`;
}

/**
 * Mốc của hợp đồng → `YYYY-MM-DD` để đổ vào `<input type="date">`.
 *
 * Cắt theo MÚI GIỜ VIỆT NAM chứ không `mocISO.slice(0, 10)`: một hạn `2026-06-20T23:59:59+07:00`
 * lưu ở dạng UTC là `2026-06-20T16:59:59Z`, cắt mười ký tự đầu vẫn ra `2026-06-20` — nhưng
 * `2026-06-20T00:30:00+07:00` là `2026-06-19T17:30:00Z`, và phép cắt ấy cho ra ngày **19**.
 */
export function ngayChoONhap(mocISO: string | null): string {
  if (mocISO === null || mocISO === "") return "";
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return "";
  const phan = new Intl.DateTimeFormat("en-CA", {
    timeZone: MUI_GIO,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(t));
  return phan;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢNG KANBAN §4.1 — CÂU CHỮ VÀ HAI PHÉP QUYẾT ĐỊNH
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Hai chế độ xem DỰNG ĐƯỢC. Chế độ thứ ba (`Sổ theo dõi` §4.3) — xem `PHAN_CHUA_DUNG`. */
export const NHAN_CHE_DO_KANBAN = "▦ Kanban";
export const NHAN_CHE_DO_DANH_SACH = "☰ Danh sách";

/** Đặt cạnh cụm chọn chế độ xem, để cán bộ không đi tìm cái nút thứ ba của đặc tả. */
export const GHI_CHU_THIEU_SO_THEO_DOI =
  "Đặc tả có chế độ xem thứ ba — Sổ theo dõi (§4.3) — và nó chưa dựng được. Lý do nằm ở phần " +
  "chưa dựng được đầu màn.";

/**
 * Cột Kanban nào còn phải đọc, sau khi tính bộ lọc `Trạng thái` của §3.
 *
 * ĐÂY LÀ PHÉP AND CỦA MÁY CHỦ ĐƯỢC NÓI RA Ở MÀN HÌNH, không phải một sự tối ưu. Kanban đọc mỗi
 * cột bằng một lời gọi mang `status=<mã cột>`; nếu cán bộ đang lọc `status=cho-duyet` thì bốn cột
 * kia CHẮC CHẮN rỗng — gửi bốn lời gọi nữa chỉ để nhận về bốn trang rỗng là bốn lời gọi thừa, và
 * một cột rỗng vì bộ lọc trông y hệt một cột rỗng vì xã không có việc nào.
 *
 * MỘT TRẠNG THÁI RẼ NHÁNH HOẶC MỘT MÃ LẠ CHO RA MẢNG RỖNG, và đó là câu trả lời đúng: §4.1 cố ý
 * không cho `tam-dung` và `chuyen-tiep` một cột nào. Màn hình phải NÓI RA điều ấy
 * (`CAU_LOC_TRANG_THAI_KHONG_CO_COT`) chứ không vẽ năm cột rỗng — năm cột rỗng đọc lên là "xã
 * không có việc nào", đúng điều ngược lại với sự thật.
 */
export function cotPhaiDoc(trangThaiDangLoc: string | undefined): readonly TrangThaiNhiemVu[] {
  if (trangThaiDangLoc === undefined || trangThaiDangLoc === "") return TRANG_THAI_CHINH;
  return TRANG_THAI_CHINH.filter((ma) => ma === trangThaiDangLoc);
}

/** Hiện khi bộ lọc Trạng thái chọn một trạng thái mà Kanban không có cột. */
export const CAU_LOC_TRANG_THAI_KHONG_CO_COT =
  "Bộ lọc Trạng thái đang chọn một trạng thái không có cột trên Kanban. Chuyển sang chế độ Danh " +
  "sách để xem những nhiệm vụ ấy.";

/**
 * Câu đứng dưới bảng, và nó là chỗ một cán bộ tìm ra việc của mình khi việc ấy "biến mất".
 *
 * Một việc vừa chuyển sang `tam-dung` rời khỏi Kanban HOÀN TOÀN — không phải lỗi, mà là §4.1. Không
 * nói ra thì người giao việc kết luận nhiệm vụ đã bị xoá.
 */
export const GHI_CHU_KANBAN_RE_NHANH =
  "Hai trạng thái rẽ nhánh — Tạm dừng và Chuyển tiếp — không có cột riêng trên Kanban (§4.1), nên " +
  "việc đang ở hai trạng thái ấy không hiện ở bảng này. Xem chúng ở chế độ Danh sách.";

/**
 * Con số trên đầu cột — **SỐ THẺ ĐÃ TẢI VỀ**, không phải tổng số việc của cột.
 *
 * `page.Result` của tuyến đọc sổ chỉ mang `items` · `next_cursor` · `has_more`: **KHÔNG CÓ TỔNG
 * SỐ**. Nên một con số trần ở đây sẽ đọc ra là "cột này có 20 việc" trong khi sự thật là "cột này
 * có ít nhất 20 việc", và con số ấy đi thẳng vào một câu báo cáo miệng với lãnh đạo. Dấu `+` là
 * toàn bộ phần trung thực của nhãn này; `GHI_CHU_DEM_COT` nói nốt phần còn lại.
 */
export function nhanDemCot(soThe: number, conNua: boolean): string {
  return conNua ? `${soThe}+` : String(soThe);
}

export const GHI_CHU_DEM_COT =
  "Số trên đầu mỗi cột là số thẻ đã tải về cột ấy; dấu + nghĩa là còn nữa. Tuyến đọc sổ trả về " +
  "từng trang và không trả tổng số, nên đây không phải tổng số việc của cột.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` màn
 * Thu - Chi dùng (`features/thu-chi/nhan-thu-chi.ts`).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Ô đổi bộ phận / người thực hiện ở form sửa (§5.7)",
    viSao:
      "`task.assign` chưa có tuyến, và `PATCH /api/v1/tasks/{ma}` CỐ Ý không đổi `bo_phan_id` hay " +
      "`nguoi_thuc_hien_ma`. Gộp việc giao người vào form sửa sẽ trao quyền giao việc cho mọi tài " +
      "khoản cầm `task.update` — một khoá khác hẳn, và bảng `quyen` có sẵn `task.assign` để tách " +
      "hai việc ấy ra.",
  },
  {
    ten: "`Chuyển tiếp` không nối hai nhiệm vụ (§6)",
    viSao:
      "§6 viết chuyển tiếp `sinh bản ghi liên kết`, nhưng KHÔNG CỘT NÀO nối hai nhiệm vụ với " +
      "nhau. Hệ quả có thật, và nó hiện ngay trên màn: một nhiệm vụ cha còn việc con đã " +
      "`chuyen-tiep` thì KHÔNG hoàn thành được — máy chủ chỉ tính `hoan-thanh` là xong, vì nó " +
      "không có đường nào lần theo để biết việc đã chuyển đi có xong ở nơi khác hay chưa.",
  },
  {
    ten: "Bộ lọc `Liên quan đến tôi` (§3)",
    viSao:
      "Máy chủ TỪ CHỐI `?scope=related` kèm lý do: bộ lọc này cần `bộ phận tôi đang giữ`, mà hợp " +
      "đồng phiên cán bộ không trả về bộ phận. Trả lời bằng ba vế còn lại sẽ tệ hơn từ chối — cán " +
      "bộ có việc do bộ phận mình đang giữ sẽ KHÔNG thấy nó trên đúng cái tab mang tên ấy.",
  },
  {
    ten: "Ô tick `Sắp đến hạn` (§3)",
    viSao:
      "Máy chủ TỪ CHỐI `?soon=`: ngưỡng là `sla.gio_sap_den_han`, số giờ của TỪNG XÃ, và chưa có " +
      "đường đọc số ấy. Con số `72 giờ` ở đặc tả là MẶC ĐỊNH xã ghi đè được, nên nung nó vào màn " +
      "hình là dựng nguồn thứ hai cho một con số mà mọi chỗ `sắp đến hạn` phải đọc từ một cột duy nhất.",
  },
  {
    ten: "Nhật ký & Trao đổi (§5.9) và `Tiếp tục` sau khi tạm dừng (§6)",
    viSao:
      "Hợp đồng không có tuyến nhật ký nào — tám tuyến Nhiệm vụ không gồm `POST .../nhat-ky` hay " +
      "tuyến đọc dòng thời gian. Kéo theo một chỗ nữa: trạng thái trước lúc tạm dừng nằm TRONG " +
      "nhật ký, nên màn hình không biết việc đang tạm dừng phải quay về đâu; nó hiện cả bốn lối và " +
      "để máy chủ từ chối ba lối sai.",
  },
  {
    ten: "Chip `{n} việc con` trên thẻ (§4.1) và khối Nhiệm vụ con (§5.10)",
    viSao:
      "`petitions.nhiemVuRa` có `parent` (mã việc cha) nhưng KHÔNG có số việc con, và không có " +
      "tuyến liệt kê việc con của một mã. Đếm trong trang đang mở sẽ ra một con số phụ thuộc vào " +
      "trang — `2 việc con` ở trang này và `3 việc con` ở trang sau, cho cùng một nhiệm vụ.",
  },
  {
    ten: "Danh mục `Trạng thái nhiệm vụ` xã sửa được (§6)",
    viSao:
      "ADR 0035 §C cho xã đổi NHÃN và THỨ TỰ bảy trạng thái, nhưng hợp đồng chỉ có `task-types`, " +
      "`task-priorities` và `task-blocs` — không có tuyến phát ra danh mục trạng thái. Bảy nhãn " +
      "trên màn hôm nay là chữ của đặc tả, giống nhau ở mọi xã.",
  },
  {
    ten: "Khối `SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO` (§5.4) và ba danh sách văn bản ở form (§7.2)",
    viSao:
      "Bảng `nhiem_vu_van_ban` CHƯA TỒN TẠI (migration 0006 ghi rõ sự vắng mặt), nên " +
      "`petitions.nhiemVuRa` không có trường nào cho ba nhóm văn bản và `petitions.taoNhiemVuVao` " +
      "không nhận chúng. Đây là chỗ phải cẩn thận khi đọc: KHÔNG có trường rỗng để hiểu thành " +
      "`chưa có văn bản nào` — không có trường. Vẽ một danh sách rỗng là nói với cán bộ một điều " +
      "chưa ai ghi. Cột `Sổ theo dõi` của §4.3 cũng thiếu đúng phần này.",
  },
  {
    ten: "Sửa `Hạn hoàn thành` ở form sửa (§5.4, §5.6)",
    viSao:
      "`PATCH /api/v1/tasks/{ma}` không nhận `due_at`. Hạn ấn định MỘT LẦN lúc tạo việc và sau đó " +
      "chỉ dịch được qua đường đề nghị lùi hạn có người duyệt (§5.8) — cố ý, vì hạn là cam kết đã " +
      "đưa ra, và `han_ban_dau` bị trigger `nhiem_vu_bat_bien` cấm ghi lại sau khi tạo. Một ô ngày " +
      "sửa trực tiếp ở form sẽ là đường vòng qua đúng vòng duyệt ấy.",
  },
  {
    ten: "⬆ Nhập từ Excel (§8) · 🗑 Xoá đã chọn (§2) · Xuất Sổ theo dõi (§4.3)",
    viSao:
      "Ba tuyến §10 đề xuất — `nhap-excel`, `xoa-nhieu`, `xuat-so-theo-doi` — không có trong hợp " +
      "đồng. Xoá từng nhiệm vụ thì có (`DELETE /api/v1/tasks/{ma}`, kèm lý do bắt buộc), nên thao " +
      "tác hàng loạt là bấm từng dòng chứ không phải một nút gom.",
  },
  {
    ten: "THÊM VIỆC CON (§5.10) và chuyển việc sang cha khác (§5.4)",
    viSao:
      "`petitions.taoNhiemVuVao.parent` và `petitions.suaNhiemVuVao.parent` nhận **id nội bộ** " +
      "của việc cha, nhưng `petitions.nhiemVuRa` KHÔNG phát ra `id` của chính nó — chỉ có `code` " +
      "(`NV19`) và `parent` (id của cha). Nên một màn hình đang mở NV19 biết cha của NV19 là ai mà " +
      "không bao giờ biết id của NV19, tức không gửi nổi một yêu cầu tạo việc con. Vẽ ô ấy ra rồi " +
      "gửi `NV19` vào `parent` sẽ là 409 `task_tree` ở mọi lần bấm. Cần thêm `id` vào phản hồi, " +
      "hoặc cho `parent` nhận mã sổ.",
  },
  {
    ten: "Khối `Duyệt / Từ chối` đề nghị lùi hạn của người khác (§5.8)",
    viSao:
      "Tuyến quyết định cần `deNghiID`, mà hợp đồng KHÔNG có tuyến nào liệt kê đề nghị đang chờ " +
      "của một nhiệm vụ, và `petitions.nhiemVuRa` cũng không mang đề nghị nào. Màn hình chỉ cầm " +
      "được id của đúng đề nghị VỪA GỬI trong cùng lượt mở — trong khi người duyệt theo ADR 0038 " +
      "là một người KHÁC, mở drawer sau đó. Phép kiểm hai lớp của ADR 0038 vẫn chạy và vẫn hiện " +
      "câu từ chối, nhưng lãnh đạo không có đề nghị nào để bấm.",
  },
  {
    ten: "Ô chọn `Người thực hiện` · `Lãnh đạo giao việc` · `Chuyên viên theo dõi` (§3, §7)",
    viSao:
      "Ba ô ấy cần danh bạ cán bộ, mà `GET /api/v1/staff` đứng sau `admin.user` — khoá quản trị " +
      "danh bạ, không phải khoá nhiệm vụ. Một cán bộ có `task.create` mà không có `admin.user` sẽ " +
      "nhận 403 khi màn hình đi đổ ba ô chọn ấy. Nên ba ô là **ô gõ mã cán bộ** (`CB-…`) chứ không " +
      "phải ô chọn, và chúng nhận đúng loại giá trị ba cột kia giữ (luật 6, bất biến 8).",
  },
  {
    ten: "CỔNG QUYỀN Ở GIAO DIỆN cho bảy khoá `task.*`",
    viSao:
      "Bảy khoá `task.read` · `task.create` · `task.update` · `task.approve` · `task.extend` · " +
      "`task.delete` · `task.assign` ĐỀU CÓ trong bảng `quyen` và đều được tuyến khai, nhưng " +
      "`lib/quyen.ts` chưa có hằng nào cho chúng và lượt này không được sửa tệp ấy. Hệ quả: màn " +
      "hình vẽ đủ nút cho mọi tài khoản đọc được sổ, và câu 403 của máy chủ ra thẳng màn hình — " +
      "cùng khuôn `document.read` đã chọn. Lớp chặn thật không đổi (máy chủ kiểm từng yêu cầu); " +
      "thứ thiếu là sự tiện dụng. RIÊNG lớp hai của ADR 0038 vẫn chạy: nút duyệt vẫn ẩn với người " +
      "không phải lãnh đạo giao việc ghi trên bản ghi.",
  },
  {
    ten: "KÉO-THẢ thẻ giữa các cột Kanban (§4.1)",
    viSao:
      "Không dựng, và lý do là KHẢ NĂNG TIẾP CẬN chứ không phải công sức: kéo-thả HTML5 không có " +
      "lối bàn phím tương đương, nên một bảng chỉ đổi được trạng thái bằng cách kéo là một bảng " +
      "cán bộ dùng bàn phím — hoặc dùng chuột không vững — không thao tác được " +
      "(`skills/accessibility-elderly`). Đổi trạng thái trên Kanban đi qua drawer: bấm `Mở NV…` " +
      "trên thẻ, dùng khối `Chuyển trạng thái` §6. ĐÓ LÀ CÙNG MỘT TUYẾN " +
      "(`POST /api/v1/tasks/{ma}/status`) và cùng chỗ in NGUYÊN VĂN câu từ chối của máy chủ — kể " +
      "cả câu liệt kê mã việc con còn lại khi hoàn thành việc cha.",
  },
  {
    ten: "SỐ LƯỢNG THẬT của mỗi cột Kanban (§4.1)",
    viSao:
      "`page.Result` chỉ mang `items` · `next_cursor` · `has_more` — KHÔNG có tổng số. Nên đầu cột " +
      "hiện số thẻ đã tải kèm dấu `+` khi còn nữa, chứ không hiện một con số trần: `20` đọc ra là " +
      "`cột này có 20 việc`, trong khi sự thật là `ít nhất 20`, và con số ấy đi thẳng vào một câu " +
      "báo cáo với lãnh đạo. Kanban cũng vì thế không có phân trang từng cột — xem tiếp ở Danh sách.",
  },
  {
    ten: "Chế độ xem `Sổ theo dõi` (§4.3)",
    viSao:
      "Cụm chọn chế độ xem có HAI nút chứ không phải ba. Bảng §4.3 lấy quá nửa số cột từ bảng " +
      "`nhiem_vu_van_ban` chưa tồn tại (xem mục §5.4 ở trên), nên dựng ra sẽ là một quyển sổ công " +
      "văn chỉ đạo thiếu đúng phần văn bản chỉ đạo.",
  },
];

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * KHÔNG CÓ HÀM NÀO Ở ĐÂY VIẾT LẠI MỘT CÂU TỪ CHỐI CỦA MÁY CHỦ, VÀ SỰ VẮNG MẶT ẤY LÀ CHỦ Ý
 *
 * Hai lần từ chối nặng nhất của màn này mang thông tin NẰM TRONG CÂU CHỮ, không nằm trong mã lỗi:
 *
 *   hoàn thành cha còn con   "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi
 *                            mới hoàn thành việc cha"        (`LoiConChuaXong`, nhiem_vu_ghi.go:331)
 *   xoá cha còn con          "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước"
 *                                                            (`LoiConChuaXoa`, nhiem_vu_ghi.go:323)
 *
 * DANH SÁCH MÃ VÀ CON SỐ LÀ TOÀN BỘ PHẦN CÓ ÍCH. Nuốt chúng thành "có lỗi xảy ra" để lại cho cán
 * bộ đúng thông tin bằng không: họ biết mình không được phép, và không biết còn vướng ở đâu.
 *
 * Nên tầng gọi và màn hình vẽ THẲNG `KetQua.thongBao` của `lib/api/goi.ts` — nó đã mang nguyên
 * văn `message` máy chủ viết. Một hằng chuỗi ở đây sẽ là bản sao thứ hai của một quy tắc nghiệp
 * vụ, và bản sao ấy trôi mà không bài kiểm nào đỏ (luật 9, cấm #2).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */
