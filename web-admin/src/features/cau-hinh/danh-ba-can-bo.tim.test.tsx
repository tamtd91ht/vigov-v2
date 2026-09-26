import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as apiDanhMuc from "@/lib/api/danh-muc";
import { chuanHoaTuKhoaTim, type TuKhoaHopLe } from "@/lib/api/can-bo";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  BangCanBo,
  CAU_SAP_XEP_KHI_TIM,
  DanhBaCanBo,
  HangLocNguoiDung,
  ThanhSapXep,
  type LocNguoiDung,
} from "./danh-ba-can-bo";
import { sangTrangSau, TRANG_DAU } from "./ngan-xep-con-tro";

/**
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * Ô TÌM VÀ BỘ LỌC BỘ PHẬN CỦA TAB NGƯỜI DÙNG — xếp theo cái giá của việc sai.
 *
 *   1. CHỮ TÌM LÊN URL. Chữ tìm thường là họ tên hay số điện thoại; trên URL nó vào log truy cập của
 *      mọi proxy và lịch sử trình duyệt của máy dùng chung ở bộ phận một cửa (luật 3, cấm #4).
 *      Không bài kiểm chức năng nào đỏ vì nó: kết quả tìm vẫn đúng.
 *   2. CON TRỎ CŨ ĐI KÈM BỘ LỌC MỚI. Máy chủ hoặc trả 400, hoặc trả một trang giữa chừng — và cán
 *      bộ không thấy những người đứng trước con trỏ, rồi kết luận họ không có trong danh bạ.
 *   3. BỎ TÌM MÀ KHÔNG VỀ DANH SÁCH. Màn hình hiện kết quả của một chữ không còn ở đâu.
 *
 * VÌ SAO CHẠY `lib/api/can-bo.ts` THẬT VÀ CHỈ THAY `fetch`: câu hỏi của tệp này là "cái gì RỜI trình
 * duyệt" — URL và thân. Thay `docTrangDanhBa` bằng `vi.fn` thì chỉ kiểm được đối số, không kiểm được
 * rằng chữ tìm không bị hàm ấy đặt lên URL.
 *
 * BỘ CHẠY HOOK TỐI THIỂU, cùng khuôn với `features/danh-ba/danh-ba-lien-he.luong.test.tsx` (tệp ấy
 * nói giới hạn của nó: không phải React, đủ cho câu hỏi "màn gửi gì, khi nào").
 *
 * Số điện thoại là số giả đã thoả thuận (luật 3, bất biến 5).
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */

const H = vi.hoisted(() => ({
  hienTai: null as null | { o: { gia: unknown }[]; i: number; choChay: (() => void)[] },
}));

vi.mock("react", async (goc) => {
  const R = await goc<typeof import("react")>();
  const lay = () => {
    const c = H.hienTai;
    if (c === null) throw new Error("hook gọi ngoài một lượt dựng");
    return { c, i: c.i++ };
  };
  return {
    ...R,
    useState: (dau: unknown) => {
      const { c, i } = lay();
      if (c.o[i] === undefined) c.o[i] = { gia: typeof dau === "function" ? (dau as () => unknown)() : dau };
      const o = c.o[i] as { gia: unknown };
      const dat = (v: unknown) => {
        o.gia = typeof v === "function" ? (v as (cu: unknown) => unknown)(o.gia) : v;
      };
      return [o.gia, dat];
    },
    useRef: (dau: unknown) => {
      const { c, i } = lay();
      if (c.o[i] === undefined) c.o[i] = { gia: { current: dau } };
      return (c.o[i] as { gia: unknown }).gia;
    },
    useEffect: (chay: () => void | (() => void), phuThuoc?: readonly unknown[]) => {
      const { c, i } = lay();
      const cu = c.o[i]?.gia as { phuThuoc?: readonly unknown[]; huy?: () => void } | undefined;
      const doi =
        cu === undefined ||
        phuThuoc === undefined ||
        phuThuoc.some((d, k) => !Object.is(d, cu.phuThuoc?.[k]));
      if (cu === undefined) c.o[i] = { gia: { phuThuoc: undefined } };
      if (!doi) return;
      c.choChay.push(() => {
        cu?.huy?.();
        const huy = chay();
        (c.o[i] as { gia: unknown }).gia = { phuThuoc, huy: typeof huy === "function" ? huy : undefined };
      });
    },
    useCallback: <T,>(f: T) => f,
    useMemo: <T,>(f: () => T) => f(),
  };
});

vi.mock("@/lib/api/danh-muc", async (goc) => ({
  ...(await goc<typeof import("@/lib/api/danh-muc")>()),
  docDanhMucDanhBa: vi.fn(),
}));

