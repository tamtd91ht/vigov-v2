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
import type { identity_danhSachThonToDanPhoRa, petitions_citizenFieldListOut } from "@/lib/api/schema.gen";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { INTAKE_DESCRIPTION, INTAKE_DONE_SENTENCE, INTAKE_TITLE, petitionPendingPart } from "./nhan-phieu";
import { StaffIntakeButton, StaffIntakeForm, StaffIntakeFormView } from "./staff-intake";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // Radix measures the "?" to place its description; jsdom has no ResizeObserver.
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
  residentialUnitId: "",
};

function unit(id: string, name: string, active: boolean) {
  return {
    id,
    code: id.toLowerCase(),
    name,
    type_code: "thon",
    type_label: "Thôn",
    household_count: null,
    population_count: null,
    active,
    head_staff_code: "",
    head_staff_name: "",
    order: 0,
  };
}

/** Two units in use and one retired: only the two may be offered for a NEW petition (ADR 0088 §1). */
const UNITS: KetQua<identity_danhSachThonToDanPhoRa> = {
  ok: true,
  duLieu: {
    items: [unit("01JTHON1", "Thôn Hà Lam", true), unit("01JTHON2", "Thôn Bình An", true), unit("01JTHON9", "Thôn Cũ", false)],
  },
} as KetQua<identity_danhSachThonToDanPhoRa>;

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

function mount(
  book: (i: StaffIntakeInput, k: string) => Promise<KetQua<{ code: string }>>,
  onBooked = vi.fn(),
  units: KetQua<identity_danhSachThonToDanPhoRa> = UNITS,
) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <StaffIntakeForm
        onClose={() => {}}
        onBooked={onBooked}
        loadFields={async () => FIELDS}
        loadUnits={async () => units}
        book={book}
      />,
    ),
  );
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
  const html = renderToStaticMarkup(<StaffIntakeFormView fields={FIELDS} units={UNITS} values={EMPTY_VALUES} />);

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

  it("prototype marks: required fields carry `*`, not “(bắt buộc)”; content is 3 rows; no reminder line", () => {
    expect(html).not.toContain("(bắt buộc)");
    expect(html.match(/<span class="ml-1 text-danger">\*<\/span>/g)?.length).toBe(2);
    expect(html).toMatch(/<textarea id="nhap-ho-noi-dung"[^>]*rows="3"/);
    expect(html).not.toContain("Không ghi số điện thoại, số CCCD");
    expect(html).toContain('class="accent-brand size-3.5"');
  });

  it("the field options are the COMMUNE'S list from the route, `can-bo` included", () => {
    expect(html).toContain('<option value="rac-thai">Rác thải – Vệ sinh môi trường</option>');
    expect(html).toContain('<option value="can-bo">Thái độ / tác phong cán bộ</option>');
  });

  it("NO channel select; photos are a DISABLED placeholder (ADR 0028 row 5, ADR 0068 §14; server refuses)", () => {
    expect(html).not.toContain("Tiếp nhận qua kênh");
    // No file can be chosen: the photo picker is a disabled button, never a file input.
    expect(html).not.toContain('type="file"');
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*>(?:(?!<\/button>).)*Đính ảnh hiện trường<\/button>/s);
    expect(html).toContain(pendingMarkerLabel("Đính ảnh hiện trường"));
  });

  it("the hamlet select is LIVE (ADR 0088 §1): only units IN USE, the empty choice first, no '?'", () => {
    expect(html.match(/<select/g)?.length).toBe(2);
    expect(html).not.toMatch(/<select id="nhap-ho-thon"[^>]*disabled=""/);
    expect(html).toContain("Thôn, tổ dân phố");
    expect(html).not.toContain(pendingMarkerLabel("Thôn, tổ dân phố"));
    const select = html.match(/<select id="nhap-ho-thon"[\s\S]*?<\/select>/)?.[0] ?? "";
    expect(select).toContain('<option value="" selected="">— Chưa xác định —</option>');
    expect(select).toContain('<option value="01JTHON1">Thôn Hà Lam</option>');
    expect(select).toContain('<option value="01JTHON2">Thôn Bình An</option>');
    // A retired unit is never offered for a new record — the server would refuse it.
    expect(select).not.toContain("01JTHON9");
    expect(select).not.toContain("Thôn Cũ");
  });

  it("the unit list failed: the server's sentence under the select; the petition can still be booked", () => {
    const cau = "Không đọc được danh sách thôn. Vui lòng thử lại.";
    const h = renderToStaticMarkup(
      <StaffIntakeFormView fields={FIELDS} units={{ ok: false, thongBao: cau }} values={{ ...EMPTY_VALUES, field: "rac-thai", content: "Rác." }} />,
    );
    expect(h).toContain(cau);
    const tag = (x: string) => (x.match(/<button[^>]*type="submit"[^>]*>/)?.[0] ?? "").replace(/\sclass="[^"]*"/, "");
    expect(tag(h)).not.toContain("disabled");
  });

  it("pressing the photos '?' opens its description and reaches no network; the body gains no key", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const h = mount(vi.fn());
    for (const id of ["intakePhotos"]) {
      const info = petitionPendingPart(id);
      const b = q<HTMLButtonElement>(h, `button[aria-label="${pendingMarkerLabel(info.ten)}"]`);
      act(() => b.click());
      expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(info.viSao);
      act(() => b.click());
    }
    expect(fetchSpy).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
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
      <StaffIntakeFormView fields={FIELDS} units={UNITS} values={{ ...EMPTY_VALUES, field: "rac-thai", content: "Rác." }} />,
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
      // No hamlet picked: `""`, which `staffIntakeBody` leaves out of the body.
      residentialUnitId: "",
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

  it("anonymous: name and phone are DISABLED, and whatever was typed is NOT sent (prototype FeedbackEntryForm.tsx:208,216)", async () => {
    const book = vi.fn(async (_i: StaffIntakeInput, _k: string) => ({ ok: true as const, duLieu: { code: "x" } }));
    const h = mount(book);
    await flush();
    fill(h);
    type(q<HTMLInputElement>(h, "#nhap-ho-nguoi-gui"), "Nguyễn Văn A");
    act(() => q<HTMLInputElement>(h, "#nhap-ho-an-danh").click());
    expect(q<HTMLInputElement>(h, "#nhap-ho-nguoi-gui").disabled).toBe(true);
    expect(q<HTMLInputElement>(h, "#nhap-ho-so-dien-thoai").disabled).toBe(true);
    submit(h);
    await flush();
    const input = book.mock.calls[0]?.[0];
    expect(input?.anonymous).toBe(true);
    expect(input?.reporterName).toBe("");
    expect(input?.reporterPhone).toBe("");
  });

  it("a picked hamlet is sent as its id; 400 `residential_unit_not_offered` is the server's sentence, inline", async () => {
    const cau = "Thôn, tổ dân phố đã chọn không thuộc xã hoặc đã ngừng dùng. Hãy chọn lại.";
    const book = vi
      .fn<(i: StaffIntakeInput, k: string) => Promise<KetQua<{ code: string }>>>()
      .mockResolvedValueOnce({ ok: false, thongBao: cau });
    const h = mount(book);
    await flush();
    fill(h);
    type(q<HTMLSelectElement>(h, "#nhap-ho-thon"), "01JTHON2");
    submit(h);
    await flush();
    expect(book.mock.calls[0]?.[0].residentialUnitId).toBe("01JTHON2");
    expect(q(h, '[role="alert"]').textContent).toBe(cau);
    // The choice stays for a correction.
    expect(q<HTMLSelectElement>(h, "#nhap-ho-thon").value).toBe("01JTHON2");
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
