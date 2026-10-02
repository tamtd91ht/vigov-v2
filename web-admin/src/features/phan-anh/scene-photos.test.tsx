import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { listPetitionPhotos } from "@/lib/api/phieu-phan-anh";
import type { petitions_phieuPhanAnhRa, petitions_photoLinkOut } from "@/lib/api/schema.gen";

import {
  congThaoTac,
  photoLinkUsable,
  PHOTO_LINK_MARGIN_MS,
  SCENE_PHOTOS_AFTER_NOT_BUILT,
  SCENE_PHOTOS_EMPTY,
  SCENE_PHOTOS_LOADING,
  SCENE_PHOTOS_TITLE,
  SCENE_PHOTOS_UNAVAILABLE,
  scenePhotoAlt,
  scenePhotoSrc,
} from "./nhan-phieu";
import { freshLinksFor, ScenePhotosView, scenePhotosState } from "./scene-photos";
import { ChiTietPhieu } from "./so-phan-anh";

/**
 * §8.4 `Trước khi xử lý` — the citizen's scene photos on the petition drawer.
 *
 * The signed links live ≤ 15 minutes: the case that matters most is the one nobody sees while building
 * (a drawer left open past expiry), so `freshLinksFor` is tested at the boundary, not only "it works".
 */

const NOW = new Date("2026-10-02T03:00:00Z");

function photo(n: number, change: Partial<petitions_photoLinkOut> = {}): petitions_photoLinkOut {
  return {
    id: `01JPHOTO${n}`,
    content_type: "image/jpeg",
    size_bytes: 120_000,
    created_at: "2026-10-02T02:50:00Z",
    url: `https://kho.example.test/t_01JTENANT/photo-${n}.jpg?X-Amz-Signature=fake`,
    url_expires_at: "2026-10-02T03:10:00Z",
    ...change,
  };
}

/** How React writes a text node: `&`, `"` and `'` are escaped. */
function asHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/'/g, "&#x27;");
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("listPetitionPhotos — the contract route, with its status", () => {
  it("GETs …/photos on the commune's own origin, no cache, the lookup code encoded", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ items: [photo(1)] }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const r = await listPetitionPhotos("PA-2026/0021");

    expect(r.ok).toBe(true);
    const [path, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(path).toBe("/api/v1/citizen-reports/PA-2026%2F0021/photos");
    expect(init.method).toBe("GET");
    expect(init.credentials).toBe("same-origin");
    expect(init.cache).toBe("no-store");
    // No tenant on the wire, in any form (rule 1, forbidden #2).
    expect(path).not.toMatch(/tenant/i);
  });

  it("503 keeps its status so the drawer can say its own sentence", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ code: "storage_not_configured", message: "Hệ thống chưa sẵn sàng xử lý ảnh." }), {
          status: 503,
        }),
      ),
    );
    const r = await listPetitionPhotos("PA-1");
    expect(r).toEqual({ ok: false, status: 503, message: "Hệ thống chưa sẵn sàng xử lý ảnh." });
  });

  it("no answer at all is status 0", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("network"))));
    const r = await listPetitionPhotos("PA-1");
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.status).toBe(0);
  });
});

describe("scenePhotosState", () => {
  it("null = loading; 503 = unavailable; another refusal = the server's sentence verbatim", () => {
    expect(scenePhotosState(null)).toEqual({ kind: "loading" });
    expect(scenePhotosState({ ok: false, status: 503, message: "x" })).toEqual({ kind: "unavailable" });
    expect(scenePhotosState({ ok: false, status: 404, message: "Không tìm thấy phiếu." })).toEqual({
      kind: "error",
      message: "Không tìm thấy phiếu.",
    });
  });

  it("an empty list is the empty state, not an error", () => {
    expect(scenePhotosState({ ok: true, data: { items: [] } })).toEqual({ kind: "empty" });
  });
});

