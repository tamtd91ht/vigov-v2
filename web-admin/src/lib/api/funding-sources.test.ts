import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createFundingSource,
  fundingSourceProjectsPath,
  fundingSourcesPath,
  listFundingSourceProjects,
  listFundingSources,
  setGrantedAmount,
} from "./funding-sources";

/**
 * The four funding-source routes. The failures that matter are silent: a read without `year` (400 on
 * every load), a POST without `Idempotency-Key`, a blank granted amount sent as `0` (recorded as
 * "granted nothing"), a commune named by the client, and a 409 rewritten instead of shown verbatim.
 */

function stubFetch(reply: Response) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) => reply);
  vi.stubGlobal("fetch", fake);
  return fake;
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function sentBody(fake: ReturnType<typeof stubFetch>): Record<string, unknown> {
  const raw = fake.mock.calls[0]?.[1]?.body;
  return typeof raw === "string" ? (JSON.parse(raw) as Record<string, unknown>) : {};
}

const SOURCE = {
  id: "01JSRC",
  name: "Ngân sách xã, phường",
  order: 1,
  year: 2026,
  granted_amount: 0,
  allocated_amount: 0,
  project_count: 0,
  disbursed_amount: 0,
  unallocated_amount: 0,
  overallocated_amount: 0,
  allocated_ratio: null,
  disbursed_of_allocated_ratio: null,
  disbursed_of_granted_ratio: null,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("GET routes", () => {
  it("paths carry `year`, are relative, encode the id and name no commune", () => {
    expect(fundingSourcesPath(2026)).toBe("/api/v1/funding-sources?year=2026");
    expect(fundingSourceProjectsPath("a/b", 2025)).toBe("/api/v1/funding-sources/a%2Fb/projects?year=2025");
    for (const p of [fundingSourcesPath(2026), fundingSourceProjectsPath("x", 2026)]) {
      expect(p).not.toMatch(/tenant/i);
      expect(p.startsWith("/")).toBe(true);
    }
  });

  it("reads the year's cards and one source's projects with GET", async () => {
    const fake = stubFetch(json({ year: 2026, items: [SOURCE], unattributed_disbursed_amount: 0 }, 200));
    const list = await listFundingSources(2026);
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    expect(list.ok).toBe(true);

    vi.unstubAllGlobals();
    const fake2 = stubFetch(json({ funding_source_id: "01JSRC", name: "x", year: 2026, items: [], disbursed_without_allocation_amount: 0 }, 200));
    expect((await listFundingSourceProjects("01JSRC", 2026)).ok).toBe(true);
    expect(fake2.mock.calls[0]?.[0]).toBe("/api/v1/funding-sources/01JSRC/projects?year=2026");
  });
});

describe("POST /api/v1/funding-sources", () => {
  it("body {name, year, granted_amount}, Idempotency-Key from the caller, 201", async () => {
    const fake = stubFetch(json({ ...SOURCE, granted_amount: 5_000_000 }, 201));
    const result = await createFundingSource({ name: "Ngân sách xã, phường", year: 2026, granted_amount: 5_000_000 }, "key-1");

    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/funding-sources");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect((fake.mock.calls[0]?.[1]?.headers as Record<string, string>)["Idempotency-Key"]).toBe("key-1");
    expect(sentBody(fake)).toEqual({ name: "Ngân sách xã, phường", year: 2026, granted_amount: 5_000_000 });
    expect(result.ok).toBe(true);
  });

  it("no amount typed: `granted_amount` is ABSENT, never 0", async () => {
    const fake = stubFetch(json(SOURCE, 201));
    await createFundingSource({ name: "A", year: 2026, granted_amount: undefined }, "k");
    expect(sentBody(fake)).not.toHaveProperty("granted_amount");

    vi.unstubAllGlobals();
    const fake2 = stubFetch(json(SOURCE, 201));
    await createFundingSource({ name: "A", year: 2026, granted_amount: null }, "k");
    expect(sentBody(fake2)).not.toHaveProperty("granted_amount");
  });

  it("an explicit 0 IS sent: 'granted nothing' is a recorded fact", async () => {
    const fake = stubFetch(json(SOURCE, 201));
    await createFundingSource({ name: "A", year: 2026, granted_amount: 0 }, "k");
    expect(sentBody(fake)).toEqual({ name: "A", year: 2026, granted_amount: 0 });
  });

  it("409 comes back as the server's sentence", async () => {
    const sentence = "Danh mục nguồn vốn của xã đã đủ số nguồn tối đa nên chưa thêm được.";
    stubFetch(json({ code: "funding_source_catalogue_full", message: sentence, trace_id: "t" }, 409));
    expect(await createFundingSource({ name: "A", year: 2026 }, "k")).toEqual({ ok: false, thongBao: sentence });
  });
});

describe("PUT /api/v1/funding-sources/{id}/annual-amounts/{year}", () => {
  it("id encoded, year in the path, body {granted_amount}, 200, no idempotency key", async () => {
    const fake = stubFetch(json({ funding_source_id: "01J/S", year: 2026, granted_amount: 9_200_000_000 }, 200));
    const result = await setGrantedAmount("01J/S", 2026, 9_200_000_000);

    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/funding-sources/01J%2FS/annual-amounts/2026");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("PUT");
    expect(sentBody(fake)).toEqual({ granted_amount: 9_200_000_000 });
    expect(fake.mock.calls[0]?.[1]?.headers as Record<string, string>).not.toHaveProperty("Idempotency-Key");
    expect(result.ok).toBe(true);
  });
});
