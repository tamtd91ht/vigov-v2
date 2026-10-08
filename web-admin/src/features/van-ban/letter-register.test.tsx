// @vitest-environment jsdom
//
// jsdom: the REAL register behind the REAL session provider, on a fake server — so the permission
// gate is tested from the session the server sends, both the allowed and the denied answer.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import type { documents_citizenLetterItemOut } from "@/lib/api/schema.gen";
import { PETITION_CREATE_PERMISSION, PETITION_READ_PERMISSION } from "@/lib/quyen";

import { LETTER_REGISTER_EMPTY, LetterRegister, LetterRegisterView, LetterTable, MERGED_DUPLICATE, NO_FILTERS } from "./letter-register";
import { NO_DEADLINE, WITHHELD_SENDER, WITHHELD_SUMMARY } from "./letter-display";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

function row(over: Partial<documents_citizenLetterItemOut> = {}): documents_citizenLetterItemOut {
  return {
    id: "L1",
    number: 5,
    year: 2026,
    received_date: "2026-10-01",
    letter_type: "khieu-nai",
    source: "nhap-tay",
    sender_name: "Nguyễn Văn A",
    sender_phone: "09****0000",
    identity_withheld: false,
    summary: "Khiếu nại việc cấp giấy phép xây dựng",
    summary_withheld: false,
    holding_unit_id: "U1",
    assignee_code: "CB-00002",
    status: "thu-ly",
    status_group: "dang-xu-ly",
    processing_due_at: null,
    resolution_due_at: null,
    days_open: 8,
    is_resolved: false,
    is_closed: false,
    ...over,
  };
}

const ROWS = [
  row(),
  row({
    id: "L2",
    number: 4,
    letter_type: "to-cao",
    identity_withheld: true,
    sender_name: null,
    sender_phone: null,
    summary: null,
    summary_withheld: true,
  }),
  row({ id: "L3", number: 3, related_letter_id: "L1", status: "da-giai-quyet", is_resolved: true, is_closed: true, days_open: 12 }),
];

