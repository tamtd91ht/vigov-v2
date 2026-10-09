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

import { BieuMauGhiNhatKy, DanhSachNhatKy, mayRemoveRowFiles } from "./nhat-ky-phieu";
import {
  LogAttachmentRemoveDialog,
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

  it("DENIED (no `onRemove`): no remove control at all — and no '?' any more (ADR 0088 §2 built it)", () => {
    const html = renderToStaticMarkup(<PetitionLogAttachmentList lookupCode="PA-1" attachments={[FILE]} />);
    expect(html).not.toContain("Gỡ tệp");
    expect(html).not.toContain(pendingMarkerLabel("Gỡ tệp đính kèm"));
  });

  it("ALLOWED (`onRemove` given): a live remove button after the download; it opens the reason dialog for THAT file", () => {
    const onRemove = vi.fn();
    const h = render(<PetitionLogAttachmentList lookupCode="PA-1" attachments={[FILE]} onRemove={onRemove} />);
    const remove = h.querySelector<HTMLButtonElement>('button[aria-label="Gỡ tệp bien-ban-hien-truong.pdf"]');
    expect(remove?.disabled).toBe(false);
    expect(remove?.getAttribute("aria-haspopup")).toBe("dialog");
    expect(h.innerHTML.indexOf("Tải về")).toBeLessThan(h.innerHTML.indexOf("Gỡ tệp"));
    act(() => remove?.click());
    expect(onRemove).toHaveBeenCalledWith(FILE);
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

describe("log file removal (ADR 0088 §2) — who is offered it, and the reason dialog", () => {
  const ROW: petitions_nhatKyPhieuRa = {
    id: "01JLOG1",
    at: "2026-10-02T02:00:00Z",
    actor_code: "CB-00123",
    action: "ghi-chu",
    status: "dang-xu-ly",
    unit: "",
    assignee: "",
    note: "",
    attachments: [FILE],
  };

  it("offered to the row's author (the uploader) or a `feedback.resolve` holder — and to nobody else", () => {
    expect(mayRemoveRowFiles(ROW, "CB-00123", false)).toBe(true);
    expect(mayRemoveRowFiles(ROW, "CB-00999", true)).toBe(true);
    // DENIED: another officer without the key; an unread session (empty code) never matches.
    expect(mayRemoveRowFiles(ROW, "CB-00999", false)).toBe(false);
    expect(mayRemoveRowFiles(ROW, "", false)).toBe(false);
    expect(mayRemoveRowFiles({ actor_code: "" }, "", false)).toBe(false);
  });

  it("the log draws the remove control only on rows the account may touch", () => {
    const other = { ...ROW, id: "01JLOG2", actor_code: "CB-00200", attachments: [{ ...FILE, id: "01JFILE2", file_name: "khac.pdf" }] };
    const html = renderToStaticMarkup(
      <DanhSachNhatKy
        dong={[ROW, other]}
        tenBoPhan={new Map()}
        danhBa={null}
        maTraCuu="PA-1"
        mayRemoveFiles={(r) => mayRemoveRowFiles(r, "CB-00123", false)}
        onRemoveFile={() => {}}
      />,
    );
    expect(html).toContain('aria-label="Gỡ tệp bien-ban-hien-truong.pdf"');
    expect(html).not.toContain('aria-label="Gỡ tệp khac.pdf"');
  });

  it("the reason is REQUIRED; the DELETE carries it; success tells the caller to re-read the log", async () => {
    const remove = vi.fn(async () => ({ ok: true as const, duLieu: undefined }));
    const onRemoved = vi.fn();
    const onClose = vi.fn();
    const h = render(
      <LogAttachmentRemoveDialog lookupCode="PA-1" file={FILE} onClose={onClose} onRemoved={onRemoved} remove={remove} />,
    );
    const submit = () => [...h.querySelectorAll<HTMLButtonElement>('button[type="submit"]')].find((b) => b.textContent?.includes("Gỡ tệp"))!;
    expect(submit().disabled).toBe(true);
    const input = h.querySelector<HTMLInputElement>("#ly-do-go-tep-phieu")!;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    act(() => {
      setter?.call(input, "  Tải nhầm tệp  ");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    expect(submit().disabled).toBe(false);
    act(() => submit().click());
    await flush();
    expect(remove).toHaveBeenCalledWith("PA-1", "01JFILE1", "Tải nhầm tệp");
    expect(onRemoved).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("a refusal (403 / 409 `legal_hold`) stays IN the dialog, verbatim, the reason kept", async () => {
    const cau = "Tệp đang bị phong toả theo yêu cầu pháp lý, chưa gỡ được.";
    const onRemoved = vi.fn();
    const h = render(
      <LogAttachmentRemoveDialog
        lookupCode="PA-1"
        file={FILE}
        onClose={() => {}}
        onRemoved={onRemoved}
        remove={async () => ({ ok: false as const, thongBao: cau })}
      />,
    );
    const input = h.querySelector<HTMLInputElement>("#ly-do-go-tep-phieu")!;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    act(() => {
      setter?.call(input, "Tải nhầm tệp");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    act(() => h.querySelector<HTMLFormElement>("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
    await flush();
    expect(h.querySelector('[role="alert"]')?.textContent).toBe(cau);
    expect(input.value).toBe("Tải nhầm tệp");
    expect(onRemoved).not.toHaveBeenCalled();
  });
});
