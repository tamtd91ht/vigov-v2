// @vitest-environment jsdom
//
// jsdom: chips are pressed, composers open, writes reach a fake `fetch` whose bodies are read.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { documents_citizenLetterOut, documents_letterLogOut } from "@/lib/api/schema.gen";

import { DEADLINE_CLEARED, DEADLINE_EDIT_LABEL, DEADLINE_SAVED, LetterDrawer, RESULT_LOCKED_HINT } from "./letter-drawer";
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
    sender_name: "Nguyễn Văn A",
    sender_phone: "09****0000",
    has_sender_address: true,
    sender_unknown: false,
    identity_withheld: false,
    summary: "Đường thôn bị ngập sau mưa",
    summary_withheld: false,
    status: "moi-vao-so",
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

function drawer(doc: documents_citizenLetterOut, opts: { canBook?: boolean; mayWork?: boolean; log?: documents_letterLogOut } = {}) {
  return (
    <LetterDrawer
      letter={{ ok: true, duLieu: doc }}
      log={{ ok: true, duLieu: opts.log ?? EMPTY_LOG }}
      now={new Date("2026-10-08T03:00:00Z")}
      units={{ pha: "xong", ten: new Map([["U1", "Văn phòng"]]) }}
      canBook={opts.canBook ?? true}
      mayWork={opts.mayWork ?? true}
      onChanged={() => {}}
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

describe("letter drawer — status chips", () => {
  it("only the server's next status is clickable; the current one says “đang ở đây”", () => {
    mount(drawer(letter()));
    const clickable = chips().filter((b) => !b.disabled);
    expect(clickable.map((b) => b.textContent)).toEqual(["Đang xử lý đơnchuyển sang"]);
    const current = chips().find((b) => b.getAttribute("aria-current") === "step")!;
    expect(current.textContent).toContain("Mới vào sổ");
    expect(current.textContent).toContain("đang ở đây");
    // No branch row from Mới vào sổ: none of its ends is reachable in one move.
    expect(dialog().textContent).not.toContain("Rẽ nhánh:");
  });

  it("from Đang xử lý đơn the branch row shows its four ends, all clickable", () => {
    mount(drawer(letter({ status: "dang-xu-ly-don", next_statuses: ["thu-ly", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"] })));
    expect(dialog().textContent).toContain("Rẽ nhánh:");
    expect(chips().filter((b) => !b.disabled).map((b) => b.firstChild?.textContent)).toEqual([
      "Thụ lý",
      "Không thụ lý",
      "Hướng dẫn",
      "Chuyển đơn",
      "Lưu đơn",
    ]);
  });

  it("Đã giải quyết without a result: the composer posts the status and shows the SERVER's 409 sentence", async () => {
    const sentence = "Chưa ghi kết quả giải quyết (văn bản đã ban hành và tóm tắt) nên chưa chuyển được sang Đã giải quyết. Hãy ghi kết quả trước.";
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify({ code: "letter_state", message: sentence, trace_id: "" }), { status: 409 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter({ status: "dang-giai-quyet", next_statuses: ["da-giai-quyet", "dinh-chi"] })));
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
    mount(drawer(letter({ status: "thu-ly", next_statuses: ["dang-giai-quyet"] }), { canBook: false, mayWork: false }));
    expect(chips().filter((b) => b.getAttribute("aria-current") !== null)).toHaveLength(0);
    expect(dialog().textContent).not.toContain("Ghi nhật ký");
    expect(dialog().textContent).not.toContain("Nội dung trả lời công dân");
    expect(dialog().textContent).not.toContain("Chuyển cho bộ phận khác");
    expect(dialog().textContent).not.toContain("Sửa thông tin người gửi");
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
    mount(drawer(letter({ status: "thu-ly", next_statuses: ["dang-giai-quyet"], processing_due_at: "2026-10-03T10:00:00Z", resolution_due_at: "2026-10-20T10:00:00Z" })));
    expect(figure()).toBe("20/10/2026");
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

describe("letter drawer — result (C10)", () => {
  it("locked outside Thụ lý / Đang giải quyết, with the server's sentence", () => {
    mount(drawer(letter({ status: "dang-xu-ly-don", next_statuses: ["thu-ly"] })));
    const no = dialog().querySelector<HTMLInputElement>("#ket-qua-so-van-ban")!;
    expect(no.disabled).toBe(true);
    expect(dialog().textContent).toContain(RESULT_LOCKED_HINT);
    expect(button("Lưu nội dung trả lời")).toBeUndefined();
  });

  it("in Đang giải quyết: the five fields are sent as a PUT", async () => {
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify(letter({ status: "dang-giai-quyet" })), { status: 200 }));
    vi.stubGlobal("fetch", fetchSpy);
    mount(drawer(letter({ status: "dang-giai-quyet", next_statuses: ["da-giai-quyet", "dinh-chi"] })));
    const type = (id: string, value: string) => {
      const el = dialog().querySelector<HTMLInputElement | HTMLTextAreaElement>(`#${id}`)!;
      const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      act(() => {
        Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
        el.dispatchEvent(new Event("input", { bubbles: true }));
      });
    };
    type("ket-qua-so-van-ban", "12/TB-UBND");
    type("ket-qua-ngay-ban-hanh", "2026-10-07");
    type("ket-qua-nguoi-ky", "Chủ tịch UBND xã");
    type("ket-qua-co-quan", "UBND xã");
    type("ket-qua-tom-tat", "Đã nạo vét mương thoát nước");
    await act(async () => button("Lưu nội dung trả lời")!.click());
    const [url, init] = fetchSpy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/citizen-letters/L1/result");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({
      result_document_no: "12/TB-UBND",
      result_document_date: "2026-10-07",
      result_signer: "Chủ tịch UBND xã",
      result_issuer: "UBND xã",
      result_summary: "Đã nạo vét mương thoát nước",
    });
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
