// @vitest-environment jsdom
//
// jsdom: the dialog's "Xem số đầy đủ" is a click that awaits a read and swaps the field in place —
// an event and a state change, which a static render cannot show.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "@/lib/api/citizen-contacts";
import type { CitizenContact, CitizenContactDetail } from "@/lib/api/citizen-contacts";

import { CitizenContactDialog, REVEAL_PHONE_BUTTON_ID } from "./citizen-contact-dialog";
import { REVEAL_BEFORE_SAVE_HINT, REVEAL_PHONE_FAILED, SUBMIT_EDIT, TAGS_WARNING } from "./citizen-directory-words";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/citizen-contacts", async (original) => ({
  ...(await original<typeof import("@/lib/api/citizen-contacts")>()),
  getCitizenContact: vi.fn(),
  revealCitizenContactPhone: vi.fn(),
  updateCitizenContact: vi.fn(),
  createCitizenContact: vi.fn(),
}));

const detailRead = vi.mocked(api.getCitizenContact);
const reveal = vi.mocked(api.revealCitizenContactPhone);
const update = vi.mocked(api.updateCitizenContact);

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

/** Fake data only (rule 3, invariant 5): the agreed `0900000xxx` numbers, masked as the server masks them. */
const ROW: CitizenContact = {
  id: "01JCONTACT",
  full_name: "Nguyễn Văn A",
  phone_masked: "09****0001",
  residential_unit_id: "",
  residential_unit_name: "",
  tags: ["tổ trưởng"],
  group_ids: [],
  is_active: true,
  source: "nhap-tay",
  has_own_app_session: false,
  created_at: "2026-10-16T01:00:00Z",
};
const DETAIL: CitizenContactDetail = {
  id: ROW.id,
  full_name: ROW.full_name,
  phone_masked: ROW.phone_masked,
  residential_unit_id: "",
  address: "Thôn 1",
  note: "",
  tags: ROW.tags,
  group_ids: [],
  is_active: true,
  source: "nhap-tay",
  import_batch_id: "",
  created_at: ROW.created_at,
  updated_at: ROW.created_at,
};

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  detailRead.mockResolvedValue({ ok: true, duLieu: DETAIL });
  reveal.mockResolvedValue({ ok: true, duLieu: { phone: "0900000001" } });
  update.mockResolvedValue({ ok: true, duLieu: null });
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.clearAllMocks();
});

const tick = () => act(async () => {});

async function mountEdit(canReveal: boolean): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<CitizenContactDialog mode={{ kind: "edit", contact: ROW }} groups={[]} units={[]} canReveal={canReveal} onClose={() => {}} onSaved={() => {}} />);
  });
  await tick();
  return host;
}

const revealButton = (el: HTMLElement) => el.querySelector<HTMLButtonElement>(`#${REVEAL_PHONE_BUTTON_ID}`);
const phoneInput = (el: HTMLElement) => el.querySelector<HTMLInputElement>("#nguoi-dan-so-dien-thoai")!;
const saveButton = (el: HTMLElement) => [...el.querySelectorAll("button")].find((b) => b.textContent === SUBMIT_EDIT)!;

describe("edit dialog — Xem số đầy đủ (ADR 0097 D2)", () => {
  it("opens MASKED: the detail is read, the number is not; the save button is held with the hint", async () => {
    const el = await mountEdit(true);
    expect(detailRead).toHaveBeenCalledWith(ROW.id);
    expect(reveal).not.toHaveBeenCalled(); // never on render: every call is an audited read
    expect(phoneInput(el).value).toBe("09****0001");
    expect(phoneInput(el).readOnly).toBe(true);
    expect(el.textContent).not.toContain("0900000001");
    expect(saveButton(el).disabled).toBe(true);
    expect(el.textContent).toContain(REVEAL_BEFORE_SAVE_HINT);
    expect(el.textContent).toContain(TAGS_WARNING);
  });

  it("DENIED — no citizen.update: no reveal button at all, the mask stays", async () => {
    const el = await mountEdit(false);
    expect(revealButton(el)).toBeNull();
    expect(phoneInput(el).value).toBe("09****0001");
    expect(reveal).not.toHaveBeenCalled();
  });

  it("one click = ONE call; a second click during the read is ignored; the full number then fills an editable field and the button goes", async () => {
    const el = await mountEdit(true);
    let answer: (r: api.CitizenPhoneResult) => void = () => {};
    reveal.mockImplementationOnce(() => new Promise((resolve) => (answer = resolve)));
    await act(async () => revealButton(el)!.click());
    expect(revealButton(el)!.disabled).toBe(true);
    await act(async () => revealButton(el)!.click());
    expect(reveal).toHaveBeenCalledTimes(1);
    expect(reveal).toHaveBeenCalledWith(ROW.id);
    await act(async () => answer({ ok: true, duLieu: { phone: "0900000001" } }));
    expect(revealButton(el)).toBeNull();
    expect(phoneInput(el).value).toBe("0900000001");
    expect(phoneInput(el).readOnly).toBe(false);
    expect(saveButton(el).disabled).toBe(false);
    expect(el.textContent).not.toContain(REVEAL_BEFORE_SAVE_HINT);
  });

  it("a refused reveal keeps the mask, says the chapter's sentence, and lets the person try again", async () => {
    reveal.mockResolvedValueOnce({ ok: false, thongBao: "Không có quyền.", forbidden: true });
    const el = await mountEdit(true);
    await act(async () => revealButton(el)!.click());
    expect(phoneInput(el).value).toBe("09****0001");
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(REVEAL_PHONE_FAILED);
    expect(el.textContent).not.toContain("Không có quyền.");
    expect(revealButton(el)).not.toBeNull();
    expect(saveButton(el).disabled).toBe(true);
  });

  it("saving after the reveal sends the FULL number and the whole form to PATCH", async () => {
    const el = await mountEdit(true);
    await act(async () => revealButton(el)!.click());
    await act(async () => saveButton(el).click());
    expect(update).toHaveBeenCalledTimes(1);
    expect(update.mock.calls[0]?.[0]).toBe(ROW.id);
    expect(update.mock.calls[0]?.[1]).toEqual({
      full_name: "Nguyễn Văn A",
      phone: "0900000001",
      residential_unit_id: "",
      address: "Thôn 1",
      note: "",
      tags: ["tổ trưởng"],
      group_ids: [],
      is_active: true,
      attributes: {},
    });
    // The detail answered no email to replace: the body carries none — never a mask.
    expect(update.mock.calls[0]?.[1]).not.toHaveProperty("email");
  });
});
