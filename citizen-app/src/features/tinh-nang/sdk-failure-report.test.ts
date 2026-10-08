import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * A CODED ZALO SDK FAILURE IS REPORTED (owner, 08/10/2026) — so the SDK's `message`, the only thing that tells
 * the causes of a -1401 apart, reaches the `vihat-miniapp` log read in Rancher.
 */

const reportClientError = vi.fn(async () => {});
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError }));

const sdk = { getAccessToken: vi.fn(), getPhoneNumber: vi.fn() };
vi.mock("zmp-sdk", () => sdk);

const { xinMaDangNhap } = await import("./zalo-api");
const { CLIENT_ERROR_FIELDS, clientErrorBody } = await import("../dang-nhap/hop-dong");

afterEach(() => {
  vi.clearAllMocks();
});

describe("reporting a failed SDK call", () => {
  it("-1401 on the access token: reports capability, code and the SDK's message verbatim", async () => {
    sdk.getAccessToken.mockRejectedValue({ code: -1401, message: "Login failed: Zalo app has not been activated" });
    const r = await xinMaDangNhap();

    expect(r.kieu).toBe("khong-lay-duoc");
    expect(reportClientError).toHaveBeenCalledTimes(1);
    expect(reportClientError).toHaveBeenCalledWith(
      expect.objectContaining({
        capability: "access-token",
        code: -1401,
        message: "Login failed: Zalo app has not been activated",
      }),
    );
  });

  it("names the step that failed: the phone step after a good access token", async () => {
    sdk.getAccessToken.mockResolvedValue("AT-SENTINEL");
    sdk.getPhoneNumber.mockRejectedValue({ code: -1401, message: "x" });
    await xinMaDangNhap();

    expect(reportClientError).toHaveBeenCalledWith(expect.objectContaining({ capability: "phone", code: -1401 }));
    // The access token obtained before the failure is never part of the report.
    expect(JSON.stringify(reportClientError.mock.calls)).not.toContain("AT-SENTINEL");
  });

  it("does not report the citizen's own refusal (-201) nor a throw without a code", async () => {
    sdk.getAccessToken.mockRejectedValueOnce({ code: -201 });
    await xinMaDangNhap();
    sdk.getAccessToken.mockRejectedValueOnce(new Error("no code"));
    await xinMaDangNhap();

    expect(reportClientError).not.toHaveBeenCalled();
  });
});

describe("report body", () => {
  it("keys are exactly the declared fields; message cut to 200; unknown app/host left out", () => {
    const full = JSON.parse(
      clientErrorBody({ capability: "phone", code: -1401, message: "m".repeat(500), app_id: "123", host: "xa.vigov.vn" }),
    ) as Record<string, unknown>;
    expect(Object.keys(full).sort()).toEqual(CLIENT_ERROR_FIELDS.map((f) => f.khoa).sort());
    expect((full["message"] as string).length).toBe(200);

    const bare = JSON.parse(
      clientErrorBody({ capability: "phone", code: -1401, message: "", app_id: null, host: null }),
    ) as Record<string, unknown>;
    expect(Object.keys(bare).sort()).toEqual(["capability", "code", "message"]);
  });
});
