import { Pencil } from "lucide-react";
import { isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { Banner } from "./banner";
import { Breadcrumb } from "./breadcrumb";
import { DATA_TABLE_CLASS, TableScroll } from "./data-table";
import { EmptyState } from "./empty-state";
import { ErrorState, RETRY_LABEL } from "./error-state";
import { Toolbar } from "./field";
import { NO_ACCESS_HOME_LABEL, NO_ACCESS_TITLE, NoAccess } from "./no-access";
import { PageHeader, PageHeaderMeta } from "./page-header";
import { NEXT_LABEL, PREV_LABEL, Pagination } from "./pagination";
import { Skeleton, SkeletonRows } from "./skeleton";

/**
 * Shared state components (spec v2 §7, §8b). They are presentational: what is pinned is what a
 * screen relies on when it adopts one — the screen's handler reaches the native control, a control
 * that cannot act is disabled rather than missing, server text is printed verbatim, nothing is
 * announced unless the screen asks, and nothing is invented (no total count, no guessed role).
 */

/** Depth-first walk of an UNRENDERED element tree, expanding function components with no hooks. */
function findAll(node: ReactNode, pick: (el: ReactElement<Record<string, unknown>>) => boolean): ReactElement<Record<string, unknown>>[] {
  const found: ReactElement<Record<string, unknown>>[] = [];
  const walk = (n: ReactNode) => {
    if (Array.isArray(n)) return n.forEach(walk);
    if (!isValidElement<Record<string, unknown>>(n)) return;
    if (pick(n)) found.push(n);
    if (typeof n.type === "function") return walk((n.type as (p: unknown) => ReactNode)(n.props));
    walk(n.props.children as ReactNode);
  };
  walk(node);
  return found;
}

const buttons = (tree: ReactNode) => findAll(tree, (el) => el.type === "button");

describe("Skeleton / SkeletonRows", () => {
  it("is hidden from assistive tech — the screen's own status sentence announces the load", () => {
    expect(renderToStaticMarkup(<Skeleton />)).toMatch(/^<span aria-hidden="true"/);
    expect(renderToStaticMarkup(<SkeletonRows />)).toMatch(/^<div aria-hidden="true"/);
  });

  it("draws the requested rows and columns, the same markup every render (no randomness)", () => {
    const a = renderToStaticMarkup(<SkeletonRows rows={3} columns={2} />);
    expect(a.match(/h-\[var\(--row-h\)\]/g)).toHaveLength(3);
    expect(a.match(/<span /g)).toHaveLength(6);
    expect(renderToStaticMarkup(<SkeletonRows rows={3} columns={2} />)).toBe(a);
  });
});

describe("ErrorState", () => {
  it("prints the server's message verbatim under the title", () => {
    const html = renderToStaticMarkup(
      <ErrorState title="Chưa tải được danh sách" message="ngan_sach: không có bảng ngân sách này trong xã" />,
    );
    expect(html).toContain("Chưa tải được danh sách");
    expect(html).toContain("ngan_sach: không có bảng ngân sách này trong xã");
    expect(html).toContain("lucide-cloud-off");
  });

  it("no onRetry: no button — a reload that reloads nothing is not drawn", () => {
    const html = renderToStaticMarkup(<ErrorState title="Lỗi" />);
    expect(html).not.toContain("<button");
    expect(html).not.toContain(RETRY_LABEL);
  });

  it("onRetry: a type=button 'Tải lại' carrying exactly the screen's handler", () => {
    const onRetry = vi.fn();
    const tree = ErrorState({ title: "Lỗi", onRetry });
    const [b] = buttons(tree);
    expect(b).toBeDefined();
    expect(b!.props.type).toBe("button");
    (b!.props.onClick as () => void)();
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(renderToStaticMarkup(tree)).toContain(RETRY_LABEL);
  });

  it("announces nothing unless the screen passes a role", () => {
    expect(renderToStaticMarkup(<ErrorState title="Lỗi" />)).not.toContain("role=");
    expect(renderToStaticMarkup(<ErrorState title="Lỗi" role="alert" />)).toContain('role="alert"');
  });
});

describe("NoAccess", () => {
  it("without a role: the sentence and the way back, no role line, no guessed role", () => {
    for (const roleName of [undefined, null, "", "   "]) {
      const html = renderToStaticMarkup(<NoAccess roleName={roleName} />);
      expect(html).toContain(NO_ACCESS_TITLE);
      expect(html).not.toContain("no-access-role");
      expect(html).not.toContain("Vai trò");
      expect(html).toContain('href="/tong-quan"');
      expect(html).toContain(NO_ACCESS_HOME_LABEL);
    }
  });

  it("with a role: names the current role", () => {
    const html = renderToStaticMarkup(<NoAccess roleName="Văn thư" />);
    expect(html).toContain("no-access-role");
    expect(html).toContain("Văn thư");
  });

  it("the way back is a link (it navigates), and its target can be changed", () => {
    const html = renderToStaticMarkup(<NoAccess homeHref="/doi-mat-khau" />);
    expect(html).toMatch(/<a [^>]*href="\/doi-mat-khau"/);
    expect(html).not.toContain("<button");
  });
});

describe("Banner", () => {
  it("every tone carries an icon and words, never colour alone", () => {
    for (const tone of ["info", "warning", "offline"] as const) {
      const html = renderToStaticMarkup(<Banner tone={tone}>Mất kết nối mạng.</Banner>);
      expect(html).toContain("<svg");
      expect(html).toContain("Mất kết nối mạng.");
    }
    expect(renderToStaticMarkup(<Banner tone="offline">x</Banner>)).toContain("lucide-wifi-off");
  });

  it("role passes through; none by default", () => {
    expect(renderToStaticMarkup(<Banner>x</Banner>)).not.toContain("role=");
    expect(renderToStaticMarkup(<Banner role="status">x</Banner>)).toContain('role="status"');
  });
});

describe("Pagination", () => {
  const noop = () => {};

  it("first page: 'Trước' disabled, 'Sau' enabled", () => {
    const [prev, next] = buttons(Pagination({ hasPrev: false, hasNext: true, onPrev: noop, onNext: noop }));
    expect(prev!.props.disabled).toBe(true);
    expect(next!.props.disabled).toBe(false);
  });

  it("last page: 'Sau' disabled, 'Trước' enabled", () => {
    const [prev, next] = buttons(Pagination({ hasPrev: true, hasNext: false, onPrev: noop, onNext: noop }));
    expect(prev!.props.disabled).toBe(false);
    expect(next!.props.disabled).toBe(true);
  });

  it("only page: both disabled but both still drawn, so the control never jumps", () => {
    const html = renderToStaticMarkup(<Pagination hasPrev={false} hasNext={false} onPrev={noop} onNext={noop} />);
    expect(html.match(/<button[^>]*disabled=""/g)).toHaveLength(2);
    expect(html).toContain(PREV_LABEL);
    expect(html).toContain(NEXT_LABEL);
  });

  it("busy: both disabled whatever the cursors say", () => {
    const bs = buttons(Pagination({ hasPrev: true, hasNext: true, onPrev: noop, onNext: noop, busy: true }));
    expect(bs.map((b) => b.props.disabled)).toEqual([true, true]);
  });

  it("calls exactly the screen's handlers, as type=button", () => {
    const onPrev = vi.fn();
    const onNext = vi.fn();
    const [prev, next] = buttons(Pagination({ hasPrev: true, hasNext: true, onPrev, onNext }));
    expect([prev!.props.type, next!.props.type]).toEqual(["button", "button"]);
    (prev!.props.onClick as () => void)();
    (next!.props.onClick as () => void)();
    expect(onPrev).toHaveBeenCalledTimes(1);
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it("shows no page number and no total — the contract returns none", () => {
    const html = renderToStaticMarkup(<Pagination hasPrev hasNext onPrev={noop} onNext={noop} />);
    expect(html).not.toMatch(/>\s*\d+\s*</);
    expect(html).not.toContain("trên");
    expect(html).toContain('<nav aria-label="Phân trang"');
  });
});

describe("Breadcrumb", () => {
  it("links earlier levels that have a route, marks the last as the current page, never links it", () => {
    const html = renderToStaticMarkup(
      <Breadcrumb items={[{ label: "Tổng quan", href: "/tong-quan" }, { label: "Nhiệm vụ", href: "/nhiem-vu" }, { label: "Chi tiết" }]} />,
    );
    expect(html).toContain('href="/tong-quan"');
    expect(html).toContain('href="/nhiem-vu"');
    expect(html).toContain('<span aria-current="page"');
    expect(html.match(/<a /g)).toHaveLength(2);
  });

  it("a level with no route is text, not a link", () => {
    const html = renderToStaticMarkup(<Breadcrumb items={[{ label: "Cấu hình" }, { label: "Nhật ký" }]} />);
    expect(html).not.toContain("<a ");
  });
});

describe("TableScroll", () => {
  it("is a focusable, labelled region keeping the legacy `bang-cuon` hook", () => {
    const html = renderToStaticMarkup(
      <TableScroll aria-label="Danh sách cán bộ" sticky>
        <table className={`bang-can-bo ${DATA_TABLE_CLASS}`} />
      </TableScroll>,
    );
    expect(html).toMatch(/^<div role="region" tabindex="0" class="bang-cuon table-scroll table-scroll--sticky"/);
    expect(html).toContain('aria-label="Danh sách cán bộ"');
    expect(html).toContain('class="bang-can-bo data-table"');
  });
});

describe("PageHeader meta / Toolbar end / EmptyState tone", () => {
  it("PageHeader draws the meta line only when given", () => {
    expect(renderToStaticMarkup(<PageHeader icon={Pencil} title="T" />)).not.toContain("page-header-meta");
    const html = renderToStaticMarkup(
      <PageHeader icon={Pencil} title="T" meta={<PageHeaderMeta at="02/10/2026 10:32" by="CB-00123" />} />,
    );
    expect(html).toContain("page-header-meta");
    expect(html).toContain("lucide-history");
    expect(html).toContain("Cập nhật 02/10/2026 10:32");
    expect(html).toContain("— CB-00123");
  });

  it("PageHeaderMeta without a person draws no dash — never a guessed name", () => {
    expect(renderToStaticMarkup(<PageHeaderMeta at="hôm nay" />)).not.toContain("—");
  });

  // Presentation pin (ADR 0068 §5). Spec 02 §1 / ADR 0068 lần 6: the prototype's header has no icon
  // tile, so the `icon` a caller still passes draws nothing — and nothing decorative replaces it.
  it("PageHeader draws no icon tile, no gradient, no shadow — the icon prop is ignored", () => {
    const html = renderToStaticMarkup(<PageHeader icon={Pencil} title="T" />);
    expect(html).not.toContain("bg-linear");
    expect(html).not.toContain("shadow");
    expect(html).not.toContain("<svg");
    expect(html).toContain('class="m-0 text-[22px] leading-tight font-bold text-navy"');
  });

  it("Toolbar's `end` slot comes after the filters; absent = nothing extra", () => {
    expect(renderToStaticMarkup(<Toolbar>a</Toolbar>)).not.toContain("toolbar-end");
    const html = renderToStaticMarkup(<Toolbar end={<span>B</span>}>A</Toolbar>);
    expect(html.indexOf("A")).toBeLessThan(html.indexOf("toolbar-end"));
  });

  it("EmptyState tone changes the circle only; default unchanged", () => {
    expect(renderToStaticMarkup(<EmptyState title="x" />)).toContain("bg-brand-50 text-brand-600");
    expect(renderToStaticMarkup(<EmptyState title="x" tone="neutral" />)).toContain("bg-surface-muted text-ink-500");
    expect(renderToStaticMarkup(<EmptyState title="x" tone="danger" />)).toContain("bg-danger-50 text-danger-600");
  });
});