function mountNode(node: React.ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

describe("letter table — masking and rows", () => {
  const el = () =>
    mountNode(
      <LetterTable
        answer={{ ok: true, duLieu: { items: ROWS, next_cursor: "", has_more: false } }}
        now={new Date("2026-10-08T03:00:00Z")}
        units={{ pha: "xong", ten: new Map([["U1", "Văn phòng"]]) }}
        onOpen={() => {}}
      />,
    );

  it("nine prototype columns, in order", () => {
    const heads = [...el().querySelectorAll("th")].map((th) => th.textContent);
    expect(heads).toEqual(["Số", "Ngày nhận", "Người gửi", "Loại đơn", "Nội dung", "Đang giữ", "Số ngày xử lý", "Hạn giải quyết", "Trạng thái"]);
  });

  it("the status badge shows the display GROUP (ADR 0084 #2), icon + word, in the prototype's colour", () => {
    const rows = el().querySelectorAll("tbody tr");
    const badge = rows[0]!.querySelectorAll("td")[8]!;
    expect(badge.textContent).toBe("Đang xử lý");
    expect(badge.querySelector("svg")).not.toBeNull();
    expect(badge.innerHTML).toContain("bg-teal/12");
  });

  it("received date reads d/m/yyyy without zero padding (prototype `formatDay`)", () => {
    const cells = [...el().querySelectorAll("tbody tr")][0]!.querySelectorAll("td");
    expect(cells[1]!.textContent).toBe("1/10/2026");
  });

  it("phone as masked text, never a link; a denunciation's sender and summary are fixed sentences", () => {
    const t = el();
    const rows = t.querySelectorAll("tbody tr");
    expect(rows[0]!.textContent).toContain("09****0000");
    expect(t.querySelector("a")).toBeNull();
    expect(t.innerHTML).not.toContain("tel:");
    expect(rows[1]!.textContent).toContain(WITHHELD_SENDER);
    expect(rows[1]!.textContent).toContain(WITHHELD_SUMMARY);
    expect(rows[1]!.textContent).toContain("Tố cáo");
  });

  it("a merged duplicate is faded and says so; a resolved one stops its count", () => {
    const rows = el().querySelectorAll("tbody tr");
    expect(rows[2]!.className).toContain("opacity-60");
    expect(rows[2]!.textContent).toContain(MERGED_DUPLICATE);
    expect(rows[2]!.textContent).toContain("12 ngày");
    expect(rows[2]!.textContent).toContain("đã giải quyết");
    // Never a computed deadline: none stored → "Không đặt hạn", and no row is tinted late.
    expect(rows[0]!.textContent).toContain(NO_DEADLINE);
    expect(el().querySelectorAll("tr.bg-danger\\/4")).toHaveLength(0);
  });

  it("“Hạn giải quyết” uses the drawer figure's rule: a finished letter keeps the date of its stage (TASK-09b)", () => {
    act(() => root?.unmount());
    host?.remove();
    const t = mountNode(
      <LetterTable
        answer={{
          ok: true,
          duLieu: {
            items: [
              // Lưu đơn — never admitted: the processing date, not "Không đặt hạn".
              row({ status: "luu-don", status_group: "luu-khong-thu-ly", is_closed: true, processing_due_at: "2026-10-03T10:00:00Z" }),
              // Đã giải quyết — admitted: the resolution date.
              row({ id: "L9", status: "da-giai-quyet", status_group: "da-giai-quyet", is_closed: true, is_resolved: true, processing_due_at: "2026-10-03T10:00:00Z", resolution_due_at: "2026-10-20T10:00:00Z" }),
            ],
            next_cursor: "",
            has_more: false,
          },
        }}
        now={new Date("2026-10-08T03:00:00Z")}
        units={{ pha: "dangDoc" }}
        onOpen={() => {}}
      />,
    );
    const rows = t.querySelectorAll("tbody tr");
    expect(rows[0]!.querySelectorAll("td")[7]!.textContent).toBe("Hạn 3/10/2026");
    expect(rows[1]!.querySelectorAll("td")[7]!.textContent).toBe("Hạn 20/10/2026");
  });

  it("empty — under a filter too — says exactly the prototype's one sentence; an error has Tải lại", () => {
    act(() => root?.unmount());
    host?.remove();
    const empty = mountNode(
      <LetterTable answer={{ ok: true, duLieu: { items: [], next_cursor: "", has_more: false } }} now={new Date()} units={{ pha: "dangDoc" }} onOpen={() => {}} />,
    );
    expect(empty.textContent).toContain("Sổ đơn thư chưa có bản ghi nào.");
    act(() => root?.unmount());
    host?.remove();
    // The whole tab under a status filter and a scope: still the one sentence, no second one.
    const byFilter = mountNode(
      <LetterRegisterView
        answer={{ ok: true, duLieu: { items: [], next_cursor: "", has_more: false } }}
        now={new Date()}
        scope="mine"
        onScope={() => {}}
        filters={{ ...NO_FILTERS, statusGroup: "dang-xu-ly" }}
        onFilters={() => {}}
        units={{ pha: "dangDoc" }}
        directory={null}
        stack={TRANG_DAU}
        goToPage={() => {}}
        onOpen={() => {}}
      />,
    );
    const card = byFilter.querySelector("section p.p-6")!;
    expect(card.textContent).toBe(LETTER_REGISTER_EMPTY);
    expect(byFilter.textContent).not.toContain("khớp bộ lọc");
    expect(byFilter.textContent).not.toContain("bỏ bớt bộ lọc");
    expect(LETTER_REGISTER_EMPTY).toBe("Sổ đơn thư chưa có bản ghi nào.");
    act(() => root?.unmount());
    host?.remove();
    const error = mountNode(
      <LetterTable answer={{ ok: false, thongBao: "Bạn không có quyền xem đơn thư." }} now={new Date()} units={{ pha: "dangDoc" }} onOpen={() => {}} onReload={() => {}} />,
    );
    expect(error.textContent).toContain("Bạn không có quyền xem đơn thư.");
    expect([...error.querySelectorAll("button")].some((b) => b.textContent?.trim() === "Tải lại")).toBe(true);
  });
});

/** A fake server answering the session with `permissions`, and the register's reads. */
function stubServer(permissions: string[]): string[] {
  const urls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      urls.push(url);
      const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200 });
      if (url === "/api/v1/sessions/current") {
        return json({
          sid: "s",
          expires_at: "2099-01-01T00:00:00Z",
          staff: { code: "CB-00009", full_name: "Cán bộ X", position: "" },
          role: { code: "r", name: "R", is_leader: false },
          permissions,
          must_change_password: false,
        });
      }
      if (url.startsWith("/api/v1/citizen-letters")) return json({ items: ROWS, next_cursor: "", has_more: false });
      if (url.startsWith("/api/v1/org-units")) return json({ items: [{ id: "U1", name: "Văn phòng" }] });
      if (url.startsWith("/api/v1/staff-directory")) return json({ items: [] });
      return new Response("{}", { status: 404 });
    }),
  );
  return urls;
}

