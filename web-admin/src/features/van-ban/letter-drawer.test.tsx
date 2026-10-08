// @vitest-environment jsdom
//
// jsdom: chips are pressed, composers open, writes reach a fake `fetch` whose bodies are read.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PREVIEW_STAFF, PREVIEW_UNITS } from "@/dev-preview/disbursement.fixture";
import { answer as fixtureAnswer } from "@/dev-preview/fixture-fetch";
import { danhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { documents_citizenLetterOut, documents_letterLogOut } from "@/lib/api/schema.gen";

import { DEADLINE_CLEARED, DEADLINE_EDIT_LABEL, DEADLINE_SAVED, LetterDrawer, RESULT_LOCKED_HINT, letterDrawerTitle } from "./letter-drawer";
import { NO_DEADLINE, WITHHELD_SENDER } from "./letter-display";

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

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function letter(over: Partial<documents_citizenLetterOut> = {}): documents_citizenLetterOut {
  return {
    id: "L1",
    number: 7,
    year: 2026,
    received_date: "2026-10-01",
    letter_type: "kien-nghi-phan-anh",
    source: "nhap-tay",
    sender_name: "Nguyễn Văn A",
    sender_phone: "09****0000",
    has_sender_address: true,
    sender_unknown: false,
    identity_withheld: false,
    summary: "Đường thôn bị ngập sau mưa",
    summary_withheld: false,
    status: "moi-vao-so",
    status_group: "moi-vao-so",
    next_statuses: ["dang-xu-ly-don"],
    holding_unit_id: "",
    assignee_code: "",
    processing_due_at: null,
    resolution_due_at: null,
    accepted_at: null,
    resolved_at: null,
    closed_at: null,
    days_open: 8,
    is_resolved: false,
    is_closed: false,
    result_summary: null,
    created_by_code: "CB-00001",
    created_at: "2026-10-01T02:00:00Z",
    updated_at: "2026-10-01T02:00:00Z",
    ...over,
  };
}

const EMPTY_LOG: documents_letterLogOut = { items: [] };

function drawer(
  doc: documents_citizenLetterOut,
  opts: { canBook?: boolean; mayWork?: boolean; log?: documents_letterLogOut; canRaiseTask?: boolean; onChanged?: () => void } = {},
) {
  return (
    <LetterDrawer
      letter={{ ok: true, duLieu: doc }}
      log={{ ok: true, duLieu: opts.log ?? EMPTY_LOG }}
      now={new Date("2026-10-08T03:00:00Z")}
      units={{ pha: "xong", ten: new Map([["U1", "Văn phòng"]]) }}
      canBook={opts.canBook ?? true}
      mayWork={opts.mayWork ?? true}
      canRaiseTask={opts.canRaiseTask ?? false}
      staff={{ ok: true, duLieu: PREVIEW_STAFF }}
      onChanged={opts.onChanged ?? (() => {})}
      onClose={() => {}}
    />
  );
}

function dialog(): HTMLElement {
  return document.querySelector("dialog")!;
}

function chips(): HTMLButtonElement[] {
  return [...dialog().querySelectorAll<HTMLButtonElement>('[role="group"] button')];
}

function button(text: string): HTMLButtonElement | undefined {
  return [...dialog().querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent?.trim() === text);
}

describe("letter drawer — status strip (ADR 0084 #2, prototype :385-484)", () => {
  const first = (b: HTMLButtonElement) => b.firstChild?.textContent;

  it("Mới vào sổ, no unit: “Phân công” and “Đang xử lý” are pressable; both rows always drawn with their sub-lines", () => {
    mount(drawer(letter()));
    expect(chips().map(first)).toEqual(["Mới vào sổ", "Phân công", "Đang xử lý", "Đã giải quyết", "Chuyển cấp trên", "Lưu, không thụ lý"]);
    expect(chips().map((b) => b.lastChild?.textContent)).toEqual(["đang ở đây", "chọn bộ phận xử lý", "chuyển sang", "—", "—", "—"]);
    expect(chips().filter((b) => !b.disabled).map(first)).toEqual(["Phân công", "Đang xử lý"]);
    expect(dialog().textContent).toContain("Rẽ nhánh:");
    expect(dialog().textContent).toContain("Đã vào sổ, chưa giao cho bộ phận nào.");
  });

  it("“Phân công” moves nothing: it focuses the “Chuyển cho bộ phận khác” box", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter()));
    act(() => chips().find((b) => first(b) === "Phân công")!.click());
    expect(document.activeElement?.id).toBe("chuyen-don-thu-bo-phan");
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(dialog().textContent).not.toContain("Chuyển sang “");
  });

  it("Mới vào sổ held by a unit reads “Đã phân công · đang ở đây” with the prototype's hint", () => {
    mount(drawer(letter({ holding_unit_id: "U1", status_group: "da-phan-cong" })));
    const current = chips().find((b) => b.getAttribute("aria-current") === "step")!;
    expect(current.textContent).toBe("Đã phân côngđang ở đây");
    expect(dialog().textContent).toContain("Đã giao cho bộ phận, chờ bộ phận bắt tay vào việc.");
  });

  it("Đang xử lý: the composer asks for the concrete TT 05 step, allowed ones only, and posts the one chosen", async () => {
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter()), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(
      drawer(
        letter({
          status: "dang-xu-ly-don",
          status_group: "dang-xu-ly",
          holding_unit_id: "U1",
          next_statuses: ["thu-ly", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"],
        }),
      ),
    );
    // The current group stays pressable: Thụ lý is a step INSIDE it.
    expect(chips().filter((b) => !b.disabled).map(first)).toEqual(["Đang xử lý", "Chuyển cấp trên", "Lưu, không thụ lý"]);
    act(() => chips().find((b) => first(b) === "Lưu, không thụ lý")!.click());
    expect(dialog().textContent).toContain("Chuyển sang “Lưu, không thụ lý”");
    const select = dialog().querySelector<HTMLSelectElement>("#don-thu-buoc-cu-the")!;
    expect([...select.options].map((o) => [o.value, o.textContent])).toEqual([
      ["khong-thu-ly", "Không thụ lý"],
      ["huong-dan", "Hướng dẫn"],
      ["luu-don", "Lưu đơn"],
    ]);
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!.call(select, "huong-dan");
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => button("Xác nhận")!.click());
    await act(async () => {});
    const [url, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/citizen-letters/L1/status");
    expect(JSON.parse(String(init.body))).toEqual({ status: "huong-dan" });
  });

  it("Chuyển cấp trên has one step: no select, posts chuyen-don", async () => {
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter()), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter({ status: "dang-xu-ly-don", status_group: "dang-xu-ly", holding_unit_id: "U1", next_statuses: ["thu-ly", "chuyen-don"] })));
    act(() => chips().find((b) => first(b) === "Chuyển cấp trên")!.click());
    expect(dialog().querySelector("#don-thu-buoc-cu-the")).toBeNull();
    await act(async () => button("Xác nhận")!.click());
    const [, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({ status: "chuyen-don" });
  });

  it("Đã giải quyết without a result: the composer posts the status and shows the SERVER's 409 sentence", async () => {
    const sentence = "Chưa ghi kết quả giải quyết (văn bản đã ban hành và tóm tắt) nên chưa chuyển được sang Đã giải quyết. Hãy ghi kết quả trước.";
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify({ code: "letter_state", message: sentence, trace_id: "" }), { status: 409 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter({ status: "dang-giai-quyet", status_group: "dang-xu-ly", holding_unit_id: "U1", next_statuses: ["da-giai-quyet", "dinh-chi"] })));
    act(() => chips().find((b) => b.textContent?.startsWith("Đã giải quyết"))!.click());
    expect(dialog().textContent).toContain("Chuyển sang “Đã giải quyết”");
    await act(async () => button("Xác nhận")!.click());
    await act(async () => {});
    const [url, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/citizen-letters/L1/status");
    expect(JSON.parse(String(init.body))).toEqual({ status: "da-giai-quyet" });
    expect(dialog().querySelector('[role="alert"]')!.textContent).toBe(sentence);
  });

  it("DENIED: a viewer who may not work on the letter gets no strip, no note box, no result form", () => {
    mount(drawer(letter({ status: "thu-ly", status_group: "dang-xu-ly", next_statuses: ["dang-giai-quyet"] }), { canBook: false, mayWork: false }));
    expect(chips().filter((b) => b.getAttribute("aria-current") !== null)).toHaveLength(0);
    expect(dialog().textContent).not.toContain("Ghi nhật ký");
    expect(dialog().textContent).not.toContain("Nội dung trả lời công dân");
    expect(dialog().textContent).not.toContain("Chuyển cho bộ phận khác");
    expect(dialog().textContent).not.toContain("Sửa thông tin người gửi");
  });

  it("DENIED routing: without petition.create the “Phân công” chip is drawn but not pressable", () => {
    mount(drawer(letter(), { canBook: false, mayWork: true }));
    expect(chips().find((b) => first(b) === "Phân công")!.disabled).toBe(true);
  });
});

