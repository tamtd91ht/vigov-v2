// @vitest-environment jsdom
//
// jsdom: typing triggers the debounced duplicate check, a candidate is linked, the booking is posted.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { DUPLICATE_FOOTER, LetterEntryDialog, MERGE_INTO_LABEL } from "./letter-entry-dialog";
import { SUMMARY_REQUIRED, emptyEntryDraft } from "./letter-display";

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
  vi.useRealTimers();
});

type Call = { url: string; init: RequestInit };

/** A fake server: duplicates answer two candidates, a booking answers 201. */
function stubServer(): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-letters/duplicates") {
        return new Response(
          JSON.stringify({
            items: [
              { id: "D1", number: 3, year: 2026, received_date: "2026-09-01", similarity: 0.82, summary: "Đường thôn ngập nước" },
              { id: "D2", number: 1, year: 2026, received_date: "2026-08-15", similarity: 0.51, summary: "Mương thoát nước tắc" },
            ],
          }),
          { status: 200 },
        );
      }
      return new Response(JSON.stringify({ id: "NEW", number: 9, year: 2026 }), { status: 201 });
    }),
  );
  return calls;
}

function mount(onBooked = vi.fn(), filled = true) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  const draft = filled
    ? { ...emptyEntryDraft("2026-10-08"), senderName: "Nguyễn Văn A", senderPhone: "0900000000", summary: "Đường thôn bị ngập sau mưa lớn" }
    : emptyEntryDraft("2026-10-08");
  act(() => r.render(<LetterEntryDialog units={{ pha: "xong", ten: new Map() }} onClose={() => {}} onBooked={onBooked} initialDraft={draft} />));
  return onBooked;
}

const form = () => document.querySelector("form")!;
const bodyOf = (c: Call) => JSON.parse(String(c.init.body)) as Record<string, unknown>;

describe("booking dialog", () => {
  it("the prototype's description, verbatim (ADR 0084 #3: the deadline now follows the type); “Không rõ người gửi” stays (C7)", () => {
    mount(vi.fn(), false);
    const text = document.querySelector("dialog")!.textContent ?? "";
    expect(text).toContain(
      "Người gửi, địa chỉ, số điện thoại và nội dung là những mục quy định bắt buộc ghi nhận. Hạn giải quyết tính theo loại đơn.",
    );
    expect(text).toContain("Không rõ người gửi");
  });

  it("“Cần nội dung đơn.” in the dialog, and nothing is sent", async () => {
    const calls = stubServer();
    mount(vi.fn(), false);
    await act(async () => form().requestSubmit());
    expect(form().querySelector('[role="alert"]')!.textContent).toBe(SUMMARY_REQUIRED);
    expect(calls).toHaveLength(0);
  });

  it("duplicate check: debounced POST with the name in the BODY; the tangerine box; linking one sets related_letter_id", async () => {
    vi.useFakeTimers();
    const calls = stubServer();
    const onBooked = mount();
    await act(async () => {
      vi.advanceTimersByTime(600);
    });
    await act(async () => {});
    vi.useRealTimers();

    const dup = calls.find((c) => c.url.startsWith("/api/v1/citizen-letters/duplicates"))!;
    expect(dup.url).toBe("/api/v1/citizen-letters/duplicates");
    expect(dup.init.method).toBe("POST");
    expect(bodyOf(dup)).toEqual({
      sender_name: "Nguyễn Văn A",
      summary: "Đường thôn bị ngập sau mưa lớn",
      letter_type: "kien-nghi-phan-anh",
    });

    const box = document.querySelector('[aria-label="Cảnh báo đơn trùng"]')!;
    expect(box.textContent).toContain("Công dân này đã có 2 đơn nội dung tương tự");
    // d/m/yyyy, as the prototype's duplicate line (`PetitionEntryForm.tsx:188`, `formatDay`).
    expect(box.textContent).toContain("Số 3/2026 · 1/9/2026 · giống 82%");
    expect(box.textContent).toContain(DUPLICATE_FOOTER);

    const toggles = [...box.querySelectorAll<HTMLButtonElement>("button")];
    expect(toggles.map((b) => b.textContent)).toEqual([MERGE_INTO_LABEL, MERGE_INTO_LABEL]);
    act(() => toggles[0]!.click());
    act(() => toggles[1]!.click());
    // ONE link at most: pressing the second releases the first.
    expect(toggles.map((b) => b.getAttribute("aria-pressed"))).toEqual(["false", "true"]);

    await act(async () => form().requestSubmit());
    await act(async () => {});
    const booking = calls.find((c) => c.url === "/api/v1/citizen-letters")!;
    expect((booking.init.headers as Record<string, string>)["Idempotency-Key"]).toMatch(/[0-9a-f-]{36}/);
    expect(bodyOf(booking)).toMatchObject({ related_letter_id: "D2", sender_name: "Nguyễn Văn A", summary: "Đường thôn bị ngập sau mưa lớn" });
    expect(bodyOf(booking)).not.toHaveProperty("number");
    expect(bodyOf(booking)).not.toHaveProperty("processing_due_at");
    expect(onBooked).toHaveBeenCalledTimes(1);
  });

  it("“Không rõ người gửi” clears and locks the three fields; the booking carries no sender", async () => {
    const calls = stubServer();
    mount();
    const unknown = document.querySelector<HTMLInputElement>("#don-thu-khong-ro")!;
    act(() => unknown.click());
    for (const id of ["don-thu-nguoi-gui", "don-thu-dia-chi", "don-thu-so-dien-thoai"]) {
      const el = document.querySelector<HTMLInputElement>(`#${id}`)!;
      expect(el.disabled).toBe(true);
      expect(el.value).toBe("");
    }
    await act(async () => form().requestSubmit());
    await act(async () => {});
    const booking = calls.find((c) => c.url === "/api/v1/citizen-letters")!;
    expect(bodyOf(booking)).toEqual({
      received_date: "2026-10-08",
      letter_type: "kien-nghi-phan-anh",
      summary: "Đường thôn bị ngập sau mưa lớn",
    });
  });
});
