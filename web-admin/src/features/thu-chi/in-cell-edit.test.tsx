// @vitest-environment jsdom
//
// jsdom: the interactions of the prototype's tree and entries dialog — click a name or a figure, type,
// Enter / Esc / leave the box — checked by WHAT IS SENT (the `saveLine` body, the PATCH on the wire)
// and what the officer reads (the box, the refusal under it, the toast). The pure decisions are in
// `nhan-thu-chi.test.ts`; this file checks the component wires them.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing provider
import type {
  finance_cotRa,
  finance_danhSachDotRa,
  finance_dongRa,
  identity_phienHienTaiRa,
} from "@/lib/api/schema.gen";
import { suaKhoanMuc, type SuaDongVao } from "@/lib/api/thu-chi"; // vi-name-ok: existing API function and type under test

import { BangThuChi, DongKhoanMuc, LINE_ORDER_TITLE, LineOrderDialog } from "./bang-thu-chi"; // vi-name-ok: existing components under test
import { HopDotThuChi } from "./dot-thu-chi"; // vi-name-ok: existing component under test
import { donViCuaBang, nhanDatDongTong, nhanSuaO, NHAN_SUA_TEN, NO_REPORT_YET } from "./nhan-thu-chi"; // vi-name-ok: existing helpers

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (s: string) => toastSuccess(s),
    error: (s: string) => toastError(s),
  },
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  toastSuccess.mockReset();
  toastError.mockReset();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) await act(async () => {});
}

