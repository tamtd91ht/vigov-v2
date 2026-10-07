// @vitest-environment jsdom
//
// jsdom for this file: recording, resolving and commenting are events whose request bodies and the
// re-reads that follow are what is under test — not a markup string.

import { act, useState, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { toast } from "sonner";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  finance_duAnRa,
  finance_projectCommentOut,
  finance_projectIssueOut,
  identity_canBoChonNguoiRa,
} from "@/lib/api/schema.gen";

import { BangDanhSach, LatestIssueCell } from "./bang-du-an";
import { ChiTietDuAn } from "./chi-tiet-du-an";
import { CommentList, ProjectCommentsPanel } from "./project-comments";
import { latestIssueDateLine, MENTION_PICKER_LIMIT, mentionLine, openIssuesLabel, staffLabel } from "./project-discussion-labels";
import { ProjectIssuesPanel, useProjectIssues } from "./project-issues";
import { staffCatalogue, type PeopleCatalogue } from "./project-people";

/**
 * §8.1 Vướng mắc, §8.4 Trao đổi, §7.2 `Vướng mắc mới nhất` and §3's issue count (889d4598).
 * The silent failures: a write that leaves the timeline stale, a 409 swallowed, a form shown to an
 * account that cannot write, a mention whose code never leaves the browser, a body rendered as HTML.
 */

// Outcomes are toasts (ADR 0068 lần 6 #4).
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
});

const fakeSession = {
  ok: true as const,
  duLieu: { permissions: ["budget.read", "budget.update"] as string[] },
};
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
  PhienProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

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
  fakeSession.duLieu.permissions = ["budget.read", "budget.update"];
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

