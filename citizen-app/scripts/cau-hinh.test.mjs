import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { demoBuild, docCauHinh, kiemTenBien, TEN_BIEN_CHO_PHEP, TEP_LOCAL, xaCoDinh } from "./cau-hinh.mjs";

/**
 * CẤU HÌNH LÚC DỰNG — PHÉP KIỂM CỦA MỘT CÁI RÀO, KHÔNG PHẢI CỦA MỘT TIỆN ÍCH.
 *
 * Thứ cái rào ấy chặn: một khoá bí mật đặt vào `.env.local` theo phản xạ của người quen `.env`
 * phía máy chủ. Ở một app Vite, mọi giá trị đi qua `define:` được **nung thẳng vào bundle** và
 * đi lên Zalo — nên "đặt nhầm chỗ" ở đây nghĩa là **đã công bố**, không thu hồi được.
 *
 * ⚠ VÌ SAO PHẢI CÓ CA KIỂM CHỨ KHÔNG CHỈ MỘT DÒNG CẢNH BÁO TRONG `.env.local.example`: cảnh báo
 * bằng chữ thì người ta đọc một lần, vào ngày họ tạo tệp. Cái rào thì đứng đó mãi — và ca kiểm
 * là thứ giữ cho cái rào không bị gỡ trong một lượt "dọn dẹp" nào đó về sau.
 *
 * Tệp này là `.mjs` có chủ đích: `tsconfig.json` chỉ gồm `src/`, còn mô-đun được kiểm nằm ở
 * `scripts/` và được `vite.config.ts` nhập — một tệp `.ts` ở đây sẽ nằm ngoài `tsc` mà vẫn giả
 * vờ được kiểm kiểu.
 */

/** Thư mục tạm có sẵn một `.env.local`. Không bao giờ đụng tệp thật của người đang chạy test. */
function thuMucCo(noi_dung) {
  const goc = mkdtempSync(join(tmpdir(), "vigov-cau-hinh-"));
  writeFileSync(join(goc, TEP_LOCAL), noi_dung, "utf8");
  // `loadEnv` nhận một đường dẫn thư mục; `docCauHinh` ghép URL nên cần dấu phân cách cuối.
  return goc.endsWith("\\") || goc.endsWith("/") ? goc : `${goc}/`;
}

describe("danh sách trắng: tên nào được phép nằm trong .env.local", () => {
  it("tên duy nhất của bước dựng thì QUA", () => {
    // ĐÚNG MỘT TÊN (27/09/2026). `VIGOV_BIEN_THE` rời danh sách cùng hai biến thể bản dựng.
    expect(TEN_BIEN_CHO_PHEP).toEqual(["VIGOV_API_HOST"]);
    expect(() => kiemTenBien("# ghi chú\nVIGOV_API_HOST=https://vidu.test\n\n")).not.toThrow();
  });

  it("`VIGOV_BIEN_THE` còn sót trong một .env.local cũ thì DỪNG, và nói phải làm gì", () => {
    // Mẫu `.env.local.example` trước 27/09 có dòng ấy, nên mọi máy đã chép mẫu đều còn nó. Bỏ qua
    // nó là để người đặt tiếp tục tin rằng nó chọn được nội dung bản dựng.
    let loi;
    try {
      kiemTenBien("VIGOV_API_HOST=https://vidu.test\nVIGOV_BIEN_THE=day-du\n");
    } catch (e) {
      loi = e;
    }
    expect(loi, "tên đã bỏ vừa lọt qua").toBeDefined();
    expect(loi.message).toMatch(/VIGOV_BIEN_THE đã bỏ/);
    expect(loi.message).toMatch(/Xoá dòng ấy/);
  });

  it("một tên LẠ thì NÉM LỖI, và lỗi nói ra hậu quả chứ không chỉ nói 'không hợp lệ'", () => {
    // `ZALO_MINIAPP_SECRET_KEY` là ví dụ thật của cái bẫy: khoá bí mật của Mini App chỉ được
    // nằm ở backend (ADR 0020, bất biến 2 · luật 8, cấm #5). Đặt vào đây là đưa nó lên Zalo.
    let loi;
    try {
      kiemTenBien("VIGOV_API_HOST=https://vidu.test\nZALO_MINIAPP_SECRET_KEY=abc123\n");
    } catch (e) {
      loi = e;
    }
    expect(loi, "một tên ngoài danh sách trắng vừa lọt qua").toBeDefined();
    expect(loi.message).toContain("ZALO_MINIAPP_SECRET_KEY");
    // Thông điệp phải dạy được người đọc, vì người gặp nó đang định làm một việc nguy hiểm.
    expect(loi.message, "không nói ra vì sao nguy hiểm").toMatch(/NUNG THẲNG vào bundle/);
    expect(loi.message, "không chỉ ra chỗ ĐÚNG cho một bí mật dòng lệnh").toMatch(/ZMP_TOKEN/);
  });

  it("bắt được cả biến viết kiểu `export`, và bỏ qua ghi chú / dòng trống", () => {
    expect(() => kiemTenBien("export AWS_SECRET_ACCESS_KEY=x\n")).toThrow(/AWS_SECRET_ACCESS_KEY/);
    expect(() => kiemTenBien("\n\n# VIGOV_API_HOST=bi-ghi-chu\n")).not.toThrow();
    expect(() => kiemTenBien("   VIGOV_API_HOST=https://vidu.test   \n")).not.toThrow();
  });
});

