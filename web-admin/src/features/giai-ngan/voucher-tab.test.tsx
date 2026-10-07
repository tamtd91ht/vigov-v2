// @vitest-environment jsdom
//
// jsdom for this file: the voucher tab reads its list from the server, writes, and reads again; the
// request bodies that leave the browser and the reads that follow are the things under test — events
// and captured requests, not a markup string.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type {
  finance_chungTuRa,
  finance_duAnRa,
  finance_projectAllocationOut,
} from "@/lib/api/schema.gen";

import { ChiTietDuAn } from "./chi-tiet-du-an";
import { KhoiChungTu, type ProjectVoucherList } from "./chung-tu-du-an";
import { CHUNG_TU_DA_XAC_NHAN, CHUNG_TU_KE_TOAN_NHAP, MISSING_FUNDING_SOURCE } from "./nhan-ghi-giai-ngan";

/**
 * §8.2 voucher tab against `GET /api/v1/investment-projects/{id}/disbursements` (db94b35c) and the
 * voucher's funding source (decision 06/10/2026: project with allocation lines → source required and
 * one of them; project without → no source at all).
 */

// Outcomes are toasts (ADR 0068 lần 6 #4).
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const fakeSession = {
  ok: true as const,
  duLieu: { permissions: ["budget.read", "budget.update", "budget.confirm"] },
};
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
  PhienProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
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
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

