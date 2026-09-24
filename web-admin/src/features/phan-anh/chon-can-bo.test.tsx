import type { ReactElement, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";

import { congThaoTac } from "./nhan-phieu";
import { ChiTietPhieu } from "./so-phan-anh";

/**
 * THE LAST UNGUARDED LINK OF THE ASSIGNEE CHAIN — the value the picker actually submits.
 *
 * The chain is: identity `nguoi_dung.ma` → directory `code` → THIS SELECT → POST `assignee` →
 * `can_bo_xu_ly_id` → compared with `Principal.Ma` by the holder rule and by `scope=mine`. Every other
 * link has a test on its own side. This one did not: `so-phan-anh.test.tsx` renders the form with no
 * unit chosen, where the officer select is empty by design, and `phieu-phan-anh.test.ts` proves only
 * that `chuyenXuLyPhieu` forwards the string it is GIVEN.
 *
 * WHY IT IS WORTH A TEST: `service-petitions` does not verify the assignee against identity
 * (`domain.KiemPhanCong`, stated there), so an option carrying `full_name` or `department_id` instead of
 * `code` is accepted, stored, and shown as "assigned" — while the assignee can never advance it and
 * never sees it under "Giao cho tôi". Every field of `identity_canBoChonNguoiRa` is a `string`, so `tsc`
 * cannot tell them apart.
 *
 * HOW, WITHOUT A DOM: the selected unit and officer live in `useState`, and a browser event is the only
 * way to change them. `useState` is therefore replaced FOR THIS FILE with a plain function that hands
 * out seeded initial values in call order, and `ChiTietPhieu` is called as a function to get its element
 * tree. That tree is then read (option values) and driven (the form's `onSubmit`) directly.
 *
 * THE SEEDING IS BY CALL ORDER — linhVucChon, boPhanChon, canBoChon, ketQua (then reNhanhMo, unseeded). A reorder of those hooks
 * makes the seeded unit land elsewhere and this file turns RED, never silently green: the assertions
 * require the officer select to be populated.
 */
const hat = vi.hoisted(() => ({ gieo: [] as unknown[] }));

vi.mock("react", async (importOriginal) => {
  const thuc = await importOriginal<typeof import("react")>();
  return {
    ...thuc,
    useState: <T,>(dau: T | (() => T)) => {
      const gieo = hat.gieo.length > 0 ? hat.gieo.shift() : undefined;
      const giaTri = gieo !== undefined ? gieo : typeof dau === "function" ? (dau as () => T)() : dau;
      return [giaTri, () => {}];
    },
  };
});

type ThuocTinh = Record<string, unknown> & { children?: ReactNode };

/** Every element of a tree, depth first. Components are NOT expanded — only the inline markup. */
function moiPhanTu(nut: ReactNode, ra: ReactElement<ThuocTinh>[] = []): ReactElement<ThuocTinh>[] {
  if (Array.isArray(nut)) {
    for (const con of nut) moiPhanTu(con as ReactNode, ra);
    return ra;
  }
  if (nut !== null && typeof nut === "object" && "props" in nut) {
    const pt = nut as ReactElement<ThuocTinh>;
    ra.push(pt);
    moiPhanTu(pt.props.children, ra);
  }
  return ra;
}

function phieu(): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-xu-ly",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ.",
    address: "",
    reporter_name: "",
    reporter_phone: "",
    anonymous: true,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: "2026-09-09T09:20:00Z",
    resolve_due: "2026-09-10T09:20:00Z",
    classify_due: "2026-09-09T11:20:00Z",
    unit: "",
    assignee: "",
    result: "",
    public: false,
  };
}

const BO_PHAN: identity_boPhanRa[] = [
  { id: "01JBOPHAN", code: "vp", name: "VĂN PHÒNG", parent_id: "", order: 0, staff_count: 0 },
  { id: "01JKHAC", code: "dc", name: "ĐỊA CHÍNH", parent_id: "", order: 1, staff_count: 0 },
];

