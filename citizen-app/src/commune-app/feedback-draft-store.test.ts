import { afterEach, describe, expect, it } from "vitest";

import type { NhapPhieu } from "../cong-dan";

import {
  createFeedbackDraftStore,
  FEEDBACK_DRAFT_KEY,
  feedbackDraftStore,
  type KeyValueStorage,
  sharedAppDraftKey,
  sharedAppDraftStore,
} from "./feedback-draft-store";

/** An in-memory Storage that records every write — tests only; production code touches the real one. */
function fakeStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  const writes: Array<{ key: string; value: string }> = [];
  const storage: KeyValueStorage = {
    getItem: (key) => (data.has(key) ? data.get(key)! : null),
    setItem: (key, value) => {
      writes.push({ key, value });
      data.set(key, value);
    },
    removeItem: (key) => {
      data.delete(key);
    },
  };
  return { storage, data, writes };
}

const DRAFT: NhapPhieu = {
  // A field CODE from the commune's catalogue (29/09/2026); the send screen restores it only if still offered.
  linh_vuc: "rac-thai",
  noi_dung: "Rác tồn đọng đầu ngõ 12",
  dia_chi: "Ngõ 12, thôn Đông",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: false,
};

const ALLOWED_KEYS = ["an_danh", "dia_chi", "dien_thoai", "ho_ten", "linh_vuc", "noi_dung"];

describe("feedback draft store — commune's own app only (ADR 0050 #7)", () => {
  it("round trip: what is saved is what is loaded", () => {
    const { storage } = fakeStorage();
    const store = createFeedbackDraftStore(() => storage);
    store.save(DRAFT);
    expect(store.load()).toEqual(DRAFT);
  });

  it("writes ONE fixed key, and only the six allowed fields — never extra ones from the caller", () => {
    const { storage, writes } = fakeStorage();
    const store = createFeedbackDraftStore(() => storage);
    // A caller object carrying more than the form fields (a token, a session id) must not reach storage.
    const polluted = { ...DRAFT, token: "phien-bi-mat", ma_phien: "x" } as unknown as NhapPhieu;
    store.save(polluted);
    expect(writes).toHaveLength(1);
    expect(writes[0]!.key).toBe(FEEDBACK_DRAFT_KEY);
    expect(FEEDBACK_DRAFT_KEY).toBe("vigov.feedback.draft.v1");
    expect(Object.keys(JSON.parse(writes[0]!.value) as object).sort()).toEqual(ALLOWED_KEYS);
    expect(writes[0]!.value).not.toContain("phien-bi-mat");
  });

  it("an anonymous draft still keeps name and phone, as require does (owner 28/09/2026)", () => {
    const { storage, writes } = fakeStorage();
    const store = createFeedbackDraftStore(() => storage);
    store.save({ ...DRAFT, an_danh: true });
    const stored = JSON.parse(writes[0]!.value) as NhapPhieu;
    expect(stored.ho_ten).toBe(DRAFT.ho_ten);
    expect(stored.dien_thoai).toBe(DRAFT.dien_thoai);
    expect(stored.an_danh).toBe(true);
    // Turning anonymity off again restores what was typed, instead of an empty form.
    expect(store.load()).toEqual({ ...DRAFT, an_danh: true });
  });

  it("bad JSON, wrong shape or wrong types → null, never a half-read draft", () => {
    for (const raw of [
      "{not json",
      "null",
      "42",
      '"chuoi"',
      "[]",
      JSON.stringify({ ...DRAFT, an_danh: "true" }),
      JSON.stringify({ ...DRAFT, noi_dung: 5 }),
      JSON.stringify({ linh_vuc: DRAFT.linh_vuc, noi_dung: DRAFT.noi_dung }),
    ]) {
      const { storage } = fakeStorage({ [FEEDBACK_DRAFT_KEY]: raw });
      expect(createFeedbackDraftStore(() => storage).load(), raw).toBeNull();
    }
  });

  it("reading rebuilds the six fields — extra keys planted in storage do not come back", () => {
    const { storage } = fakeStorage({ [FEEDBACK_DRAFT_KEY]: JSON.stringify({ ...DRAFT, token: "x" }) });
    const loaded = createFeedbackDraftStore(() => storage).load();
    expect(Object.keys(loaded ?? {}).sort()).toEqual(ALLOWED_KEYS);
  });

  it("an empty draft is no draft: nothing is asked, and saving one removes the old one", () => {
    const empty: NhapPhieu = { ...DRAFT, noi_dung: "  ", dia_chi: "" };
    const { storage, data } = fakeStorage({ [FEEDBACK_DRAFT_KEY]: JSON.stringify(empty) });
    const store = createFeedbackDraftStore(() => storage);
    expect(store.load()).toBeNull();
    store.save(DRAFT);
    expect(data.has(FEEDBACK_DRAFT_KEY)).toBe(true);
    store.save(empty);
    expect(data.has(FEEDBACK_DRAFT_KEY)).toBe(false);
    expect(fakeStorage().storage.getItem(FEEDBACK_DRAFT_KEY)).toBeNull();
    expect(createFeedbackDraftStore(() => fakeStorage().storage).load()).toBeNull();
  });

  it("clear removes the draft", () => {
    const { storage, data } = fakeStorage();
    const store = createFeedbackDraftStore(() => storage);
    store.save(DRAFT);
    store.clear();
    expect(data.has(FEEDBACK_DRAFT_KEY)).toBe(false);
    expect(store.load()).toBeNull();
  });

  it("storage that throws — on access or on any call — never crashes the screen", () => {
    const boom = () => {
      throw new Error("SecurityError");
    };
    const throwing: KeyValueStorage = { getItem: boom, setItem: boom, removeItem: boom };
    for (const store of [createFeedbackDraftStore(() => throwing), createFeedbackDraftStore(boom), createFeedbackDraftStore(() => null)]) {
      expect(() => store.save(DRAFT)).not.toThrow();
      expect(() => store.clear()).not.toThrow();
      expect(store.load()).toBeNull();
    }
  });
});

