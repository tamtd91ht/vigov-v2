import { afterEach, describe, expect, it } from "vitest";

import type { NhapPhieu } from "../cong-dan";

import { createFeedbackDraftStore, FEEDBACK_DRAFT_KEY, feedbackDraftStore, type KeyValueStorage } from "./feedback-draft-store";

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
  linh_vuc: "Rác thải – Vệ sinh môi trường",
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

  it("an anonymous draft keeps no name and no phone on the device", () => {
    const { storage, writes } = fakeStorage();
    const store = createFeedbackDraftStore(() => storage);
    store.save({ ...DRAFT, an_danh: true });
    const stored = JSON.parse(writes[0]!.value) as NhapPhieu;
    expect(stored.ho_ten).toBe("");
    expect(stored.dien_thoai).toBe("");
    expect(stored.an_danh).toBe(true);
    expect(writes[0]!.value).not.toContain(DRAFT.ho_ten);
    expect(writes[0]!.value).not.toContain(DRAFT.dien_thoai);
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
