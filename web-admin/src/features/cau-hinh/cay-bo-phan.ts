/**
 * Phần QUYẾT ĐỊNH của tab "Sơ đồ tổ chức" — hàm thuần, không dựng DOM, nên kiểm được đúng những ca
 * không nhìn thấy trên một màn hình đang chạy tốt:
 *
 *   1. cây dựng từ danh sách phẳng (`parent_id`), con dưới cha, sắp theo `order` rồi theo tên;
 *   2. ô `Bộ phận cha` của biểu mẫu sửa không chứa chính bộ phận ấy lẫn con cháu của nó;
 *   3. thân PATCH chỉ mang trường ĐÃ ĐỔI, không bao giờ mang `code`, và "dời lên gốc" là
 *      `parent_id: ""` chứ không phải vắng mặt;
 *   4. `Idempotency-Key` của lần thêm sinh lúc MỞ biểu mẫu và đi theo MỌI lần bấm `Lưu` của biểu
 *      mẫu ấy.
 *
 * Tách khỏi `tab-so-do-to-chuc.tsx` vì một nhánh `if` giữa hai thẻ JSX là nhánh không bài test nào
 * chạm tới (cùng lý lẽ `tang-danh-muc.ts` của tab Danh mục).
 */

import type { KetQua } from "@/lib/api/goi";
import type { identity_boPhanDaGhiRa, identity_boPhanRa } from "@/lib/api/schema.gen";
import {
  suaBoPhan,
  themBoPhan,
  type SuaBoPhanVao,
  type ThemBoPhanVao,
} from "@/lib/api/so-do-to-chuc";

import { CHUA_CO_THAY_DOI, LOI_THU_TU, daLuu, daThem } from "./nhan-so-do";

/* ---- cây ------------------------------------------------------------------------------------ */

export type NutCay = { readonly bp: identity_boPhanRa; readonly con: readonly NutCay[] };

/**
 * Thứ tự trong CÙNG MỘT CẤP: `order` trước, tên sau — đúng khoá máy chủ sắp danh sách
 * (`service-identity/internal/store/bo_phan.go:74`, `ORDER BY bp.thu_tu, bp.ten`).
 *
 * SẮP LẠI Ở ĐÂY DÙ MÁY CHỦ ĐÃ SẮP, vì máy chủ sắp một danh sách PHẲNG: con của bộ phận đứng đầu
 * có thể nằm cuối mảng. Gom con dưới cha rồi giữ thứ tự mảng thì đúng hôm nay và sai lặng lẽ vào
 * ngày tuyến đổi cách sắp — một phép sắp tường minh không phụ thuộc điều ấy.
 */
function soSanh(a: identity_boPhanRa, b: identity_boPhanRa): number {
  return a.order - b.order || a.name.localeCompare(b.name, "vi");
}

/**
 * Dựng cây từ danh sách phẳng.
 *
 * KHÔNG BỎ MẤT BỘ PHẬN NÀO. Một bộ phận có `parent_id` không nằm trong danh sách, hay một vòng lặp
 * trong dữ liệu (máy chủ chặn bằng 409, nhưng đây là chỗ đọc chứ không phải chỗ ghi), được đưa lên
 * cấp cao nhất thay vì biến mất: một bộ phận không hiện trên sơ đồ là một bộ phận không ai sửa được,
 * trong khi việc vẫn đang được giao cho nó ở các màn khác.
 */
export function dungCay(items: readonly identity_boPhanRa[]): readonly NutCay[] {
  const coId = new Set(items.map((b) => b.id));
  const conCua = new Map<string, identity_boPhanRa[]>();
  const goc: identity_boPhanRa[] = [];

  for (const b of items) {
    if (b.parent_id === "" || b.parent_id === b.id || !coId.has(b.parent_id)) {
      goc.push(b);
    } else {
      const ds = conCua.get(b.parent_id) ?? [];
      ds.push(b);
      conCua.set(b.parent_id, ds);
    }
  }

  const daVe = new Set<string>();
  const dung = (b: identity_boPhanRa): NutCay => {
    daVe.add(b.id);
    const con: NutCay[] = [];
    for (const c of [...(conCua.get(b.id) ?? [])].sort(soSanh)) {
      if (!daVe.has(c.id)) con.push(dung(c));
    }
    return { bp: b, con };
  };

  const cay: NutCay[] = [];
  for (const b of [...goc].sort(soSanh)) {
    if (!daVe.has(b.id)) cay.push(dung(b));
  }
  // Bộ phận chỉ nằm trong một vòng lặp thì không gốc nào chạm tới. Đưa lên cấp cao nhất.
  for (const b of [...items].sort(soSanh)) {
    if (!daVe.has(b.id)) cay.push(dung(b));
  }
  return cay;
}

