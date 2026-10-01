import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * 403 `chua_xac_thuc_so` → HỎI công dân → mở lại phiên KÈM số → gọi lại việc cũ ĐÚNG MỘT LẦN.
 *
 * NGUỒN PHIÊN LÀ NGUỒN THẬT (`api/phien-vigov.ts`), không giả lập: `reopenSessionWithPhone` GHI vào nó,
 * và ca nào cũng phải thấy đúng bearer mà lời gọi lại mang đi. Chỉ `fetch` và hàm mở lại tiêm vào là giả.
 */

import { guiPhanAnh, phanAnhCuaToi, traCuuPhieu } from "../api/goi-vigov";
import { thanGuiPhanAnh } from "../api/hop-dong-phan-anh";
import { taoLanGui } from "../api/lan-gui";
import { type ReopenWithPhone, type ReopenWithPhoneResult, reopenSessionWithPhone } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { buocSauKhiGui } from "./GuiPhanAnhScreen";
import { COMMUNE_APP_SESSION, PHONE_VERIFICATION, PHONE_VERIFICATION_TASK } from "./noi-dung";
import {
  createPhoneVerification,
  PhoneVerificationPanel,
  type PhoneVerificationState,
  phoneVerificationMessage,
} from "./phone-verification";

const COMMUNE = "Xã Thử Nghiệm";
const OLD_TOKEN = "tok-cu";
const NEW_TOKEN = "tok-moi";

type Call = { url: string; init: RequestInit };
let calls: Call[] = [];

function reply(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

/** Phản hồi lần lượt; hết danh sách thì lặp phản hồi cuối. */
function stubFetch(...replies: Array<ReturnType<typeof reply>>) {
  let i = 0;
  vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return Promise.resolve(replies[Math.min(i++, replies.length - 1)]!);
  });
}

const PHONE_REQUIRED = reply(403, { code: "chua_xac_thuc_so", message: "x", trace_id: "" });

const TICKET = {
  code: "PA7K2QX9M4TD",
  status: "da-tiep-nhan",
  field: "",
  field_label: "",
  content: "Ổ gà lớn trước cổng chợ",
  address: "",
  reporter_name: "",
  reporter_phone: "",
  anonymous: true,
  clock_from: "2026-09-24T01:30:00Z",
  acknowledge_due: null,
  resolve_due: null,
  result: "",
};

const header = (c: Call, name: string) => (c.init.headers as Record<string, string>)[name];

/** Hàm mở lại giả: ghi lại số lần gọi, trả một kết quả cố định. */
function fakeReopen(result: ReopenWithPhoneResult) {
  return vi.fn<ReopenWithPhone>(async () => result);
}

const VERIFIED: ReopenWithPhoneResult = { kieu: "xong", token: NEW_TOKEN, ten_xa: COMMUNE, da_xac_thuc_so: true };

beforeEach(() => {
  calls = [];
  datPhienViGov({ token: OLD_TOKEN, ten_xa: COMMUNE });
});

afterEach(() => {
  vi.unstubAllGlobals();
  datPhienViGov(null);
});

/* ─────────────────────────────── lớp gọi: nhánh mới của 403 ─────────────────────────────── */

