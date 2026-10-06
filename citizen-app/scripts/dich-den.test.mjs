import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { docCauHinh, TEN_BIEN_CHO_PHEP, TEP_LOCAL } from "./cau-hinh.mjs";
import { VIGOV_PLATFORM_API_HOST } from "./deploy-config.mjs";
import {
  appConfigChoLanDay,
  appIdFromPlatform,
  appIdTrongToken,
  checkPlatformApiHost,
  docCo,
  hasTarget,
  kiemTenMien,
  kiemToken,
  miniAppIdQuery,
  nhanPhienBan,
  noTargetMessage,
  parseTargetAnswer,
  planLines,
  TARGET_PROMPT,
  targetOf,
  tokenTrongTepEnv,
  UNVERIFIED_CONFIRM_PROMPT,
  unverifiedRefusal,
} from "./dich-den.mjs";
import { lookupMiniAppId } from "./mini-app-id-lookup.mjs";

/**
 * CHỌN ĐÍCH CỦA LẦN ĐẨY (ADR 0047). Mỗi ca dưới là một cách đẩy nhầm app mà không lỗi nào báo ra:
 * gõ nhầm cờ thành đẩy app chung, tên miền lạ rơi về app chung, token của xã A đẩy lên app xã B.
 *
 * Các App ID ở đây là GIẢ (`111…`, `222…`) — không phải App ID của app nào.
 */

/** JWT giả, không chữ ký thật: `dich-den` chỉ GIẢI MÃ payload, không xác minh. */
function tokenCua(app_id) {
  const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256", typ: "JWT" })}.${b64({ appId: app_id })}.chu-ky-gia`;
}

describe("tên miền: chỉ nhận tên máy trần", () => {
  it("tên miền hợp lệ thì qua", () => {
    expect(kiemTenMien("xa-a.vigov.example")).toBe("xa-a.vigov.example");
    expect(kiemTenMien("phuong-1.a0.vn")).toBe("phuong-1.a0.vn");
  });

  it.each([
    ["https://xa-a.vigov.example", /scheme/],
    ["xa-a.vigov.example:443", /cổng/],
    ["xa-a.vigov.example/phan-anh", /đường dẫn/],
    ["Xa-A.vigov.example", /chữ hoa/],
    ["localhost", /hai phần/],
    ["xa-a..example", /phần rỗng/],
    ["-xa.vigov.example", /gạch nối/],
    ["xa-a.vigov.example.", /phần rỗng/],
    ["xa_a.vigov.example", /ký tự lạ/],
  ])("%s thì DỪNG", (ten, vi_sao) => {
    expect(() => kiemTenMien(ten)).toThrow(vi_sao);
  });
});

describe("token trong .env do zmp login ghi ra", () => {
  const t = tokenCua("1111111111111111111");
  it.each([
    [`APP_ID=1111111111111111111\nZMP_TOKEN=${t}\n`],
    [`APP_ID="1"\r\nZMP_TOKEN="${t}"\r\n`],
    [`ZMP_TOKEN='${t}'`],
  ])("đọc được token dù có nháy hay CRLF", (noi_dung) => {
    expect(tokenTrongTepEnv(noi_dung)).toBe(t);
  });

  it.each([[""], ["APP_ID=1\n"], ["ZMP_TOKEN=\n"], ['ZMP_TOKEN=""'], [undefined]])(
    "không có token thì null — deploy dừng, không đẩy bằng token rỗng",
    (noi_dung) => {
      expect(tokenTrongTepEnv(noi_dung)).toBeNull();
    },
  );
});