/** Lets every pending fetch promise and the renders it causes settle. */
async function settle(): Promise<void> {
  for (let i = 0; i < 4; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function enter(el: HTMLInputElement | HTMLSelectElement, value: string): void {
  const proto = el instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  act(() => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

function buttonByText(el: HTMLElement, text: string): HTMLButtonElement {
  const b = [...el.querySelectorAll("button")].find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function voucher(over: Partial<finance_chungTuRa> = {}): finance_chungTuRa {
  return {
    id: "01JCT1",
    project_id: "01JDA1",
    payment_date: "2026-09-07",
    amount: 30_000_000,
    description: "Thanh toán đợt 3",
    status: CHUNG_TU_KE_TOAN_NHAP,
    entered_by: "CB-00123",
    unlock_count: 0,
    ...over,
  };
}

function line(id: string, name: string, amount: number, drawn: number): finance_projectAllocationOut {
  return { funding_source_id: id, name, amount, disbursed_amount: drawn, disbursed_ratio: null };
}

const TWO_LINES = [line("S1", "Ngân sách tỉnh", 1_500_000_000, 300_000_000), line("S2", "Ngân sách xã", 500_000_000, 0)];

const PROJECT: finance_duAnRa = {
  id: "01JDA1",
  code: "DA01",
  year: 2026,
  category_id: "01JHM1",
  name: "Bê tông hoá đường trục chính",
  planned_amount: 2_000_000_000,
  approved_amount: 2_000_000_000,
  disbursed_amount: 30_000_000,
  remaining_amount: 1_970_000_000,
  disbursed_ratio: 150,
  delay_score: null,
  is_delayed: false,
  disbursement_deadline: "2026-12-31",
  delay_threshold: 1000,
  delay_threshold_source: "mac_dinh",
  funding_allocations: TWO_LINES,
};

type Seen = { method: string; url: string; body: Record<string, unknown> | null };

/**
 * Fake finance server. `vouchers` is what the list route returns NOW — a test swaps it to show that
 * the screen shows the server's list after a write, not a row it patched in itself.
 */
function stubServer(state: { vouchers: finance_chungTuRa[]; writeReply?: () => Response }): Seen[] {
  const seen: Seen[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      seen.push({ method, url, body: init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : null });
      if (method === "GET" && url === "/api/v1/investment-projects/01JDA1") return json(200, PROJECT);
      if (method === "GET" && url === "/api/v1/investment-projects/01JDA1/disbursements") {
        return json(200, { project_id: "01JDA1", items: state.vouchers, count: state.vouchers.length });
      }
      if (method !== "GET" && url.startsWith("/api/v1/disbursements")) {
        return state.writeReply?.() ?? json(method === "POST" && url === "/api/v1/disbursements" ? 201 : 200, voucher());
      }
      return new Response("unexpected", { status: 500 });
    }),
  );
  return seen;
}

const listReads = (seen: Seen[]) =>
  seen.filter((s) => s.method === "GET" && s.url === "/api/v1/investment-projects/01JDA1/disbursements").length;

describe("§8.2 list — read from the server", () => {
  it("renders the server's rows, the source name or '—', and the count on the tab", async () => {
    stubServer({
      vouchers: [
        voucher({ id: "A", funding_source_id: "S1", funding_source_name: "Ngân sách tỉnh" }),
        voucher({ id: "B", payment_date: "2026-08-01" }),
      ],
    });
    const el = mount(<ChiTietDuAn id="01JDA1" />);
    await settle();

    expect(el.querySelector("#tab-chung-tu-du-an")?.textContent).toBe("Chứng từ (2)");
    const rows = [...el.querySelectorAll('[aria-label="Chứng từ giải ngân của dự án"] table tbody tr')];
    expect(rows).toHaveLength(2);
    // Column order: Ngày chi · Số tiền · Nguồn vốn · ...
    expect(rows.map((r) => r.querySelectorAll("td")[2]?.textContent)).toEqual(["Ngân sách tỉnh", "—"]);
  });

  it("a failed read is an error with retry, never an empty table and never a '(0)'", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) =>
        url.endsWith("/disbursements")
          ? json(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." })
          : url === "/api/v1/investment-projects/01JDA1"
            ? json(200, PROJECT)
            : new Response("unexpected", { status: 500 }),
      ),
    );
    const el = mount(<ChiTietDuAn id="01JDA1" />);
    await settle();

    expect(el.textContent).toContain("Chưa tải được danh sách chứng từ");
    expect(el.textContent).toContain("Bạn không có quyền thực hiện thao tác này.");
    expect(el.textContent).not.toContain("chưa có chứng từ giải ngân nào");
    expect(el.querySelector("#tab-chung-tu-du-an")?.textContent).toBe("Chứng từ");
  });

  it("RE-READS after a write: the list shown is the server's new list, not a patched row", async () => {
    const state = { vouchers: [voucher()] };
    const seen = stubServer(state);
    const el = mount(<ChiTietDuAn id="01JDA1" />);
    await settle();
    expect(listReads(seen)).toBe(1);

    // The server now holds the confirmed voucher AND another one this screen never wrote.
    state.vouchers = [
      voucher({ status: CHUNG_TU_DA_XAC_NHAN }),
      voucher({ id: "OTHER", description: "Khoản chi người khác vừa ghi" }),
    ];
    await act(async () => {
      el.querySelector<HTMLButtonElement>('button[aria-label="Xác nhận chứng từ ngày 7/9/2026"]')!.click();
    });
    await settle();

    expect(seen.some((s) => s.method === "POST" && s.url === "/api/v1/disbursements/01JCT1/confirmation")).toBe(true);
    expect(listReads(seen)).toBe(2);
    expect(toast.success).toHaveBeenCalledWith("Đã xác nhận.");
    expect(el.querySelector("#tab-chung-tu-du-an")?.textContent).toBe("Chứng từ (2)");
    expect(el.textContent).toContain("Khoản chi người khác vừa ghi");
  });
});

/** The voucher block on its own, with a list already read. */
function block(allocations: readonly finance_projectAllocationOut[], items: finance_chungTuRa[] = []) {
  const vouchers: ProjectVoucherList = { phase: "ready", items, count: items.length };
  const written = vi.fn();
  const el = mount(
    <KhoiChungTu
      duAnID="01JDA1"
      allocations={allocations}
      vouchers={vouchers}
      coGhi
      coXacNhan
      daGhiXong={written}
    />,
  );
  return { el, written };
}

function sourceSelect(el: HTMLElement): HTMLSelectElement | null {
  return el.querySelector<HTMLSelectElement>("#nguon-von-chung-tu");
}

function fillVoucher(el: HTMLElement): void {
  enter(el.querySelector<HTMLInputElement>("#ngay-chi-chung-tu")!, "2026-09-07");
  enter(el.querySelector<HTMLInputElement>("#so-tien-chung-tu")!, "30000000");
  enter(el.querySelector<HTMLInputElement>("#noi-dung-chung-tu")!, "Thanh toán đợt 3");
}