describe("lớp gọi: 403 chỉ là `can-xac-thuc-so` khi mã là `chua_xac_thuc_so`", () => {
  it("cả ba tuyến phản ánh: 403 `chua_xac_thuc_so` → `can-xac-thuc-so`", async () => {
    stubFetch(PHONE_REQUIRED);
    expect(await guiPhanAnh(taoLanGui("{}"))).toEqual({ kieu: "can-xac-thuc-so" });
    expect(await traCuuPhieu("PA7K2QX9M4TD")).toEqual({ kieu: "can-xac-thuc-so" });
    expect(await phanAnhCuaToi("")).toEqual({ kieu: "can-xac-thuc-so" });
  });

  it("403 mã khác · thân không có mã · thân không phải JSON → `loi-may-chu`, không hỏi số", async () => {
    for (const r of [
      reply(403, { code: "forbidden" }),
      reply(403, { message: "chua_xac_thuc_so" }),
      reply(403, "chua_xac_thuc_so"),
      reply(403, null),
    ]) {
      stubFetch(r);
      expect((await guiPhanAnh(taoLanGui("{}"))).kieu).toBe("loi-may-chu");
      expect((await phanAnhCuaToi("")).kieu).toBe("loi-may-chu");
    }
    vi.stubGlobal("fetch", async () => ({
      status: 403,
      ok: false,
      json: async () => {
        throw new SyntaxError("not json");
      },
    }));
    // KHÔNG phải `loi-mang`: máy chủ đã trả lời tới nơi.
    expect((await traCuuPhieu("PA7K2QX9M4TD")).kieu).toBe("loi-may-chu");
  });

  it("màn gửi: `can-xac-thuc-so` thành bước `can-so`, không thành một câu lỗi", () => {
    expect(buocSauKhiGui({ kieu: "can-xac-thuc-so" })).toEqual({ kieu: "can-so" });
  });
});

/* ─────────────────────────── ghi phiên mới: chỉ khi nó sửa được việc ─────────────────────────── */

describe("reopenSessionWithPhone — phiên mới chỉ thay phiên cũ khi cùng xã VÀ đã xác thực số", () => {
  it("cùng xã, đã xác thực → ghi, `da-xac-thuc`", async () => {
    expect(await reopenSessionWithPhone(fakeReopen(VERIFIED))).toEqual({ kieu: "da-xac-thuc" });
    expect(layPhienViGov()).toEqual({ token: NEW_TOKEN, ten_xa: COMMUNE });
  });

  it("xã KHÁC → `khac-xa`, phiên cũ giữ nguyên (không đổi xã lặng lẽ)", async () => {
    const r = await reopenSessionWithPhone(fakeReopen({ ...VERIFIED, ten_xa: "Xã Khác" }));
    expect(r).toEqual({ kieu: "khac-xa" });
    expect(layPhienViGov()).toEqual({ token: OLD_TOKEN, ten_xa: COMMUNE });
  });

  it("máy chủ vẫn chưa xác thực số → `van-chua-xac-thuc`, không ghi", async () => {
    const r = await reopenSessionWithPhone(fakeReopen({ ...VERIFIED, da_xac_thuc_so: false }));
    expect(r).toEqual({ kieu: "van-chua-xac-thuc" });
    expect(layPhienViGov()?.token).toBe(OLD_TOKEN);
  });

  it("bearer rỗng → `chua-mo`; hàm tiêm ném → `thu-lai`; các nhánh khác đi nguyên", async () => {
    expect(await reopenSessionWithPhone(fakeReopen({ ...VERIFIED, token: "" }))).toEqual({ kieu: "chua-mo" });
    expect(
      await reopenSessionWithPhone(async () => {
        throw new Error("x");
      }),
    ).toEqual({ kieu: "thu-lai" });
    for (const kieu of ["tu-choi", "chua-mo", "thu-lai", "ngoai-zalo"] as const) {
      expect(await reopenSessionWithPhone(fakeReopen({ kieu }))).toEqual({ kieu });
    }
    expect(layPhienViGov()?.token).toBe(OLD_TOKEN);
  });

  it("không có phiên hiện tại → `chua-mo`, và hàm mở lại KHÔNG được gọi", async () => {
    datPhienViGov(null);
    const reopen = fakeReopen(VERIFIED);
    expect(await reopenSessionWithPhone(reopen)).toEqual({ kieu: "chua-mo" });
    expect(reopen).not.toHaveBeenCalled();
    expect(layPhienViGov()).toBeNull();
  });
});

/* ─────────────────────────────── máy trạng thái: hỏi · mở lại · gọi lại một lần ─────────────────────────────── */

function machineWith(reopen: ReopenWithPhone | undefined) {
  const states: Array<PhoneVerificationState | null> = [];
  const m = createPhoneVerification(reopen, (s) => states.push(s));
  return { m, states, last: () => states[states.length - 1] };
}