function may<P extends object>(Comp: (p: P) => unknown, props: P) {
  const trang = { o: [] as { gia: unknown }[], i: 0, choChay: [] as (() => void)[] };
  let cay: unknown = null;
  const ve = () => {
    trang.i = 0;
    trang.choChay = [];
    H.hienTai = trang;
    try {
      cay = Comp(props);
    } finally {
      H.hienTai = null;
    }
    for (const f of trang.choChay) f();
    return cay;
  };
  return { ve, cay: () => cay };
}

async function xongMang(m: { ve: () => unknown }) {
  m.ve();
  await new Promise((r) => setTimeout(r, 0));
  m.ve();
}

type PhanTu = { type: unknown; props: Record<string, unknown> };

function tatCa(goc: unknown, dung: (p: PhanTu) => boolean): PhanTu[] {
  const ra: PhanTu[] = [];
  const duyet = (n: unknown) => {
    if (Array.isArray(n)) return n.forEach(duyet);
    if (typeof n !== "object" || n === null || !("props" in n) || !("type" in n)) return;
    const p = n as PhanTu;
    if (dung(p)) ra.push(p);
    duyet(p.props.children);
  };
  duyet(goc);
  return ra;
}

function phaiCo<P>(goc: unknown, Comp: (p: P) => unknown): P {
  const ds = tatCa(goc, (p) => p.type === Comp);
  if (ds.length !== 1) throw new Error(`cần đúng một phần tử, có ${ds.length}`);
  return ds[0]?.props as P;
}

/* ---- dữ liệu và máy chủ giả ------------------------------------------------------------------ */

const A: identity_canBoTomTat = {
  id: "01J00000000000000000000001",
  code: "CB-00001",
  full_name: "Nguyễn Văn A",
  email: "nva@demo.invalid",
  position: "Chuyên viên",
  department_id: "BP-LE",
  role_id: "",
  phone: "02350000001",
  mobile: "0900000000",
  has_account: false,
  active: true,
  last_login_at: null,
  created_at: "2026-09-01T02:00:00Z",
  has_zalo: false,
  published: false,
  display_order: null,
  consent_recorded_at: null,
};

/** Chữ tìm mang CẢ họ tên có dấu lẫn một số điện thoại — hai thứ luật 3 cấm lên URL. */
const CHU = "0900000000 Nguyễn";

function hopLe(tho: string): TuKhoaHopLe {
  const tu = chuanHoaTuKhoaTim(tho);
  if (tu.loai !== "hopLe") throw new Error("chữ mẫu phải hợp lệ");
  return tu;
}

type LoiGoi = { url: string; method: string; than: Record<string, unknown> | null };

let gia: ReturnType<typeof vi.fn>;

function cacLoiGoi(): LoiGoi[] {
  return (gia.mock.calls as [string, RequestInit | undefined][]).map(([url, tc]) => ({
    url,
    method: tc?.method ?? "GET",
    than: tc?.body === undefined ? null : (JSON.parse(String(tc.body)) as Record<string, unknown>),
  }));
}

const cuoi = () => cacLoiGoi().at(-1) as LoiGoi;

