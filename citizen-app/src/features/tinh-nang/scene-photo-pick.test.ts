import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE SHELL'S HALF OF THE SCENE PHOTOS — `takeScenePhoto` / `chooseScenePhotos` in `zalo-api.ts`, and the table
 * in `App.tsx` that turns them into the state half's `ScenePhotoPickResult`.
 *
 * What must hold:
 *   · the camera asks `requestCameraPermission` FIRST; `userAllow: false` is the citizen's no, and the camera
 *     is then never opened. Since 08/10/2026 it is the PHONE'S camera (`chooseImage`), not Zalo's (`zcamera_photo`)
 *   · the picker gets the right type and count, the edit view off — and NEVER `serverUploadUrl`: the picker
 *     uploads nothing, it hands back local temp paths
 *   · the SDK's own citizen codes (`constants.js`: -2002 denied, -2003 cancel) are what they are; any other
 *     code carries the capability and the code, for the "Mã hỗ trợ" line
 *
 * `zmp-sdk` is replaced because the real module needs a Zalo runtime; the functions under test are real.
 */
const sdk = vi.hoisted(() => ({
  requestCameraPermission: vi.fn<() => Promise<{ userAllow: boolean; message: string }>>(),
  openMediaPicker: vi.fn<(args: Record<string, unknown>) => Promise<{ data: string[] | string }>>(),
  chooseImage: vi.fn<(args: Record<string, unknown>) => Promise<{ filePaths: string[] }>>(),
}));
vi.mock("zmp-sdk", () => sdk);
// Coded failures here would POST a real report on a machine whose `.env.local` sets the API host.
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError: async () => {} }));

import { toScenePhotoPickResult } from "../../App";
import { CONSENT_DIALOG, SCENE_PHOTOS, XA_TN } from "../../cong-dan/man/noi-dung";

import { chooseScenePhotos, KHAI_BAO_LOI_GOI, takeScenePhoto } from "./zalo-api";

const zaloError = (code: number) => Object.assign(new Error("platform text that must not travel"), { code });

beforeEach(() => {
  sdk.requestCameraPermission.mockReset().mockResolvedValue({ userAllow: true, message: "" });
  sdk.openMediaPicker.mockReset().mockResolvedValue({ data: ["zalo-temp://a.jpg"] });
  sdk.chooseImage.mockReset().mockResolvedValue({ filePaths: ["blob:camera-1"] });
});

describe("Chụp ảnh — camera permission, then the PHONE'S camera for one photo (owner, 08/10/2026)", () => {
  it("allowed: `chooseImage` with the camera only, ONE photo, the BACK camera; Zalo's own camera is not used", async () => {
    expect(await takeScenePhoto()).toEqual({ kieu: "xong", du_lieu: ["blob:camera-1"] });
    expect(sdk.requestCameraPermission).toHaveBeenCalledTimes(1);
    expect(sdk.chooseImage.mock.calls).toEqual([[{ count: 1, sourceType: ["camera"], cameraType: "back" }]]);
    // The full-screen Zalo camera (`openMediaPicker` `zcamera_photo`) is what the owner asked to replace.
    expect(sdk.openMediaPicker).not.toHaveBeenCalled();
  });

  it("empty paths are dropped — an empty answer is `xong` with nothing to keep", async () => {
    sdk.chooseImage.mockResolvedValue({ filePaths: ["", "blob:camera-2"] });
    expect(await takeScenePhoto()).toEqual({ kieu: "xong", du_lieu: ["blob:camera-2"] });
    sdk.chooseImage.mockResolvedValue({ filePaths: [] });
    expect(await takeScenePhoto()).toEqual({ kieu: "xong", du_lieu: [] });
  });

  it("not allowed: tu-choi, and the camera is never opened", async () => {
    sdk.requestCameraPermission.mockResolvedValue({ userAllow: false, message: "denied" });
    expect(await takeScenePhoto()).toEqual({ kieu: "tu-choi" });
    expect(sdk.chooseImage).not.toHaveBeenCalled();
  });

  it("the permission call refused with a code: the CAMERA capability, the code verbatim", async () => {
    sdk.requestCameraPermission.mockRejectedValue(zaloError(-1403));
    expect(await takeScenePhoto()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "camera", code: -1403, transient: false },
    });
  });

  it("the camera step refused: the capability moves to PHOTOS (the step that failed)", async () => {
    sdk.chooseImage.mockRejectedValue(zaloError(-2004));
    expect(await takeScenePhoto()).toMatchObject({ kieu: "khong-lay-duoc", failure: { capability: "photos", code: -2004 } });
  });
});