async function save(el: HTMLElement, label: string): Promise<void> {
  await act(async () => {
    buttonByText(el, label).click();
  });
  await settle();
}

describe("§8.2 form — `Rút từ nguồn vốn`", () => {
  it("project with NO allocation line: no select, and the POST carries no source", async () => {
    const seen = stubServer({ vouchers: [] });
    const { el, written } = block([]);
    act(() => buttonByText(el, "Ghi nhận khoản chi").click());

    expect(sourceSelect(el)).toBeNull();
    fillVoucher(el);
    await save(el, "Lưu khoản chi");

    const post = seen.find((s) => s.method === "POST");
    expect(post?.body).not.toHaveProperty("funding_source_id");
    expect(written).toHaveBeenCalledTimes(1);
  });

  it("options are the project's lines, 'Tên — còn X'; nothing preselected with two; required", async () => {
    const seen = stubServer({ vouchers: [] });
    const { el, written } = block(TWO_LINES);
    act(() => buttonByText(el, "Ghi nhận khoản chi").click());

    const select = sourceSelect(el)!;
    expect([...select.options].map((o) => o.value)).toEqual(["", "S1", "S2"]);
    expect([...select.options].map((o) => o.textContent)).toEqual([
      "— Chọn nguồn vốn —",
      "Ngân sách tỉnh — còn 1,2 tỷ",
      "Ngân sách xã — còn 500 triệu",
    ]);
    expect(select.value).toBe("");

    fillVoucher(el);
    await save(el, "Lưu khoản chi");
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(MISSING_FUNDING_SOURCE);
    expect(seen.some((s) => s.method === "POST")).toBe(false);
    expect(written).not.toHaveBeenCalled();

    enter(sourceSelect(el)!, "S2");
    await save(el, "Lưu khoản chi");
    expect(seen.find((s) => s.method === "POST")?.body).toMatchObject({ funding_source_id: "S2" });
    expect(written).toHaveBeenCalledTimes(1);
  });

  it("exactly one line: preselected, and sent", async () => {
    const seen = stubServer({ vouchers: [] });
    const { el } = block([TWO_LINES[0]!]);
    act(() => buttonByText(el, "Ghi nhận khoản chi").click());

    expect(sourceSelect(el)!.value).toBe("S1");
    fillVoucher(el);
    await save(el, "Lưu khoản chi");
    expect(seen.find((s) => s.method === "POST")?.body).toMatchObject({ funding_source_id: "S1" });
  });

  it("409 from the server: its sentence shows verbatim, without the field tag, and nothing re-reads", async () => {
    stubServer({
      vouchers: [],
      writeReply: () =>
        json(409, {
          code: "source_not_allocated",
          message:
            "`funding_source_id`: nguồn vốn này không được phân bổ cho dự án. Hãy chọn một nguồn đã phân bổ cho dự án, hoặc bỏ trống nếu dự án chưa khai nguồn vốn nào.",
        }),
    });
    const { el, written } = block([TWO_LINES[0]!]);
    act(() => buttonByText(el, "Ghi nhận khoản chi").click());
    fillVoucher(el);
    await save(el, "Lưu khoản chi");

    expect(toast.error).toHaveBeenCalledWith(
      "Nguồn vốn này không được phân bổ cho dự án. Hãy chọn một nguồn đã phân bổ cho dự án, hoặc bỏ trống nếu dự án chưa khai nguồn vốn nào.",
    );
    expect(written).not.toHaveBeenCalled();
  });

  it("EDIT: pre-filled with the voucher's source; PATCH omits it when unchanged, sends it when changed", async () => {
    const seen = stubServer({ vouchers: [] });
    const filed = voucher({ funding_source_id: "S1", funding_source_name: "Ngân sách tỉnh" });
    const { el } = block(TWO_LINES, [filed]);

    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Sửa chứng từ ngày 7/9/2026"]')!.click());
    expect(sourceSelect(el)!.value).toBe("S1");
    enter(el.querySelector<HTMLInputElement>("#noi-dung-chung-tu")!, "Thanh toán đợt 4");
    await save(el, "Lưu thay đổi");
    expect(seen.find((s) => s.method === "PATCH")?.body).toEqual({ description: "Thanh toán đợt 4" });

    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Sửa chứng từ ngày 7/9/2026"]')!.click());
    enter(sourceSelect(el)!, "S2");
    await save(el, "Lưu thay đổi");
    expect(seen.filter((s) => s.method === "PATCH")[1]?.body).toEqual({ funding_source_id: "S2" });
  });

  it("EDIT of a voucher whose source the project no longer allocates: that source stays shown, not swapped", () => {
    stubServer({ vouchers: [] });
    const filed = voucher({ funding_source_id: "OLD", funding_source_name: "Vốn chương trình mục tiêu" });
    const { el } = block(TWO_LINES, [filed]);

    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Sửa chứng từ ngày 7/9/2026"]')!.click());
    const select = sourceSelect(el)!;
    expect(select.value).toBe("OLD");
    expect(select.selectedOptions[0]?.textContent).toBe("Vốn chương trình mục tiêu — không còn phân bổ cho dự án");
  });
});