describe("letter drawer — chip row (prototype :489-511)", () => {
  it("status GROUP · type · source · deadline", () => {
    mount(drawer(letter({ source: "mini-app", holding_unit_id: "U1", status_group: "da-phan-cong" })));
    const row = dialog().querySelector("[data-letter-chips]")!;
    expect(row.textContent).toBe("Đã phân côngKiến nghị, phản ánhMini AppKhông đặt hạn");
  });
});

describe("letter drawer — masking", () => {
  it("the phone is the masked TEXT — no tel: link anywhere; the address is never shown", () => {
    mount(drawer(letter()));
    expect(dialog().textContent).toContain("09****0000");
    expect(dialog().querySelector('a[href^="tel:"]')).toBeNull();
    expect(dialog().innerHTML).not.toContain("tel:");
    expect(dialog().textContent).toContain("Có địa chỉ (không hiển thị)");
  });

  it("a denunciation for a non-assignee: the sender is one fixed sentence", () => {
    mount(
      drawer(
        letter({
          letter_type: "to-cao",
          identity_withheld: true,
          sender_name: null,
          sender_phone: null,
          has_sender_address: false,
          sender_unknown: null,
          assignee_code: "CB-00002",
        }),
      ),
    );
    const header = dialog().querySelector("header")!;
    expect(header.textContent).toContain(WITHHELD_SENDER);
    expect(dialog().textContent).toContain("Tố cáo");
  });

  it("the sender-correction form opens EMPTY; the masked phone is a placeholder only", () => {
    mount(drawer(letter()));
    act(() => button("Sửa thông tin người gửi")!.click());
    const phone = dialog().querySelector<HTMLInputElement>("#sua-nguoi-gui-sdt")!;
    expect(phone.value).toBe("");
    expect(phone.placeholder).toBe("09****0000");
    expect(dialog().querySelector<HTMLInputElement>("#sua-nguoi-gui-ten")!.value).toBe("");
    expect(dialog().querySelector<HTMLInputElement>("#sua-nguoi-gui-dia-chi")!.value).toBe("");
  });

  it("no deadline anywhere reads “Không đặt hạn”", () => {
    mount(drawer(letter()));
    expect(dialog().textContent).toContain(NO_DEADLINE);
  });
});