describe("đọc cấu hình: tệp local, biến shell, và không gì khác", () => {
  it("đọc được giá trị từ .env.local — thứ mà `process.env` KHÔNG tự thấy", () => {
    // Đây là chính cái bẫy đã làm tệp local "không chạy": Vite không nạp `.env.local` vào
    // `process.env`, chỉ `loadEnv` mới đọc các tệp `.env*`.
    const goc = thuMucCo("VIGOV_API_HOST=https://tu-tep.test\n");
    expect(process.env.VIGOV_API_HOST, "môi trường test đang có sẵn biến này").toBeUndefined();
    expect(docCauHinh(goc)).toEqual({ VIGOV_API_HOST: "https://tu-tep.test" });
  });

  it("biến shell THẮNG tệp — CI và một lần đẩy tay phải đè được tệp local", () => {
    const goc = thuMucCo("VIGOV_API_HOST=https://tu-tep.test\n");
    const truoc = process.env.VIGOV_API_HOST;
    process.env.VIGOV_API_HOST = "https://tu-shell.test";
    try {
      expect(docCauHinh(goc).VIGOV_API_HOST).toBe("https://tu-shell.test");
    } finally {
      if (truoc === undefined) delete process.env.VIGOV_API_HOST;
      else process.env.VIGOV_API_HOST = truoc;
    }
  });

  it("KHÔNG có .env.local cũng chạy được — CI chỉ dùng biến shell", () => {
    const goc = mkdtempSync(join(tmpdir(), "vigov-khong-tep-"));
    expect(() => docCauHinh(`${goc}/`)).not.toThrow();
  });

  it("TRẢ VỀ ĐÚNG CÁC KHOÁ CỦA DANH SÁCH TRẮNG, không bao giờ trả cả môi trường", () => {
    // ĐÂY LÀ RÀO THỨ HAI, VÀ NÓ CHẶN MỘT ĐƯỜNG RÒ KHÁC HẲN. `loadEnv(…, "")` gom TOÀN BỘ
    // `process.env` — đã đo: 87 khoá trên máy dựng hôm nay, trong đó `citizen-app/.env` (của
    // `zmp-cli`) có `ZMP_TOKEN`. Nếu hàm này trả nguyên đống ấy ra, một lượt sửa "cho tiện"
    // kiểu `define: { ...docCauHinh() }` sẽ nung token đăng nhập Zalo vào bundle gửi duyệt.
    // Danh sách trắng chỉ canh TÊN trong tệp; ca này canh BỀ MẶT TRẢ VỀ.
    const goc = thuMucCo("VIGOV_API_HOST=https://tu-tep.test\n");
    const truoc = process.env.ZMP_TOKEN;
    process.env.ZMP_TOKEN = "gia-lap-token-khong-duoc-lot";
    try {
      const cau_hinh = docCauHinh(goc);
      expect(Object.keys(cau_hinh).sort()).toEqual([...TEN_BIEN_CHO_PHEP].sort());
      expect(JSON.stringify(cau_hinh)).not.toContain("gia-lap-token-khong-duoc-lot");
    } finally {
      if (truoc === undefined) delete process.env.ZMP_TOKEN;
      else process.env.ZMP_TOKEN = truoc;
    }
  });
});