async function mountRegister(): Promise<HTMLDivElement> {
  const el = mountNode(
    <PhienProvider>
      <LetterRegister />
    </PhienProvider>,
  );
  for (let i = 0; i < 5; i++) await act(async () => {});
  return el;
}

const bookButton = (el: HTMLElement) => [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Vào sổ đơn thư");
const importButton = (el: HTMLElement) => [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Nhập từ Excel");

describe("letter register — “Nhập từ Excel” (ADR 0084 #5, prototype ExcelImportDialog)", () => {
  type Call = { url: string; method: string; key: string | undefined };

  /** Session with petition.create; the import routes answer `preview` and `imported`. */
  function importServer(preview: unknown, imported: { status: number; body: unknown }): Call[] {
    const calls: Call[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        const method = (init?.method ?? "GET").toUpperCase();
        calls.push({ url, method, key: (init?.headers as Record<string, string> | undefined)?.["Idempotency-Key"] });
        const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
        if (url === "/api/v1/sessions/current") {
          return json({
            sid: "s",
            expires_at: "2099-01-01T00:00:00Z",
            staff: { code: "CB-00009", full_name: "Cán bộ X", position: "" },
            role: { code: "r", name: "R", is_leader: false },
            permissions: [PETITION_READ_PERMISSION, PETITION_CREATE_PERMISSION],
            must_change_password: false,
          });
        }
        if (url === "/api/v1/citizen-letters/import-previews") return json(preview);
        if (url === "/api/v1/citizen-letters/imports") return json(imported.body, imported.status);
        if (url.startsWith("/api/v1/citizen-letter-counts")) return json({ count: 3 });
        if (url.startsWith("/api/v1/citizen-letters")) return json({ items: ROWS, next_cursor: "", has_more: false });
        if (url.startsWith("/api/v1/org-units")) return json({ items: [{ id: "U1", name: "Văn phòng" }] });
        if (url.startsWith("/api/v1/staff-directory")) return json({ items: [] });
        return new Response("{}", { status: 404 });
      }),
    );
    return calls;
  }

  const ROW = { row: 2, year: 2026, letter_type: "de-nghi", received_date: "2026-10-01", holding_unit_id: null, processing_due_at: null, sender_unknown: false };

  function importDialog(): HTMLDialogElement | undefined {
    return [...document.querySelectorAll("dialog")].find((d) => d.textContent?.includes("Nhập sổ đơn thư từ Excel"));
  }

  async function choose(name: string) {
    const input = importDialog()!.querySelector<HTMLInputElement>("#tep-nhap-don-thu")!;
    const file = new File(["PK"], name);
    Object.defineProperty(input, "files", { configurable: true, value: { 0: file, length: 1, item: () => file } });
    await act(async () => input.dispatchEvent(new Event("change", { bubbles: true })));
    await act(async () => {});
  }

  it("opens the prototype's dialog; a clean check enables “Nhập 2 đơn thư”; 201 closes it and re-reads the list and the count", async () => {
    const onBooked = vi.fn();
    const calls = importServer({ valid: true, letters: [ROW, { ...ROW, row: 3 }], errors: [] }, { status: 201, body: { created: [ROW, ROW] } });
    const el = mountNode(
      <PhienProvider>
        <LetterRegister onBooked={onBooked} />
      </PhienProvider>,
    );
    for (let i = 0; i < 5; i++) await act(async () => {});
    await act(async () => importButton(el)!.click());
    const d = importDialog()!;
    expect(d.textContent).toContain("Tải mẫu sổ đơn thư");
    expect(d.textContent).toContain("Chọn tệp .xlsx");
    const submit = () => [...importDialog()!.querySelectorAll("button")].find((b) => b.textContent?.startsWith("Nhập"))!;
    expect(submit().disabled).toBe(true);

    await choose("so-don-thu.xlsx");
    const preview = calls.find((c) => c.url === "/api/v1/citizen-letters/import-previews")!;
    expect(preview.method).toBe("POST");
    expect(importDialog()!.textContent).toContain("2 dòng hợp lệ, sẵn sàng nhập");
    expect(submit().textContent).toBe("Nhập 2 đơn thư");
    expect(submit().disabled).toBe(false);

    const listReads = calls.filter((c) => c.url.startsWith("/api/v1/citizen-letters?") || c.url === "/api/v1/citizen-letters").length;
    await act(async () => submit().click());
    await act(async () => {});
    const sent = calls.find((c) => c.url === "/api/v1/citizen-letters/imports")!;
    expect(sent.key).toBeTruthy();
    expect(importDialog()).toBeUndefined();
    expect(onBooked).toHaveBeenCalledTimes(1);
    expect(calls.filter((c) => c.url.startsWith("/api/v1/citizen-letters?") || c.url === "/api/v1/citizen-letters").length).toBeGreaterThan(listReads);
  });

  it("a refused check lists Dòng | Cột | Vấn đề and keeps “Nhập” disabled; a non-.xlsx file is stopped before any upload", async () => {
    const calls = importServer(
      { valid: false, letters: [], errors: [{ row: 4, column: "Loại đơn", message: "Loại đơn không hợp lệ." }, { row: 0, column: "", message: "Tệp quá 200 dòng." }] },
      { status: 201, body: { created: [] } },
    );
    const el = await (async () => {
      const node = mountNode(
        <PhienProvider>
          <LetterRegister />
        </PhienProvider>,
      );
      for (let i = 0; i < 5; i++) await act(async () => {});
      return node;
    })();
    await act(async () => importButton(el)!.click());
    await choose("so.csv");
    expect(calls.some((c) => c.url === "/api/v1/citizen-letters/import-previews")).toBe(false);
    expect(importDialog()!.textContent).toContain("Chỉ nhận tệp Excel .xlsx");

    await choose("so.xlsx");
    const d = importDialog()!;
    expect(d.textContent).toContain("1 dòng sai");
    expect(d.textContent).not.toContain("trên tổng số");
    const cells = [...d.querySelectorAll("tbody tr")].map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent));
    expect(cells).toEqual([
      ["Cả tệp", "—", "Tệp quá 200 dòng."],
      ["4", "Loại đơn", "Loại đơn không hợp lệ."],
    ]);
    const submit = [...d.querySelectorAll("button")].find((b) => b.textContent === "Nhập")!;
    expect(submit.disabled).toBe(true);
  });
});