/** Types into a React-controlled input (the native setter, then the `input` event React listens to). */
function typeInto(input: HTMLInputElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function press(el: Element, key: string): void {
  act(() => {
    el.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

function submit(form: HTMLFormElement): void {
  act(() => {
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
}

function byLabel<T extends HTMLElement>(label: string): T {
  const el = [...host!.querySelectorAll<T>("[aria-label]")].find((e) => e.getAttribute("aria-label") === label);
  expect(el).toBeDefined();
  return el!;
}

const COT: finance_cotRa[] = [
  { id: "C1", name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam", numerator_column_id: null, denominator_column_id: null },
  { id: "C2", name: "Chi ngân sách", order: 2, type: "so", role: "chi-ngan-sach", numerator_column_id: null, denominator_column_id: null },
];

const UNIT = donViCuaBang({
  id: "01JBANG",
  code: "NS-2026-CHI-01",
  year: 2026,
  kind: "chi",
  revision: 1,
  title: "BÁO CÁO CHI",
  unit: "trieu-dong",
  unit_label: "Triệu đồng",
});

const LINE: finance_dongRa = {
  id: "I",
  parent_id: "A",
  no: "I",
  name: "Chi đầu tư phát triển",
  order: 1,
  method: "manual",
  level: 1,
  is_headline: false,
  values: { C1: 1234567890000, C2: null },
};

function mountRow(
  saveLine: (body: SuaDongVao) => Promise<string | null>, // vi-name-ok: existing request type
  openLineOrder: () => void = () => {},
): HTMLDivElement {
  return mount(
    <table>
      <tbody>
        <DongKhoanMuc
          hien={{ dong: LINE, cap: 1, coCon: false, moRong: false }}
          cot={COT}
          donVi={UNIT}
          dongTongId=""
          coGhi
          coXacNhan={false}
          sheetLock={null}
          dangGui={false}
          moRongDoi={() => {}}
          saveLine={saveLine}
          openLineOrder={openLineOrder}
          them={() => {}}
          go={() => {}}
          datTong={() => {}}
          doiCachTinh={() => {}}
          moDot={() => {}}
        />
      </tbody>
    </table>,
  );
}

function nameButton(): HTMLButtonElement {
  const el = host!.querySelector<HTMLButtonElement>(`button[title="${NHAN_SUA_TEN}"]`);
  expect(el).not.toBeNull();
  return el!;
}

function openBox(): HTMLInputElement | null {
  return host!.querySelector<HTMLInputElement>('input[type="text"]');
}

type Body = Parameters<typeof suaKhoanMuc>[1];

describe("tên khoản mục — sửa ngay trong ô (prototype `:528-557`)", () => {
  it("bấm tên → ô nhập điền sẵn tên; Enter gửi ĐÚNG `{ name }`", async () => {
    const save = vi.fn(async (_body: Body) => null);
    mountRow(save);
    act(() => nameButton().click());
    const box = openBox()!;
    expect(box.value).toBe("Chi đầu tư phát triển");
    expect(document.activeElement).toBe(box);

    typeInto(box, "Chi đầu tư công");
    press(box, "Enter");
    await settle();

    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith({ name: "Chi đầu tư công" });
    expect(openBox()).toBeNull();
  });

  it("Esc huỷ: không gửi gì, tên cũ trở lại", async () => {
    const save = vi.fn(async (_body: Body) => null);
    mountRow(save);
    act(() => nameButton().click());
    typeInto(openBox()!, "Gõ nhầm");
    press(openBox()!, "Escape");
    await settle();

    expect(save).not.toHaveBeenCalled();
    expect(openBox()).toBeNull();
    expect(nameButton().textContent).toBe("Chi đầu tư phát triển");
  });

  it("rời ô cũng lưu; Enter rồi rời ô trong lúc chờ KHÔNG gửi lần thứ hai", async () => {
    let release: (v: string | null) => void = () => {};
    const save = vi.fn((_body: Body) => new Promise<string | null>((r) => (release = r)));
    mountRow(save);
    act(() => nameButton().click());
    const box = openBox()!;
    typeInto(box, "Tên mới");
    press(box, "Enter");
    act(() => box.blur());
    await act(async () => release(null));
    await settle();
    expect(save).toHaveBeenCalledTimes(1);

    // Leaving the box alone (no Enter) saves too.
    act(() => nameButton().click());
    typeInto(openBox()!, "Tên khác");
    act(() => openBox()!.blur());
    await act(async () => release(null));
    await settle();
    expect(save).toHaveBeenCalledTimes(2);
    expect(save).toHaveBeenLastCalledWith({ name: "Tên khác" });
  });

  it("máy chủ từ chối: câu của máy chủ hiện NGAY DƯỚI ô, ô vẫn mở", async () => {
    const save = vi.fn(async (_body: Body) => "Kỳ tháng 9/2026 đã chốt (mã CK-2026-09).");
    mountRow(save);
    act(() => nameButton().click());
    typeInto(openBox()!, "Tên mới");
    press(openBox()!, "Enter");
    await settle();

    expect(openBox()).not.toBeNull();
    const alert = host!.querySelector('[role="alert"]');
    expect(alert?.textContent).toBe("Kỳ tháng 9/2026 đã chốt (mã CK-2026-09).");
    expect(openBox()!.getAttribute("aria-describedby")).toBe(alert?.id);
    // Leaving the box while the refusal shows does not resend it.
    act(() => openBox()!.blur());
    await settle();
    expect(save).toHaveBeenCalledTimes(1);
  });
});

describe("ô số — sửa ngay trong ô (prototype `:572-600`)", () => {
  const LABEL = nhanSuaO("Dự toán năm", "Chi đầu tư phát triển", "1.234.567,9");

  it("bấm số (hiện làm tròn) → ô mở với con số CHÍNH XÁC; Enter gửi ĐÚNG một cột", async () => {
    const save = vi.fn(async (_body: Body) => null);
    mountRow(save);
    act(() => byLabel<HTMLButtonElement>(LABEL).click());
    const box = openBox()!;
    expect(box.value).toBe("1.234.567,89");
    expect(box.getAttribute("inputmode")).toBe("decimal");

    typeInto(box, "2,5");
    press(box, "Enter");
    await settle();

    expect(save).toHaveBeenCalledWith({ values: { C1: 2500000 } });
  });

  it("Esc huỷ; Enter không sửa gì cũng không gửi", async () => {
    const save = vi.fn(async (_body: Body) => null);
    mountRow(save);
    act(() => byLabel<HTMLButtonElement>(LABEL).click());
    typeInto(openBox()!, "9");
    press(openBox()!, "Escape");
    act(() => byLabel<HTMLButtonElement>(LABEL).click());
    press(openBox()!, "Enter");
    await settle();

    expect(save).not.toHaveBeenCalled();
    expect(openBox()).toBeNull();
  });

  it("gõ sai khuôn: câu lý do dưới ô, không gửi", async () => {
    const save = vi.fn(async (_body: Body) => null);
    mountRow(save);
    act(() => byLabel<HTMLButtonElement>(LABEL).click());
    typeInto(openBox()!, "1.5");
    press(openBox()!, "Enter");
    await settle();

    expect(save).not.toHaveBeenCalled();
    expect(host!.querySelector('[role="alert"]')?.textContent).toContain("dấu chấm ngăn hàng nghìn");
  });
});

describe("PATCH trên dây — chỉ trường vừa sửa", () => {
  it("`{ name }` đi lên đúng một trường, phương thức PATCH, đúng dòng", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init: RequestInit) => {
        calls.push({ url, init });
        return new Response(JSON.stringify({ id: "I" }), { status: 200, headers: { "Content-Type": "application/json" } });
      }),
    );
    await suaKhoanMuc("I", { name: "Chi đầu tư công" });
    expect(calls[0]!.url).toBe("/api/v1/budget-lines/I");
    expect(calls[0]!.init.method).toBe("PATCH");
    expect(calls[0]!.init.body).toBe('{"name":"Chi đầu tư công"}');

    await suaKhoanMuc("I", { values: { C1: 2500000 } });
    expect(calls[1]!.init.body).toBe('{"values":{"C1":2500000}}');
  });
});

describe("TT và thứ tự hiển thị", () => {
  it("nút bút chì cuối cột Cách tính mở hộp riêng", () => {
    const openLineOrder = vi.fn();
    mountRow(async () => null, openLineOrder);
    const pencil = byLabel<HTMLButtonElement>(`${LINE_ORDER_TITLE} của Chi đầu tư phát triển`);
    expect(pencil.title).toBe(LINE_ORDER_TITLE);
    // Last control of the cell.
    expect(pencil.parentElement?.lastElementChild).toBe(pencil);
    act(() => pencil.click());
    expect(openLineOrder).toHaveBeenCalledTimes(1);
  });

  it("hộp gửi CHỈ trường đã đổi", () => {
    const luu = vi.fn();
    mount(<LineOrderDialog dong={LINE} dangGui={false} huy={() => {}} luu={luu} />);
    const order = host!.querySelector<HTMLInputElement>('input[name="order"]')!;
    typeInto(order, "5");
    submit(host!.querySelector("form")!);
    expect(luu).toHaveBeenCalledWith({ order: 5 });
  });
});

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

describe("hộp các đợt — thông báo thành công là toast (prototype `FiscalEntriesDialog.tsx:88,251`)", () => {
  const LIST: finance_danhSachDotRa = {
    line_id: "I",
    method: "entries",
    entries: [{ id: "D1", line_id: "I", date: "2026-09-20", content: "Đợt 1", values: { C1: null, C2: 1000000 } }],
  };

  function stubEntries(): { url: string; init: RequestInit }[] {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init: RequestInit) => {
        calls.push({ url, init });
        if (init.method === "POST") return json({ id: "D2", line_id: "I", date: "2026-10-09", content: "x", values: {} }, 201);
        if (init.method === "DELETE") return new Response(null, { status: 204 });
        return json(LIST);
      }),
    );
    return calls;
  }

  function mountDialog(onChanged: () => void = () => {}): void {
    mount(
      <HopDotThuChi
        khoanMucId="I"
        tenKhoanMuc="Chi đầu tư phát triển"
        cot={COT}
        donVi={UNIT}
        coGhi
        coXacNhan
        closes={[]}
        sheetYear={2026}
        dong={() => {}}
        daDoiSoLieu={onChanged}
      />,
    );
  }

  function entryForm(): HTMLFormElement {
    return host!.querySelector<HTMLFormElement>('form[aria-label="Ghi một đợt"]')!;
  }

  it("thiếu nội dung, thiếu số: câu của prototype, không gửi", async () => {
    const calls = stubEntries();
    mountDialog();
    await settle();
    submit(entryForm());
    expect(host!.textContent).toContain("Nhập nội dung đợt thu, chi.");
    typeInto(host!.querySelector<HTMLInputElement>("#dot-content")!, "Thu đợt 2");
    submit(entryForm());
    expect(host!.textContent).toContain("Nhập ít nhất một con số.");
    expect(calls.filter((c) => c.init.method === "POST")).toHaveLength(0);
  });

  it("ghi một đợt: toast 'Đã ghi một đợt.', bảng đọc lại; không còn dòng chữ xanh trong hộp", async () => {
    stubEntries();
    const onChanged = vi.fn();
    mountDialog(onChanged);
    await settle();
    typeInto(host!.querySelector<HTMLInputElement>("#dot-content")!, "Thu đợt 2");
    typeInto(host!.querySelector<HTMLInputElement>("#dot-gia-C2")!, "1,5");
    submit(entryForm());
    await settle();

    expect(toastSuccess).toHaveBeenCalledWith("Đã ghi một đợt.");
    expect(onChanged).toHaveBeenCalled();
    expect(host!.textContent).not.toContain("Đã ghi đợt.");
  });

  it("gỡ một đợt: thùng rác mở hộp lý do; gỡ xong là toast 'Đã gỡ đợt.'", async () => {
    const calls = stubEntries();
    mountDialog();
    await settle();
    act(() => byLabel<HTMLButtonElement>("Gỡ đợt ngày 20/9/2026").click());
    const reason = host!.querySelector<HTMLInputElement>("#go-dot-reason")!;
    typeInto(reason, "Ghi nhầm khoản mục");
    submit(reason.form!);
    await settle();

    expect(calls.find((c) => c.init.method === "DELETE")?.url).toBe("/api/v1/budget-entries/D1");
    expect(toastSuccess).toHaveBeenCalledWith("Đã gỡ đợt.");
  });
});

