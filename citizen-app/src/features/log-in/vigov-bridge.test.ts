import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toCitizenShape, toReopenWithPhoneResult } from "../../App";

import { openCitizenSessionViaBridge, reopenCitizenSessionWithPhone } from "./vigov-bridge";
import { openVigovSessionViaBridge, issueSession, reopenViGovSessionWithPhone } from "./server-calls";
import {
  BRIDGE_FIELDS_WITH_PHONE,
  bridgeBodyWithPhone,
  readResponse,
  readVigovBridgeResponse,
  sessionRequestBody,
  vigovBridgeRequestBody,
  VIGOV_BRIDGE_SENT_FIELDS,
  SESSION_SENT_FIELDS,
} from "./contract";

/**
 * NHÁNH CẦU PHIÊN ViGov CỦA `POST /api/v1/sessions` — phía Mini App.
 *
 * Máy chủ (`vihat-miniapp` `internal/httpapi/sessions_vigov.go:158-175`) trả `{ vigovSession: {...} }`
 * khi cầu BẬT; hôm nay cầu TẮT và nhánh cũ trả 400 cho thân không có `phoneToken`. Cả hai đường phải
 * chạy: bật thì có phiên, tắt thì không bao giờ giả vờ có.
 */

const ADDRESS = "https://mini.vidu.vn/api/v1/sessions";
const DOMAIN = "xa-vi-du.vigov.example";

type Call = { address: string; options: RequestInit };
let calls: Call[] = [];

