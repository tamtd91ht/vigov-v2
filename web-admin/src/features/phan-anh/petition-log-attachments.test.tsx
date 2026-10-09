// @vitest-environment jsdom
//
// jsdom for this file: a file is CHOSEN, then three calls run; a download link is asked for only on a
// CLICK. Both are behaviour, not markup.

import { act } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { storedIds } from "@/features/nhiem-vu/task-attachments";
import type { petitions_nhatKyPhieuRa, petitions_taskAttachmentOut } from "@/lib/api/schema.gen";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { petitionPendingPart } from "./nhan-phieu";
import { BieuMauGhiNhatKy, DanhSachNhatKy } from "./nhat-ky-phieu";
import {
  PetitionLogAttachmentList,
  usePetitionLogAttachments,
  type LogAttachmentDeps,
} from "./petition-log-attachments";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

async function flush(times = 6) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function render(node: React.ReactNode) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

const FILE: petitions_taskAttachmentOut = {
  id: "01JFILE1",
  file_name: "bien-ban-hien-truong.pdf",
  mime_type: "application/pdf",
  size_bytes: 2048,
  status: "stored",
};

describe("usePetitionLogAttachments — the three calls on the PETITION's routes", () => {
  let api: ReturnType<typeof usePetitionLogAttachments> | null = null;
  function Harness({ deps }: { deps: LogAttachmentDeps }) {
    api = usePetitionLogAttachments("PA-1", deps);
    return <p>{api.items.map((i) => i.state.kind).join(",")}</p>;
  }

  function deps(change: Partial<LogAttachmentDeps> = {}): LogAttachmentDeps {
    return {
      request: vi.fn(async () => ({
        ok: true as const,
        data: {
          attachment: { ...FILE, status: "pending" },
          upload: { url: "https://kho.example.test/tmp", fields: { key: "t_01J/x" }, expires_at: "x" },
        },
      })),
      upload: vi.fn(async () => ({ ok: true as const, data: null })),
      complete: vi.fn(async () => ({ ok: true as const, data: FILE })),
      ...change,
    };
  }

  it("declare (name, type, size, one key) → upload → complete; only STORED ids are sent", async () => {
    const d = deps();
    render(<Harness deps={d} />);
    act(() => api?.add([new File(["%PDF-1.7"], "bien-ban-hien-truong.pdf", { type: "application/pdf" })]));
    await flush();
    const [code, body, key] = (d.request as ReturnType<typeof vi.fn>).mock.calls[0] as [string, unknown, string];
    expect(code).toBe("PA-1");
    expect(body).toEqual({ file_name: "bien-ban-hien-truong.pdf", content_type: "application/pdf", size: 8 });
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(d.upload).toHaveBeenCalledTimes(1);
    expect(d.complete).toHaveBeenCalledWith("PA-1", "01JFILE1");
    expect(storedIds(api!.items)).toEqual(["01JFILE1"]);
  });

  it("a type outside PDF / JPG / PNG is refused before any call", async () => {
    const d = deps();
    render(<Harness deps={d} />);
    act(() => api?.add([new File(["x"], "so-lieu.xlsx", { type: "application/vnd.ms-excel" })]));
    await flush();
    expect(d.request).not.toHaveBeenCalled();
    expect(storedIds(api!.items)).toEqual([]);
  });

  it("503 at completion: retry the COMPLETION (never a new upload); its id is not sent until stored", async () => {
    const complete = vi
      .fn<LogAttachmentDeps["complete"]>()
      .mockResolvedValueOnce({ ok: false, status: 503, message: "Chưa quét được mã độc. Vui lòng thử lại." })
      .mockResolvedValueOnce({ ok: true, data: FILE });
    const d = deps({ complete });
    render(<Harness deps={d} />);
    act(() => api?.add([new File(["%PDF"], "a.pdf", { type: "application/pdf" })]));
    await flush();
    expect(api!.items[0]?.state.kind).toBe("retry");
    expect(storedIds(api!.items)).toEqual([]);
    act(() => api?.retry(api!.items[0]!.key));
    await flush();
    expect(d.upload).toHaveBeenCalledTimes(1);
    expect(complete).toHaveBeenCalledTimes(2);
    expect(storedIds(api!.items)).toEqual(["01JFILE1"]);
  });
});

