import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  diaChiTinXa,
  NEWS_CATEGORIES_PATH,
  newsCategoriesAddress,
  type NewsCategory,
  readNewsCategories,
} from "../api/hop-dong-cong-khai";
import { XA_TN } from "./noi-dung";
import { CategoryChips, type CategoryRows, groupCategories } from "./TinTucAppXa";

/**
 * Card D2 (owner decisions 30/09/2026): two-level category chips above the news list, from
 * `GET /commune-news/categories` (comms 58abea4c) and the list's new `?category=`.
 */

const DOMAIN = "xa-thu.vigov.vn";

describe("/commune-news/categories — read field by field", () => {
  const ROOT = { id: "dm-1", name: "Y tế", order: 1 };
  const CHILD = { id: "dm-2", name: "Tiêm chủng", parent_id: "dm-1", order: 2 };

  it("a root without parent_id reads as null; a child keeps its parent", () => {
    expect(readNewsCategories({ items: [ROOT, CHILD] })).toEqual([
      { id: "dm-1", name: "Y tế", parent_id: null, order: 1 },
      { id: "dm-2", name: "Tiêm chủng", parent_id: "dm-1", order: 2 },
    ]);
    expect(readNewsCategories({ items: [] })).toEqual([]);
  });

  it("malformed anywhere is malformed everywhere — the whole list is null, no row dropped silently", () => {
    for (const bad of [
      { ...ROOT, id: "" },
      { ...ROOT, id: 7 },
      { ...ROOT, name: "   " },
      { ...ROOT, name: undefined },
      { ...ROOT, order: "1" },
      { ...ROOT, order: 1.5 },
      { ...ROOT, order: undefined },
      { ...ROOT, parent_id: "" },
      { ...ROOT, parent_id: 3 },
      null,
      "dm-1",
    ]) {
      expect(readNewsCategories({ items: [CHILD, bad] }), JSON.stringify(bad)).toBeNull();
    }
    expect(readNewsCategories({ items: [ROOT, { ...CHILD, id: "dm-1" }] }), "repeated id").toBeNull();
    expect(readNewsCategories({})).toBeNull();
    expect(readNewsCategories(null)).toBeNull();
  });
});

describe("addresses — category only when set", () => {
  it("the list sends `category` only when a category is chosen", () => {
    expect(new URL(diaChiTinXa(DOMAIN, "", "tin-tuc")).searchParams.has("category")).toBe(false);
    expect(new URL(diaChiTinXa(DOMAIN, "", "tin-tuc", "")).searchParams.has("category")).toBe(false);
    const withCategory = new URL(diaChiTinXa(DOMAIN, "", "tin-tuc", "dm-1"));
    expect(withCategory.searchParams.get("category")).toBe("dm-1");
    expect(withCategory.searchParams.get("type")).toBe("tin-tuc");
    expect(withCategory.searchParams.get("host")).toBe(DOMAIN);
  });

  it("the categories route carries host and type, nothing else", () => {
    const url = new URL(newsCategoriesAddress(DOMAIN, "su-kien"));
    expect(url.pathname).toBe(NEWS_CATEGORIES_PATH);
    expect([...url.searchParams.keys()].sort()).toEqual(["host", "type"]);
    expect([...new URL(newsCategoriesAddress(DOMAIN)).searchParams.keys()]).toEqual(["host"]);
  });
});

const cat = (id: string, name: string, order: number, parent_id: string | null = null): NewsCategory => ({
  id,
  name,
  order,
  parent_id,
});

