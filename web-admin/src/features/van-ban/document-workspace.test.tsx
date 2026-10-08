import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { parseDrillDown } from "@/lib/drill-down";

import {
  DOCUMENT_PAGE_SUBTITLE,
  DOCUMENT_PAGE_TITLE,
  DocumentWorkspace,
  LETTER_PAGE_TITLE,
  documentPageTitle,
  initialDocumentTab,
} from "./document-workspace";

/**
 * The page frame of `/van-ban` (ADR 0078, prototype `DocumentWorkspace.tsx:105-166`): header → tab bar
 * → the active tab. Rendered on the server, so no effect runs — a register shows its first-load
 * state, which is enough to see what the frame puts where.
 */
function render(search: Record<string, string> = {}) {
  return renderToStaticMarkup(
    <PhienProvider>
      <DocumentWorkspace drillDown={parseDrillDown("incoming-documents", search)} />
    </PhienProvider>,
  );
}

function selectedTab(html: string): string {
  const tag = [...html.matchAll(/<button[^>]*role="tab"[^>]*>/g)].map((m) => m[0]).find((t) => t.includes('aria-selected="true"'));
  return /id="(tab-van-ban-[a-z]+)"/.exec(tag ?? "")?.[1] ?? "";
}

describe("khung trang Văn bản & đơn thư", () => {
  const html = render();

  it("đầu trang: tiêu đề, phụ đề của prototype, rồi thanh tab, rồi khung tab", () => {
    const h1 = html.indexOf(`>${LETTER_PAGE_TITLE}</h1>`);
    const subtitle = html.indexOf(DOCUMENT_PAGE_SUBTITLE);
    const tablist = html.indexOf('role="tablist"');
    const panel = html.indexOf('role="tabpanel"');
    expect(h1).toBeGreaterThan(-1);
    expect(html).toMatch(/<h1[^>]*class="[^"]*text-\[22px\][^"]*font-bold[^"]*text-navy/);
    expect(subtitle).toBeGreaterThan(h1);
    expect(tablist).toBeGreaterThan(subtitle);
    expect(panel).toBeGreaterThan(tablist);
  });

  it("vào trang KHÔNG kèm lọc Tổng quan: tab Đơn thư công dân, như prototype (ADR 0078 #5)", () => {
    expect(selectedTab(html)).toBe("tab-van-ban-petitions");
    expect(html).toContain('aria-labelledby="tab-van-ban-petitions"');
    expect(html).toContain("Sổ theo dõi đơn khiếu nại");
    expect(initialDocumentTab({ kind: "none" })).toBe("petitions");
  });

  it("vào trang KÈM lọc Tổng quan (metric=…, hợp lệ hay không): tab Văn bản đến, nơi lọc ấy áp vào", () => {
    const active = render({ metric: "overdue" });
    expect(selectedTab(active)).toBe("tab-van-ban-incoming");
    expect(active).toContain("Đang tải sổ văn bản đến");

    const invalid = render({ metric: "khong-co" });
    expect(selectedTab(invalid)).toBe("tab-van-ban-incoming");
  });

  it("tiêu đề H1 theo tab (prototype `DocumentWorkspace.tsx:108`): Đơn thư/Báo cáo → “Đơn thư công dân”; Văn bản đến/đi giữ “Văn bản & đơn thư”", () => {
    expect(LETTER_PAGE_TITLE).toBe("Đơn thư công dân");
    expect(DOCUMENT_PAGE_TITLE).toBe("Văn bản & đơn thư");
    expect(documentPageTitle("petitions")).toBe(LETTER_PAGE_TITLE);
    expect(documentPageTitle("report")).toBe(LETTER_PAGE_TITLE);
    expect(documentPageTitle("incoming")).toBe(DOCUMENT_PAGE_TITLE);
    expect(documentPageTitle("outgoing")).toBe(DOCUMENT_PAGE_TITLE);
    // Rendered: the page opened on Đơn thư says so; opened on Văn bản đến it keeps the page's own title.
    expect(html).toMatch(/<h1[^>]*>Đơn thư công dân<\/h1>/);
    expect(render({ metric: "overdue" })).toMatch(/<h1[^>]*>Văn bản &amp; đơn thư<\/h1>/);
  });

  it("thứ tự tab: Văn bản đến · Văn bản đi · Đơn thư công dân · Báo cáo", () => {
    const tabs = [...html.matchAll(/<button[^>]*role="tab"[^>]*>([^<]*)<\/button>/g)].map((m) => m[1]);
    expect(tabs).toEqual(["Văn bản đến", "Văn bản đi", "Đơn thư công dân", "Báo cáo"]);
  });

  it("phiên chưa đọc xong: đầu trang CHƯA vẽ nút ghi nào (ba trạng thái, không hai); không còn “Quét & OCR”", () => {
    // Server render: no effect runs, so the session is "not read yet" — neither "may book" nor
    // "may not". The booking button appears once the session says `petition.create`
    // (`letter-register.test.tsx` covers both answers).
    const header = html.slice(0, html.indexOf('role="tablist"'));
    expect(header).not.toContain("Vào sổ đơn thư");
    expect(html).not.toContain("OCR");
  });
});
