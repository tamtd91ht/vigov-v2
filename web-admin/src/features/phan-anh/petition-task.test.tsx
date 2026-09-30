import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { createTaskFromPetition } from "@/lib/api/phieu-phan-anh";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhBaChonNguoiRa,
  petitions_nhatKyPhieuRa,
  petitions_phieuPhanAnhRa,
  petitions_taoNhiemVuVao,
} from "@/lib/api/schema.gen";

import {
  congThaoTac,
  danhBaTheoMa,
  LUONG_CHINH,
  MOI_TRANG_THAI,
  nhanThaoTacNhatKy,
  PETITION_TASK_BUTTON,
  petitionTaskOffered,
  petitionTaskTitle,
  TASK_REGISTER_HREF,
} from "./nhan-phieu";
import { DanhSachNhatKy } from "./nhat-ky-phieu";
import { PetitionTaskView } from "./petition-task";
import { ChiTietPhieu } from "./so-phan-anh";

/**
 * `Tạo nhiệm vụ` on the petition drawer — POST /api/v1/citizen-reports/{maTraCuu}/tasks.
 *
 * THE DENIED CASES ARE THE POINT: the developer's account holds every key, so a button leaking to an
 * account without `task.create`, or onto a closed or `can-bo` petition, is what nobody sees while
 * building. The server still refuses each of them; these tests keep the screen from offering them.
 */

const BOTH = ["task.create", "feedback.read"];

const TWO_KEYS_ALLOWED = ["da-chuyen-xu-ly", "dang-xu-ly", "da-xu-ly", "cho-dan-xac-nhan"];

function petition(change: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-xu-ly",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.",
    address: "Tổ 6, thôn Hà Lam",
    // Masked by the server; the agreed fake number (rule 3, invariant 5).
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: "2026-09-09T09:20:00Z",
    resolve_due: "2026-09-10T09:20:00Z",
    classify_due: "2026-09-09T11:20:00Z",
    unit: "01JBOPHAN",
    assignee: "",
    result: "",
    public: false,
    ...change,
  };
}

const DIRECTORY: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: {
    items: [
      { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "01JBOPHAN" },
    ],
  },
};

/** The button's own markup — its label also appears in the log table and must not be confused. */
const BUTTON = 'id="nut-tao-nhiem-vu-tu-phieu"';

function drawer(permissions: readonly string[], p = petition(), withWritePath = true): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={new Date("2026-09-10T02:00:00Z")}
      cong={congThaoTac(true, true, true)}
      tenBoPhan={new Map()}
      boPhan={[]}
      danhBa={DIRECTORY}
      dangGui={false}
      loiGhi={null}
      permissions={permissions}
      onTaskCreated={withWritePath ? () => {} : undefined}
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

describe("petitionTaskOffered — two keys, four statuses, never `can-bo`", () => {
  it("both keys, each of the four allowed statuses: offered", () => {
    for (const status of TWO_KEYS_ALLOWED) {
      expect(petitionTaskOffered(BOTH, { status, field: "rac-thai" }), status).toBe(true);
    }
  });

  it("DENIED — either key alone, or none, is not enough", () => {
    for (const keys of [["task.create"], ["feedback.read"], [], ["task.read", "feedback.assign"]]) {
      expect(petitionTaskOffered(keys, { status: "dang-xu-ly", field: "rac-thai" }), keys.join()).toBe(
        false,
      );
    }
  });

  it("DENIED — not yet classified, and the three terminal statuses", () => {
    for (const status of MOI_TRANG_THAI.filter((s) => !TWO_KEYS_ALLOWED.includes(s))) {
      expect(petitionTaskOffered(BOTH, { status, field: "rac-thai" }), status).toBe(false);
    }
    // The four allowed ones all come from the lifecycle — no typo passes silently.
    for (const s of TWO_KEYS_ALLOWED) expect(LUONG_CHINH).toContain(s);
  });

  it("DENIED — an unknown status code (fail closed)", () => {
    expect(petitionTaskOffered(BOTH, { status: "da-huy", field: "rac-thai" })).toBe(false);
  });

  it("DENIED — the staff-conduct field `can-bo`, and no settled field", () => {
    expect(petitionTaskOffered(BOTH, { status: "dang-xu-ly", field: "can-bo" })).toBe(false);
    expect(petitionTaskOffered(BOTH, { status: "dang-xu-ly", field: "" })).toBe(false);
  });
});