describe("cờ dòng lệnh", () => {
  it("không cờ nào là CHƯA CÓ ĐÍCH — không phải app chung", () => {
    const co = docCo([]);
    expect(co).toEqual({ ten_mien: null, shared_app: false, app_id: null, phat_hanh: false, chi_thu: false });
    expect(hasTarget(co)).toBe(false);
  });

  it("đọc đủ các cờ của app riêng", () => {
    expect(docCo(["--domain=xa-a.vigov.example", "--app-id=123", "--phat-hanh", "--thu"])).toEqual({
      ten_mien: "xa-a.vigov.example",
      shared_app: false,
      app_id: "123",
      phat_hanh: true,
      chi_thu: true,
    });
  });

  it("`--app=vihat` là App ViHAT", () => {
    const co = docCo(["--app=vihat"]);
    expect(co.shared_app).toBe(true);
    expect(co.ten_mien).toBeNull();
    expect(hasTarget(co)).toBe(true);
  });

  it("`--app=vihat` cùng `--domain` thì DỪNG và chỉ tới platform-admin — QR của xã không làm bằng một lần đẩy", () => {
    for (const argv of [
      ["--app=vihat", "--domain=xa-a.vigov.example"],
      ["--domain=xa-a.vigov.example", "--app=vihat"],
    ]) {
      expect(() => docCo(argv)).toThrow(/platform-admin.*Mở bằng app ViHAT/s);
    }
  });

  it("`--app=<khác vihat>` thì DỪNG", () => {
    for (const c of ["--app=xa-a.vigov.example", "--app=", "--app=ViHAT", "--app=chung"]) {
      expect(() => docCo([c]), c).toThrow(/Chỉ nhận --app=vihat/);
    }
  });

  it("`--vao-thang` (bỏ 06/10/2026) thì DỪNG và nói `--domain` nay luôn làm việc ấy", () => {
    for (const argv of [["--vao-thang"], ["--domain=xa-a.vigov.example", "--vao-thang"], ["--vao-thang=co"]]) {
      expect(() => docCo(argv), argv.join(" ")).toThrow(/--vao-thang đã bỏ.*--domain.*LUÔN nung/s);
    }
  });

  it("`--app-id` phải chỉ gồm chữ số; hai giá trị khác nhau thì DỪNG", () => {
    for (const c of ["--app-id=", "--app-id=12a", "--app-id= 12", "--app-id=<APP-ID>"]) {
      expect(() => docCo(["--app=vihat", c]), c).toThrow(/chỉ gồm chữ số/);
    }
    expect(() => docCo(["--app-id=1", "--app-id=2"])).toThrow(/hai giá trị/);
    expect(docCo(["--app-id=1", "--app-id=1"]).app_id).toBe("1");
  });

  it("hai `--domain` khác nhau thì DỪNG", () => {
    expect(() => docCo(["--domain=xa-a.vigov.example", "--domain=xa-b.vigov.example"])).toThrow(/hai giá trị/);
  });

  it("`--bien-the` (cờ đã bỏ 27/09/2026) thì DỪNG và nói vì sao — không lặng lẽ bị bỏ qua", () => {
    // Người gõ cờ ấy đang tin rằng họ chọn được nội dung bản dựng. Bỏ qua nó là để họ tin tiếp.
    for (const c of ["--bien-the=goc", "--bien-the=day-du", "--bien-the"]) {
      expect(() => docCo([c]), c).toThrow(/đã bỏ.*không còn biến thể/s);
    }
  });

  it("cờ gõ nhầm thì DỪNG và liệt kê các cờ còn nhận", () => {
    expect(() => docCo(["--domian=xa-a.vigov.example"])).toThrow(/không có.*--app=vihat.*--domain/s);
    expect(() => docCo(["--domain", "xa-a.vigov.example"])).toThrow(/không có/);
    expect(() => docCo(["--app", "vihat"])).toThrow(/không có/);
  });

  it("`--domain=` để trống thì DỪNG — không hiểu thành app chung", () => {
    expect(() => docCo(["--domain="])).toThrow(/để trống.*--app=vihat/s);
  });

  it("`--domain` sai hình dạng thì DỪNG ngay lúc đọc cờ", () => {
    expect(() => docCo(["--domain=https://xa-a.vigov.example"])).toThrow(/scheme/);
  });
});

describe("đích không cờ: hỏi vihat hay tên miền — không có menu", () => {
  it("`vihat` (hoa thường, khoảng trắng) là App ViHAT; tên miền trần là app riêng", () => {
    expect(parseTargetAnswer(" ViHAT \n")).toEqual({ shared_app: true, ten_mien: null });
    expect(parseTargetAnswer("xa-a.vigov.example")).toEqual({ shared_app: false, ten_mien: "xa-a.vigov.example" });
    expect(TARGET_PROMPT).toBe("App ViHAT (gõ vihat) hay tên miền xã: ");
  });

  it("trống hay không phải tên miền trần thì ném — deploy.mjs hỏi lại", () => {
    expect(() => parseTargetAnswer("")).toThrow(/Chưa gõ gì/);
    expect(() => parseTargetAnswer(undefined)).toThrow(/Chưa gõ gì/);
    expect(() => parseTargetAnswer("1")).toThrow(/không hợp lệ/);
    expect(() => parseTargetAnswer("https://xa-a.vigov.example")).toThrow(/scheme/);
  });

  it("không có người để hỏi: câu từ chối nêu đủ HAI cách chọn, không liệt kê xã nào", () => {
    const msg = noTargetMessage();
    expect(msg).toMatch(/không phải một cửa sổ lệnh có người ngồi/);
    expect(msg).toMatch(/--app=vihat/);
    expect(msg).toMatch(/--domain=<tên-miền>/);
    expect(msg).toMatch(/citizen-app\/\.env/);
    expect(msg).not.toMatch(/vigov\.vn/);
  });

  it("targetOf / miniAppIdQuery: đích → truy vấn của tuyến", () => {
    expect(() => targetOf({ shared_app: false, ten_mien: null })).toThrow(/Chưa chọn đích/);
    const vihat = targetOf({ shared_app: true, ten_mien: null });
    const xa = targetOf({ shared_app: false, ten_mien: "xa-a.vigov.example" });
    expect(vihat).toEqual({ loai: "app-chung", ten_mien: null });
    expect(miniAppIdQuery(vihat)).toEqual({ app: "vihat" });
    expect(miniAppIdQuery(xa)).toEqual({ host: "xa-a.vigov.example" });
  });
});

describe("địa chỉ platform: hằng đã commit, chỉ origin https", () => {
  it("hằng của deploy-config.mjs qua phép kiểm và là đúng origin chủ dự án giao", () => {
    expect(checkPlatformApiHost(VIGOV_PLATFORM_API_HOST)).toBe("https://platform.api.vigov.vn");
  });

  it.each([
    ["", /rỗng/],
    ["platform.api.vigov.vn", /không phải một địa chỉ/],
    ["http://platform.api.vigov.vn", /https/],
    ["https://platform.api.vigov.vn/api/v1", /chỉ là origin/],
    ["https://platform.api.vigov.vn?x=1", /chỉ là origin/],
    ["https://platform.api.vigov.vn/?", /chỉ là origin/],
    ["https://u:p@platform.api.vigov.vn", /tên\/mật khẩu/],
  ])("%s thì DỪNG, và câu báo chỉ ra chỗ sửa", (v, why) => {
    expect(() => checkPlatformApiHost(v)).toThrow(why);
    expect(() => checkPlatformApiHost(v)).toThrow(/deploy-config\.mjs/);
  });

  it("dấu `/` cuối được bỏ", () => {
    expect(checkPlatformApiHost("https://platform.vigov.example/")).toBe("https://platform.vigov.example");
  });
});

