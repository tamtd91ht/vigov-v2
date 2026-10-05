import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { DOCUMENT_PAGE_SUBTITLE, DOCUMENT_PAGE_TITLE, DocumentWorkspace } from "./document-workspace";

/**
 * The page frame of `/van-ban` (ADR 0068 lần 5, prototype `DocumentWorkspace.tsx`): header → tab bar
 * → the active tab. Rendered on the server, so no effect runs — the register shows its first-load
 * state, which is enough to see what the frame puts where.
 */
describe("khung trang Văn bản & đơn thư", () => {
  const html = renderToStaticMarkup(
    <PhienProvider>
      <DocumentWorkspace />
    </PhienProvider>,
  );

  it("đầu trang: tiêu đề, phụ đề của prototype, rồi thanh tab, rồi khung tab", () => {
    const h1 = html.indexOf(`>${DOCUMENT_PAGE_TITLE.replace("&", "&amp;")}</h1>`);
    const subtitle = html.indexOf(DOCUMENT_PAGE_SUBTITLE);
    const tablist = html.indexOf('role="tablist"');
    const panel = html.indexOf('role="tabpanel"');
    expect(h1).toBeGreaterThan(-1);
    expect(subtitle).toBeGreaterThan(h1);
    expect(tablist).toBeGreaterThan(subtitle);
    expect(panel).toBeGreaterThan(tablist);
  });

  it("vào trang là tab Văn bản đến — một sổ đang chạy, không phải tab Đơn thư của prototype", () => {
    const tab = /<button[^>]*id="tab-van-ban-incoming"[^>]*>/.exec(html)?.[0] ?? "";
    expect(tab).toContain('aria-selected="true"');
    expect(html).toContain('aria-labelledby="tab-van-ban-incoming"');
    expect(html).toContain("Đang tải sổ văn bản đến");
    expect(html).not.toContain("Sổ theo dõi đơn khiếu nại");
  });

  it("nút đầu trang của tab Văn bản đến nằm TRONG đầu trang, trước thanh tab", () => {
    // The session is not read on the server, so the create button is not drawn yet (three states);
    // the OCR placeholder is, and it sits in the header.
    expect(html.indexOf("Quét &amp; OCR")).toBeGreaterThan(-1);
    expect(html.indexOf("Quét &amp; OCR")).toBeLessThan(html.indexOf('role="tablist"'));
  });
});