describe("letter drawer — the clerk's “Hạn xử lý” (ADR 0079 lô 5 Q18)", () => {
  function figure(): string {
    const dt = [...dialog().querySelectorAll("dt")].find((d) => d.textContent === "Hạn xử lý")!;
    return dt.nextElementSibling!.textContent ?? "";
  }

  function typeDate(value: string) {
    const el = dialog().querySelector<HTMLInputElement>("#don-thu-han-xu-ly")!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
      el.dispatchEvent(new Event("input", { bubbles: true }));
    });
  }

  it("no deadline: the figure reads the prototype's “Không đặt”", () => {
    mount(drawer(letter()));
    expect(figure()).toBe("Không đặt");
  });

  it("a stored deadline of the CURRENT phase reads as its date (resolution from Thụ lý)", () => {
    mount(drawer(letter({ status: "thu-ly", status_group: "dang-xu-ly", accepted_at: "2026-10-02T02:00:00Z", next_statuses: ["dang-giai-quyet"], processing_due_at: "2026-10-03T10:00:00Z", resolution_due_at: "2026-10-20T10:00:00Z" })));
    expect(figure()).toBe("20/10/2026");
  });

  it("a finished letter still shows the deadline of the phase it ended in (prototype: one figure, its `sla.due_at`)", () => {
    mount(drawer(letter({ status: "luu-don", status_group: "luu-khong-thu-ly", is_closed: true, next_statuses: [], processing_due_at: "2026-10-03T10:00:00Z" })));
    expect(figure()).toBe("3/10/2026");
  });

  it("set: PATCH …/deadline with exactly { due_at } at 17:00 +07:00 of the chosen day; success says so", async () => {
    const changed = vi.fn();
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter({ processing_due_at: "2026-10-20T10:00:00Z" })), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(
      <LetterDrawer
        letter={{ ok: true, duLieu: letter() }}
        log={{ ok: true, duLieu: EMPTY_LOG }}
        now={new Date("2026-10-08T03:00:00Z")}
        units={{ pha: "xong", ten: new Map() }}
        canBook
        mayWork
        onChanged={changed}
        onClose={() => {}}
      />,
    );
    act(() => button(DEADLINE_EDIT_LABEL)!.click());
    // Nothing to clear yet: no “Không đặt” button while the letter has no deadline.
    expect(button("Không đặt")).toBeUndefined();
    typeDate("2026-10-20");
    await act(async () => button("Lưu")!.click());
    await act(async () => {});
    const [url, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/citizen-letters/L1/deadline");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(String(init.body))).toEqual({ due_at: "2026-10-20T17:00:00+07:00" });
    expect(changed).toHaveBeenCalledTimes(1);
    expect(dialog().textContent).toContain(DEADLINE_SAVED);
  });

  it("clear: “Không đặt” sends { due_at: null }; the editor opens on the stored date", async () => {
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter()), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter({ processing_due_at: "2026-10-19T20:00:00Z" })));
    act(() => button(DEADLINE_EDIT_LABEL)!.click());
    expect(dialog().querySelector<HTMLInputElement>("#don-thu-han-xu-ly")!.value).toBe("2026-10-20");
    await act(async () => button("Không đặt")!.click());
    await act(async () => {});
    const [url, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/citizen-letters/L1/deadline");
    expect(JSON.parse(String(init.body))).toEqual({ due_at: null });
    expect(dialog().textContent).toContain(DEADLINE_CLEARED);
  });

  it("409 (finished meanwhile): the SERVER's sentence is shown in the editor", async () => {
    const sentence = "Đơn đã kết thúc xử lý nên không đặt hay bỏ hạn xử lý được.";
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ code: "letter_state", message: sentence, trace_id: "" }), { status: 409 })));
    mount(drawer(letter()));
    act(() => button(DEADLINE_EDIT_LABEL)!.click());
    typeDate("2026-10-20");
    await act(async () => button("Lưu")!.click());
    await act(async () => {});
    expect(dialog().querySelector('form[aria-label="Sửa hạn xử lý"] [role="alert"]')!.textContent).toBe(sentence);
  });

  it("400 (year out of range): the SERVER's sentence is shown", async () => {
    const sentence = "Hạn xử lý: năm phải từ 2000 đến 2200.";
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ code: "invalid_request", message: sentence, trace_id: "" }), { status: 400 })));
    mount(drawer(letter()));
    act(() => button(DEADLINE_EDIT_LABEL)!.click());
    typeDate("2300-01-01");
    await act(async () => button("Lưu")!.click());
    await act(async () => {});
    expect(dialog().querySelector('[role="alert"]')!.textContent).toBe(sentence);
  });

  it("DENIED: without petition.create there is no edit control — the figure is still read", () => {
    mount(drawer(letter({ processing_due_at: "2026-10-20T10:00:00Z" }), { canBook: false, mayWork: true }));
    expect(button(DEADLINE_EDIT_LABEL)).toBeUndefined();
    expect(dialog().querySelector("#don-thu-han-xu-ly")).toBeNull();
    expect(figure()).toBe("20/10/2026");
  });

  it("a finished letter offers no edit even to petition.create (no phase deadline; server would 409)", () => {
    mount(drawer(letter({ status: "luu-don", next_statuses: [], is_closed: true })));
    expect(button(DEADLINE_EDIT_LABEL)).toBeUndefined();
  });
});