describe("createPhoneVerification — hỏi trước, xin sau, gọi lại ĐÚNG MỘT LẦN", () => {
  it("gặp 403 thì HỎI; chưa bấm đồng ý thì không mở lại, không gọi lại", () => {
    const reopen = fakeReopen(VERIFIED);
    const rerun = vi.fn();
    const { m, last } = machineWith(reopen);
    m.onPhoneRequired(rerun);
    expect(last()).toEqual({ kieu: "hoi" });
    expect(reopen).not.toHaveBeenCalled();
    expect(rerun).not.toHaveBeenCalled();
  });

  it("đồng ý → mở lại MỘT lần → gọi lại MỘT lần; 403 lần hai → câu kết cục, KHÔNG hỏi lại, không mở lại", async () => {
    const reopen = fakeReopen(VERIFIED);
    const rerun = vi.fn();
    const { m, states, last } = machineWith(reopen);
    m.onPhoneRequired(rerun);
    await m.allow();
    expect(reopen).toHaveBeenCalledTimes(1);
    expect(rerun).toHaveBeenCalledTimes(1);
    expect(states).toEqual([{ kieu: "hoi" }, { kieu: "dang-xac-nhan" }, null]);

    // Lời gọi lại vẫn trả 403.
    m.onPhoneRequired(rerun);
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "van-chua-xac-thuc" });
    await m.allow(); // một cú bấm lạc cũng không làm gì
    expect(reopen).toHaveBeenCalledTimes(1);
    expect(rerun).toHaveBeenCalledTimes(1);
  });

  it("công dân bấm 'Không chia sẻ' → câu trung thực, không mở lại; lần 403 sau được HỎI lại (họ có quyền đổi ý)", async () => {
    const reopen = fakeReopen(VERIFIED);
    const rerun = vi.fn();
    const { m, last } = machineWith(reopen);
    m.onPhoneRequired(rerun);
    m.decline();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "tu-choi" });
    await m.allow(); // không còn việc chờ: không làm gì
    expect(reopen).not.toHaveBeenCalled();
    expect(rerun).not.toHaveBeenCalled();
    m.onPhoneRequired(rerun);
    expect(last()).toEqual({ kieu: "hoi" });
  });

  it("từ chối trên hộp thoại Zalo → không gọi lại", async () => {
    const rerun = vi.fn();
    const { m, last } = machineWith(fakeReopen({ kieu: "tu-choi" }));
    m.onPhoneRequired(rerun);
    await m.allow();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "tu-choi" });
    expect(rerun).not.toHaveBeenCalled();
  });

  it("mạng hỏng → câu mời bấm lại; bấm lại thành công → gọi lại đúng một lần", async () => {
    let n = 0;
    const reopen = vi.fn<ReopenWithPhone>(async () => (n++ === 0 ? { kieu: "thu-lai" } : VERIFIED));
    const rerun = vi.fn();
    const { m, last } = machineWith(reopen);
    m.onPhoneRequired(rerun);
    await m.allow();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "thu-lai" });
    expect(rerun).not.toHaveBeenCalled();
    await m.allow();
    expect(reopen).toHaveBeenCalledTimes(2);
    expect(rerun).toHaveBeenCalledTimes(1);
  });

  it("khác xã · vẫn chưa xác thực · cầu tắt · ngoài Zalo → kết cục CUỐI: 403 sau không hỏi lại", async () => {
    for (const [result, outcome] of [
      [{ ...VERIFIED, ten_xa: "Xã Khác" }, "khac-xa"],
      [{ ...VERIFIED, da_xac_thuc_so: false }, "van-chua-xac-thuc"],
      [{ kieu: "chua-mo" }, "chua-mo"],
      [{ kieu: "ngoai-zalo" }, "ngoai-zalo"],
    ] as const) {
      const reopen = fakeReopen(result);
      const rerun = vi.fn();
      const { m, last } = machineWith(reopen);
      m.onPhoneRequired(rerun);
      await m.allow();
      expect(last(), outcome).toEqual({ kieu: "ket-qua", outcome });
      m.onPhoneRequired(rerun);
      expect(last(), outcome).toEqual({ kieu: "ket-qua", outcome });
      expect(reopen, outcome).toHaveBeenCalledTimes(1);
      expect(rerun, outcome).not.toHaveBeenCalled();
    }
    expect(layPhienViGov()?.token).toBe(OLD_TOKEN);
  });

  it("không có hàm mở lại (lần mở này không qua xác nhận xã) → không hỏi, nói thẳng", () => {
    const rerun = vi.fn();
    const { m, last } = machineWith(undefined);
    m.onPhoneRequired(rerun);
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "chua-mo" });
    expect(rerun).not.toHaveBeenCalled();
  });
});

