import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as apiDanhMuc from "@/lib/api/danh-muc";
import { chuanHoaTuKhoaTim } from "@/lib/api/can-bo";
import type { TuKhoaHopLe } from "@/lib/api/can-bo";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { Skeleton } from "@/components/ui/skeleton";
import { HopXoa } from "@/features/danh-ba/hop-xoa";
import { CAU_THIEU_LY_DO } from "@/features/danh-ba/xoa-dong";

import { BAN_TRONG, NUT_THEM_CAN_BO } from "@/components/danh-ba/nhan-ghi-danh-ba";

import { BangCanBo, DanhBaCanBo, HangLocNguoiDung, SEARCH_DEBOUNCE_MS } from "./danh-ba-can-bo";
import type { LocNguoiDung } from "./danh-ba-can-bo";
import { OMatKhauTam, XacNhanTaiKhoan } from "./mat-khau-tam";
import { SEARCH_PLACEHOLDER, roleChangeFailed } from "./nhan-can-bo";
import { sangTrangSau, TRANG_DAU } from "./ngan-xep-con-tro";
import { StaffAccountForm } from "./staff-account-form";

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
const COUNT_QUERIES_PATH = "/api/v1/staff-count-queries";

let gia: ReturnType<typeof vi.fn>;
/** What `GET /api/v1/staff-counts` answers in the current test: a total, or a refusal. */
let countsAnswer: { status: number; body: unknown };
/**
 * What `POST /api/v1/staff-count-queries` answers for a given body. A test may return a promise it
 * resolves later, to make an older search's count arrive AFTER a newer one's.
 */
let countQueryAnswer: (body: { q: string }) => Promise<{ status: number; body: unknown }>;
/** What the list routes answer (GET and search alike). */
let listAnswer: { items: identity_canBoTomTat[]; next_cursor: string; has_more: boolean };
/**
 * The write routes of the add/edit flows, by `METHOD path`. Absent = not expected in this test: the
 * fake answers 500 so an unexpected write is visible as a failure, never as a silent success.
 */
let writeAnswers: Record<string, { status: number; body: unknown }>;

function allCalls(): LoiGoi[] {
  return (gia.mock.calls as [string, RequestInit | undefined][]).map(([url, tc]) => ({
    url,
    method: tc?.method ?? "GET",
    than:
      tc?.body === undefined || tc.body === null ? null : (JSON.parse(String(tc.body)) as Record<string, unknown>),
  }));
}

