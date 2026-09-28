import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  ATTACH_ACCEPT,
  ATTACH_EMPTY_REFUSED,
  ATTACH_TYPE_REFUSED,
  afterCompletion,
  anyInFlight,
  attachmentStateText,
  declaredType,
  formatBytes,
  storedIds,
  type AttachmentItem,
} from "./task-attachments";
import { AttachmentPicker, TimelineAttachments } from "./task-attachments-ui";

/**
 * `📎 Đính kèm` (A4). No DOM: pure rules, what each state renders (allowed and refused), and by
 * source the wiring of the flow and of the entry.
 */

const item = (key: string, state: AttachmentItem["state"]): AttachmentItem => ({ key, name: `${key}.pdf`, size: 2048, state });

describe("pre-check — convenience only; NO size limit is written here", () => {
  it("pdf / jpeg / png by the browser's type, or by extension when the type is empty", () => {
    expect(declaredType({ name: "a.pdf", type: "application/pdf", size: 1 })).toEqual({ ok: true, contentType: "application/pdf" });
    expect(declaredType({ name: "a.JPG", type: "", size: 1 })).toEqual({ ok: true, contentType: "image/jpeg" });
    expect(declaredType({ name: "a.docx", type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", size: 1 })).toEqual({
      ok: false,
      message: ATTACH_TYPE_REFUSED,
    });
    expect(declaredType({ name: "a.pdf", type: "", size: 0 })).toEqual({ ok: false, message: ATTACH_EMPTY_REFUSED });
    // A huge file is NOT refused here — the server's policy decides.
    expect(declaredType({ name: "a.pdf", type: "application/pdf", size: 10 ** 12 }).ok).toBe(true);
    expect(ATTACH_ACCEPT).toContain("application/pdf");
  });

  it("no hard-coded size limit anywhere in the pure module", () => {
    const src = readFileSync(fileURLToPath(new URL("./task-attachments.ts", import.meta.url)), "utf8");
    expect(src).not.toMatch(/MAX_BYTES|maxBytes|\d+\s*\*\s*1024\s*\*\s*1024\s*[;,)]/);
  });
});

describe("completion answers → state", () => {
  const stored = { id: "f", file_name: "a.pdf", mime_type: "application/pdf", size_bytes: 1, status: "stored" };
  it("200 stored ⇒ stored; 422 ⇒ refused for good; 503 / 409 / no answer ⇒ retry the completion", () => {
    expect(afterCompletion("f", { ok: true, data: stored })).toEqual({ kind: "stored", id: "f" });
    expect(afterCompletion("f", { ok: false, status: 422, message: "mã độc" })).toEqual({ kind: "refused", message: "mã độc" });
    for (const status of [503, 409, 0]) {
      expect(afterCompletion("f", { ok: false, status, message: "x" })).toEqual({ kind: "retry", id: "f", message: "x" });
    }
  });

  it("only STORED ids go with the entry; anything still moving holds the entry", () => {
    const items = [
      item("a", { kind: "stored", id: "A" }),
      item("b", { kind: "refused", message: "x" }),
      item("c", { kind: "retry", id: "C", message: "x" }),
      item("d", { kind: "stored", id: "D" }),
    ];
    expect(storedIds(items)).toEqual(["A", "D"]);
    expect(anyInFlight(items)).toBe(false);
    expect(anyInFlight([...items, item("e", { kind: "uploading", id: "E", percent: 40 })])).toBe(true);
  });

  it("words per state, and sizes with a Vietnamese decimal comma", () => {
    expect(attachmentStateText({ kind: "uploading", id: "x", percent: 42 })).toBe("Đang tải 42%");
    expect(attachmentStateText({ kind: "refused", message: "Tệp bị từ chối vì phát hiện mã độc." })).toContain("Bị từ chối: Tệp bị từ chối vì phát hiện mã độc.");
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1536)).toBe("1,5 KB");
    expect(formatBytes(1.2 * 1024 * 1024)).toBe("1,2 MB");
  });
});