describe("§8.2 row — spec 07 presentation, status codes unchanged", () => {
  it("labels per spec 00 §6 over the UNCHANGED codes (ADR 0011)", () => {
    stubServer({ vouchers: [] });
    const { el } = block(TWO_LINES, [
      voucher({ id: "A", status: CHUNG_TU_KE_TOAN_NHAP }),
      voucher({ id: "B", status: CHUNG_TU_DA_XAC_NHAN }),
      voucher({ id: "C", status: "da-khoa" }),
    ]);
    const badges = [...el.querySelectorAll("[data-voucher-status]")].map((b) => [
      b.getAttribute("data-voucher-status"),
      b.textContent,
    ]);
    expect(badges).toEqual([
      ["ke-toan-nhap", "Kế toán nhập"],
      ["da-xac-nhan", "Lãnh đạo đã xác nhận"],
      ["da-khoa", "Đã khoá"],
    ]);
    // No lifecycle sentence, no "Khoá lúc" line (spec 07 rows D14, D19).
    expect(el.textContent).not.toContain("Vòng đời");
    expect(el.textContent).not.toContain("Khoá lúc");
  });

  it("a DRAFT row draws `Khoá` disabled with its '?' — the one-call confirm+lock is a backend dependency", () => {
    stubServer({ vouchers: [] });
    const { el } = block(TWO_LINES, [voucher({ status: CHUNG_TU_KE_TOAN_NHAP })]);
    const spot = el.querySelector<HTMLElement>("tbody [data-pending]")!;
    expect(spot).not.toBeNull();
    const button = [...spot.querySelectorAll("button")].find((b) => b.textContent === "Khoá")!;
    expect(button.disabled).toBe(true);
    expect(spot.querySelector("button[data-pending-marker]")?.getAttribute("aria-label")).toContain(
      "Khoá khoản chi chưa xác nhận",
    );
    // After `Xác nhận`, as the prototype orders the buttons.
    const words = [...el.querySelectorAll("tbody button")].map((b) => b.textContent?.trim()).filter(Boolean);
    expect(words.indexOf("Khoá")).toBe(words.indexOf("Xác nhận") + 1);
  });

  it("a CONFIRMED row has the live `Khoá` (no '?'), and the lock outcome is the spec's toast", async () => {
    stubServer({ vouchers: [] });
    const { el } = block(TWO_LINES, [voucher({ status: CHUNG_TU_DA_XAC_NHAN })]);
    expect(el.querySelector("tbody [data-pending]")).toBeNull();
    await act(async () => {
      el.querySelector<HTMLButtonElement>('button[aria-label="Khoá chứng từ ngày 7/9/2026"]')!.click();
    });
    await settle();
    expect(toast.success).toHaveBeenCalledWith("Đã xác nhận và khoá.");
  });

  it("without the keys the buttons are simply absent — no denial notes (spec 07)", () => {
    stubServer({ vouchers: [] });
    const vouchers: ProjectVoucherList = { phase: "ready", items: [voucher()], count: 1 };
    const el = mount(
      <KhoiChungTu duAnID="01JDA1" allocations={[]} vouchers={vouchers} coGhi={false} coXacNhan={false} daGhiXong={() => {}} />,
    );
    expect(el.querySelectorAll("tbody button")).toHaveLength(0);
    expect(el.textContent).not.toContain("chưa được cấp quyền");
  });
});
