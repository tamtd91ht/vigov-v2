import { describe, expect, it } from "vitest";

import { thanCongKhai } from "@/lib/api/can-bo";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { QUYEN_CONG_KHAI_DANH_BA, quyetDinhTheoKhoa } from "@/lib/quyen";

import {
  CANH_BAO_CONG_KHAI,
  CANH_BAO_RUT,
  CAU_CHUA_XAC_NHAN,
  CAU_THU_TU_SAI,
  O_DA_HOI_Y,
  banCongKhaiTu,
  docThuTu,
  dongYGhiLuc,
  duocCongKhaiTheoPhien,
  guiCongKhaiDuoc,
  yeuCauCongKhai,
  yeuCauRut,
} from "./cong-khai";

/**
 * Công khai một người lên danh bạ Zalo Mini App (#12). Ba điều chịu lực:
 *
 *   1. CHƯA TICK THÌ KHÔNG CÓ YÊU CẦU — và `consent_confirmed` chỉ `true` khi đã tick.
 *   2. `display_order` LUÔN CÓ MẶT trong thân, mang giá trị đang có — PUT thiếu nó là xoá nó.
 *   3. Câu chữ nói trước hậu quả: dữ liệu cá nhân ra công khai; rút là mất dấu đồng ý.
 */

function canBo(ghiDe: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Chuyên viên",
    department_id: "",
    role_id: "",
    phone: "02350000001",
    mobile: "0900000001",
    has_account: false,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...ghiDe,
  };
}

describe("công khai — chưa tick thì không có yêu cầu nào", () => {
  it("mở hộp: ô tick LUÔN trống, kể cả người từng được công khai", () => {
    expect(banCongKhaiTu(canBo()).daHoiY).toBe(false);
    expect(banCongKhaiTu(canBo({ published: true, consent_recorded_at: "2026-09-01T00:00:00Z" })).daHoiY).toBe(false);
  });

  it("mở hộp: thứ tự nạp giá trị đang có, kể cả 0", () => {
    expect(banCongKhaiTu(canBo({ display_order: 0 })).thuTu).toBe("0");
    expect(banCongKhaiTu(canBo({ display_order: 7 })).thuTu).toBe("7");
    expect(banCongKhaiTu(canBo()).thuTu).toBe("");
  });

  it("nút gửi chỉ bấm được khi đã tick", () => {
    expect(guiCongKhaiDuoc({ daHoiY: false, thuTu: "" })).toBe(false);
    expect(guiCongKhaiDuoc({ daHoiY: true, thuTu: "" })).toBe(true);
  });

  it("chưa tick → câu từ chối, KHÔNG có yêu cầu (nên không có thân nào mang consent)", () => {
    expect(yeuCauCongKhai({ daHoiY: false, thuTu: "3" })).toEqual({ loi: CAU_CHUA_XAC_NHAN });
  });

  it("đã tick → consent `true`, published `true`, thứ tự đã đổi thành số", () => {
    const kq = yeuCauCongKhai({ daHoiY: true, thuTu: " 3 " });
    expect(kq).toEqual({ yeuCau: { congKhai: true, daXacNhanDongY: true, thuTu: 3 } });
    if ("yeuCau" in kq) {
      expect(thanCongKhai(kq.yeuCau)).toEqual({
        published: true,
        consent_confirmed: true,
        display_order: 3,
      });
    }
  });

  it("thứ tự để trống → `display_order: null` VẪN CÓ MẶT trong thân", () => {
    const kq = yeuCauCongKhai({ daHoiY: true, thuTu: "" });
    if (!("yeuCau" in kq)) throw new Error("phải có yêu cầu");
    const than = thanCongKhai(kq.yeuCau);
    expect(Object.keys(than).sort()).toEqual(["consent_confirmed", "display_order", "published"]);
    expect(than.display_order).toBeNull();
  });

  it("thứ tự sai → câu từ chối, không gửi", () => {
    for (const sai of ["-1", "1.5", "abc", "1e3", "99999999999999999999"]) {
      expect(yeuCauCongKhai({ daHoiY: true, thuTu: sai })).toEqual({ loi: CAU_THU_TU_SAI });
    }
  });
});