describe("the drawer draws the button only when offered", () => {
  it("both keys on an open petition: the button is there", () => {
    expect(drawer(BOTH)).toContain(BUTTON);
  });

  it("DENIED — without `task.create`, without `feedback.read`, and with an unknown session", () => {
    expect(drawer(["feedback.read"])).not.toContain(BUTTON);
    expect(drawer(["task.create"])).not.toContain(BUTTON);
    expect(drawer([])).not.toContain(BUTTON);
  });

  it("DENIED — closed, rejected, referred up, and `can-bo`", () => {
    for (const status of ["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"]) {
      expect(drawer(BOTH, petition({ status })), status).not.toContain(BUTTON);
    }
    expect(drawer(BOTH, petition({ field: "can-bo" }))).not.toContain(BUTTON);
  });

  it("DENIED — a caller that passes no write path draws nothing, whatever the keys", () => {
    expect(drawer(BOTH, petition(), false)).not.toContain(BUTTON);
  });

  it("the default of `permissions` is NO key (fail closed)", () => {
    const html = renderToStaticMarkup(
      <ChiTietPhieu
        phieu={petition()}
        bayGio={new Date("2026-09-10T02:00:00Z")}
        cong={congThaoTac(true, true, true)}
        tenBoPhan={new Map()}
        boPhan={[]}
        danhBa={DIRECTORY}
        dangGui={false}
        loiGhi={null}
        onTaskCreated={() => {}}
        dong={() => {}}
        phanLoai={() => {}}
        chuyenXuLy={() => {}}
        tienTrangThai={() => {}}
        dongPhieuLai={() => {}}
        khongTiepNhan={() => {}}
        chuyenCapTren={() => {}}
      />,
    );
    expect(html).not.toContain(BUTTON);
  });
});

function view(change: Partial<Parameters<typeof PetitionTaskView>[0]> = {}): string {
  return renderToStaticMarkup(
    <PetitionTaskView
      lookupCode="PA-2026-0021"
      open={false}
      catalogue={{ loai: [], mucUuTien: [], khoi: [], boPhan: [] }}
      danhBa={DIRECTORY}
      leaders={DIRECTORY}
      sending={false}
      error={null}
      createdCode={null}
      toggle={() => {}}
      cancel={() => {}}
      send={() => {}}
      {...change}
    />,
  );
}

describe("PetitionTaskView — the form, the refusal, the result", () => {
  it("closed: only the button", () => {
    const html = view();
    expect(html).toContain(`>${PETITION_TASK_BUTTON}</button>`);
    expect(html).not.toContain('id="giao-tieu-de"');
  });

  it("open: the shared task form, title pre-filled with the lookup code ONLY — no petition content", () => {
    const html = view({ open: true });
    expect(html).toContain('id="giao-tieu-de"');
    expect(html).toContain(`value="${petitionTaskTitle("PA-2026-0021")}"`);
    expect(petitionTaskTitle("PA-2026-0021")).toBe("Xử lý phản ánh PA-2026-0021");
    // Personal data never flows into the form (rule 3).
    const p = petition();
    expect(html).not.toContain(p.content);
    expect(html).not.toContain(p.reporter_name);
    expect(html).not.toContain(p.address);
  });

  it("any 409: the server's sentence is shown verbatim", () => {
    const sentence =
      "Phiếu chưa được phân loại và chuyển xử lý nên chưa tạo nhiệm vụ được. " +
      "Hãy phân loại và chuyển xử lý phiếu trước.";
    expect(view({ open: true, error: sentence })).toContain(sentence);
  });

  it("after 201: the new task code, with a link to the task register", () => {
    const html = view({ createdCode: "NV12" });
    expect(html).toContain("Đã tạo nhiệm vụ NV12.");
    expect(html).toContain(`href="${TASK_REGISTER_HREF}"`);
  });
});

