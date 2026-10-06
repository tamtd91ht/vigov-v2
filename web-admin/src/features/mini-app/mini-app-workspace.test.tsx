import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

import { NO_MINI_APP_ACCESS } from "./mini-app-tabs";
import { MiniAppFrame } from "./mini-app-workspace";

/**
 * Tab gating of `/mini-app`: each tab on its own key, the other tab's screen never mounted, the denied and
 * unread cases drawn. The two screens are stand-ins — what matters is WHICH one renders.
 */

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-10-06T12:00:00Z",
      staff: { code: "CB001", full_name: "Cán bộ thử", position: "Chuyên viên" },
      permissions: [...permissions],
    },
  } as PhienDaDoc;
}

const CONTENT = <p>CONTENT-SCREEN</p>;
const DIRECTORY = <p>DIRECTORY-SCREEN</p>;

function render(session: PhienDaDoc, requested: "noi-dung" | "danh-ba" = "noi-dung") {
  return renderToStaticMarkup(
    <MiniAppFrame session={session} requested={requested} content={CONTENT} directory={DIRECTORY} />,
  );
}

const tabLinks = (html: string) => html.match(/<a [^>]*>.*?<\/a>/g) ?? [];

describe("/mini-app tabs", () => {
  it("both keys: two tab links in prototype order, the requested one current, its screen only", () => {
    const html = render(sessionWith(["content.read", "admin.user"]), "danh-ba");
    const links = tabLinks(html);
    expect(links.map((a) => /href="([^"]*)"/.exec(a)?.[1])).toEqual([
      "/mini-app?tab=noi-dung",
      "/mini-app?tab=danh-ba",
    ]);
    expect(links[0]).toContain(">Nội dung</a>");
    expect(links[1]).toContain(">Danh bạ cán bộ</a>");
    expect(links[1]).toContain('aria-current="page"');
    expect(links[0]).not.toContain("aria-current");
    expect(html).toContain("DIRECTORY-SCREEN");
    expect(html).not.toContain("CONTENT-SCREEN");
  });

  it("both keys, no tab asked: the content tab", () => {
    const html = render(sessionWith(["content.read", "admin.user"]));
    expect(html).toContain("CONTENT-SCREEN");
    expect(html).not.toContain("DIRECTORY-SCREEN");
  });

  it("DENIED content: only admin.user → one tab, the directory, even when the content tab is asked for", () => {
    const html = render(sessionWith(["admin.user", "content.update"]), "noi-dung");
    expect(tabLinks(html)).toHaveLength(1);
    expect(html).not.toContain(">Nội dung</a>");
    expect(html).toContain("DIRECTORY-SCREEN");
    expect(html).not.toContain("CONTENT-SCREEN");
  });

  it("DENIED directory: only content.read → one tab, the content, even when ?tab=danh-ba", () => {
    const html = render(sessionWith(["content.read", "admin.user.delete"]), "danh-ba");
    expect(tabLinks(html)).toHaveLength(1);
    expect(html).not.toContain("Danh bạ cán bộ");
    expect(html).toContain("CONTENT-SCREEN");
    expect(html).not.toContain("DIRECTORY-SCREEN");
  });

  it("DENIED both: no tab, no screen, the sentence naming both keys", () => {
    const html = render(sessionWith(["report.read", "content.update"]));
    expect(html).not.toContain("/mini-app?tab=");
    expect(html).not.toContain("SCREEN");
    expect(html).toContain(NO_MINI_APP_ACCESS);
  });

  it("session not read yet: no tab, no screen (nothing fires a read), only the status line", () => {
    const html = render(null, "danh-ba");
    expect(html).not.toContain("<a ");
    expect(html).not.toContain("SCREEN");
    expect(html).toContain('role="status"');
  });

  it("session could not be read: the server's sentence, no tab, no screen", () => {
    const html = render({ ok: false, thongBao: "Phiên làm việc đã hết hạn." } as PhienDaDoc);
    expect(html).toContain("Phiên làm việc đã hết hạn.");
    expect(html).not.toContain("<a ");
    expect(html).not.toContain("SCREEN");
  });
});
