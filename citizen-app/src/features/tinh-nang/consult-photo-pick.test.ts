import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE VIHAT INTRO'S TWO PHOTO CALLS — `takeConsultPhoto` / `chooseConsultPhotos` in `zalo-api.ts` (owner 12/10/2026,
 * "Tư vấn và báo giá"). What must hold:
 *   · the camera asks `requestCameraPermission` first and opens the phone's back camera for ONE photo;
 *   · the picker gets the type, the count, the edit view off, the SDK's medium compression — and NEVER
 *     `serverUploadUrl`: it hands back local paths, the upload is `api/goi-may-chu.ts`'s, after the request exists;
 *   · the citizen's own codes are what they are (-2003 closed → `huy`, -2002 → `tu-choi`).
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

import { chooseConsultPhotos, takeConsultPhoto } from "./zalo-api";

const zaloError = (code: number) => Object.assign(new Error("platform text"), { code });

beforeEach(() => {
  sdk.requestCameraPermission.mockReset().mockResolvedValue({ userAllow: true, message: "" });
  sdk.openMediaPicker.mockReset().mockResolvedValue({ data: ["zalo-temp://a.jpg"] });
  sdk.chooseImage.mockReset().mockResolvedValue({ filePaths: ["blob:camera-1"] });
});

describe("Chụp ảnh on Tư vấn và báo giá", () => {
  it("permission first, then the back camera for one photo; a no never opens the camera", async () => {
    expect(await takeConsultPhoto()).toEqual({ kieu: "xong", du_lieu: ["blob:camera-1"] });
    expect(sdk.requestCameraPermission).toHaveBeenCalledTimes(1);
    expect(sdk.chooseImage.mock.calls).toEqual([[{ count: 1, sourceType: ["camera"], cameraType: "back" }]]);
    sdk.chooseImage.mockClear();
    sdk.requestCameraPermission.mockResolvedValue({ userAllow: false, message: "" });
    expect(await takeConsultPhoto()).toEqual({ kieu: "tu-choi" });
    expect(sdk.chooseImage).not.toHaveBeenCalled();
  });
});

describe("Chọn ảnh on Tư vấn và báo giá", () => {
  it("photo type, the room as the count, no crop, medium compression, and no upload URL", async () => {
    sdk.openMediaPicker.mockResolvedValue({ data: ["a", "", "b", "c"] });
    expect(await chooseConsultPhotos(2)).toEqual({ kieu: "xong", du_lieu: ["a", "b"] });
    const args = sdk.openMediaPicker.mock.calls[0]![0];
    expect(args).toEqual({ type: "photo", maxSelectItem: 2, editView: { enable: false }, compressLevel: 2 });
    expect(args).not.toHaveProperty("serverUploadUrl");
  });

  it("closed is `huy`, -2002 is the person's no", async () => {
    sdk.openMediaPicker.mockRejectedValue(zaloError(-2003));
    expect(await chooseConsultPhotos(3)).toEqual({ kieu: "huy" });
    sdk.openMediaPicker.mockRejectedValue(zaloError(-2002));
    expect(await chooseConsultPhotos(3)).toEqual({ kieu: "tu-choi" });
  });
});
