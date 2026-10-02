import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  AFTER_PHOTO_CLOSED,
  AFTER_PHOTO_GO_TO,
  AFTER_PHOTO_REQUIRED_HINT,
  AFTER_PHOTO_UPLOAD_BUTTON,
  AFTER_PHOTO_UPLOAD_DENIED,
  AFTER_PHOTOS_LOADING,
  congThaoTac,
} from "./nhan-phieu";
import { ChiTietPhieu } from "./so-phan-anh";

/**
 * The drawer's two new pieces: the `Sau khi xử lý` column (inside the photo card) and the close
 * block's answer to 409 `after_photo_required`. Both are UX over a server decision, so the DENIED
 * cases are pinned as hard as the allowed ones.
 */

function petition(change: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "cho-dan-xac-nhan",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ.",
    address: "Tổ 6",
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-10-02T02:00:00Z",
    booked_at: "2026-10-02T02:01:00Z",
    acknowledge_due: "2026-10-02T04:00:00Z",
    resolve_due: "2026-10-03T04:00:00Z",
    classify_due: "2026-10-02T06:00:00Z",
    unit: "",
    assignee: "",
    result: "",
    public: false,
    ...change,
  };
}

function drawer(permissions: readonly string[], p = petition(), closeRefusal: string | null = null): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={new Date("2026-10-02T03:00:00Z")}
      cong={congThaoTac(false, false, permissions.includes("feedback.resolve"))}
      tenBoPhan={new Map()}
      boPhan={[]}
      danhBa={null}
      dangGui={false}
      loiGhi={null}
      permissions={permissions}
      closeRefusal={closeRefusal}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
    />,
  );
}

const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");

describe("`Sau khi xử lý` in the drawer", () => {
  it("with `feedback.read` + `feedback.resolve` on an open petition: the column loads and offers the upload", () => {
    const html = drawer(["feedback.read", "feedback.resolve"]);
    expect(html).toContain('id="sau-xu-ly-PA-2026-0021"');
    expect(html).toContain(AFTER_PHOTOS_LOADING);
    expect(html).toContain(AFTER_PHOTO_UPLOAD_BUTTON);
    expect(html).not.toContain("Chưa dựng");
  });

  it("DENIED — `feedback.read` only: the list still loads, NO upload, the sentence names the key", () => {
    const html = drawer(["feedback.read"]);
    expect(html).toContain(AFTER_PHOTOS_LOADING);
    expect(html).not.toContain(AFTER_PHOTO_UPLOAD_BUTTON);
    expect(html).toContain(esc(AFTER_PHOTO_UPLOAD_DENIED));
  });

  it("DENIED — no `feedback.read`: no photo card at all", () => {
    const html = drawer(["feedback.resolve"]);
    expect(html).not.toContain('id="sau-xu-ly-PA-2026-0021"');
    expect(html).not.toContain(AFTER_PHOTO_UPLOAD_BUTTON);
  });

  it.each(["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"])(
    "`%s`: the server refuses an upload — no button, the sentence, even with the key",
    (status) => {
      const html = drawer(["feedback.read", "feedback.resolve"], petition({ status }));
      expect(html).not.toContain(AFTER_PHOTO_UPLOAD_BUTTON);
      expect(html).toContain(AFTER_PHOTO_CLOSED);
    },
  );
});

describe("the close block — 409 `after_photo_required`", () => {
  // A reworded commune sentence: no client copy of a default could make this green.
  const CAU = "Xã yêu cầu có ảnh nghiệm thu trước khi đóng phiếu.";

  it("the commune's sentence, VERBATIM, inside the close block, with the way to the upload", () => {
    const html = drawer(["feedback.read", "feedback.resolve"], petition(), CAU);
    const block = html.slice(html.indexOf('id="ket-qua-xu-ly"'));
    expect(block).toContain(CAU);
    expect(block).toContain(esc(AFTER_PHOTO_REQUIRED_HINT));
    expect(block).toContain(`href="#sau-xu-ly-PA-2026-0021"`);
    expect(block).toContain(AFTER_PHOTO_GO_TO);
    expect(block).toContain('role="alert"');
  });

  it("no refusal: nothing of it on the page", () => {
    const html = drawer(["feedback.read", "feedback.resolve"]);
    expect(html).not.toContain(esc(AFTER_PHOTO_REQUIRED_HINT));
    expect(html).not.toContain(AFTER_PHOTO_GO_TO);
  });
});

describe("the status sentence under the stepper — all nine (ADR 0027 Bổ sung 2026-10-02)", () => {
  it.each([
    ["da-tiep-nhan", "Phiếu vừa vào sổ, chưa phân cho ai."],
    ["da-dong", "Phiếu đã đóng. Phải có ảnh sau xử lý mới đóng được."],
    ["chuyen-cap-tren", "Vượt thẩm quyền của xã, đã chuyển lên cấp trên."],
  ])("`%s` → “%s”", (status, sentence) => {
    expect(drawer(["feedback.read"], petition({ status }))).toContain(sentence);
  });

  it("the overdue NUMBER is not on the drawer (ADR 0007 decision 10a): `Quá hạn` + the deadline only", () => {
    const html = drawer(["feedback.read"], petition({ resolve_due: "2026-10-01T00:00:00Z" }));
    expect(html).toContain("Quá hạn · hạn cuối");
    expect(html).not.toMatch(/Quá hạn \d+ (ngày|giờ)/);
    expect(html).not.toContain("late_working_seconds");
  });
});