describe("letter register — status filter (ADR 0084 #5)", () => {
  it("the prototype's group labels in its order, no “Chờ phân công”; choosing one sends status_group", async () => {
    const urls = stubServer([PETITION_READ_PERMISSION]);
    const el = await mountRegister();
    const select = el.querySelector<HTMLSelectElement>("#don-thu-loc-trang-thai")!;
    expect([...select.options].map((o) => o.textContent)).toEqual([
      "Tất cả trạng thái",
      "Mới vào sổ",
      "Đã phân công",
      "Đang xử lý",
      "Đã giải quyết",
      "Chuyển cấp trên",
      "Lưu, không thụ lý",
    ]);
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!.call(select, "da-phan-cong");
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => {});
    const last = urls.filter((u) => u.startsWith("/api/v1/citizen-letters")).at(-1)!;
    const q = new URL(last, "http://x").searchParams;
    expect(q.get("status_group")).toBe("da-phan-cong");
    expect(q.get("status")).toBeNull();
  });
});

describe("letter register — permission gating (UX only; the server decides)", () => {
  it("ALLOWED: petition.create draws “Vào sổ đơn thư” and an ENABLED “Nhập từ Excel” — no “?” left", async () => {
    stubServer([PETITION_READ_PERMISSION, PETITION_CREATE_PERMISSION]);
    const el = await mountRegister();
    expect(bookButton(el)).toBeDefined();
    const excel = importButton(el)!;
    expect(excel.disabled).toBe(false);
    expect(el.querySelector("[data-pending-marker]")).toBeNull();
  });

  it("DENIED: petition.read alone sees the register but no booking and no import button", async () => {
    stubServer([PETITION_READ_PERMISSION]);
    const el = await mountRegister();
    expect(el.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(bookButton(el)).toBeUndefined();
    expect(importButton(el)).toBeUndefined();
  });

  it("scope “Giao cho tôi” asks the server with scope=mine — never a staff code from the client", async () => {
    const urls = stubServer([PETITION_READ_PERMISSION]);
    const el = await mountRegister();
    const mine = [...el.querySelectorAll<HTMLButtonElement>('[aria-label="Lọc nhanh theo người xử lý"] button')].find(
      (b) => b.textContent === "Giao cho tôi",
    )!;
    await act(async () => mine.click());
    await act(async () => {});
    const last = urls.filter((u) => u.startsWith("/api/v1/citizen-letters")).at(-1)!;
    expect(last).toContain("scope=mine");
    expect(last).not.toContain("CB-00009");
  });
});