describe("letter drawer — reply “Nội dung trả lời công dân” (ADR 0084 #2)", () => {
  const type = (id: string, value: string) => {
    const el = dialog().querySelector<HTMLInputElement | HTMLTextAreaElement>(`#${id}`)!;
    const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    act(() => {
      Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
      el.dispatchEvent(new Event("input", { bubbles: true }));
    });
  };
  const resolving = { status: "dang-giai-quyet", status_group: "dang-xu-ly", holding_unit_id: "U1", next_statuses: ["da-giai-quyet", "dinh-chi"] };

  it("locked outside Thụ lý / Đang giải quyết, with the server's sentence", () => {
    mount(drawer(letter({ status: "dang-xu-ly-don", status_group: "dang-xu-ly", next_statuses: ["thu-ly"] })));
    expect(dialog().querySelector<HTMLTextAreaElement>("#ket-qua-tom-tat")!.disabled).toBe(true);
    expect(dialog().textContent).toContain(RESULT_LOCKED_HINT);
    expect(button("Lưu nội dung trả lời")).toBeUndefined();
  });

  it("kiến nghị-phản ánh: ONE textarea with the prototype's placeholder, no document field, no “Tóm tắt kết quả”; PUT the summary alone", async () => {
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter(resolving)), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter(resolving)));
    const area = dialog().querySelector<HTMLTextAreaElement>("#ket-qua-tom-tat")!;
    expect(area.placeholder).toBe("Ghi rõ kết quả giải quyết để trả lời người gửi đơn.");
    expect(dialog().querySelector("#ket-qua-so-van-ban")).toBeNull();
    expect(dialog().textContent).not.toContain("Tóm tắt kết quả");
    type("ket-qua-tom-tat", "Đã nạo vét mương thoát nước");
    await act(async () => button("Lưu nội dung trả lời")!.click());
    const [url, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/citizen-letters/L1/result");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({ result_summary: "Đã nạo vét mương thoát nước" });
  });

  it("khiếu nại: the four document fields BELOW the textarea, all five sent", async () => {
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter(resolving)), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter({ ...resolving, letter_type: "khieu-nai" })));
    const html = dialog().innerHTML;
    expect(html.indexOf('id="ket-qua-tom-tat"')).toBeLessThan(html.indexOf('id="ket-qua-so-van-ban"'));
    type("ket-qua-tom-tat", "Đã giải quyết khiếu nại");
    type("ket-qua-so-van-ban", "12/QĐ-UBND");
    type("ket-qua-ngay-ban-hanh", "2026-10-07");
    type("ket-qua-nguoi-ky", "Chủ tịch UBND xã");
    type("ket-qua-co-quan", "UBND xã");
    await act(async () => button("Lưu nội dung trả lời")!.click());
    const [, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({
      result_document_no: "12/QĐ-UBND",
      result_document_date: "2026-10-07",
      result_signer: "Chủ tịch UBND xã",
      result_issuer: "UBND xã",
      result_summary: "Đã giải quyết khiếu nại",
    });
  });

  it("a saved summary-only reply is shown read-only to a viewer who may not work on the letter", () => {
    mount(
      drawer(letter({ status: "da-giai-quyet", status_group: "da-giai-quyet", is_closed: true, next_statuses: [], result_summary: "Đã trả lời" }), {
        canBook: false,
        mayWork: false,
      }),
    );
    expect(dialog().querySelector<HTMLTextAreaElement>("#ket-qua-tom-tat")!.value).toBe("Đã trả lời");
  });
});

