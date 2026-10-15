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

import type { UploadResult } from "@/lib/api/upload";
import type { petitions_photoOut } from "@/lib/api/schema.gen";

import { INTAKE_DESCRIPTION, INTAKE_DONE_SENTENCE, INTAKE_PHOTOS_SOME_FAILED, INTAKE_TITLE } from "./nhan-phieu";
import { addIntakePhotos, StaffIntakeButton, StaffIntakeForm, StaffIntakeFormView } from "./staff-intake";

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
  content: "",
  address: "",
  reporterName: "",
  reporterPhone: "",
  anonymous: false,
  residentialUnitId: "",
  channel: "",
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

type Upload = (code: string, file: File, contentType: string, key: string) => Promise<UploadResult<petitions_photoOut>>;

const STORED: UploadResult<petitions_photoOut> = {
  ok: true,
  data: { id: "01JP", content_type: "image/jpeg", size_bytes: 3, status: "stored", created_at: "x" } as petitions_photoOut,
};

function mount(
  book: (i: StaffIntakeInput, k: string) => Promise<KetQua<{ code: string }>>,
  onBooked = vi.fn(),
  units: KetQua<identity_danhSachThonToDanPhoRa> = UNITS,
  uploadPhoto: Upload = vi.fn(async () => STORED),
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
        uploadPhoto={uploadPhoto}
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
    expect(html).toContain("Ghi lại lời người dân: sự việc gì, ở đâu, từ khi nào.");
    expect(html).toContain("Đầu ngõ thôn Hà Lam");
    expect(html).toContain("Người dân đề nghị gửi ẩn danh");
    expect(html).toContain("Vào sổ phản ánh");
    expect(html).toContain("Huỷ");
  });

  it("NO `Dân phản ánh lúc` box (bug sheet; prototype FeedbackEntryForm.tsx has none)", () => {
    expect(html).not.toContain("Dân phản ánh lúc");
    expect(html).not.toContain('type="datetime-local"');
    expect(html).not.toContain("nhap-ho-luc");
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

  it("`Tiếp nhận qua kênh` is a SELECT: the tester's four channels, in order, the default selected (bug sheet web-admin-141026)", () => {
    expect(html).toContain("Tiếp nhận qua kênh");
    const select = html.match(/<select id="nhap-ho-kenh"[\s\S]*?<\/select>/)?.[0] ?? "";
    const options = [...select.matchAll(/<option value="([^"]+)"( selected="")?>([^<]+)<\/option>/g)].map((m) => [
      m[1],
      m[3],
      m[2] !== undefined,
    ]);
    expect(options).toEqual([
      ["can-bo-nhap-ho", "Cán bộ, trưởng thôn nhập hộ", true],
      ["tong-dai", "Tổng đài điện thoại", false],
      ["web-xa", "Trang thông tin của xã", false],
      ["zalo-oa", "Zalo OA", false],
    ]);
    // Never the citizen's own channel (server: `StaffMayRecord`).
    expect(select).not.toContain("zalo-mini-app");
    expect(html).not.toContain("Kênh tiếp nhận: Cán bộ, trưởng thôn nhập hộ.");
  });

  it("`Đính ảnh hiện trường` is LIVE: a real image file input, no '?' (backend 7c739880)", () => {
    expect(html).toMatch(/<input id="nhap-ho-anh"[^>]*type="file"[^>]*multiple=""[^>]*accept="image\/\*"/);
    expect(html).not.toMatch(/<input id="nhap-ho-anh"[^>]*disabled=""/);
    expect(html).toContain("Đính ảnh hiện trường");
    expect(html).not.toContain(pendingMarkerLabel("Đính ảnh hiện trường"));
  });

  it("the hamlet select is LIVE (ADR 0088 §1): only units IN USE, the empty choice first, no '?'", () => {
    // Lĩnh vực · Thôn · Kênh.
    expect(html.match(/<select/g)?.length).toBe(3);
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
    type(q<HTMLTextAreaElement>(h, "#nhap-ho-noi-dung"), "Rác tồn đọng ở đầu ngõ.");
    type(q<HTMLInputElement>(h, "#nhap-ho-so-dien-thoai"), "0900000000");
  }

  it("sends the typed values, NO clock (the server counts from booking), and ONE key; success shows the LOOKUP CODE and the sentence", async () => {
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
      // No `Dân phản ánh lúc` on the form: `""`, which `staffIntakeBody` leaves out of the body.
      clockFrom: "",
      // No hamlet picked: `""`, which `staffIntakeBody` leaves out of the body.
      residentialUnitId: "",
      // The channel select left on its default: `""`, which `staffIntakeBody` leaves out of the body.
      channel: "",
    });
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(onBooked).toHaveBeenCalledTimes(1);
    // The code, large, and what to do with it.
    expect(h.textContent).toContain("PA-2026-0042");
    expect(h.textContent).toContain(INTAKE_DONE_SENTENCE);
    expect(h.querySelector('[role="status"]')?.textContent).toContain("PA-2026-0042");
  });

  it("a 400 refusal: the server's sentence; the retry REUSES the key; success renews it", async () => {
    const cau = "Lĩnh vực này xã chưa bật. Hãy chọn lĩnh vực khác.";
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
});