describe("trạng thái rỗng của bảng", () => {
  it("bảng chưa có: câu của prototype, nguyên văn", async () => {
    const session: identity_phienHienTaiRa = {
      sid: "s",
      expires_at: "2099-01-01T00:00:00Z",
      staff: { code: "CB-00001", full_name: "Cán bộ A", position: "Kế toán" },
      role: null,
      permissions: [],
      must_change_password: false,
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (url.startsWith("/api/v1/budget-sheets")) {
          return json({ code: "not_found", message: "ngan_sach: không có bảng ngân sách này trong xã", trace_id: "" }, 404);
        }
        if (url.startsWith("/api/v1/budget-indicators")) {
          return json({
            year: 2026,
            revenue_achievement: { name: "Thu đạt dự toán", basis_points: null },
            expenditure_achievement: { name: "Chi đạt dự toán", basis_points: null },
            balance: { amount: null },
            revenue_totals: [],
          });
        }
        if (url.startsWith("/api/v1/budget-period-closes")) return json({ year: 2026, closes: [] });
        return json(session);
      }),
    );
    mount(
      <PhienProvider>
        <BangThuChi />
      </PhienProvider>,
    );
    await settle();
    expect(host!.textContent).toContain(NO_REPORT_YET);
  });
});

describe("ngôi sao dòng tổng — đổi TẠI CHỖ, không nháy màn (bảng lỗi khách hàng dòng 51)", () => {
  const SESSION: identity_phienHienTaiRa = {
    sid: "s",
    expires_at: "2099-01-01T00:00:00Z",
    staff: { code: "CB-00001", full_name: "Cán bộ A", position: "Kế toán" },
    role: null,
    permissions: ["budget.read", "budget.confirm"],
    must_change_password: false,
  };
  const top = (id: string, name: string, order: number): finance_dongRa => ({
    id,
    no: id,
    name,
    order,
    method: "manual",
    level: 0,
    is_headline: false,
    values: { C1: 1_000_000, C2: 500_000 },
  });

  function stubSheet(headline: () => Promise<Response>) {
    let serverHeadline = "A";
    const calls = { sheet: 0, indicators: 0, headline: 0 };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (url.startsWith("/api/v1/budget-sheets")) {
          calls.sheet += 1;
          return json({
            sheet: { id: "01JBANG", code: "NS-2026-CHI-01", year: 2026, kind: "chi", revision: 1, title: "BÁO CÁO CHI", unit: "trieu-dong", unit_label: "Triệu đồng" },
            columns: COT,
            lines: [top("A", "Tổng chi", 1), top("B", "Chi thường xuyên", 2)],
            summary: { headline_line_id: serverHeadline, cells: [], indicator: { name: "Chi đạt dự toán", basis_points: null } },
          });
        }
        if (url.startsWith("/api/v1/budget-indicators")) {
          calls.indicators += 1;
          return json({
            year: 2026,
            revenue_achievement: { name: "Thu đạt dự toán", basis_points: null },
            expenditure_achievement: { name: "Chi đạt dự toán", basis_points: null },
            balance: { amount: null },
            revenue_totals: [],
          });
        }
        if (url.startsWith("/api/v1/budget-period-closes")) return json({ year: 2026, closes: [] });
        if (url.includes("/headline")) {
          calls.headline += 1;
          const r = await headline();
          if (r.ok) serverHeadline = "B";
          return r;
        }
        return json(SESSION);
      }),
    );
    return calls;
  }

  const star = (name: string) => byLabel<HTMLButtonElement>(nhanDatDongTong(name));

  it("bấm sao: sao chuyển NGAY, không hiện trạng thái tải; máy chủ trả lời xong thì đọc lại LẶNG LẼ", async () => {
    let answer: (r: Response) => void = () => {};
    const calls = stubSheet(() => new Promise<Response>((ok) => (answer = ok)));
    mount(
      <PhienProvider>
        <BangThuChi />
      </PhienProvider>,
    );
    await settle();
    expect(star("Tổng chi").getAttribute("aria-pressed")).toBe("true");
    expect(calls.sheet).toBe(1);

    act(() => star("Chi thường xuyên").click());
    // Optimistic: moved before the server answers; the table never fell back to its skeleton.
    expect(star("Chi thường xuyên").getAttribute("aria-pressed")).toBe("true");
    expect(star("Tổng chi").getAttribute("aria-pressed")).toBe("false");
    expect(host!.textContent).not.toContain("Đang tải bảng ngân sách");

    await act(async () => answer(json({ ...top("B", "Chi thường xuyên", 2), is_headline: true })));
    await settle();
    expect(calls.headline).toBe(1);
    expect(calls.sheet).toBe(2);
    expect(host!.textContent).not.toContain("Đang tải bảng ngân sách");
    expect(star("Chi thường xuyên").getAttribute("aria-pressed")).toBe("true");
  });

  it("máy chủ từ chối: câu của máy chủ nguyên văn, sao về đúng chỗ máy chủ giữ", async () => {
    const cau = "Kỳ ngân sách đã khoá, không đổi được dòng tổng.";
    stubSheet(async () => json({ code: "period_closed", message: cau, trace_id: "" }, 409));
    mount(
      <PhienProvider>
        <BangThuChi />
      </PhienProvider>,
    );
    await settle();
    act(() => star("Chi thường xuyên").click());
    await settle();
    expect(host!.textContent).toContain(cau);
    expect(star("Tổng chi").getAttribute("aria-pressed")).toBe("true");
    expect(star("Chi thường xuyên").getAttribute("aria-pressed")).toBe("false");
  });
});
