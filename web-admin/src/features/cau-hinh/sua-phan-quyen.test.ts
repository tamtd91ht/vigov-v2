import { describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { identity_cotPhanQuyenRa } from "@/lib/api/schema.gen";

import { dungBangDaCap } from "./ma-tran-quyen";
import { coTheSuaPhanQuyen, maVaiTroCuaToi } from "./quyen-tab";
import {
  LOI_SAI_VAI_TRO,
  banSuaMoi,
  batDauLuu,
  batTatO,
  bangHienThi,
  cotDaSua,
  guiCot,
  huyCot,
  ketThucLuu,
  tapHienTai,
  thanLuuCot,
  type BanSua,
  type HamLuuCot,
} from "./sua-phan-quyen";

/**
 * Lưu THEO TỪNG CỘT (§12.5). Ca chịu lực nhất là "chỉ cột bấm Lưu được gửi": thân của `PUT` là
 * TOÀN BỘ tập quyền của một vai trò, nên gửi nhầm tập — hay hợp của mọi cột — là cấp lặng lẽ
 * những quyền không ai tick, và máy chủ lưu đúng thứ nó nhận.
 */

const GOC = dungBangDaCap([
  { role_id: "vt-1", permission: "task.read" },
  { role_id: "vt-1", permission: "document.read" },
  { role_id: "vt-2", permission: "admin.user" },
]);

function moi(): BanSua {
  return banSuaMoi(GOC);
}

function tap(b: BanSua, id: string): string[] {
  return [...tapHienTai(b, id)].sort();
}

describe("bật/tắt ô và trạng thái đã sửa của từng cột", () => {
  it("bật một ô thì chỉ cột ấy thành đã sửa, cột kia vẫn sạch", () => {
    const b = batTatO(moi(), "vt-1", "admin.role");
    expect(cotDaSua(b, "vt-1")).toBe(true);
    expect(cotDaSua(b, "vt-2")).toBe(false);
    expect(tap(b, "vt-1")).toEqual(["admin.role", "document.read", "task.read"]);
  });

  it("bật rồi tắt lại đúng ô ấy thì cột trở về SẠCH — không còn gì để lưu", () => {
    const b = batTatO(batTatO(moi(), "vt-1", "admin.role"), "vt-1", "admin.role");
    expect(cotDaSua(b, "vt-1")).toBe(false);
  });

  it("Huỷ đưa cột về bản máy chủ và bỏ câu lỗi của cột; cột khác không bị động tới", () => {
    let b = batTatO(moi(), "vt-1", "task.read");
    b = batTatO(b, "vt-2", "task.read");
    b = ketThucLuu(batDauLuu(b, "vt-1"), "vt-1", { ok: false, thongBao: "Không tìm thấy vai trò." });

    const h = huyCot(b, "vt-1");
    expect(cotDaSua(h, "vt-1")).toBe(false);
    expect(tap(h, "vt-1")).toEqual(["document.read", "task.read"]);
    expect(h.loi.has("vt-1")).toBe(false);
    expect(cotDaSua(h, "vt-2")).toBe(true);
  });

  it("cột đang lưu thì không bật/tắt được — phản hồi sắp về sẽ thay cả cột", () => {
    const b = batDauLuu(batTatO(moi(), "vt-1", "admin.role"), "vt-1");
    expect(batTatO(b, "vt-1", "task.read")).toBe(b);
  });

  it("bảng hiện ra là bản máy chủ đè bởi cột đang sửa", () => {
    const b = batTatO(moi(), "vt-2", "admin.user");
    const hien = bangHienThi(b);
    expect(hien.get("vt-2")?.has("admin.user")).toBe(false);
    expect(hien.get("vt-1")?.has("task.read")).toBe(true);
  });
});

describe("thân của PUT — toàn bộ tập của MỘT cột, sắp xếp, không trùng", () => {
  it("sắp theo chữ và là toàn bộ tập, không phải phần chênh lệch", () => {
    const b = batTatO(moi(), "vt-1", "admin.role");
    expect(thanLuuCot(b, "vt-1")).toEqual({
      permissions: ["admin.role", "document.read", "task.read"],
    });
  });

  it("dữ liệu đọc về có trùng thì thân vẫn không trùng", () => {
    const b = banSuaMoi(
      dungBangDaCap([
        { role_id: "vt-1", permission: "task.read" },
        { role_id: "vt-1", permission: "task.read" },
      ]),
    );
    expect(thanLuuCot(batTatO(b, "vt-1", "admin.role"), "vt-1")).toEqual({
      permissions: ["admin.role", "task.read"],
    });
  });

  it("gỡ hết thì thân là `[]`, không phải thiếu trường", () => {
    let b = batTatO(moi(), "vt-2", "admin.user");
    expect(thanLuuCot(b, "vt-2")).toEqual({ permissions: [] });
    b = batTatO(b, "vt-2", "admin.user");
    expect(cotDaSua(b, "vt-2")).toBe(false);
  });
});

describe("guiCot — một lời gọi, một vai trò, một thân", () => {
  it("HAI cột đang sửa, bấm Lưu một cột: đúng MỘT lời gọi, đúng vai trò ấy, đúng tập của nó", async () => {
    let b = batTatO(moi(), "vt-1", "admin.role");
    b = batTatO(b, "vt-2", "budget.read");

    const luu = vi.fn<HamLuuCot>(async (id) => ({
      ok: true,
      duLieu: { role_id: id, permissions: [] },
    }));
    await guiCot(b, "vt-1", luu);

    expect(luu).toHaveBeenCalledTimes(1);
    expect(luu).toHaveBeenCalledWith("vt-1", {
      permissions: ["admin.role", "document.read", "task.read"],
    });
    // Không khoá nào của cột kia lọt vào thân — vế phủ định là vế bắt được phép gộp cả ma trận.
    const than = luu.mock.calls[0]?.[1];
    expect(than?.permissions).not.toContain("budget.read");
    expect(than?.permissions).not.toContain("admin.user");
  });
});

describe("kết thúc lưu", () => {
  it("200: cột thay bằng tập MÁY CHỦ TRẢ, không bằng tập đã gửi; cột kia vẫn giữ phần đang sửa", () => {
    let b = batTatO(moi(), "vt-1", "admin.role");
    b = batTatO(b, "vt-2", "budget.read");
    b = batDauLuu(b, "vt-1");

    const kq: KetQua<identity_cotPhanQuyenRa> = {
      ok: true,
      duLieu: { role_id: "vt-1", permissions: ["admin.role", "task.read"] },
    };
    const sau = ketThucLuu(b, "vt-1", kq);

    expect(cotDaSua(sau, "vt-1")).toBe(false);
    expect(tap(sau, "vt-1")).toEqual(["admin.role", "task.read"]);
    expect(sau.dangLuu.has("vt-1")).toBe(false);
    expect(cotDaSua(sau, "vt-2")).toBe(true);
    expect(tap(sau, "vt-2")).toEqual(["admin.user", "budget.read"]);
  });

  const TU_CHOI = [
    "Khoá quyền sai dạng.",
    "Quyền không có trong danh mục của hệ thống: x.khong-co. Hãy tải lại ma trận phân quyền.",
    "Không lưu được phân quyền cho vai trò mà chính bạn đang giữ. Hãy nhờ một người quản trị khác của xã thực hiện.",
    "Bạn không thêm hay bỏ được quyền mà tài khoản của bạn không có: budget.confirm. Hãy nhờ người có đủ quyền thực hiện.",
    "Không tìm thấy vai trò.",
    "Xã phải luôn còn ít nhất một cán bộ đang hoạt động giữ quyền admin.user. Hãy cấp quyền này cho một vai trò khác có cán bộ đang hoạt động trước, rồi lưu lại.",
  ];

  for (const cau of TU_CHOI) {
    it(`từ chối "${cau.slice(0, 40)}…": câu nguyên văn gắn vào cột, phần đã tick còn nguyên`, () => {
      const b = batDauLuu(batTatO(moi(), "vt-2", "admin.user"), "vt-2");
      const sau = ketThucLuu(b, "vt-2", { ok: false, thongBao: cau });

      expect(sau.loi.get("vt-2")).toBe(cau);
      expect(cotDaSua(sau, "vt-2")).toBe(true);
      expect(tap(sau, "vt-2")).toEqual([]);
      expect(sau.dangLuu.has("vt-2")).toBe(false);
      // Bản máy chủ không đổi — lần ghi đã bị từ chối.
      expect([...(sau.goc.get("vt-2") ?? [])]).toEqual(["admin.user"]);
    });
  }

  it("lần lưu mới bắt đầu thì câu lỗi cũ của cột được bỏ", () => {
    const b = ketThucLuu(batDauLuu(batTatO(moi(), "vt-1", "x.y"), "vt-1"), "vt-1", {
      ok: false,
      thongBao: "Không tìm thấy vai trò.",
    });
    expect(batDauLuu(b, "vt-1").loi.has("vt-1")).toBe(false);
  });

  it("200 cho một vai trò KHÁC thì không áp vào cột nào", () => {
    const b = batDauLuu(batTatO(moi(), "vt-1", "admin.role"), "vt-1");
    const sau = ketThucLuu(b, "vt-1", {
      ok: true,
      duLieu: { role_id: "vt-2", permissions: ["admin.role"] },
    });
    expect(sau.loi.get("vt-1")).toBe(LOI_SAI_VAI_TRO);
    expect(cotDaSua(sau, "vt-1")).toBe(true);
    expect([...(sau.goc.get("vt-2") ?? [])]).toEqual(["admin.user"]);
  });
});

/**
 * HAI CỘT CÙNG LÚC. Mỗi ca ở trên chỉ có MỘT cột đang lưu, nên một `ketThucLuu` viết `new Set()` /
 * `new Map()` thay vì bỏ đúng một khoá vẫn xanh. Trên màn hình, cán bộ bấm Lưu cột A rồi cột B trước
 * khi A trả lời là chuyện bình thường; phản hồi của A mà xoá trạng thái của B thì:
 *   - B hết "đang lưu" → ô của B bấm được giữa lúc lần ghi của B còn trên dây, rồi phản hồi của B
 *     về đè mất những ô vừa bấm;
 *   - câu từ chối của B biến mất → cán bộ tưởng B đã lưu, trong khi máy chủ đã từ chối nó.
 */
describe("phản hồi của một cột không động tới trạng thái của cột khác", () => {
  function haiCotDangLuu(): BanSua {
    let b = batTatO(moi(), "vt-1", "admin.role");
    b = batTatO(b, "vt-2", "budget.read");
    return batDauLuu(batDauLuu(b, "vt-1"), "vt-2");
  }

  it("A trả 200 trong lúc B còn trên dây: B vẫn đang lưu, vẫn khoá, phần sửa của B còn nguyên", () => {
    const sau = ketThucLuu(haiCotDangLuu(), "vt-1", {
      ok: true,
      duLieu: { role_id: "vt-1", permissions: ["admin.role"] },
    });
    expect(sau.dangLuu.has("vt-1")).toBe(false);
    expect(sau.dangLuu.has("vt-2")).toBe(true);
    // Ô của B vẫn không bấm được — phản hồi của B sắp về sẽ thay cả cột.
    expect(batTatO(sau, "vt-2", "task.read")).toBe(sau);
    expect(tap(sau, "vt-2")).toEqual(["admin.user", "budget.read"]);
  });

  it("A bị từ chối trong lúc B còn trên dây: B vẫn đang lưu, bản máy chủ của B không đổi", () => {
    const sau = ketThucLuu(haiCotDangLuu(), "vt-1", { ok: false, thongBao: "Không tìm thấy vai trò." });
    expect(sau.dangLuu.has("vt-2")).toBe(true);
    expect([...(sau.goc.get("vt-2") ?? [])]).toEqual(["admin.user"]);
    expect(sau.loi.has("vt-2")).toBe(false);
  });

  const TU_CHOI_B =
    "Bạn không thêm hay bỏ được quyền mà tài khoản của bạn không có: budget.read. Hãy nhờ người có đủ quyền thực hiện.";

  function bDaBiTuChoi(): BanSua {
    const b = ketThucLuu(haiCotDangLuu(), "vt-2", { ok: false, thongBao: TU_CHOI_B });
    expect(b.loi.get("vt-2")).toBe(TU_CHOI_B);
    return b;
  }

  it("B đã bị từ chối, rồi A trả 200: câu từ chối của B VẪN hiện", () => {
    const sau = ketThucLuu(bDaBiTuChoi(), "vt-1", {
      ok: true,
      duLieu: { role_id: "vt-1", permissions: ["admin.role"] },
    });
    expect(sau.loi.get("vt-2")).toBe(TU_CHOI_B);
    expect(cotDaSua(sau, "vt-2")).toBe(true);
  });

  it("B đã bị từ chối, rồi A cũng bị từ chối: HAI câu, mỗi câu dưới cột của nó", () => {
    const sau = ketThucLuu(bDaBiTuChoi(), "vt-1", { ok: false, thongBao: "Không tìm thấy vai trò." });
    expect(sau.loi.get("vt-1")).toBe("Không tìm thấy vai trò.");
    expect(sau.loi.get("vt-2")).toBe(TU_CHOI_B);
  });

  it("B đã bị từ chối, rồi A trả 200 cho NHẦM vai trò: câu của B vẫn còn", () => {
    const sau = ketThucLuu(bDaBiTuChoi(), "vt-1", {
      ok: true,
      duLieu: { role_id: "vt-9", permissions: [] },
    });
    expect(sau.loi.get("vt-1")).toBe(LOI_SAI_VAI_TRO);
    expect(sau.loi.get("vt-2")).toBe(TU_CHOI_B);
  });

  it("Huỷ hay bấm Lưu lại cột A không xoá câu từ chối của cột B", () => {
    let b = ketThucLuu(bDaBiTuChoi(), "vt-1", { ok: false, thongBao: "Không tìm thấy vai trò." });
    expect(huyCot(b, "vt-1").loi.get("vt-2")).toBe(TU_CHOI_B);
    b = batDauLuu(b, "vt-1");
    expect(b.loi.get("vt-2")).toBe(TU_CHOI_B);
    expect(b.loi.has("vt-1")).toBe(false);
  });
});

describe("tiện dụng từ phiên: ai được thấy ô bấm, cột nào là của chính mình", () => {
  const phien = (quyen: string[], vaiTro: string | null) => ({
    ok: true as const,
    duLieu: {
      sid: "s",
      expires_at: "2026-09-24T12:00:00Z",
      staff: { code: "CB-00001", full_name: "Cán bộ", position: "Chuyên viên" },
      role: vaiTro === null ? null : { code: vaiTro, name: "X", is_leader: false },
      permissions: quyen,
      must_change_password: false,
    },
  });

  it("có `admin.role` thì sửa được", () => {
    expect(coTheSuaPhanQuyen(phien(["admin.role"], null))).toBe(true);
  });

  it("KHÔNG có `admin.role` thì không — kể cả khi có `admin.user`", () => {
    expect(coTheSuaPhanQuyen(phien(["admin.user", "admin.audit"], null))).toBe(false);
  });

  it("không đọc được phiên thì không — đóng khi không chắc", () => {
    expect(coTheSuaPhanQuyen({ ok: false, thongBao: "Phiên làm việc đã hết hạn." })).toBe(false);
  });

  it("mã vai trò của mình lấy từ phiên; không có vai trò hoặc không đọc được thì `null`", () => {
    expect(maVaiTroCuaToi(phien([], "chuyen-vien"))).toBe("chuyen-vien");
    expect(maVaiTroCuaToi(phien([], null))).toBeNull();
    expect(maVaiTroCuaToi({ ok: false, thongBao: "x" })).toBeNull();
  });
});