describe("cái rào thật sự được nối vào hai chỗ dùng", () => {
  // Một mô-đun đúng mà không ai gọi là một mô-đun không canh gì cả. Hai ca dưới đọc mã nguồn
  // thật của hai tệp gọi nó — cùng lối với `bien-the.test.ts` đọc `vite.config.ts?raw`.
  const doc = (duong) => readFileSync(new URL(duong, import.meta.url), "utf8");

  it("`vite.config.ts` lấy GIÁ TRỊ qua mô-đun này, không lấy thẳng từ `process.env`", () => {
    const ma = doc("../vite.config.ts");
    expect(ma).toMatch(/from "\.\/scripts\/cau-hinh\.mjs"/);

    // Kiểm hai hàm SINH RA GIÁ TRỊ, không kiểm "tệp có nhắc process.env hay không": nhắc thì
    // vẫn có chỗ chính đáng (khoá cache đọc biến shell để biết khi nào phải đọc lại đĩa). Một
    // ca cấm theo chuỗi sẽ đỏ vì đúng dòng hợp lệ ấy, và rồi bị nới cho qua.
    expect(ma, "`diaChiMayChu` không đọc từ cấu hình chung").toMatch(
      /diaChiMayChu\(\)[^}]*cauHinh\(\)\.VIGOV_API_HOST/s,
    );
    // Và không còn đọc biến thể nào (27/09/2026): một `VIGOV_BIEN_THE` trở lại tệp này là hai bản
    // dựng trở lại — thứ chủ sản phẩm đã bỏ.
    expect(ma, "`vite.config.ts` lại đọc VIGOV_BIEN_THE").not.toMatch(/\.VIGOV_BIEN_THE\b|\[["']VIGOV_BIEN_THE["']\]/);
    expect(ma, "`vite.config.ts` lại có `resolve.alias`").not.toMatch(/\balias\s*:/);

    // Và `define` chỉ được nhận ĐÚNG MỘT khoá đã đặt tên. Một ngày nào đó `define: { ...env }`
    // là ngày mọi biến môi trường của máy dựng đi vào bundle gửi lên Zalo.
    const define = /define:\s*\{([^}]*)\}/.exec(ma)?.[1] ?? "";
    expect(define, "không tìm thấy khối define trong vite.config.ts").not.toBe("");
    expect(define, "`define` đang trải cả một đối tượng vào bundle").not.toContain("...");
    expect(define).toContain("__VIGOV_API_HOST__");
  });

  it("`deploy.mjs` đọc CÙNG nguồn ấy — nó chạy ngoài Vite nên không tự thấy tệp", () => {
    const ma = doc("./deploy.mjs");
    expect(ma).toMatch(/from "\.\/cau-hinh\.mjs"/);
    expect(ma).toMatch(/docCauHinh\(\)\.VIGOV_API_HOST/);
  });

  it("`.env.local.example` mở đầu bằng cảnh báo, và chỉ chứa placeholder", () => {
    const vidu = doc("../.env.local.example");
    expect(vidu.split("\n")[0], "dòng đầu không phải cảnh báo").toMatch(/KHÔNG PHẢI CHỖ ĐỂ BÍ MẬT/);
    expect(vidu).toMatch(/cp \.env\.local\.example \.env\.local/);
    // Placeholder, không phải một địa chỉ thật (luật 8, bất biến 6).
    expect(vidu).toMatch(/VIGOV_API_HOST=https:\/\/<[^>]+>/);
    for (const ten of TEN_BIEN_CHO_PHEP) expect(vidu).toContain(ten);
  });
});

describe("xã cố định của bản dựng (`--vao-thang`, 27/09/2026)", () => {
  it("không đặt thì rỗng — mọi bản dựng thường là app chung", () => {
    expect(xaCoDinh({})).toBe("");
    expect(xaCoDinh({ VIGOV_XA_CO_DINH: "  " })).toBe("");
  });

  it("tên miền trần thì nhận nguyên", () => {
    expect(xaCoDinh({ VIGOV_XA_CO_DINH: "xa-a.vigov.example" })).toBe("xa-a.vigov.example");
  });

  it.each(["https://xa-a.vigov.example", "Xa-A.vigov.example", "localhost", "xa-a.vigov.example/x"])(
    "%s thì DỪNG bước dựng — không nung một tên miền hỏng",
    (gia_tri) => {
      expect(() => xaCoDinh({ VIGOV_XA_CO_DINH: gia_tri })).toThrow(/tên miền trần/);
    },
  );

  it("đặt trong `.env.local` thì CHẶN — chỉ deploy.mjs được đặt nó, cho một lần dựng", () => {
    expect(() => kiemTenBien("VIGOV_XA_CO_DINH=xa-a.vigov.example\n")).toThrow(/--vao-thang/);
  });

  it("`deploy.mjs` xoá biến khỏi môi trường dựng khi không có cờ, và chỉ đặt nó bằng `--domain`", () => {
    const ma = readFileSync(new URL("./deploy.mjs", import.meta.url), "utf8");
    expect(ma).toMatch(/delete env_dung\[BIEN_XA_CO_DINH\]/);
    expect(ma).toMatch(/if \(vao_thang\) env_dung\[BIEN_XA_CO_DINH\] = dich\.ten_mien;/);
    expect(ma).toMatch(/dung\(env_dung\)/);
  });
});

describe("demo build of the commune app (`--vao-thang --demo`, owner 30/09/2026)", () => {
  const COMMUNE = { VIGOV_XA_CO_DINH: "xa-a.vigov.example" };

  it("unset (or blank) → false: every ordinary build is not a demo", () => {
    expect(demoBuild({})).toBe(false);
    expect(demoBuild({ ...COMMUNE, VIGOV_DEMO: "  " })).toBe(false);
  });

  it("\"1\" with a commune baked in → true", () => {
    expect(demoBuild({ ...COMMUNE, VIGOV_DEMO: "1" })).toBe(true);
  });

  it("without VIGOV_XA_CO_DINH the BUILD STOPS — the shared app has no commune screens", () => {
    expect(() => demoBuild({ VIGOV_DEMO: "1" })).toThrow(/--vao-thang/);
  });

  it.each(["true", "yes", "0", "2"])("VIGOV_DEMO=%s STOPS the build — only \"1\", only from deploy.mjs", (value) => {
    expect(() => demoBuild({ ...COMMUNE, VIGOV_DEMO: value })).toThrow(/chỉ nhận "1"/);
  });

  it("set in `.env.local` → BLOCKED: one forgotten line would make every build on the machine a demo", () => {
    expect(() => kiemTenBien("VIGOV_DEMO=1\n")).toThrow(/--demo/);
  });

  it("`deploy.mjs` removes it from the build environment and sets it only with `--demo`", () => {
    const code = readFileSync(new URL("./deploy.mjs", import.meta.url), "utf8");
    expect(code).toMatch(/delete env_dung\[DEMO_BUILD_VAR\]/);
    expect(code).toMatch(/if \(demo\) env_dung\[DEMO_BUILD_VAR\] = "1";/);
  });

  it("`vite.config.ts` bakes it through `define`, next to the commune domain", () => {
    const code = readFileSync(new URL("../vite.config.ts", import.meta.url), "utf8");
    expect(code).toMatch(/__VIGOV_DEMO__: JSON\.stringify\(demoBuild\(\)\)/);
  });
});