/** Một dòng của cây trải phẳng theo thứ tự hiển thị — `cap` 0 là cấp cao nhất. */
export type DongPhang = { readonly bp: identity_boPhanRa; readonly cap: number };

/** Trải cây theo chiều sâu, đúng thứ tự thẻ hiện trên màn hình. Dùng cho ô `Bộ phận cha`. */
export function traiPhang(cay: readonly NutCay[]): readonly DongPhang[] {
  const ra: DongPhang[] = [];
  const di = (nut: NutCay, cap: number) => {
    ra.push({ bp: nut.bp, cap });
    for (const c of nut.con) di(c, cap + 1);
  };
  for (const n of cay) di(n, 0);
  return ra;
}

/** `id` và mọi bộ phận con cháu của nó, theo `parent_id`. */
export function voiConChau(items: readonly identity_boPhanRa[], id: string): ReadonlySet<string> {
  const ket = new Set<string>([id]);
  let themDuoc = true;
  // Lặp tới khi không thêm được gì: đúng cả khi danh sách không theo thứ tự cha trước con.
  while (themDuoc) {
    themDuoc = false;
    for (const b of items) {
      if (!ket.has(b.id) && ket.has(b.parent_id)) {
        ket.add(b.id);
        themDuoc = true;
      }
    }
  }
  return ket;
}

/**
 * Những bộ phận được chọn làm cha.
 *
 * Biểu mẫu sửa bỏ CHÍNH bộ phận ấy và MỌI con cháu của nó. Đây là tiện dụng, không phải phép chặn:
 * máy chủ từ chối vòng lặp bằng 409 `org_unit_cycle` kèm câu của nó, và màn hình hiện nguyên câu ấy
 * nếu ô chọn từng lệch với dữ liệu (một người khác vừa dời bộ phận trong lúc biểu mẫu đang mở).
 */
export function luaChonCha(
  cay: readonly NutCay[],
  items: readonly identity_boPhanRa[],
  dangSuaId: string | null,
): readonly DongPhang[] {
  const tat = traiPhang(cay);
  if (dangSuaId === null) return tat;
  const loai = voiConChau(items, dangSuaId);
  return tat.filter((d) => !loai.has(d.bp.id));
}

/* ---- biểu mẫu ------------------------------------------------------------------------------- */

/**
 * Biểu mẫu nào đang mở. MỘT biểu mẫu cho cả tab: hai bản nháp mở cùng lúc trên màn 320px là một
 * bản nháp không nhìn thấy, và bản nháp không nhìn thấy là bản bị gửi nhầm.
 *
 * `chaId` của lần thêm: `""` là thêm ở cấp cao nhất (nút `+ Thêm bộ phận`), một id là thêm bộ phận
 * con (nút `＋` trên thẻ). Biểu mẫu mở ngay dưới thẻ nó nói tới — xem `theNeo`.
 */
export type DangMo =
  | {
      readonly kieu: "them";
      readonly chaId: string;
      readonly tenCha: string | null;
      readonly khoaChongTrung: string;
    }
  | { readonly kieu: "sua"; readonly bp: identity_boPhanRa };

/** Bản nháp đang gõ. Chuỗi hết, kể cả thứ tự — ô nhập của trình duyệt trả về chuỗi. */
export type BanNhap = {
  readonly ten: string;
  readonly ma: string;
  readonly chaId: string;
  readonly thuTu: string;
};

/**
 * Mở biểu mẫu thêm — và SINH KHOÁ CHỐNG TRÙNG NGAY LÚC NÀY, không lúc gửi.
 *
 * Bấm `Lưu` lần thứ hai sau một lỗi mạng phải mang ĐÚNG khoá của lần đầu: lần đầu có thể đã tới
 * máy chủ, và một khoá mới biến lần thử lại thành một bộ phận thứ hai cùng tên trên sơ đồ. Khoá
 * sống bằng đời một lần mở biểu mẫu; đóng rồi mở lại là một lần thêm khác, một khoá khác.
 *
 * `taoKhoa` là tham số để bài test đếm được số lần sinh; màn hình truyền `crypto.randomUUID`.
 */
