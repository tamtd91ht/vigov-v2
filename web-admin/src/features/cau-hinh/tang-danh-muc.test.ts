import { describe, expect, it } from "vitest";

import type { MucDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";

import {
  LOI_THIEU_LY_DO_XOA,
  TANG_DON_VI,
  TANG_HE_THONG,
  TANG_RE_NHANH,
  choBatLai,
  kiemLyDoXoa,
  laMucGhi,
  thaoTacCuaMuc,
  thaoTacTheoTang,
} from "./tang-danh-muc";

/**
 * BA TẦNG → NHỮNG NÚT ĐƯỢC VẼ. Đây là quyết định mà một màn hình chạy tốt KHÔNG tiết lộ: mọi
 * dòng đều hiện, mọi nút đều bấm được, và cái sai chỉ lộ ra khi cán bộ bấm `Xoá` ở một dòng của
 * hệ thống và nhận về 409 — rồi kết luận phần mềm hỏng.
 *
 * Bảng đối chiếu, lấy từ `service-documents/internal/domain/danh_muc_ba_tang.go:46-55` và từ
 * trigger `danh_muc_ba_tang` trong `migrations/0003_danh_muc_loai_van_ban.sql`:
 *
 *   tầng 1 (`don-vi`)                    đổi nhãn CÓ · tắt CÓ    · xoá CÓ
 *   tầng 2 (`he-thong`)                  đổi nhãn CÓ · tắt CÓ    · xoá KHÔNG
 *   tầng 3 (`he-thong` + rẽ nhánh theo mã) đổi nhãn CÓ · tắt KHÔNG · xoá KHÔNG
 */

/** Một mục đúng hình dạng hợp đồng của năm danh mục có đường ghi. */
function muc(tier: number, active = true): MucDanhMuc {
  return {
    id: `01JH-${tier}`,
    code: "cong-van",
    label: "Công văn",
    active,
    is_default: false,
    order: 1,
    source: tier === TANG_DON_VI ? "don-vi" : "he-thong",
    tier,
  };
}

/** Một mục của hai danh mục CHỈ ĐỌC — hợp đồng không phát ra `order`, `source`, `tier`. */
function mucChiDoc(): MucDanhMuc {
  return { id: "01JH-x", code: "thon", label: "Thôn", active: true, is_default: false };
}

describe("thao tác được phép theo tầng", () => {
  it("tầng 1 — đơn vị tự thêm: đủ ba thao tác", () => {
    expect(thaoTacTheoTang(TANG_DON_VI)).toEqual({ doiNhan: true, tat: true, xoa: true });
  });

  it("tầng 2 — phần mềm cấp: KHÔNG xoá được, chỉ tắt", () => {
    // `docs/ui-ux/14-cau-hinh.md:182` và ADR 0024 hệ quả #4: mục nguồn hệ thống được TẮT, không
    // được xoá. Xoá cứng còn giải phóng lại một mã đã cấp, thứ luật 7 bất biến 3 cấm.
    expect(thaoTacTheoTang(TANG_HE_THONG)).toEqual({ doiNhan: true, tat: true, xoa: false });
  });

  it("tầng 3 — mã nguồn rẽ nhánh theo mã: KHÔNG tắt, KHÔNG xoá, chỉ đổi nhãn", () => {
    // Tắt một mã mà mã nguồn đang rẽ nhánh theo sẽ để lại một nhánh không còn dòng nào tới được,
    // và màn hình vẫn trông bình thường (`danh_muc_ba_tang.go:86-89`).
    expect(thaoTacTheoTang(TANG_RE_NHANH)).toEqual({ doiNhan: true, tat: false, xoa: false });
  });

  it("ĐÓNG KHI KHÔNG CHẮC: tầng lạ hoặc không có tầng thì không vẽ nút nào", () => {
    // Chiều sai an toàn là thiếu một nút — cán bộ hỏi, người khác trả lời được. Chiều còn lại là
    // mời người ta bấm vào đúng thao tác mà quy tắc ba tầng dựng lên để chặn.
    for (const la of [null, 0, 4, -1, Number.NaN]) {
      expect(thaoTacTheoTang(la)).toEqual({ doiNhan: false, tat: false, xoa: false });
    }
  });
});

describe("mục nào có đường ghi", () => {
  it("mục của năm danh mục có `tier`, `source`, `order` — nhận ra được", () => {
    expect(laMucGhi(muc(TANG_DON_VI))).toBe(true);
  });

  it("mục của hai danh mục chỉ đọc KHÔNG có tầng, nên không có thao tác nào", () => {
    // Hai danh mục ấy không có tuyến ghi nào trong hợp đồng. Phép thử hỏi đúng thứ nó cần biết —
    // "hợp đồng có nói tầng của dòng này không" — chứ không hỏi tên nhóm.
    expect(laMucGhi(mucChiDoc())).toBe(false);
    expect(thaoTacCuaMuc(mucChiDoc())).toEqual({ doiNhan: false, tat: false, xoa: false });
  });
});

describe("bật lại một mục đã tắt", () => {
  it("KHÔNG cùng câu hỏi với tắt: tầng 3 đã tắt vẫn bật lại được", () => {
    // Trigger chỉ từ chối chiều `true -> false`. Giấu nốt nút bật thì một dòng tầng 3 lỡ tắt sẽ
    // không còn đường nào quay lại (`danh_muc_ba_tang.go:114-121`).
    expect(choBatLai(muc(TANG_RE_NHANH, false))).toBe(true);
    expect(choBatLai(muc(TANG_HE_THONG, false))).toBe(true);
    expect(choBatLai(muc(TANG_DON_VI, false))).toBe(true);
  });

  it("mục đang dùng thì không có nút bật lại", () => {
    expect(choBatLai(muc(TANG_DON_VI, true))).toBe(false);
  });
});

describe("lý do xoá — ô bắt buộc", () => {
  it("rỗng thì TỪ CHỐI, kèm câu nói rõ lý do được lưu để làm gì", () => {
    const kq = kiemLyDoXoa("");
    expect(kq.ok).toBe(false);
    if (kq.ok) return;
    expect(kq.loi).toBe(LOI_THIEU_LY_DO_XOA);
  });

  it("CHỈ CÓ DẤU CÁCH cũng là rỗng", () => {
    // Một ô toàn dấu cách qua được phép kiểm `=== ""` ngây thơ, rồi máy chủ cắt trắng và từ chối
    // — tức là màn hình đã nói "đã gửi" cho một thao tác hỏng.
    expect(kiemLyDoXoa("   ").ok).toBe(false);
    expect(kiemLyDoXoa("\t\n ").ok).toBe(false);
  });

  it("có chữ thì nhận, và cắt trắng hai đầu trước khi gửi", () => {
    const kq = kiemLyDoXoa("  Trùng với loại Công văn  ");
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.giaTri).toBe("Trùng với loại Công văn");
  });

  it("câu báo lỗi nói HẬU QUẢ, không chỉ nói 'bắt buộc'", () => {
    expect(LOI_THIEU_LY_DO_XOA).toContain("lý do");
    expect(LOI_THIEU_LY_DO_XOA.toLowerCase()).not.toContain("lỗi");
    // Lý do được lưu cạnh dòng đã xoá và là câu trả lời duy nhất còn lại vào ngày có người hỏi.
    expect(LOI_THIEU_LY_DO_XOA).toMatch(/vì sao|lưu/);
  });
});
