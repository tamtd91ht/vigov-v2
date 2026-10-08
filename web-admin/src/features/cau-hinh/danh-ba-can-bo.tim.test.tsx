import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as apiDanhMuc from "@/lib/api/danh-muc";
import { chuanHoaTuKhoaTim } from "@/lib/api/can-bo";
import type { TuKhoaHopLe } from "@/lib/api/can-bo";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { Skeleton } from "@/components/ui/skeleton";
import { HopXoa } from "@/features/danh-ba/hop-xoa";
import { CAU_THIEU_LY_DO } from "@/features/danh-ba/xoa-dong";

import { BangCanBo, DanhBaCanBo, HangLocNguoiDung, SEARCH_DEBOUNCE_MS } from "./danh-ba-can-bo";
import type { LocNguoiDung } from "./danh-ba-can-bo";
import { SEARCH_PLACEHOLDER } from "./nhan-can-bo";
import { sangTrangSau, TRANG_DAU } from "./ngan-xep-con-tro";

/**
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * Ô TÌM, ĐẾM VÀ XOÁ CỦA MÀN NGƯỜI DÙNG — xếp theo cái giá của việc sai.
 *
 *   1. CHỮ TÌM LÊN URL. Chữ tìm thường là họ tên hay số điện thoại; trên URL nó vào log truy cập của
 *      mọi proxy và lịch sử trình duyệt của máy dùng chung ở bộ phận một cửa (luật 3, cấm #4).
 *      Không bài kiểm chức năng nào đỏ vì nó: kết quả tìm vẫn đúng.
 *   2. CON TRỎ CŨ ĐI KÈM BỘ LỌC MỚI. Máy chủ hoặc trả 400, hoặc trả một trang giữa chừng — và cán
 *      bộ không thấy những người đứng trước con trỏ, rồi kết luận họ không có trong danh bạ.
 *   3. MỖI PHÍM MỘT LỜI GỌI. Tìm chạy theo khi gõ (prototype), nhưng chỉ sau khi tay dừng.
 *   4. XOÁ KHÔNG LÝ DO. Xoá mềm luôn mang lý do (luật 7); không có đường nào gửi DELETE thiếu nó.
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

const COUNTS_PATH = "/api/v1/staff-counts";

let gia: ReturnType<typeof vi.fn>;
/** What `GET /api/v1/staff-counts` answers in the current test: a total, or a refusal. */
let countsAnswer: { status: number; body: unknown };

function allCalls(): LoiGoi[] {
  return (gia.mock.calls as [string, RequestInit | undefined][]).map(([url, tc]) => ({
    url,
    method: tc?.method ?? "GET",
    than: tc?.body === undefined ? null : (JSON.parse(String(tc.body)) as Record<string, unknown>),
  }));
}

/** The LIST calls only — the count read is not part of the search/paging questions below. */
function cacLoiGoi(): LoiGoi[] {
  return allCalls().filter((c) => c.url !== COUNTS_PATH);
}

const cuoi = () => cacLoiGoi().at(-1) as LoiGoi;

function json(status: number, body: unknown): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  countsAnswer = { status: 200, body: { total: 7, published: 0, no_department: { total: 0, published: 0 }, departments: [] } };
  gia = vi.fn(async (url: string, init?: RequestInit) => {
    if (url === COUNTS_PATH) return json(countsAnswer.status, countsAnswer.body);
    if (init?.method === "DELETE") return json(204, null);
    return json(200, { items: [A], next_cursor: "CB-00001", has_more: true });
  });
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
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function moMan(props: { canDelete?: boolean } = {}) {
  const m = may(DanhBaCanBo, props);
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

/* ---- 1. chữ tìm đi trong THÂN, URL đứng yên -------------------------------------------------- */

describe("tìm: chữ đi trong thân POST, URL không mang nó", () => {
  it("tìm → POST tới đúng đường dẫn hằng, thân có `q` đã chuẩn hoá, không `sort`/`order`", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(`  ${CHU}  `) });
    await xongMang(m);

    const lg = cuoi();
    expect(lg.method).toBe("POST");
    expect(lg.url).toBe("/api/v1/staff/searches");
    expect(lg.than).toMatchObject({ q: CHU, cursor: "" });
    expect(lg.than).not.toHaveProperty("sort");
    expect(lg.than).not.toHaveProperty("order");
  });

  it("KHÔNG URL NÀO của cả luồng — mở, tìm, sang trang — chứa chữ tìm", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);

    const ds = allCalls();
    expect(ds.filter((x) => x.method === "POST").length).toBe(2);
    for (const { url, method } of ds) {
      for (const h of [...hinhDang(CHU), ...hinhDang("0900000000"), ...hinhDang("Nguyễn")]) {
        expect(url).not.toContain(h);
      }
      expect(url).not.toMatch(/[?&]q=/);
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

  it("the default list is the server's order: no `sort`/`order` on the GET (both sort columns are gone)", async () => {
    await moMan();
    const url = new URL(cuoi().url, "https://mot-xa.test");
    expect(url.pathname).toBe("/api/v1/staff");
    expect(url.searchParams.has("sort")).toBe(false);
    expect(url.searchParams.has("order")).toBe(false);
  });
});

