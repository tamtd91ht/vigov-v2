import { afterEach, describe, expect, it, vi } from "vitest";

import type { SystemMessage } from "@/lib/api/system-messages";

import {
  ADD_CODE_PLACEHOLDER,
  addMessageFlow,
  groupOfCode,
  DELETE_REASON_MISSING,
  deleteMessageFlow,
  editableText,
  emptyAddDraft,
  isTextChanged,
  restoreMessageFlow,
  saveMessageFlow,
  switchMessageFlow,
  SYSTEM_MESSAGE_MAX,
  TEXT_EMPTY,
  TEXT_EMPTY_COMMUNE,
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
    expect(TEXT_EMPTY).toMatch(/Khôi phục lời gốc/);
  });

  it("counts CHARACTERS: 1000 Vietnamese letters pass, 1001 do not", () => {
    // "ệ" is one code point but may be several UTF-16 units in decomposed form; the server counts runes.
    expect(validateMessageText("ệ".repeat(SYSTEM_MESSAGE_MAX)).ok).toBe(true);
    expect(validateMessageText("ệ".repeat(SYSTEM_MESSAGE_MAX + 1))).toEqual({ ok: false, message: TEXT_TOO_LONG });
  });
});

const SHIPPED: SystemMessage = {
  code: "feedback.never_public",
  group_code: "phan-anh",
  origin: "shipped",
  description: "d",
  default_text: "Mặc định.",
  current_text: "Mặc định.",
  overridden: false,
  is_active: true,
};

const COMMUNE: SystemMessage = {
  code: "chung.loi-chao",
  group_code: "chung",
  origin: "commune",
  description: "",
  current_text: "Xã xin chào.",
  overridden: false,
  is_active: true,
};

function lastCall(fake: ReturnType<typeof vi.fn>) {
  const [path, init] = fake.mock.calls.at(-1) as unknown as [string, RequestInit];
  return {
    path,
    method: init.method,
    headers: new Headers(init.headers),
    body: init.body === undefined ? undefined : (JSON.parse(String(init.body)) as unknown),
  };
}

describe("saveMessageFlow — edit", () => {
  it("our refusal does NOT send the request", async () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    expect(await saveMessageFlow("petitions", SHIPPED, "  ")).toEqual({
      ok: false,
      thongBao: TEXT_EMPTY,
    });
    expect(fake).not.toHaveBeenCalled();
  });

  it("a COMMUNE sentence: empty is refused WITHOUT pointing to Khôi phục (it has no lời gốc)", async () => {
    vi.stubGlobal("fetch", vi.fn());
    expect(await saveMessageFlow("petitions", COMMUNE, " ")).toEqual({ ok: false, thongBao: TEXT_EMPTY_COMMUNE });
    expect(TEXT_EMPTY_COMMUNE).not.toMatch(/Khôi phục/);
  });

  it("a COMMUNE sentence is reworded with PATCH …/{code} { text }, never the override route", async () => {
    const fake = vi.fn(async () => reply(200, { ...COMMUNE, current_text: "Xin chào." }));
    vi.stubGlobal("fetch", fake);
    expect((await saveMessageFlow("finance", { ...COMMUNE, code: "giai-ngan.x" }, " Xin chào. ")).ok).toBe(true);
    const c = lastCall(fake);
    expect(c.path).toBe("/api/v1/finance-system-messages/giai-ngan.x");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ text: "Xin chào." });
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
    expect(await saveMessageFlow("petitions", SHIPPED, "  Câu của xã.  ")).toEqual({
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
    expect(await saveMessageFlow("finance", { ...SHIPPED, code: "budget.scope_notice" }, "x")).toEqual({
      ok: false,
      thongBao: sentence,
    });
  });
});

