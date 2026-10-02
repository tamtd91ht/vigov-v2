// @vitest-environment jsdom
//
// jsdom for this file: the intake is a FLOW — choose a field, type, press "Vào sổ phản ánh", read the
// code. What it emits (the body, the idempotency key across a retry) is only visible by pressing.

import { act } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { StaffIntakeInput } from "@/lib/api/phieu-phan-anh";
import type { petitions_citizenFieldListOut } from "@/lib/api/schema.gen";

import { INTAKE_DESCRIPTION, INTAKE_DONE_SENTENCE, INTAKE_TITLE } from "./nhan-phieu";
import { StaffIntakeButton, StaffIntakeForm, StaffIntakeFormView } from "./staff-intake";

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
});

const FIELDS: KetQua<petitions_citizenFieldListOut> = {
  ok: true,
  duLieu: {
    items: [
      { code: "rac-thai", label: "Rác thải – Vệ sinh môi trường", icon: null, tone: null },
      { code: "can-bo", label: "Thái độ / tác phong cán bộ", icon: null, tone: null },
    ],
  },
};

const EMPTY_VALUES = {
  field: "",
  clockLocal: "",
  content: "",
  address: "",
  reporterName: "",
  reporterPhone: "",
  anonymous: false,
};

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

/** Set a controlled input's value the way a user does (React listens to the native setter + event). */
function type(el: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement, value: string) {
  const proto = Object.getPrototypeOf(el) as object;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
  act(() => {
    setter?.call(el, value);
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

function mount(book: (i: StaffIntakeInput, k: string) => Promise<KetQua<{ code: string }>>, onBooked = vi.fn()) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<StaffIntakeForm onClose={() => {}} onBooked={onBooked} loadFields={async () => FIELDS} book={book} />));
  return host;
}

function q<T extends Element>(h: HTMLElement, sel: string): T {
  const el = h.querySelector(sel);
  if (el === null) throw new Error(`no ${sel}`);
  return el as T;
}