/**
 * THE CAMERA CLOSED WITHOUT A PHOTO. `chooseImage` is a hidden file input that settles only on `change`, so a
 * closed camera never answers (`zalo-api.ts` `untilReturnWithoutPhoto`). The page coming back visible and focused,
 * with still no photo after the grace period, is `huy` — the photo buttons are free again. A photo that arrives
 * within the grace period is kept. A page with a window is faked here: this file runs under Node.
 */
describe("Chụp ảnh — the camera closed without a photo is `huy`, never a button stuck disabled", () => {
  type Listener = () => void;
  let listeners: Map<string, Listener[]>;
  const fire = (type: string) => listeners.get(type)?.forEach((l) => l());
  const target = () => ({
    addEventListener: (type: string, l: Listener) => listeners.set(type, [...(listeners.get(type) ?? []), l]),
    removeEventListener: (type: string, l: Listener) =>
      listeners.set(type, (listeners.get(type) ?? []).filter((x) => x !== l)),
  });

  beforeEach(() => {
    listeners = new Map();
    vi.useFakeTimers();
    vi.stubGlobal("window", target());
    vi.stubGlobal("document", { ...target(), visibilityState: "visible" });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("back on the page, nothing after the grace period → huy; the listeners are removed", async () => {
    sdk.chooseImage.mockReturnValue(new Promise(() => {}));
    const pick = takeScenePhoto();
    await vi.advanceTimersByTimeAsync(0);
    expect(sdk.chooseImage).toHaveBeenCalledTimes(1);
    fire("focus");
    await vi.advanceTimersByTimeAsync(2999);
    let settled = false;
    void pick.then(() => (settled = true));
    await vi.advanceTimersByTimeAsync(0);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(await pick).toEqual({ kieu: "huy" });
    expect([...listeners.values()].flat()).toEqual([]);
  });

  it("a photo delivered just after the page came back is kept", async () => {
    let deliver: (v: { filePaths: string[] }) => void = () => {};
    sdk.chooseImage.mockReturnValue(new Promise((r) => (deliver = r)));
    const pick = takeScenePhoto();
    await vi.advanceTimersByTimeAsync(0);
    fire("visibilitychange");
    await vi.advanceTimersByTimeAsync(500);
    deliver({ filePaths: ["blob:camera-3"] });
    expect(await pick).toEqual({ kieu: "xong", du_lieu: ["blob:camera-3"] });
    expect([...listeners.values()].flat()).toEqual([]);
  });

  it("no return to the page → no timer: the call is simply still open", async () => {
    sdk.chooseImage.mockReturnValue(new Promise(() => {}));
    let settled = false;
    void takeScenePhoto().then(() => (settled = true));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(settled).toBe(false);
  });
});

describe("Chọn ảnh có sẵn — the photo picker, at most the slots left", () => {
  it("type photo, maxSelectItem = slots, a single string normalised, empty paths dropped", async () => {
    sdk.openMediaPicker.mockResolvedValue({ data: ["zalo-temp://a.jpg", "", "zalo-temp://b.jpg"] });
    expect(await chooseScenePhotos(3)).toEqual({ kieu: "xong", du_lieu: ["zalo-temp://a.jpg", "zalo-temp://b.jpg"] });
    const args = sdk.openMediaPicker.mock.calls[0]![0];
    expect(args).toMatchObject({ type: "photo", maxSelectItem: 3 });
    expect(args).not.toHaveProperty("serverUploadUrl");
    sdk.openMediaPicker.mockResolvedValue({ data: "zalo-temp://c.jpg" });
    expect(await chooseScenePhotos(1)).toEqual({ kieu: "xong", du_lieu: ["zalo-temp://c.jpg"] });
  });

  it("the citizen closing the picker is `huy`, not a failure; -2002 is their no", async () => {
    sdk.openMediaPicker.mockRejectedValue(zaloError(-2003));
    expect(await chooseScenePhotos(5)).toEqual({ kieu: "huy" });
    sdk.openMediaPicker.mockRejectedValue(zaloError(-2002));
    expect(await chooseScenePhotos(5)).toEqual({ kieu: "tu-choi" });
  });
});

describe("App.tsx — the bridge table", () => {
  it("each branch crosses as what the citizen does next", () => {
    expect(toScenePhotoPickResult({ kieu: "xong", du_lieu: ["p"] })).toEqual({ kind: "xong", paths: ["p"] });
    expect(toScenePhotoPickResult({ kieu: "huy" })).toEqual({ kind: "huy" });
    expect(toScenePhotoPickResult({ kieu: "tu-choi" })).toEqual({ kind: "tu-choi" });
    expect(toScenePhotoPickResult({ kieu: "ngoai-zalo" })).toEqual({ kind: "ngoai-zalo" });
    expect(toScenePhotoPickResult({ kieu: "khong-lay-duoc" })).toEqual({ kind: "thu-lai" });
    expect(
      toScenePhotoPickResult({ kieu: "khong-lay-duoc", failure: { capability: "photos", code: -1403, transient: false } }),
    ).toEqual({ kind: "thu-lai", zalo: { capability: "photos", code: -1403, transient: false } });
  });
});

/**
 * THE ZALO SUBMISSION DECLARES THE MOMENT THE APP ACTUALLY HAS (owner, 09/10/2026): the long explanation card
 * became one question with "Cho phép" / "Không". A row still describing the card — or not naming the question a
 * reviewer will see on screen — sends the reviewer looking for a screen that is not there.
 */
describe("the commune app's declarations name the short questions, word for word", () => {
  const row = (api: string) => KHAI_BAO_LOI_GOI.find((r) => r.api === api)!;

  it("camera and photo picker: the question's exact words, and 'Cho phép' before Zalo", () => {
    expect(row("requestCameraPermission").commune_app!.de_lam_gi).toContain(`“${SCENE_PHOTOS.camera_question}”`);
    expect(row("openMediaPicker").commune_app!.de_lam_gi).toContain(`“${SCENE_PHOTOS.library_question}”`);
    for (const api of ["requestCameraPermission", "openMediaPicker", "chooseImage"]) {
      expect(row(api).commune_app!.de_lam_gi, api).toContain(`“${CONSENT_DIALOG.allow}”`);
    }
    expect(row("chooseImage").de_lam_gi).toContain(`“${CONSENT_DIALOG.allow}”`);
  });

  it("getUserInfo: the third moment — a number already confirmed, then the name question", () => {
    expect(row("getUserInfo").de_lam_gi).toContain(`“${XA_TN.name_prompt_question}”`);
    expect(row("getUserInfo").commune_app!.de_lam_gi).toContain(`“${XA_TN.name_prompt_question}”`);
  });

  it("no row still speaks of the old card's buttons", () => {
    for (const r of KHAI_BAO_LOI_GOI) {
      for (const text of [r.de_lam_gi, r.commune_app?.de_lam_gi ?? ""]) {
        expect(text, r.api).not.toMatch(/“Tiếp tục”|“Để sau”|đã nói rõ ảnh dùng để làm gì/);
      }
    }
  });
});
