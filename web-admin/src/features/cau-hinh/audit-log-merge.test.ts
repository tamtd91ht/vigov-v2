import { describe, expect, it } from "vitest";

import type { AuditSourceKey } from "@/lib/api/audit-entries";
import type { audit_EntryView } from "@/lib/api/schema.gen";

import {
  applyPage,
  atKey,
  compareAt,
  heldBackCount,
  initialSources,
  markLoading,
  nextToLoad,
  safeBoundary,
  visibleRows,
  type SourceState,
} from "./audit-log-merge";

function entry(at: string, action = "thao_tac"): audit_EntryView {
  return {
    at,
    actor_kind: "staff",
    actor_code: "CB-00123",
    actor_ip: "10.0.0.1",
    action,
    subject: "S",
    delta: null,
  };
}

function page(items: audit_EntryView[], hasMore: boolean, cursor = hasMore ? "next" : "") {
  return { ok: true as const, duLieu: { items, has_more: hasMore, next_cursor: cursor } };
}

/** All five answered: the listed ones with pages, the rest empty and finished. */
function answered(
  pages: Partial<Record<AuditSourceKey, ReturnType<typeof page> | { ok: false; thongBao: string }>>,
): SourceState[] {
  let s = initialSources();
  for (const k of ["identity", "documents", "finance", "comms", "petitions"] as const) {
    s = applyPage(s, k, pages[k] ?? page([], false));
  }
  return s;
}

const actions = (s: readonly SourceState[]) => visibleRows(s).map((r) => r.entry.action);

describe("ordering", () => {
  it("merges five sources by `at` descending, each row naming its source", () => {
    const s = answered({
      identity: page([entry("2026-09-28T10:00:00Z", "i1"), entry("2026-09-28T07:00:00Z", "i2")], false),
      finance: page([entry("2026-09-28T09:00:00Z", "f1")], false),
      petitions: page([entry("2026-09-28T11:00:00Z", "p1")], false),
    });
    expect(actions(s)).toEqual(["p1", "i1", "f1", "i2"]);
    expect(visibleRows(s).map((r) => r.source)).toEqual(["petitions", "identity", "finance", "identity"]);
  });

  it("compares instants, not strings: +07:00 and Z of the same moment are equal", () => {
    expect(compareAt(atKey("2026-09-28T17:00:00+07:00"), atKey("2026-09-28T10:00:00Z"))).toBe(0);
    const s = answered({
      identity: page([entry("2026-09-28T16:30:00+07:00", "earlier")], false),
      documents: page([entry("2026-09-28T10:00:00Z", "later")], false),
    });
    expect(actions(s)).toEqual(["later", "earlier"]);
  });

  it("sub-millisecond digits order two rows in the same millisecond", () => {
    expect(
      compareAt(atKey("2026-09-28T10:00:00.123400Z"), atKey("2026-09-28T10:00:00.123900Z")),
    ).toBeLessThan(0);
  });

  it("equal instants keep source order, then the server's own order", () => {
    const t = "2026-09-28T10:00:00Z";
    const s = answered({
      finance: page([entry(t, "f1"), entry(t, "f2")], false),
      identity: page([entry(t, "i1")], false),
    });
    expect(actions(s)).toEqual(["i1", "f1", "f2"]);
  });

  it("row keys are stable per source and index", () => {
    const s = answered({ comms: page([entry("2026-09-28T10:00:00Z")], false) });
    expect(visibleRows(s)[0]?.rowKey).toBe("comms:0");
  });
});