/* ─────────────────────────────── ghép thật: lớp gọi + máy trạng thái ─────────────────────────────── */

describe("ghép thật với lớp gọi — cùng lần gửi, bearer mới, và không vòng lặp", () => {
  it("GỬI: 403 → đồng ý → gửi lại CÙNG thân, CÙNG khoá, với bearer MỚI → có mã tra cứu", async () => {
    stubFetch(PHONE_REQUIRED, reply(201, TICKET));
    // Nháp của công dân: thân dựng MỘT lần từ nội dung họ gõ. Lần gọi lại dùng lại đúng lần gửi này.
    const lan = taoLanGui(thanGuiPhanAnh({ noi_dung: "Ổ gà lớn", dia_chi: "", ho_ten: "", dien_thoai: "", an_danh: true }));
    const results: string[] = [];
    const send = async () => {
      const r = await guiPhanAnh(lan);
      results.push(r.kieu);
      if (r.kieu === "can-xac-thuc-so") m.onPhoneRequired(() => void send());
    };
    const reopen = fakeReopen(VERIFIED);
    const { m } = machineWith(reopen);

    await send();
    await m.allow();
    await vi.waitFor(() => expect(results).toEqual(["can-xac-thuc-so", "xong"]));

    expect(calls).toHaveLength(2);
    expect(header(calls[0]!, "Authorization")).toBe(`Bearer ${OLD_TOKEN}`);
    expect(header(calls[1]!, "Authorization")).toBe(`Bearer ${NEW_TOKEN}`);
    expect(header(calls[1]!, "Idempotency-Key")).toBe(header(calls[0]!, "Idempotency-Key"));
    expect(calls[1]!.init.body).toBe(calls[0]!.init.body);
    expect(reopen).toHaveBeenCalledTimes(1);
  });

  it("máy chủ VẪN trả 403 sau khi mở lại → đúng HAI lời gọi, một lần mở lại, rồi dừng", async () => {
    stubFetch(PHONE_REQUIRED);
    const reopen = fakeReopen(VERIFIED);
    const { m, last } = machineWith(reopen);
    const load = async () => {
      const r = await phanAnhCuaToi("");
      if (r.kieu === "can-xac-thuc-so") m.onPhoneRequired(() => void load());
    };
    await load();
    await m.allow();
    await vi.waitFor(() => expect(last()).toEqual({ kieu: "ket-qua", outcome: "van-chua-xac-thuc" }));
    // Thêm một nhịp: nếu có vòng lặp, nó đã kịp gọi thêm.
    await new Promise((r) => setTimeout(r, 10));
    expect(calls).toHaveLength(2);
    expect(reopen).toHaveBeenCalledTimes(1);
  });

  it("từ chối → chỉ MỘT lời gọi đi ra, và không gì được ghi lại trên máy", async () => {
    stubFetch(PHONE_REQUIRED);
    const { m } = machineWith(fakeReopen({ kieu: "tu-choi" }));
    const r = await traCuuPhieu("PA7K2QX9M4TD");
    if (r.kieu === "can-xac-thuc-so") m.onPhoneRequired(() => void traCuuPhieu("PA7K2QX9M4TD"));
    await m.allow();
    expect(calls).toHaveLength(1);
    expect(layPhienViGov()).toEqual({ token: OLD_TOKEN, ten_xa: COMMUNE });
  });
});

/* ─────────────────────────────── khung hiện: chữ, nút, trình đọc màn hình ─────────────────────────────── */

