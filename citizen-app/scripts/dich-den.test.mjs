import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { describe, expect, it } from "vitest";

import { docCauHinh, TEN_BIEN_CHO_PHEP, TEP_LOCAL } from "./cau-hinh.mjs";
import {
  appConfigChoLanDay,
  appIdPrompt,
  appIdTrongToken,
  buildMenu,
  checkAppIdInput,
  chonDich,
  docCo,
  hasTarget,
  kiemBangAnhXa,
  kiemTenMien,
  kiemToken,
  laPlaceholder,
  missingAppIdMessage,
  nhanPhienBan,
  noTargetMessage,
  pickMenuItem,
  planLines,
  savedRegistryNote,
  tokenTrongTepEnv,
  updateRegistryText,
} from "./dich-den.mjs";
import { APP_ID_APP_CHUNG, APP_ID_THEO_TEN_MIEN, COMMUNE_TERMS_BY_DOMAIN } from "./ung-dung-theo-ten-mien.mjs";

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

const BANG = {
  "xa-a.vigov.example": "1111111111111111111",
  "xa-a-cu.vigov.example": "1111111111111111111", // tên miền cũ sau sáp nhập — cùng App ID là hợp lệ
  "xa-b.vigov.example": "2222222222222222222",
};

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

describe("tệp ánh xạ", () => {
  it("tệp thật trong kho qua được phép kiểm của chính nó", () => {
    expect(() => kiemBangAnhXa(APP_ID_THEO_TEN_MIEN, APP_ID_APP_CHUNG)).not.toThrow();
  });

  it("tệp thật chỉ có App ID chủ dự án đã giao — không App ID nào được đoán ra", () => {
    // Every real pair is pinned here, so adding one is a deliberate two-file change a reviewer
    // sees — and the same pair needs its `mini_app` row in service-platform (ADR 0047, #3).
    // `deploy.mjs` can write the registry when asked; this pin then stays red until it is updated too.
    const DA_GIAO = {
      "thangbinh-danang.vigov.vn": "3043188591857102858", // owner, 2026-10-01 (replaced 3291993990104489440)
    };
    expect(APP_ID_THEO_TEN_MIEN).toEqual(DA_GIAO);
    for (const app_id of Object.values(APP_ID_THEO_TEN_MIEN)) expect(app_id).toMatch(/^\d+$/);
    expect(APP_ID_APP_CHUNG).toBeNull();
  });

  it("khoá không phải tên miền trần thì DỪNG", () => {
    expect(() => kiemBangAnhXa({ "https://xa-a.vigov.example": "1" }, null)).toThrow(/scheme/);
  });

  it("App ID rỗng, có khoảng trắng, hay không phải chữ số thì DỪNG; placeholder thì qua", () => {
    expect(() => kiemBangAnhXa({ "xa-a.vigov.example": "" }, null)).toThrow(/rỗng/);
    expect(() => kiemBangAnhXa({ "xa-a.vigov.example": "11 11" }, null)).toThrow(/khoảng trắng/);
    expect(() => kiemBangAnhXa({ "xa-a.vigov.example": "11a" }, null)).toThrow(/chữ số/);
    expect(() => kiemBangAnhXa({ "xa-a.vigov.example": "<APP-ID>" }, null)).not.toThrow();
    expect(() => kiemBangAnhXa(BANG, "abc")).toThrow(/chữ số/);
  });

  it("App ID app chung trùng App ID một xã thì DỪNG — App ViHAT sẽ đè lên app của xã", () => {
    expect(() => kiemBangAnhXa(BANG, "2222222222222222222")).toThrow(/đè lên app của xã/);
    expect(() => kiemBangAnhXa(BANG, "3333333333333333333")).not.toThrow();
  });
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

describe("menu chọn đích khi không có cờ", () => {
  it("1 = App ViHAT, rồi từng tên miền theo thứ tự trong tệp, mỗi dòng mang App ID", () => {
    const menu = buildMenu(BANG, null);
    expect(menu.map((m) => m.label)).toEqual([
      "App ViHAT (chưa có App ID)",
      "app riêng xã xa-a.vigov.example (1111111111111111111)",
      "app riêng xã xa-a-cu.vigov.example (1111111111111111111)",
      "app riêng xã xa-b.vigov.example (2222222222222222222)",
    ]);
    expect(menu.map((m) => m.number)).toEqual([1, 2, 3, 4]);
    expect(menu[0]).toMatchObject({ shared_app: true, ten_mien: null });
    expect(menu[3]).toMatchObject({ shared_app: false, ten_mien: "xa-b.vigov.example" });
    expect(buildMenu(BANG, "3333333333333333333")[0].label).toBe("App ViHAT (3333333333333333333)");
    expect(buildMenu({ "xa-c.vigov.example": "<APP-ID>" }, null)[1].label).toMatch(/chưa có App ID/);
  });

  it("chỉ nhận một số có trên menu", () => {
    const menu = buildMenu(BANG, null);
    expect(pickMenuItem(menu, " 1 ").shared_app).toBe(true);
    expect(pickMenuItem(menu, "4").ten_mien).toBe("xa-b.vigov.example");
    for (const t of ["", "0", "5", "1.5", "abc", "-1", undefined]) {
      expect(() => pickMenuItem(menu, t), String(t)).toThrow(/từ 1 đến 4/);
    }
  });

  it("không có người để hỏi: câu từ chối liệt kê mọi cách chọn bằng cờ", () => {
    const msg = noTargetMessage(buildMenu(BANG, null));
    expect(msg).toMatch(/không phải một cửa sổ lệnh có người ngồi/);
    expect(msg).toMatch(/--app=vihat/);
    expect(msg).toMatch(/--domain=xa-b\.vigov\.example/);
    expect(msg).toMatch(/--app-id=<chữ số>/);
    expect(msg).toMatch(/citizen-app\/\.env/);
  });
});

describe("chọn đích → App ID", () => {
  const vihat = { shared_app: true, ten_mien: null, app_id: null };
  const xa = (ten_mien, app_id = null) => ({ shared_app: false, ten_mien, app_id });

  it("chưa chọn đích thì ném — hàm không tự chọn", () => {
    expect(() => chonDich(xa(null), BANG, null)).toThrow(/Chưa chọn đích/);
  });

  it("tệp có App ID → App ID của tệp; tên miền cũ trỏ cùng App ID", () => {
    expect(chonDich(xa("xa-b.vigov.example"), BANG, null)).toEqual({
      loai: "app-rieng",
      ten_mien: "xa-b.vigov.example",
      app_id: "2222222222222222222",
      app_id_source: "registry",
    });
    expect(chonDich(xa("xa-a-cu.vigov.example"), BANG, null).app_id).toBe("1111111111111111111");
    expect(chonDich(vihat, BANG, "3333333333333333333")).toEqual({
      loai: "app-chung",
      ten_mien: null,
      app_id: "3333333333333333333",
      app_id_source: "registry",
    });
  });

  it("tệp chưa có (null / placeholder / tên miền chưa có dòng), không --app-id → app_id null: phải hỏi", () => {
    expect(chonDich(vihat, BANG, null)).toMatchObject({ app_id: null, app_id_source: null });
    expect(chonDich(xa("xa-z.vigov.example"), BANG, null)).toMatchObject({ loai: "app-rieng", app_id: null });
    expect(chonDich(xa("xa-c.vigov.example"), { "xa-c.vigov.example": "<APP-ID>" }, null).app_id).toBeNull();
  });

  it("khoá kế thừa của Object (`constructor`) không phải tên miền có trong bảng", () => {
    // `in` / `bang[x]` thấy cả nguyên mẫu; `Object.hasOwn` thì không.
    expect(chonDich(xa("constructor"), BANG, null).app_id).toBeNull();
  });

  it("tệp chưa có, có --app-id → App ID của cờ", () => {
    expect(chonDich({ ...vihat, app_id: "3333333333333333333" }, BANG, null)).toMatchObject({
      app_id: "3333333333333333333",
      app_id_source: "flag",
    });
    expect(chonDich(xa("xa-z.vigov.example", "9999999999999999999"), BANG, null)).toMatchObject({
      app_id: "9999999999999999999",
      app_id_source: "flag",
    });
  });

  it("--app-id trùng tệp thì qua; KHÁC tệp thì DỪNG", () => {
    expect(chonDich(xa("xa-b.vigov.example", "2222222222222222222"), BANG, null).app_id_source).toBe("registry");
    expect(() => chonDich(xa("xa-b.vigov.example", "1111111111111111111"), BANG, null)).toThrow(
      /--app-id=1111111111111111111 khác App ID.*\(2222222222222222222\)/s,
    );
    expect(() => chonDich({ ...vihat, app_id: "4444444444444444444" }, BANG, "3333333333333333333")).toThrow(
      /khác App ID của App ViHAT/,
    );
  });

  it("--app-id của phía bên kia thì DỪNG: App ViHAT mang App ID một xã, app riêng mang App ID App ViHAT", () => {
    expect(() => chonDich({ ...vihat, app_id: "2222222222222222222" }, BANG, null)).toThrow(/đè app của xã/);
    expect(() => chonDich(xa("xa-z.vigov.example", "3333333333333333333"), BANG, "3333333333333333333")).toThrow(
      /đè app chung/,
    );
  });

  it("câu hỏi và câu từ chối nói đúng đích và chỉ chỗ lấy App ID", () => {
    const d_vihat = chonDich(vihat, BANG, null);
    const d_xa = chonDich(xa("xa-z.vigov.example"), BANG, null);
    expect(appIdPrompt(d_vihat)).toBe("Nhập App ID của App ViHAT (xem platform-admin → chi tiết xã → ô QR): ");
    expect(appIdPrompt(d_xa)).toBe(
      "Nhập App ID của app riêng của xã xa-z.vigov.example (xem platform-admin → chi tiết xã → ô QR): ",
    );
    expect(missingAppIdMessage(d_xa)).toMatch(/xa-z\.vigov\.example.*--app-id=<chữ số>/s);
  });

  it("App ID gõ vào: bỏ khoảng trắng hai đầu, chỉ nhận chữ số", () => {
    expect(checkAppIdInput(" 123 \n")).toBe("123");
    for (const t of ["", "12 3", "abc", "<APP-ID>", undefined]) {
      expect(() => checkAppIdInput(t), String(t)).toThrow(/chỉ gồm chữ số/);
    }
  });
});

describe("ghi App ID vào tệp ánh xạ", () => {
  const that = readFileSync(new URL("./ung-dung-theo-ten-mien.mjs", import.meta.url), "utf8");
  const vihat = { loai: "app-chung", ten_mien: null };
  const xa = (ten_mien) => ({ loai: "app-rieng", ten_mien });
  /** Load the edited text as a module, the way deploy.mjs re-imports it after writing. */
  const nap = async (text) => {
    const thu_muc = mkdtempSync(join(tmpdir(), "vigov-registry-"));
    const tep = join(thu_muc, "registry.mjs");
    writeFileSync(tep, text, "utf8");
    return import(pathToFileURL(tep).href);
  };

  it("App ViHAT: đổi đúng một dòng `APP_ID_APP_CHUNG`", async () => {
    const moi = updateRegistryText(that, vihat, "3333333333333333333");
    const mod = await nap(moi);
    expect(mod.APP_ID_APP_CHUNG).toBe("3333333333333333333");
    expect(mod.APP_ID_THEO_TEN_MIEN).toEqual(APP_ID_THEO_TEN_MIEN);
    expect(() => kiemBangAnhXa(mod.APP_ID_THEO_TEN_MIEN, mod.APP_ID_APP_CHUNG)).not.toThrow();
    const cu = that.split("\n");
    const khac = moi.split("\n").filter((d, i) => d !== cu[i]);
    expect(khac).toEqual(['export const APP_ID_APP_CHUNG = "3333333333333333333";']);
    // Ghi lần hai thay đúng giá trị ấy.
    expect(updateRegistryText(moi, vihat, "4")).toMatch(/^export const APP_ID_APP_CHUNG = "4";$/m);
  });

  it("tên miền chưa có: thêm một dòng cuối bảng App ID, không đụng COMMUNE_TERMS_BY_DOMAIN", async () => {
    const mod = await nap(updateRegistryText(that, xa("xa-z.vigov.example"), "9999999999999999999"));
    expect(mod.APP_ID_THEO_TEN_MIEN).toEqual({ ...APP_ID_THEO_TEN_MIEN, "xa-z.vigov.example": "9999999999999999999" });
    expect(mod.COMMUNE_TERMS_BY_DOMAIN).toEqual(COMMUNE_TERMS_BY_DOMAIN);
    expect(mod.APP_ID_APP_CHUNG).toBeNull();
  });

  it("tên miền đã có (kể cả placeholder): đổi đúng dòng ấy, giữ dấu phẩy", () => {
    const text = [
      "export const APP_ID_THEO_TEN_MIEN = {",
      '  "xa-a.vigov.example": "<APP-ID>",',
      '  "xa-ab.vigov.example": "5"',
      "};",
      "export const APP_ID_APP_CHUNG = null;",
      "export const COMMUNE_TERMS_BY_DOMAIN = {",
      '  "xa-a.vigov.example": { displayName: "X" },',
      "};",
      "",
    ].join("\n");
    const moi = updateRegistryText(text, xa("xa-a.vigov.example"), "7");
    expect(moi).toContain('  "xa-a.vigov.example": "7",\n');
    expect(moi).toContain('  "xa-ab.vigov.example": "5"\n');
    expect(moi).toContain('  "xa-a.vigov.example": { displayName: "X" },');
    // A dot in a domain is a literal dot, not "any character".
    const moi2 = updateRegistryText(text, xa("xa-ab.vigov.example"), "8");
    expect(moi2).toContain('  "xa-ab.vigov.example": "8"\n');
    expect(moi2).toContain('  "xa-a.vigov.example": "<APP-ID>",');
  });

  it("hình dạng lạ hay App ID không phải chữ số thì NÉM, không đoán", () => {
    expect(() => updateRegistryText("export const X = 1;\n", vihat, "1")).toThrow(/APP_ID_APP_CHUNG/);
    expect(() => updateRegistryText("export const X = 1;\n", xa("xa-a.vigov.example"), "1")).toThrow(
      /APP_ID_THEO_TEN_MIEN/,
    );
    expect(() => updateRegistryText(that, vihat, "12a")).toThrow(/chữ số/);
  });

  it("lời nhắc sau khi ghi: phải commit; app riêng còn cần dòng MiniApp ở service-platform", () => {
    expect(savedRegistryNote(vihat)).toMatch(/PHẢI được commit/);
    expect(savedRegistryNote(vihat)).not.toMatch(/MiniApp/);
    expect(savedRegistryNote(xa("xa-z.vigov.example"))).toMatch(/MiniApp.*service-platform/s);
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
    expect(kiemToken(rieng_a, tokenCua("1111111111111111111"), BANG).ok).toBe(true);
    expect(kiemToken(vihat, tokenCua("3333333333333333333"), BANG).ok).toBe(true);
  });

  it("token app khác thì TỪ CHỐI", () => {
    const kq = kiemToken(rieng_a, tokenCua("2222222222222222222"), BANG);
    expect(kq.ok).toBe(false);
    expect(kq.ly_do).toMatch(/2222222222222222222.*không phải 1111111111111111111/);
    expect(kiemToken(vihat, tokenCua("4444444444444444444"), BANG).ok).toBe(false);
  });

  it("không token trong môi trường thì TỪ CHỐI — CẢ App ViHAT; không đích nào dùng `.env` của máy", () => {
    for (const dich of [rieng_a, vihat]) {
      expect(kiemToken(dich, undefined, BANG)).toMatchObject({ ok: false, ly_do: expect.stringMatching(/\.env/) });
      expect(kiemToken(dich, "", BANG).ok).toBe(false);
    }
  });

  it("token không đọc được claim thì TỪ CHỐI", () => {
    expect(kiemToken(rieng_a, "khong-phai-jwt", BANG).ok).toBe(false);
    expect(kiemToken(vihat, "khong-phai-jwt", BANG).ok).toBe(false);
  });

  it("đích chưa có App ID (null / placeholder) thì TỪ CHỐI, kể cả token có claim trùng", () => {
    expect(kiemToken({ ...vihat, app_id: null }, tokenCua("3333333333333333333"), BANG).ok).toBe(false);
    const vi_du = { loai: "app-rieng", ten_mien: "xa-c.vigov.example", app_id: "<APP-ID>" };
    expect(kiemToken(vi_du, tokenCua("<APP-ID>"), BANG)).toMatchObject({ ok: false });
  });

  it("App ViHAT với token của một XÃ thì TỪ CHỐI và nói rõ sẽ đè app của xã", () => {
    const kq = kiemToken(vihat, tokenCua("2222222222222222222"), BANG);
    expect(kq.ok).toBe(false);
    expect(kq.ly_do).toMatch(/app RIÊNG/);
  });

  it("lý do không bao giờ chứa token", () => {
    const token = tokenCua("2222222222222222222");
    for (const dich of [rieng_a, vihat]) {
      const { ly_do } = kiemToken(dich, token, BANG);
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
    mota: "nhan",
  };
  const vihat = { loai: "app-chung", ten_mien: null, app_id: "3333333333333333333", app_id_source: "flag" };
  const rieng = { loai: "app-rieng", ten_mien: "xa-a.vigov.example", app_id: "1", app_id_source: "registry" };

  it("App ViHAT: đích, App ID và nguồn của nó, KHÔNG nung xã, nguồn token", () => {
    const t = planLines({ ...base, dich: vihat }).join("\n");
    expect(t).toMatch(/Đích {5}: App ViHAT/);
    expect(t).toMatch(/App ID {3}: 3333333333333333333 {2}\(--app-id\)/);
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
    expect(t).toMatch(/\(tệp ánh xạ\)/);
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
    const cam = /ung-dung-theo-ten-mien|dich-den/;
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
    expect(ma).toMatch(/kiemToken\(dich, token, /);
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

describe("`deploy.mjs --thu` end to end: the plan, before anything happens", () => {
  const deploy = fileURLToPath(new URL("./deploy.mjs", import.meta.url));
  // `--thu` builds nothing and pushes nothing. spawnSync gives the child a PIPE for stdin — not a TTY —
  // so every run here is the non-interactive path (Jenkins). ZMP_TOKEN emptied: the dry run must not
  // depend on this machine.
  const dryRun = (flags, env = {}) =>
    spawnSync(process.execPath, [deploy, ...flags], {
      encoding: "utf8",
      input: "",
      env: { ...process.env, ZMP_TOKEN: "", VIGOV_XA_CO_DINH: "", ...env },
    });

  it("--domain: exits 0, and the build line ALWAYS bakes in the commune domain", () => {
    const r = dryRun(["--domain=thangbinh-danang.vigov.vn", "--thu"]);
    expect(r.status, r.stderr).toBe(0);
    expect(r.stdout).toMatch(/VIGOV_XA_CO_DINH=thangbinh-danang\.vigov\.vn vite build/);
    expect(r.stdout).toMatch(/Nung xã {2}: CÓ/);
    expect(r.stdout).toMatch(/APP_ID=3043188591857102858 ZMP_TOKEN=<môi trường>/);
    // No token in a non-interactive run: the plan says the real run would stop.
    expect(r.stdout).toMatch(/Lần chạy thật sẽ DỪNG ở đây: ZMP_TOKEN chưa có/);
  });

  it("--app=vihat --app-id: plan of the shared app, NO domain baked, the App ID in the label", () => {
    const r = dryRun(["--app=vihat", "--app-id=123", "--thu"], {
      ZMP_TOKEN: tokenCua("123"),
      VIGOV_XA_CO_DINH: "thangbinh-danang.vigov.vn",
    });
    expect(r.status, r.stderr).toBe(0);
    expect(r.stdout).toMatch(/Đích {5}: App ViHAT/);
    expect(r.stdout).toMatch(/Nung xã {2}: KHÔNG/);
    expect(r.stdout).toMatch(/Nhãn {5}: app-vihat · app 123 · /);
    expect(r.stdout).toMatch(/VIGOV_API_HOST=.+ vite build/);
    expect(r.stdout).not.toMatch(/VIGOV_XA_CO_DINH=/);
    expect(r.stdout).not.toMatch(/Lần chạy thật sẽ DỪNG/);
    expect(r.stdout).not.toContain(tokenCua("123"));
  });

  it("no target, no terminal: refuses and lists the options — never a default", () => {
    const r = dryRun(["--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/chưa chọn đích/);
    expect(r.stderr).toMatch(/--app=vihat/);
    expect(r.stderr).toMatch(/--domain=thangbinh-danang\.vigov\.vn/);
    expect(r.stdout).not.toMatch(/ĐỌC TRƯỚC KHI/);
  });

  it("shared app with no App ID anywhere, no terminal: refuses and asks for --app-id", () => {
    const r = dryRun(["--app=vihat", "--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/App ID của App ViHAT chưa có.*--app-id=<chữ số>/s);
  });

  it("--vao-thang refuses with the removal sentence", () => {
    const r = dryRun(["--vao-thang", "--domain=thangbinh-danang.vigov.vn", "--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/--vao-thang đã bỏ/);
  });

  it("--app-id that disagrees with the registry refuses", () => {
    const r = dryRun(["--domain=thangbinh-danang.vigov.vn", "--app-id=1", "--thu"]);
    expect(r.status).toBe(2);
    expect(r.stderr).toMatch(/khác App ID/);
  });
});
