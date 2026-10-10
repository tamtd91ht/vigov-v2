// @vitest-environment jsdom
//
// jsdom for this file: the officer PICKS an audio file on the create form and presses Lưu — the order of the
// requests that one press sends (the item first, then the ONE audio upload with the new id) is
// the whole point (prototype `ContentItemForm.tsx:150-180`).

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing export (rule 12 inv 3)
import { installFakeUploadXHR, type FakeAnswer } from "@/lib/api/upload-test-support";

import { AUDIO_SAVED_NOT_UPLOADED } from "./broadcast-audio";
import { CREATED_DRAFT_TOAST, UPLOADING_FILES_LABEL } from "./nhan-noi-dung";
import { SoNoiDung } from "./so-noi-dung"; // vi-name-ok: existing export (rule 12 inv 3)

const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  T.success.mockClear();
  T.error.mockClear();
});

const NEW_ID = "01JNEWBROADCAST00000000000";
const AUDIO_ID = "01JAUDIO000000000000000001";

const CREATED = {
  id: NEW_ID,
  type: "truyen-thanh",
  category_id: "",
  title: "Bản tin sáng",
  summary: "",
  body: "",
  image_url: "",
  has_image: false,
  published_on: "",
  view_count: 0,
  status: "an",
  source: "soan-tay",
  source_url: "",
  source_ref: "",
  hand_edited: false,
  author_code: "CB-00123",
  created_at: "2026-10-09T02:00:00Z",
  updated_at: "2026-10-09T02:00:00Z",
};

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

type Call = { path: string; method: string; body: Record<string, unknown> | undefined };

/** The upload's text parts as the server reads them — `size` is the file's byte count, a string. */
function recordUploads(calls: Call[], answer: () => FakeAnswer | Promise<FakeAnswer>): void {
  installFakeUploadXHR((req) => {
    calls.push({
      path: req.url,
      method: req.method,
      body: Object.fromEntries(req.parts.filter(([, v]) => typeof v === "string")) as Record<string, unknown>,
    });
    return answer();
  });
}

/** `upload`: the audio upload's answer — `ready` by default. */
function fakeServer(upload: () => FakeAnswer | Promise<FakeAnswer> = () => ({ status: 201, body: { id: AUDIO_ID, content_item_id: NEW_ID, status: "ready", mime_type: "audio/mpeg", size_bytes: 3, duration_seconds: 750 } })) {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit): Promise<Response> => {
      const method = init?.method ?? "GET";
      calls.push({ path, method, body: init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : undefined });
      if (path === "/api/v1/sessions/current") return json(200, { permissions: ["content.read", "content.update"], must_change_password: false });
      if (method === "POST" && path === "/api/v1/content-items") return json(201, CREATED);
      if (method === "GET" && path === `/api/v1/content-items/${NEW_ID}`) return json(200, CREATED);
      if (method === "GET" && path.startsWith("/api/v1/content-items")) return json(200, { items: [], has_more: false, next_cursor: "" });
      if (path.startsWith("/api/v1/content-categories") || path.startsWith("/api/v1/commune-staff")) return json(200, { items: [] });
      return json(404, { code: "not_found", message: "Không có." });
    }),
  );
  recordUploads(calls, upload);
  return calls;
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