describe("switchMessageFlow — Tắt / Bật lại", () => {
  it("a reworded SHIPPED sentence: PATCH …/{code}/override { is_active: false }", async () => {
    const fake = vi.fn(async () => reply(200, { ...SHIPPED, overridden: true, is_active: false }));
    vi.stubGlobal("fetch", fake);
    const r = await switchMessageFlow("reporting", { ...SHIPPED, code: "report.title", overridden: true });
    expect(r.ok).toBe(true);
    const c = lastCall(fake);
    expect(c.path).toBe("/api/v1/reporting-system-messages/report.title/override");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ is_active: false });
  });

  it("a switched-off COMMUNE sentence: PATCH …/{code} { is_active: true }", async () => {
    const fake = vi.fn(async () => reply(200, COMMUNE));
    vi.stubGlobal("fetch", fake);
    await switchMessageFlow("petitions", { ...COMMUNE, is_active: false });
    const c = lastCall(fake);
    expect(c.path).toBe("/api/v1/petitions-system-messages/chung.loi-chao");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ is_active: true });
  });

  it("a shipped sentence the commune NEVER reworded has the switch too (owner, 09/10/2026): PATCH …/override { is_active: false }", async () => {
    const fake = vi.fn(async () => reply(200, { ...SHIPPED, is_active: false }));
    vi.stubGlobal("fetch", fake);
    const r = await switchMessageFlow("finance", { ...SHIPPED, code: "budget.scope_notice" });
    expect(r).toEqual({ ok: true, duLieu: { ...SHIPPED, is_active: false } });
    const c = lastCall(fake);
    expect(c.path).toBe("/api/v1/finance-system-messages/budget.scope_notice/override");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ is_active: false });
  });

  it("a refusal comes back as the server's sentence", async () => {
    const sentence = "Không có câu hệ thống mang mã này ở phân hệ Tiếp dân – Nhiệm vụ.";
    vi.stubGlobal("fetch", vi.fn(async () => reply(404, { code: "not_found", message: sentence })));
    expect(await switchMessageFlow("petitions", SHIPPED)).toEqual({ ok: false, thongBao: sentence });
  });
});

describe("addMessageFlow — Thêm câu mới", () => {
  it("code or text missing → spec 07's sentence, NOTHING sent", async () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    for (const d of [
      { ...emptyAddDraft(), text: "Câu." },
      { ...emptyAddDraft(), code: "chung.a" },
      { ...emptyAddDraft(), code: "  ", text: "  " },
    ]) {
      expect(await addMessageFlow(d, "k-1")).toEqual({ ok: false, thongBao: "Cần cả mã và nội dung câu." });
    }
    expect(fake).not.toHaveBeenCalled();
  });

  it.each([
    ["chung", "petitions", "/api/v1/petitions-system-messages"],
    ["phan-anh", "petitions", "/api/v1/petitions-system-messages"],
    ["giai-ngan", "finance", "/api/v1/finance-system-messages"],
  ] as const)("prefix '%s.' → group from the CODE, sent to %s: POST %s with the Idempotency-Key, trimmed fields", async (group, module, path) => {
    const created = { ...COMMUNE, code: `${group}.loi-chao`, group_code: group };
    const fake = vi.fn(async () => reply(201, created));
    vi.stubGlobal("fetch", fake);
    const r = await addMessageFlow(
      { code: ` ${group}.loi-chao `, description: "  Ở đầu thư. ", text: " Xin chào. " },
      "key-123",
    );
    expect(r).toEqual({ ok: true, duLieu: { module, message: created } });
    const c = lastCall(fake);
    expect(c.path).toBe(path);
    expect(c.method).toBe("POST");
    expect(c.headers.get("Idempotency-Key")).toBe("key-123");
    expect(c.body).toEqual({ group_code: group, code: `${group}.loi-chao`, text: "Xin chào.", description: "Ở đầu thư." });
  });

  it("an empty description is OMITTED, not sent as ''", async () => {
    const fake = vi.fn(async () => reply(201, COMMUNE));
    vi.stubGlobal("fetch", fake);
    await addMessageFlow({ ...emptyAddDraft(), code: "chung.a", text: "B.", description: "   " }, "k");
    expect(lastCall(fake).body).toEqual({ group_code: "chung", code: "chung.a", text: "B." });
  });

  it("409 message_code_taken: the server's sentence, verbatim", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => reply(409, { code: "message_code_taken", message: "Mã này đã được dùng cho một câu khác." })),
    );
    expect(await addMessageFlow({ ...emptyAddDraft(), code: "chung.a", text: "B." }, "k")).toEqual({
      ok: false,
      thongBao: "Mã này đã được dùng cho một câu khác.",
    });
  });

  it.each(["bao-cao.a", "loi-chao", "chung", "chunga.b", "Chung.a", ".chung.a", "feedback.x"])(
    "code '%s' has none of the three prefixes → refused in place with the prefix sentence, NOTHING sent (fail closed)",
    async (code) => {
      const fake = vi.fn();
      vi.stubGlobal("fetch", fake);
      expect(await addMessageFlow({ code, description: "", text: "B." }, "k")).toEqual({
        ok: false,
        thongBao: "Mã câu phải bắt đầu bằng chung., phan-anh. hoặc giai-ngan.",
      });
      expect(fake).not.toHaveBeenCalled();
    },
  );

  it("missing fields are reported before the prefix", async () => {
    vi.stubGlobal("fetch", vi.fn());
    expect(await addMessageFlow({ code: "khong-tien-to", description: "", text: " " }, "k")).toEqual({
      ok: false,
      thongBao: "Cần cả mã và nội dung câu.",
    });
  });

  it("groupOfCode reads the prefix; the placeholder is spec 07's fixed one and is itself a valid code", () => {
    expect(groupOfCode("phan-anh.cam-on")).toEqual({ group: "phan-anh", module: "petitions" });
    expect(groupOfCode("giai-ngan.nhac")).toEqual({ group: "giai-ngan", module: "finance" });
    expect(groupOfCode("bao-cao.x")).toBeUndefined();
    expect(ADD_CODE_PLACEHOLDER).toBe("chung.loi-chao");
    expect(groupOfCode(ADD_CODE_PLACEHOLDER)?.group).toBe("chung");
    expect("group" in emptyAddDraft()).toBe(false);
  });
});