describe("PhoneVerificationPanel — nói vì sao trước, hai nút to, kết cục có việc làm tiếp", () => {
  const render = (state: PhoneVerificationState, draftKept = false) =>
    renderToStaticMarkup(
      createElement(PhoneVerificationPanel, {
        state,
        task: "submit",
        focusId: "cd-xac-thuc-so",
        draftKept,
        onAllow: () => {},
        onDecline: () => {},
      }),
    );

  it("hỏi: tiêu đề nhận tiêu điểm, câu vì sao, câu Zalo sẽ hỏi gì, rồi hai nút", () => {
    const html = render({ kieu: "hoi" });
    expect(html).toContain(`id="cd-xac-thuc-so" tabindex="-1">${PHONE_VERIFICATION.title}`);
    expect(html).toContain(PHONE_VERIFICATION.why);
    expect(html).toContain(PHONE_VERIFICATION.zalo_asks);
    expect(html).toContain(`<button type="button" class="cd-nut">${PHONE_VERIFICATION.allow}</button>`);
    expect(html).toContain(`<button type="button" class="cd-nut-phu">${PHONE_VERIFICATION.decline}</button>`);
    // Câu vì sao đứng TRƯỚC nút đồng ý.
    expect(html.indexOf(PHONE_VERIFICATION.why)).toBeLessThan(html.indexOf(PHONE_VERIFICATION.allow));
  });

  it("the commune's own app names ITS route (ADR 0066) — the rating block's reopen says the same as the gate", () => {
    const html = renderToStaticMarkup(
      createElement(PhoneVerificationPanel, {
        state: { kieu: "hoi" },
        task: "rate",
        zaloAsks: COMMUNE_APP_SESSION.zalo_asks,
        onAllow: () => {},
        onDecline: () => {},
      }),
    );
    expect(html).toContain(COMMUNE_APP_SESSION.zalo_asks);
    expect(html).not.toContain(PHONE_VERIFICATION.zalo_asks);
    // The one commune-app caller passes it: `PhanAnhAppXa` is rendered only by `TrangXa` (the commune's own app).
    const source = Object.values(
      import.meta.glob("./PhanAnhAppXa.tsx", { query: "?raw", import: "default", eager: true }),
    )[0] as string;
    expect(source).toContain("zaloAsks={COMMUNE_APP_SESSION.zalo_asks}");
  });

  it("đang xác nhận: một vùng `role=\"status\"`, không nút nào bấm được", () => {
    const html = render({ kieu: "dang-xac-nhan" });
    expect(html).toContain('role="status"');
    expect(html).not.toContain("<button");
  });

  it("kết cục: `role=\"alert\"`, câu nói đúng việc của màn; chỉ `thu-lai` có nút bấm lại", () => {
    for (const outcome of ["tu-choi", "thu-lai", "ngoai-zalo", "chua-mo", "van-chua-xac-thuc", "khac-xa"] as const) {
      const html = render({ kieu: "ket-qua", outcome });
      expect(html, outcome).toContain('role="alert"');
      expect(html, outcome).toContain(phoneVerificationMessage(outcome, "submit"));
      expect(html.includes("<button"), outcome).toBe(outcome === "thu-lai");
    }
  });

  it("màn gửi nói nháp vẫn còn", () => {
    expect(render({ kieu: "hoi" }, true)).toContain(PHONE_VERIFICATION.draft_kept);
    expect(render({ kieu: "ket-qua", outcome: "tu-choi" }, true)).toContain(PHONE_VERIFICATION.draft_kept);
    expect(render({ kieu: "hoi" }, false)).not.toContain(PHONE_VERIFICATION.draft_kept);
  });

  it("mọi câu kết cục nhắc đúng việc của màn, không mã lỗi, không 'đổi xã'", () => {
    for (const task of ["submit", "lookup", "mine"] as const) {
      for (const outcome of ["tu-choi", "thu-lai", "ngoai-zalo", "chua-mo", "van-chua-xac-thuc", "khac-xa"] as const) {
        const cau = phoneVerificationMessage(outcome, task);
        expect(cau).toContain(PHONE_VERIFICATION_TASK[task]);
        expect(cau).not.toMatch(/\b(4\d\d|5\d\d)\b|chua_xac_thuc_so|token|đổi xã/i);
      }
    }
  });
});