async function settle(): Promise<void> {
  for (let i = 0; i < 10; i += 1) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function typeInto(el: HTMLInputElement, value: string): void {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function pickFile(input: HTMLInputElement, file: File): Promise<void> {
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  await act(async () => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

const mp3 = () => new File([new Uint8Array([0x49, 0x44, 0x33])], "ban-tin-sang.mp3", { type: "audio/mpeg" });

/** The screen, the `Truyền thanh` tab, `Thêm nội dung`, a title, a duration and a picked file. */
async function fillBroadcast(duration = "750"): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <PhienProvider>
        <SoNoiDung />
      </PhienProvider>,
    ),
  );
  await settle();
  const byText = (t: string) => Array.from(host!.querySelectorAll("button")).find((b) => b.textContent === t)!;
  await act(async () => byText("Truyền thanh").click());
  await settle();
  await act(async () => byText("Thêm nội dung").click());
  await settle();
  typeInto(host!.querySelector<HTMLInputElement>("#tieu-de-noi-dung")!, "Bản tin sáng");
  typeInto(host!.querySelector<HTMLInputElement>("#thoi-luong-am-thanh")!, duration);
  await pickFile(host!.querySelector<HTMLInputElement>("#tep-am-thanh")!, mp3());
}

async function pressSave(): Promise<void> {
  await act(async () => {
    host!.querySelector<HTMLFormElement>("dialog form")!.requestSubmit();
  });
}

describe("Truyền thanh — the audio picked at create time, uploaded after the POST", () => {
  it("the create form offers the picker at once (no save-first notice); the picked file is only held", async () => {
    const calls = fakeServer();
    await fillBroadcast();
    const dialog = host!.querySelector("dialog")!;
    expect(dialog.textContent).not.toContain("bấm Lưu trước");
    expect(dialog.textContent).toContain("ban-tin-sang.mp3");
    // Picking sends nothing: no item exists yet to attach it to.
    expect(calls.filter((c) => c.path.includes("audio-files"))).toHaveLength(0);
  });

  it("one Lưu: POST the item, then ONE upload with the NEW id and the typed duration — no completion call", async () => {
    const calls = fakeServer();
    await fillBroadcast("750");
    await pressSave();
    await settle();
    const writes = calls.filter((c) => c.method === "POST" && c.path !== "/api/v1/sessions/current");
    expect(writes.map((c) => c.path)).toEqual(["/api/v1/content-items", "/api/v1/content-items/audio-files"]);
    expect(writes[0]!.body).toMatchObject({ type: "truyen-thanh", title: "Bản tin sáng" });
    expect(writes[1]!.body).toEqual({
      size: "3",
      file_name: "ban-tin-sang.mp3",
      content_type: "audio/mpeg",
      content_item_id: NEW_ID,
      audio_duration_seconds: "750",
    });
    expect(T.success).toHaveBeenCalledWith(CREATED_DRAFT_TOAST);
    expect(T.error).not.toHaveBeenCalled();
    expect(host!.querySelector("dialog")).toBeNull();
  });

  it("while the file moves, Lưu says `Đang tải tệp…`", async () => {
    let release: (a: FakeAnswer) => void = () => {};
    const gate = new Promise<FakeAnswer>((r) => {
      release = r;
    });
    fakeServer(() => gate);
    await fillBroadcast();
    await pressSave();
    await settle();
    const submit = host!.querySelector<HTMLButtonElement>('dialog button[type="submit"]')!;
    expect(submit.textContent).toBe(UPLOADING_FILES_LABEL);
    expect(submit.disabled).toBe(true);
    await act(async () => release({ status: 201, body: { id: AUDIO_ID, content_item_id: NEW_ID, status: "ready", mime_type: "audio/mpeg", size_bytes: 3, duration_seconds: 750 } }));
    await settle();
  });

  it("a failed upload after the save: the item stays saved, a dialog stays open with the prototype's sentence", async () => {
    const calls = fakeServer(() => ({ status: 422, body: { code: "audio_rejected", message: "Tệp không phải âm thanh hợp lệ." } }));
    await fillBroadcast();
    await pressSave();
    await settle();
    expect(T.error).toHaveBeenCalledWith(AUDIO_SAVED_NOT_UPLOADED);
    expect(AUDIO_SAVED_NOT_UPLOADED).toBe("Đã lưu nội dung nhưng chưa tải được tệp lên.");
    expect(T.success).not.toHaveBeenCalled();
    const dialog = host!.querySelector("dialog")!;
    expect(dialog).not.toBeNull();
    expect(dialog.textContent).toContain(AUDIO_SAVED_NOT_UPLOADED);
    // The upload's own reason, verbatim.
    expect(dialog.textContent).toContain("Tệp không phải âm thanh hợp lệ.");
    // It is now the EDIT dialog of the saved item: a second Lưu cannot create a second item.
    expect(dialog.querySelector("h3")!.textContent).toBe("Sửa nội dung");
    expect(calls.filter((c) => c.method === "POST" && c.path === "/api/v1/content-items")).toHaveLength(1);
  });

  it("a picked file with no valid duration: nothing is sent, the duration box says why", async () => {
    const calls = fakeServer();
    await fillBroadcast("");
    await pressSave();
    await settle();
    expect(calls.filter((c) => c.method === "POST" && c.path.startsWith("/api/v1/content-items"))).toHaveLength(0);
    expect(host!.querySelector("dialog")!.textContent).toContain("Nhập thời lượng của tệp âm thanh");
  });
});

/* ── Editing a saved broadcast: the same two boxes, Lưu = PATCH then (if a file was picked) the upload ── */

const SAVED_ID = "01JSAVEDBROADCAST000000000";

const SAVED_ROW = {
  ...CREATED,
  id: SAVED_ID,
  title: "Bản tin chiều",
  audio_file_id: "01JOLDAUDIO000000000000001",
  audio_duration_seconds: 3900,
  audio: { file_id: "01JOLDAUDIO000000000000001", status: "ready", mime_type: "audio/mpeg", size_bytes: 9, duration_seconds: 3900 },
};

const NO_AUDIO_ROW = { ...CREATED, id: SAVED_ID, title: "Bản tin chiều" };

function editServer(row: Record<string, unknown>): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit): Promise<Response> => {
      const method = init?.method ?? "GET";
      calls.push({ path, method, body: init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : undefined });
      if (path === "/api/v1/sessions/current") return json(200, { permissions: ["content.read", "content.update"], must_change_password: false });
      if (method === "PATCH" && path === `/api/v1/content-items/${SAVED_ID}`) return json(200, row);
      if (method === "GET" && path === `/api/v1/content-items/${SAVED_ID}`) return json(200, row);
      if (method === "GET" && path.startsWith("/api/v1/content-items")) return json(200, { items: [row], has_more: false, next_cursor: "" });
      if (path.startsWith("/api/v1/content-categories") || path.startsWith("/api/v1/commune-staff")) return json(200, { items: [] });
      return json(404, { code: "not_found", message: "Không có." });
    }),
  );
  recordUploads(calls, () => ({
    status: 201,
    body: { id: AUDIO_ID, content_item_id: SAVED_ID, status: "ready", mime_type: "audio/mpeg", size_bytes: 3, duration_seconds: 3900 },
  }));
  return calls;
}