describe("rút — gửi lại thứ tự đang có, không bao giờ xác nhận đồng ý", () => {
  it("thứ tự đang đặt đi lên NGUYÊN VẸN — thiếu nó là máy chủ xoá nó", () => {
    const than = thanCongKhai(yeuCauRut(canBo({ published: true, display_order: 4 })));
    expect(than).toEqual({ published: false, consent_confirmed: false, display_order: 4 });
  });

  it("không có thứ tự thì gửi `null` — vẫn có mặt", () => {
    const than = thanCongKhai(yeuCauRut(canBo({ published: true })));
    expect(than).toHaveProperty("display_order", null);
  });

  it("rút KHÔNG BAO GIỜ gửi `consent_confirmed: true`, kể cả khi bên gọi truyền true", () => {
    expect(
      thanCongKhai({ congKhai: false, daXacNhanDongY: true, thuTu: null }).consent_confirmed,
    ).toBe(false);
  });
});

describe("khoá quyền của hai nút Mini App", () => {
  it("là `content.update`, và `admin.user` một mình KHÔNG mở nó (ca bị từ chối)", () => {
    expect(QUYEN_CONG_KHAI_DANH_BA).toBe("content.update");
    const phien = (permissions: string[]) =>
      ({ ok: true, duLieu: { permissions } }) as unknown as Parameters<typeof quyetDinhTheoKhoa>[0];
    expect(quyetDinhTheoKhoa(phien(["admin.user"]), QUYEN_CONG_KHAI_DANH_BA).hien).toBe(false);
    expect(quyetDinhTheoKhoa(phien(["admin.user", "content.update"]), QUYEN_CONG_KHAI_DANH_BA).hien).toBe(true);
    expect(
      quyetDinhTheoKhoa({ ok: false, thongBao: "Phiên hết hạn." }, QUYEN_CONG_KHAI_DANH_BA).hien,
    ).toBe(false);
  });

  it("`duocCongKhaiTheoPhien` — chỉ vẽ nút khi phiên đã đọc xong VÀ có `content.update`", () => {
    const phien = (permissions: string[]) =>
      ({ ok: true, duLieu: { permissions } }) as unknown as Parameters<typeof duocCongKhaiTheoPhien>[0];
    expect(duocCongKhaiTheoPhien(null)).toBe(false);
    expect(duocCongKhaiTheoPhien({ ok: false, thongBao: "Phiên hết hạn." })).toBe(false);
    expect(duocCongKhaiTheoPhien(phien(["admin.user"]))).toBe(false);
    expect(duocCongKhaiTheoPhien(phien(["content.update"]))).toBe(true);
  });
});

describe("docThuTu", () => {
  it("rỗng → null; số nguyên từ 0 → số", () => {
    expect(docThuTu("")).toEqual({ ok: true, thuTu: null });
    expect(docThuTu("  ")).toEqual({ ok: true, thuTu: null });
    expect(docThuTu("0")).toEqual({ ok: true, thuTu: 0 });
    expect(docThuTu("12")).toEqual({ ok: true, thuTu: 12 });
  });
});

describe("câu chữ nói trước hậu quả", () => {
  it("cảnh báo công khai nói: dữ liệu cá nhân, Nghị định 13, công khai, phải được đồng ý", () => {
    expect(CANH_BAO_CONG_KHAI).toContain("dữ liệu cá nhân");
    expect(CANH_BAO_CONG_KHAI).toContain("Nghị định 13/2023/NĐ-CP");
    expect(CANH_BAO_CONG_KHAI).toContain("công khai");
    expect(CANH_BAO_CONG_KHAI).toContain("đồng ý");
    expect(O_DA_HOI_Y).toContain("Đã hỏi ý và người này đồng ý");
  });

  it("cảnh báo rút nói dấu đồng ý bị xoá và phải hỏi ý lại", () => {
    expect(CANH_BAO_RUT).toContain("Dấu ghi nhận đồng ý sẽ bị xoá");
    expect(CANH_BAO_RUT).toContain("hỏi ý người này một lần nữa");
  });
});

describe("dongYGhiLuc — dd/mm/yyyy hh:mm, giờ Việt Nam", () => {
  it("UTC 07:05 là 14:05 giờ Việt Nam", () => {
    expect(dongYGhiLuc("2026-09-24T07:05:00Z")).toBe("Đồng ý ghi lúc 24/09/2026 14:05");
  });

  it("qua nửa đêm UTC thì sang ngày theo giờ Việt Nam", () => {
    expect(dongYGhiLuc("2026-09-24T18:30:00Z")).toBe("Đồng ý ghi lúc 25/09/2026 01:30");
  });

  it("null → null; chuỗi hỏng hiện nguyên văn", () => {
    expect(dongYGhiLuc(null)).toBeNull();
    expect(dongYGhiLuc("khong-phai-ngay")).toBe("Đồng ý ghi lúc khong-phai-ngay");
  });
});
