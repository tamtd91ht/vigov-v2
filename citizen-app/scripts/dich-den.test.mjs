import { mkdtempSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { docCauHinh, TEN_BIEN_CHO_PHEP, TEP_LOCAL } from "./cau-hinh.mjs";
import {
  appIdTrongToken,
  chonDich,
  docCo,
  kiemBangAnhXa,
  kiemTenMien,
  kiemToken,
  laPlaceholder,
  appConfigChoLanDay,
  nhanPhienBan,
  tokenTrongTepEnv,
} from "./dich-den.mjs";
import { APP_ID_APP_CHUNG, APP_ID_THEO_TEN_MIEN } from "./ung-dung-theo-ten-mien.mjs";

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
    const DA_GIAO = {
      "thangbinh-danang.vigov.vn": "3291993990104489440", // owner, 2026-09-27
    };
    const that = Object.fromEntries(
      Object.entries(APP_ID_THEO_TEN_MIEN).filter(([, app_id]) => !laPlaceholder(app_id)),
    );
    expect(that).toEqual(DA_GIAO);
    for (const app_id of Object.values(that)) expect(app_id).toMatch(/^\d+$/);
    expect(APP_ID_APP_CHUNG).toBeNull();
  });

  it("khoá không phải tên miền trần thì DỪNG", () => {
    expect(() => kiemBangAnhXa({ "https://xa-a.vigov.example": "1" }, null)).toThrow(/scheme/);
  });

  it("App ID rỗng hoặc có khoảng trắng thì DỪNG", () => {
    expect(() => kiemBangAnhXa({ "xa-a.vigov.example": "" }, null)).toThrow(/rỗng/);
    expect(() => kiemBangAnhXa({ "xa-a.vigov.example": "11 11" }, null)).toThrow(/khoảng trắng/);
  });

  it("App ID app chung trùng App ID một xã thì DỪNG — `goc` sẽ đè lên app của xã", () => {
    expect(() => kiemBangAnhXa(BANG, "2222222222222222222")).toThrow(/đè lên app của xã/);
    expect(() => kiemBangAnhXa(BANG, "3333333333333333333")).not.toThrow();
  });
});

describe("cờ dòng lệnh", () => {
  it("không cờ nào là app chung, bản thử nghiệm", () => {
    expect(docCo([])).toEqual({ ten_mien: null, phat_hanh: false, chi_thu: false, vao_thang: false });
  });

  it("đọc đủ bốn cờ", () => {
    expect(docCo(["--domain=xa-a.vigov.example", "--vao-thang", "--phat-hanh", "--thu"])).toEqual({
      ten_mien: "xa-a.vigov.example",
      phat_hanh: true,
      chi_thu: true,
      vao_thang: true,
    });
  });

  it("`--vao-thang` không kèm `--domain` thì DỪNG — app chung không bao giờ nung một xã", () => {
    expect(() => docCo(["--vao-thang"])).toThrow(/cần --domain/);
    expect(() => docCo(["--vao-thang", "--phat-hanh"])).toThrow(/cần --domain/);
  });

  it("`--bien-the` (cờ đã bỏ 27/09/2026) thì DỪNG và nói vì sao — không lặng lẽ bị bỏ qua", () => {
    // Người gõ cờ ấy đang tin rằng họ chọn được nội dung bản dựng. Bỏ qua nó là để họ tin tiếp.
    for (const c of ["--bien-the=goc", "--bien-the=day-du", "--bien-the"]) {
      expect(() => docCo([c]), c).toThrow(/đã bỏ.*không còn biến thể/s);
    }
  });

  it("cờ gõ nhầm thì DỪNG, không lặng lẽ thành app chung", () => {
    expect(() => docCo(["--domian=xa-a.vigov.example"])).toThrow(/không có/);
    expect(() => docCo(["--domain", "xa-a.vigov.example"])).toThrow(/không có/);
  });

  it("`--domain=` để trống thì DỪNG — không hiểu thành app chung", () => {
    expect(() => docCo(["--domain="])).toThrow(/bỏ hẳn cờ --domain/);
  });

  it("`--domain` sai hình dạng thì DỪNG ngay lúc đọc cờ", () => {
    expect(() => docCo(["--domain=https://xa-a.vigov.example"])).toThrow(/scheme/);
  });
});