describe("PetitionLogAttachmentList — the download link is asked for AT THE CLICK", () => {
  it("nothing is fetched on render; the click asks once and opens the signed link in a detached tab", async () => {
    const download = vi.fn(async () => ({
      ok: true as const,
      data: { url: "https://kho.example.test/f?sig=1", expires_at: "x" },
    }));
    const tab = { opener: {} as unknown, location: { href: "" }, close: vi.fn() };
    vi.stubGlobal("open", vi.fn(() => tab));
    const h = render(<PetitionLogAttachmentList lookupCode="PA-1" attachments={[FILE]} download={download} />);
    expect(download).not.toHaveBeenCalled();
    expect(h.textContent).toContain("bien-ban-hien-truong.pdf");
    act(() => h.querySelector<HTMLButtonElement>("button")?.click());
    await flush();
    expect(download).toHaveBeenCalledWith("PA-1", "01JFILE1");
    expect(tab.opener).toBeNull();
    expect(tab.location.href).toBe("https://kho.example.test/f?sig=1");
  });

  it("removing a file has no route: a DISABLED remove button with its '?', after the download (ADR 0068 §14)", () => {
    const html = renderToStaticMarkup(<PetitionLogAttachmentList lookupCode="PA-1" attachments={[FILE]} />);
    const remove = html.match(/<button[^>]*aria-label="Gỡ tệp bien-ban-hien-truong.pdf"[^>]*>/)?.[0] ?? "";
    expect(remove).toContain('disabled=""');
    expect(html).toContain(pendingMarkerLabel(petitionPendingPart("logFileRemoval").ten));
    expect(html.indexOf("Tải về")).toBeLessThan(html.indexOf("Gỡ tệp"));
  });

  it("a refusal closes the blank tab and says the server's sentence", async () => {
    const tab = { opener: {} as unknown, location: { href: "" }, close: vi.fn() };
    vi.stubGlobal("open", vi.fn(() => tab));
    const h = render(
      <PetitionLogAttachmentList
        lookupCode="PA-1"
        attachments={[FILE]}
        download={async () => ({ ok: false as const, status: 403, message: "Bạn không có quyền tải tệp này." })}
      />,
    );
    act(() => h.querySelector<HTMLButtonElement>("button")?.click());
    await flush();
    expect(tab.close).toHaveBeenCalled();
    expect(h.querySelector('[role="alert"]')?.textContent).toContain("Bạn không có quyền tải tệp này.");
  });
});

describe("the log — files on a row, and the entry form waiting for uploads", () => {
  const ROW: petitions_nhatKyPhieuRa = {
    id: "01JLOG1",
    at: "2026-10-02T02:00:00Z",
    actor_code: "CB-00123",
    action: "ghi-chu",
    status: "dang-xu-ly",
    unit: "",
    assignee: "",
    note: "Đã chụp biên bản.",
    attachments: [FILE],
  };

  it("a row with files lists them with a download button", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy dong={[ROW]} tenBoPhan={new Map()} danhBa={null} maTraCuu="PA-1" />,
    );
    expect(html).toContain("bien-ban-hien-truong.pdf");
    expect(html).toContain('aria-label="Tải về bien-ban-hien-truong.pdf"');
  });

  it("the `nhap-ho` row reads “Nhập hộ phản ánh”", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy dong={[{ ...ROW, action: "nhap-ho", attachments: [] }]} tenBoPhan={new Map()} danhBa={null} maTraCuu="PA-1" />,
    );
    expect(html).toContain("Nhập hộ phản ánh");
  });

  it("while a file still moves, the entry waits: the note says so and the submit is disabled", () => {
    const tag = (h: string) => (h.match(/<button type="submit"[^>]*>/)?.[0] ?? "").replace(/\sclass="[^"]*"/, "");
    const props = { id: "f", noiDung: "Đã tới.", datNoiDung: () => {}, dangGui: false, loi: null, gui: () => {} };
    const waiting = renderToStaticMarkup(<BieuMauGhiNhatKy {...props} choTep dinhKem={<p>PICKER</p>} />);
    expect(waiting).toContain("PICKER");
    expect(waiting).toContain("Chờ các tệp tải lên và kiểm tra xong rồi mới ghi nhật ký.");
    expect(tag(waiting)).toContain("disabled");
    expect(tag(renderToStaticMarkup(<BieuMauGhiNhatKy {...props} />))).not.toContain("disabled");
  });
});