describe("the default store fails closed outside the commune's own app", () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  afterEach(() => {
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else delete (globalThis as { localStorage?: unknown }).localStorage;
  });

  it("with no fixed commune (shared app build, every test run) it never touches localStorage", () => {
    const { storage, writes, data } = fakeStorage({ [FEEDBACK_DRAFT_KEY]: JSON.stringify(DRAFT) });
    Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true, writable: true });
    feedbackDraftStore.save(DRAFT);
    feedbackDraftStore.clear();
    expect(writes).toEqual([]);
    expect(data.has(FEEDBACK_DRAFT_KEY), "clear reached storage in the shared app").toBe(true);
    expect(feedbackDraftStore.load(), "load read storage in the shared app").toBeNull();
  });
});

/**
 * THE SHARED APP'S STORE (owner 06/10/2026: the shared app borrows the commune's full interface from a QR).
 * One origin serves every commune there, so the key MUST name the commune — otherwise commune A's draft, with
 * the citizen's name and phone, is offered inside commune B (rule 1).
 */
describe("shared app draft store — one key per commune host", () => {
  const HOST_A = "xa-a.vigov.example";
  const HOST_B = "xa-b.vigov.example";
  const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  afterEach(() => {
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else delete (globalThis as { localStorage?: unknown }).localStorage;
  });

  it("the key is the own app's key plus the host — and differs between two communes", () => {
    expect(sharedAppDraftKey(HOST_A)).toBe(`${FEEDBACK_DRAFT_KEY}:${HOST_A}`);
    expect(sharedAppDraftKey(HOST_B)).not.toBe(sharedAppDraftKey(HOST_A));
    // Never the own app's bare key: that one belongs to a commune's own build.
    expect(sharedAppDraftKey(HOST_A)).not.toBe(FEEDBACK_DRAFT_KEY);
    // Case does not make a second commune (domains are case-insensitive).
    expect(sharedAppDraftKey("XA-A.vigov.example")).toBe(sharedAppDraftKey(HOST_A));
  });

  it("no well-formed host → no key (fail closed, never a default key)", () => {
    for (const bad of ["", "  ", "khong-co-cham", "https://xa-a.vigov.example", "xa a.vigov.example"]) {
      expect(sharedAppDraftKey(bad), bad).toBeNull();
    }
  });

  it("commune A's draft is never offered in commune B, and the own app's draft is never offered in either", () => {
    const { storage, writes } = fakeStorage({ [FEEDBACK_DRAFT_KEY]: JSON.stringify(DRAFT) });
    Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true, writable: true });
    const a = sharedAppDraftStore(HOST_A);
    const b = sharedAppDraftStore(HOST_B);
    // The bare own-app key is not read by the shared app.
    expect(a.load()).toBeNull();
    a.save(DRAFT);
    expect(writes.map((w) => w.key)).toEqual([`${FEEDBACK_DRAFT_KEY}:${HOST_A}`]);
    expect(a.load()).toEqual(DRAFT);
    expect(b.load(), "commune A's draft reached commune B").toBeNull();
    // Clearing B leaves A's draft alone; clearing A removes only A's.
    b.clear();
    expect(a.load()).toEqual(DRAFT);
    a.clear();
    expect(a.load()).toBeNull();
    expect(storage.getItem(FEEDBACK_DRAFT_KEY), "the own app's draft was touched").not.toBeNull();
  });

  it("a malformed host gets a store that touches nothing", () => {
    const { storage, writes, data } = fakeStorage({ [FEEDBACK_DRAFT_KEY]: JSON.stringify(DRAFT) });
    Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true, writable: true });
    const store = sharedAppDraftStore("");
    store.save(DRAFT);
    store.clear();
    expect(writes).toEqual([]);
    expect(store.load()).toBeNull();
    expect(data.has(FEEDBACK_DRAFT_KEY)).toBe(true);
  });
});
