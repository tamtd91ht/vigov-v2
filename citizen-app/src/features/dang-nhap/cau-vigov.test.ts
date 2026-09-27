import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { sangKieuCongDan } from "../../App";

import { moPhienCongDanQuaCau } from "./cau-vigov";
import { moPhienViGovQuaCau, phatHanhPhien } from "./goi-may-chu";
import {
  docTraLoi,
  docTraLoiCauViGov,
  thanYeuCau,
  thanYeuCauCauViGov,
  TRUONG_GUI_DI_CAU_VIGOV,
  TRUONG_GUI_DI_PHIEN,
} from "./hop-dong";

/**
 * NHÁNH CẦU PHIÊN ViGov CỦA `POST /api/v1/sessions` — phía Mini App.
 *
 * Máy chủ (`vihat-miniapp` `internal/httpapi/sessions_vigov.go:158-175`) trả `{ vigovSession: {...} }`
 * khi cầu BẬT; hôm nay cầu TẮT và nhánh cũ trả 400 cho thân không có `phoneToken`. Cả hai đường phải
 * chạy: bật thì có phiên, tắt thì không bao giờ giả vờ có.
 */

const DIA_CHI = "https://mini.vidu.vn/api/v1/sessions";
const TEN_MIEN = "xa-vi-du.vigov.example";

type LoiGoi = { dia_chi: string; tuy_chon: RequestInit };
let loi_goi: LoiGoi[] = [];

function traLoi(status: number, than: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => than };
}