describe("the safe boundary (ADR 0054 §6)", () => {
  it("rows older than the NEWEST frontier of the sources with more are held back", () => {
    // identity loaded down to 10:00 and has more; finance down to 08:00 and has more. finance's 09:00
    // row must wait: identity's next page may hold a 09:30 row.
    const s = answered({
      identity: page([entry("2026-09-28T12:00:00Z", "i1"), entry("2026-09-28T10:00:00Z", "i2")], true),
      finance: page(
        [entry("2026-09-28T11:00:00Z", "f1"), entry("2026-09-28T09:00:00Z", "f2"), entry("2026-09-28T08:00:00Z", "f3")],
        true,
      ),
    });
    expect(actions(s)).toEqual(["i1", "f1", "i2"]);
    expect(heldBackCount(s)).toBe(2);
    const b = safeBoundary(s);
    expect(b.kind === "at" && b.holder).toBe("identity");
    // "Xem thêm" loads the source holding the boundary.
    expect(nextToLoad(s)).toBe("identity");
  });

  it("a row exactly AT the boundary is shown (not older than it)", () => {
    const s = answered({
      identity: page([entry("2026-09-28T10:00:00Z", "i1")], true),
      finance: page([entry("2026-09-28T10:00:00Z", "f1"), entry("2026-09-28T09:59:59Z", "f2")], false),
    });
    expect(actions(s)).toEqual(["i1", "f1"]);
  });

  it("loading the holder's next page moves the boundary and releases held rows in order", () => {
    let s = answered({
      identity: page([entry("2026-09-28T10:00:00Z", "i1")], true),
      finance: page([entry("2026-09-28T09:00:00Z", "f1"), entry("2026-09-28T07:00:00Z", "f2")], false),
    });
    expect(actions(s)).toEqual(["i1"]);
    s = markLoading(s, "identity");
    expect(s.find((x) => x.key === "identity")?.loading).toBe(true);
    s = applyPage(s, "identity", page([entry("2026-09-28T08:30:00Z", "i2")], false));
    expect(actions(s)).toEqual(["i1", "f1", "i2", "f2"]);
    expect(nextToLoad(s)).toBeNull();
    expect(heldBackCount(s)).toBe(0);
  });

  it("the cursor followed is the source's own next_cursor, appended, never replaced", () => {
    let s = answered({ documents: page([entry("2026-09-28T10:00:00Z", "d1")], true, "cur-d-2") });
    expect(s.find((x) => x.key === "documents")?.cursor).toBe("cur-d-2");
    s = applyPage(s, "documents", page([entry("2026-09-28T09:00:00Z", "d2")], false));
    const d = s.find((x) => x.key === "documents");
    expect(d?.items.map((e) => e.action)).toEqual(["d1", "d2"]);
    expect(d?.hasMore).toBe(false);
  });

  it("before every source answered, NOTHING is shown — an unanswered source may hold anything", () => {
    let s = initialSources();
    s = applyPage(s, "identity", page([entry("2026-09-28T10:00:00Z", "i1")], false));
    expect(visibleRows(s)).toEqual([]);
    expect(safeBoundary(s).kind).toBe("blocked");
  });

  it("has_more with an empty cursor is treated as the end, not as page one again", () => {
    const s = answered({ comms: page([entry("2026-09-28T10:00:00Z", "c1")], true, "") });
    expect(s.find((x) => x.key === "comms")?.hasMore).toBe(false);
    expect(nextToLoad(s)).toBeNull();
  });

  it("nothing more anywhere: every loaded row shows and there is no Xem thêm", () => {
    const s = answered({ identity: page([entry("2026-09-28T10:00:00Z")], false) });
    expect(safeBoundary(s)).toEqual({ kind: "none" });
    expect(nextToLoad(s)).toBeNull();
  });
});

describe("one service failing (ADR 0054 §7)", () => {
  it("the others still show, and the failed one leaves the boundary", () => {
    const s = answered({
      identity: page([entry("2026-09-28T10:00:00Z", "i1")], false),
      finance: { ok: false, thongBao: "Không kết nối được máy chủ. Vui lòng thử lại." },
    });
    expect(actions(s)).toEqual(["i1"]);
    const f = s.find((x) => x.key === "finance");
    expect(f?.error).toBe("Không kết nối được máy chủ. Vui lòng thử lại.");
    expect(nextToLoad(s)).toBeNull();
  });

  it("a failed 'Xem thêm' keeps the rows already loaded and the SAME cursor for a retry", () => {
    let s = answered({
      identity: page([entry("2026-09-28T10:00:00Z", "i1")], true, "cur-i-2"),
      finance: page([entry("2026-09-28T08:00:00Z", "f1")], false),
    });
    expect(actions(s)).toEqual(["i1"]);
    s = applyPage(s, "identity", { ok: false, thongBao: "Lỗi máy chủ." });
    const i = s.find((x) => x.key === "identity");
    expect(i?.items).toHaveLength(1);
    expect(i?.cursor).toBe("cur-i-2");
    expect(i?.error).toBe("Lỗi máy chủ.");
    // Out of the boundary: finance's row is no longer held behind a source that cannot answer.
    expect(actions(s)).toEqual(["i1", "f1"]);
  });

  it("a successful retry clears the error and the source rejoins the boundary", () => {
    let s = answered({ petitions: { ok: false, thongBao: "Lỗi." } });
    s = applyPage(s, "petitions", page([entry("2026-09-28T10:00:00Z", "p1")], true));
    expect(s.find((x) => x.key === "petitions")?.error).toBeNull();
    expect(nextToLoad(s)).toBe("petitions");
  });
});