// Code, name and unit are three DIFFERENT strings on every row, so whichever field an option carries
// is identifiable from its value alone.
const DANH_BA: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: {
    items: [
      { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "01JBOPHAN" },
      { code: "CB-00124", full_name: "Phạm Văn D", position: "", department_id: "01JBOPHAN" },
      { code: "CB-00200", full_name: "Lê Văn C", position: "Trưởng thôn", department_id: "01JKHAC" },
    ],
  },
};

function dung(boPhanChon: string, canBoChon: string) {
  const goi: Array<[string, string | undefined]> = [];
  hat.gieo = [undefined, boPhanChon, canBoChon, undefined];
  const cay = ChiTietPhieu({
    phieu: phieu(),
    bayGio: new Date("2026-09-10T02:00:00Z"),
    cong: congThaoTac(false, true, false),
    tenBoPhan: new Map(BO_PHAN.map((b) => [b.id, b.name])),
    boPhan: BO_PHAN,
    danhBa: DANH_BA,
    dangGui: false,
    loiGhi: null,
    dong: () => {},
    phanLoai: () => {},
    chuyenXuLy: (boPhanID, maCanBo) => {
      goi.push([boPhanID, maCanBo]);
    },
    tienTrangThai: () => {},
    dongPhieuLai: () => {},
    khongTiepNhan: () => {},
    chuyenCapTren: () => {},
  });
  expect(hat.gieo, "ChiTietPhieu không còn gọi useState đủ bốn lần theo thứ tự đã gieo").toEqual([]);

  const tatCa = moiPhanTu(cay);
  const oCanBo = tatCa.find((p) => p.type === "select" && p.props.id === "chon-can-bo");
  if (oCanBo === undefined) throw new Error("không thấy ô chọn cán bộ #chon-can-bo");
  const luaChon = moiPhanTu(oCanBo.props.children)
    .filter((p) => p.type === "option")
    .map((p) => String(p.props.value));
  const form = tatCa.find(
    (p) => p.type === "form" && moiPhanTu(p.props.children).some((c) => c.props.id === "chon-can-bo"),
  );
  if (form === undefined) throw new Error("không thấy biểu mẫu chứa ô chọn cán bộ");
  const gui = () =>
    (form.props.onSubmit as (e: { preventDefault: () => void }) => void)({ preventDefault: () => {} });
  return { oCanBo, luaChon, gui, goi };
}

describe("ô chọn cán bộ xử lý — thứ ĐƯỢC GỬI ĐI là mã cán bộ", () => {
  it("mỗi lựa chọn mang `code` làm giá trị, và chỉ người của bộ phận đang chọn", () => {
    // ĐỘT BIẾN ĐÃ CHẠY: `value={cb.full_name}` hoặc `value={cb.department_id}` ở <option> — ĐỎ.
    const { luaChon } = dung("01JBOPHAN", "");
    expect(luaChon).toEqual(["", "CB-00123", "CB-00124"]);
  });

  it("bấm Chuyển xử lý: gửi (bộ phận, MÃ cán bộ) — đúng chuỗi của lựa chọn đang chọn", () => {
    // ĐỘT BIẾN ĐÃ CHẠY: đổi chỗ hai đối số trong onSubmit, hoặc bỏ `canBoChon` — ĐỎ.
    const { oCanBo, gui, goi } = dung("01JBOPHAN", "CB-00123");
    expect(oCanBo.props.value).toBe("CB-00123");
    gui();
    expect(goi).toEqual([["01JBOPHAN", "CB-00123"]]);
  });

  it("để bộ phận phân công: KHÔNG gửi mã cán bộ nào, kể cả chuỗi rỗng", () => {
    const { gui, goi } = dung("01JBOPHAN", "");
    gui();
    expect(goi).toEqual([["01JBOPHAN", undefined]]);
  });
});