function datFetch(p: ReturnType<typeof traLoi> | Error) {
  vi.stubGlobal("fetch", (dia_chi: string, tuy_chon: RequestInit) => {
    loi_goi.push({ dia_chi, tuy_chon });
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const PHIEN_VIGOV = {
  vigovSession: {
    token: "tok-vigov-thu",
    expiresAt: "2026-09-27T10:00:00Z",
    tenantDisplayName: "Xã Của Phiên",
    phoneVerified: false,
  },
};

beforeEach(() => {
  loi_goi = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("thân gửi đi — ba trường, khoá hai chiều với bảng khai", () => {
  it("accessToken · communeHostHint · communeConfirmed=true — KHÔNG phoneToken", () => {
    const than = JSON.parse(thanYeuCauCauViGov({ ma_truy_cap: "ma-zalo", ten_mien_xa: TEN_MIEN })) as Record<
      string,
      unknown
    >;
    expect(than).toEqual({ accessToken: "ma-zalo", communeHostHint: TEN_MIEN, communeConfirmed: true });
    expect(Object.keys(than).sort()).toEqual(TRUONG_GUI_DI_CAU_VIGOV.map((t) => t.khoa).sort());
  });

  it("thân của khối đăng nhập KHÔNG đổi (Tư vấn · Yêu cầu của tôi giữ nguyên hành vi)", () => {
    const than = JSON.parse(thanYeuCau({ ma_so_dien_thoai: "x", ma_truy_cap: "y" })) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual(TRUONG_GUI_DI_PHIEN.map((t) => t.khoa).sort());
  });
});

describe("đọc thân 201 — hai phiên, hai khoá, không bao giờ lẫn", () => {
  it("`vigovSession` đủ → phiên ViGov", () => {
    expect(docTraLoiCauViGov(PHIEN_VIGOV)).toEqual({
      token: "tok-vigov-thu",
      het_han: "2026-09-27T10:00:00Z",
      ten_xa: "Xã Của Phiên",
      da_xac_thuc_so: false,
      ten_mien_xa: null,
    });
  });

  /**
   * `communePrimaryHost` — TUỲ CHỌN. Có và đúng khuôn → khoá tra của hai màn công khai. Vắng, `""`
   * (phiên không xã / xã chưa có tên miền chính), sai kiểu hay sai khuôn → `null`, và PHIÊN VẪN CÒN:
   * mất tên miền chỉ mất tin tức và danh bạ, không được mất việc gửi phản ánh.
   */
  it("`communePrimaryHost` có và đúng khuôn → `ten_mien_xa`, chữ thường", () => {
    const voi = (v: unknown) => docTraLoiCauViGov({ vigovSession: { ...PHIEN_VIGOV.vigovSession, communePrimaryHost: v } });
    expect(voi(TEN_MIEN)).toMatchObject({ token: "tok-vigov-thu", ten_mien_xa: TEN_MIEN });
    expect(voi("Xa-Vi-Du.Vigov.Example")).toMatchObject({ ten_mien_xa: TEN_MIEN });
  });

  it("`communePrimaryHost` vắng · rỗng · sai kiểu · sai khuôn → `null`, phiên vẫn dùng được", () => {
    const voi = (v: unknown) => docTraLoiCauViGov({ vigovSession: { ...PHIEN_VIGOV.vigovSession, communePrimaryHost: v } });
    for (const v of [undefined, "", 7, null, true, "localhost", "https://xa.vigov.vn", "xa.vn/duong", "xa.vn:443", "a b.vn"]) {
      const kq = voi(v);
      expect(kq, JSON.stringify(v)).toMatchObject({ token: "tok-vigov-thu", ten_xa: "Xã Của Phiên", ten_mien_xa: null });
    }
  });

  it("KHÔNG đọc nhầm một tên trường khác làm tên miền (`communeHost`, `communeHostHint`)", () => {
    // Tên trường đã chốt là `communePrimaryHost` (27/09/2026). Một tên gần giống là một máy chủ khác
    // hợp đồng, và đoán theo nó là để hai màn công khai mở cho một xã không ai xác nhận.
    const kq = docTraLoiCauViGov({
      vigovSession: { ...PHIEN_VIGOV.vigovSession, communeHost: TEN_MIEN, communeHostHint: TEN_MIEN },
    });
    expect(kq).toMatchObject({ ten_mien_xa: null });
  });

  it("thân của phiếu THƯƠNG MẠI (`token` ở gốc) KHÔNG BAO GIỜ thành phiên ViGov", () => {
    expect(docTraLoiCauViGov({ token: "tok-thuong-mai", expiresAt: "2026-09-27T10:00:00Z" })).toBe("khong-co-phien");
  });

  it("phiên không xã (không `token`) · tên xã rỗng → không có phiên", () => {
    expect(docTraLoiCauViGov({ vigovSession: { tenantDisplayName: "", phoneVerified: false } })).toBe("khong-co-phien");
    expect(
      docTraLoiCauViGov({ vigovSession: { token: "t", tenantDisplayName: " ", phoneVerified: false } }),
    ).toBe("khong-co-phien");
  });

  it("sai kiểu → `null`", () => {
    expect(docTraLoiCauViGov(null)).toBeNull();
    expect(docTraLoiCauViGov({ vigovSession: "x" })).toBeNull();
    expect(docTraLoiCauViGov({ vigovSession: { token: 1, tenantDisplayName: "a", phoneVerified: false } })).toBeNull();
    expect(docTraLoiCauViGov({ vigovSession: { token: "t", tenantDisplayName: "a" } })).toBeNull();
  });

  it("khối đăng nhập đọc thân cầu thành 'không đọc được' — không cầm nhầm bearer của ViGov", () => {
    expect(docTraLoi(PHIEN_VIGOV)).toBeNull();
  });
});

describe("lời gọi cầu — mỗi mã trạng thái một nhánh", () => {
  it("POST đúng tuyến, JSON, không Authorization", async () => {
    datFetch(traLoi(201, PHIEN_VIGOV));
    const kq = await moPhienViGovQuaCau({ ma_truy_cap: "ma-zalo", ten_mien_xa: TEN_MIEN }, DIA_CHI);
    expect(kq.kieu).toBe("xong");
    expect(loi_goi[0]!.dia_chi).toBe(DIA_CHI);
    expect(loi_goi[0]!.tuy_chon.method).toBe("POST");
    expect(loi_goi[0]!.tuy_chon.headers).toEqual({ "Content-Type": "application/json" });
  });

  it("cầu TẮT hôm nay: 400 của nhánh cũ → `cau-tat`, không phải lỗi", async () => {
    datFetch(traLoi(400, { error: "yeu cau hong" }));
    expect(await moPhienViGovQuaCau({ ma_truy_cap: "m", ten_mien_xa: TEN_MIEN }, DIA_CHI)).toEqual({ kieu: "cau-tat" });
  });

  it("các nhánh còn lại", async () => {
    const CA: Array<[ReturnType<typeof traLoi> | Error, string]> = [
      [traLoi(201, { token: "tok-thuong-mai", expiresAt: "x" }), "cau-tat"],
      [traLoi(201, { vigovSession: { tenantDisplayName: "", phoneVerified: false } }), "cau-tat"],
      [traLoi(201, { vigovSession: 3 }), "khong-goi-duoc"],
      [traLoi(401, {}), "ma-het-han"],
      [traLoi(422, {}), "chua-san-sang"],
      [traLoi(429, {}), "tam-ngung"],
      [traLoi(502, {}), "tam-ngung"],
      [traLoi(503, {}), "tam-ngung"],
      [traLoi(500, {}), "khong-goi-duoc"],
      [new Error("mat mang"), "khong-goi-duoc"],
    ];
    for (const [p, kieu] of CA) {
      datFetch(p);
      expect((await moPhienViGovQuaCau({ ma_truy_cap: "m", ten_mien_xa: TEN_MIEN }, DIA_CHI)).kieu).toBe(kieu);
    }
  });

  it("không có địa chỉ → không gọi", async () => {
    const f = vi.fn();
    vi.stubGlobal("fetch", f);
    expect(await moPhienViGovQuaCau({ ma_truy_cap: "m", ten_mien_xa: TEN_MIEN }, "")).toEqual({ kieu: "chua-khai-host" });
    expect(f).not.toHaveBeenCalled();
  });

  it("`phatHanhPhien` của khối đăng nhập vẫn đọc phiếu thương mại như trước", async () => {
    datFetch(traLoi(201, { token: "tok-thuong-mai", expiresAt: "2026-09-27T10:00:00Z" }));
    expect(await phatHanhPhien({ ma_so_dien_thoai: "p", ma_truy_cap: "a" }, DIA_CHI)).toEqual({
      kieu: "xong",
      phien: { token: "tok-thuong-mai", het_han: "2026-09-27T10:00:00Z" },
    });
  });
});

describe("ghép: lấy mã Zalo rồi gọi cầu", () => {
  it("chuyền đúng mã và tên miền; KHÔNG xin số điện thoại", async () => {
    const goi = vi.fn(async () => ({ kieu: "cau-tat" as const }));
    const kq = await moPhienCongDanQuaCau(TEN_MIEN, async () => ({ kieu: "xong", du_lieu: "ma-zalo" }), goi);
    expect(kq).toEqual({ kieu: "cau-tat" });
    expect(goi).toHaveBeenCalledWith({ ma_truy_cap: "ma-zalo", ten_mien_xa: TEN_MIEN });
  });

  it("ngoài Zalo / không lấy được mã / mã rỗng → không gọi cầu", async () => {
    const goi = vi.fn();
    expect(await moPhienCongDanQuaCau(TEN_MIEN, async () => ({ kieu: "ngoai-zalo" }), goi)).toEqual({ kieu: "ngoai-zalo" });
    expect(await moPhienCongDanQuaCau(TEN_MIEN, async () => ({ kieu: "khong-lay-duoc" }), goi)).toEqual({
      kieu: "khong-lay-duoc-ma",
    });
    expect(await moPhienCongDanQuaCau(TEN_MIEN, async () => ({ kieu: "xong", du_lieu: "" }), goi)).toEqual({
      kieu: "khong-lay-duoc-ma",
    });
    expect(goi).not.toHaveBeenCalled();
  });
});

describe("App.tsx dịch kết quả sang kiểu của nửa nhà nước — theo việc người dân làm tiếp", () => {
  it("xong → bearer + tên xã của PHIÊN + tên miền chính của phiên (hoặc `null`)", () => {
    expect(
      sangKieuCongDan({
        kieu: "xong",
        phien: { token: "t", het_han: "", ten_xa: "Xã Của Phiên", da_xac_thuc_so: false, ten_mien_xa: TEN_MIEN },
      }),
    ).toEqual({ kieu: "xong", token: "t", ten_xa: "Xã Của Phiên", ten_mien: TEN_MIEN });
    expect(
      sangKieuCongDan({
        kieu: "xong",
        phien: { token: "t", het_han: "", ten_xa: "Xã Của Phiên", da_xac_thuc_so: false, ten_mien_xa: null },
      }),
    ).toEqual({ kieu: "xong", token: "t", ten_xa: "Xã Của Phiên", ten_mien: null });
  });

  it("thân 201 thật → `sangKieuCongDan`: `communePrimaryHost` đi suốt tới kiểu của nửa nhà nước", async () => {
    datFetch(traLoi(201, { vigovSession: { ...PHIEN_VIGOV.vigovSession, communePrimaryHost: TEN_MIEN } }));
    const kq = sangKieuCongDan(await moPhienViGovQuaCau({ ma_truy_cap: "m", ten_mien_xa: TEN_MIEN }, DIA_CHI));
    expect(kq).toEqual({ kieu: "xong", token: "tok-vigov-thu", ten_xa: "Xã Của Phiên", ten_mien: TEN_MIEN });
  });

  it("bấm lại không đổi được gì → `chua-mo`; bấm lại có thể được → `thu-lai`", () => {
    for (const kieu of ["cau-tat", "chua-san-sang", "chua-khai-host"] as const) {
      expect(sangKieuCongDan({ kieu }), kieu).toEqual({ kieu: "chua-mo" });
    }
    for (const kieu of ["ma-het-han", "tam-ngung", "khong-goi-duoc", "khong-lay-duoc-ma"] as const) {
      expect(sangKieuCongDan({ kieu }), kieu).toEqual({ kieu: "thu-lai" });
    }
    expect(sangKieuCongDan({ kieu: "ngoai-zalo" })).toEqual({ kieu: "ngoai-zalo" });
  });
});
