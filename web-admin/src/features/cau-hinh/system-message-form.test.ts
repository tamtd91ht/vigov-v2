import { afterEach, describe, expect, it, vi } from "vitest";

import {
  lastEditLine,
  MESSAGES_NOT_RAISED_YET,
  restoreMessageFlow,
  saveMessageFlow,
  SYSTEM_MESSAGE_MAX,
  TEXT_EMPTY,
  TEXT_TOO_LONG,
  validateMessageText,
} from "./system-message-form";

function reply(status: number, body?: unknown) {
  return body === undefined
    ? new Response(null, { status })
    : new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("validateMessageText", () => {
  it("trims before anything else", () => {
    expect(validateMessageText("  Câu mới.  \n")).toEqual({ ok: true, text: "Câu mới." });
  });

  it("empty (or only spaces) is refused and points to Khôi phục", () => {
    expect(validateMessageText("   ")).toEqual({ ok: false, message: TEXT_EMPTY });
    expect(TEXT_EMPTY).toMatch(/Khôi phục câu mặc định/);
  });

  it("counts CHARACTERS: 1000 Vietnamese letters pass, 1001 do not", () => {
    // "ệ" is one code point but may be several UTF-16 units in decomposed form; the server counts runes.
    expect(validateMessageText("ệ".repeat(SYSTEM_MESSAGE_MAX)).ok).toBe(true);
    expect(validateMessageText("ệ".repeat(SYSTEM_MESSAGE_MAX + 1))).toEqual({ ok: false, message: TEXT_TOO_LONG });
  });
});

describe("saveMessageFlow — edit", () => {
  it("our refusal does NOT send the request", async () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    expect(await saveMessageFlow("petitions", "feedback.never_public", "  ")).toEqual({
      ok: false,
      thongBao: TEXT_EMPTY,
    });
    expect(fake).not.toHaveBeenCalled();
  });

  it("sends the TRIMMED text with PUT and returns the server's message", async () => {
    const saved = {
      code: "feedback.never_public",
      description: "d",
      default_text: "Mặc định.",
      current_text: "Câu của xã.",
      overridden: true,
      updated_at: "2026-09-28T10:00:00Z",
      updated_by: "CB-00123",
    };
    const fake = vi.fn(async () => reply(200, saved));
    vi.stubGlobal("fetch", fake);
    expect(await saveMessageFlow("petitions", "feedback.never_public", "  Câu của xã.  ")).toEqual({
      ok: true,
      duLieu: saved,
    });
    const [path, init] = fake.mock.calls[0] as unknown as [string, RequestInit];
    expect(path).toBe("/api/v1/petitions-system-messages/feedback.never_public/override");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({ text: "Câu của xã." });
  });

  it("the server's 400 sentence comes back verbatim", async () => {
    const sentence = "Nội dung câu chứa ký tự điều khiển hoặc ký tự vô hình.";
    vi.stubGlobal("fetch", vi.fn(async () => reply(400, { code: "invalid_text", message: sentence })));
    expect(await saveMessageFlow("finance", "budget.scope_notice", "x")).toEqual({ ok: false, thongBao: sentence });
  });
});

describe("restoreMessageFlow — Khôi phục câu mặc định", () => {
  it("DELETE the override; 204 is success", async () => {
    const fake = vi.fn(async () => reply(204));
    vi.stubGlobal("fetch", fake);
    expect(await restoreMessageFlow("finance", "budget.scope_notice")).toEqual({ ok: true, duLieu: null });
    const [path, init] = fake.mock.calls[0] as unknown as [string, RequestInit];
    expect(path).toBe("/api/v1/finance-system-messages/budget.scope_notice/override");
    expect(init.method).toBe("DELETE");
  });

  it("a refusal is the server's sentence", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => reply(403, { code: "forbidden", message: "Bạn không có quyền." })));
    expect(await restoreMessageFlow("petitions", "feedback.never_public")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền.",
    });
  });
});

describe("small decisions", () => {
  it("exactly the two keys no branch raises yet are marked", () => {
    expect([...MESSAGES_NOT_RAISED_YET].sort()).toEqual([
      "feedback.after_photo_required",
      "feedback.unknown_field",
    ]);
  });

  it("last-edit line only for an overridden sentence", () => {
    const base = { code: "c", description: "", default_text: "", current_text: "", updated_at: null };
    expect(lastEditLine({ ...base, overridden: false })).toBeNull();
    expect(lastEditLine({ ...base, overridden: true, updated_by: "CB-00123" })).toBe("Sửa lần cuối: CB-00123");
  });
});
