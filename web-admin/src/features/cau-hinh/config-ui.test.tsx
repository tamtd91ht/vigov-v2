import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { ConfigFormRow, ConfigLoading, ConfigTable, EmptyRow, RowActions, StatusBadge } from "./config-ui";

/** Every class on the first element of `html`. */
function rootClasses(html: string): string[] {
  return (html.match(/^<[a-z]+[^>]*class="([^"]*)"/)?.[1] ?? "").split(" ");
}

/**
 * Tailwind v4 emits `space-y-*` as `:where(.space-y-4 > :not(:last-child)) { margin-block-end: … }` —
 * ZERO specificity. Any margin utility on a child (`m-0`, `my-0`, `mb-0`) is a class and wins, so the
 * parent's gap silently disappears: the grey add row of Trường bản đồ, Thôn, Danh mục and Sơ đồ touched
 * the table under it (VALIDATE round 2, 08/10/2026). The shared pieces must therefore carry NO
 * block-axis margin of their own; a `<form>` has none to cancel in standards mode, and the legacy
 * sheet has no bare `form` rule.
 */
const BLOCK_MARGIN = /^(?:m|my|mt|mb)-/;

describe("config-ui pieces leave the parent's space-y gap alone", () => {
  it("ConfigFormRow sets no block-axis margin", () => {
    const html = renderToStaticMarkup(<ConfigFormRow columns="sm:grid-cols-[1fr_auto]" />);
    expect(html.startsWith("<form")).toBe(true);
    expect(rootClasses(html).filter((c) => BLOCK_MARGIN.test(c))).toEqual([]);
  });

  it("…and inside a space-y parent the rendered row keeps the gap class path (no override on the child)", () => {
    const html = renderToStaticMarkup(
      <section className="space-y-4">
        <ConfigFormRow columns="sm:grid-cols-[1fr_auto]" />
        <ConfigTable label="t">
          <tbody />
        </ConfigTable>
      </section>,
    );
    expect(html).not.toMatch(/<form[^>]*class="[^"]*\b(?:m|my|mb)-0\b/);
  });

  it("ConfigTable, RowActions, StatusBadge, EmptyRow, ConfigLoading: no block-axis margin either", () => {
    const pieces = [
      renderToStaticMarkup(
        <ConfigTable label="t">
          <tbody />
        </ConfigTable>,
      ),
      renderToStaticMarkup(<RowActions />),
      renderToStaticMarkup(<StatusBadge active />),
      renderToStaticMarkup(<ConfigLoading />),
    ];
    for (const html of pieces) expect(rootClasses(html).filter((c) => BLOCK_MARGIN.test(c))).toEqual([]);
    const row = renderToStaticMarkup(
      <table>
        <tbody>
          <EmptyRow colSpan={2}>x</EmptyRow>
        </tbody>
      </table>,
    );
    expect(row).not.toMatch(/class="[^"]*\b(?:m|my|mt|mb)-0\b/);
  });

  it("frames draw all four sides (preflight is off: a one-sided border needs `border-0` first)", () => {
    const table = rootClasses(
      renderToStaticMarkup(
        <ConfigTable label="t">
          <tbody />
        </ConfigTable>,
      ),
    );
    expect(table).toContain("border");
    expect(table).toContain("border-solid");
    const form = rootClasses(renderToStaticMarkup(<ConfigFormRow columns="" />));
    expect(form).toContain("border");
    expect(form).toContain("border-solid");
  });
});

describe("StatusBadge (user decision 09/10/2026)", () => {
  it("text-only like the prototype — no tone icon on either chip, the word tells them apart", () => {
    const on = renderToStaticMarkup(<StatusBadge active />);
    const off = renderToStaticMarkup(<StatusBadge active={false} />);
    expect(on).not.toContain("<svg");
    expect(off).not.toContain("<svg");
    expect(on).toContain("Đang dùng");
    expect(off).toContain("Ngừng dùng");
  });
});