beforeEach(() => {
  gia = vi.fn(async () =>
    new Response(JSON.stringify({ items: [A], next_cursor: "CB-00001", has_more: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
  vi.stubGlobal("fetch", gia);
  vi.mocked(apiDanhMuc.docDanhMucDanhBa).mockResolvedValue({
    boPhan: {
      ok: true,
      duLieu: { items: [{ id: "BP-LE", code: "le", name: "VĂN PHÒNG", parent_id: "", order: 1, staff_count: 1 }] },
    },
    vaiTro: { ok: true, duLieu: { items: [] } },
  } as unknown as Awaited<ReturnType<typeof apiDanhMuc.docDanhMucDanhBa>>);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function moMan() {
  const m = may(DanhBaCanBo, {});
  await xongMang(m);
  return m;
}

const hangLoc = (m: { cay: () => unknown }) => phaiCo(m.cay(), HangLocNguoiDung);

function diToiTrang(m: { cay: () => unknown }) {
  const nav = tatCa(m.cay(), (p) => typeof p.props.diToiTrang === "function")[0];
  return nav?.props.diToiTrang as (x: ReturnType<typeof sangTrangSau>) => void;
}

/** Mọi cách chữ tìm có thể hiện trên một URL: nguyên văn, mã hoá kiểu URL, và kiểu form (`+`). */
function hinhDang(chu: string): string[] {
  return [chu, encodeURIComponent(chu), new URLSearchParams({ x: chu }).toString().slice(2)];
}

/* ---- 1. chữ tìm và bộ phận đi trong THÂN, URL đứng yên --------------------------------------- */

describe("tìm: chữ và bộ phận đi trong thân POST, URL không mang cái nào", () => {
  it("tìm kèm bộ phận → POST tới đúng đường dẫn hằng, thân có `q` đã chuẩn hoá và `unit`", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ boPhan: "BP-LE" });
    await xongMang(m);
    hangLoc(m).doiLoc({ tuKhoa: hopLe(`  ${CHU}  `) });
    await xongMang(m);

    const lg = cuoi();
    expect(lg.method).toBe("POST");
    expect(lg.url).toBe("/api/v1/staff/searches");
    expect(lg.than).toMatchObject({ q: CHU, unit: "BP-LE", cursor: "" });
    // Tuyến tìm không có `sort`/`order`: gửi lên là gửi một thứ máy chủ không đọc.
    expect(lg.than).not.toHaveProperty("sort");
    expect(lg.than).not.toHaveProperty("order");
  });

  it("KHÔNG URL NÀO của cả luồng — mở, lọc, tìm, sang trang, đổi bộ phận — chứa chữ tìm", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    hangLoc(m).doiLoc({ boPhan: "BP-LE" });
    await xongMang(m);

    const ds = cacLoiGoi();
    expect(ds.filter((x) => x.method === "POST").length).toBe(3);
    for (const { url, method } of ds) {
      for (const h of [...hinhDang(CHU), ...hinhDang("0900000000"), ...hinhDang("Nguyễn")]) {
        expect(url).not.toContain(h);
      }
      expect(url).not.toMatch(/[?&]q=/);
      // Đang tìm thì bộ phận cũng nằm trong thân, không thêm vào URL của tuyến tìm.
      if (method === "POST") expect(url).toBe("/api/v1/staff/searches");
    }
  });

  it("cả luồng không ghi gì ra console", async () => {
    const nghe = (["log", "info", "warn", "error", "debug"] as const).map((k) =>
      vi.spyOn(console, k).mockImplementation(() => undefined),
    );
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    for (const n of nghe) expect(n).not.toHaveBeenCalled();
  });
});

/* ---- 2. con trỏ về trang đầu khi truy vấn đổi ------------------------------------------------ */

describe("đổi chữ tìm hay bộ phận là về trang đầu", () => {
  it("danh sách trang 2 → bắt đầu tìm: thân KHÔNG mang con trỏ của danh sách", async () => {
    const m = await moMan();
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(cuoi().url).toContain("cursor=CB-00001");

    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ cursor: "" });
  });

  it("đang tìm ở trang 2 → đổi bộ phận: con trỏ về rỗng; đổi chữ: cũng về rỗng", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    // Vế đối chứng: phân trang của một lần tìm có đi, và con trỏ đi trong thân.
    expect(cuoi().than).toMatchObject({ q: CHU, cursor: "CB-00001" });

    hangLoc(m).doiLoc({ boPhan: "BP-LE" });
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ q: CHU, unit: "BP-LE", cursor: "" });

    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    hangLoc(m).doiLoc({ tuKhoa: hopLe("Trần") });
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ q: "Trần", cursor: "" });
  });

  it("danh sách trang 2 → đổi bộ phận: URL GET không còn `cursor`", async () => {
    const m = await moMan();
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    hangLoc(m).doiLoc({ boPhan: "BP-LE" });
    await xongMang(m);
    expect(cuoi().method).toBe("GET");
    expect(cuoi().url).toContain("unit=BP-LE");
    expect(cuoi().url).not.toContain("cursor=");
  });
});

/* ---- 3. bỏ tìm → danh sách thường, sắp xếp đang chọn còn nguyên -------------------------------- */

describe("bỏ tìm quay về danh sách phân trang như trước", () => {
  it("chọn Ngày tạo, tìm, rồi bỏ tìm → GET với ĐÚNG sắp xếp đã chọn, bộ phận còn giữ", async () => {
    const m = await moMan();
    phaiCo(m.cay(), ThanhSapXep).doiSapXep?.("created_at");
    await xongMang(m);
    hangLoc(m).doiLoc({ boPhan: "BP-LE" });
    await xongMang(m);
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    expect(cuoi().method).toBe("POST");

    hangLoc(m).doiLoc({ tuKhoa: null });
    await xongMang(m);
    const lg = cuoi();
    expect(lg.method).toBe("GET");
    expect(lg.than).toBeNull();
    const url = new URL(lg.url, "https://mot-xa.test");
    expect(url.pathname).toBe("/api/v1/staff");
    expect(url.searchParams.get("sort")).toBe("created_at");
    expect(url.searchParams.get("unit")).toBe("BP-LE");
    expect(url.searchParams.has("cursor")).toBe(false);
  });

  it("đang tìm: thanh sắp xếp nói thứ tự thật, bảng nhận `code`/`asc` và KHÔNG có hàm sắp xếp", async () => {
    const m = await moMan();
    phaiCo(m.cay(), ThanhSapXep).doiSapXep?.("created_at");
    await xongMang(m);
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);

    expect(phaiCo(m.cay(), ThanhSapXep).doiSapXep).toBeNull();
    const bang = phaiCo(m.cay(), BangCanBo);
    expect(bang.khoa).toBe("code");
    expect(bang.chieu).toBe("asc");
    expect(bang.doiSapXep).toBeNull();

    const html = renderToStaticMarkup(<ThanhSapXep khoa="created_at" chieu="desc" doiSapXep={null} />);
    expect(html).toContain(CAU_SAP_XEP_KHI_TIM);
    expect(html).not.toContain("<button");
  });
});

