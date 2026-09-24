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
  diaChiViGov: (d: string) => (trang.host === "" ? "" : `${trang.host}${d}`),
}));

import { KetQuaGui } from "../man/GuiPhanAnhScreen";
import { GUI, TRA_CUU } from "../man/noi-dung";
import { KetQuaTraCuu } from "../man/TraCuuPhieuScreen";

import { guiPhanAnh, traCuuPhieu } from "./goi-vigov";
import { DUONG_DAN_PHAN_ANH_CUA_TOI, type PhanAnhMoi, thanGuiPhanAnh, TRUONG_DUOC_NHAN } from "./hop-dong-phan-anh";
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