async function openEdit(title: string): Promise<HTMLDialogElement> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <PhienProvider>
        <SoNoiDung />
      </PhienProvider>,
    ),
  );
  await settle();
  const titleButton = Array.from(host.querySelectorAll("button")).find((b) => b.textContent === title)!;
  await act(async () => titleButton.click());
  await settle();
  return host.querySelector("dialog")!;
}

const writesOf = (calls: Call[]) =>
  calls
    .filter((c) => c.method !== "GET" && c.path !== "/api/v1/sessions/current")
    .map((c) => ({ method: c.method, path: c.path, body: c.body }));

describe("Truyền thanh — editing a saved broadcast", () => {
  it("the same two boxes as create: the saved duration in seconds and `Đã có tệp — chọn tệp mới để thay`; no player", async () => {
    editServer(SAVED_ROW);
    const dialog = await openEdit("Bản tin chiều");
    expect(dialog.querySelector<HTMLInputElement>("#thoi-luong-am-thanh")!.value).toBe("3900");
    expect(dialog.textContent).toContain("Đã có tệp — chọn tệp mới để thay");
    expect(dialog.querySelector("audio")).toBeNull();
    expect(dialog.textContent).not.toContain("Gỡ âm thanh");
  });

  it("a new file: ONE PATCH removing the old file, then ONE upload with the duration", async () => {
    const calls = editServer(SAVED_ROW);
    await openEdit("Bản tin chiều");
    await pickFile(host!.querySelector<HTMLInputElement>("#tep-am-thanh")!, mp3());
    expect(host!.querySelector("dialog")!.textContent).toContain("ban-tin-sang.mp3");
    await pressSave();
    await settle();
    expect(writesOf(calls)).toEqual([
      { method: "PATCH", path: `/api/v1/content-items/${SAVED_ID}`, body: { audio_file_id: "" } },
      {
        method: "POST",
        path: "/api/v1/content-items/audio-files",
        body: {
          size: "3",
          file_name: "ban-tin-sang.mp3",
          content_type: "audio/mpeg",
          content_item_id: SAVED_ID,
          audio_duration_seconds: "3900",
        },
      },
    ]);
    expect(T.error).not.toHaveBeenCalled();
    expect(host!.querySelector("dialog")).toBeNull();
  });

  it("only the duration changed: one PATCH `{audio_duration_seconds}`, no upload", async () => {
    const calls = editServer(SAVED_ROW);
    await openEdit("Bản tin chiều");
    typeInto(host!.querySelector<HTMLInputElement>("#thoi-luong-am-thanh")!, "4000");
    await pressSave();
    await settle();
    expect(writesOf(calls)).toEqual([
      { method: "PATCH", path: `/api/v1/content-items/${SAVED_ID}`, body: { audio_duration_seconds: 4000 } },
    ]);
  });

  it("a broadcast with no file, only a file picked: no PATCH (nothing else changed), the upload alone", async () => {
    const calls = editServer(NO_AUDIO_ROW);
    const dialog = await openEdit("Bản tin chiều");
    expect(dialog.textContent).toContain("Chọn tệp từ máy");
    typeInto(host!.querySelector<HTMLInputElement>("#thoi-luong-am-thanh")!, "750");
    await pickFile(host!.querySelector<HTMLInputElement>("#tep-am-thanh")!, mp3());
    await pressSave();
    await settle();
    expect(writesOf(calls).map((c) => `${c.method} ${c.path}`)).toEqual(["POST /api/v1/content-items/audio-files"]);
  });
});