/** The LIST calls only — the count read is not part of the search/paging questions below. */
function cacLoiGoi(): LoiGoi[] {
  return allCalls().filter((c) => c.url !== COUNTS_PATH && c.url !== COUNT_QUERIES_PATH);
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
  countQueryAnswer = async () => ({ status: 200, body: { total: 3 } });
  listAnswer = { items: [A], next_cursor: "CB-00001", has_more: true };
  writeAnswers = {};
  gia = vi.fn(async (url: string, init?: RequestInit) => {
    if (url === COUNTS_PATH) return json(countsAnswer.status, countsAnswer.body);
    if (url === COUNT_QUERIES_PATH) {
      const a = await countQueryAnswer(JSON.parse(String(init?.body)) as { q: string });
      return json(a.status, a.body);
    }
    if (init?.method === "DELETE") return json(204, null);
    const method = init?.method ?? "GET";
    const isList = (method === "GET" && url.startsWith("/api/v1/staff")) || url === "/api/v1/staff/searches";
    if (!isList) {
      const w = writeAnswers[`${method} ${url}`];
      return w === undefined ? json(500, { code: "unexpected", message: "Không mong đợi." }) : json(w.status, w.body);
    }
    return json(200, listAnswer);
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

async function moMan(props: { canDelete?: boolean; selfCode?: string | null } = {}) {
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
    // Two search pages and ONE count of the search (paging does not count again).
    expect(ds.filter((x) => x.method === "POST").length).toBe(3);
    for (const { url, method } of ds) {
      for (const h of [...hinhDang(CHU), ...hinhDang("0900000000"), ...hinhDang("Nguyễn")]) {
        expect(url).not.toContain(h);
      }
      expect(url).not.toMatch(/[?&]q=/);
      if (method === "POST") expect(["/api/v1/staff/searches", COUNT_QUERIES_PATH]).toContain(url);
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

  it("prototype shape: no 'Tìm' button, no unit filter; aria-label 'Tìm cán bộ', prototype placeholder, no `name`, no autofill", () => {
    const { m } = chay({ tuKhoa: null, boPhan: "" });
    const cay = m.cay();
    expect(tatCa(cay, (p) => p.props.type === "submit")).toHaveLength(0);
    expect(tatCa(cay, (p) => p.type === "select")).toHaveLength(0);
    const input = tatCa(cay, (p) => p.type === "input")[0]?.props ?? {};
    expect(input["aria-label"]).toBe("Tìm cán bộ");
    expect(input.placeholder).toBe(SEARCH_PLACEHOLDER);
    // The prototype's words, verbatim: since 4b0b9ce3 the server also matches email and unit name.
    expect(SEARCH_PLACEHOLDER).toBe("Tìm theo tên, thư điện tử, bộ phận…");
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

/* ---- 7. sắp xếp theo cột (owner decision 08/10/2026, contract 4b0b9ce3) ----------------------- */

/** Click a sort head the way the table does: through the `onSort` the screen hands it. */
function sortBy(m: { cay: () => unknown }, key: Parameters<NonNullable<Parameters<typeof BangCanBo>[0]["onSort"]>>[0]) {
  const onSort = phaiCo(m.cay(), BangCanBo).onSort;
  if (onSort === undefined) throw new Error("the table got no onSort");
  onSort(key);
}

function lastListUrl(): URL {
  const c = cuoi();
  expect(c.method).toBe("GET");
  return new URL(c.url, "https://mot-xa.test");
}

describe("column sort — asc, desc, then back to the server's default (code asc)", () => {
  it("first click asc, second desc, third drops sort/order from the GET; the table is told each time", async () => {
    const m = await moMan();

    sortBy(m, "full_name");
    await xongMang(m);
    expect(lastListUrl().searchParams.get("sort")).toBe("full_name");
    expect(lastListUrl().searchParams.get("order")).toBe("asc");
    expect(phaiCo(m.cay(), BangCanBo).sort).toEqual({ key: "full_name", order: "asc" });

    sortBy(m, "full_name");
    await xongMang(m);
    expect(lastListUrl().searchParams.get("order")).toBe("desc");
    expect(phaiCo(m.cay(), BangCanBo).sort).toEqual({ key: "full_name", order: "desc" });

    sortBy(m, "full_name");
    await xongMang(m);
    expect(lastListUrl().searchParams.has("sort")).toBe(false);
    expect(lastListUrl().searchParams.has("order")).toBe(false);
    expect(phaiCo(m.cay(), BangCanBo).sort).toBeNull();
  });

  it("another column starts at asc, whatever the previous column's direction", async () => {
    const m = await moMan();
    sortBy(m, "status");
    sortBy(m, "status");
    await xongMang(m);
    sortBy(m, "phone");
    await xongMang(m);
    expect(lastListUrl().searchParams.get("sort")).toBe("phone");
    expect(lastListUrl().searchParams.get("order")).toBe("asc");
  });

  it("changing sort restarts from the FIRST page — the old cursor belongs to the old order", async () => {
    const m = await moMan();
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(lastListUrl().searchParams.get("cursor")).toBe("CB-00001");

    sortBy(m, "department");
    await xongMang(m);
    expect(lastListUrl().searchParams.has("cursor")).toBe(false);
    expect(lastListUrl().searchParams.get("sort")).toBe("department");
  });

  it("while searching, sort/order go in the search BODY; a sort change sends cursor \"\"", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ cursor: "CB-00001" });

    sortBy(m, "last_login_at");
    await xongMang(m);
    const c = cuoi();
    expect(c.url).toBe("/api/v1/staff/searches");
    expect(c.than).toMatchObject({ q: CHU, sort: "last_login_at", order: "asc", cursor: "" });
  });

  it("the sort survives a new search: the new words are read in the chosen order, from page one", async () => {
    const m = await moMan();
    sortBy(m, "position");
    sortBy(m, "position");
    await xongMang(m);
    hangLoc(m).doiLoc({ tuKhoa: hopLe("Trần") });
    await xongMang(m);
    expect(cuoi().than).toMatchObject({ q: "Trần", sort: "position", order: "desc", cursor: "" });
  });
});

/* ---- 8. dòng đếm khi đang tìm: POST /api/v1/staff-count-queries ------------------------------- */

function countQueryCalls(): LoiGoi[] {
  return allCalls().filter((c) => c.url === COUNT_QUERIES_PATH);
}

describe("count line while searching — M is the search's own total (owner decision 3)", () => {
  it("searching → POST /staff-count-queries with the same q/unit/published; line N/M from it", async () => {
    const m = await moMan();
    expect(countQueryCalls()).toHaveLength(0);
    expect(paragraphs(m.cay())).toContain("Hiển thị 1/7 cán bộ.");

    hangLoc(m).doiLoc({ tuKhoa: hopLe(`  ${CHU} `) });
    await xongMang(m);

    const calls = countQueryCalls();
    expect(calls).toHaveLength(1);
    expect(calls[0]?.method).toBe("POST");
    expect(calls[0]?.than).toEqual({ q: CHU, unit: "", published: null });
    expect(paragraphs(m.cay())).toContain("Hiển thị 1/3 cán bộ.");
  });

  it("paging inside a search does not count again; dropping the search goes back to the commune total", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    diToiTrang(m)(sangTrangSau(TRANG_DAU, "CB-00001"));
    await xongMang(m);
    expect(countQueryCalls()).toHaveLength(1);
    expect(paragraphs(m.cay())).toContain("Hiển thị 1/3 cán bộ.");

    hangLoc(m).doiLoc({ tuKhoa: null });
    await xongMang(m);
    expect(countQueryCalls()).toHaveLength(1);
    expect(paragraphs(m.cay())).toContain("Hiển thị 1/7 cán bộ.");
  });

  it("count read refused while searching → `Hiển thị N cán bộ.`, never the commune total", async () => {
    countQueryAnswer = async () => ({ status: 400, body: { code: "invalid_request", message: "Không hợp lệ." } });
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    const lines = paragraphs(m.cay());
    expect(lines).toContain("Hiển thị 1 cán bộ.");
    expect(lines).not.toContain("Hiển thị 1/7 cán bộ.");
  });

  it("STALE ANSWER: an older search's count arriving after a newer one's is never shown", async () => {
    const pending = new Map<string, (total: number) => void>();
    countQueryAnswer = (body) =>
      new Promise((resolve) => pending.set(body.q, (total) => resolve({ status: 200, body: { total } })));

    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe("Nguyễn") });
    await xongMang(m);
    // The list of the first search is on screen, its count is not back yet: no total at all.
    expect(paragraphs(m.cay())).toContain("Hiển thị 1 cán bộ.");

    hangLoc(m).doiLoc({ tuKhoa: hopLe("Trần") });
    await xongMang(m);
    // The new list is back, the new count is not: neither the old search's total nor the commune's.
    expect(paragraphs(m.cay())).toContain("Hiển thị 1 cán bộ.");

    pending.get("Trần")?.(9);
    await xongMang(m);
    expect(paragraphs(m.cay())).toContain("Hiển thị 1/9 cán bộ.");

    pending.get("Nguyễn")?.(5);
    await xongMang(m);
    const lines = paragraphs(m.cay());
    expect(lines).toContain("Hiển thị 1/9 cán bộ.");
    expect(lines).not.toContain("Hiển thị 1/5 cán bộ.");
  });

  it("the count route never carries the words on its URL", async () => {
    const m = await moMan();
    hangLoc(m).doiLoc({ tuKhoa: hopLe(CHU) });
    await xongMang(m);
    for (const { url } of countQueryCalls()) {
      expect(url).toBe(COUNT_QUERIES_PATH);
    }
  });
});

/* ---- 9. pager only with another page (user 09/10/2026) ---------------------------------------- */

describe("pager — hidden on a one-page list", () => {
  const pagers = (m: { cay: () => unknown }) => tatCa(m.cay(), (p) => typeof p.props.diToiTrang === "function");

  it("one page (no `has_more`, first page) → no `Trang trước / Trang sau` at all", async () => {
    listAnswer = { items: [A], next_cursor: "", has_more: false };
    const m = await moMan();
    expect(pagers(m)).toHaveLength(0);
  });

  it("another page → the pager is there", async () => {
    const m = await moMan();
    expect(pagers(m)).toHaveLength(1);
  });
});

/* ---- 10. edit: role chosen in the dialog, PUT .../role AFTER the PATCH (user 09/10/2026) --------- */

/** Run the effects and promise chains of a flow with several awaits in a row. */
async function settle(m: { ve: () => unknown }) {
  for (let i = 0; i < 6; i++) await xongMang(m);
}

const form = (m: { cay: () => unknown }) => phaiCo(m.cay(), StaffAccountForm);
const PATCH_A = `PATCH /api/v1/staff/${A.id}`;
const ROLE_A = `PUT /api/v1/staff/${A.id}/role`;

function writes(): LoiGoi[] {
  return allCalls().filter((c) => c.method !== "GET" && c.url !== "/api/v1/staff/searches" && c.url !== COUNT_QUERIES_PATH);
}

describe("edit dialog — role change calls the role route AFTER the patch", () => {
  async function openEdit(props: { selfCode?: string | null } = {}) {
    const m = await moMan(props);
    phaiCo(m.cay(), BangCanBo).thaoTac.sua(A);
    m.ve();
    return m;
  }

  it("role changed → PATCH /staff/{id}, THEN PUT /staff/{id}/role {role_id}; dialog closes on success", async () => {
    writeAnswers[PATCH_A] = { status: 200, body: A };
    writeAnswers[ROLE_A] = { status: 200, body: { ...A, role_id: "VT-2" } };
    const m = await openEdit();
    expect(form(m).roleId).toBe("");
    expect(form(m).roleChoice).toBe("editable");
    form(m).setRoleId("VT-2");
    m.ve();
    form(m).onSubmit();
    await settle(m);

    const w = writes();
    expect(w.map((c) => `${c.method} ${c.url}`)).toEqual([PATCH_A, ROLE_A]);
    expect(w[0]?.than).not.toHaveProperty("role_id");
    expect(w[1]?.than).toEqual({ role_id: "VT-2" });
    expect(tatCa(m.cay(), (p) => p.type === StaffAccountForm)).toHaveLength(0);
  });

  it("role unchanged → the PATCH alone, no role route", async () => {
    writeAnswers[PATCH_A] = { status: 200, body: A };
    const m = await openEdit();
    form(m).setDraft({ ...form(m).draft, chucDanh: "Công chức Văn phòng" });
    m.ve();
    form(m).onSubmit();
    await settle(m);
    expect(writes().map((c) => `${c.method} ${c.url}`)).toEqual([PATCH_A]);
  });

  it("role route refused (#13/#14) → its sentence in the dialog; Lưu again retries the ROLE only", async () => {
    const sentence = "Xã phải luôn còn ít nhất một người quản trị.";
    writeAnswers[PATCH_A] = { status: 200, body: A };
    writeAnswers[ROLE_A] = { status: 409, body: { code: "last_admin", message: sentence } };
    const m = await openEdit();
    form(m).setRoleId("VT-2");
    m.ve();
    form(m).onSubmit();
    await settle(m);

    expect(form(m).serverError).toBe(roleChangeFailed(sentence));
    expect(form(m).serverError).toContain(sentence);

    form(m).onSubmit();
    await settle(m);
    const w = writes().map((c) => `${c.method} ${c.url}`);
    expect(w.filter((x) => x === PATCH_A)).toHaveLength(1);
    expect(w.filter((x) => x === ROLE_A)).toHaveLength(2);
  });

  it("DENIED, own row (#14): the choice is disabled and no role route is ever called", async () => {
    writeAnswers[PATCH_A] = { status: 200, body: A };
    const m = await openEdit({ selfCode: A.code });
    expect(form(m).roleChoice).toBe("own");
    // Even if a value slipped through (the radios are disabled), the screen does not send it.
    form(m).setRoleId("VT-2");
    m.ve();
    form(m).onSubmit();
    await settle(m);
    expect(writes().map((c) => `${c.method} ${c.url}`)).toEqual([PATCH_A]);
  });
});

/* ---- 11. add: with an email, the account is granted at once (user 09/10/2026) --------------------- */

const B: identity_canBoTomTat = { ...A, id: "01J00000000000000000000002", code: "CB-00002", full_name: "Trần Thị B", email: "ttb@demo.invalid" };
const GRANT_B = `POST /api/v1/staff/${B.id}/account`;
/** Fake, and obviously so (rule 8). */
const FAKE_PASSWORD = "mat-khau-gia-de-kiem-tra";

describe("add dialog — email filled chains the grant-account route", () => {
  async function openAdd() {
    const m = await moMan();
    const add = tatCa(m.cay(), (p) => p.props.children === NUT_THEM_CAN_BO)[0];
    (add?.props.onClick as () => void)();
    m.ve();
    return m;
  }

  it("email → POST /staff, THEN POST /staff/{id}/account; the one-time password opens; form closed", async () => {
    writeAnswers["POST /api/v1/staff"] = { status: 201, body: B };
    writeAnswers[GRANT_B] = { status: 201, body: { staff: { ...B, has_account: true }, temporary_password: FAKE_PASSWORD } };
    const m = await openAdd();
    expect(form(m).open.kieu).toBe("them");
    form(m).setDraft({ ...BAN_TRONG, hoTen: "Trần Thị B", email: "ttb@demo.invalid" });
    m.ve();
    form(m).onSubmit();
    await settle(m);

    const w = writes();
    expect(w.map((c) => `${c.method} ${c.url}`)).toEqual(["POST /api/v1/staff", GRANT_B]);
    expect(w[0]?.than).toMatchObject({ full_name: "Trần Thị B", email: "ttb@demo.invalid" });
    expect(w[0]?.than).not.toHaveProperty("password");
    expect(phaiCo(m.cay(), OMatKhauTam).matKhauTam).toMatchObject({ kieu: "cap", maCanBo: B.code, matKhau: FAKE_PASSWORD });
    expect(tatCa(m.cay(), (p) => p.type === StaffAccountForm)).toHaveLength(0);
  });

  it("grant refused → the entry stays, the grant confirmation opens with the server's sentence (retry)", async () => {
    const sentence = "Thư điện tử này đã được dùng cho một tài khoản khác.";
    writeAnswers["POST /api/v1/staff"] = { status: 201, body: B };
    writeAnswers[GRANT_B] = { status: 409, body: { code: "email_taken", message: sentence } };
    const m = await openAdd();
    form(m).setDraft({ ...BAN_TRONG, hoTen: "Trần Thị B", email: "ttb@demo.invalid" });
    m.ve();
    form(m).onSubmit();
    await settle(m);

    const confirm = phaiCo(m.cay(), XacNhanTaiKhoan);
    expect(confirm.dangMo).toMatchObject({ kieu: "cap", canBo: { id: B.id } });
    expect(confirm.loiMayChu).toBe(sentence);
    expect(tatCa(m.cay(), (p) => p.type === OMatKhauTam)).toHaveLength(0);
    // No delete of the entry: the only writes are the add and the one grant attempt.
    expect(writes().map((c) => `${c.method} ${c.url}`)).toEqual(["POST /api/v1/staff", GRANT_B]);
  });

  it("no email → the directory entry only, no grant call", async () => {
    writeAnswers["POST /api/v1/staff"] = { status: 201, body: { ...B, email: "" } };
    const m = await openAdd();
    form(m).setDraft({ ...BAN_TRONG, hoTen: "Trần Thị B" });
    m.ve();
    form(m).onSubmit();
    await settle(m);
    expect(writes().map((c) => `${c.method} ${c.url}`)).toEqual(["POST /api/v1/staff"]);
    expect(tatCa(m.cay(), (p) => p.type === OMatKhauTam || p.type === XacNhanTaiKhoan)).toHaveLength(0);
  });
});
