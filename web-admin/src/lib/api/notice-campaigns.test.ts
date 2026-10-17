import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createNoticeCampaign,
  getNoticeCampaign,
  listNoticeCampaigns,
  listNoticeRecipients,
  listNoticeTemplates,
  previewNoticeCampaign,
  type NoticeDraft,
} from "./notice-campaigns";

/**
 * The five `notice-*` routes (ADR 0099 #4–#9). The failures that matter are silent: a send without
 * `Idempotency-Key` (comms refuses — idem.Required), BOTH sources sent (comms refuses), a stray field going
 * up from a spread, an empty `cursor=` (comms answers 400), and a refusal whose `code` the screen needs
 * (`no_recipients`) but never receives.
 */

function stubFetch(reply: Response) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) => reply);
  vi.stubGlobal("fetch", fake);
  return fake;
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function sentBody(fake: ReturnType<typeof stubFetch>): Record<string, unknown> | undefined {
  const raw = fake.mock.calls[0]?.[1]?.body;
  return typeof raw === "string" ? (JSON.parse(raw) as Record<string, unknown>) : undefined;
}

function sentHeaders(fake: ReturnType<typeof stubFetch>): Record<string, string> {
  return (fake.mock.calls[0]?.[1]?.headers ?? {}) as Record<string, string>;
}

const ZNS: NoticeDraft = {
  channel: "zalo-zns",
  sourceId: "01JOA1",
  templateRef: "zt-1",
  audience: { kind: "nguoi-dan-theo-nhom", ids: ["01JG1"], tags: [] },
  params: { ngay_tiem: "20/10" },
};
const SMS: NoticeDraft = {
  channel: "sms",
  sourceId: "01JSMS",
  templateRef: "st-1",
  audience: { kind: "nguoi-dan-theo-nhan", ids: [], tags: ["hội phụ nữ"] },
  params: { "1": "20/10" },
};

