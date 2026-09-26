import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * LỚP GỌI ViGov VỚI MỘT PHIÊN GIẢ — chỉ trong tệp này.
 *
 * `vi.mock` thay `phien-vigov` và `dia-chi-vigov` cho RIÊNG tệp test này: mã sản phẩm vẫn chỉ có một
 * nguồn phiên, và nguồn ấy trả `null` (`cong-dan.test.tsx` khẳng định điều đó trên mã thật). Không
 * có khe tham số nào trong mã sản phẩm để "đưa" một phiên vào — đó là lý do phải giả lập ở tầng mô-đun.
 */
const trang = vi.hoisted(() => ({
  phien: { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" } as { token: string; ten_xa: string } | null,
  host: "https://vigov.vidu.vn",
}));

vi.mock("./phien-vigov", () => ({ layPhienViGov: () => trang.phien }));
vi.mock("./dia-chi-vigov", () => ({
  diaChiViGov: (_dich_vu: "petitions", d: string) => (trang.host === "" ? "" : `${trang.host}${d}`),
}));

import { KetQuaGui } from "../man/GuiPhanAnhScreen";
import { CUA_TOI, GUI, THE_PHIEU, TRA_CUU, TRANG_THAI } from "../man/noi-dung";
import {
  batDauTai,
  DANH_SACH_DAU,
  PhanAnhCuaToiScreen,
  sauKhiTai,
  ThanDanhSach,
  ThePhieuTomTat,
} from "../man/PhanAnhCuaToiScreen";
import { KetQuaTraCuu, TraCuuPhieuScreen } from "../man/TraCuuPhieuScreen";

import { guiPhanAnh, phanAnhCuaToi, traCuuPhieu } from "./goi-vigov";
import {
  DO_DAI_NHANH_KET_THUC,
  docPhieu,
  docTrangPhieuCuaToi,
  DUONG_DAN_PHAN_ANH_CUA_TOI,
  type PhanAnhMoi,
  type PhieuCuaToiTomTat,
  SO_DONG_MOI_TRANG,
  thanGuiPhanAnh,
  TRUONG_DUOC_NHAN,
} from "./hop-dong-phan-anh";
import { taoLanGui } from "./lan-gui";

type LoiGoi = { dia_chi: string; tuy_chon: RequestInit };

let loi_goi: LoiGoi[] = [];

/** Một phản hồi giả. Không dùng `Response` để khỏi phụ thuộc môi trường Node có hay không. */
function traLoi(status: number, than: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => than };
}