describe("letter drawer — prototype presentation (TASK-02)", () => {
  it("header line reads d/m/yyyy without zero padding", () => {
    expect(letterDrawerTitle(7, 2026, "2026-09-04")).toBe("Đơn số 7/2026 · nhận ngày 4/9/2026");
    mount(drawer(letter()));
    expect(dialog().querySelector("h2")!.textContent).toBe("Đơn số 7/2026 · nhận ngày 1/10/2026");
  });

  it("“Bộ phận đang giữ” is the unit name only — no “Phụ trách” line; “Chưa chuyển” when none", () => {
    const held = () => {
      const dt = [...dialog().querySelectorAll("dt")].find((d) => d.textContent === "Bộ phận đang giữ")!;
      return dt.nextElementSibling!.textContent;
    };
    mount(drawer(letter({ holding_unit_id: "U1", assignee_code: "CB-00002" })));
    expect(held()).toBe("Văn phòng");
    act(() => root?.unmount());
    host?.remove();
    mount(drawer(letter()));
    expect(held()).toBe("Chưa chuyển");
  });

  it("the task button carries the prototype's sub-line", () => {
    mount(drawer(letter(), { canRaiseTask: true }));
    expect(dialog().textContent).toContain("Nhiệm vụ kế thừa hạn xử lý của đơn, để hai bên không lệch nhau.");
  });

  it("the deadline CHIP reads the figure's instant: a finished letter keeps its stage's date (TASK-09b row 4)", () => {
    mount(drawer(letter({ status: "luu-don", status_group: "luu-khong-thu-ly", is_closed: true, next_statuses: [], processing_due_at: "2026-10-03T10:00:00Z" })));
    expect(dialog().querySelector("[data-letter-chips]")!.textContent).toContain("Hạn 3/10/2026");
    expect(dialog().querySelector("[data-letter-chips]")!.textContent).not.toContain(NO_DEADLINE);
  });
});