async function settle() {
  for (let i = 0; i < 10; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function image(name: string): File {
  return new File([new Uint8Array([0xff, 0xd8, 0xff])], name, { type: "image/jpeg" });
}

/** Choose files on the hidden input the way the browser does: `files` set, then `change`. */
function pick(h: HTMLElement, files: File[]) {
  const input = q<HTMLInputElement>(h, "#nhap-ho-anh");
  Object.defineProperty(input, "files", { value: files, configurable: true });
  act(() => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function fillMinimal(h: HTMLElement) {
  type(q<HTMLSelectElement>(h, "#nhap-ho-linh-vuc"), "rac-thai");
  type(q<HTMLTextAreaElement>(h, "#nhap-ho-noi-dung"), "Rác tồn đọng ở đầu ngõ.");
}

function booked(code: string) {
  return vi.fn(async (_i: StaffIntakeInput, _k: string) => ({ ok: true as const, duLieu: { code } }));
}

describe("Tiếp nhận qua kênh — what the select sends", () => {
  it("a picked channel is sent as `channel`; `Tổng đài điện thoại` is on screen", async () => {
    const book = booked("PA-1");
    const h = mount(book);
    await flush();
    expect(q<HTMLSelectElement>(h, "#nhap-ho-kenh").value).toBe("can-bo-nhap-ho");
    expect(h.textContent).toContain("Tổng đài điện thoại");
    fillMinimal(h);
    type(q<HTMLSelectElement>(h, "#nhap-ho-kenh"), "tong-dai");
    submit(h);
    await flush();
    expect(book.mock.calls[0]?.[0].channel).toBe("tong-dai");
  });

  it("left on the default: `channel` is empty (no key in the body; the server applies can-bo-nhap-ho)", async () => {
    const book = booked("PA-1");
    const h = mount(book);
    await flush();
    fillMinimal(h);
    submit(h);
    await flush();
    expect(book.mock.calls[0]?.[0].channel).toBe("");
  });

  it("picked back to the default: sent explicitly as `can-bo-nhap-ho` (the officer chose it)", async () => {
    const book = booked("PA-1");
    const h = mount(book);
    await flush();
    fillMinimal(h);
    type(q<HTMLSelectElement>(h, "#nhap-ho-kenh"), "web-xa");
    type(q<HTMLSelectElement>(h, "#nhap-ho-kenh"), "can-bo-nhap-ho");
    submit(h);
    await flush();
    expect(book.mock.calls[0]?.[0].channel).toBe("can-bo-nhap-ho");
  });
});

describe("Đính ảnh hiện trường — chosen on the form, uploaded AFTER the booking", () => {
  it("at most five: the extra ones are left out and COUNTED, never dropped silently", () => {
    let n = 0;
    const mint = (file: File) => ({ key: `k${++n}`, file, preview: null });
    const four = addIntakePhotos([], [1, 2, 3, 4].map((i) => image(`${i}.jpg`)), mint);
    expect(four.dropped).toBe(0);
    const next = addIntakePhotos(four.photos, [image("5.jpg"), image("6.jpg"), image("7.jpg")], mint);
    expect(next.photos).toHaveLength(5);
    expect(next.dropped).toBe(2);
  });

  it("nothing is uploaded before the 201; then each photo, IN ORDER, to the new code, each with its OWN key", async () => {
    const order: string[] = [];
    const book = vi.fn(async (_i: StaffIntakeInput, _k: string) => {
      order.push("book");
      return { ok: true as const, duLieu: { code: "PA-2026-0050" } };
    });
    const upload = vi.fn<Upload>(async (code, file) => {
      order.push(`upload ${code} ${file.name}`);
      return STORED;
    });
    const h = mount(book, vi.fn(), UNITS, upload);
    await flush();
    fillMinimal(h);
    pick(h, [image("a.jpg"), image("b.jpg")]);
    expect(upload).not.toHaveBeenCalled();
    // The chosen photos show before sending, each with its ×.
    expect(h.querySelectorAll('ul[aria-label="Ảnh hiện trường đã chọn"] li')).toHaveLength(2);
    submit(h);
    await settle();

    expect(order).toEqual(["book", "upload PA-2026-0050 a.jpg", "upload PA-2026-0050 b.jpg"]);
    const keys = upload.mock.calls.map((c) => c[3]);
    expect(keys[0]).toMatch(/^[0-9a-f-]{36}$/);
    expect(new Set(keys).size).toBe(2);
    expect(keys).not.toContain(book.mock.calls[0]?.[1]);
    expect(upload.mock.calls[0]?.[2]).toBe("image/jpeg");
    expect(h.textContent).toContain("PA-2026-0050");
    expect(h.textContent).toContain("Đã đính vào phiếu.");
  });

  it("a removed photo is not uploaded", async () => {
    const upload = vi.fn<Upload>(async () => STORED);
    const h = mount(booked("PA-1"), vi.fn(), UNITS, upload);
    await flush();
    fillMinimal(h);
    pick(h, [image("a.jpg"), image("b.jpg")]);
    act(() => q<HTMLButtonElement>(h, 'button[aria-label="Bỏ ảnh đã chọn 1/2"]').click());
    submit(h);
    await settle();
    expect(upload).toHaveBeenCalledTimes(1);
    expect(upload.mock.calls[0]?.[1].name).toBe("b.jpg");
  });

  it("a photo that FAILS leaves the booked code on screen, names the photo, and offers Gửi lại (new key) for a refusal of the moment", async () => {
    const cau = "Chưa quét được mã độc. Vui lòng thử lại sau ít phút.";
    const upload = vi
      .fn<Upload>()
      .mockResolvedValueOnce(STORED)
      .mockResolvedValueOnce({ ok: false, status: 503, code: "malware_scan_unavailable", message: cau })
      .mockResolvedValueOnce({ ok: false, status: 415, code: "photo_rejected", message: "Không phải ảnh." })
      .mockResolvedValueOnce(STORED);
    const onBooked = vi.fn();
    const h = mount(booked("PA-2026-0051"), onBooked, UNITS, upload);
    await flush();
    fillMinimal(h);
    pick(h, [image("a.jpg"), image("b.jpg"), image("c.jpg")]);
    submit(h);
    await settle();

    expect(onBooked).toHaveBeenCalledTimes(1);
    expect(h.querySelector('[role="status"]')?.textContent).toContain("PA-2026-0051");
    expect(h.textContent).toContain(INTAKE_PHOTOS_SOME_FAILED);
    const alerts = [...h.querySelectorAll('[role="alert"]')].map((e) => e.textContent);
    expect(alerts).toEqual([`Chưa gửi được: ${cau}`, "Bị từ chối: Không phải ảnh."]);
    // Gửi lại only where nothing was stored and the cause is the moment (503) — not on the 415.
    const retry = [...h.querySelectorAll("button")].filter((b) => b.textContent === "Gửi lại");
    expect(retry).toHaveLength(1);

    act(() => retry[0]?.click());
    await settle();
    expect(upload).toHaveBeenCalledTimes(4);
    expect(upload.mock.calls[3]?.[0]).toBe("PA-2026-0051");
    expect(upload.mock.calls[3]?.[3]).not.toBe(upload.mock.calls[1]?.[3]);
    expect(h.textContent).toContain("PA-2026-0051");
    expect([...h.querySelectorAll('[role="alert"]')].map((e) => e.textContent)).toEqual(["Bị từ chối: Không phải ảnh."]);
  });

  it("the booking itself refused: no photo is uploaded, the chosen ones stay for the retry", async () => {
    const upload = vi.fn<Upload>(async () => STORED);
    const h = mount(async () => ({ ok: false, thongBao: "Lĩnh vực này xã chưa bật." }), vi.fn(), UNITS, upload);
    await flush();
    fillMinimal(h);
    pick(h, [image("a.jpg")]);
    submit(h);
    await settle();
    expect(upload).not.toHaveBeenCalled();
    expect(h.querySelectorAll('ul[aria-label="Ảnh hiện trường đã chọn"] li')).toHaveLength(1);
  });
});