const CAMPAIGN = {
  id: "01JC1",
  title: "Lịch tiêm",
  channel: "zalo-zns",
  oa_id: "01JOA1",
  sms_account_id: null,
  source_name: "UBND xã Tân Phú",
  template_ref: "zt-1",
  template_name: "Thông báo lịch tiêm",
  audience_kind: "nguoi-dan-theo-nhom",
  audience_ids: ["01JG1"],
  recipient_count: 11,
  skipped_count: 1,
  sent_count: 0,
  failed_count: 0,
  unit_price: "200.00",
  estimated_cost: "2200.00",
  status: "dang-gui",
  created_at: "2026-10-17T01:00:00Z",
  created_by: "CB-00123",
  created_by_name: "Nguyễn Văn Hùng",
  finished_at: null,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("GET routes", () => {
  it("templates: GET /api/v1/notice-templates, both lists as they came", async () => {
    const fake = stubFetch(json({ zns: [], sms: [] }, 200));
    expect(await listNoticeTemplates()).toEqual({ ok: true, duLieu: { zns: [], sms: [] } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/notice-templates");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
  });

  it("history: no cursor on the first page (an empty `cursor=` is a 400), the cursor encoded on the next", async () => {
    const fake = stubFetch(json({ items: [CAMPAIGN], next_cursor: "" }, 200));
    expect(await listNoticeCampaigns()).toEqual({ ok: true, duLieu: { items: [CAMPAIGN], next_cursor: "" } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/notice-campaigns");
    await listNoticeCampaigns("a b&c");
    expect(fake.mock.calls[1]?.[0]).toBe("/api/v1/notice-campaigns?cursor=a+b%26c");
  });

  it("one campaign: the id is encoded into the path", async () => {
    const fake = stubFetch(json(CAMPAIGN, 200));
    await getNoticeCampaign("a/b");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/notice-campaigns/a%2Fb");
  });
});

describe("preview", () => {
  it("ZNS: oa_id only, the audience as given, params keyed by field — nothing else", async () => {
    const fake = stubFetch(json({ content: null, recipient_count: 11, skipped_count: 0, unit_price: "200.00", estimated_cost: "2200.00", missing_params: [] }, 200));
    const r = await previewNoticeCampaign(ZNS);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/notice-campaign-previews");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentBody(fake)).toEqual({
      channel: "zalo-zns",
      oa_id: "01JOA1",
      template_ref: "zt-1",
      audience: { kind: "nguoi-dan-theo-nhom", ids: ["01JG1"], tags: [] },
      params: { ngay_tiem: "20/10" },
    });
    expect(r.ok).toBe(true);
  });

  it("SMS: sms_account_id only, positional params", async () => {
    const fake = stubFetch(json({ content: "x", recipient_count: 9, skipped_count: 0, unit_price: null, estimated_cost: null, missing_params: [] }, 200));
    await previewNoticeCampaign(SMS);
    const body = sentBody(fake)!;
    expect(body.sms_account_id).toBe("01JSMS");
    expect("oa_id" in body).toBe(false);
    expect(body.params).toEqual({ "1": "20/10" });
  });

  it("a refusal keeps the server's sentence and code", async () => {
    stubFetch(json({ code: "source_unavailable", message: "OA này cần được uỷ quyền lại trước khi gửi.", trace_id: "t" }, 409));
    expect(await previewNoticeCampaign(ZNS)).toEqual({
      ok: false,
      thongBao: "OA này cần được uỷ quyền lại trước khi gửi.",
      code: "source_unavailable",
    });
  });
});

describe("imported sources (ADR 0099 §Sửa đổi S1–S4)", () => {
  const FILE_SMS: NoticeDraft = {
    channel: "sms",
    sourceId: "01JSMS",
    templateRef: "",
    audience: { kind: "tep-nhap", ids: [], tags: [] },
    params: {},
    recipients: [{ phone: "0900000000", values: { variable_1: "Bà A" } }, { phone: "0900000001" }],
    smsContentMode: "noi-dung-chung",
    content: "Mời {{variable_1}}",
    mapping: { phone_column: "Số điện thoại", variables: { variable_1: "Họ tên" } },
  };
  const PREVIEW = { content: "Mời Bà A", recipient_count: 2, skipped_count: 0, invalid_count: 0, duplicate_count: 0, unit_price: null, estimated_cost: null, missing_params: [] };

  it("SMS from a file: rows, mode, shared text and mapping go up; NO template_ref; a row without values sends none", async () => {
    const fake = stubFetch(json(PREVIEW, 200));
    expect(await previewNoticeCampaign(FILE_SMS)).toEqual({ ok: true, duLieu: PREVIEW });
    expect(sentBody(fake)).toEqual({
      channel: "sms",
      sms_account_id: "01JSMS",
      audience: { kind: "tep-nhap", ids: [], tags: [] },
      params: {},
      recipients: [{ phone: "0900000000", values: { variable_1: "Bà A" } }, { phone: "0900000001" }],
      sms_content_mode: "noi-dung-chung",
      content: "Mời {{variable_1}}",
      mapping: { phone_column: "Số điện thoại", variables: { variable_1: "Họ tên" } },
    });
  });

  it("the send carries the same body plus the title; a typed list sends no mapping key at all", async () => {
    const fake = stubFetch(json({ ...CAMPAIGN, invalid_count: 0, recipient_mapping: null, content: null, sms_content_mode: null }, 201));
    const typed: NoticeDraft = { ...ZNS, audience: { kind: "danh-sach-so", ids: [], tags: [] }, recipients: [{ phone: "0900000000" }] };
    await createNoticeCampaign(typed, "t", "k");
    const body = sentBody(fake)!;
    expect(body).toMatchObject({ title: "t", template_ref: "zt-1", oa_id: "01JOA1", recipients: [{ phone: "0900000000" }] });
    expect("mapping" in body).toBe(false);
    expect("sms_content_mode" in body).toBe(false);
    expect("content" in body).toBe(false);
  });

  it("recipients: GET /api/v1/notice-campaigns/{id}/recipients, the id encoded, no empty cursor, the next cursor encoded", async () => {
    const page = { items: [{ id: "r1", phone_masked: "09****0000", status: "that-bai", error_message: "Sai mẫu (146)", sent_at: null, created_at: "2026-10-17T01:00:00Z" }], next_cursor: "" };
    const fake = stubFetch(json(page, 200));
    expect(await listNoticeRecipients("a/b")).toEqual({ ok: true, duLieu: page });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/notice-campaigns/a%2Fb/recipients");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    await listNoticeRecipients("01JC1", "x y");
    expect(fake.mock.calls[1]?.[0]).toBe("/api/v1/notice-campaigns/01JC1/recipients?cursor=x+y");
  });
});

describe("send", () => {
  it("POST /api/v1/notice-campaigns with the Idempotency-Key, the title and the preview's body; 201 is the campaign", async () => {
    const fake = stubFetch(json(CAMPAIGN, 201));
    const r = await createNoticeCampaign(ZNS, "Lịch tiêm", "key-1");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentHeaders(fake)["Idempotency-Key"]).toBe("key-1");
    expect(sentBody(fake)).toEqual({
      title: "Lịch tiêm",
      channel: "zalo-zns",
      oa_id: "01JOA1",
      template_ref: "zt-1",
      audience: { kind: "nguoi-dan-theo-nhom", ids: ["01JG1"], tags: [] },
      params: { ngay_tiem: "20/10" },
    });
    expect(r).toEqual({ ok: true, duLieu: CAMPAIGN });
  });

  it("a 200 is not a created campaign; 409 no_recipients carries its code", async () => {
    stubFetch(json(CAMPAIGN, 200));
    expect((await createNoticeCampaign(ZNS, "t", "k")).ok).toBe(false);
    stubFetch(json({ code: "no_recipients", message: "Đối tượng đã chọn không có ai nhận được tin.", trace_id: "" }, 409));
    const r = await createNoticeCampaign(ZNS, "t", "k");
    expect(r.ok === false && r.code).toBe("no_recipients");
  });
});