describe("letter drawer — “Chuyển thành nhiệm vụ” (ADR 0085 A, prototype :590-627)", () => {
  const UNIT = PREVIEW_UNITS.items[0]!.id;
  const held = () =>
    letter({
      holding_unit_id: UNIT,
      assignee_code: "CB-00002",
      status: "thu-ly",
      status_group: "dang-xu-ly",
      accepted_at: "2026-10-02T02:00:00Z",
      next_statuses: ["dang-giai-quyet"],
      processing_due_at: "2026-10-03T10:00:00Z",
      resolution_due_at: "2026-10-20T10:00:00Z",
    });

  /** GETs answered by the dev-preview fixtures (catalogues, directories); POSTs recorded and answered by `post`. */
  function server(post: (body: Record<string, unknown>, key: string) => Response) {
    const posts: { body: Record<string, unknown>; key: string }[] = [];
    const fake = vi.fn(async (input: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const url = new URL(input, "http://xa-a.vigov.test");
      if (method === "GET") return fixtureAnswer("GET", url);
      const body = JSON.parse(String(init?.body)) as Record<string, unknown>;
      const key = (init?.headers as Record<string, string>)["Idempotency-Key"]!;
      posts.push({ body, key });
      expect(url.pathname).toBe("/api/v1/citizen-letter-tasks");
      return post(body, key);
    });
    vi.stubGlobal("fetch", fake);
    return posts;
  }
  const created = (code: string) => () => new Response(JSON.stringify({ code }), { status: 201, headers: { "Content-Type": "application/json" } });
  const refused = (status: number, code: string, message: string) => () =>
    new Response(JSON.stringify({ code, message, trace_id: "" }), { status, headers: { "Content-Type": "application/json" } });

  /** The task dialog — the second `<dialog>`, over the drawer. */
  function taskDialog(): HTMLDialogElement {
    return [...document.querySelectorAll("dialog")].find((d) => d.textContent?.includes("Tạo nhiệm vụ từ hồ sơ này"))!;
  }
  async function openDialog() {
    await act(async () => button("Chuyển thành nhiệm vụ")!.click());
    // Catalogues load on first open.
    await act(async () => {});
  }
  async function submit() {
    const form = taskDialog().querySelector("form")!;
    await act(async () => form.requestSubmit());
    await act(async () => {});
  }

  it("DENIED: without task.create + petition.read there is no button and no sub-line", () => {
    mount(drawer(held(), { canRaiseTask: false }));
    expect(button("Chuyển thành nhiệm vụ")).toBeUndefined();
    expect(dialog().textContent).not.toContain("Nhiệm vụ kế thừa hạn xử lý của đơn");
  });

  it("a DENUNCIATION never shows the button, even to a holder of both keys (server 422 is only the backstop)", () => {
    mount(drawer(letter({ letter_type: "to-cao", identity_withheld: true, summary: null, summary_withheld: true }), { canRaiseTask: true }));
    expect(button("Chuyển thành nhiệm vụ")).toBeUndefined();
  });

  it("allowed: enabled, no “?” — the dialog opens with title, unit and assignee pre-filled and the deadline READ-ONLY", async () => {
    server(created("NV41"));
    mount(drawer(held(), { canRaiseTask: true }));
    const raise = button("Chuyển thành nhiệm vụ")!;
    expect(raise.disabled).toBe(false);
    expect(dialog().querySelector("[data-pending-marker]")).toBeNull();
    await openDialog();
    const d = taskDialog();
    expect(d.querySelector<HTMLInputElement>("#giao-tieu-de")!.value).toBe("Đường thôn bị ngập sau mưa");
    expect(d.querySelector<HTMLSelectElement>("#giao-bo-phan")!.value).toBe(UNIT);
    expect(d.querySelector<HTMLSelectElement>("#giao-nguoi-thuc-hien")!.value).toBe("CB-00002");
    // No editable deadline: the server sets it from the letter.
    expect(d.querySelector("#giao-han")).toBeNull();
    const due = d.querySelector("[data-inherited-due]")!.textContent;
    expect(due).toContain("17:00 ngày 20/10/2026");
    expect(due).toContain("Kế thừa hạn của đơn");
  });

  it("submit: POST with letter_id, the edited fields, NO due_at / source; success names the code and links to it", async () => {
    const changed = vi.fn();
    const posts = server(created("NV41"));
    mount(drawer(held(), { canRaiseTask: true, onChanged: changed }));
    await openDialog();
    await submit();
    expect(posts).toHaveLength(1);
    const body = posts[0]!.body;
    expect(body.letter_id).toBe("L1");
    expect(body.title).toBe("Đường thôn bị ngập sau mưa");
    expect(body.unit).toBe(UNIT);
    expect(body.assignee).toBe("CB-00002");
    expect(body.type).toBe("co-ban");
    for (const k of ["due_at", "source", "source_id"]) expect(body, k).not.toHaveProperty(k);
    expect(posts[0]!.key).not.toBe("");
    // The dialog closed; the outcome is in the drawer, with the task's link.
    expect(taskDialog()).toBeUndefined();
    const status = [...dialog().querySelectorAll('[role="status"]')].find((p) => p.textContent?.includes("NV41"))!;
    expect(status.textContent).toContain("Đã tạo nhiệm vụ NV41.");
    expect(status.querySelector("a")!.getAttribute("href")).toBe("/nhiem-vu?task=NV41");
    expect(changed).toHaveBeenCalled();
  });

  it("a refusal is shown VERBATIM in the dialog; a retry reuses the key, a NEW opening takes a fresh one", async () => {
    let answer = refused(422, "assignment_required", "Chọn bộ phận hoặc người thực hiện.");
    const posts = server(() => answer());
    mount(drawer(held(), { canRaiseTask: true }));
    await openDialog();
    await submit();
    expect(taskDialog().textContent).toContain("Chọn bộ phận hoặc người thực hiện.");
    await submit();
    expect(posts[1]!.key).toBe(posts[0]!.key);
    // Huỷ, then open again: a new attempt.
    await act(async () => [...taskDialog().querySelectorAll("button")].find((b) => b.textContent === "Huỷ")!.click());
    await openDialog();
    answer = created("NV42");
    await submit();
    expect(posts[2]!.key).not.toBe(posts[0]!.key);
  });

  it("a letter with no deadline: the dialog says the task will have none — and still sends no due_at", async () => {
    const posts = server(created("NV43"));
    mount(drawer(letter({ holding_unit_id: UNIT }), { canRaiseTask: true }));
    await openDialog();
    const due = taskDialog().querySelector("[data-inherited-due]")!.textContent;
    expect(due).toContain(NO_DEADLINE);
    expect(due).toContain("Đơn không đặt hạn nên nhiệm vụ cũng không có hạn.");
    await submit();
    expect(posts[0]!.body).not.toHaveProperty("due_at");
  });

  it("the timeline names the actor only (the prototype's `actor_name`); the bare code when the directory does not know it", () => {
    mount(
      <LetterDrawer
        letter={{ ok: true, duLieu: letter() }}
        log={{
          ok: true,
          duLieu: {
            items: [
              { id: "2", letter_id: "L1", at: "2026-10-02T02:00:00Z", actor_code: "CB-00009", kind: "ghi-chu", content: "x" },
              { id: "1", letter_id: "L1", at: "2026-10-01T02:00:00Z", actor_code: "CB-00001", kind: "ghi-chu", content: "y" },
            ],
          },
        }}
        now={new Date("2026-10-08T03:00:00Z")}
        units={{ pha: "xong", ten: new Map() }}
        directory={danhBaTheoMa([{ code: "CB-00001", full_name: "Trần Thị B", position: "", department_id: "", email_masked: null }])}
        canBook
        mayWork
        onChanged={() => {}}
        onClose={() => {}}
      />,
    );
    const names = [...dialog().querySelectorAll('section[aria-labelledby="tieu-de-dong-thoi-gian-don-thu"] li b')].map((b) => b.textContent);
    expect(names[0]).toBe("CB-00009");
    expect(names[1]).toBe("Trần Thị B");
  });
});