describe("deleteMessageFlow — Xoá câu xã tự thêm", () => {
  it("no reason → refused before sending (rule 7)", async () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    expect(await deleteMessageFlow("petitions", "chung.a", "   ")).toEqual({ ok: false, thongBao: DELETE_REASON_MISSING });
    expect(fake).not.toHaveBeenCalled();
  });

  it("DELETE …/{code} with { reason } trimmed in the BODY; 204 is success", async () => {
    const fake = vi.fn(async () => reply(204));
    vi.stubGlobal("fetch", fake);
    expect(await deleteMessageFlow("finance", "giai-ngan.a", " Trùng câu khác. ")).toEqual({ ok: true, duLieu: null });
    const c = lastCall(fake);
    expect(c.path).toBe("/api/v1/finance-system-messages/giai-ngan.a");
    expect(c.method).toBe("DELETE");
    expect(c.body).toEqual({ reason: "Trùng câu khác." });
  });
});

describe("small decisions — the card", () => {
  it("the edit box holds the commune's words of a switched-off rewording, not the default in force", () => {
    expect(editableText({ ...SHIPPED, overridden: true, is_active: false, override_text: "Của xã." })).toBe("Của xã.");
    expect(editableText(SHIPPED)).toBe("Mặc định.");
    expect(editableText(COMMUNE)).toBe("Xã xin chào.");
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
  it("Lưu is enabled only by a change beyond surrounding spaces", () => {
    expect(isTextChanged("Câu cũ.", "Câu cũ.")).toBe(false);
    expect(isTextChanged("  Câu cũ. ", "Câu cũ.")).toBe(false);
    expect(isTextChanged("Câu mới.", "Câu cũ.")).toBe(true);
    expect(isTextChanged("", "Câu cũ.")).toBe(true);
  });

  it("owner 08/10/2026 'Bỏ hết, đúng prototype': no last-edit line, no not-raised set, no restore confirmation", async () => {
    const form = await import("./system-message-form");
    for (const gone of [
      "lastEditLine",
      "MESSAGES_NOT_RAISED_YET",
      "NOT_RAISED_NOTE",
      "RESTORE_CONFIRM",
      "RESTORE_CONFIRM_BUTTON",
      "CANCEL_BUTTON",
    ]) {
      expect(gone in form).toBe(false);
    }
    expect(form.SYSTEM_MESSAGE_SECTIONS.some((s) => "note" in s)).toBe(false);
  });
});