function datFetch(...phan_hoi: Array<ReturnType<typeof traLoi> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (dia_chi: string, tuy_chon: RequestInit) => {
    loi_goi.push({ dia_chi, tuy_chon });
    const p = phan_hoi[Math.min(i++, phan_hoi.length - 1)]!;
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const tieuDe = (g: LoiGoi) => g.tuy_chon.headers as Record<string, string>;

const PHIEU_RA = {
  code: "PA7K2QX9M4TD",
  channel: "zalo-mini-app",
  status: "da-tiep-nhan",
  field: "",
  field_label: "",
  content: "Ổ gà lớn trước cổng chợ",
  address: "Đầu ngõ thôn Hà Lam",
  reporter_name: "Nguyễn V. A.",
  reporter_phone: "09****0000",
  anonymous: false,
  clock_from: "2026-09-24T01:30:00Z",
  acknowledge_due: "2026-09-24T03:30:00Z",
  resolve_due: null,
  result: "",
};

const PA: PhanAnhMoi = {
  noi_dung: "  Ổ gà lớn trước cổng chợ  ",
  dia_chi: "Đầu ngõ thôn Hà Lam",
  ho_ten: "Nguyễn Văn A",
  dien_thoai: "0900000000",
  an_danh: false,
};

beforeEach(() => {
  loi_goi = [];
  trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
  trang.host = "https://vigov.vidu.vn";
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("gửi phản ánh — khoá chống trùng, bearer, thân", () => {
  it("POST đúng tuyến, bearer lấy từ nguồn phiên, có Idempotency-Key", async () => {
    datFetch(traLoi(201, PHIEU_RA));
    const kq = await guiPhanAnh(taoLanGui(thanGuiPhanAnh(PA)));

    expect(kq.kieu).toBe("xong");
    expect(loi_goi).toHaveLength(1);
    expect(loi_goi[0]!.dia_chi).toBe(`https://vigov.vidu.vn${DUONG_DAN_PHAN_ANH_CUA_TOI}`);
    expect(loi_goi[0]!.tuy_chon.method).toBe("POST");
    expect(tieuDe(loi_goi[0]!)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    expect(tieuDe(loi_goi[0]!)["Idempotency-Key"]).toMatch(/^[0-9a-f]{32}$/);
    // Dữ liệu cá nhân không bao giờ nằm trên đường dẫn.
    expect(loi_goi[0]!.dia_chi).not.toContain("0900000000");
  });

  it("GỬI LẠI cùng lần gửi: cùng khoá, cùng thân. Lần gửi MỚI: khoá mới", async () => {
    datFetch(new Error("mất mạng"), traLoi(201, PHIEU_RA), traLoi(201, PHIEU_RA));
    const lan = taoLanGui(thanGuiPhanAnh(PA));

    expect(await guiPhanAnh(lan)).toEqual({ kieu: "loi-mang" });
    expect((await guiPhanAnh(lan)).kieu).toBe("xong");
    const lan_moi = taoLanGui(thanGuiPhanAnh(PA));
    await guiPhanAnh(lan_moi);

    const khoa = loi_goi.map((g) => tieuDe(g)["Idempotency-Key"]);
    expect(khoa[0]).toBe(khoa[1]);
    expect(loi_goi[0]!.tuy_chon.body).toBe(loi_goi[1]!.tuy_chon.body);
    expect(khoa[2]).not.toBe(khoa[0]);
  });

  it("thân mang ĐÚNG năm trường máy chủ nhận — không lĩnh vực, không xã, không người gửi", () => {
    const than = JSON.parse(thanGuiPhanAnh(PA)) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual([...TRUONG_DUOC_NHAN].sort());
    for (const cam of [
      "field",
      "linh_vuc",
      "tenant",
      "tenant_id",
      "commune",
      "xa",
      "citizen_id",
      "cong_dan_id",
      "channel",
      "code",
      "status",
      "clock_from",
      "acknowledge_due",
      "resolve_due",
    ]) {
      expect(than, `thân gửi đi mang trường bị từ chối: ${cam}`).not.toHaveProperty(cam);
    }
    expect(than["content"]).toBe("Ổ gà lớn trước cổng chợ");
  });

  it("gửi ẩn danh thì KHÔNG gửi họ tên và số điện thoại", () => {
    const than = JSON.parse(thanGuiPhanAnh({ ...PA, an_danh: true })) as Record<string, unknown>;
    expect(than["anonymous"]).toBe(true);
    expect(than["reporter_name"]).toBe("");
    expect(than["reporter_phone"]).toBe("");
  });

  it("201 vẽ ra MÃ TRA CỨU, tình trạng, và hạn xem phiếu theo giờ Việt Nam", async () => {
    datFetch(traLoi(201, PHIEU_RA));
    const kq = await guiPhanAnh(taoLanGui(thanGuiPhanAnh(PA)));
    if (kq.kieu !== "xong") throw new Error(`mong đợi xong, nhận ${kq.kieu}`);

    const html = renderToStaticMarkup(createElement(KetQuaGui, { phieu: kq.phieu, onGuiKhac: () => {} }));
    expect(html).toContain("PA7K2QX9M4TD");
    expect(html).toContain("Đã tiếp nhận");
    // 03:30Z = 10:30 giờ Việt Nam.
    expect(html).toContain(GUI.se_xem_truoc("24/09/2026 10:30"));
    // Chỉ những gì máy chủ trả: số điện thoại ĐÃ CHE, không phải số người dân gõ.
    expect(html).toContain("09****0000");
    expect(html).not.toContain("0900000000");
  });

  it("mỗi mã trạng thái rơi vào đúng một nhánh", async () => {
    for (const [status, kieu] of [
      [400, "khong-hop-le"],
      [401, "het-phien"],
      [409, "dang-xu-ly-truoc"],
      [503, "kenh-chua-mo"],
      [500, "loi-may-chu"],
      [502, "loi-may-chu"],
    ] as const) {
      datFetch(traLoi(status, { code: "x", message: "y", trace_id: "" }));
      expect((await guiPhanAnh(taoLanGui("{}"))).kieu, `mã ${status}`).toBe(kieu);
    }
    datFetch(traLoi(201, { code: "" }));
    expect((await guiPhanAnh(taoLanGui("{}"))).kieu, "201 sai khuôn").toBe("loi-may-chu");
  });

  it("không phiên hoặc không địa chỉ: dừng TRƯỚC fetch", async () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);

    trang.phien = null;
    expect(await guiPhanAnh(taoLanGui("{}"))).toEqual({ kieu: "chua-co-phien" });
    expect(await traCuuPhieu("PA7K2QX9M4TD")).toEqual({ kieu: "chua-co-phien" });

    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    trang.host = "";
    expect(await guiPhanAnh(taoLanGui("{}"))).toEqual({ kieu: "chua-cau-hinh" });
    expect(await traCuuPhieu("PA7K2QX9M4TD")).toEqual({ kieu: "chua-cau-hinh" });

    expect(fetch_gia).not.toHaveBeenCalled();
  });
});

describe("tra cứu phiếu", () => {
  it("GET đúng tuyến với mã đã mã hoá, bearer, KHÔNG có Idempotency-Key", async () => {
    datFetch(traLoi(200, PHIEU_RA));
    const kq = await traCuuPhieu("  PA7K2QX9M4TD ");
    expect(kq.kieu).toBe("xong");
    expect(loi_goi[0]!.dia_chi).toBe(`https://vigov.vidu.vn${DUONG_DAN_PHAN_ANH_CUA_TOI}/PA7K2QX9M4TD`);
    expect(loi_goi[0]!.tuy_chon.method).toBe("GET");
    expect(tieuDe(loi_goi[0]!)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    expect(tieuDe(loi_goi[0]!)).not.toHaveProperty("Idempotency-Key");

    datFetch(traLoi(404, {}));
    await traCuuPhieu("a/../b");
    expect(loi_goi[1]!.dia_chi).toBe(`https://vigov.vidu.vn${DUONG_DAN_PHAN_ANH_CUA_TOI}/a%2F..%2Fb`);
  });

  it("404 là MỘT câu trung tính — bất kể máy chủ nói gì trong thân", async () => {
    datFetch(traLoi(404, { code: "not_found", message: "Không tìm thấy phiếu phản ánh.", trace_id: "t1" }));
    const mot = await traCuuPhieu("MA-KHONG-CO");
    datFetch(traLoi(404, { code: "khac", message: "một câu khác hẳn", trace_id: "t2" }));
    const hai = await traCuuPhieu("MA-CUA-NGUOI-KHAC");
    expect(mot).toEqual({ kieu: "khong-thay" });
    expect(hai).toEqual(mot);

    const ve = (kq: typeof mot) => renderToStaticMarkup(createElement(KetQuaTraCuu, { kq }));
    expect(ve(mot)).toContain(TRA_CUU.khong_thay);
    expect(ve(hai)).toBe(ve(mot));
    expect(ve(mot)).not.toContain("một câu khác hẳn");
  });

  it("phiếu đã đóng hiện kết quả của xã, hai hạn theo +07, và không lộ trường nội bộ", async () => {
    datFetch(
      traLoi(200, {
        ...PHIEU_RA,
        status: "da-dong",
        field: "giao-thong",
        field_label: "",
        resolve_due: "2026-09-26T09:00:00Z",
        result: "Đã vá ổ gà ngày 25/09.",
        assignee: "01JCANBONOIBO000000000000",
        note: "ghi chú nội bộ của cán bộ",
      }),
    );
    const kq = await traCuuPhieu("PA7K2QX9M4TD");
    const html = renderToStaticMarkup(createElement(KetQuaTraCuu, { kq }));
    expect(html).toContain("Đã đóng");
    expect(html).toContain("Đã vá ổ gà ngày 25/09.");
    expect(html).toContain("26/09/2026 16:00");
    expect(html).toContain("24/09/2026 10:30");
    // Mã lĩnh vực thô không hiện; trường máy chủ lỡ gửi thêm không đi tới màn hình.
    expect(html).not.toContain("giao-thong");
    expect(html).not.toContain("01JCANBONOIBO");
    expect(html).not.toContain("ghi chú nội bộ");
  });

  it("phiếu ẩn danh không hiện họ tên hay số, kể cả đã che", async () => {
    datFetch(traLoi(200, { ...PHIEU_RA, anonymous: true, reporter_name: "", reporter_phone: "" }));
    const html = renderToStaticMarkup(
      createElement(KetQuaTraCuu, { kq: await traCuuPhieu("PA7K2QX9M4TD") }),
    );
    expect(html).toContain("Gửi ẩn danh");
    expect(html).not.toContain("09****");
  });
});

/**
 * HAI NHÁNH KẾT THÚC — `reason` và `receiving_body` (migration 0011, `phieu_cua_toi.go`). Hai trường
 * TUỲ CHỌN, chỉ có nghĩa ở `khong-tiep-nhan` (lý do) và `chuyen-cap-tren` (lý do + cơ quan nhận).
 */
describe("tra cứu phiếu — lý do và cơ quan nhận của hai nhánh kết thúc", () => {
  const LY_DO = "Việc thuộc thẩm quyền của Ban quản lý khu công nghiệp.";
  const CO_QUAN = "Ban quản lý các khu công nghiệp tỉnh";

  async function veTra(than: unknown): Promise<string> {
    datFetch(traLoi(200, than));
    const kq = await traCuuPhieu("PA7K2QX9M4TD");
    expect(kq.kieu).toBe("xong");
    return renderToStaticMarkup(createElement(KetQuaTraCuu, { kq }));
  }

  it("parser nhận hai trường khi có, và coi vắng mặt là rỗng", () => {
    const tu_choi = docPhieu({ ...PHIEU_RA, status: "khong-tiep-nhan", reason: LY_DO });
    expect(tu_choi?.ly_do).toBe(LY_DO);
    expect(tu_choi?.co_quan_nhan).toBe("");

    const chuyen = docPhieu({ ...PHIEU_RA, status: "chuyen-cap-tren", reason: LY_DO, receiving_body: CO_QUAN });
    expect(chuyen?.ly_do).toBe(LY_DO);
    expect(chuyen?.co_quan_nhan).toBe(CO_QUAN);

    // Phiếu cũ, không có hai khoá: vẫn là một phiếu hợp lệ.
    const cu = docPhieu(PHIEU_RA);
    expect(cu).not.toBeNull();
    expect(cu?.ly_do).toBe("");
    expect(cu?.co_quan_nhan).toBe("");
  });

  it("parser từ chối sai kiểu, kể cả ở trạng thái không phải nhánh", () => {
    for (const status of ["khong-tiep-nhan", "chuyen-cap-tren", "da-tiep-nhan"]) {
      for (const sai of [null, 42, true, ["x"], { vi: "x" }]) {
        expect(docPhieu({ ...PHIEU_RA, status, reason: sai }), `${status} reason=${String(sai)}`).toBeNull();
        expect(docPhieu({ ...PHIEU_RA, status, receiving_body: sai }), `${status} body=${String(sai)}`).toBeNull();
      }
    }
  });

  it("giới hạn đếm theo KÝ TỰ: 2000 / 200 ký tự có dấu nhận, thêm một ký tự là sai khuôn", () => {
    expect(DO_DAI_NHANH_KET_THUC).toEqual({ ly_do: 2000, co_quan_nhan: 200 });
    const base = { ...PHIEU_RA, status: "chuyen-cap-tren" };
    // "ệ" là một ký tự; đếm theo byte UTF-8 sẽ là ba và từ chối oan.
    expect(docPhieu({ ...base, reason: "ệ".repeat(2000) })?.ly_do).toHaveLength(2000);
    expect(docPhieu({ ...base, reason: "ệ".repeat(2001) })).toBeNull();
    expect(docPhieu({ ...base, receiving_body: "ệ".repeat(200) })?.co_quan_nhan).toHaveLength(200);
    expect(docPhieu({ ...base, receiving_body: "ệ".repeat(201) })).toBeNull();
    // Ký tự ngoài BMP là HAI đơn vị UTF-16 nhưng MỘT ký tự — như `utf8.RuneCountInString`.
    expect(docPhieu({ ...base, receiving_body: "𠀀".repeat(200) })).not.toBeNull();
  });

  it("trạng thái khác: hai trường bị bỏ đi và KHÔNG hiện, dù máy chủ lỡ gửi", async () => {
    for (const status of Object.keys(TRANG_THAI).filter((s) => s !== "khong-tiep-nhan" && s !== "chuyen-cap-tren")) {
      const than = { ...PHIEU_RA, status, reason: LY_DO, receiving_body: CO_QUAN };
      const p = docPhieu(than);
      expect(p?.ly_do, status).toBe("");
      expect(p?.co_quan_nhan, status).toBe("");
      const html = await veTra(than);
      expect(html, status).not.toContain(LY_DO);
      expect(html, status).not.toContain(CO_QUAN);
      expect(html, status).not.toContain(THE_PHIEU.ly_do_khong_tiep_nhan);
      expect(html, status).not.toContain(THE_PHIEU.co_quan_tiep_nhan);
      expect(html, status).not.toContain(THE_PHIEU.ly_do_chuyen);
    }
  });

  it("không tiếp nhận: hiện lý do, KHÔNG hiện cơ quan nhận (không ai nhận cả)", async () => {
    const html = await veTra({ ...PHIEU_RA, status: "khong-tiep-nhan", reason: LY_DO, receiving_body: CO_QUAN });
    expect(html).toContain("Không tiếp nhận");
    expect(html).toContain(TRANG_THAI["khong-tiep-nhan"]!.giai_thich!);
    expect(html).toContain(THE_PHIEU.ly_do_khong_tiep_nhan);
    expect(html).toContain(LY_DO);
    expect(html).not.toContain(THE_PHIEU.co_quan_tiep_nhan);
    expect(html).not.toContain(CO_QUAN);
    expect(html).not.toContain(THE_PHIEU.lien_he_co_quan);
  });

  it("chuyển cấp trên: cơ quan tiếp nhận, lý do chuyển, và việc làm tiếp", async () => {
    const html = await veTra({ ...PHIEU_RA, status: "chuyen-cap-tren", reason: LY_DO, receiving_body: CO_QUAN });
    expect(html).toContain("Chuyển cấp trên");
    expect(html).toContain(TRANG_THAI["chuyen-cap-tren"]!.giai_thich!);
    expect(html).toContain(THE_PHIEU.co_quan_tiep_nhan);
    expect(html).toContain(CO_QUAN);
    expect(html).toContain(THE_PHIEU.ly_do_chuyen);
    expect(html).toContain(LY_DO);
    expect(html).toContain(THE_PHIEU.lien_he_co_quan);
    // Cơ quan trước, lý do sau — cùng thứ tự câu dòng phụ trạng thái chỉ xuống.
    expect(html.indexOf(CO_QUAN)).toBeLessThan(html.indexOf(LY_DO));
  });

  it("nhánh kết thúc mà máy chủ không gửi chữ: nói việc cần làm, không để ô trống", async () => {
    const html = await veTra({ ...PHIEU_RA, status: "chuyen-cap-tren" });
    expect(html).toContain(THE_PHIEU.co_quan_tiep_nhan);
    expect(html).toContain(THE_PHIEU.chua_ghi);
    // Không có tên cơ quan thì không mời "liên hệ cơ quan ở trên".
    expect(html).not.toContain(THE_PHIEU.lien_he_co_quan);
  });

  it("lý do dài hiện ĐỦ, trong lớp xuống dòng, không bị cắt", async () => {
    const dai = `${"Xã đã xác minh tại hiện trường và nhận thấy ".repeat(40)}HẾT.\nDòng hai của lý do.`;
    expect([...dai].length).toBeLessThanOrEqual(2000);
    const html = await veTra({ ...PHIEU_RA, status: "khong-tiep-nhan", reason: dai });
    expect(html).toContain(`<span class="cd-phieu__ly-do">${dai}</span>`);

    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    const khoi = /\.cd-phieu__ly-do\s*\{([^}]*)\}/.exec(css);
    expect(khoi, "styles.css không còn khối nào cho .cd-phieu__ly-do").not.toBeNull();
    expect(khoi![1]).toMatch(/white-space:\s*pre-wrap/);
    expect(khoi![1]).toMatch(/overflow-wrap:\s*anywhere/);
    // Không một luật nào ở bất kỳ đâu cắt chữ của lớp này.
    for (const m of css.matchAll(/([^{}]*cd-phieu__ly-do[^{}]*)\{([^}]*)\}/g)) {
      expect(m[2]).not.toMatch(/text-overflow|line-clamp|max-height|overflow:\s*hidden|nowrap/);
    }
  });

  it("đọc và vẽ hai nhánh không ghi gì ra console (luật 3)", async () => {
    const goi = (["log", "info", "warn", "error", "debug"] as const).map((k) => vi.spyOn(console, k));
    await veTra({ ...PHIEU_RA, status: "chuyen-cap-tren", reason: LY_DO, receiving_body: CO_QUAN });
    docPhieu({ ...PHIEU_RA, status: "khong-tiep-nhan", reason: 42 });
    for (const s of goi) {
      expect(s).not.toHaveBeenCalled();
      s.mockRestore();
    }
  });
});

/**
 * "PHẢN ÁNH CỦA TÔI" — `GET /api/v1/my-citizen-reports` (danh sách), với phiên giả của tệp này.
 */
describe("phản ánh của tôi — lời gọi và đọc trang", () => {
  const DONG_RA = {
    code: "PA7K2QX9M4TD",
    status: "dang-xu-ly",
    field: "giao-thong",
    field_label: "Giao thông",
    content_excerpt: "Ổ gà lớn trước cổng chợ…",
    clock_from: "2026-09-24T01:30:00Z",
    acknowledge_due: "2026-09-24T03:30:00Z",
    resolve_due: "2026-09-26T09:00:00Z",
  };
  const TRANG_RA = { items: [DONG_RA], next_cursor: "c1+/=&x", has_more: true };

  /** Mọi tham số trên đường dẫn của một lời gọi. */
  const thamSo = (g: LoiGoi) => new URL(g.dia_chi).searchParams;

  it("GET đúng tuyến, bearer từ nguồn phiên, CHỈ `limit` ở trang đầu — không danh tính, không xã", async () => {
    datFetch(traLoi(200, TRANG_RA));
    const kq = await phanAnhCuaToi("");
    expect(kq.kieu).toBe("xong");
    expect(loi_goi).toHaveLength(1);

    const g = loi_goi[0]!;
    const url = new URL(g.dia_chi);
    expect(`${url.origin}${url.pathname}`).toBe(`https://vigov.vidu.vn${DUONG_DAN_PHAN_ANH_CUA_TOI}`);
    expect(g.tuy_chon.method).toBe("GET");
    expect(g.tuy_chon.body).toBeUndefined();
    expect(tieuDe(g)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    // Đúng hai tiêu đề — không tiêu đề nào mang xã hay danh tính, không khoá chống trùng.
    expect(Object.keys(tieuDe(g)).sort()).toEqual(["Accept", "Authorization"]);
    expect([...thamSo(g).keys()]).toEqual(["limit"]);
    expect(thamSo(g).get("limit")).toBe(String(SO_DONG_MOI_TRANG));
    expect(SO_DONG_MOI_TRANG).toBeGreaterThanOrEqual(1);
    expect(SO_DONG_MOI_TRANG).toBeLessThanOrEqual(100);
  });

  it("trang sau truyền con trỏ NGUYÊN VĂN — kể cả ký tự `+ / = &`", async () => {
    datFetch(traLoi(200, TRANG_RA));
    await phanAnhCuaToi("c1+/=&x");
    const q = thamSo(loi_goi[0]!);
    expect(q.get("cursor")).toBe("c1+/=&x");
    expect([...q.keys()].sort()).toEqual(["cursor", "limit"]);
  });

  it("không một tham số nào nói của ai / xã nào / xếp thế nào, ở mọi trang", async () => {
    datFetch(traLoi(200, TRANG_RA));
    await phanAnhCuaToi("");
    await phanAnhCuaToi("c1");
    expect(loi_goi).toHaveLength(2);
    for (const g of loi_goi) {
      for (const cam of [
        "tenant",
        "tenant_id",
        "commune",
        "xa",
        "phone",
        "reporter_phone",
        "citizen_id",
        "cong_dan_id",
        "sort",
        "order",
        "status",
      ]) {
        expect(thamSo(g).has(cam), `tham số bị cấm: ${cam}`).toBe(false);
      }
    }
  });

  it("ánh xạ đủ trường, giữ hai `null` của hai hạn", async () => {
    datFetch(
      traLoi(200, {
        items: [DONG_RA, { ...DONG_RA, code: "PB2", acknowledge_due: null, resolve_due: null }],
        next_cursor: "",
        has_more: false,
      }),
    );
    const kq = await phanAnhCuaToi("");
    if (kq.kieu !== "xong") throw new Error(`mong đợi xong, nhận ${kq.kieu}`);
    expect(kq.trang.con_nua).toBe(false);
    expect(kq.trang.con_tro).toBe("");
    expect(kq.trang.muc[0]).toEqual({
      ma_tra_cuu: "PA7K2QX9M4TD",
      trang_thai: "dang-xu-ly",
      linh_vuc: "giao-thong",
      nhan_linh_vuc: "Giao thông",
      trich_noi_dung: "Ổ gà lớn trước cổng chợ…",
      goc_dem_han: "2026-09-24T01:30:00Z",
      han_tiep_nhan: "2026-09-24T03:30:00Z",
      han_xu_ly_xong: "2026-09-26T09:00:00Z",
    });
    expect(kq.trang.muc[1]!.han_tiep_nhan).toBeNull();
    expect(kq.trang.muc[1]!.han_xu_ly_xong).toBeNull();
  });

  it("danh sách rỗng là một trang hợp lệ", () => {
    expect(docTrangPhieuCuaToi({ items: [], next_cursor: "", has_more: false })).toEqual({
      muc: [],
      con_tro: "",
      con_nua: false,
    });
  });

  it("sai khuôn là null — một dòng hỏng làm hỏng cả trang, không bỏ lặng lẽ", () => {
    const thieu_ma: Record<string, unknown> = { ...DONG_RA };
    delete thieu_ma["code"];
    for (const sai of [
      null,
      [],
      { items: null, next_cursor: "", has_more: false },
      { items: [], has_more: false },
      { items: [], next_cursor: "", has_more: "false" },
      // Còn nữa mà không có con trỏ: "Xem thêm" sẽ tải lại trang đầu và nhân đôi danh sách.
      { items: [], next_cursor: "", has_more: true },
      { items: [DONG_RA, thieu_ma], next_cursor: "", has_more: false },
      { items: [{ ...DONG_RA, code: "" }], next_cursor: "", has_more: false },
      { items: [{ ...DONG_RA, resolve_due: 5 }], next_cursor: "", has_more: false },
      { items: [{ ...DONG_RA, content_excerpt: undefined }], next_cursor: "", has_more: false },
    ]) {
      expect(docTrangPhieuCuaToi(sai), JSON.stringify(sai)).toBeNull();
    }
  });

  it("mỗi mã trạng thái rơi vào đúng một nhánh — 404/409/503 không mượn câu của tuyến khác", async () => {
    for (const [status, kieu] of [
      [400, "khong-hop-le"],
      [401, "het-phien"],
      [404, "loi-may-chu"],
      [409, "loi-may-chu"],
      [503, "loi-may-chu"],
      [500, "loi-may-chu"],
    ] as const) {
      datFetch(traLoi(status, { code: "x", message: "y", trace_id: "" }));
      expect((await phanAnhCuaToi("")).kieu, `mã ${status}`).toBe(kieu);
    }
    datFetch(traLoi(200, { items: "x" }));
    expect((await phanAnhCuaToi("")).kieu, "200 sai khuôn").toBe("loi-may-chu");
    datFetch(new Error("mất mạng"));
    expect(await phanAnhCuaToi("")).toEqual({ kieu: "loi-mang" });
  });

  it("không phiên hoặc không địa chỉ: dừng TRƯỚC fetch", async () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);
    trang.phien = null;
    expect(await phanAnhCuaToi("")).toEqual({ kieu: "chua-co-phien" });
    trang.phien = { token: "", ten_xa: "Xã Thử Nghiệm" };
    expect(await phanAnhCuaToi("c1")).toEqual({ kieu: "chua-co-phien" });
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    trang.host = "";
    expect(await phanAnhCuaToi("")).toEqual({ kieu: "chua-cau-hinh" });
    expect(fetch_gia).not.toHaveBeenCalled();
  });
});

describe("phản ánh của tôi — màn hình", () => {
  const phieu = (ma: string, trang_thai = "da-tiep-nhan"): PhieuCuaToiTomTat => ({
    ma_tra_cuu: ma,
    trang_thai,
    linh_vuc: "",
    nhan_linh_vuc: "",
    trich_noi_dung: "Đèn đường hỏng ở đầu ngõ",
    goc_dem_han: "2026-09-24T01:30:00Z",
    han_tiep_nhan: "2026-09-24T03:30:00Z",
    han_xu_ly_xong: null,
  });
  const xong = (muc: PhieuCuaToiTomTat[], con_tro: string) =>
    ({ kieu: "xong", trang: { muc, con_tro, con_nua: con_tro !== "" } }) as const;
  const ve = (ds: typeof DANH_SACH_DAU) =>
    renderToStaticMarkup(
      createElement(ThanDanhSach, { ds, onMo: () => {}, onTai: () => {}, onGuiPhanAnh: () => {} }),
    );

  it("có phiên: màn mở ra ở trạng thái đang tải, có tên xã của phiên, chưa hiện câu 'chưa gửi'", () => {
    const html = renderToStaticMarkup(
      createElement(PhanAnhCuaToiScreen, { onQuayLai: () => {}, onMoPhieu: () => {}, onGuiPhanAnh: () => {} }),
    );
    expect(html).toContain(CUA_TOI.tieu_de);
    expect(html).toContain("Xã Thử Nghiệm");
    expect(html).toContain(CUA_TOI.dang_tai);
    expect(html).not.toContain(CUA_TOI.trong);
  });

  it("rỗng: câu 'Bạn chưa gửi phản ánh nào.' và một nút Gửi phản ánh", () => {
    const html = ve(sauKhiTai(DANH_SACH_DAU, xong([], "")));
    expect(CUA_TOI.trong).toBe("Bạn chưa gửi phản ánh nào.");
    expect(html).toContain(CUA_TOI.trong);
    expect(html).toContain(`<button type="button" class="cd-nut">${GUI.tieu_de}</button>`);
    expect(html).not.toContain(CUA_TOI.nut_xem_them);
    expect(html).not.toContain(CUA_TOI.dang_tai);
  });

  it("'Xem thêm' NỐI trang sau vào cuối, dùng con trỏ của trang trước, và bỏ dòng trùng", () => {
    let ds = sauKhiTai(DANH_SACH_DAU, xong([phieu("PA1"), phieu("PA2")], "c1"));
    expect(ds.con_tro).toBe("c1");
    expect(ve(ds)).toContain(CUA_TOI.nut_xem_them);

    ds = batDauTai(ds);
    expect(ds.dang_tai).toBe(true);
    // Đang tải thêm: danh sách cũ VẪN hiện, không nháy về trạng thái rỗng.
    expect(ve(ds)).toContain("PA1");
    expect(ve(ds)).toContain(CUA_TOI.dang_tai_them);

    ds = sauKhiTai(ds, xong([phieu("PA2"), phieu("PA3")], ""));
    expect(ds.muc.map((p) => p.ma_tra_cuu)).toEqual(["PA1", "PA2", "PA3"]);
    expect(ds.con_nua).toBe(false);
    const html = ve(ds);
    expect(html).not.toContain(CUA_TOI.nut_xem_them);
    expect(html).toContain(CUA_TOI.het_danh_sach);
    expect(html.indexOf("PA1")).toBeLessThan(html.indexOf("PA3"));
  });

  it("lỗi giữ nguyên danh sách đã có và mời Thử lại; hết phiên thì không mời", () => {
    const co = sauKhiTai(DANH_SACH_DAU, xong([phieu("PA1")], "c1"));
    const mang = sauKhiTai(batDauTai(co), { kieu: "loi-mang" });
    expect(mang.muc).toHaveLength(1);
    expect(mang.con_tro).toBe("c1");
    expect(ve(mang)).toContain(CUA_TOI.loi_mang);
    expect(ve(mang)).toContain(CUA_TOI.nut_thu_lai);
    expect(ve(mang)).toContain("PA1");

    const sau_lan_dau = sauKhiTai(DANH_SACH_DAU, { kieu: "khong-hop-le" });
    expect(ve(sau_lan_dau)).toContain(CUA_TOI.loi_may_chu);
    expect(ve(sau_lan_dau)).toContain(CUA_TOI.nut_thu_lai);
    expect(ve(sau_lan_dau)).not.toContain(CUA_TOI.trong);

    const het = sauKhiTai(DANH_SACH_DAU, { kieu: "het-phien" });
    expect(ve(het)).toContain(CUA_TOI.het_phien);
    expect(ve(het)).not.toContain(CUA_TOI.nut_thu_lai);

    // Máy chủ nói chưa có phiên: màn đóng lại thành "kênh chưa mở".
    expect(sauKhiTai(DANH_SACH_DAU, { kieu: "chua-co-phien" }).kenh_dong).toBe(true);
  });

  it("thẻ: mã to, trạng thái bằng chữ đã duyệt, lĩnh vực, trích đoạn, mốc +07 — đủ chín trạng thái", () => {
    expect(Object.keys(TRANG_THAI)).toHaveLength(9);
    for (const [ma_tt, { nhan }] of Object.entries(TRANG_THAI)) {
      const html = renderToStaticMarkup(
        createElement(ThePhieuTomTat, { phieu: phieu("PA7K2QX9M4TD", ma_tt), onMo: () => {} }),
      );
      expect(html, ma_tt).toContain(`<strong class="cd-the-cua-toi__trang-thai">${nhan}</strong>`);
      expect(html, ma_tt).not.toContain(ma_tt);
    }

    const html = renderToStaticMarkup(
      createElement(ThePhieuTomTat, {
        phieu: { ...phieu("PA7K2QX9M4TD"), nhan_linh_vuc: "Giao thông", han_xu_ly_xong: "2026-09-26T09:00:00Z" },
        onMo: () => {},
      }),
    );
    expect(html).toContain('<span class="cd-the-cua-toi__ma">PA7K2QX9M4TD</span>');
    expect(html).toContain("Giao thông");
    expect(html).toContain("Đèn đường hỏng ở đầu ngõ");
    expect(html).toContain(`${THE_PHIEU.gui_luc}: 24/09/2026 08:30`);
    expect(html).toContain(`${THE_PHIEU.han_xu_ly}: 26/09/2026 16:00`);
    // Mốc cố định, không đếm "quá hạn" ở client (ADR 0007).
    expect(html).not.toMatch(/quá hạn/i);
    // Cả thẻ là một nút.
    expect(html).toMatch(/^<li class="cd-cua-toi__muc"><button type="button" class="cd-the-cua-toi">/);
  });

  it("thẻ: chưa có hạn xử lý thì KHÔNG hiện dòng hạn; mã lĩnh vực thô không bao giờ hiện", () => {
    const html = renderToStaticMarkup(
      createElement(ThePhieuTomTat, {
        phieu: { ...phieu("PA1"), linh_vuc: "giao-thong", nhan_linh_vuc: "" },
        onMo: () => {},
      }),
    );
    expect(html).not.toContain(THE_PHIEU.han_xu_ly);
    expect(html).not.toContain("giao-thong");
    expect(html).toContain(THE_PHIEU.da_phan_loai);
  });

  it("chạm thẻ gọi onMo với đúng mã", () => {
    const mo = vi.fn();
    const el = ThePhieuTomTat({ phieu: phieu("PA9"), onMo: mo }) as unknown as {
      props: { children: { props: { onClick: () => void } } };
    };
    el.props.children.props.onClick();
    expect(mo).toHaveBeenCalledWith("PA9");
  });

  it("mở tra cứu từ danh sách: mã điền sẵn vào ô", () => {
    const html = renderToStaticMarkup(
      createElement(TraCuuPhieuScreen, { onQuayLai: () => {}, ma_ban_dau: "PA7K2QX9M4TD" }),
    );
    expect(html).toMatch(/<input[^>]*value="PA7K2QX9M4TD"/);
  });
});
