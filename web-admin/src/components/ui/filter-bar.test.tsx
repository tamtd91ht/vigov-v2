import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { FilterBar } from "./filter-bar";

/**
 * FilterBar holds three promises a screen relies on: a closed panel keeps its controls IN the
 * document (`hidden`, never unmounted — a filter set before closing still exists), a filter that is
 * on behind the button is said on the button and opens the panel, and `aria-expanded` tells the
 * truth about the panel.
 */
function ve(moreActiveCount: number, more: ReactNode =<select id="loc-loai" />): string {
  return renderToStaticMarkup(
    <FilterBar
      id="test-filters"
      primary={<input id="tim" type="search" />}
      more={more}
      moreActiveCount={moreActiveCount}
    />,
  );
}

function panel(html: string): string {
  const start = html.indexOf('id="test-filters-more"');
  return html.slice(html.lastIndexOf("<div", start), html.indexOf(">", start) + 1);
}

describe("FilterBar", () => {
  it("nothing on behind the button: panel closed with `hidden`, controls still in the markup", () => {
    const html = ve(0);
    expect(panel(html)).toContain('hidden=""');
    expect(html).toContain('<select id="loc-loai">');
    expect(html).toMatch(/aria-expanded="false"[^>]*aria-controls="test-filters-more"/);
    expect(html).toMatch(/>Bộ lọc<svg/);
    expect(html).not.toContain("Bộ lọc ·");
  });

  it("filters on behind the button: “Bộ lọc · N”, panel open, aria-expanded true", () => {
    const html = ve(2);
    expect(html).toContain("Bộ lọc · 2");
    expect(panel(html)).not.toContain("hidden");
    expect(html).toMatch(/aria-expanded="true"[^>]*aria-controls="test-filters-more"/);
  });

  it("the search box comes before the toggle, the toggle before the panel", () => {
    const html = ve(0);
    expect(html.indexOf('id="tim"')).toBeLessThan(html.indexOf('id="test-filters-toggle"'));
    expect(html.indexOf('id="test-filters-toggle"')).toBeLessThan(html.indexOf('id="test-filters-more"'));
  });

  it("the toggle is a plain button that submits nothing", () => {
    expect(ve(0)).toMatch(/<button[^>]*id="test-filters-toggle"[^>]*type="button"/);
  });

  it("no `more`: no toggle and no panel", () => {
    const html = renderToStaticMarkup(
      <FilterBar id="test-filters" primary={<input id="tim" />} moreActiveCount={0} />,
    );
    expect(html).not.toContain("test-filters-toggle");
    expect(html).not.toContain("test-filters-more");
    expect(html).toContain('id="tim"');
  });
});