/* ---- 2. con trỏ về trang đầu khi truy vấn đổi ------------------------------------------------ */

describe("đổi chữ tìm là về trang đầu", () => {
  it("danh sách trang 2 → bắt đầu tìm: thân KHÔNG mang con trỏ của danh sách", async () => {
    const m = await moMan();
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(cuoi().url).toContain("cursor=CB-00001");

    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ cursor: "" });
  });

  it("đang tìm ở trang 2 → đổi chữ: con trỏ về rỗng", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ q: CHU, cursor: "CB-00001" });

    hangLoc(m).doiLoc({ tuKhoa: hopLe("Trần") });
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ q: "Trần", cursor: "" });
  });

  it("bỏ tìm → GET danh sách, không con trỏ", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    hangLoc(m).doiLoc({ tuKhoa: null });
    await xongMang(m);
    const lg = cuoi();
    expect(lg.method).toBe("GET");
    expect(lg.than).toBeNull();
    const url = new URL(lg.url, "https://mot-xa.test");
    expect(url.pathname).toBe("/api/v1/staff");
    expect(url.searchParams.has("cursor")).toBe(false);
  });
});

/* ---- 3. ô tìm: chạy theo khi gõ, sau khi tay dừng --------------------------------------------- */

describe("ô tìm của màn Người dùng — live, debounced (decision 3)", () => {
  function chay(loc: LocNguoiDung, doiLoc = vi.fn()) {
    const m = may(HangLocNguoiDung, { loc, doiLoc });
    m.ve();
    const nhap = (chu: string) => {
      const o = tatCa(m.cay(), (p) => p.props.id === "tim-nguoi-dung")[0];
      (o?.props.onChange as (e: unknown) => void)({ target: { value: chu } });
      m.ve();
    };
    return { m, nhap, doiLoc };
  }

  it("typing calls nothing at once; after the pause `doiLoc` gets the NORMALISED words, once", () => {
    vi.useFakeTimers();
    const { nhap, doiLoc, m } = chay({ tuKhoa: null, boPhan: "" });
    nhap("  0900");
    nhap(`  ${CHU}   `);
    expect(doiLoc).not.toHaveBeenCalled();
    vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS - 1);
    expect(doiLoc).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    m.ve();
    expect(doiLoc).toHaveBeenCalledTimes(1);
    expect(doiLoc).toHaveBeenCalledWith({ tuKhoa: { loai: "hopLe", tu: CHU } });
  });

  it("first mount with an empty box sends nothing (no reset of the page the list just read)", () => {
    vi.useFakeTimers();
    const { doiLoc } = chay({ tuKhoa: null, boPhan: "" });
    vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS * 2);
    expect(doiLoc).not.toHaveBeenCalled();
  });

  it("box cleared while searching → after the pause, search is dropped (`tuKhoa: null`)", () => {
    vi.useFakeTimers();
    const { nhap, doiLoc } = chay({ tuKhoa: hopLe(CHU), boPhan: "" });
    nhap("");
    vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
    expect(doiLoc).toHaveBeenCalledWith({ tuKhoa: null });
  });

  it("words too long → the refusal under the box, and NOTHING leaves the browser", () => {
    vi.useFakeTimers();
    const { nhap, doiLoc, m } = chay({ tuKhoa: null, boPhan: "" });
    nhap("x".repeat(500));
    vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
    m.ve();
    expect(doiLoc).not.toHaveBeenCalled();
    const loi = tatCa(m.cay(), (p) => p.props.id === "loi-tim-nguoi-dung")[0];
    expect(String(loi?.props.children)).toMatch(/quá dài/);
  });

  it("prototype shape: no 'Tìm' button, no unit filter; aria-label 'Tìm cán bộ', true placeholder, no `name`, no autofill", () => {
    const { m } = chay({ tuKhoa: null, boPhan: "" });
    const cay = m.cay();
    expect(tatCa(cay, (p) => p.props.type === "submit")).toHaveLength(0);
    expect(tatCa(cay, (p) => p.type === "select")).toHaveLength(0);
    const input = tatCa(cay, (p) => p.type === "input")[0]?.props ?? {};
    expect(input["aria-label"]).toBe("Tìm cán bộ");
    expect(input.placeholder).toBe(SEARCH_PLACEHOLDER);
    // The server searches name, position and the two phones — never email or unit (decision 3).
    expect(SEARCH_PLACEHOLDER).toBe("Tìm theo tên, chức danh, số điện thoại…");
    expect(input).not.toHaveProperty("name");
    expect(input.autoComplete).toBe("off");
    expect(String(input.className)).toContain("pl-9");
  });
});

