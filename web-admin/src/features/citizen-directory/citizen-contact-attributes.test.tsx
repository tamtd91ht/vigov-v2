// @vitest-environment jsdom
//
// The Add / Edit dialog with the commune's dynamic attributes (ADR 0101 #1–#4, #6): one control per type in
// the configured order, placeholders and required markers, the conversion of what is typed into what is
// sent (dateTime → epoch millis, decimal comma → number), the local refusal of a wrong value, the static
// Email (masked on edit, never sent back), and the values of retired fields carried over.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "@/lib/api/citizen-contacts";
import type { CitizenContact, CitizenContactDetail } from "@/lib/api/citizen-contacts";
import type { CitizenField } from "@/lib/api/citizen-fields";

import { attributeInputId } from "./citizen-attribute-inputs";
import { DECIMAL_INVALID, REQUIRED_TEXT } from "./citizen-attributes";
import { CitizenContactDialog } from "./citizen-contact-dialog";
import { EDIT_EMAIL_BUTTON, EMAIL_INVALID, SUBMIT_ADD, SUBMIT_EDIT } from "./citizen-directory-words";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/citizen-contacts", async (original) => ({
  ...(await original<typeof import("@/lib/api/citizen-contacts")>()),
  getCitizenContact: vi.fn(),
  revealCitizenContactPhone: vi.fn(),
  updateCitizenContact: vi.fn(),
  createCitizenContact: vi.fn(),
}));

const create = vi.mocked(api.createCitizenContact);
const update = vi.mocked(api.updateCitizenContact);
const detailRead = vi.mocked(api.getCitizenContact);
const reveal = vi.mocked(api.revealCitizenContactPhone);

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const base = (over: Partial<CitizenField>): CitizenField => ({
  code: "abcdef",
  label: "X",
  field_type: "singleText",
  placeholder: "",
  options: [],
  number_kind: "",
  date_pattern: "",
  required: false,
  show_in_list: false,
  sort_order: 0,
  created_at: "2026-10-17T00:00:00Z",
  updated_at: "2026-10-17T00:00:00Z",
  retired_at: null,
  display_mode: "",
  layout_width: "half",
  ...over,
});

/** The live fields, in the configured order. */
const FIELDS: CitizenField[] = [
  base({ code: "nghenghi", label: "Nghề nghiệp", placeholder: "Ví dụ: nông dân", required: true }),
  base({ code: "ghichu02", label: "Hoàn cảnh", field_type: "multiText", placeholder: "Mô tả ngắn" }),
  base({
    code: "nhucau01",
    label: "Nhu cầu hỗ trợ",
    field_type: "combobox",
    options: [
      { value: "dien", label: "Điện", retired: false },
      { value: "nuoc", label: "Nước", retired: false },
      { value: "cu-1", label: "Cũ", retired: true },
    ],
  }),
  base({
    code: "hongheo1",
    label: "Phân loại hộ",
    field_type: "singleBox",
    options: [
      { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
      { value: "can-ngheo", label: "Cận nghèo", retired: false },
    ],
  }),
  base({ code: "dientich", label: "Diện tích (m²)", field_type: "number", number_kind: "decimal" }),
  base({ code: "ngaysinh", label: "Ngày sinh chủ hộ", field_type: "dateTime", date_pattern: "dd/MM/yyyy" }),
  base({ code: "henluc01", label: "Hẹn lúc", field_type: "dateTime", date_pattern: "dd/MM/yyyy HH:mm" }),
];

/** 17/10/2026 00:00 in Vietnam. */
const OCT17_VN = Date.UTC(2026, 9, 16, 17, 0);

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  create.mockResolvedValue({ ok: true, duLieu: { id: "01JNEW" } });
  update.mockResolvedValue({ ok: true, duLieu: null });
  reveal.mockResolvedValue({ ok: true, duLieu: { phone: "0900000001" } });
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.clearAllMocks();
});

const tick = () => act(async () => {});

async function mount(node: React.ReactNode): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => root!.render(node));
  await tick();
  return host;
}

