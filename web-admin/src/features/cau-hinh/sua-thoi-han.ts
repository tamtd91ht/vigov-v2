/**
 * Biểu mẫu sửa một dòng thời hạn: từ năm ô nhập ra **đúng những trường đã đổi**. Hàm thuần: không
 * gọi mạng, không dựng DOM.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO PHẢI SO VỚI DÒNG GỐC CHỨ KHÔNG GỬI CẢ NĂM Ô.
 *
 * `PATCH /api/v1/sla/{id}` đọc "không nhắc tới" là "giữ nguyên" (`suaSLAVao` toàn con trỏ). Gửi cả
 * năm ô thì mọi lần sửa đều là một lần ghi đè năm con số — và hai quản trị viên sửa hai cột khác
 * nhau trên cùng một dòng sẽ khiến người lưu sau lặng lẽ kéo bốn cột kia về giá trị mình đã đọc
 * lúc mở màn hình. Không có gì báo lỗi, và thứ bị kéo lùi là một cam kết của xã với người dân.
 *
 * VÌ SAO CÓ PHÉP KIỂM SỐ Ở ĐÂY, TRONG KHI PHẦN CÒN LẠI CỐ Ý KHÔNG KIỂM GÌ.
 *
 * `Number("12 giờ")` ra `NaN`, và `JSON.stringify({x: NaN})` ra `{"x":null}` — mà `null` vào một
 * `*int` của Go là con trỏ rỗng, tức **"không nhắc tới"**. Nên một lỗi gõ sẽ đi qua toàn bộ đường
 * ghi và trở thành "không đổi gì", với màn hình báo Đã lưu. Đó là lý do phép kiểm này tồn tại: nó
 * chặn một giá trị biến thành sự im lặng, không phải chép lại quy tắc nghiệp vụ của máy chủ. Cận
 * trên, quan hệ giữa các cột và mọi từ chối khác vẫn là việc của máy chủ, và câu từ chối là câu
 * của máy chủ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { SuaGioVao } from "@/lib/api/thoi-han-xu-ly";
import type { identity_dongSLARa, identity_suaSLAVao } from "@/lib/api/schema.gen";

import { LOI_KHONG_DOI_GI, LOI_SO_GIO_LA } from "./nhan-thoi-han";

/** Khoá của một ô giờ — lấy thẳng từ hợp đồng, không gõ lại. */
export type KhoaGio = keyof identity_suaSLAVao;

/**
 * Tên cột trên màn hình. `Record` ĐÒI ĐỦ NĂM KHOÁ: ngày hợp đồng có con số thứ sáu, dòng này đỏ
 * ngay — chứ không lặng lẽ sinh ra một ô nhập không có nhãn, hoặc một cột không ai sửa được.
 *
 * Tên lấy đúng đầu cột của đặc tả (`14-cau-hinh.md §8`), không rút gọn: cán bộ đối chiếu màn hình
 * với bảng in trong tài liệu.
 */
export const NHAN_COT: Record<KhoaGio, string> = {
  acknowledge_hours: "Tiếp nhận",
  resolve_hours: "Xử lý xong",
  due_soon_hours: "Sắp đến hạn khi còn",
  escalate_leader_hours: "Báo lãnh đạo trực tiếp",
  escalate_president_hours: "Báo Chủ tịch",
};

/** Thứ tự cột trên bảng và trong biểu mẫu — theo đúng thứ tự đặc tả, không theo vần. */
export const COT_GIO = [
  "acknowledge_hours",
  "resolve_hours",
  "due_soon_hours",
  "escalate_leader_hours",
  "escalate_president_hours",
] as const satisfies readonly KhoaGio[];

/** Bản nháp đang gõ: chuỗi, vì ô nhập của trình duyệt trả về chuỗi. */
export type BanNhapGio = Record<KhoaGio, string>;

/** Nạp dòng đang có vào biểu mẫu — ô nào cũng có sẵn con số hiện tại, không ô nào trống. */
export function banTuDong(d: identity_dongSLARa): BanNhapGio {
  return {
    acknowledge_hours: String(d.acknowledge_hours),
    resolve_hours: String(d.resolve_hours),
    due_soon_hours: String(d.due_soon_hours),
    escalate_leader_hours: String(d.escalate_leader_hours),
    escalate_president_hours: String(d.escalate_president_hours),
  };
}

export type KetQuaSoan = { ok: true; than: SuaGioVao } | { ok: false; loi: string };

/**
 * Năm ô nhập + dòng gốc → thân `PATCH`.
 *
 * KHÔNG ĐỔI GÌ THÌ TỪ CHỐI TẠI CHỖ, không gửi `{}` lên để nhận 400. Cán bộ mở biểu mẫu rồi bấm Lưu
 * mà không sửa gì là chuyện thường; một câu ở đây đọc dễ hơn một lời từ chối của máy chủ.
 */
export function soanSua(goc: identity_dongSLARa, ban: BanNhapGio): KetQuaSoan {
  const than: { -readonly [K in KhoaGio]?: number } = {};

  for (const khoa of COT_GIO) {
    const sach = ban[khoa].trim();
    // Ô TRỐNG KHÔNG PHẢI "GIỮ NGUYÊN". Biểu mẫu mở ra đã có sẵn con số, nên một ô bị xoá trắng là
    // một ý định — và ý định ấy (bỏ hẳn một con số) không có trên tuyến. Nói ra, đừng đoán.
    const so = sach === "" ? Number.NaN : Number(sach);
    if (!Number.isInteger(so) || so <= 0) return { ok: false, loi: LOI_SO_GIO_LA };
    if (so !== goc[khoa]) than[khoa] = so;
  }

  if (Object.keys(than).length === 0) return { ok: false, loi: LOI_KHONG_DOI_GI };
  return { ok: true, than };
}
