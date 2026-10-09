import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The register's two new header pieces are gated by the SESSION'S keys — UX only, both routes check
 * them (rule 5, forbidden #1). Pinned in both directions: the account that writes the code has every
 * key, so a piece leaking past its gate is invisible while building.
 *
 * `usePhien` is replaced FOR THIS FILE; effects do not run under `renderToStaticMarkup`, so nothing
 * reaches the network.
 */
const session = vi.hoisted(() => ({ permissions: [] as string[] | null }));

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () =>
    session.permissions === null ? null : { ok: true, duLieu: { permissions: session.permissions } },
}));

// The header button lives in the page frame (`PetitionWorkspace`, the prototype's header row).
import { PetitionWorkspace } from "./petition-workspace";

const render = () => renderToStaticMarkup(<PetitionWorkspace />);

// `SoPhanAnh` starts reads on mount; under `renderToStaticMarkup` effects never run, so no stub is needed.

/** The button itself — NOT the words, which also appear in the "not built" list (§11's hamlet/photos). */
const INTAKE_BUTTON = /<button[^>]*aria-haspopup="dialog"[^>]*>.*?Nhập hộ phản ánh<\/button>/;

beforeEach(() => {
  session.permissions = [];
});

describe("`+ Nhập hộ phản ánh` — `feedback.create`", () => {
  it("with the key: the button", () => {
    session.permissions = ["feedback.read", "feedback.create"];
    expect(render()).toMatch(INTAKE_BUTTON);
  });

  it("DENIED — without it, or with the session unknown: no button", () => {
    for (const p of [["feedback.read", "feedback.resolve", "report.read"], null]) {
      session.permissions = p;
      expect(render()).not.toMatch(INTAKE_BUTTON);
      // Every dialog trigger left must be a placeholder's "?" (ADR 0068 §14), never the intake button.
      const triggers = render().match(/<button[^>]*aria-haspopup="dialog"[^>]*>/g) ?? [];
      for (const tag of triggers) expect(tag).toContain("data-pending-marker");
    }
  });
});

describe("KPI cards — `feedback.read` AND `report.read`", () => {
  it("both keys: the row is drawn (loading first: the sentence and the prototype's one block)", () => {
    session.permissions = ["feedback.read", "report.read"];
    const html = render();
    expect(html).toContain("Đang tải số liệu phản ánh…");
    expect(html).toContain('aria-label="Số liệu phản ánh"');
  });

  it("DENIED — one key of the two, none, or an unknown session: no card", () => {
    for (const p of [["feedback.read"], ["report.read"], [], null]) {
      session.permissions = p;
      const html = render();
      expect(html, String(p)).not.toContain('aria-label="Số liệu phản ánh"');
      expect(html, String(p)).not.toContain("Đang tải số liệu phản ánh…");
    }
  });
});

describe("the page frame — `feedback.read`", () => {
  it("DENIED — without it: the header still names the screen, the register is not drawn", () => {
    session.permissions = ["feedback.create"];
    const html = render();
    expect(html).toContain("<h1");
    expect(html).toContain("Phản ánh của người dân");
    expect(html).toContain("feedback.read");
    expect(html).not.toContain("Sổ phản ánh của xã");
    expect(html).not.toContain('id="petition-filters"');
  });

  it("the prototype's header: plain title, verbatim subtitle, mb-5, no icon tile", () => {
    session.permissions = ["feedback.read"];
    const html = render();
    expect(html).toMatch(/<header class="[^"]*mb-5/);
    expect(html).toContain(">Phản ánh của người dân</h1>");
    expect(html).toContain(
      "Tiếp nhận từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh trước và sau khi xử lý.",
    );
    expect(html).not.toContain("lucide-message-square-warning");
  });
});

/**
 * Owner decision D3 (09/10/2026): an account WITHOUT `feedback.restricted` reads the prototype's note
 * (`FeedbackWorkspace.tsx:125-131`), FIRST SENTENCE ONLY — the key can be granted to others than the
 * chairman, so "Chỉ Chủ tịch Uỷ ban đọc được." would be false. A holder of the key sees the petitions
 * themselves and no note.
 */
describe("restricted-flow note — accounts without `feedback.restricted`", () => {
  const NOTE = "Phản ánh về tác phong cán bộ đi luồng riêng và không hiển thị ở đây.";

  it("without the key: the one sentence, never the chairman sentence", () => {
    session.permissions = ["feedback.read"];
    const html = render();
    expect(html).toContain(NOTE);
    expect(html).toContain("lucide-shield-alert");
    expect(html).not.toContain("Chủ tịch Uỷ ban đọc được");
  });

  it("DENIED direction — with the key: no note", () => {
    session.permissions = ["feedback.read", "feedback.restricted"];
    expect(render()).not.toContain(NOTE);
  });

  it("without `feedback.read`: no register, so no note either", () => {
    session.permissions = ["feedback.create"];
    expect(render()).not.toContain(NOTE);
  });
});