describe("the picker — per-file state, progress as status, refusals as alert", () => {
  const html = renderToStaticMarkup(
    <AttachmentPicker
      fieldId="ghi-nhat-ky-NV19"
      items={[
        item("a", { kind: "uploading", id: "A", percent: 30 }),
        item("b", { kind: "refused", message: "Tệp lớn hơn dung lượng tối đa được phép đính kèm." }),
        item("c", { kind: "retry", id: "C", message: "Chưa quét được mã độc…" }),
        item("d", { kind: "stored", id: "D" }),
      ]}
      disabled={false}
      onAdd={() => {}}
      onRetry={() => {}}
      onRemove={() => {}}
    />,
  );

  it("the control is a keyboard-reachable label of a multi-file input, pdf/jpg/png", () => {
    expect(html).toMatch(/<label for="ghi-nhat-ky-NV19-dinh-kem" class="nut-phu">📎 Đính kèm/);
    expect(html).toMatch(/type="file"[^>]*multiple=""[^>]*accept="\.pdf,\.jpg,\.jpeg,\.png/);
  });

  it("progress `role=status`, refusals `role=alert` with the server's sentence; retry only on the retry state", () => {
    expect(html).toContain('<span role="status">Đang tải 30%</span>');
    expect(html).toContain('<span role="alert">Bị từ chối: Tệp lớn hơn dung lượng tối đa được phép đính kèm.</span>');
    expect(html.split(">Kiểm tra lại<").length - 1).toBe(1);
    // Every file can be removed before the entry is sent.
    expect(html.split(">Bỏ<").length - 1).toBe(4);
    expect(html).toContain('aria-label="Bỏ d.pdf"');
  });
});

describe("timeline row — name, size, type and a download action; nothing for no files", () => {
  it("renders each file; an empty list draws nothing", () => {
    const html = renderToStaticMarkup(
      <TimelineAttachments
        taskCode="NV19"
        attachments={[{ id: "f1", file_name: "bien-ban.pdf", mime_type: "application/pdf", size_bytes: 2048, status: "stored" }]}
      />,
    );
    expect(html).toContain("bien-ban.pdf · 2 KB · PDF");
    expect(html).toContain('aria-label="Tải về bien-ban.pdf"');
    expect(renderToStaticMarkup(<TimelineAttachments taskCode="NV19" attachments={[]} />)).toBe("");
  });
});

describe("wiring (source)", () => {
  const UI = readFileSync(fileURLToPath(new URL("./task-attachments-ui.tsx", import.meta.url)), "utf8");
  const API = readFileSync(fileURLToPath(new URL("../../lib/api/task-attachments.ts", import.meta.url)), "utf8");
  const LOG = readFileSync(fileURLToPath(new URL("./nhat-ky-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("flow a → b → c in order; the upload never sends a cookie; the retry re-completes, never re-uploads", () => {
    const a = UI.indexOf("await requestAttachmentUpload(");
    const b = UI.indexOf("await uploadToStorage(");
    const c = UI.indexOf("await complete(key, id);");
    expect(a).toBeGreaterThan(-1);
    expect(a).toBeLessThan(b);
    expect(b).toBeLessThan(c);
    expect(API).toContain("xhr.withCredentials = false;");
    expect(UI).toContain('if (it !== undefined && it.state.kind === "retry") void complete(key, it.state.id);');
  });

  it("the download link is asked for at the click and opened detached — never stored in state", () => {
    expect(UI).toContain('const tab = window.open("", "_blank");');
    expect(UI).toContain("tab.opener = null;");
    expect(UI).not.toMatch(/set\w*\(r\.data\.url\)/);
  });

  it("the entry waits for files in flight, sends stored ids, clears the list after 201; rows render files", () => {
    expect(LOG).toContain("disabled={sending || waiting || note === null}");
    expect(LOG).toContain("files.clear();");
    expect(LOG).toContain("<TimelineAttachments taskCode={maNhiemVu} attachments={d.attachments ?? []} />");
  });
});