describe("letter drawer — timeline “Phụ trách” (doc §4.5)", () => {
  it("names the person only, like the actor; the bare code when the directory does not know it", () => {
    mount(
      <LetterDrawer
        letter={{ ok: true, duLieu: letter() }}
        log={{
          ok: true,
          duLieu: {
            items: [
              { id: "2", letter_id: "L1", at: "2026-10-02T02:00:00Z", actor_code: "CB-00001", kind: "luan-chuyen", to_unit_id: "U1", assignee_code: "CB-00001" },
              { id: "1", letter_id: "L1", at: "2026-10-01T02:00:00Z", actor_code: "CB-00001", kind: "luan-chuyen", to_unit_id: "U1", assignee_code: "CB-00077" },
            ],
          },
        }}
        now={new Date("2026-10-08T03:00:00Z")}
        units={{ pha: "xong", ten: new Map([["U1", "Văn phòng"]]) }}
        directory={danhBaTheoMa([{ code: "CB-00001", full_name: "Trần Thị B", position: "", department_id: "", email_masked: null }])}
        canBook
        mayWork
        onChanged={() => {}}
        onClose={() => {}}
      />,
    );
    const rows = [...dialog().querySelectorAll('section[aria-labelledby="tieu-de-dong-thoi-gian-don-thu"] li')];
    expect(rows[0]!.textContent).toContain("Phụ trách: Trần Thị B");
    expect(rows[0]!.textContent).not.toContain("CB-00001)");
    expect(rows[1]!.textContent).toContain("Phụ trách: CB-00077");
  });
});