/* ---- ô nhập: hình dạng chặn rò rỉ, và ba đường vào `doiLoc` ------------------------------------ */

describe("ô tìm của tab Người dùng", () => {
  const BO_PHAN = [{ id: "BP-LE", name: "VĂN PHÒNG" }];

  function chay(loc: LocNguoiDung, doiLoc = vi.fn()) {
    const m = may(HangLocNguoiDung, { loc, boPhan: BO_PHAN, doiLoc });
    m.ve();
    const nhap = (chu: string) => {
      const o = tatCa(m.cay(), (p) => p.props.id === "tim-nguoi-dung")[0];
      (o?.props.onChange as (e: unknown) => void)({ target: { value: chu } });
      m.ve();
    };
    const gui = () => {
      const f = tatCa(m.cay(), (p) => p.type === "form")[0];
      let chan = false;
      (f?.props.onSubmit as (e: unknown) => void)({ preventDefault: () => (chan = true) });
      m.ve();
      return chan;
    };
    return { m, nhap, gui, doiLoc };
  }

  it("Enter với chữ hợp lệ → `doiLoc` nhận chữ ĐÃ chuẩn hoá; mặc định của trình duyệt bị chặn", () => {
    const { nhap, gui, doiLoc } = chay({ tuKhoa: null, boPhan: "" });
    nhap(`  ${CHU}   `);
    expect(doiLoc).not.toHaveBeenCalled(); // gõ từng phím không gọi mạng
    expect(gui()).toBe(true);
    expect(doiLoc).toHaveBeenCalledWith({ tuKhoa: { loai: "hopLe", tu: CHU } });
  });

  it("ô trống rồi gửi → bỏ tìm (`tuKhoa: null`), không gửi chữ rỗng lên tuyến tìm", () => {
    const { nhap, gui, doiLoc } = chay({ tuKhoa: null, boPhan: "" });
    nhap("   ");
    gui();
    expect(doiLoc).toHaveBeenCalledWith({ tuKhoa: null });
  });

  it("xoá trắng ô trong lúc đang tìm → bỏ tìm ngay, không đợi Enter", () => {
    const { nhap, doiLoc } = chay({ tuKhoa: hopLe(CHU), boPhan: "" });
    nhap("");
    expect(doiLoc).toHaveBeenCalledWith({ tuKhoa: null });
  });

  it("chọn một id không có trong danh mục → lọc 'tất cả', không gửi id không ai chọn", () => {
    const { m, doiLoc } = chay({ tuKhoa: null, boPhan: "" });
    const chon = tatCa(m.cay(), (p) => p.props.id === "loc-bo-phan-nguoi-dung")[0];
    (chon?.props.onChange as (e: unknown) => void)({ target: { value: "BP-LA" } });
    expect(doiLoc).toHaveBeenCalledWith({ boPhan: "" });
    (chon?.props.onChange as (e: unknown) => void)({ target: { value: "BP-LE" } });
    expect(doiLoc).toHaveBeenLastCalledWith({ boPhan: "BP-LE" });
  });

  it("ô nhập KHÔNG có `name`, form là POST, tắt tự điền; hai ô đều có nhãn", () => {
    // Trước khi JavaScript chạy xong, Enter trong một form GET đưa mọi ô có `name` lên URL.
    // (React bị thay bằng bộ chạy ở tệp này, nên đọc cây phần tử thay vì dựng HTML.)
    const { m } = chay({ tuKhoa: null, boPhan: "" });
    const cay = m.cay();
    const input = tatCa(cay, (p) => p.type === "input")[0]?.props ?? {};
    expect(input).not.toHaveProperty("name");
    expect(input.autoComplete).toBe("off");
    expect(tatCa(cay, (p) => p.type === "form")[0]?.props.method).toBe("post");
    const nhan = tatCa(cay, (p) => p.type === "label").map((p) => p.props.htmlFor);
    expect(nhan).toEqual(["tim-nguoi-dung", "loc-bo-phan-nguoi-dung"]);
    expect(tatCa(cay, (p) => p.type === "select")[0]?.props.id).toBe("loc-bo-phan-nguoi-dung");
  });
});