describe("App ID từ platform → đích (appIdFromPlatform)", () => {
  const vihat = { loai: "app-chung", ten_mien: null };
  const xa = { loai: "app-rieng", ten_mien: "xa-a.vigov.example" };
  const http = (status, body = null) => ({ kind: "http", status, body });
  const ok = (app_id, source) => http(200, { app_id, source });

  it("200 đúng loại đích → App ID ấy; --app-id trùng thì vẫn qua", () => {
    expect(appIdFromPlatform(vihat, ok("3333333333333333333", "chung"))).toEqual({
      kind: "verified",
      app_id: "3333333333333333333",
    });
    expect(appIdFromPlatform(xa, ok("1111111111111111111", "rieng"), "1111111111111111111").app_id).toBe(
      "1111111111111111111",
    );
  });

  it("200 mà --app-id KHÁC → DỪNG, không chọn hộ", () => {
    expect(() => appIdFromPlatform(xa, ok("1111111111111111111", "rieng"), "2222222222222222222")).toThrow(
      /--app-id=2222222222222222222 khác App ID platform.*\(1111111111111111111\)/s,
    );
  });

  it("200 sai loại: App ViHAT ra `rieng` (đè app xã), xã ra `chung` (đè app chung) → DỪNG", () => {
    expect(() => appIdFromPlatform(vihat, ok("1", "rieng"))).toThrow(/app RIÊNG.*đè app của xã/s);
    expect(() => appIdFromPlatform(xa, ok("3", "chung"))).toThrow(/app CHUNG.*đè App ViHAT/s);
  });

  it("200 thân hỏng → DỪNG", () => {
    for (const body of [
      null,
      {},
      { app_id: "12a", source: "chung" },
      { app_id: 3, source: "chung" },
      { app_id: "3", source: "x" },
    ]) {
      expect(() => appIdFromPlatform(vihat, http(200, body)), JSON.stringify(body)).toThrow(/không đúng hợp đồng/);
    }
  });

  it("404 mini_app_id_not_found → DỪNG, chỉ chỗ sửa ở platform-admin — kể cả có --app-id", () => {
    const nf = http(404, { code: "mini_app_id_not_found", message: "x", trace_id: "t" });
    expect(() => appIdFromPlatform(vihat, nf)).toThrow(/platform-admin → khai App ViHAT/);
    expect(() => appIdFromPlatform(xa, nf, "1111111111111111111")).toThrow(
      /chi tiết xã xa-a\.vigov\.example → gắn App ID.*--app-id không thay được/s,
    );
  });

  it("409 → DỪNG và nêu mã; 400 → DỪNG", () => {
    expect(() => appIdFromPlatform(xa, http(409, { code: "mini_app_id_ambiguous" }), "1")).toThrow(
      /409 \(mini_app_id_ambiguous\).*hơn một app/s,
    );
    expect(() => appIdFromPlatform(xa, http(400, { code: "invalid_query" }))).toThrow(/400 invalid_query/);
  });

  it("không tới được (mạng · 5xx · 429 · 404 không mã) không --app-id → DỪNG, fail closed, nói lối khẩn cấp", () => {
    for (const o of [
      { kind: "network", reason: "ECONNREFUSED" },
      http(503),
      http(500, { code: "internal" }),
      http(429, { code: "rate_limited" }),
      http(404),
      http(404, { code: "not_found" }),
    ]) {
      expect(() => appIdFromPlatform(vihat, o), JSON.stringify(o)).toThrow(/fail closed.*--app-id=<chữ số>/s);
    }
  });

  it("không tới được + --app-id → CHƯA ĐỐI CHIẾU, kèm cảnh báo lớn; không bao giờ 'verified'", () => {
    const r = appIdFromPlatform(xa, { kind: "network", reason: "quá 10 giây" }, "1111111111111111111");
    expect(r).toMatchObject({ kind: "unverified", app_id: "1111111111111111111" });
    expect(r.warning).toMatch(/KHÔNG được đối chiếu với platform \(không kết nối được \(quá 10 giây\)\)/);
    expect(appIdFromPlatform(vihat, http(429), "3").kind).toBe("unverified");
    expect(unverifiedRefusal(xa)).toMatch(/không phải một cửa sổ lệnh có người ngồi.*Jenkins/s);
    expect(UNVERIFIED_CONFIRM_PROMPT).toMatch(/\(c\/k\)/);
  });

  it("mã khác (401, 403, 302) → DỪNG, không phải lối khẩn cấp", () => {
    for (const s of [401, 403, 302]) {
      expect(() => appIdFromPlatform(vihat, http(s), "3"), String(s)).toThrow(/không đúng hợp đồng/);
    }
  });
});