export function moThem(chaId: string, tenCha: string | null, taoKhoa: () => string): DangMo {
  return { kieu: "them", chaId, tenCha, khoaChongTrung: taoKhoa() };
}

/**
 * Thẻ mà biểu mẫu mở ngay dưới: bộ phận cha của lần thêm con, bộ phận đang sửa — `null` là đầu
 * tab (thêm ở cấp cao nhất). Mở tại chỗ để ở 320px cán bộ vẫn thấy thẻ mình vừa bấm.
 */
export function theNeo(dangMo: DangMo): string | null {
  if (dangMo.kieu === "sua") return dangMo.bp.id;
  return dangMo.chaId === "" ? null : dangMo.chaId;
}

/**
 * Nút mở biểu mẫu — tiêu điểm TRẢ VỀ đây khi biểu mẫu đóng, để người dùng bàn phím không bị đẩy
 * về đầu trang. Một hàm cho cả hai phía (nút vẽ `id` này, tab tìm lại nó), nên hai bên không lệch.
 */
export function idNutMo(kieu: "themGoc" | "themCon" | "sua", boPhanId = ""): string {
  switch (kieu) {
    case "themGoc":
      return "nut-them-bo-phan";
    case "themCon":
      return `nut-them-con-${boPhanId}`;
    case "sua":
      return `nut-sua-bo-phan-${boPhanId}`;
  }
}

export function banThem(chaId: string): BanNhap {
  return { ten: "", ma: "", chaId, thuTu: "" };
}

export function banSua(bp: identity_boPhanRa): BanNhap {
  return { ten: bp.name, ma: bp.code, chaId: bp.parent_id, thuTu: String(bp.order) };
}

/**
 * Ô `Thứ tự`: rỗng = không nhập; số nguyên = số ấy; thứ khác = lỗi TẠI CHỖ.
 *
 * KHÔNG `parseInt`: `parseInt("3 chữ")` trả 3, tức nuốt một lỗi gõ thành một con số. Và không lặng
 * lẽ bỏ qua ô gõ sai như bỏ qua ô rỗng: cán bộ gõ "ba" rồi thấy bộ phận không nhúc nhích sẽ không
 * biết vì sao.
 */
function docThuTu(o: string): { ok: true; giaTri: number | undefined } | { ok: false } {
  const sach = o.trim();
  if (sach === "") return { ok: true, giaTri: undefined };
  const n = Number(sach);
  return Number.isInteger(n) ? { ok: true, giaTri: n } : { ok: false };
}

export type ThanDung<T> =
  | { readonly kieu: "gui"; readonly than: T }
  | { readonly kieu: "khongDoi" }
  | { readonly kieu: "loi"; readonly loi: string };

/**
 * Thân POST từ bản nháp — TỪNG TRƯỜNG.
 *
 * `code` RỖNG THÌ VẮNG MẶT, để máy chủ tự sinh mã từ tên. `parent_id` rỗng thì vắng (gốc). Không
 * kiểm tên rỗng, độ dài tên hay khuôn mã ở đây: máy chủ kiểm cả ba kèm một câu tiếng Việt nói rõ
 * phải sửa gì, và một bản sao ở client là bản sao sẽ trôi (luật 9).
 *
 * TÊN GỬI NGUYÊN CHỮ ĐÃ GÕ — không viết hoa hộ. Xem `GIAI_THICH_O_TEN`.
 */
export function thanThem(ban: BanNhap): ThanDung<ThemBoPhanVao> {
  const thuTu = docThuTu(ban.thuTu);
  if (!thuTu.ok) return { kieu: "loi", loi: LOI_THU_TU };
  const ma = ban.ma.trim();
  return {
    kieu: "gui",
    than: {
      name: ban.ten,
      parent_id: ban.chaId === "" ? undefined : ban.chaId,
      order: thuTu.giaTri,
      code: ma === "" ? undefined : ma,
    },
  };
}

