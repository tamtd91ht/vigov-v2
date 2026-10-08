import { afterEach, describe, expect, it, vi } from "vitest";

import {
  addCitizenLetterNote,
  bookCitizenLetter,
  checkLetterDuplicates,
  citizenLetterListPath,
  citizenLetterReportPath,
  correctCitizenLetterSender,
  moveCitizenLetter,
  routeCitizenLetter,
  setCitizenLetterDeadline,
} from "./citizen-letters";

type Call = { url: string; init: RequestInit };

function stubFetch(status: number, body: unknown = {}): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
    }),
  );
  return calls;
}

const sentBody = (c: Call) => JSON.parse(String(c.init.body)) as Record<string, unknown>;
const header = (c: Call, name: string) => (c.init.headers as Record<string, string> | undefined)?.[name];

afterEach(() => vi.unstubAllGlobals());

describe("citizen-letter list path", () => {
  it("sends only the filters that are set, with the contract's names; `all` is the server's default", () => {
    expect(citizenLetterListPath()).toBe("/api/v1/citizen-letters");
    expect(citizenLetterListPath({ scope: "all", status: "", cursor: null })).toBe("/api/v1/citizen-letters");
    const path = citizenLetterListPath({
      scope: "mine",
      receivedFrom: "2026-10-01",
      receivedTo: "2026-10-08",
      holdingUnit: "U1",
      assignee: "CB-00002",
      status: "thu-ly",
      cursor: "abc",
    });
    const q = new URL(path, "http://x").searchParams;
    expect(q.get("scope")).toBe("mine");
    expect(q.get("received_from")).toBe("2026-10-01");
    expect(q.get("received_to")).toBe("2026-10-08");
    expect(q.get("holding_unit")).toBe("U1");
    expect(q.get("assignee")).toBe("CB-00002");
    expect(q.get("status")).toBe("thu-ly");
    expect(q.get("cursor")).toBe("abc");
  });

  it("the report path always carries the year (the server requires it)", () => {
    expect(citizenLetterReportPath(2026)).toBe("/api/v1/citizen-letter-report?year=2026");
  });
});

describe("citizen-letter writes", () => {
  it("booking: the Idempotency-Key the caller holds, no server-owned field, empty optionals left out", async () => {
    const calls = stubFetch(201, { id: "L1" });
    await bookCitizenLetter(
      {
        received_date: "2026-10-08",
        letter_type: "khieu-nai",
        summary: "Nội dung",
        sender_name: "",
        related_letter_id: "L0",
        // A stray field from a row just read must not travel — the body is built field by field.
        ...({ number: 7, status: "thu-ly", processing_due_at: "x" } as object),
      },
      "key-1",
    );
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe("/api/v1/citizen-letters");
    expect(calls[0]!.init.method).toBe("POST");
    expect(header(calls[0]!, "Idempotency-Key")).toBe("key-1");
    expect(sentBody(calls[0]!)).toEqual({
      received_date: "2026-10-08",
      letter_type: "khieu-nai",
      summary: "Nội dung",
      related_letter_id: "L0",
    });
  });

  it("duplicate check: a POST whose BODY carries the name — the URL never does (rule 3, forbidden #4)", async () => {
    const calls = stubFetch(200, { items: [] });
    await checkLetterDuplicates({ sender_name: "Nguyễn Văn A", summary: "Đường thôn bị ngập", letter_type: "kien-nghi-phan-anh" });
    expect(calls[0]!.url).toBe("/api/v1/citizen-letters/duplicates");
    expect(calls[0]!.url).not.toContain("?");
    expect(calls[0]!.init.method).toBe("POST");
    expect(sentBody(calls[0]!)).toEqual({
      sender_name: "Nguyễn Văn A",
      summary: "Đường thôn bị ngập",
      letter_type: "kien-nghi-phan-anh",
    });
  });

  it("routing, status: the id is encoded into the path; the optional field is left out when empty", async () => {
    const calls = stubFetch(200, {});
    await routeCitizenLetter("a/b", { to_unit: "U1", assignee: "", reason: "Lý do" });
    expect(calls[0]!.url).toBe("/api/v1/citizen-letters/a%2Fb/routings");
    expect(sentBody(calls[0]!)).toEqual({ to_unit: "U1", reason: "Lý do" });
    await moveCitizenLetter("L1", { status: "dang-xu-ly-don", note: "" });
    expect(sentBody(calls[1]!)).toEqual({ status: "dang-xu-ly-don" });
  });

  it("sender correction: KEY PRESENCE is the meaning — absent stays, null clears", async () => {
    const calls = stubFetch(200, {});
    await correctCitizenLetterSender("L1", { sender_phone: "0900000000" });
    expect(sentBody(calls[0]!)).toEqual({ sender_phone: "0900000000" });
    await correctCitizenLetterSender("L1", { sender_name: null, sender_phone: null, sender_address: null });
    expect(sentBody(calls[1]!)).toEqual({ sender_name: null, sender_phone: null, sender_address: null });
    expect(calls[1]!.init.method).toBe("PATCH");
  });

  it("deadline: PATCH with EXACTLY { due_at } — an instant sets, null clears; the id is in the path", async () => {
    const calls = stubFetch(200, {});
    await setCitizenLetterDeadline("L/1", { due_at: "2026-10-20T17:00:00+07:00" });
    expect(calls[0]!.url).toBe("/api/v1/citizen-letters/L%2F1/deadline");
    expect(calls[0]!.init.method).toBe("PATCH");
    expect(sentBody(calls[0]!)).toEqual({ due_at: "2026-10-20T17:00:00+07:00" });
    await setCitizenLetterDeadline("L1", { due_at: null });
    expect(sentBody(calls[1]!)).toEqual({ due_at: null });
  });

  it("a note carries the draft's Idempotency-Key (the route requires one)", async () => {
    const calls = stubFetch(201, { id: "E1" });
    const k = await addCitizenLetterNote("L1", "Đã gọi điện cho công dân", "note-key");
    expect(k.ok).toBe(true);
    expect(calls[0]!.url).toBe("/api/v1/citizen-letters/L1/log-entries");
    expect(header(calls[0]!, "Idempotency-Key")).toBe("note-key");
  });

  it("a refusal comes back as the server's sentence, verbatim", async () => {
    stubFetch(409, { code: "letter_state", message: "Chưa ghi kết quả giải quyết.", trace_id: "t" });
    const k = await moveCitizenLetter("L1", { status: "da-giai-quyet" });
    expect(k).toEqual({ ok: false, thongBao: "Chưa ghi kết quả giải quyết." });
  });
});