describe("chọn đích: tên miền → App ID", () => {
  it("không tên miền → app chung", () => {
    expect(chonDich(null, BANG, null)).toEqual({ loai: "app-chung", ten_mien: null, app_id: null });
    expect(chonDich(null, BANG, "3333333333333333333").app_id).toBe("3333333333333333333");
  });

  it("tên miền có trong bảng → App ID của nó; tên miền cũ trỏ cùng App ID", () => {
    expect(chonDich("xa-b.vigov.example", BANG, null)).toEqual({
      loai: "app-rieng",
      ten_mien: "xa-b.vigov.example",
      app_id: "2222222222222222222",
    });
    expect(chonDich("xa-a-cu.vigov.example", BANG, null).app_id).toBe("1111111111111111111");
  });

  it("tên miền lạ thì DỪNG, không rơi về app chung, và không liệt kê bảng", () => {
    let loi;
    try {
      chonDich("xa-z.vigov.example", BANG, "3333333333333333333");
    } catch (e) {
      loi = e;
    }
    expect(loi, "tên miền lạ vừa được chọn một đích").toBeDefined();
    expect(loi.message).toMatch(/ung-dung-theo-ten-mien\.mjs/);
    expect(loi.message).toMatch(/MiniApp/);
    for (const [ten, app_id] of Object.entries(BANG)) {
      expect(loi.message).not.toContain(ten);
      expect(loi.message).not.toContain(app_id);
    }
  });

  it("khoá kế thừa của Object (`constructor`, `__proto__`) không phải tên miền có trong bảng", () => {
    // `in` / `bang[x]` thấy cả nguyên mẫu; `Object.hasOwn` thì không. Hai tên dưới không qua
    // `kiemTenMien` nên không tới được đây từ dòng lệnh — ca này canh hàm, không canh cờ.
    expect(() => chonDich("constructor", BANG, null)).toThrow(/chưa có/);
  });
});

describe("token: đích thật là claim appId của ZMP_TOKEN", () => {
  const rieng_a = { loai: "app-rieng", ten_mien: "xa-a.vigov.example", app_id: "1111111111111111111" };
  const chung = { loai: "app-chung", ten_mien: null, app_id: null };

  it("đọc được claim appId; không phải JWT thì null", () => {
    expect(appIdTrongToken(tokenCua("1111111111111111111"))).toBe("1111111111111111111");
    expect(appIdTrongToken(tokenCua(1111))).toBe("1111");
    expect(appIdTrongToken("khong-phai-jwt")).toBeNull();
    expect(appIdTrongToken("a.!!!.c")).toBeNull();
    expect(appIdTrongToken(undefined)).toBeNull();
  });

  it("app riêng: token đúng App ID thì qua", () => {
    expect(kiemToken(rieng_a, tokenCua("1111111111111111111"), BANG).ok).toBe(true);
  });

  it("app riêng: token của xã khác thì TỪ CHỐI", () => {
    const kq = kiemToken(rieng_a, tokenCua("2222222222222222222"), BANG);
    expect(kq.ok).toBe(false);
    expect(kq.ly_do).toMatch(/2222222222222222222.*không phải 1111111111111111111/);
  });

  it("app riêng: không token trong môi trường thì TỪ CHỐI — không dùng `.env` của máy", () => {
    expect(kiemToken(rieng_a, undefined, BANG)).toMatchObject({ ok: false, ly_do: expect.stringMatching(/\.env/) });
    expect(kiemToken(rieng_a, "", BANG).ok).toBe(false);
  });

  it("app riêng: token không đọc được claim thì TỪ CHỐI", () => {
    expect(kiemToken(rieng_a, "khong-phai-jwt", BANG).ok).toBe(false);
  });

  it("app riêng: App ID đích còn placeholder thì TỪ CHỐI, kể cả token có claim trùng", () => {
    const vi_du = { loai: "app-rieng", ten_mien: "xa-vi-du.vigov.example", app_id: "<APP-ID>" };
    expect(kiemToken(vi_du, tokenCua("<APP-ID>"), BANG)).toMatchObject({ ok: false });
  });

  it("app chung: không token trong môi trường thì qua — đường cũ, zmp đọc .env", () => {
    expect(kiemToken(chung, undefined, BANG).ok).toBe(true);
  });

  it("app chung: token của một XÃ thì TỪ CHỐI — `goc` sẽ đè lên app của xã", () => {
    const kq = kiemToken(chung, tokenCua("2222222222222222222"), BANG);
    expect(kq.ok).toBe(false);
    expect(kq.ly_do).toMatch(/app RIÊNG/);
  });

  it("app chung đã khai App ID: token app khác thì TỪ CHỐI, đúng thì qua", () => {
    const chung_khai = { ...chung, app_id: "3333333333333333333" };
    expect(kiemToken(chung_khai, tokenCua("4444444444444444444"), BANG).ok).toBe(false);
    expect(kiemToken(chung_khai, tokenCua("3333333333333333333"), BANG).ok).toBe(true);
  });

  it("lý do không bao giờ chứa token", () => {
    const token = tokenCua("2222222222222222222");
    for (const dich of [rieng_a, chung]) {
      const { ly_do } = kiemToken(dich, token, BANG);
      expect(ly_do).not.toContain(token);
      expect(ly_do).not.toContain(token.split(".")[1]);
    }
  });
});

