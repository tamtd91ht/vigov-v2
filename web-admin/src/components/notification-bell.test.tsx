import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { StaffNotification } from "@/lib/api/notifications";

import type { BellFeed } from "./notification-bell";
import { NotificationBellView } from "./notification-bell";
import {
  EMPTY_SENTENCE,
  READ_ALL_BUTTON,
  badgeText,
  bellLabel,
  kindLabel,
  safeLink,
} from "./notification-bell-logic";

/**
 * The header bell. Whose notices it shows is the server's decision (session → staff code); the view
 * only renders what it was sent — so these cases check what it DRAWS and which links it will FOLLOW.
 */

function notice(patch: Partial<StaffNotification> = {}): StaffNotification {
  return {
    id: "01JNOTICE",
    kind: "sap-den-han",
    title: "Nhiệm vụ sắp đến hạn: Rà soát hồ sơ",
    body: "Còn 4 giờ làm việc",
    link: "/nhiem-vu?id=01JTASK",
    read: false,
    read_at: null,
    created_at: "2026-09-29T02:00:00Z",
    ...patch,
  };
}

const noop = () => {};

function view(unread: number | null, feed: BellFeed, actionError = "") {
  return renderToStaticMarkup(
    <NotificationBellView
      unread={unread}
      feed={feed}
      actionError={actionError}
      onToggle={noop}
      onOpen={noop}
      onReadAll={noop}
      onMore={noop}
    />,
  );
}

describe("the bell button", () => {
  it("aria-label says the count (§8 wording); badge caps at 99+", () => {
    expect(bellLabel(3)).toBe("Thông báo — 3 thông báo chưa đọc");
    expect(badgeText(0)).toBeNull();
    expect(badgeText(5)).toBe("5");
    expect(badgeText(120)).toBe("99+");
    const html = view(3, { phase: "closed" });
    expect(html).toContain('aria-label="Thông báo — 3 thông báo chưa đọc"');
    expect(html).toContain('aria-expanded="false"');
    expect(html).not.toContain(READ_ALL_BUTTON);
  });

  it("a failed count draws no number — never a guessed 0", () => {
    expect(badgeText(null)).toBeNull();
    const html = view(null, { phase: "closed" });
    expect(html).toContain('aria-label="Thông báo"');
    expect(html).not.toContain("huy-hieu-chuong");
  });
});

describe("the panel", () => {
  it("items: kind label, title, second line, time HH:mm dd/MM/yyyy; unread marked", () => {
    const html = view(1, {
      phase: "ready",
      items: [notice(), notice({ id: "b", kind: "ban-tin-tuan", read: true, read_at: "2026-09-29T03:00:00Z", body: "" })],
      nextCursor: null,
    });
    expect(html).toContain("Sắp đến hạn");
    expect(html).toContain("Bản tin tuần");
    expect(html).toContain("Nhiệm vụ sắp đến hạn: Rà soát hồ sơ");
    expect(html).toContain("Còn 4 giờ làm việc");
    expect(html).toContain("09:00 29/09/2026");
    expect(html).toContain("muc-chuong chua-doc");
    expect(html).toContain("Chưa đọc");
    expect(html).toContain(`>${READ_ALL_BUTTON}</button>`);
  });

  it("empty inbox → the sentence; nothing unread → no 'Đọc hết'", () => {
    const html = view(0, { phase: "ready", items: [], nextCursor: null });
    expect(html).toContain(EMPTY_SENTENCE);
    expect(html).not.toContain(READ_ALL_BUTTON);
  });

  it("a failed read and a failed mark are the server's sentences", () => {
    expect(view(null, { phase: "failed", message: "Đã xảy ra lỗi. Vui lòng thử lại." })).toContain(
      'role="alert">Đã xảy ra lỗi. Vui lòng thử lại.</p>',
    );
    expect(view(1, { phase: "ready", items: [notice()], nextCursor: null }, "Không tìm thấy thông báo này.")).toContain(
      "Không tìm thấy thông báo này.",
    );
  });

  it("more pages → 'Xem thêm'", () => {
    expect(view(1, { phase: "ready", items: [notice()], nextCursor: "c2" })).toContain(">Xem thêm</button>");
  });

  it("NO RAW HTML: markup in a title or body is drawn as text", () => {
    const html = view(1, {
      phase: "ready",
      items: [notice({ title: "<img src=x onerror=alert(1)>", body: "<script>alert(1)</script>" })],
      nextCursor: null,
    });
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(html).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
    expect(html).not.toContain("<script>");
    expect(html).not.toContain("<img");
  });

  it("the link is never rendered as an href — the router is given it on click", () => {
    const html = view(1, { phase: "ready", items: [notice({ link: "/phan-anh?id=01J" })], nextCursor: null });
    expect(html).not.toContain("href=");
    expect(html).not.toContain("/phan-anh?id=01J");
  });
});

describe("kind labels", () => {
  it("the four kinds, and an unknown one as 'Thông báo'", () => {
    expect(kindLabel("sap-den-han")).toBe("Sắp đến hạn");
    expect(kindLabel("qua-han")).toBe("Quá hạn");
    expect(kindLabel("leo-thang")).toBe("Leo thang");
    expect(kindLabel("ban-tin-tuan")).toBe("Bản tin tuần");
    // ADR 0081 #5, migration 0023 — the bell-only disbursement mention.
    expect(kindLabel("giai-ngan.nhac-ten")).toBe("Được nhắc tên trong trao đổi giải ngân");
    expect(kindLabel("khac")).toBe("Thông báo");
  });
});

describe("safeLink — only a path of this site is followed", () => {
  it.each(["/nhiem-vu?id=01J", "/phan-anh", "/van-ban?don-thu=01J#x", "/"])("%s is followed", (l) => {
    expect(safeLink(l)).toBe(l);
  });

  it.each([
    "",
    "//evil.example/x",
    "/\\evil.example",
    "https://evil.example",
    "javascript:alert(1)",
    "nhiem-vu",
    "/a\\b",
    "/a\nb",
    " /nhiem-vu",
  ])("%j is refused", (l) => {
    expect(safeLink(l)).toBeNull();
  });
});