/**
 * Thân PATCH — CHỈ những trường đã đổi so với bộ phận lúc mở biểu mẫu.
 *
 * `code` KHÔNG CÓ Ở ĐÂY, và kiểu `SuaBoPhanVao` không cho nó có: máy chủ từ chối 400 CẢ yêu cầu khi
 * thân nhắc tới `code` (`bo_phan.go:208`).
 *
 * `parent_id: ""` LÀ "DỜI LÊN CẤP CAO NHẤT", khác hẳn vắng mặt ("không đổi"). Ô chọn "Không có"
 * mang giá trị `""`, nên đổi từ một cha sang "Không có" gửi đúng chuỗi rỗng ấy.
 *
 * Ô `Thứ tự` bị xoá trắng thì KHÔNG ĐỔI, không phải số 0: đẩy một bộ phận lên đầu cấp vì cán bộ chỉ
 * định sửa tên là một lần đổi không ai yêu cầu.
 *
 * KHÔNG ĐỔI GÌ THÌ KHÔNG GỬI: máy chủ trả 400 cho thân rỗng, và một lần bấm `Lưu` không đổi gì đáng
 * nhận một câu nói đúng điều ấy hơn là một câu từ chối.
 */
export function thanSua(goc: identity_boPhanRa, ban: BanNhap): ThanDung<SuaBoPhanVao> {
  const thuTu = docThuTu(ban.thuTu);
  if (!thuTu.ok) return { kieu: "loi", loi: LOI_THU_TU };

  const than: { name?: string; parent_id?: string; order?: number } = {};
  if (ban.ten !== goc.name) than.name = ban.ten;
  if (ban.chaId !== goc.parent_id) than.parent_id = ban.chaId;
  if (thuTu.giaTri !== undefined && thuTu.giaTri !== goc.order) than.order = thuTu.giaTri;

  return Object.keys(than).length === 0 ? { kieu: "khongDoi" } : { kieu: "gui", than };
}

/* ---- gửi ------------------------------------------------------------------------------------ */

/** Hai lời gọi ghi. Tham số để bài test thay được; màn hình dùng `API_SO_DO`. */
export type ApiSoDo = {
  readonly them: (than: ThemBoPhanVao, khoa: string) => Promise<KetQua<identity_boPhanDaGhiRa>>;
  readonly sua: (id: string, than: SuaBoPhanVao) => Promise<KetQua<identity_boPhanDaGhiRa>>;
};

export const API_SO_DO: ApiSoDo = { them: themBoPhan, sua: suaBoPhan };

/**
 * Kết quả một lần bấm `Lưu`. HAI LOẠI LỖI RIÊNG: lỗi tại chỗ ("ô này gõ sai", chưa gửi gì) và câu
 * của máy chủ (nguyên văn — kể cả câu 409 về vòng lặp và câu 409 về mã đã dùng).
 */
export type KetQuaGui =
  | { readonly kieu: "xong"; readonly cau: string }
  | { readonly kieu: "loiTaiCho"; readonly loi: string }
  | { readonly kieu: "loiMayChu"; readonly thongBao: string };

/**
 * Gửi biểu mẫu đang mở.
 *
 * LẦN THÊM DÙNG `dangMo.khoaChongTrung` — khoá đã sinh lúc mở, không sinh ở đây. Hàm này được gọi
 * lại nguyên vẹn mỗi lần cán bộ bấm `Lưu` sau một lỗi, nên một khoá sinh trong thân hàm là một khoá
 * mới mỗi lần bấm.
 */
export async function guiBieuMau(dangMo: DangMo, ban: BanNhap, api: ApiSoDo): Promise<KetQuaGui> {
  if (dangMo.kieu === "them") {
    const d = thanThem(ban);
    if (d.kieu === "loi") return { kieu: "loiTaiCho", loi: d.loi };
    if (d.kieu === "khongDoi") return { kieu: "loiTaiCho", loi: CHUA_CO_THAY_DOI };
    const kq = await api.them(d.than, dangMo.khoaChongTrung);
    return kq.ok ? { kieu: "xong", cau: daThem(kq.duLieu.name) } : { kieu: "loiMayChu", thongBao: kq.thongBao };
  }

  const d = thanSua(dangMo.bp, ban);
  if (d.kieu === "loi") return { kieu: "loiTaiCho", loi: d.loi };
  if (d.kieu === "khongDoi") return { kieu: "loiTaiCho", loi: CHUA_CO_THAY_DOI };
  const kq = await api.sua(dangMo.bp.id, d.than);
  return kq.ok ? { kieu: "xong", cau: daLuu(kq.duLieu.name) } : { kieu: "loiMayChu", thongBao: kq.thongBao };
}
