import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * A FAILED CALL TO ViGov — which branch it becomes, and that nothing of it reaches the console. The branch is
 * the citizen's next step ("check your network" vs "the system is busy"), so a wrong one tells them to fix the
 * wrong thing; and a console line would carry a citizen's name, phone or petition text into Zalo's debug log.
 */
vi.mock("./phien-vigov", () => ({ layPhienViGov: () => ({ token: "tok-bi-mat", ten_xa: "Xã Thử" }) }));
vi.mock("./dia-chi-vigov", () => ({ diaChiViGov: (_s: string, d: string) => `https://petitions.vidu.vn${d}` }));

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

const consoleSpies = () => (["log", "info", "warn", "error", "debug"] as const).map((k) => vi.spyOn(console, k));

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("a failed call", () => {
  it("a network failure on send is `loi-mang`, and nothing is written to the console", async () => {
    const spies = consoleSpies();
    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("Failed to fetch")));
    expect(await guiPhanAnh(lan())).toEqual({ kieu: "loi-mang" });
    for (const s of spies) expect(s).not.toHaveBeenCalled();
  });

  it("a 201 whose body is not JSON is a server failure, not 'check your network'", async () => {
    const spies = consoleSpies();
    vi.stubGlobal("fetch", () =>
      Promise.resolve({
        status: 201,
        json: async () => {
          throw new SyntaxError('Unexpected token \'<\', "<html>0900000000" is not valid JSON');
        },
      }),
    );
    expect(await guiPhanAnh(lan())).toEqual({ kieu: "loi-may-chu" });
    for (const s of spies) expect(s).not.toHaveBeenCalled();
  });

  it("a 201 whose body is cut off mid-read (timeout, dropped link) is still the network", async () => {
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

  it("a 500 on lookup is `loi-may-chu`, and nothing is written to the console", async () => {
    const spies = consoleSpies();
    vi.stubGlobal("fetch", () => Promise.resolve({ status: 500, json: async () => ({}) }));
    expect(await traCuuPhieu("PA7K2QX9M4TD")).toEqual({ kieu: "loi-may-chu" });
    for (const s of spies) expect(s).not.toHaveBeenCalled();
  });
});