describe("groupCategories — roots, direct children, orphans not shown", () => {
  const list = [
    cat("a", "Y tế", 1),
    cat("a1", "Tiêm chủng", 2, "a"),
    cat("a1x", "Mũi nhắc lại", 3, "a1"), // grandchild: not in row 2, filtered through a1 by the server
    cat("b", "Giáo dục", 4),
    cat("o", "Mồ côi", 5, "missing"), // parent absent from the list
    cat("o1", "Con của mồ côi", 6, "o"),
  ];
  const rows = groupCategories(list);

  it("roots are the categories without a parent, in order", () => {
    expect(rows.roots.map((r) => r.id)).toEqual(["a", "b"]);
  });

  it("each root lists only its DIRECT children", () => {
    expect(rows.children.get("a")!.map((c) => c.id)).toEqual(["a1"]);
    expect(rows.children.get("b")).toEqual([]);
  });

  it("an orphan and anything under it appear nowhere", () => {
    const shown = [...rows.roots, ...[...rows.children.values()].flat()].map((c) => c.id);
    expect(shown).not.toContain("o");
    expect(shown).not.toContain("o1");
  });

  it("orders by `order` even if the server did not", () => {
    const r = groupCategories([cat("b", "B", 2), cat("a2", "A2", 5, "a"), cat("a", "A", 1), cat("a1", "A1", 3, "a")]);
    expect(r.roots.map((c) => c.id)).toEqual(["a", "b"]);
    expect(r.children.get("a")!.map((c) => c.id)).toEqual(["a1", "a2"]);
  });
});

describe("CategoryChips — markup", () => {
  const rows: CategoryRows = groupCategories([
    cat("a", "Y tế", 1),
    cat("a1", "Tiêm chủng", 2, "a"),
    cat("b", "Giáo dục", 3),
  ]);
  const render = (root: string | null, child: string | null, r: CategoryRows | null = rows) =>
    renderToStaticMarkup(createElement(CategoryChips, { rows: r, root, child, onRoot: () => {}, onChild: () => {} }));
  const pressed = (html: string) =>
    [...html.matchAll(/<button[^>]*aria-pressed="(true|false)"[^>]*>([^<]*)</g)].map((m) => [m[2], m[1]]);

  it("no rows, a failed load (null) or only orphans → no chip area at all", () => {
    expect(render(null, null, null)).toBe("");
    expect(render(null, null, groupCategories([]))).toBe("");
    expect(render(null, null, groupCategories([cat("o", "Mồ côi", 1, "missing")]))).toBe("");
  });

  it("row 1 is 'Tất cả' + the roots; 'Tất cả' pressed by default; no row 2", () => {
    const html = render(null, null);
    expect(html).toContain(`role="group" aria-label="${XA_TN.news_category_filter}"`);
    expect(pressed(html)).toEqual([
      [XA_TN.loc_tat_ca, "true"],
      ["Y tế", "false"],
      ["Giáo dục", "false"],
    ]);
    expect(html).not.toContain("xa-chips--phu");
  });

  it("a root with children opens row 2: 'Tất cả mục này' + its direct children, subordinate", () => {
    const html = render("a", null);
    expect(html).toContain(`aria-label="${XA_TN.news_subcategory_filter("Y tế")}"`);
    expect(pressed(html)).toEqual([
      [XA_TN.loc_tat_ca, "false"],
      ["Y tế", "true"],
      ["Giáo dục", "false"],
      [XA_TN.news_category_all_in_root, "true"],
      ["Tiêm chủng", "false"],
    ]);
    expect(html).toContain('class="xa-chips xa-chips--phu"');
    expect(html).toContain('class="xa-chip xa-chip--phu xa-chip--on"');
    expect(XA_TN.news_category_all_in_root).toBe("Tất cả mục này");
  });

  it("the chosen child is pressed, and marked by class as well as aria (never colour alone)", () => {
    const html = render("a", "a1");
    expect(pressed(html).slice(3)).toEqual([
      [XA_TN.news_category_all_in_root, "false"],
      ["Tiêm chủng", "true"],
    ]);
    expect(html).toMatch(/class="xa-chip xa-chip--phu xa-chip--on" aria-pressed="true">Tiêm chủng</);
  });

  it("a root without children shows no row 2", () => {
    expect(render("b", null)).not.toContain("xa-chips--phu");
  });

  it("every chip is a real button", () => {
    expect(render("a", null).match(/<button type="button"/g)).toHaveLength(5);
  });
});