function submit(h: HTMLElement) {
  act(() => {
    q<HTMLFormElement>(h, "form").dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
}

describe("the modal's shape (§11) — what is on it and what is NOT", () => {
  const html = renderToStaticMarkup(<StaffIntakeFormView fields={FIELDS} values={EMPTY_VALUES} />);

  it("title, the approved description, the fields of §11", () => {
    expect(html).toContain(INTAKE_TITLE);
    expect(html).toContain(INTAKE_DESCRIPTION.replace(/&/g, "&amp;"));
    expect(html).toContain("— Chọn lĩnh vực —");
    expect(html).toContain("Dân phản ánh lúc");
    expect(html).toContain('type="datetime-local"');
    expect(html).toContain("Ghi lại lời người dân: sự việc gì, ở đâu, từ khi nào.");
    expect(html).toContain("Đầu ngõ thôn Hà Lam");
    expect(html).toContain("Người dân đề nghị gửi ẩn danh");
    expect(html).toContain("Vào sổ phản ánh");
    expect(html).toContain("Huỷ");
  });

  it("the field options are the COMMUNE'S list from the route, `can-bo` included", () => {
    expect(html).toContain('<option value="rac-thai">Rác thải – Vệ sinh môi trường</option>');
    expect(html).toContain('<option value="can-bo">Thái độ / tác phong cán bộ</option>');
  });

  it("NO channel select, NO hamlet, NO photo input (ADR 0028 Bổ sung 2026-10-02 row 5; server refuses)", () => {
    expect(html).not.toContain("Tiếp nhận qua kênh");
    expect(html).not.toContain("Thôn, tổ dân phố");
    expect(html).not.toContain('type="file"');
    expect(html.match(/<select/g)?.length).toBe(1);
  });

  it("503 on the field list: the server's sentence and Tải lại, the select disabled", () => {
    const cau = "Chưa đọc được danh mục lĩnh vực. Vui lòng thử lại sau ít phút.";
    const h = renderToStaticMarkup(
      <StaffIntakeFormView fields={{ ok: false, thongBao: cau }} values={EMPTY_VALUES} onReloadFields={() => {}} />,
    );
    expect(h).toContain(cau);
    expect(h).toContain("Tải lại");
    expect(h).toMatch(/<select[^>]*disabled=""/);
  });

  it("the submit is disabled until a field and the content are given", () => {
    // The class carries Tailwind's `disabled:` variants, so it is stripped before looking for the attribute.
    const tag = (h: string) => (h.match(/<button[^>]*type="submit"[^>]*>/)?.[0] ?? "").replace(/\sclass="[^"]*"/, "");
    expect(tag(html)).toContain("disabled");
    const ok = renderToStaticMarkup(
      <StaffIntakeFormView fields={FIELDS} values={{ ...EMPTY_VALUES, field: "rac-thai", content: "Rác." }} />,
    );
    expect(tag(ok)).not.toContain("disabled");
  });
});

describe("the button — UX only, `feedback.create`", () => {
  it("DENIED: no `feedback.create` (or an unknown session) — no button", () => {
    for (const p of [[], ["feedback.read", "feedback.resolve"]]) {
      expect(renderToStaticMarkup(<StaffIntakeButton permissions={p} onBooked={() => {}} />)).toBe("");
    }
  });

  it("with `feedback.create`: the button", () => {
    expect(renderToStaticMarkup(<StaffIntakeButton permissions={["feedback.create"]} onBooked={() => {}} />)).toContain(
      "Nhập hộ phản ánh",
    );
  });
});

describe("booking — the flow", () => {
  function fill(h: HTMLElement) {
    type(q<HTMLSelectElement>(h, "#nhap-ho-linh-vuc"), "rac-thai");
    type(q<HTMLInputElement>(h, "#nhap-ho-luc"), "2026-10-02T08:30");
    type(q<HTMLTextAreaElement>(h, "#nhap-ho-noi-dung"), "Rác tồn đọng ở đầu ngõ.");
    type(q<HTMLInputElement>(h, "#nhap-ho-so-dien-thoai"), "0900000000");
  }

  it("sends the typed values, the clock in +07:00, and ONE key; success shows the LOOKUP CODE and the sentence", async () => {
    const book = vi.fn(async (_i: StaffIntakeInput, _k: string) => ({ ok: true as const, duLieu: { code: "PA-2026-0042" } }));
    const onBooked = vi.fn();
    const h = mount(book, onBooked);
    await flush();
    fill(h);
    submit(h);
    await flush();

    expect(book).toHaveBeenCalledTimes(1);
    const [input, key] = book.mock.calls[0] as [StaffIntakeInput, string];
    expect(input).toEqual({
      field: "rac-thai",
      content: "Rác tồn đọng ở đầu ngõ.",
      address: "",
      reporterName: "",
      reporterPhone: "0900000000",
      anonymous: false,
      clockFrom: "2026-10-02T08:30:00+07:00",
    });
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(onBooked).toHaveBeenCalledTimes(1);
    // The code, large, and what to do with it.
    expect(h.textContent).toContain("PA-2026-0042");
    expect(h.textContent).toContain(INTAKE_DONE_SENTENCE);
    expect(h.querySelector('[role="status"]')?.textContent).toContain("PA-2026-0042");
  });

  it("400 `clock_from_out_of_range`: the server's sentence; the retry REUSES the key; success renews it", async () => {
    const cau =
      "Thời điểm người dân phản ánh không được sớm hơn 7 ngày trước lúc vào sổ, và không được muộn hơn lúc vào sổ. Hãy kiểm tra lại ô \"Dân phản ánh lúc\".";
    const book = vi
      .fn<(i: StaffIntakeInput, k: string) => Promise<KetQua<{ code: string }>>>()
      .mockResolvedValueOnce({ ok: false, thongBao: cau })
      .mockResolvedValueOnce({ ok: true, duLieu: { code: "PA-2026-0043" } });
    const h = mount(book);
    await flush();
    fill(h);
    submit(h);
    await flush();
    expect(q(h, '[role="alert"]').textContent).toBe(cau);
    // The form keeps what was typed.
    expect(q<HTMLTextAreaElement>(h, "#nhap-ho-noi-dung").value).toBe("Rác tồn đọng ở đầu ngõ.");

    submit(h);
    await flush();
    expect(book.mock.calls[1]?.[1]).toBe(book.mock.calls[0]?.[1]);
    expect(h.textContent).toContain("PA-2026-0043");
  });

  it.each([
    ["Chưa ấn định được thời hạn xử lý theo cấu hình của xã nên phiếu CHƯA được vào sổ. Hãy kiểm tra bảng thời hạn xử lý và lịch làm việc ở màn hình Cấu hình."],
    ["Chưa kiểm tra được lĩnh vực nên phiếu CHƯA được vào sổ. Vui lòng thử lại sau ít phút."],
  ])("503: “%s” reaches the officer verbatim, no code shown", async (cau) => {
    const h = mount(async () => ({ ok: false, thongBao: cau }));
    await flush();
    fill(h);
    submit(h);
    await flush();
    expect(q(h, '[role="alert"]').textContent).toBe(cau);
    expect(h.textContent).not.toContain("Mã tra cứu");
  });

  it("a blank clock sends NO `clock_from` — the server takes the booking instant", async () => {
    const book = vi.fn(async (_i: StaffIntakeInput, _k: string) => ({ ok: true as const, duLieu: { code: "x" } }));
    const h = mount(book);
    await flush();
    fill(h);
    type(q<HTMLInputElement>(h, "#nhap-ho-luc"), "");
    submit(h);
    await flush();
    expect(book.mock.calls[0]?.[0].clockFrom).toBe("");
  });
});
