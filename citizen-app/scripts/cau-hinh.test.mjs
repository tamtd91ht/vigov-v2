import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { docCauHinh, kiemTenBien, TEN_BIEN_CHO_PHEP, TEP_LOCAL } from "./cau-hinh.mjs";

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
  it("hai tên của bước dựng thì QUA", () => {
    expect(TEN_BIEN_CHO_PHEP).toEqual(["VIGOV_BIEN_THE", "VIGOV_API_HOST"]);
    expect(() =>
      kiemTenBien("# ghi chú\nVIGOV_API_HOST=https://vidu.test\n\nVIGOV_BIEN_THE=goc\n"),
    ).not.toThrow();
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
    const goc = thuMucCo("VIGOV_API_HOST=https://tu-tep.test\nVIGOV_BIEN_THE=goc\n");
    expect(process.env.VIGOV_API_HOST, "môi trường test đang có sẵn biến này").toBeUndefined();
    expect(docCauHinh(goc)).toEqual({
      VIGOV_BIEN_THE: "goc",
      VIGOV_API_HOST: "https://tu-tep.test",
    });
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

  it("TRẢ VỀ ĐÚNG HAI KHOÁ, không bao giờ trả cả môi trường", () => {
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
    expect(ma, "`docBienThe` không đọc từ cấu hình chung").toMatch(
      /docBienThe\(\)[^}]*cauHinh\(\)\.VIGOV_BIEN_THE/s,
    );

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