/* ---- 4. đếm, đang tải ------------------------------------------------------------------------- */

/** Text of every `<p>` in the tree whose only child is a string. */
function paragraphs(cay: unknown): string[] {
  return tatCa(cay, (p) => p.type === "p" && typeof p.props.children === "string").map((p) =>
    String(p.props.children),
  );
}

describe("count line and loading (decision 3, U6/U7)", () => {
  it("`Hiển thị N/M cán bộ.` with M from GET /api/v1/staff-counts", async () => {
    const m = await moMan();
    expect(allCalls().some((c) => c.url === COUNTS_PATH)).toBe(true);
    expect(paragraphs(m.cay())).toContain("Hiển thị 1/7 cán bộ.");
  });

  it("count read refused → `Hiển thị N cán bộ.`, never a made-up total", async () => {
    countsAnswer = { status: 403, body: { code: "forbidden", message: "Không có quyền." } };
    const m = await moMan();
    expect(paragraphs(m.cay())).toContain("Hiển thị 1 cán bộ.");
  });

  it("first load → three 44px skeleton bars in a `space-y-2` box", () => {
    const m = may(DanhBaCanBo, {});
    m.ve();
    const bars = tatCa(m.cay(), (p) => p.type === Skeleton);
    expect(bars).toHaveLength(3);
    for (const b of bars) expect(b.props.className).toBe("h-11 w-full");
  });
});

/* ---- 5. xoá: hộp lý do, DELETE mang lý do (decision 1) ---------------------------------------- */

describe("delete — reason dialog, then DELETE /api/v1/staff/{id} {reason}", () => {
  it("Trash2 opens the reason dialog; an empty reason sends nothing; a reason is sent in the body", async () => {
    const m = await moMan({ canDelete: true });
    const table = phaiCo(m.cay(), BangCanBo);
    expect(table.canDelete).toBe(true);
    table.thaoTac.xoa(A);
    m.ve();

    let dialog = phaiCo(m.cay(), HopXoa);
    expect(dialog.canBo.id).toBe(A.id);
    dialog.onGui();
    m.ve();
    expect(allCalls().some((c) => c.method === "DELETE")).toBe(false);
    expect(phaiCo(m.cay(), HopXoa).loiMayChu).toBe(CAU_THIEU_LY_DO);

    phaiCo(m.cay(), HopXoa).datLyDo("Nhập trùng với dòng CB-00002");
    m.ve();
    dialog = phaiCo(m.cay(), HopXoa);
    dialog.onGui();
    await xongMang(m);

    const del = allCalls().find((c) => c.method === "DELETE");
    expect(del?.url).toBe(`/api/v1/staff/${A.id}`);
    expect(del?.than).toEqual({ reason: "Nhập trùng với dòng CB-00002" });
    // Closed after the 204, and the list is read again.
    expect(tatCa(m.cay(), (p) => p.type === HopXoa)).toHaveLength(0);
  });

  it("DENIED: without the key the table is told so (no Trash2 drawn)", async () => {
    const m = await moMan();
    expect(phaiCo(m.cay(), BangCanBo).canDelete).toBe(false);
  });
});

/* ---- 6. danh mục bộ phận/vai trò đọc lại khi màn quay về hiện (ND-01/ND-02) ------------------- */

describe("danh mục: màn Người dùng đọc lại khi được hiện lại", () => {
  it("ẩn → không đọc; hiện lại → đọc thêm đúng một lượt; đổi trang không đọc", async () => {
    const props = { active: true };
    const m = may(DanhBaCanBo, props);
    await xongMang(m);
    const doc = vi.mocked(apiDanhMuc.docDanhMucDanhBa);
    expect(doc).toHaveBeenCalledTimes(1);

    props.active = false;
    await xongMang(m);
    expect(doc).toHaveBeenCalledTimes(1);

    props.active = true;
    await xongMang(m);
    expect(doc).toHaveBeenCalledTimes(2);

    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(doc).toHaveBeenCalledTimes(2);
  });

  it("panel mở sẵn trong trạng thái ẩn → chưa đọc, tới khi được chọn", async () => {
    const props = { active: false };
    const m = may(DanhBaCanBo, props);
    await xongMang(m);
    const doc = vi.mocked(apiDanhMuc.docDanhMucDanhBa);
    expect(doc).not.toHaveBeenCalled();
    props.active = true;
    await xongMang(m);
    expect(doc).toHaveBeenCalledTimes(1);
  });
});
