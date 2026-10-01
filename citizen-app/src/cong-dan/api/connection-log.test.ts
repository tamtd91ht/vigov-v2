import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * THE `--demo` DIAGNOSTIC LOG — what reaches the console when a call to ViGov fails, and what never does.
 * The flag is a compile-time constant, so the `--demo` side is reached by mocking the module.
 */
const flag = vi.hoisted(() => ({ demo: true }));
vi.mock("../../lib/demo-build", () => ({
  get DEMO_BUILD() {
    return flag.demo;
  },
}));
vi.mock("./phien-vigov", () => ({ layPhienViGov: () => ({ token: "tok-bi-mat", ten_xa: "Xã Thử" }) }));
vi.mock("./dia-chi-vigov", () => ({ diaChiViGov: (_s: string, d: string) => `https://petitions.vidu.vn${d}` }));

import { describeThrown } from "./connection-log";
import { guiPhanAnh, traCuuPhieu } from "./goi-vigov";
import { taoLanGui } from "./lan-gui";
import { thanGuiPhanAnh } from "./hop-dong-phan-anh";

const lan = () =>
  taoLanGui(
    thanGuiPhanAnh({
      noi_dung: "Ổ gà lớn trước cổng chợ",
      dia_chi: "Đầu ngõ thôn Hà Lam",
      ho_ten: "Nguyễn Văn Hùng",
      dien_thoai: "0900000000",
      an_danh: false,
    }),
  );

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  flag.demo = true;
});

describe("connection log", () => {
  it("a network failure on send: one console.warn, with the route, the error and no personal data", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("Failed to fetch")));
    expect(await guiPhanAnh(lan())).toEqual({ kieu: "loi-mang" });
    expect(warn).toHaveBeenCalledTimes(1);
    const record = warn.mock.calls[0]![1] as Record<string, unknown>;
    expect(record).toMatchObject({
      route: "send-petition",
      method: "POST",
      host: "petitions.vidu.vn",
      outcome: "loi-mang",
      error: "TypeError: Failed to fetch",
    });
    const printed = JSON.stringify(warn.mock.calls);
    for (const secret of ["0900000000", "Nguyễn Văn Hùng", "Ổ gà", "Hà Lam", "tok-bi-mat", "/api/"]) {
      expect(printed, secret).not.toContain(secret);
    }
  });

  it("a 201 whose body is not JSON is a server failure, not 'check your network' — and its body is not quoted", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.stubGlobal("fetch", () =>
      Promise.resolve({
        status: 201,
        json: async () => {
          throw new SyntaxError('Unexpected token \'<\', "<html>0900000000" is not valid JSON');
        },
      }),
    );
    expect(await guiPhanAnh(lan())).toEqual({ kieu: "loi-may-chu" });
    const record = warn.mock.calls[0]![1] as Record<string, unknown>;
    expect(record).toMatchObject({ status: 201, error: "SyntaxError", outcome: "loi-may-chu" });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("0900000000");
  });

  it("a 201 whose body is cut off mid-read (timeout, dropped link) is still the network", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    for (const err of [Object.assign(new Error("aborted"), { name: "AbortError" }), new TypeError("network error")]) {
      vi.stubGlobal("fetch", () =>
        Promise.resolve({
          status: 201,
          json: async () => {
            throw err;
          },
        }),
      );
      expect(await guiPhanAnh(lan()), err.name).toEqual({ kieu: "loi-mang" });
    }
  });

  it("the lookup code is in the path, and the path is never logged", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.stubGlobal("fetch", () => Promise.resolve({ status: 500, json: async () => ({}) }));
    expect(await traCuuPhieu("PA7K2QX9M4TD")).toEqual({ kieu: "loi-may-chu" });
    expect(warn.mock.calls[0]![1]).toMatchObject({ route: "lookup-petition", status: 500 });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PA7K2QX9M4TD");
  });

  it("outside the --demo build nothing is written", async () => {
    flag.demo = false;
    const spies = (["log", "info", "warn", "error", "debug"] as const).map((k) => vi.spyOn(console, k));
    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("Failed to fetch")));
    expect(await guiPhanAnh(lan())).toEqual({ kieu: "loi-mang" });
    for (const s of spies) expect(s).not.toHaveBeenCalled();
  });

  it("describeThrown keeps a message only for network failures", () => {
    expect(describeThrown(new SyntaxError("body text here"))).toBe("SyntaxError");
    expect(describeThrown(Object.assign(new Error("x"), { name: "AbortError" }))).toBe("AbortError: x");
    expect(describeThrown("raw")).toBe("string");
  });
});