describe("lời gọi platform (lookupMiniAppId), fetch giả", () => {
  it("dựng đúng URL, GET, không theo chuyển hướng; trả status + thân JSON", async () => {
    const seen = [];
    const fetchImpl = async (url, init) => {
      seen.push({ url: String(url), init });
      return new Response(JSON.stringify({ app_id: "3", source: "chung" }), { status: 200 });
    };
    const r = await lookupMiniAppId("https://platform.vigov.example", { app: "vihat" }, { fetchImpl });
    expect(r).toEqual({ kind: "http", status: 200, body: { app_id: "3", source: "chung" } });
    expect(seen[0].url).toBe("https://platform.vigov.example/api/v1/mini-app-ids?app=vihat");
    expect(seen[0].init).toMatchObject({ method: "GET", redirect: "error" });
    expect(seen[0].init.signal).toBeInstanceOf(AbortSignal);
    await lookupMiniAppId("https://platform.vigov.example", { host: "xa-a.vigov.example" }, { fetchImpl });
    expect(seen[1].url).toBe("https://platform.vigov.example/api/v1/mini-app-ids?host=xa-a.vigov.example");
  });

  it("thân không phải JSON → body null; fetch ném → network, hết giờ nói rõ", async () => {
    const text = async () => new Response("404 page not found", { status: 404 });
    expect(await lookupMiniAppId("https://p.example", { app: "vihat" }, { fetchImpl: text })).toEqual({
      kind: "http",
      status: 404,
      body: null,
    });
    const refused = async () => {
      throw Object.assign(new TypeError("fetch failed"), { cause: { code: "ECONNREFUSED" } });
    };
    expect(await lookupMiniAppId("https://p.example", { app: "vihat" }, { fetchImpl: refused })).toEqual({
      kind: "network",
      reason: "ECONNREFUSED",
    });
    const slow = async () => {
      throw Object.assign(new Error("t"), { name: "TimeoutError" });
    };
    expect(
      await lookupMiniAppId("https://p.example", { app: "vihat" }, { fetchImpl: slow, timeoutMs: 10_000 }),
    ).toEqual({ kind: "network", reason: "quá 10 giây" });
  });
});

describe("token: đích thật là claim appId của ZMP_TOKEN", () => {
  const rieng_a = { loai: "app-rieng", ten_mien: "xa-a.vigov.example", app_id: "1111111111111111111" };
  const vihat = { loai: "app-chung", ten_mien: null, app_id: "3333333333333333333" };

  it("đọc được claim appId; không phải JWT thì null", () => {
    expect(appIdTrongToken(tokenCua("1111111111111111111"))).toBe("1111111111111111111");
    expect(appIdTrongToken(tokenCua(1111))).toBe("1111");
    expect(appIdTrongToken("khong-phai-jwt")).toBeNull();
    expect(appIdTrongToken("a.!!!.c")).toBeNull();
    expect(appIdTrongToken(undefined)).toBeNull();
  });

  it("token đúng App ID thì qua — cả hai đích", () => {
    expect(kiemToken(rieng_a, tokenCua("1111111111111111111")).ok).toBe(true);
    expect(kiemToken(vihat, tokenCua("3333333333333333333")).ok).toBe(true);
  });

  it("token app khác thì TỪ CHỐI", () => {
    const kq = kiemToken(rieng_a, tokenCua("2222222222222222222"));
    expect(kq.ok).toBe(false);
    expect(kq.ly_do).toMatch(/2222222222222222222.*không phải 1111111111111111111/);
    expect(kiemToken(vihat, tokenCua("4444444444444444444")).ok).toBe(false);
  });

  it("không token trong môi trường thì TỪ CHỐI — CẢ App ViHAT; không đích nào dùng `.env` của máy", () => {
    for (const dich of [rieng_a, vihat]) {
      expect(kiemToken(dich, undefined)).toMatchObject({ ok: false, ly_do: expect.stringMatching(/\.env/) });
      expect(kiemToken(dich, "").ok).toBe(false);
    }
  });

  it("token không đọc được claim thì TỪ CHỐI", () => {
    expect(kiemToken(rieng_a, "khong-phai-jwt").ok).toBe(false);
    expect(kiemToken(vihat, "khong-phai-jwt").ok).toBe(false);
  });

  it("đích chưa có App ID (null / không phải chữ số) thì TỪ CHỐI, kể cả token có claim trùng", () => {
    expect(kiemToken({ ...vihat, app_id: null }, tokenCua("3333333333333333333")).ok).toBe(false);
    const vi_du = { loai: "app-rieng", ten_mien: "xa-c.vigov.example", app_id: "<APP-ID>" };
    expect(kiemToken(vi_du, tokenCua("<APP-ID>"))).toMatchObject({ ok: false });
  });

  it("lý do không bao giờ chứa token", () => {
    const token = tokenCua("2222222222222222222");
    for (const dich of [rieng_a, vihat]) {
      const { ly_do } = kiemToken(dich, token);
      expect(ly_do).not.toContain(token);
      expect(ly_do).not.toContain(token.split(".")[1]);
    }
  });
});