describe("letter drawer — log", () => {
  it("empty log: the prototype's sentence; three entries: three read-only lines, newest first as sent", () => {
    mount(drawer(letter()));
    expect(dialog().textContent).toContain("Chưa chuyển cho bộ phận nào.");
    act(() => root?.unmount());
    host?.remove();
    mount(
      drawer(letter(), {
        log: {
          items: [
            { id: "3", letter_id: "L1", at: "2026-10-03T02:00:00Z", actor_code: "CB-00002", kind: "ghi-chu", content: "Đã gọi điện" },
            { id: "2", letter_id: "L1", at: "2026-10-02T02:00:00Z", actor_code: "CB-00001", kind: "luan-chuyen", to_unit_id: "U1", content: "Chuyển xử lý" },
            { id: "1", letter_id: "L1", at: "2026-10-01T02:00:00Z", actor_code: "CB-00001", kind: "chuyen-trang-thai", from_status: "moi-vao-so", to_status: "dang-xu-ly-don" },
          ],
        },
      }),
    );
    const rows = [...dialog().querySelectorAll('section[aria-labelledby="tieu-de-dong-thoi-gian-don-thu"] li')];
    expect(rows).toHaveLength(3);
    expect(rows[0]!.textContent).toContain("Đã gọi điện");
    expect(rows[1]!.textContent).toContain("Văn phòng");
    expect(rows[2]!.textContent).toContain("Đang xử lý đơn");
    for (const r of rows) expect(r.querySelector("button")).toBeNull();
  });
});