describe("nhãn phiên bản phân biệt được đích trên console Zalo", () => {
  const chung = { sha: "abc1234", luc: "2026-09-27 10:00", dirty: false };

  it("app chung và app riêng cùng commit ra hai nhãn khác nhau", () => {
    // Nhãn không còn mang tên biến thể (27/09/2026): bundle là một, chỉ ĐÍCH phân biệt hai lần đẩy.
    const a = nhanPhienBan({ ...chung, dich: { loai: "app-chung", ten_mien: null, app_id: null } });
    const b = nhanPhienBan({
      ...chung,
      dich: { loai: "app-rieng", ten_mien: "xa-a.vigov.example", app_id: "1111111111111111111" },
    });
    expect(a).toBe("app-chung · abc1234 · 2026-09-27 10:00");
    expect(b).toBe("xa-a.vigov.example · app 1111111111111111111 · abc1234 · 2026-09-27 10:00");
  });

  it("dirty vẫn hiện", () => {
    expect(
      nhanPhienBan({ ...chung, dirty: true, dich: { loai: "app-chung", ten_mien: null, app_id: null } }),
    ).toMatch(/ · dirty$/);
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

  it("`zmp login` chạy trong thư mục tạm — không bao giờ ghi đè `citizen-app/.env` của app chung", () => {
    const ma = doc("./deploy.mjs");
    const than = ma.slice(ma.indexOf("function dangNhapRieng("), ma.indexOf("function nghi("));
    expect(than).toMatch(/mkdtempSync\(/);
    expect(than).toMatch(/cwd: thu_muc/);
    expect(than).toMatch(/rmSync\(thu_muc/);
  });
});

describe("app-config.json cho một lần đẩy", () => {
  const goc = readFileSync(new URL("../app-config.json", import.meta.url), "utf8");

  it("không cờ thì nguyên văn — app chung không đổi một byte", () => {
    expect(appConfigChoLanDay(goc, false)).toBe(goc);
  });

  it("--vao-thang ẩn thanh tiêu đề Zalo, giữ mọi khoá khác", () => {
    const moi = JSON.parse(appConfigChoLanDay(goc, true));
    const cu = JSON.parse(goc);
    expect(moi.app.actionBarHidden).toBe(true);
    expect({ ...moi.app, actionBarHidden: undefined }).toEqual({ ...cu.app, actionBarHidden: undefined });
    expect(moi.listSyncJS).toEqual(cu.listSyncJS);
  });

  it("deploy.mjs trả tệp về nguyên văn trong `finally`", () => {
    const ma = readFileSync(new URL("./deploy.mjs", import.meta.url), "utf8");
    expect(ma).toMatch(/finally \{\s*if \(vao_thang\) writeFileSync\(TEP_APP_CONFIG, app_config_goc/);
  });
});
