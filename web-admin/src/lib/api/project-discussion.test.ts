import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getProjectComments,
  getProjectIssues,
  postProjectComment,
  recordProjectIssue,
  resolveProjectIssue,
} from "./project-discussion";

/**
 * The five §8.1 / §8.4 routes. The failures that matter are silent: a POST without `Idempotency-Key`,
 * an id concatenated raw into the path, a commune or an author named by the client, a 409 rewritten
 * instead of shown verbatim, and a body on the no-body resolution route.
 */

function stubFetch(reply: () => Response) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) => reply());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const ISSUE = {
  id: "01JVM1",
  project_id: "DA/1",
  title: "Chờ Sở thẩm định thiết kế",
  recorded_by: "CB-00001",
  recorded_at: "2026-08-27T02:00:00Z",
  resolved: false,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("issues", () => {
  it("GET encodes the id, is relative, and names no commune", async () => {
    const fake = stubFetch(() => json({ project_id: "DA/1", items: [ISSUE], count: 1, open_count: 1 }, 200));
    const r = await getProjectIssues("DA/1");
    expect(r.ok && r.duLieu.open_count).toBe(1);
    const [path, init] = fake.mock.calls[0]!;
    expect(path).toBe("/api/v1/investment-projects/DA%2F1/issues");
    expect(init?.method).toBe("GET");
    expect(path).not.toMatch(/tenant/i);
  });

  it("POST sends the text AS TYPED with the Idempotency-Key, and nothing else", async () => {
    const fake = stubFetch(() => json(ISSUE, 201));
    const typed = "  Chờ Sở thẩm định\nHồ sơ nộp 20/8  ";
    const r = await recordProjectIssue("DA/1", typed, "key-1");
    expect(r.ok).toBe(true);
    const [path, init] = fake.mock.calls[0]!;
    expect(path).toBe("/api/v1/investment-projects/DA%2F1/issues");
    expect(init?.method).toBe("POST");
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("key-1");
    // The server splits and validates; the client neither trims nor cuts.
    expect(JSON.parse(String(init?.body))).toEqual({ text: typed });
  });

  it("POST refused (403) returns the server's sentence verbatim", async () => {
    stubFetch(() => json({ code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }, 403));
    const r = await recordProjectIssue("DA1", "x", "k");
    expect(r).toEqual({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
  });

  it("resolution: POST with NO body and NO Content-Type, path on the issue id", async () => {
    const fake = stubFetch(() => json({ ...ISSUE, resolved: true }, 200));
    const r = await resolveProjectIssue("01J/VM1");
    expect(r.ok && r.duLieu.resolved).toBe(true);
    const [path, init] = fake.mock.calls[0]!;
    expect(path).toBe("/api/v1/project-issues/01J%2FVM1/resolution");
    expect(init?.method).toBe("POST");
    expect(init?.body).toBeUndefined();
    expect(new Headers(init?.headers).get("Content-Type")).toBeNull();
  });

  it("resolution 409: the server's sentence, prefix stripped, not a guess of ours", async () => {
    stubFetch(() =>
      json(
        {
          code: "issue_already_resolved",
          message: "vuong_mac: vướng mắc này đã được ghi là đã gỡ — nếu vướng mắc quay lại, hãy ghi nhận một vướng mắc mới",
        },
        409,
      ),
    );
    const r = await resolveProjectIssue("01JVM1");
    expect(r.ok).toBe(false);
    expect(!r.ok && r.thongBao).toBe(
      "Vướng mắc này đã được ghi là đã gỡ — nếu vướng mắc quay lại, hãy ghi nhận một vướng mắc mới",
    );
  });
});

describe("comments", () => {
  it("GET encodes the id", async () => {
    const fake = stubFetch(() => json({ project_id: "DA/1", items: [], count: 0 }, 200));
    const r = await getProjectComments("DA/1");
    expect(r.ok && r.duLieu.count).toBe(0);
    expect(fake.mock.calls[0]![0]).toBe("/api/v1/investment-projects/DA%2F1/comments");
  });

  it("POST sends body + codes with the Idempotency-Key — and only those two fields", async () => {
    const fake = stubFetch(() =>
      json(
        {
          id: "C1",
          project_id: "DA1",
          body: "@Nguyễn Văn A xem giúp",
          author_code: "CB-00002",
          mentioned_staff_codes: ["CB-00001"],
          created_at: "2026-08-27T02:00:00Z",
        },
        201,
      ),
    );
    const input = { body: "@Nguyễn Văn A xem giúp", mentioned_staff_codes: ["CB-00001"], author_code: "CB-9" };
    const r = await postProjectComment("DA1", input, "key-2");
    expect(r.ok).toBe(true);
    const [path, init] = fake.mock.calls[0]!;
    expect(path).toBe("/api/v1/investment-projects/DA1/comments");
    expect(init?.method).toBe("POST");
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("key-2");
    expect(JSON.parse(String(init?.body))).toEqual({
      body: "@Nguyễn Văn A xem giúp",
      mentioned_staff_codes: ["CB-00001"],
    });
  });

  it("POST with no mentions sends an empty list, never omits it", async () => {
    const fake = stubFetch(() => json({}, 201));
    await postProjectComment("DA1", { body: "ok" }, "k");
    expect(JSON.parse(String(fake.mock.calls[0]![1]?.body))).toEqual({ body: "ok", mentioned_staff_codes: [] });
  });

  it("network failure is one sentence, not a throw", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("offline"))));
    const r = await postProjectComment("DA1", { body: "ok" }, "k");
    expect(r.ok).toBe(false);
  });
});
