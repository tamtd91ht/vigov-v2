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

import { SoPhanAnh } from "./so-phan-anh";

const render = () => renderToStaticMarkup(<SoPhanAnh />);

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
      expect(render()).not.toContain('aria-haspopup="dialog"');
    }
  });
});

describe("KPI cards — `feedback.read` AND `report.read`", () => {
  it("both keys: the row is drawn (loading first)", () => {
    session.permissions = ["feedback.read", "report.read"];
    const html = render();
    expect(html).toContain("Đang tải số liệu phản ánh…");
    expect(html).toContain("Điểm hài lòng trung bình");
  });

  it("DENIED — one key of the two, none, or an unknown session: no card", () => {
    for (const p of [["feedback.read"], ["report.read"], [], null]) {
      session.permissions = p;
      const html = render();
      expect(html, String(p)).not.toContain("Điểm hài lòng trung bình");
      expect(html, String(p)).not.toContain("Đang tải số liệu phản ánh…");
    }
  });
});