describe("ScenePhotosView — what the officer reads", () => {
  it("list: one thumbnail per photo, alt text is a position, never a name", () => {
    const items = [photo(1), photo(2), photo(3)];
    const html = renderToStaticMarkup(
      <ScenePhotosView lookupCode="PA-1" state={{ kind: "list", items }} />,
    );
    expect(html).toContain(asHtml(SCENE_PHOTOS_TITLE));
    expect(html.match(/<img /g)?.length).toBe(3);
    for (let i = 0; i < 3; i++) expect(html).toContain(`alt="${scenePhotoAlt(i, 3)}"`);
    expect(html).toContain('alt="Ảnh hiện trường 1/3"');
    expect(html).toContain('referrerPolicy="no-referrer"');
    // A lazy request can fire after the link expired.
    expect(html).not.toContain('loading="lazy"');
    // The signed link is an image source only — never a link an officer could copy into a chat.
    expect(html).not.toMatch(/<a [^>]*kho\.example\.test/);
  });

  it("empty state sentence", () => {
    const html = renderToStaticMarkup(<ScenePhotosView lookupCode="PA-1" state={{ kind: "empty" }} />);
    expect(html).toContain(asHtml(SCENE_PHOTOS_EMPTY));
    expect(html).not.toContain("<img");
  });

  it("503: the storage sentence, a retry button, and no image", () => {
    const html = renderToStaticMarkup(
      <ScenePhotosView lookupCode="PA-1" state={scenePhotosState({ ok: false, status: 503, message: "x" })} />,
    );
    expect(html).toContain(asHtml(SCENE_PHOTOS_UNAVAILABLE));
    expect(html).toContain("Kho ảnh tạm thời chưa sẵn sàng");
    expect(html).toContain("Tải lại ảnh");
    expect(html).not.toContain("<img");
  });

  it("loading is a sentence, not a spinner alone", () => {
    const html = renderToStaticMarkup(<ScenePhotosView lookupCode="PA-1" state={{ kind: "loading" }} />);
    expect(html).toContain(asHtml(SCENE_PHOTOS_LOADING));
  });

  it("the 'after' column says it is not built — and never claims a closing rule that is not enforced", () => {
    const html = renderToStaticMarkup(<ScenePhotosView lookupCode="PA-1" state={{ kind: "empty" }} />);
    expect(html).toContain(asHtml(SCENE_PHOTOS_AFTER_NOT_BUILT));
    expect(html).not.toContain("Bắt buộc phải có trước khi đóng phiếu");
    expect(html).not.toContain("Tải ảnh sau xử lý");
  });

  it("full-size view: an in-page dialog titled by position, with the larger image", () => {
    const items = [photo(1), photo(2)];
    const html = renderToStaticMarkup(
      <ScenePhotosView lookupCode="PA-1" state={{ kind: "list", items }} openIndex={1} />,
    );
    expect(html).toContain('role="dialog"');
    expect(html).toContain(">Ảnh hiện trường 2/2</h5>");
    expect(html.match(/<img /g)?.length).toBe(3);
    expect(html).toContain("Xem ảnh liền trước");
    expect(html).not.toContain("Xem ảnh tiếp theo");
  });

  it("a server string that is not http(s) never becomes a src", () => {
    const html = renderToStaticMarkup(
      <ScenePhotosView
        lookupCode="PA-1"
        state={{ kind: "list", items: [photo(1, { url: "javascript:alert(1)" })] }}
      />,
    );
    expect(html).not.toContain("javascript:");
    expect(html).not.toContain("<img");
    expect(scenePhotoSrc({ url: "javascript:alert(1)" })).toBeNull();
    expect(scenePhotoSrc({ url: "" })).toBeNull();
  });
});

