import { describe, expect, it } from "vitest";

import type { comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  contentRefusalText,
  ERR_VIDEO_DURATION_FORMAT,
  ERR_VIDEO_DURATION_RANGE,
  FORM_TRONG,
  formatVideoDuration,
  giaTriTuHang,
  parseVideoDuration,
  thanSua,
  thanThem,
  validateTypeFields,
} from "./nhan-noi-dung";

/** `Thời lượng video` (ADR 0091 §7, migration 0028): typed as m:ss, sent as `video_duration_seconds`, video only. */

const VIDEO = { ...FORM_TRONG, type: "video", title: "Hướng dẫn phòng chống bão", video_url: "https://example.vn/v" };

describe("parsing what the officer typed", () => {
  it("m:ss → seconds; empty = not given", () => {
    expect(parseVideoDuration("3:25")).toEqual({ ok: true, seconds: 205 });
    expect(parseVideoDuration(" 90:00 ")).toEqual({ ok: true, seconds: 5400 });
    expect(parseVideoDuration("0:01")).toEqual({ ok: true, seconds: 1 });
    expect(parseVideoDuration("360:00")).toEqual({ ok: true, seconds: 21600 });
    expect(parseVideoDuration("")).toEqual({ ok: true, seconds: null });
  });

  it("malformed or out of 1..21600 seconds is refused with a Vietnamese sentence", () => {
    for (const bad of ["3:5", "325", "3:60", "a:bc", "-1:00", "1:00:00"]) {
      expect(parseVideoDuration(bad)).toEqual({ ok: false, message: ERR_VIDEO_DURATION_FORMAT });
    }
    expect(parseVideoDuration("0:00")).toEqual({ ok: false, message: ERR_VIDEO_DURATION_RANGE });
    expect(parseVideoDuration("360:01")).toEqual({ ok: false, message: ERR_VIDEO_DURATION_RANGE });
  });

  it("seconds → m:ss for the edit form", () => {
    expect(formatVideoDuration(205)).toBe("3:25");
    expect(formatVideoDuration(5400)).toBe("90:00");
    expect(formatVideoDuration(undefined)).toBe("");
    expect(formatVideoDuration(0)).toBe("");
  });
});

describe("the per-type check holds Lưu on a malformed length", () => {
  it("video only", () => {
    expect(validateTypeFields({ ...VIDEO, video_duration: "3:5" })).toBe(ERR_VIDEO_DURATION_FORMAT);
    expect(validateTypeFields({ ...VIDEO, video_duration: "3:05" })).toBeNull();
    // A hidden box on another type is never checked — it is never sent.
    expect(validateTypeFields({ ...VIDEO, type: "tin-tuc", video_duration: "3:5" })).toBeNull();
  });
});

describe("request bodies", () => {
  it("POST: video_duration_seconds when given, for a video only", () => {
    expect(thanThem({ ...VIDEO, video_duration: "3:25" }).video_duration_seconds).toBe(205);
    expect(thanThem({ ...VIDEO, video_duration: "" })).not.toHaveProperty("video_duration_seconds");
    expect(thanThem({ ...VIDEO, type: "tin-tuc", video_duration: "3:25" })).not.toHaveProperty("video_duration_seconds");
  });

  it("PATCH: sent only when the seconds changed; an emptied box sends 0 (clear)", () => {
    const before = giaTriTuHang({
      id: "N1",
      type: "video",
      category_id: "",
      title: "Hướng dẫn",
      summary: "",
      body: "",
      image_url: "",
      status: "an",
      video_url: "https://example.vn/v",
      video_duration_seconds: 205,
    } as comms_noiDungRa);
    expect(before.video_duration).toBe("3:25");
    expect(thanSua(before, before)).not.toHaveProperty("video_duration_seconds");
    expect(thanSua(before, { ...before, video_duration: "3:25 " })).not.toHaveProperty("video_duration_seconds");
    expect(thanSua(before, { ...before, video_duration: "4:00" }).video_duration_seconds).toBe(240);
    expect(thanSua(before, { ...before, video_duration: "" }).video_duration_seconds).toBe(0);
  });
});

describe("the server's 422 in Vietnamese", () => {
  it("the wire name becomes the box's label; the rule stays the server's", () => {
    expect(contentRefusalText("`video_duration_seconds` phải là số giây từ 1 đến 21600 (6 giờ)")).toBe(
      "Thời lượng video phải là số giây từ 1 đến 21600 (6 giờ)",
    );
    expect(contentRefusalText("`video_duration_seconds` chỉ dùng cho loại video")).toBe(
      "Thời lượng video chỉ dùng cho loại video",
    );
    expect(contentRefusalText("Tiêu đề quá dài.")).toBe("Tiêu đề quá dài.");
  });
});
