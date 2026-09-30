import { afterEach, describe, expect, it, vi } from "vitest";

import { AUDIT_SOURCES, auditQuery, getAuditEntries } from "./audit-entries";
import { LOI_KHONG_RO } from "./goi";

function reply(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function call(fake: ReturnType<typeof stubFetch>, i = 0) {
  const [path, init] = fake.mock.calls[i] as unknown as [string, RequestInit];
  return { path, method: init.method, headers: new Headers(init.headers), body: init.body };
}

const PAGE = {
  items: [
    {
      at: "2026-09-28T10:00:00Z",
      actor_kind: "staff",
      actor_code: "CB-00123",
      actor_ip: "10.0.0.1",
      action: "khoa_tai_khoan_can_bo",
      subject: "CB-00200",
      delta: { before: "hoat-dong", after: "khoa" },
    },
  ],
  next_cursor: "c2",
  has_more: true,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("getAuditEntries — one page of one service", () => {
  it("five sources, five distinct paths, exactly ADR 0054 §3's", () => {
    expect(AUDIT_SOURCES.map((s) => s.path)).toEqual([
      "/api/v1/identity-audit-entries",
      "/api/v1/documents-audit-entries",
      "/api/v1/finance-audit-entries",
      "/api/v1/comms-audit-entries",
      "/api/v1/petitions-audit-entries",
    ]);
  });

  it.each(AUDIT_SOURCES.map((s) => [s.key, s.path] as const))(
    "%s: GET its own route, no body, no tenant anywhere",
    async (key, path) => {
      const fake = stubFetch(() => reply(200, PAGE));
      expect(await getAuditEntries(key, {}, "")).toEqual({ ok: true, duLieu: PAGE });
      const c = call(fake);
      expect(c.path).toBe(path);
      expect(c.method).toBe("GET");
      expect(c.body).toBeUndefined();
      expect(c.path).not.toMatch(/tenant/i);
    },
  );

  it("filters and cursor are encoded; the +07:00 offset survives as %2B", async () => {
    const fake = stubFetch(() => reply(200, PAGE));
    await getAuditEntries(
      "finance",
      {
        from: "2026-09-01T00:00:00+07:00",
        to: "2026-10-01T00:00:00+07:00",
        actor: "CB-00123",
        action: "them_du_an",
        subject: "DA-2026 01",
      },
      "c2",
    );
    const url = new URL(call(fake).path, "https://xa.example");
    expect(url.pathname).toBe("/api/v1/finance-audit-entries");
    expect(url.searchParams.get("from")).toBe("2026-09-01T00:00:00+07:00");
    expect(url.searchParams.get("to")).toBe("2026-10-01T00:00:00+07:00");
    expect(url.searchParams.get("actor")).toBe("CB-00123");
    expect(url.searchParams.get("action")).toBe("them_du_an");
    expect(url.searchParams.get("subject")).toBe("DA-2026 01");
    expect(url.searchParams.get("cursor")).toBe("c2");
    expect(call(fake).path).toContain("from=2026-09-01T00%3A00%3A00%2B07%3A00");
  });

  it("empty filters and the first-page cursor are NOT sent (the server refuses an empty cursor=)", () => {
    expect(auditQuery({}, "")).toBe("");
    expect(auditQuery({ actor: "", action: undefined }, "")).toBe("");
    expect(auditQuery({ actor: "system" }, "")).toBe("?actor=system");
  });

  it("no `limit` is sent — every service applies the same default", () => {
    expect(auditQuery({ actor: "system" }, "c")).not.toContain("limit");
  });

  it("403 and 400 are the server's sentence, verbatim", async () => {
    stubFetch(() => reply(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }));
    expect(await getAuditEntries("identity", {}, "")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
    stubFetch(() => reply(400, { code: "bad_range", message: "Mốc bắt đầu phải trước mốc kết thúc." }));
    expect(await getAuditEntries("comms", { from: "b", to: "a" }, "")).toEqual({
      ok: false,
      thongBao: "Mốc bắt đầu phải trước mốc kết thúc.",
    });
  });

  it("a proxy page (502, not JSON) is 'no connection', never a partial success", async () => {
    stubFetch(() => new Response("<html>Bad Gateway</html>", { status: 502 }));
    expect(await getAuditEntries("petitions", {}, "")).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });
});