describe("expired links are refetched, never used", () => {
  it("photoLinkUsable: valid until the margin before expiry; unparsable is expired (fail closed)", () => {
    const at = (ms: number) => ({ url_expires_at: new Date(NOW.getTime() + ms).toISOString() });
    expect(photoLinkUsable(at(PHOTO_LINK_MARGIN_MS + 1000), NOW)).toBe(true);
    expect(photoLinkUsable(at(PHOTO_LINK_MARGIN_MS), NOW)).toBe(false);
    expect(photoLinkUsable(at(-1000), NOW)).toBe(false);
    expect(photoLinkUsable({ url_expires_at: "không phải ngày" }, NOW)).toBe(false);
  });

  it("a usable link opens without a new call (no needless signing over personal data)", async () => {
    const reload = vi.fn();
    const r = await freshLinksFor([photo(1)], "01JPHOTO1", NOW, reload);
    expect(reload).not.toHaveBeenCalled();
    expect(r).toEqual({ kind: "fresh", items: [photo(1)], refetched: false });
  });

  it("an expired link re-reads the list once and uses the NEW link", async () => {
    const stale = photo(1, { url_expires_at: "2026-10-02T02:59:00Z" });
    const renewed = photo(1, { url: "https://kho.example.test/new.jpg", url_expires_at: "2026-10-02T03:15:00Z" });
    const reload = vi.fn(async () => ({ ok: true as const, data: { items: [renewed] } }));

    const r = await freshLinksFor([stale], stale.id, NOW, reload);

    expect(reload).toHaveBeenCalledTimes(1);
    expect(r).toEqual({ kind: "fresh", items: [renewed], refetched: true });
  });

  it("force (the full-size image failed) re-reads even a link that looks valid", async () => {
    const reload = vi.fn(async () => ({ ok: true as const, data: { items: [photo(1)] } }));
    await freshLinksFor([photo(1)], "01JPHOTO1", NOW, reload, true);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("a refused re-read is returned as such — the stale list is not kept", async () => {
    const stale = photo(1, { url_expires_at: "2026-10-02T02:00:00Z" });
    const reload = vi.fn(async () => ({ ok: false as const, status: 503, message: "x" }));
    const r = await freshLinksFor([stale], stale.id, NOW, reload);
    expect(r).toEqual({ kind: "failed", result: { ok: false, status: 503, message: "x" } });
  });
});

describe("drawer gating — UX only, the server decides", () => {
  function petition(): petitions_phieuPhanAnhRa {
    return {
      code: "PA-2026-0021",
      channel: "zalo-mini-app",
      status: "da-tiep-nhan",
      field: "",
      field_label: "",
      content: "Rác tồn đọng ở đầu ngõ.",
      address: "Tổ 6",
      reporter_name: "Nguyễn V. A.",
      reporter_phone: "09****0000",
      anonymous: false,
      clock_from: "2026-10-02T02:00:00Z",
      booked_at: "2026-10-02T02:01:00Z",
      acknowledge_due: "2026-10-02T04:00:00Z",
      resolve_due: "",
      classify_due: "2026-10-02T06:00:00Z",
      unit: "",
      assignee: "",
      result: "",
      public: false,
    };
  }

  function drawer(permissions: readonly string[]): string {
    return renderToStaticMarkup(
      <ChiTietPhieu
        phieu={petition()}
        bayGio={NOW}
        cong={congThaoTac(false, false, false)}
        tenBoPhan={new Map()}
        boPhan={[]}
        danhBa={null}
        dangGui={false}
        loiGhi={null}
        permissions={permissions}
        dong={() => {}}
        phanLoai={() => {}}
        chuyenXuLy={() => {}}
        tienTrangThai={() => {}}
        dongPhieuLai={() => {}}
        khongTiepNhan={() => {}}
        chuyenCapTren={() => {}}
      />,
    );
  }

  it("with `feedback.read`: the block is drawn and starts by saying it is loading", () => {
    const html = drawer(["feedback.read"]);
    expect(html).toContain(asHtml(SCENE_PHOTOS_TITLE));
    expect(html).toContain(asHtml(SCENE_PHOTOS_LOADING));
  });

  it("DENIED — without `feedback.read` (an unknown session holds no key): no photo block at all", () => {
    for (const perms of [[], ["feedback.classify", "task.create"]]) {
      const html = drawer(perms);
      expect(html).not.toContain(asHtml(SCENE_PHOTOS_TITLE));
      expect(html).not.toContain(asHtml(SCENE_PHOTOS_LOADING));
    }
  });
});