function typeInto(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
const control = (el: HTMLElement, code: string) => el.querySelector<HTMLInputElement>(`#${attributeInputId(code)}`)!;
const choice = (el: HTMLElement, code: string, value: string) =>
  el.querySelector<HTMLInputElement>(`[data-attribute="${code}"] input[value="${value}"]`)!;
const buttonNamed = (el: HTMLElement, text: string) => [...el.querySelectorAll("button")].find((b) => b.textContent === text)!;

async function fillStatics(el: HTMLElement) {
  typeInto(el.querySelector<HTMLInputElement>("#nguoi-dan-ho-ten")!, "Nguyễn Văn A");
  typeInto(el.querySelector<HTMLInputElement>("#nguoi-dan-so-dien-thoai")!, "0900000001");
}

describe("add dialog — dynamic controls", () => {
  it("draws each type's control after the static fields, in order, with placeholders and required markers", async () => {
    const el = await mount(<CitizenContactDialog mode={{ kind: "add", idempotencyKey: "k" }} groups={[]} units={[]} fields={FIELDS} canReveal onClose={() => {}} onSaved={() => {}} />);
    expect(control(el, "nghenghi").tagName).toBe("INPUT");
    expect(control(el, "nghenghi").placeholder).toBe("Ví dụ: nông dân");
    expect(el.querySelector(`label[for="${attributeInputId("nghenghi")}"]`)!.textContent).toBe("Nghề nghiệp*");
    expect(control(el, "ghichu02").tagName).toBe("TEXTAREA");
    expect(control(el, "dientich").inputMode).toBe("decimal");
    expect(control(el, "ngaysinh").type).toBe("date");
    expect(control(el, "henluc01").type).toBe("datetime-local");
    expect(choice(el, "nhucau01", "dien").type).toBe("checkbox");
    expect(choice(el, "hongheo1", "ho-ngheo").type).toBe("radio");
    // A retired option is not offered to a new person.
    expect(choice(el, "nhucau01", "cu-1")).toBeNull();
    // Order: the dynamic controls come after Ghi chú, in sort order.
    const ids = [...el.querySelectorAll("input, textarea")].map((n) => n.id).filter((id) => id !== "");
    expect(ids.indexOf("nguoi-dan-ghi-chu")).toBeLessThan(ids.indexOf(attributeInputId("nghenghi")));
    expect(ids.indexOf(attributeInputId("nghenghi"))).toBeLessThan(ids.indexOf(attributeInputId("ngaysinh")));
  });

  it("refuses a missing required value, a bad decimal and a bad email BEFORE sending", async () => {
    const el = await mount(<CitizenContactDialog mode={{ kind: "add", idempotencyKey: "k" }} groups={[]} units={[]} fields={FIELDS} canReveal onClose={() => {}} onSaved={() => {}} />);
    await fillStatics(el);
    typeInto(control(el, "dientich"), "12,5,1");
    typeInto(el.querySelector<HTMLInputElement>("#nguoi-dan-email")!, "khong-phai-email");
    await act(async () => buttonNamed(el, SUBMIT_ADD).click());
    expect(create).not.toHaveBeenCalled();
    const alerts = [...el.querySelectorAll('[role="alert"]')].map((a) => a.textContent);
    expect(alerts).toContain(REQUIRED_TEXT);
    expect(alerts).toContain(DECIMAL_INVALID);
    expect(alerts).toContain(EMAIL_INVALID);
  });

  it("sends what was typed converted: date → epoch millis (+07:00), decimal comma → number, choices → values, email", async () => {
    const el = await mount(<CitizenContactDialog mode={{ kind: "add", idempotencyKey: "k" }} groups={[]} units={[]} fields={FIELDS} canReveal onClose={() => {}} onSaved={() => {}} />);
    await fillStatics(el);
    typeInto(el.querySelector<HTMLInputElement>("#nguoi-dan-email")!, "nguoidan@example.com");
    typeInto(control(el, "nghenghi"), "Nông dân");
    typeInto(control(el, "dientich"), "12,5");
    typeInto(control(el, "ngaysinh"), "2026-10-17");
    typeInto(control(el, "henluc01"), "2026-10-17T08:30");
    act(() => choice(el, "nhucau01", "nuoc").click());
    act(() => choice(el, "nhucau01", "dien").click());
    act(() => choice(el, "hongheo1", "can-ngheo").click());
    await act(async () => buttonNamed(el, SUBMIT_ADD).click());
    expect(create).toHaveBeenCalledTimes(1);
    const body = create.mock.calls[0]![0];
    expect(body.email).toBe("nguoidan@example.com");
    expect(body.attributes).toEqual({
      nghenghi: "Nông dân",
      nhucau01: ["dien", "nuoc"],
      hongheo1: "can-ngheo",
      dientich: 12.5,
      ngaysinh: OCT17_VN,
      henluc01: OCT17_VN + 8.5 * 3600_000,
    });
  });
});

describe("edit dialog — stored values, email mask, retired values", () => {
  const ROW: CitizenContact = {
    id: "01JCONTACT",
    full_name: "Nguyễn Văn A",
    phone_masked: "09****0001",
    residential_unit_id: "",
    residential_unit_name: "",
    tags: [],
    group_ids: [],
    is_active: true,
    source: "nhap-tay",
    has_own_app_session: false,
    created_at: "2026-10-16T01:00:00Z",
    attributes: {},
  };
  const DETAIL: CitizenContactDetail = {
    id: ROW.id,
    full_name: ROW.full_name,
    phone_masked: ROW.phone_masked,
    residential_unit_id: "",
    address: "",
    note: "",
    tags: [],
    group_ids: [],
    is_active: true,
    source: "nhap-tay",
    import_batch_id: "",
    created_at: ROW.created_at,
    updated_at: ROW.created_at,
    email_masked: "ng***@example.com",
    attributes: { nghenghi: "Nông dân", nhucau01: ["cu-1"], ngaysinh: OCT17_VN, cucu0001: "của thuộc tính đã ngừng" },
  };

  async function mountEdit(): Promise<HTMLDivElement> {
    detailRead.mockResolvedValue({ ok: true, duLieu: DETAIL });
    const el = await mount(<CitizenContactDialog mode={{ kind: "edit", contact: ROW }} groups={[]} units={[]} fields={FIELDS} canReveal onClose={() => {}} onSaved={() => {}} />);
    await act(async () => el.querySelector<HTMLButtonElement>("#xem-so-day-du")!.click());
    return el;
  }

  it("fills the controls from the stored values; a retired option the person holds stays ticked and labelled", async () => {
    const el = await mountEdit();
    expect(control(el, "nghenghi").value).toBe("Nông dân");
    expect(control(el, "ngaysinh").value).toBe("2026-10-17");
    expect(choice(el, "nhucau01", "cu-1").checked).toBe(true);
    expect(el.querySelector('[data-attribute="nhucau01"]')!.textContent).toContain("(ngừng dùng)");
  });

  it("the email is shown MASKED and read-only; saving without 'Nhập email mới' sends `email: null` (keep); retired fields' values are kept", async () => {
    const el = await mountEdit();
    const email = el.querySelector<HTMLInputElement>("#nguoi-dan-email")!;
    expect(email.value).toBe("ng***@example.com");
    expect(email.readOnly).toBe(true);
    await act(async () => buttonNamed(el, SUBMIT_EDIT).click());
    const body = update.mock.calls[0]![1];
    expect(body.email).toBeNull();
    expect(body.attributes).toEqual({ cucu0001: "của thuộc tính đã ngừng", nghenghi: "Nông dân", nhucau01: ["cu-1"], ngaysinh: OCT17_VN });
  });

  it("'Nhập email mới' opens an empty box; what is typed is sent", async () => {
    const el = await mountEdit();
    await act(async () => buttonNamed(el, EDIT_EMAIL_BUTTON).click());
    const email = el.querySelector<HTMLInputElement>("#nguoi-dan-email")!;
    expect(email.value).toBe("");
    typeInto(email, "moi@example.com");
    await act(async () => buttonNamed(el, SUBMIT_EDIT).click());
    expect(update.mock.calls[0]![1].email).toBe("moi@example.com");
  });
});