async function settle(): Promise<void> {
  for (let i = 0; i < 4; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function type(el: HTMLTextAreaElement, value: string): void {
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement {
  const b = [...el.querySelectorAll("button")].find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const STAFF_ITEMS: identity_canBoChonNguoiRa[] = [
  { code: "CB-00001", full_name: "Nguyễn Văn An", position: "Kế toán", department_id: "" },
  { code: "CB-00002", full_name: "Trần Thị Bình", position: "Chủ tịch", department_id: "" },
  { code: "CB-00003", full_name: "Nguyễn Văn A", position: "", department_id: "" },
];
const STAFF = staffCatalogue({ ok: true, duLieu: { items: STAFF_ITEMS } });
const LOADING: PeopleCatalogue<identity_canBoChonNguoiRa> = { phase: "loading" };

function issue(over: Partial<finance_projectIssueOut> = {}): finance_projectIssueOut {
  return {
    id: "VM1",
    project_id: "DA1",
    title: "Chờ Sở thẩm định thiết kế",
    recorded_by: "CB-00001",
    // 02:05 UTC = 09:05 in Hà Nội.
    recorded_at: "2026-08-27T02:05:00Z",
    resolved: false,
    ...over,
  };
}

function comment(over: Partial<finance_projectCommentOut> = {}): finance_projectCommentOut {
  return {
    id: "C1",
    project_id: "DA1",
    body: "Ý kiến",
    author_code: "CB-00002",
    mentioned_staff_codes: [],
    created_at: "2026-08-27T02:05:00Z",
    ...over,
  };
}

type Seen = { method: string; url: string; body: Record<string, unknown> | null; key: string | null };

/** Fake finance server for one project: `issues` / `comments` are what the read routes return NOW. */
function stubServer(state: {
  issues: finance_projectIssueOut[];
  comments?: finance_projectCommentOut[];
  writeReply?: () => Response;
}): Seen[] {
  const seen: Seen[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      seen.push({
        method,
        url,
        body: init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : null,
        key: new Headers(init?.headers).get("Idempotency-Key"),
      });
      if (method === "GET" && url === "/api/v1/investment-projects/DA1/issues") {
        const open = state.issues.filter((i) => !i.resolved).length;
        return json(200, { project_id: "DA1", items: state.issues, count: state.issues.length, open_count: open });
      }
      if (method === "GET" && url === "/api/v1/investment-projects/DA1/comments") {
        const items = state.comments ?? [];
        return json(200, { project_id: "DA1", items, count: items.length });
      }
      if (method === "GET" && url === "/api/v1/investment-projects/DA1") return json(200, PROJECT);
      if (method === "GET" && url === "/api/v1/investment-projects/DA1/disbursements") {
        return json(200, { project_id: "DA1", items: [], count: 0 });
      }
      if (method === "GET" && url.startsWith("/api/v1/staff-directory")) return json(200, { items: STAFF_ITEMS });
      if (method === "GET" && url.startsWith("/api/v1/org-units")) return json(200, { items: [] });
      if (method === "POST") return state.writeReply?.() ?? json(url.endsWith("/resolution") ? 200 : 201, {});
      return new Response("unexpected", { status: 500 });
    }),
  );
  return seen;
}

const reads = (seen: Seen[], suffix: string) => seen.filter((s) => s.method === "GET" && s.url.endsWith(suffix)).length;

/** The panel as the page drives it: its own reload count, re-read on `onChanged`. */
function IssuesHarness({ canRecord, staff = STAFF }: { canRecord: boolean; staff?: PeopleCatalogue<identity_canBoChonNguoiRa> }) {
  const [n, setN] = useState(0);
  const issues = useProjectIssues("DA1", String(n));
  return (
    <ProjectIssuesPanel projectId="DA1" issues={issues} staff={staff} canRecord={canRecord} onChanged={() => setN((x) => x + 1)} />
  );
}

/* ── Pure rules ─────────────────────────────────────────────────────────────────────────────── */

describe("labels", () => {
  it("staffLabel: the directory's name, else the code itself — never blank", () => {
    expect(staffLabel("CB-00001", STAFF)).toBe("Nguyễn Văn An");
    expect(staffLabel("CB-99999", STAFF)).toBe("CB-99999");
    expect(staffLabel("CB-00001", LOADING)).toBe("CB-00001");
  });

  it("§7.2 date line: the commune's calendar day, unpadded, '· đã gỡ' when resolved", () => {
    // 23:30 UTC on the 26th is already the 27th in Hà Nội.
    const base = { id: "x", text: "t", recorded_at: "2026-08-26T23:30:00Z" };
    expect(latestIssueDateLine({ ...base, resolved: true })).toBe("27/8/2026 · đã gỡ");
    expect(latestIssueDateLine({ ...base, resolved: false })).toBe("27/8/2026");
  });

  it("§3 count: 0 is '0', absent is NOT 0", () => {
    expect(openIssuesLabel(3)).toBe("3 vướng mắc đang theo dõi");
    expect(openIssuesLabel(0)).toBe("0 vướng mắc đang theo dõi");
    expect(openIssuesLabel(undefined)).toBe("Chưa đọc được số vướng mắc");
    expect(openIssuesLabel(null)).toBe("Chưa đọc được số vướng mắc");
  });
});

describe("mentions — pure rules", () => {
  it("spec 07 'Nhắc: @A, @B' line: directory names, the code when unresolved, nothing when nobody", () => {
    expect(mentionLine(["CB-00001", "CB-99999"], STAFF)).toBe("Nhắc: @Nguyễn Văn An, @CB-99999");
    expect(mentionLine([], STAFF)).toBeNull();
  });
});

/* ── §8.1 Vướng mắc ─────────────────────────────────────────────────────────────────────────── */

describe("§8.1 issues — timeline", () => {
  it("renders who · HH:mm dd/MM/yyyy · chip + strike-through when resolved; 'Đã gỡ xong' only on open ones", async () => {
    stubServer({
      issues: [
        issue({ id: "OPEN", recorded_by: "CB-99999" }),
        issue({
          id: "DONE",
          title: "Vướng giải phóng mặt bằng",
          description: "Hộ dân chưa đồng ý",
          resolved: true,
          resolved_at: "2026-09-01T03:00:00Z",
          resolved_by: "CB-00002",
        }),
      ],
    });
    const el = mount(<IssuesHarness canRecord />);
    await settle();

    const open = el.querySelector<HTMLElement>('[data-issue="OPEN"]')!;
    // A code the directory does not list is shown AS the code — the accountable "who" is never blank.
    expect(open.textContent).toContain("CB-99999");
    expect(open.querySelector("time")?.textContent).toBe("09:05 27/08/2026");
    expect(open.textContent).not.toContain("Đã gỡ lúc");
    expect(buttonByText(open, "Đã gỡ xong").disabled).toBe(false);

    const done = el.querySelector<HTMLElement>('[data-issue="DONE"]')!;
    expect(done.hasAttribute("data-resolved")).toBe(true);
    expect(done.textContent).toContain("Nguyễn Văn An");
    expect(done.textContent).toContain("Đã gỡ");
    expect(done.querySelector("p.line-through")?.textContent).toBe("Vướng giải phóng mặt bằng");
    // Spec 07: no "Gỡ lúc … · người gỡ" line (row D31).
    expect(done.querySelector("[data-resolved-by]")).toBeNull();
    expect([...done.querySelectorAll("button")].some((b) => b.textContent?.includes("Đã gỡ xong"))).toBe(false);
  });

  it("an empty project says so; a failed read is an error with retry, never 'no issues'", async () => {
    stubServer({ issues: [] });
    const empty = mount(<IssuesHarness canRecord />);
    await settle();
    expect(empty.textContent).toContain("Chưa ghi nhận vướng mắc nào ở dự án này.");
    act(() => root?.unmount());
    host?.remove();

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => json(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." })),
    );
    const failed = mount(<IssuesHarness canRecord />);
    await settle();
    expect(failed.textContent).toContain("Chưa tải được vướng mắc của dự án");
    expect(failed.textContent).toContain("Bạn không có quyền thực hiện thao tác này.");
    expect(failed.textContent).not.toContain("Chưa ghi nhận vướng mắc nào");
  });
});

describe("§8.1 issues — record and resolve", () => {
  it("records the text as typed with an Idempotency-Key, then RE-READS the timeline and clears the box", async () => {
    const state = { issues: [] as finance_projectIssueOut[] };
    const seen = stubServer(state);
    const el = mount(<IssuesHarness canRecord />);
    await settle();
    expect(reads(seen, "/issues")).toBe(1);

    const box = el.querySelector<HTMLTextAreaElement>("#vuong-mac-moi")!;
    expect(box.placeholder).toBe("Vướng mắc đang gặp ở dự án này…");
    const send = buttonByText(el, "Ghi nhận");
    expect(send.disabled).toBe(true);
    type(box, "Chờ Sở thẩm định\nHồ sơ nộp 20/8");
    expect(send.disabled).toBe(false);

    state.issues = [issue({ id: "NEW", title: "Chờ Sở thẩm định" })];
    await act(async () => send.click());
    await settle();

    const post = seen.find((s) => s.method === "POST")!;
    expect(post.url).toBe("/api/v1/investment-projects/DA1/issues");
    expect(post.body).toEqual({ text: "Chờ Sở thẩm định\nHồ sơ nộp 20/8" });
    expect(post.key).not.toBeNull();
    expect(reads(seen, "/issues")).toBe(2);
    expect(toast.success).toHaveBeenCalledWith("Đã ghi nhận vướng mắc.");
    expect(el.querySelector('[data-issue="NEW"]')).not.toBeNull();
    expect(el.querySelector<HTMLTextAreaElement>("#vuong-mac-moi")!.value).toBe("");
  });

  it("a refused record keeps the text, shows the server's sentence, and the retry reuses the SAME key", async () => {
    const state = {
      issues: [] as finance_projectIssueOut[],
      writeReply: () => json(400, { code: "bad", message: "vuong_mac: dòng đầu của `text` quá dài (tối đa 500 ký tự)" }),
    };
    const seen = stubServer(state);
    const el = mount(<IssuesHarness canRecord />);
    await settle();
    const box = el.querySelector<HTMLTextAreaElement>("#vuong-mac-moi")!;
    type(box, "dài");
    await act(async () => buttonByText(el, "Ghi nhận").click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("Dòng đầu của `text` quá dài (tối đa 500 ký tự)");
    expect(box.value).toBe("dài");
    expect(reads(seen, "/issues")).toBe(1);

    await act(async () => buttonByText(el, "Ghi nhận").click());
    await settle();
    const posts = seen.filter((s) => s.method === "POST");
    expect(posts).toHaveLength(2);
    expect(posts[1]!.key).toBe(posts[0]!.key);
  });

  it("'Đã gỡ xong' posts the resolution with no body, then re-reads: the entry turns resolved", async () => {
    const state = { issues: [issue()] };
    const seen = stubServer(state);
    const el = mount(<IssuesHarness canRecord />);
    await settle();

    state.issues = [issue({ resolved: true, resolved_at: "2026-09-01T03:00:00Z", resolved_by: "CB-00002" })];
    await act(async () => buttonByText(el, "Đã gỡ xong").click());
    await settle();

    const post = seen.find((s) => s.method === "POST")!;
    expect(post.url).toBe("/api/v1/project-issues/VM1/resolution");
    expect(post.body).toBeNull();
    expect(reads(seen, "/issues")).toBe(2);
    expect(el.querySelector('[data-issue="VM1"]')?.hasAttribute("data-resolved")).toBe(true);
    expect(toast.success).toHaveBeenCalledWith("Đã đóng vướng mắc.");
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("409 issue_already_resolved: the server's sentence is shown, and the timeline is re-read", async () => {
    const state = {
      issues: [issue()],
      writeReply: () =>
        json(409, {
          code: "issue_already_resolved",
          message:
            "vuong_mac: vướng mắc này đã được ghi là đã gỡ — nếu vướng mắc quay lại, hãy ghi nhận một vướng mắc mới",
        }),
    };
    const seen = stubServer(state);
    const el = mount(<IssuesHarness canRecord />);
    await settle();

    state.issues = [issue({ resolved: true, resolved_at: "2026-09-01T03:00:00Z", resolved_by: "CB-00002" })];
    await act(async () => buttonByText(el, "Đã gỡ xong").click());
    await settle();

    expect(toast.error).toHaveBeenCalledWith(
      "Vướng mắc này đã được ghi là đã gỡ — nếu vướng mắc quay lại, hãy ghi nhận một vướng mắc mới",
    );
    expect(reads(seen, "/issues")).toBe(2);
    expect(el.querySelector('[data-issue="VM1"]')?.hasAttribute("data-resolved")).toBe(true);
  });

  it("DENIED (no budget.update): no form, no 'Đã gỡ xong', no denial note (spec 07) — the timeline still shows", async () => {
    const seen = stubServer({ issues: [issue()] });
    const el = mount(<IssuesHarness canRecord={false} />);
    await settle();
    expect(el.querySelector("#vuong-mac-moi")).toBeNull();
    expect([...el.querySelectorAll("button")].some((b) => b.textContent?.includes("Ghi nhận"))).toBe(false);
    expect([...el.querySelectorAll("button")].some((b) => b.textContent?.includes("Đã gỡ xong"))).toBe(false);
    expect(el.textContent).not.toContain("chưa được cấp quyền");
    expect(el.querySelector('[data-issue="VM1"]')).not.toBeNull();
    expect(seen.some((s) => s.method === "POST")).toBe(false);
  });

  it("the tracking-task note is a disabled '?' spot, never printed as something that happens", async () => {
    stubServer({ issues: [] });
    const el = mount(<IssuesHarness canRecord />);
    await settle();
    expect(el.textContent).not.toContain("hệ thống tự sinh một nhiệm vụ theo dõi");
    const spot = el.querySelector<HTMLElement>("form [data-pending]")!;
    expect(spot.textContent).toContain("Tự sinh nhiệm vụ theo dõi");
    expect(spot.querySelector("button[data-pending-marker]")).not.toBeNull();
  });
});

/* ── §8.4 Trao đổi ──────────────────────────────────────────────────────────────────────────── */

describe("§8.4 discussion", () => {
  it("lists oldest first with the author's name and time; mentions as 'Nhắc: …'; NEVER raw HTML", () => {
    const html = renderToStaticMarkup(
      <CommentList
        staff={STAFF}
        items={[
          comment({ id: "A", body: "Nhờ xem <img src=x onerror=alert(1)>", mentioned_staff_codes: ["CB-00001"] }),
          comment({ id: "B", body: "Đã xem", author_code: "CB-00001" }),
        ]}
      />,
    );
    expect(html).toContain("Trần Thị Bình");
    expect(html).toContain("09:05 27/08/2026");
    expect(html).toMatch(/<p[^>]*data-mentions=""[^>]*>Nhắc: @Nguyễn Văn An<\/p>/);
    // Escaped: the body is text, so a tag in it is shown, never parsed.
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(html).not.toContain("<img");
    expect(html.indexOf('data-comment="A"')).toBeLessThan(html.indexOf('data-comment="B"'));
  });

  it("spec 07 chips: the first eight staff under the box; a chip toggles, and the chips ON are the codes SENT", async () => {
    const state = { issues: [] as finance_projectIssueOut[], comments: [] as finance_projectCommentOut[] };
    const seen = stubServer(state);
    const many = staffCatalogue({
      ok: true,
      duLieu: { items: Array.from({ length: 10 }, (_, k) => ({ ...STAFF_ITEMS[0]!, code: `CB-1000${k}`, full_name: `Cán bộ ${k}` })) },
    });
    const el = mount(<ProjectCommentsPanel projectId="DA1" staff={many} canComment />);
    await settle();
    expect(el.textContent).toContain("Chưa có ý kiến trao đổi nào.");
    const box = el.querySelector<HTMLTextAreaElement>("#trao-doi-moi")!;
    expect(box.placeholder).toBe("Nhập ý kiến trao đổi về dự án này…");

    const picker = el.querySelector<HTMLElement>("[data-mention-picker]")!;
    expect(picker.querySelectorAll("button")).toHaveLength(MENTION_PICKER_LIMIT);
    const chip = (code: string) => picker.querySelector<HTMLButtonElement>(`[data-staff-code="${code}"]`)!;
    act(() => chip("CB-10002").click());
    act(() => chip("CB-10005").click());
    act(() => chip("CB-10005").click()); // toggled off again
    expect(chip("CB-10002").getAttribute("aria-pressed")).toBe("true");
    expect(chip("CB-10005").getAttribute("aria-pressed")).toBe("false");

    type(box, "Nhờ xem hồ sơ");
    state.comments = [comment({ id: "NEW", body: "Nhờ xem hồ sơ", mentioned_staff_codes: ["CB-10002"] })];
    await act(async () => buttonByText(el, "Gửi").click());
    await settle();

    const post = seen.find((s) => s.method === "POST")!;
    expect(post.url).toBe("/api/v1/investment-projects/DA1/comments");
    expect(post.body).toEqual({ body: "Nhờ xem hồ sơ", mentioned_staff_codes: ["CB-10002"] });
    expect(post.key).not.toBeNull();
    // Re-read after the write; the box and the chips are cleared.
    expect(reads(seen, "/comments")).toBe(2);
    expect(el.querySelector('[data-comment="NEW"] [data-mentions]')?.textContent).toBe("Nhắc: @Cán bộ 2");
    expect(box.value).toBe("");
    expect(chip("CB-10002").getAttribute("aria-pressed")).toBe("false");
  });

  it("a refused post keeps the text and shows the server's sentence", async () => {
    stubServer({
      issues: [],
      comments: [],
      writeReply: () => json(400, { code: "bad", message: "trao_doi: `body` quá dài (tối đa 4000 ký tự)" }),
    });
    const el = mount(<ProjectCommentsPanel projectId="DA1" staff={STAFF} canComment />);
    await settle();
    const box = el.querySelector<HTMLTextAreaElement>("#trao-doi-moi")!;
    type(box, "dài");
    await act(async () => buttonByText(el, "Gửi").click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("`body` quá dài (tối đa 4000 ký tự)");
    expect(box.value).toBe("dài");
  });

  it("the mention notification is a disabled '?' spot — nothing claims anyone was notified", async () => {
    stubServer({ issues: [], comments: [] });
    const el = mount(<ProjectCommentsPanel projectId="DA1" staff={STAFF} canComment />);
    await settle();
    const spot = el.querySelector<HTMLElement>("form [data-pending]")!;
    expect(spot.textContent).toContain("Thông báo cho người được nhắc tên — chưa có");
    expect(spot.querySelector("button[data-pending-marker]")).not.toBeNull();
  });

  it("DENIED (no budget.read): no composer, no denial note (spec 07)", async () => {
    const seen = stubServer({ issues: [], comments: [comment()] });
    const el = mount(<ProjectCommentsPanel projectId="DA1" staff={STAFF} canComment={false} />);
    await settle();
    expect(el.querySelector("#trao-doi-moi")).toBeNull();
    expect(el.textContent).not.toContain("chưa được cấp quyền");
    expect(seen.some((s) => s.method === "POST")).toBe(false);
  });
});

/* ── The page: tab count and gating from the session ────────────────────────────────────────── */

const PROJECT: finance_duAnRa = {
  id: "DA1",
  code: "DA01",
  year: 2026,
  category_id: "",
  name: "Bê tông hoá đường trục chính",
  planned_amount: 1_000_000_000,
  approved_amount: 1_000_000_000,
  disbursed_amount: 0,
  remaining_amount: 1_000_000_000,
  disbursed_ratio: 0,
  delay_score: null,
  is_delayed: false,
  disbursement_deadline: "2026-12-31",
  delay_threshold: 1000,
  delay_threshold_source: "mac_dinh",
};

describe("project page", () => {
  it("Vướng mắc is the open tab and carries the server's open_count", async () => {
    stubServer({ issues: [issue({ id: "A" }), issue({ id: "B", resolved: true }), issue({ id: "C" })] });
    const el = mount(<ChiTietDuAn id="DA1" />);
    await settle();
    expect(el.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toBe("Vướng mắc (2)");
    expect(el.querySelector("#vuong-mac-moi")).not.toBeNull();
  });

  it("an account with budget.read only: timeline yes, issue form no, comment composer yes", async () => {
    fakeSession.duLieu.permissions = ["budget.read"];
    stubServer({ issues: [issue()] });
    const el = mount(<ChiTietDuAn id="DA1" />);
    await settle();
    expect(el.querySelector('[data-issue="VM1"]')).not.toBeNull();
    expect(el.querySelector("#vuong-mac-moi")).toBeNull();
    expect(el.querySelector("#trao-doi-moi")).not.toBeNull();
  });
});

/* ── §7.2 list column ───────────────────────────────────────────────────────────────────────── */

describe("§7.2 'Vướng mắc mới nhất'", () => {
  it("text + 'd/M/yyyy · đã gỡ' when resolved, the day alone when open, '—' when none", () => {
    const resolved = renderToStaticMarkup(
      <LatestIssueCell issue={{ id: "1", text: "Chờ thẩm định", recorded_at: "2026-08-27T02:00:00Z", resolved: true }} />,
    );
    expect(resolved).toContain("Chờ thẩm định");
    expect(resolved).toContain("27/8/2026 · đã gỡ");
    expect(resolved).toContain('data-latest-issue="resolved"');

    const open = renderToStaticMarkup(
      <LatestIssueCell issue={{ id: "1", text: "Chờ thẩm định", recorded_at: "2026-08-27T02:00:00Z", resolved: false }} />,
    );
    expect(open).toContain('data-latest-issue="open"');
    expect(open.replace(/<[^>]*>/g, "")).toBe("Chờ thẩm định27/8/2026");

    expect(renderToStaticMarkup(<LatestIssueCell issue={undefined} />).replace(/<[^>]*>/g, "")).toBe("—");
  });

  it("the table fills the last column from each row's latest_issue — no '?' left in the header", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={{
          year: 2026,
          delay_threshold: 1000,
          delay_threshold_source: "mac_dinh",
          items: [
            { ...PROJECT, latest_issue: { id: "1", text: "Vướng mặt bằng", recorded_at: "2026-08-27T02:00:00Z", resolved: false } },
            { ...PROJECT, id: "DA2", code: "DA02" },
          ],
        }}
        danhMuc={[]}
      />,
    );
    expect(html).toContain("Vướng mặt bằng");
    expect(html).not.toContain("data-pending");
    const rows = html.split("<tr").filter((r) => r.includes("<td"));
    expect(rows).toHaveLength(2);
    expect(rows[1]!.replace(/<[^>]*>/g, "").endsWith("—")).toBe(true);
  });
});
