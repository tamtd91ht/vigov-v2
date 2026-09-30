import type { ComponentProps } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { sangTrangSau, TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";
import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import * as apiCanBo from "@/lib/api/can-bo";
import * as apiDanhMuc from "@/lib/api/danh-muc";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BieuMauGhiCanBo } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import { banTuCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";

import { BangLienHe } from "./bang-lien-he";
import { CAU_CHUA_XAC_NHAN } from "./cong-khai";
import { DanhBaLienHe, HangLoc } from "./danh-ba-lien-he";
import { HopCongKhai } from "./hop-cong-khai";
import { HopXoa } from "./hop-xoa";
import { CAU_CO_TAI_KHOAN, CAU_THIEU_LY_DO } from "./xoa-dong";

/**
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * LUỒNG CỦA MÀN DANH BẠ — CHẠY CHÍNH `DanhBaLienHe`, KHÔNG PHẢI MỘT BẢN CHÉP CỦA NÓ.
 *
 * Mọi phép quyết định của màn (`yeuCauCongKhai`, `yeuCauRut`, `yeuCauXoa`, `apLoc`) đã có ca kiểm
 * thuần. Thứ KHÔNG ca nào chạm tới là CHỖ NỐI: màn có thật sự đi qua các phép ấy không, có mở hộp
 * với ô tick trống không, có trao nút 🗑 chỉ cho người cầm `admin.user.delete` không, có đọc lại
 * với đúng bộ lọc đang áp không. Sai ở những chỗ ấy thì mọi ca thuần vẫn xanh.
 *
 * KHÔNG THÊM PHỤ THUỘC (`vitest.config.mts` nói vì sao không jsdom): bốn hook React mà màn dùng —
 * `useState`, `useEffect`, `useCallback`, `useMemo` — được thay bằng một bộ chạy tối thiểu dưới đây,
 * và màn được GỌI NHƯ MỘT HÀM. Cây trả về giữ nguyên các component con (`BangLienHe`, `HopCongKhai`,
 * `HopXoa`…) dưới dạng phần tử CHƯA dựng, nên ca kiểm đọc được đúng props màn trao cho chúng và gọi
 * đúng hàm màn trao — y như một lần bấm nút gọi `onClick` của nút ấy. Bốn tuyến mạng bị thay bằng
 * `vi.fn`, nên thứ được kiểm là ĐỐI SỐ màn gửi đi.
 *
 * GIỚI HẠN, NÓI THẲNG: bộ chạy này không phải React. Nó không kiểm thứ tự render, không gộp setState,
 * không có Strict Mode. Nó đủ cho câu hỏi "màn gửi gì, khi nào" — câu hỏi của tệp này — và không đủ
 * cho câu hỏi về thời điểm hay hiệu năng.
 *
 * Số điện thoại là số giả `09000000xx` (luật 3, bất biến 5).
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */

/* ---- bộ chạy hook tối thiểu ------------------------------------------------------------------ */

type OHook = { gia: unknown };
type TrangThaiChay = {
  o: OHook[];
  i: number;
  choChay: (() => void)[];
};

const H = vi.hoisted(() => ({
  hienTai: null as null | { o: { gia: unknown }[]; i: number; choChay: (() => void)[] },
  phien: null as unknown,
}));

vi.mock("react", async (goc) => {
  const R = await goc<typeof import("react")>();
  const lay = () => {
    const c = H.hienTai;
    if (c === null) throw new Error("hook gọi ngoài một lượt dựng");
    const i = c.i++;
    return { c, i };
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

vi.mock("@/features/phien/phien-hien-tai", async (goc) => ({
  ...(await goc<typeof import("@/features/phien/phien-hien-tai")>()),
  usePhien: () => H.phien,
}));

vi.mock("@/lib/api/can-bo", async (goc) => ({
  ...(await goc<typeof import("@/lib/api/can-bo")>()),
  docTrangDanhBa: vi.fn(),
  datCongKhaiCanBo: vi.fn(),
  xoaCanBo: vi.fn(),
  suaCanBo: vi.fn(),
}));

vi.mock("@/lib/api/danh-muc", async (goc) => ({
  ...(await goc<typeof import("@/lib/api/danh-muc")>()),
  layDanhMucBoPhan: vi.fn(),
}));

const docTrang = vi.mocked(apiCanBo.docTrangDanhBa);
const congKhai = vi.mocked(apiCanBo.datCongKhaiCanBo);
const xoa = vi.mocked(apiCanBo.xoaCanBo);
const sua = vi.mocked(apiCanBo.suaCanBo);

/** Dựng một component bằng bộ chạy trên; mỗi `ve()` là một lượt render, rồi chạy effect đến hạn. */
function may<P extends object>(Comp: (p: P) => unknown, props: P) {
  const trang: TrangThaiChay = { o: [], i: 0, choChay: [] };
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

/**
 * Dựng lại (để effect vừa đến hạn chạy và phát lời gọi mạng), chờ mọi promise đã hẹn (lời gọi giả +
 * `.then` + `.finally`), rồi dựng lại lần nữa để cây mang kết quả.
 */
async function xongMang(m: { ve: () => unknown }) {
  m.ve();
  await new Promise((r) => setTimeout(r, 0));
  m.ve();
}

/* ---- đọc cây phần tử ------------------------------------------------------------------------- */

type PhanTu = { type: unknown; props: Record<string, unknown> };

function laPhanTu(x: unknown): x is PhanTu {
  return typeof x === "object" && x !== null && "props" in x && "type" in x;
}

function tatCa(goc: unknown, dung: (p: PhanTu) => boolean): PhanTu[] {
  const ra: PhanTu[] = [];
  const duyet = (n: unknown) => {
    if (Array.isArray(n)) return n.forEach(duyet);
    if (!laPhanTu(n)) return;
    if (dung(n)) ra.push(n);
    duyet(n.props.children);
  };
  duyet(goc);
  return ra;
}

/** Props màn trao cho component con `Comp` — `null` nếu màn không dựng nó. */
function propsCua<P>(goc: unknown, Comp: (p: P) => unknown): P | null {
  const ds = tatCa(goc, (p) => p.type === Comp);
  if (ds.length > 1) throw new Error("dựng hai lần một component chỉ được có một");
  return (ds[0]?.props as P | undefined) ?? null;
}

function phaiCo<P>(goc: unknown, Comp: (p: P) => unknown): P {
  const p = propsCua(goc, Comp);
  if (p === null) throw new Error("màn không dựng component cần kiểm");
  return p;
}

/* ---- dữ liệu --------------------------------------------------------------------------------- */

function canBo(ghiDe: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Chuyên viên",
    department_id: "BP-LE",
    role_id: "",
    phone: "02350000001",
    mobile: "0900000001",
    has_account: false,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...ghiDe,
  };
}

const A = canBo({ display_order: 3 });
const B = canBo({
  id: "01J00000000000000000000002",
  code: "CB-00124",
  full_name: "Trần Thị B",
  mobile: "0900000002",
  published: true,
  display_order: 4,
  consent_recorded_at: "2026-09-20T02:00:00Z",
});
const C = canBo({ id: "01J00000000000000000000003", code: "CB-00125", full_name: "Lê Văn C", has_account: true });

function phien(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { permissions } } as unknown as PhienDaDoc;
}

const DU_QUYEN = ["admin.user", "content.update", "admin.user.delete"];

beforeEach(() => {
  H.phien = phien(DU_QUYEN);
  docTrang.mockResolvedValue({ ok: true, duLieu: { items: [A, B, C], next_cursor: "CB-00125", has_more: true } });
  congKhai.mockImplementation(async (_id, yc) => ({ ok: true, duLieu: { ...A, published: yc.congKhai } }));
  xoa.mockResolvedValue({ ok: true, duLieu: null });
  sua.mockResolvedValue({ ok: true, duLieu: A });
  vi.mocked(apiDanhMuc.layDanhMucBoPhan).mockResolvedValue({
    ok: true,
    duLieu: { items: [{ id: "BP-LE", code: "le", name: "VĂN PHÒNG", parent_id: "", order: 1, staff_count: 2 }] },
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

/** Mở màn, chờ trang đầu về. */
async function moMan() {
  const m = may(DanhBaLienHe, {});
  m.ve();
  await xongMang(m);
  return m;
}

/* ---- công khai ------------------------------------------------------------------------------- */

describe("công khai qua màn thật — không tick thì không có lời gọi nào", () => {
  it("mở hộp: ô tick TRỐNG, thứ tự nạp giá trị đang có; gửi khi chưa tick → không gọi mạng, nói lý do", async () => {
    const m = await moMan();
    phaiCo(m.cay(), BangLienHe).congKhai?.onThem(A);
    m.ve();

    const hop = phaiCo(m.cay(), HopCongKhai);
    expect(hop.ban).toEqual({ daHoiY: false, thuTu: "3" });

    // Nút đang mờ; `onGui` gọi thẳng là đúng ca "ai đó bật lại nút bằng DevTools".
    hop.onGui();
    m.ve();
    expect(congKhai).not.toHaveBeenCalled();
    expect(phaiCo(m.cay(), HopCongKhai).loiMayChu).toBe(CAU_CHUA_XAC_NHAN);
  });

  it("tick rồi gửi → ĐÚNG MỘT lời gọi, consent `true`, thứ tự là SỐ đang có; xong thì đóng hộp và đọc lại", async () => {
    const m = await moMan();
    phaiCo(m.cay(), BangLienHe).congKhai?.onThem(A);
    m.ve();
    const hop = phaiCo(m.cay(), HopCongKhai);
    hop.datBan({ ...hop.ban, daHoiY: true });
    m.ve();
    phaiCo(m.cay(), HopCongKhai).onGui();
    await xongMang(m);

    expect(congKhai).toHaveBeenCalledTimes(1);
    expect(congKhai).toHaveBeenCalledWith(A.id, { congKhai: true, daXacNhanDongY: true, thuTu: 3 });
    expect(propsCua(m.cay(), HopCongKhai)).toBeNull();
    expect(docTrang).toHaveBeenCalledTimes(2);
  });

  it("mở LẠI hộp — sau khi đã tick rồi huỷ, hay sau một lần công khai xong — ô tick lại TRỐNG", async () => {
    const m = await moMan();
    const moVaTick = () => {
      phaiCo(m.cay(), BangLienHe).congKhai?.onThem(A);
      m.ve();
      const hop = phaiCo(m.cay(), HopCongKhai);
      hop.datBan({ ...hop.ban, daHoiY: true });
      m.ve();
      return phaiCo(m.cay(), HopCongKhai);
    };

    moVaTick().onHuy();
    m.ve();
    phaiCo(m.cay(), BangLienHe).congKhai?.onThem(A);
    m.ve();
    expect(phaiCo(m.cay(), HopCongKhai).ban.daHoiY).toBe(false);

    phaiCo(m.cay(), HopCongKhai).onHuy();
    m.ve();
    moVaTick().onGui();
    await xongMang(m);
    phaiCo(m.cay(), BangLienHe).congKhai?.onThem(A);
    m.ve();
    expect(phaiCo(m.cay(), HopCongKhai).ban.daHoiY).toBe(false);
  });

  it("rút: consent `false` và thứ tự ĐANG CÓ — kể cả khi hộp công khai vừa rồi đã được tick", async () => {
    const m = await moMan();
    // Tick trong hộp công khai của A, KHÔNG gửi, rồi mở hộp rút của B.
    phaiCo(m.cay(), BangLienHe).congKhai?.onThem(A);
    m.ve();
    const hopA = phaiCo(m.cay(), HopCongKhai);
    hopA.datBan({ ...hopA.ban, daHoiY: true });
    m.ve();
    phaiCo(m.cay(), BangLienHe).congKhai?.onRut(B);
    m.ve();
    phaiCo(m.cay(), HopCongKhai).onGui();
    await xongMang(m);

    expect(congKhai).toHaveBeenCalledTimes(1);
    const [id, yc] = congKhai.mock.calls[0] ?? [];
    expect(id).toBe(B.id);
    expect(yc).toEqual({ congKhai: false, daXacNhanDongY: false, thuTu: 4 });
    expect(apiCanBo.thanCongKhai(yc as apiCanBo.YeuCauCongKhai)).toEqual({
      published: false,
      consent_confirmed: false,
      display_order: 4,
    });
  });
});

/* ---- khoá quyền → nút ------------------------------------------------------------------------ */

describe("màn trao nút theo ĐÚNG khoá của phiên", () => {
  async function bangVoi(quyen: PhienDaDoc) {
    H.phien = quyen;
    const m = await moMan();
    return phaiCo(m.cay(), BangLienHe);
  }

  it("chỉ `admin.user` → bảng KHÔNG nhận hành động Mini App lẫn nút 🗑 (ca bị từ chối)", async () => {
    const bang = await bangVoi(phien(["admin.user"]));
    expect(bang.congKhai).toBeUndefined();
    expect(bang.onXoa).toBeUndefined();
  });

  it("`content.update` không mở 🗑; `admin.user.delete` không mở Mini App", async () => {
    const chiCK = await bangVoi(phien(["admin.user", "content.update"]));
    expect(chiCK.congKhai).toBeDefined();
    expect(chiCK.onXoa).toBeUndefined();

    const chiXoa = await bangVoi(phien(["admin.user", "admin.user.delete"]));
    expect(chiXoa.congKhai).toBeUndefined();
    expect(chiXoa.onXoa).toBeDefined();
  });

  it("phiên chưa đọc xong hay đọc hỏng → không nút nào (fail closed)", async () => {
    for (const p of [null, { ok: false, thongBao: "Phiên hết hạn." } as PhienDaDoc]) {
      const bang = await bangVoi(p);
      expect(bang.congKhai).toBeUndefined();
      expect(bang.onXoa).toBeUndefined();
    }
  });
});

/* ---- xoá ------------------------------------------------------------------------------------- */

describe("xoá qua màn thật", () => {
  it("lý do rỗng → không gọi mạng; dòng có tài khoản → không gọi mạng, kể cả khi `onGui` bị gọi thẳng", async () => {
    const m = await moMan();
    phaiCo(m.cay(), BangLienHe).onXoa?.(A);
    m.ve();
    phaiCo(m.cay(), HopXoa).datLyDo("   ");
    m.ve();
    phaiCo(m.cay(), HopXoa).onGui();
    m.ve();
    expect(phaiCo(m.cay(), HopXoa).loiMayChu).toBe(CAU_THIEU_LY_DO);

    phaiCo(m.cay(), HopXoa).onHuy();
    m.ve();
    phaiCo(m.cay(), BangLienHe).onXoa?.(C);
    m.ve();
    phaiCo(m.cay(), HopXoa).datLyDo("nhập trùng");
    m.ve();
    phaiCo(m.cay(), HopXoa).onGui();
    m.ve();
    expect(phaiCo(m.cay(), HopXoa).loiMayChu).toBe(CAU_CO_TAI_KHOAN);

    expect(xoa).not.toHaveBeenCalled();
  });

  it("mở hộp xoá lần hai: lý do của lần trước KHÔNG còn", async () => {
    const m = await moMan();
    phaiCo(m.cay(), BangLienHe).onXoa?.(A);
    m.ve();
    phaiCo(m.cay(), HopXoa).datLyDo("lý do cho người khác");
    m.ve();
    phaiCo(m.cay(), HopXoa).onHuy();
    m.ve();
    phaiCo(m.cay(), BangLienHe).onXoa?.(B);
    m.ve();
    expect(phaiCo(m.cay(), HopXoa).lyDo).toBe("");
  });

  it("sau 204, đọc lại với CÙNG chữ tìm, bộ lọc khối, bộ lọc hiển thị và con trỏ — không về trang đầu", async () => {
    const m = await moMan();
    const tu = apiCanBo.chuanHoaTuKhoaTim("Nguyễn");
    if (tu.loai !== "hopLe") throw new Error("chữ mẫu phải hợp lệ");

    // Tìm, lọc khối, lọc "đang hiện", rồi sang trang 2.
    const loc = phaiCo(m.cay(), HangLoc);
    loc.doiLoc({ tuKhoa: tu });
    await xongMang(m);
    phaiCo(m.cay(), HangLoc).doiLoc({ boPhan: "BP-LE" });
    await xongMang(m);
    phaiCo(m.cay(), HangLoc).doiLoc({ hienThi: "1" });
    await xongMang(m);
    const trang = tatCa(m.cay(), (p) => typeof p.props.diToiTrang === "function")[0];
    const diToi = trang?.props.diToiTrang as (x: ReturnType<typeof sangTrangSau>) => void;
    diToi(sangTrangSau(TRANG_DAU, "CB-00125"));
    await xongMang(m);

    const truocXoa = docTrang.mock.calls.at(-1);
    expect(truocXoa).toEqual([tu, { boPhan: "BP-LE", congKhai: true, cursor: "CB-00125" }]);

    phaiCo(m.cay(), BangLienHe).onXoa?.(A);
    m.ve();
    phaiCo(m.cay(), HopXoa).datLyDo("  nhập trùng  ");
    m.ve();
    phaiCo(m.cay(), HopXoa).onGui();
    await xongMang(m);

    expect(xoa).toHaveBeenCalledWith(A.id, { loai: "hopLe", lyDo: "nhập trùng" });
    expect(propsCua(m.cay(), HopXoa)).toBeNull();
    const soLanDoc = docTrang.mock.calls.length;
    expect(docTrang.mock.calls.at(-1)).toEqual(truocXoa);
    expect(soLanDoc).toBe(6);
  });

  it("máy chủ từ chối (409) → hộp CÒN MỞ với câu máy chủ, và KHÔNG đọc lại", async () => {
    const cau = "Cán bộ này đang có tài khoản đăng nhập nên không xoá được.";
    xoa.mockResolvedValueOnce({ ok: false, thongBao: cau });
    const m = await moMan();
    phaiCo(m.cay(), BangLienHe).onXoa?.(A);
    m.ve();
    phaiCo(m.cay(), HopXoa).datLyDo("nhập trùng");
    m.ve();
    phaiCo(m.cay(), HopXoa).onGui();
    await xongMang(m);

    expect(phaiCo(m.cay(), HopXoa).loiMayChu).toBe(cau);
    expect(docTrang).toHaveBeenCalledTimes(1);
  });
});

/* ---- biểu mẫu sửa dùng chung ----------------------------------------------------------------- */

describe("sửa qua màn Danh bạ — `has_zalo` chỉ đi khi đổi", () => {
  it("lưu mà không đụng ô Có Zalo → PATCH KHÔNG mang `has_zalo`; đổi ô ấy → mang đúng giá trị mới", async () => {
    const m = await moMan();
    phaiCo(m.cay(), BangLienHe).onSua(A);
    m.ve();
    const bm = phaiCo(m.cay(), BieuMauGhiCanBo);
    expect(bm.ban).toEqual(banTuCanBo(A));
    bm.onGui();
    await xongMang(m);
    expect(sua.mock.calls[0]?.[1]).not.toHaveProperty("has_zalo");

    phaiCo(m.cay(), BangLienHe).onSua(A);
    m.ve();
    const bm2 = phaiCo(m.cay(), BieuMauGhiCanBo);
    bm2.datBan({ ...bm2.ban, coZalo: true });
    m.ve();
    phaiCo(m.cay(), BieuMauGhiCanBo).onGui();
    await xongMang(m);
    expect(sua.mock.calls[1]?.[1]).toHaveProperty("has_zalo", true);
  });
});

/* ---- ô tìm ----------------------------------------------------------------------------------- */

describe("ô tìm — chữ tìm chỉ đi tới `doiLoc`, không tới log", () => {
  const BO_PHAN = [{ id: "BP-LE", name: "VĂN PHÒNG" }];

  function guiTim(chu: string, doiLoc: ComponentProps<typeof HangLoc>["doiLoc"]) {
    const m = may(HangLoc, { loc: { tuKhoa: null, boPhan: "", hienThi: "" }, boPhan: BO_PHAN, doiLoc });
    m.ve();
    const oTim = tatCa(m.cay(), (p) => p.props.id === "tim-danh-ba")[0];
    (oTim?.props.onChange as (e: unknown) => void)({ target: { value: chu } });
    m.ve();
    const form = tatCa(m.cay(), (p) => p.type === "form")[0];
    let chan = false;
    (form?.props.onSubmit as (e: unknown) => void)({ preventDefault: () => (chan = true) });
    m.ve();
    return { m, chan };
  }

  it("Enter với chữ hợp lệ → `doiLoc` nhận chữ ĐÃ chuẩn hoá; mặc định của trình duyệt bị chặn", () => {
    const doiLoc = vi.fn();
    const { chan } = guiTim("  0900000000   Nguyễn ", doiLoc);
    expect(chan).toBe(true);
    expect(doiLoc).toHaveBeenCalledWith({ tuKhoa: { loai: "hopLe", tu: "0900000000 Nguyễn" } });
  });

  it("chữ quá dài → KHÔNG đổi bộ lọc, câu từ chối không nhắc lại chữ đã gõ", () => {
    const doiLoc = vi.fn();
    const go = `0900000000${"a".repeat(apiCanBo.TU_KHOA_TIM_TOI_DA)}`;
    const { m } = guiTim(go, doiLoc);
    expect(doiLoc).not.toHaveBeenCalled();
    const loi = tatCa(m.cay(), (p) => p.props.id === "loi-tim-danh-ba")[0];
    expect(String(loi?.props.children)).not.toContain("0900000000");
    expect(String(loi?.props.children)).not.toBe("");
  });

  it("cả luồng tìm → đọc → xoá không ghi gì ra console", async () => {
    const nghe = (["log", "info", "warn", "error", "debug"] as const).map((k) =>
      vi.spyOn(console, k).mockImplementation(() => undefined),
    );
    guiTim("0900000000", vi.fn());
    const m = await moMan();
    const tu = apiCanBo.chuanHoaTuKhoaTim("0900000000");
    if (tu.loai !== "hopLe") throw new Error("chữ mẫu phải hợp lệ");
    phaiCo(m.cay(), HangLoc).doiLoc({ tuKhoa: tu });
    await xongMang(m);
    for (const n of nghe) expect(n).not.toHaveBeenCalled();
    expect(docTrang.mock.calls.at(-1)?.[1]).toEqual({ boPhan: "", congKhai: null, cursor: null });
  });
});
