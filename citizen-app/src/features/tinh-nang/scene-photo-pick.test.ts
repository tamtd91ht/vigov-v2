import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE SHELL'S HALF OF THE SCENE PHOTOS — `takeScenePhoto` / `chooseScenePhotos` in `zalo-api.ts`, and the table
 * in `App.tsx` that turns them into the state half's `ScenePhotoPickResult`.
 *
 * What must hold:
 *   · the camera asks `requestCameraPermission` FIRST; `userAllow: false` is the citizen's no, and Zalo's
 *     camera is then never opened
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
}));
vi.mock("zmp-sdk", () => sdk);

import { toScenePhotoPickResult } from "../../App";

import { chooseScenePhotos, takeScenePhoto } from "./zalo-api";

const zaloError = (code: number) => Object.assign(new Error("platform text that must not travel"), { code });

beforeEach(() => {
  sdk.requestCameraPermission.mockReset().mockResolvedValue({ userAllow: true, message: "" });
  sdk.openMediaPicker.mockReset().mockResolvedValue({ data: ["zalo-temp://a.jpg"] });
});

describe("Chụp ảnh — camera permission, then Zalo's camera for one photo", () => {
  it("allowed: one zcamera_photo, edit view off, no upload URL", async () => {
    expect(await takeScenePhoto()).toEqual({ kieu: "xong", du_lieu: ["zalo-temp://a.jpg"] });
    expect(sdk.requestCameraPermission).toHaveBeenCalledTimes(1);
    const args = sdk.openMediaPicker.mock.calls[0]![0];
    expect(args).toMatchObject({ type: "zcamera_photo", maxSelectItem: 1, editView: { enable: false } });
    expect(args).not.toHaveProperty("serverUploadUrl");
  });

  it("not allowed: tu-choi, and the camera is never opened", async () => {
    sdk.requestCameraPermission.mockResolvedValue({ userAllow: false, message: "denied" });
    expect(await takeScenePhoto()).toEqual({ kieu: "tu-choi" });
    expect(sdk.openMediaPicker).not.toHaveBeenCalled();
  });

  it("the permission call refused with a code: the CAMERA capability, the code verbatim", async () => {
    sdk.requestCameraPermission.mockRejectedValue(zaloError(-1403));
    expect(await takeScenePhoto()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "camera", code: -1403, transient: false },
    });
  });

  it("the camera step refused: the capability moves to PHOTOS (the step that failed)", async () => {
    sdk.openMediaPicker.mockRejectedValue(zaloError(-2004));
    expect(await takeScenePhoto()).toMatchObject({ kieu: "khong-lay-duoc", failure: { capability: "photos", code: -2004 } });
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