function respond(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function stubFetch(p: ReturnType<typeof respond> | Error) {
  vi.stubGlobal("fetch", (address: string, options: RequestInit) => {
    calls.push({ address, options });
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const VIGOV_SESSION = {
  vigovSession: {
    token: "tok-vigov-thu",
    expiresAt: "2026-09-27T10:00:00Z",
    tenantDisplayName: "Xã Của Phiên",
    phoneVerified: false,
  },
};

beforeEach(() => {
  calls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("thân gửi đi — ba trường, khoá hai chiều với bảng khai", () => {
  it("accessToken · communeHostHint · communeConfirmed=true — KHÔNG phoneToken", () => {
    const body = JSON.parse(vigovBridgeRequestBody({ ma_truy_cap: "ma-zalo", commune_domain: DOMAIN })) as Record<
      string,
      unknown
    >;
    expect(body).toEqual({ accessToken: "ma-zalo", communeHostHint: DOMAIN, communeConfirmed: true });
    expect(Object.keys(body).sort()).toEqual(VIGOV_BRIDGE_SENT_FIELDS.map((t) => t.khoa).sort());
  });

  it("thân của khối đăng nhập KHÔNG đổi (Tư vấn · Yêu cầu của tôi giữ nguyên hành vi)", () => {
    const body = JSON.parse(sessionRequestBody({ ma_so_dien_thoai: "x", ma_truy_cap: "y" })) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual(SESSION_SENT_FIELDS.map((t) => t.khoa).sort());
  });
});

describe("đọc thân 201 — hai phiên, hai khoá, không bao giờ lẫn", () => {
  it("`vigovSession` đủ → phiên ViGov", () => {
    expect(readVigovBridgeResponse(VIGOV_SESSION)).toEqual({
      token: "tok-vigov-thu",
      expires_at: "2026-09-27T10:00:00Z",
      commune_name: "Xã Của Phiên",
      phone_verified: false,
      commune_domain: null,
    });
  });

  /**
   * `communePrimaryHost` — TUỲ CHỌN. Có và đúng khuôn → khoá tra của hai màn công khai. Vắng, `""`
   * (phiên không xã / xã chưa có tên miền chính), sai kiểu hay sai khuôn → `null`, và PHIÊN VẪN CÒN:
   * mất tên miền chỉ mất tin tức và danh bạ, không được mất việc gửi phản ánh.
   */
  it("`communePrimaryHost` có và đúng khuôn → `commune_domain`, chữ thường", () => {
    const readWith = (v: unknown) => readVigovBridgeResponse({ vigovSession: { ...VIGOV_SESSION.vigovSession, communePrimaryHost: v } });
    expect(readWith(DOMAIN)).toMatchObject({ token: "tok-vigov-thu", commune_domain: DOMAIN });
    expect(readWith("Xa-Vi-Du.Vigov.Example")).toMatchObject({ commune_domain: DOMAIN });
  });

  it("`communePrimaryHost` vắng · rỗng · sai kiểu · sai khuôn → `null`, phiên vẫn dùng được", () => {
    const readWith = (v: unknown) => readVigovBridgeResponse({ vigovSession: { ...VIGOV_SESSION.vigovSession, communePrimaryHost: v } });
    for (const v of [undefined, "", 7, null, true, "localhost", "https://xa.vigov.vn", "xa.vn/duong", "xa.vn:443", "a b.vn"]) {
      const result = readWith(v);
      expect(result, JSON.stringify(v)).toMatchObject({ token: "tok-vigov-thu", commune_name: "Xã Của Phiên", commune_domain: null });
    }
  });

  it("KHÔNG đọc nhầm một tên trường khác làm tên miền (`communeHost`, `communeHostHint`)", () => {
    // Tên trường đã chốt là `communePrimaryHost` (27/09/2026). Một tên gần giống là một máy chủ khác
    // hợp đồng, và đoán theo nó là để hai màn công khai mở cho một xã không ai xác nhận.
    const result = readVigovBridgeResponse({
      vigovSession: { ...VIGOV_SESSION.vigovSession, communeHost: DOMAIN, communeHostHint: DOMAIN },
    });
    expect(result).toMatchObject({ commune_domain: null });
  });

  it("thân của phiếu THƯƠNG MẠI (`token` ở gốc) KHÔNG BAO GIỜ thành phiên ViGov", () => {
    expect(readVigovBridgeResponse({ token: "tok-thuong-mai", expiresAt: "2026-09-27T10:00:00Z" })).toBe("khong-co-phien");
  });

  it("phiên không xã (không `token`) · tên xã rỗng → không có phiên", () => {
    expect(readVigovBridgeResponse({ vigovSession: { tenantDisplayName: "", phoneVerified: false } })).toBe("khong-co-phien");
    expect(
      readVigovBridgeResponse({ vigovSession: { token: "t", tenantDisplayName: " ", phoneVerified: false } }),
    ).toBe("khong-co-phien");
  });

  it("sai kiểu → `null`", () => {
    expect(readVigovBridgeResponse(null)).toBeNull();
    expect(readVigovBridgeResponse({ vigovSession: "x" })).toBeNull();
    expect(readVigovBridgeResponse({ vigovSession: { token: 1, tenantDisplayName: "a", phoneVerified: false } })).toBeNull();
    expect(readVigovBridgeResponse({ vigovSession: { token: "t", tenantDisplayName: "a" } })).toBeNull();
  });

  it("khối đăng nhập đọc thân cầu thành 'không đọc được' — không cầm nhầm bearer của ViGov", () => {
    expect(readResponse(VIGOV_SESSION)).toBeNull();
  });
});

describe("lời gọi cầu — mỗi mã trạng thái một nhánh", () => {
  it("POST đúng tuyến, JSON, không Authorization", async () => {
    stubFetch(respond(201, VIGOV_SESSION));
    const result = await openVigovSessionViaBridge({ ma_truy_cap: "ma-zalo", commune_domain: DOMAIN }, ADDRESS);
    expect(result.kind).toBe("xong");
    expect(calls[0]!.address).toBe(ADDRESS);
    expect(calls[0]!.options.method).toBe("POST");
    expect(calls[0]!.options.headers).toEqual({ "Content-Type": "application/json" });
  });

  it("cầu TẮT hôm nay: 400 của nhánh cũ → `cau-tat`, không phải lỗi", async () => {
    stubFetch(respond(400, { error: "yeu cau hong" }));
    expect(await openVigovSessionViaBridge({ ma_truy_cap: "m", commune_domain: DOMAIN }, ADDRESS)).toEqual({ kind: "cau-tat" });
  });

  it("các nhánh còn lại", async () => {
    const CASES: Array<[ReturnType<typeof respond> | Error, string]> = [
      [respond(201, { token: "tok-thuong-mai", expiresAt: "x" }), "cau-tat"],
      [respond(201, { vigovSession: { tenantDisplayName: "", phoneVerified: false } }), "cau-tat"],
      [respond(201, { vigovSession: 3 }), "khong-goi-duoc"],
      [respond(401, {}), "ma-het-han"],
      [respond(422, {}), "chua-san-sang"],
      [respond(429, {}), "tam-ngung"],
      [respond(502, {}), "tam-ngung"],
      [respond(503, {}), "tam-ngung"],
      [respond(500, {}), "khong-goi-duoc"],
      [new Error("mat mang"), "khong-goi-duoc"],
    ];
    for (const [p, kind] of CASES) {
      stubFetch(p);
      expect((await openVigovSessionViaBridge({ ma_truy_cap: "m", commune_domain: DOMAIN }, ADDRESS)).kind).toBe(kind);
    }
  });

  it("không có địa chỉ → không gọi", async () => {
    const f = vi.fn();
    vi.stubGlobal("fetch", f);
    expect(await openVigovSessionViaBridge({ ma_truy_cap: "m", commune_domain: DOMAIN }, "")).toEqual({ kind: "chua-khai-host" });
    expect(f).not.toHaveBeenCalled();
  });

  it("`issueSession` của khối đăng nhập vẫn đọc phiếu thương mại như trước", async () => {
    stubFetch(respond(201, { token: "tok-thuong-mai", expiresAt: "2026-09-27T10:00:00Z" }));
    expect(await issueSession({ ma_so_dien_thoai: "p", ma_truy_cap: "a" }, ADDRESS)).toEqual({
      kind: "xong",
      session: { token: "tok-thuong-mai", expires_at: "2026-09-27T10:00:00Z" },
    });
  });
});

describe("ghép: lấy mã Zalo rồi gọi cầu", () => {
  it("chuyền đúng mã và tên miền; KHÔNG xin số điện thoại", async () => {
    const call = vi.fn(async () => ({ kind: "cau-tat" as const }));
    const result = await openCitizenSessionViaBridge(DOMAIN, async () => ({ kieu: "xong", du_lieu: "ma-zalo" }), call);
    expect(result).toEqual({ kind: "cau-tat" });
    expect(call).toHaveBeenCalledWith({ ma_truy_cap: "ma-zalo", commune_domain: DOMAIN });
  });

  it("ngoài Zalo / không lấy được mã / mã rỗng → không gọi cầu", async () => {
    const call = vi.fn();
    expect(await openCitizenSessionViaBridge(DOMAIN, async () => ({ kieu: "ngoai-zalo" }), call)).toEqual({ kind: "ngoai-zalo" });
    expect(await openCitizenSessionViaBridge(DOMAIN, async () => ({ kieu: "khong-lay-duoc" }), call)).toEqual({
      kind: "khong-lay-duoc-ma",
    });
    expect(await openCitizenSessionViaBridge(DOMAIN, async () => ({ kieu: "xong", du_lieu: "" }), call)).toEqual({
      kind: "khong-lay-duoc-ma",
    });
    expect(call).not.toHaveBeenCalled();
  });
});

/**
 * MỞ LẠI PHIÊN KÈM SỐ (28/09/2026) — khi ViGov trả 403 `chua_xac_thuc_so` và công dân bấm đồng ý.
 * Thân thứ hai của cùng tuyến; thân xác nhận xã ở trên KHÔNG đổi.
 */
describe("mở lại phiên kèm `phoneToken` — thân, lời gọi, và không một chỗ nào giữ mã số", () => {
  const REQUEST = { ma_truy_cap: "ma-zalo", commune_domain: DOMAIN, ma_so_dien_thoai: "ma-so-thu" };
  const BOTH_CODES = { ma_truy_cap: "ma-zalo", ma_so_dien_thoai: "ma-so-thu" };

  it("thân = ba khoá của bước xác nhận xã + `phoneToken`, khoá hai chiều với bảng khai", () => {
    const body = JSON.parse(bridgeBodyWithPhone(REQUEST)) as Record<string, unknown>;
    expect(body).toEqual({
      accessToken: "ma-zalo",
      communeHostHint: DOMAIN,
      communeConfirmed: true,
      phoneToken: "ma-so-thu",
    });
    expect(Object.keys(body).sort()).toEqual(BRIDGE_FIELDS_WITH_PHONE.map((t) => t.khoa).sort());
    // Câu khai `phoneToken` là CHÍNH dòng của khối đăng nhập — tham chiếu, không phải bản chép thứ hai.
    expect(BRIDGE_FIELDS_WITH_PHONE).toContain(SESSION_SENT_FIELDS.find((t) => t.khoa === "phoneToken"));
  });

  it("POST đúng tuyến, mang `phoneToken`, đọc `phoneVerified` của phiên mới", async () => {
    stubFetch(respond(201, { vigovSession: { ...VIGOV_SESSION.vigovSession, phoneVerified: true } }));
    const result = await reopenViGovSessionWithPhone(REQUEST, ADDRESS);
    expect(result).toMatchObject({ kind: "xong", session: { token: "tok-vigov-thu", phone_verified: true } });
    expect(calls).toHaveLength(1);
    expect(calls[0]!.address).toBe(ADDRESS);
    expect(JSON.parse(calls[0]!.options.body as string)).toMatchObject({ phoneToken: "ma-so-thu" });
    // Mã số KHÔNG nằm trên đường dẫn, không trong tiêu đề.
    expect(calls[0]!.address).not.toContain("ma-so-thu");
    expect(JSON.stringify(calls[0]!.options.headers)).not.toContain("ma-so-thu");
  });

  it("cùng bảng mã trạng thái với lần mở đầu", async () => {
    for (const [reply, kind] of [
      [respond(400, {}), "cau-tat"],
      [respond(401, {}), "ma-het-han"],
      [respond(422, {}), "chua-san-sang"],
      [respond(502, {}), "tam-ngung"],
      [new Error("mat mang"), "khong-goi-duoc"],
    ] as const) {
      stubFetch(reply);
      expect((await reopenViGovSessionWithPhone(REQUEST, ADDRESS)).kind).toBe(kind);
    }
  });

  it("ghép: lấy HAI mã rồi gọi cầu một lần, với đúng tên miền đã xác nhận", async () => {
    const bridge = vi.fn(async () => ({ kind: "cau-tat" as const }));
    const result = await reopenCitizenSessionWithPhone(
      DOMAIN,
      async () => ({ kieu: "xong", du_lieu: BOTH_CODES }),
      bridge,
    );
    expect(result).toEqual({ kind: "cau-tat" });
    expect(bridge).toHaveBeenCalledTimes(1);
    expect(bridge).toHaveBeenCalledWith(REQUEST);
  });

  it("TỪ CHỐI trên hộp thoại của Zalo → `tu-choi`, KHÔNG gọi cầu", async () => {
    const bridge = vi.fn();
    expect(await reopenCitizenSessionWithPhone(DOMAIN, async () => ({ kieu: "tu-choi" }), bridge)).toEqual({
      kind: "tu-choi",
    });
    expect(bridge).not.toHaveBeenCalled();
  });

  it("ngoài Zalo · không lấy được · mã số rỗng · mã phiên rỗng → không gọi cầu", async () => {
    const bridge = vi.fn();
    expect(await reopenCitizenSessionWithPhone(DOMAIN, async () => ({ kieu: "ngoai-zalo" }), bridge)).toEqual({
      kind: "ngoai-zalo",
    });
    for (const codes of [
      { ...BOTH_CODES, ma_so_dien_thoai: "" },
      { ...BOTH_CODES, ma_truy_cap: "" },
    ]) {
      expect(
        await reopenCitizenSessionWithPhone(DOMAIN, async () => ({ kieu: "xong", du_lieu: codes }), bridge),
      ).toEqual({ kind: "khong-lay-duoc-ma" });
    }
    expect(await reopenCitizenSessionWithPhone(DOMAIN, async () => ({ kieu: "khong-lay-duoc" }), bridge)).toEqual({
      kind: "khong-lay-duoc-ma",
    });
    expect(bridge).not.toHaveBeenCalled();
  });

  it("mã số không vào console, và không đi ngược lên kết quả", async () => {
    const spies = (["log", "info", "warn", "error", "debug"] as const).map((m) =>
      vi.spyOn(console, m).mockImplementation(() => {}),
    );
    try {
      stubFetch(respond(201, { vigovSession: { ...VIGOV_SESSION.vigovSession, phoneVerified: true } }));
      const result = await reopenCitizenSessionWithPhone(
        DOMAIN,
        async () => ({ kieu: "xong", du_lieu: BOTH_CODES }),
        (req) => reopenViGovSessionWithPhone(req, ADDRESS),
      );
      expect(JSON.stringify(result)).not.toContain("ma-so-thu");
      expect(JSON.stringify(toReopenWithPhoneResult(result))).not.toContain("ma-so-thu");
      for (const s of spies) expect(s).not.toHaveBeenCalled();
    } finally {
      for (const s of spies) s.mockRestore();
    }
  });

  it("App.tsx dịch: `xong` mang cờ `phone_verified`; `tu-choi` giữ nguyên; còn lại như lần mở đầu", () => {
    expect(
      toReopenWithPhoneResult({
        kind: "xong",
        session: { token: "t", expires_at: "", commune_name: "Xã Của Phiên", phone_verified: true, commune_domain: DOMAIN },
      }),
    ).toEqual({ kind: "xong", token: "t", commune_name: "Xã Của Phiên", phone_verified: true });
    expect(toReopenWithPhoneResult({ kind: "tu-choi" })).toEqual({ kind: "tu-choi" });
    expect(toReopenWithPhoneResult({ kind: "cau-tat" })).toEqual({ kind: "chua-mo" });
    expect(toReopenWithPhoneResult({ kind: "tam-ngung" })).toEqual({ kind: "thu-lai" });
    expect(toReopenWithPhoneResult({ kind: "ngoai-zalo" })).toEqual({ kind: "ngoai-zalo" });
  });
});

describe("App.tsx dịch kết quả sang kiểu của nửa nhà nước — theo việc người dân làm tiếp", () => {
  it("xong → bearer + tên xã của PHIÊN + tên miền chính của phiên (hoặc `null`)", () => {
    expect(
      toCitizenShape({
        kind: "xong",
        session: { token: "t", expires_at: "", commune_name: "Xã Của Phiên", phone_verified: false, commune_domain: DOMAIN },
      }),
    ).toEqual({ kind: "xong", token: "t", commune_name: "Xã Của Phiên", domain: DOMAIN });
    expect(
      toCitizenShape({
        kind: "xong",
        session: { token: "t", expires_at: "", commune_name: "Xã Của Phiên", phone_verified: false, commune_domain: null },
      }),
    ).toEqual({ kind: "xong", token: "t", commune_name: "Xã Của Phiên", domain: null });
  });

  it("thân 201 thật → `toCitizenShape`: `communePrimaryHost` đi suốt tới kiểu của nửa nhà nước", async () => {
    stubFetch(respond(201, { vigovSession: { ...VIGOV_SESSION.vigovSession, communePrimaryHost: DOMAIN } }));
    const result = toCitizenShape(await openVigovSessionViaBridge({ ma_truy_cap: "m", commune_domain: DOMAIN }, ADDRESS));
    expect(result).toEqual({ kind: "xong", token: "tok-vigov-thu", commune_name: "Xã Của Phiên", domain: DOMAIN });
  });

  it("bấm lại không đổi được gì → `chua-mo`; bấm lại có thể được → `thu-lai`", () => {
    for (const kind of ["cau-tat", "chua-san-sang", "chua-khai-host"] as const) {
      expect(toCitizenShape({ kind }), kind).toEqual({ kind: "chua-mo" });
    }
    for (const kind of ["ma-het-han", "tam-ngung", "khong-goi-duoc", "khong-lay-duoc-ma"] as const) {
      expect(toCitizenShape({ kind }), kind).toEqual({ kind: "thu-lai" });
    }
    expect(toCitizenShape({ kind: "ngoai-zalo" })).toEqual({ kind: "ngoai-zalo" });
  });
});
