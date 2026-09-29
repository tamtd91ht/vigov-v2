/**
 * Ngăn xếp con trỏ — cách duy nhất để có nút "Trang trước" trên một danh sách đọc theo mốc.
 *
 * VÌ SAO PHẢI TỰ GIỮ NGĂN XẾP: hợp đồng chỉ trả `next_cursor` + `has_more`, tức là chỉ trả
 * đường đi **xuôi**. Không có `prev_cursor`, và không có `offset` — máy chủ đọc theo mốc
 * (keyset) và cố ý không đếm tổng số (`core/page/page.go`). Muốn quay lại thì client phải nhớ
 * mình đã đi qua những mốc nào.
 *
 * CẠM BẪY Ở ĐÂY LÀ MỘT DÒNG RẤT NGẮN: `?offset=` — bịa ra một tham số máy chủ không nhận. Nó
 * không làm đỏ test nào ở client, vì client chỉ dựng chuỗi; nó trả 400 lúc chạy, và chỉ ở
 * trang thứ hai.
 *
 * `null` là trang đầu: "không có con trỏ". Cố ý không dùng chuỗi rỗng cho trạng thái ấy —
 * `has_more: false` đi kèm `next_cursor: ""` theo hợp đồng, nên chuỗi rỗng đã có nghĩa khác.
 */

export type NganXepConTro = {
  /** Những mốc đã đi qua, cũ nhất trước. Phần tử đầu là `null` — trang đầu tiên. */
  readonly daQua: readonly (string | null)[];
  /** Mốc của trang đang xem. `null` là trang đầu. */
  readonly hienTai: string | null;
};

export const TRANG_DAU: NganXepConTro = { daQua: [], hienTai: null };

/**
 * Sang trang sau, nhớ lại mốc của trang đang xem để còn quay về.
 *
 * Con trỏ rỗng là lỗi lập trình, không phải một trạng thái: hợp đồng chỉ phát ra `next_cursor`
 * không rỗng khi `has_more` là `true`. Nên ở đây hỏng to và hỏng ngay, thay vì âm thầm nhảy về
 * trang đầu — đó là kiểu hỏng hiện cho cán bộ trang 1 trong khi họ tin mình đang ở trang 4.
 */
export function sangTrangSau(nganXep: NganXepConTro, conTroTiep: string): NganXepConTro {
  if (conTroTiep === "") {
    throw new Error("sangTrangSau: con trỏ rỗng — chỉ gọi khi has_more và next_cursor có giá trị");
  }
  return { daQua: [...nganXep.daQua, nganXep.hienTai], hienTai: conTroTiep };
}

/** Có trang trước để về hay không — quyết định nút "Trang trước" có bấm được. */
export function coTrangTruoc(nganXep: NganXepConTro): boolean {
  return nganXep.daQua.length > 0;
}

/** Về trang trước. Đang ở trang đầu thì đứng yên — không có gì để bật lên khỏi ngăn xếp. */
export function veTrangTruoc(nganXep: NganXepConTro): NganXepConTro {
  if (nganXep.daQua.length === 0) return TRANG_DAU;
  const truoc = nganXep.daQua[nganXep.daQua.length - 1] ?? null;
  return { daQua: nganXep.daQua.slice(0, -1), hienTai: truoc };
}