describe("createTaskFromPetition — the wire", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  function stubFetch(status: number, body: unknown) {
    const fake = vi.fn(
      async (_path: string, _init?: RequestInit) =>
        new Response(JSON.stringify(body), {
          status,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fake);
    return fake;
  }

  // What `FormGiaoViec` hands over is `petitions_taoNhiemVuVao`, whose type HAS the source pair.
  const FROM_FORM: petitions_taoNhiemVuVao = {
    auto_code: true,
    type: "theo-van-ban",
    title: "Xử lý phản ánh PA-2026-0021",
    unit: "01JBOPHAN",
    due_at: "2026-10-07T17:00:00+07:00",
    source: "phan-anh",
    source_id: "PA-2026-9999",
  };

  it("POST to the petition's path, 201, the body WITHOUT `source` / `source_id`, with Idempotency-Key", async () => {
    const fake = stubFetch(201, { code: "NV12" });
    const kq = await createTaskFromPetition("PA/2026 0021", FROM_FORM, "k-open-1");

    expect(kq.ok).toBe(true);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA%2F2026%200021/tasks");
    const init = fake.mock.calls[0]?.[1];
    expect(init?.method).toBe("POST");
    const sent = JSON.parse(String(init?.body)) as Record<string, unknown>;
    expect(sent).not.toHaveProperty("source");
    expect(sent).not.toHaveProperty("source_id");
    expect(sent).toEqual({
      auto_code: true,
      type: "theo-van-ban",
      title: "Xử lý phản ánh PA-2026-0021",
      unit: "01JBOPHAN",
      due_at: "2026-10-07T17:00:00+07:00",
    });
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("k-open-1");
  });

  it("a retry reuses the SAME key — the function never mints one", async () => {
    const fake = stubFetch(503, { code: "unavailable", message: "Máy chủ bận.", trace_id: "x" });
    await createTaskFromPetition("PA-2026-0021", FROM_FORM, "k-fixed");
    await createTaskFromPetition("PA-2026-0021", FROM_FORM, "k-fixed");
    expect(new Headers(fake.mock.calls[1]?.[1]?.headers).get("Idempotency-Key")).toBe("k-fixed");
  });

  it.each([
    ["petition_state", "Phiếu đã đóng hoặc đã kết thúc nên không tạo nhiệm vụ mới từ phiếu này được."],
    ["petition_not_classified", "Phiếu chưa được phân loại và chuyển xử lý nên chưa tạo nhiệm vụ được."],
    ["restricted_field_no_task", "Phiếu thuộc lĩnh vực phản ánh về cán bộ không tạo nhiệm vụ."],
    ["some_future_code", "Một câu từ chối mới của máy chủ."],
  ])("409 `%s`: the server's message verbatim, whatever the code", async (code, message) => {
    stubFetch(409, { code, message, trace_id: "x" });
    expect(await createTaskFromPetition("PA-2026-0021", FROM_FORM, "k")).toEqual({
      ok: false,
      thongBao: message,
    });
  });
});

describe("timeline label of `tao-nhiem-vu`", () => {
  it("labelled “Tạo nhiệm vụ”; the note (the task code) is shown as the server wrote it", () => {
    expect(nhanThaoTacNhatKy("tao-nhiem-vu")).toBe("Tạo nhiệm vụ");
    const row: petitions_nhatKyPhieuRa = {
      id: "01JDONG9",
      at: "2026-09-30T03:05:00Z",
      actor_code: "CB-00123",
      action: "tao-nhiem-vu",
      status: "dang-xu-ly",
      unit: "",
      assignee: "",
      note: "NV12",
    };
    const html = renderToStaticMarkup(
      <DanhSachNhatKy
        dong={[row]}
        tenBoPhan={new Map()}
        danhBa={DIRECTORY.ok ? danhBaTheoMa(DIRECTORY.duLieu.items) : null}
      />,
    );
    expect(html).toContain("Tạo nhiệm vụ");
    expect(html).not.toContain("chưa có nhãn");
    expect(html).toContain("NV12");
  });
});