describe("nhãn phiên bản phân biệt được đích trên console Zalo", () => {
  const chung = { sha: "abc1234", luc: "2026-09-27 10:00", dirty: false };

  it("App ViHAT và app riêng cùng commit ra hai nhãn khác nhau, cả hai mang App ID", () => {
    const a = nhanPhienBan({ ...chung, dich: { loai: "app-chung", ten_mien: null, app_id: "3333333333333333333" } });
    const b = nhanPhienBan({
      ...chung,
      dich: { loai: "app-rieng", ten_mien: "xa-a.vigov.example", app_id: "1111111111111111111" },
    });
    expect(a).toBe("app-vihat · app 3333333333333333333 · abc1234 · 2026-09-27 10:00");
    expect(b).toBe("xa-a.vigov.example · app 1111111111111111111 · abc1234 · 2026-09-27 10:00");
  });

  it("dirty vẫn hiện", () => {
    expect(
      nhanPhienBan({ ...chung, dirty: true, dich: { loai: "app-chung", ten_mien: null, app_id: "3" } }),
    ).toMatch(/ · dirty$/);
  });
});

describe("kế hoạch in ra trước khi chạy", () => {
  const base = {
    kiem_token: { ok: true, ly_do: "thuộc đúng App ID 1." },
    token_source: "ZMP_TOKEN của môi trường",
    phat_hanh: false,
    api_host: "https://api.vigov.example",
    platform_host: "https://platform.vigov.example",
    mota: "nhan",
  };
  const vihat = { loai: "app-chung", ten_mien: null, app_id: "3333333333333333333", app_id_source: "unverified" };
  const rieng = { loai: "app-rieng", ten_mien: "xa-a.vigov.example", app_id: "1", app_id_source: "platform" };

  it("App ViHAT: đích, App ID và nguồn của nó, KHÔNG nung xã, nguồn token", () => {
    const t = planLines({ ...base, dich: vihat }).join("\n");
    expect(t).toMatch(/Đích {5}: App ViHAT/);
    expect(t).toMatch(/App ID {3}: 3333333333333333333 {2}\(--app-id, CHƯA ĐỐI CHIẾU platform\)/);
    expect(t).toMatch(/Nung xã {2}: KHÔNG/);
    expect(t).toMatch(/Token {4}: ZMP_TOKEN của môi trường — thuộc đúng App ID/);
    expect(t).not.toMatch(/Logo xã|Banner xã/);
    expect(t).toMatch(/THỬ NGHIỆM/);
  });

  it("app riêng: tên miền CÓ nung vào bundle; logo/banner; token không qua thì nói ra", () => {
    const t = planLines({
      ...base,
      dich: rieng,
      phat_hanh: true,
      kiem_token: { ok: false, ly_do: "lý do" },
      logo: "scripts/logo-xa/xa-a.vigov.example.png",
    }).join("\n");
    expect(t).toMatch(/Đích {5}: APP RIÊNG của xã xa-a\.vigov\.example/);
    expect(t).toMatch(/\(platform https:\/\/platform\.vigov\.example\)/);
    expect(t).toMatch(/Nung xã {2}: CÓ — xa-a\.vigov\.example/);
    expect(t).toMatch(/Logo xã {2}: scripts\/logo-xa/);
    expect(t).toMatch(/Banner xã: \(chưa có/);
    expect(t).toMatch(/KHÔNG QUA: lý do/);
    expect(t).toMatch(/PHÁT HÀNH/);
  });
});

describe("App ID và token KHÔNG BAO GIỜ tới được bundle", () => {
  const doc = (duong) => readFileSync(new URL(duong, import.meta.url), "utf8");

  it("docCauHinh vẫn TRẢ VỀ ĐÚNG HAI KHOÁ khi APP_ID và ZMP_TOKEN có trong môi trường", () => {
    const goc = mkdtempSync(join(tmpdir(), "vigov-dich-den-"));
    writeFileSync(join(goc, TEP_LOCAL), "VIGOV_API_HOST=https://tu-tep.test\n", "utf8");
    const truoc = { APP_ID: process.env.APP_ID, ZMP_TOKEN: process.env.ZMP_TOKEN };
    process.env.APP_ID = "1111111111111111111";
    process.env.ZMP_TOKEN = tokenCua("1111111111111111111");
    try {
      const cau_hinh = docCauHinh(`${goc}/`);
      expect(Object.keys(cau_hinh).sort()).toEqual([...TEN_BIEN_CHO_PHEP].sort());
      expect(JSON.stringify(cau_hinh)).not.toContain("1111111111111111111");
      expect(JSON.stringify(cau_hinh)).not.toContain(process.env.ZMP_TOKEN);
    } finally {
      for (const [k, v] of Object.entries(truoc)) {
        if (v === undefined) delete process.env[k];
        else process.env[k] = v;
      }
    }
  });

  it("tệp ánh xạ không được nhập từ `vite.config.ts` hay bất kỳ tệp sản phẩm nào dưới `src/`", () => {
    // Vite chỉ gói thứ được nhập từ điểm vào, và `define` chỉ nhận khoá đặt tên (ca ở
    // cau-hinh.test.mjs). Nên "không ai trong đường dựng nhập tệp ánh xạ" là đủ để nó không vào
    // bundle — ca này ghim điều kiện ấy.
    //
    // TỆP TEST ĐƯỢC MIỄN (27/09/2026), và chỉ chúng: không điểm vào nào tới được một `*.test.*`, nên
    // chúng không thể kéo tệp ánh xạ vào bundle. `bundle-for-zalo.test.ts` đọc tệp ấy bằng `?raw`
    // để khẳng định điều mạnh hơn — không khoá / App ID nào của nó có mặt trong bundle dựng thật.
    const cam = /ung-dung-theo-ten-mien|dich-den|deploy-config|mini-app-id-lookup/;
    expect(doc("../vite.config.ts")).not.toMatch(cam);
    const src = fileURLToPath(new URL("../src/", import.meta.url));
    const di = (thu_muc) => {
      for (const ten of readdirSync(thu_muc)) {
        const duong = join(thu_muc, ten);
        if (statSync(duong).isDirectory()) di(duong);
        else if (/\.(m?[jt]sx?)$/.test(ten) && !ten.includes(".test.")) {
          expect(readFileSync(duong, "utf8"), `${duong} nhập tệp chọn đích`).not.toMatch(cam);
        }
      }
    };
    di(src);
  });

  it("`deploy.mjs` không ghi vào `process.env` — App ID chỉ đi vào tiến trình con của zmp", () => {
    // `dung()` kế thừa `process.env`; App ID đặt vào đó là giá trị theo xã nằm cạnh bước dựng.
    const ma = doc("./deploy.mjs");
    expect(ma).not.toMatch(/process\.env(\.[A-Z_]+|\[[^\]]+\])\s*=[^=]/);
    expect(ma).toMatch(/function zmp\(env, /);
    // Và cổng token thật sự được nối: lần chạy thật dừng khi token không khớp đích.
    // The token checked is the one handed to zmp: from the shell, or from the login below.
    expect(ma).toMatch(/let token = process\.env\.ZMP_TOKEN/);
    expect(ma).toMatch(/token = dangNhapRieng\(dich\.app_id\)/);
    expect(ma).toMatch(/kiemToken\(dich, token\)/);
    expect(ma).toMatch(/ZMP_TOKEN: token/);
    expect(ma).toMatch(/if \(!kiem_token\.ok && !chi_thu\)/);
  });

  it("`zmp login` chạy trong thư mục tạm — không bao giờ ghi một token xuống `citizen-app/.env`", () => {
    const ma = doc("./deploy.mjs");
    const than = ma.slice(ma.indexOf("function dangNhapRieng("), ma.indexOf("function nghi("));
    expect(than).toMatch(/mkdtempSync\(/);
    expect(than).toMatch(/cwd: thu_muc/);
    expect(than).toMatch(/rmSync\(thu_muc/);
  });
});

describe("app-config.json cho một lần đẩy", () => {
  const goc = readFileSync(new URL("../app-config.json", import.meta.url), "utf8");

  it("App ViHAT: nguyên văn — không đổi một byte", () => {
    expect(appConfigChoLanDay(goc, false)).toBe(goc);
  });

  it("app riêng ẩn thanh tiêu đề Zalo, giữ mọi khoá khác", () => {
    const moi = JSON.parse(appConfigChoLanDay(goc, true));
    const cu = JSON.parse(goc);
    expect(moi.app.actionBarHidden).toBe(true);
    expect({ ...moi.app, actionBarHidden: undefined }).toEqual({ ...cu.app, actionBarHidden: undefined });
    expect(moi.listSyncJS).toEqual(cu.listSyncJS);
  });

  it("deploy.mjs đổi tệp CHỈ cho app riêng và trả về nguyên văn trong `finally`", () => {
    const ma = readFileSync(new URL("./deploy.mjs", import.meta.url), "utf8");
    expect(ma).toMatch(/const own_app = dich\.loai === "app-rieng";/);
    expect(ma).toMatch(/if \(own_app\) writeFileSync\(TEP_APP_CONFIG, appConfigChoLanDay\(app_config_goc, true\)/);
    expect(ma).toMatch(/finally \{\s*if \(own_app\) writeFileSync\(TEP_APP_CONFIG, app_config_goc/);
  });
});

describe("mọi script dòng lệnh phải PHÂN TÍCH ĐƯỢC", () => {
  // Các ca trên đọc `deploy.mjs` như VĂN BẢN — một dấu nháy hỏng vẫn xanh hết. Lần phát hành
  // 28/09/2026 chết ngay dòng đầu vì đúng lỗi ấy. `node --check` phân tích mà không chạy gì.
  const thu_muc = fileURLToPath(new URL(".", import.meta.url));
  for (const ten of readdirSync(thu_muc).filter((t) => t.endsWith(".mjs") && !t.includes(".test."))) {
    it(`${ten} qua node --check`, () => {
      const r = spawnSync(process.execPath, ["--check", join(thu_muc, ten)], { encoding: "utf8" });
      expect(r.status, r.stderr).toBe(0);
    });
  }
});

describe("logo xã tạm thời chỉ đi vào đúng bản dựng app riêng của xã", () => {
  const ma = readFileSync(new URL("./deploy.mjs", import.meta.url), "utf8");

  it("chỉ chép cho app riêng, và xoá trong `finally` sau bước dựng", () => {
    expect(ma).toMatch(/const LOGO_NGUON = own_app \?/);
    expect(ma).toMatch(/if \(co_logo\) copyFileSync\(LOGO_NGUON, LOGO_DICH\)/);
    expect(ma).toMatch(/finally \{[^}]*if \(co_logo\) rmSync\(LOGO_DICH/);
  });

  it("logo Thăng Bình có ở chỗ deploy.mjs tìm, và bản chép vào public/ bị git bỏ qua", () => {
    expect(existsSync(new URL("./logo-xa/thangbinh-danang.vigov.vn.png", import.meta.url))).toBe(true);
    expect(readFileSync(new URL("../.gitignore", import.meta.url), "utf8")).toMatch(/^public\/logo-xa\.png$/m);
  });
});

describe("banner xã tạm thời chỉ đi vào đúng bản dựng app riêng của xã", () => {
  // Same exception, same shape as the logo: a banner left in `public/` would ship one commune's picture
  // inside the next build — the shared app included.
  const ma = readFileSync(new URL("./deploy.mjs", import.meta.url), "utf8");

  it("chỉ chép cho app riêng, và xoá trong `finally` sau bước dựng", () => {
    expect(ma).toMatch(/const BANNER_SOURCE = own_app \?/);
    expect(ma).toMatch(/if \(has_banner\) copyFileSync\(BANNER_SOURCE, BANNER_TARGET\)/);
    // The block is read whole: the logo's `{ force: true }` before it would stop a `[^}]*` match short.
    const sau_dung = /ma = dung\(env_dung\);\s*\} finally \{([\s\S]*?)\n\}/.exec(ma);
    expect(sau_dung, "khối finally sau bước dựng").not.toBeNull();
    expect(sau_dung[1]).toMatch(/if \(has_banner\) rmSync\(BANNER_TARGET/);
  });

  it("bản chép vào public/ bị git bỏ qua", () => {
    expect(readFileSync(new URL("../.gitignore", import.meta.url), "utf8")).toMatch(/^public\/banner-xa\.png$/m);
  });
});

describe("`deploy.mjs --thu` end to end, against a local stub of the platform route", () => {
  const deploy = fileURLToPath(new URL("./deploy.mjs", import.meta.url));
  const VIHAT = "3333333333333333333";
  const THANG_BINH = "4444444444444444444"; // fake: the stub's answer, not the real App ID

  // The stub answers the FIXED contract (06/10/2026). Every query not listed is a plain-text 404 — what an
  // ingress without the route answers.
  const ANSWERS = {
    "app=vihat": [200, { app_id: VIHAT, source: "chung" }],
    "host=thangbinh-danang.vigov.vn": [200, { app_id: THANG_BINH, source: "rieng" }],
    "host=xa-swap.vigov.example": [200, { app_id: VIHAT, source: "chung" }],
    "host=xa-none.vigov.example": [404, { code: "mini_app_id_not_found", message: "x", trace_id: "t" }],
    "host=xa-dup.vigov.example": [409, { code: "mini_app_id_ambiguous", message: "x", trace_id: "t" }],
    "host=xa-down.vigov.example": [503, { code: "internal", message: "x", trace_id: "t" }],
    "host=xa-limit.vigov.example": [429, { code: "rate_limited", message: "x", trace_id: "t" }],
  };
  const seen = [];
  let server;
  let port;
  let closedPort;
  let preloadDir;

  // NO OVERRIDE KNOB IN PRODUCTION CODE. The platform origin is a committed constant; the child is pointed at
  // the stub by a preload (`node --import`) written here, which swaps the ORIGIN of every fetch to the stub.
  // deploy.mjs itself — the https check, the URL it builds, the mapping — runs unchanged.
  const preloadFor = (p) => {
    const file = join(preloadDir, `redirect-${p}.mjs`);
    writeFileSync(
      file,
      "const real = globalThis.fetch;\n" +
        "globalThis.fetch = (input, init) => { const u = new URL(String(input)); " +
        `return real(new URL(u.pathname + u.search, "http://127.0.0.1:${p}"), init); };\n`,
      "utf8",
    );
    return pathToFileURL(file).href;
  };

  beforeAll(async () => {
    preloadDir = mkdtempSync(join(tmpdir(), "vigov-platform-stub-"));
    server = createServer((req, res) => {
      const u = new URL(req.url, "http://stub");
      seen.push(`${req.method} ${u.pathname}?${u.searchParams.toString()}`);
      const hit = u.pathname === "/api/v1/mini-app-ids" ? ANSWERS[u.searchParams.toString()] : undefined;
      if (hit === undefined) {
        res.writeHead(404, { "content-type": "text/plain" });
        res.end("404 page not found");
        return;
      }
      res.writeHead(hit[0], { "content-type": "application/json" });
      res.end(JSON.stringify(hit[1]));
    });
    await new Promise((ok) => server.listen(0, "127.0.0.1", ok));
    port = server.address().port;
    // A port that WAS free and is closed again: connecting to it is refused (no platform at all).
    const tmp = createServer();
    await new Promise((ok) => tmp.listen(0, "127.0.0.1", ok));
    closedPort = tmp.address().port;
    await new Promise((ok) => tmp.close(ok));
  });

  afterAll(async () => {
    await new Promise((ok) => server.close(ok));
  });

  // Async spawn: the stub lives in THIS process, so a spawnSync would block the loop that must answer it.
  // stdin is a pipe, closed at once — not a TTY — so every run here is the non-interactive path (Jenkins).
  // ZMP_TOKEN emptied: the dry run must not depend on this machine.
  const dryRun = (flags, env = {}, { to = port } = {}) =>
    new Promise((resolve) => {
      const child = spawn(process.execPath, ["--import", preloadFor(to), deploy, ...flags], {
        env: { ...process.env, ZMP_TOKEN: "", VIGOV_XA_CO_DINH: "", ...env },
      });
      let stdout = "";
      let stderr = "";
      child.stdout.on("data", (d) => (stdout += d));
      child.stderr.on("data", (d) => (stderr += d));
      child.stdin.end();
      child.on("close", (status) => resolve({ status, stdout, stderr }));
    });

  it("--domain: --thu ASKS the platform, prints its App ID, and ALWAYS bakes in the commune domain", async () => {
    const before = seen.length;
    const r = await dryRun(["--domain=thangbinh-danang.vigov.vn", "--thu"]);
    expect(r.status, r.stderr).toBe(0);
    expect(seen.slice(before)).toEqual(["GET /api/v1/mini-app-ids?host=thangbinh-danang.vigov.vn"]);
    expect(r.stdout).toMatch(
      new RegExp(`App ID {3}: ${THANG_BINH} {2}\\(platform https://platform\\.api\\.vigov\\.vn\\)`),
    );
    expect(r.stdout).toMatch(/VIGOV_XA_CO_DINH=thangbinh-danang\.vigov\.vn vite build/);
    expect(r.stdout).toMatch(/Nung xã {2}: CÓ/);
    expect(r.stdout).toMatch(new RegExp(`APP_ID=${THANG_BINH} ZMP_TOKEN=<môi trường>`));
    // No token in a non-interactive run: the plan says the real run would stop.
    expect(r.stdout).toMatch(/Lần chạy thật sẽ DỪNG ở đây: ZMP_TOKEN chưa có/);
  });

  it("--app=vihat: plan of the shared app, NO domain baked, the platform's App ID in the label", async () => {
    const before = seen.length;
    const r = await dryRun(["--app=vihat", `--app-id=${VIHAT}`, "--thu"], {
      ZMP_TOKEN: tokenCua(VIHAT),
      VIGOV_XA_CO_DINH: "thangbinh-danang.vigov.vn",
    });
    expect(r.status, r.stderr).toBe(0);
    expect(seen.slice(before)).toEqual(["GET /api/v1/mini-app-ids?app=vihat"]);
    expect(r.stdout).toMatch(/Đích {5}: App ViHAT/);
    expect(r.stdout).toMatch(/Nung xã {2}: KHÔNG/);
    expect(r.stdout).toMatch(new RegExp(`Nhãn {5}: app-vihat · app ${VIHAT} · `));
    expect(r.stdout).toMatch(/VIGOV_API_HOST=.+ vite build/);
    expect(r.stdout).not.toMatch(/VIGOV_XA_CO_DINH=/);
    expect(r.stdout).not.toMatch(/Lần chạy thật sẽ DỪNG/);
    expect(r.stdout).not.toContain(tokenCua(VIHAT));
  });

  it("token of another app: the plan says the real run would stop", async () => {
    const r = await dryRun(["--app=vihat", "--thu"], { ZMP_TOKEN: tokenCua(THANG_BINH) });
    expect(r.status, r.stderr).toBe(0);
    expect(r.stdout).toMatch(
      new RegExp(`Lần chạy thật sẽ DỪNG ở đây: ZMP_TOKEN là token của App ID ${THANG_BINH}, không phải ${VIHAT}`),
    );
  });

  it.each([
    [["--app=vihat", "--app-id=1"], /--app-id=1 khác App ID platform/],
    [["--domain=xa-none.vigov.example"], /chưa có App ID nào.*platform-admin → chi tiết xã xa-none/s],
    [["--domain=xa-none.vigov.example", "--app-id=1"], /--app-id không thay được/],
    [["--domain=xa-dup.vigov.example"], /409 \(mini_app_id_ambiguous\)/],
    [["--domain=xa-swap.vigov.example"], /app CHUNG.*đè App ViHAT/s],
    [["--domain=xa-down.vigov.example"], /platform lỗi \(503\).*fail closed/s],
    [["--domain=xa-limit.vigov.example"], /429.*fail closed/s],
    [["--domain=xa-new.vigov.example"], /chưa có tuyến tra App ID/],
  ])("%j --thu refuses (exit 2) with the platform's reason, no plan", async (flags, why) => {
    const r = await dryRun([...flags, "--thu"]);
    expect(r.status, r.stdout).toBe(2);
    expect(r.stderr).toMatch(why);
    expect(r.stdout).not.toMatch(/ĐỌC TRƯỚC KHI/);
    expect(r.stderr).not.toMatch(/\n\s+at /); // a sentence, never a stack trace
  });

  it("platform down + --app-id, no terminal: loud warning, then refuses — the fallback needs a person", async () => {
    const r = await dryRun(["--domain=xa-down.vigov.example", "--app-id=1111111111111111111", "--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/App ID 1111111111111111111 .*KHÔNG được đối chiếu với platform/);
    expect(r.stderr).toMatch(/không phải một cửa sổ lệnh có người ngồi để xác nhận/);
    expect(r.stdout).not.toMatch(/ĐỌC TRƯỚC KHI/);
  });

  it("no platform at all (connection refused): refuses, fail closed", async () => {
    const r = await dryRun(["--app=vihat", "--thu"], {}, { to: closedPort });
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/không kết nối được.*fail closed/s);
  });

  it("no target, no terminal: refuses with the two forms — never a default, never asks the platform", async () => {
    const before = seen.length;
    const r = await dryRun(["--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/chưa chọn đích/);
    expect(r.stderr).toMatch(/--app=vihat/);
    expect(r.stderr).toMatch(/--domain=<tên-miền>/);
    expect(r.stdout).not.toMatch(/ĐỌC TRƯỚC KHI/);
    expect(seen.length).toBe(before);
  });

  it("--vao-thang refuses with the removal sentence", async () => {
    const r = await dryRun(["--vao-thang", "--domain=thangbinh-danang.vigov.vn", "--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/--vao-thang đã bỏ/);
  });
});
