import { describe, expect, it } from "vitest";

import { createFeedbackDraftStore } from "../commune-app/feedback-draft-store";
import {
  accountlessReportBody,
  PHOTO_UPLOAD_PARTS,
  photoUploadForm,
  ratingBody,
  thanGuiPhanAnh,
} from "../cong-dan/api/hop-dong-phan-anh";
import {
  BRIDGE_FIELDS_WITH_PHONE,
  bridgeBodyWithPhone,
  CLIENT_ERROR_FIELDS,
  clientErrorBody,
  COMMUNE_APP_LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS,
  communeAppSessionBody,
  LOCATION_FIELDS,
  locationBody,
  thanYeuCauCauViGov,
  TRUONG_GUI_DI_CAU_VIGOV,
} from "../features/dang-nhap/hop-dong";
import { KHAI_BAO_LOI_GOI } from "../features/tinh-nang/zalo-api";

import {
  COMMUNE_PRIVACY_EFFECTIVE_DATE,
  COMMUNE_PRIVACY_VERSION,
  communePrivacy,
  PETITION_RETENTION,
  PLATFORM_SERVER,
  SESSION_RETENTION,
} from "./commune-privacy";
import { communeTerms, type CommuneTermsValues } from "./commune-terms";
import { cauKhaiDichRaNgoai, COMMUNE_DESTINATIONS, DICH_MO_RA_NGOAI } from "./dich-ra-ngoai";
import { communeAppCalls, communePrivacyDocument, ketXuatVanBan, SCENE_PHOTO_UPLOAD_FIELDS } from "./ket-xuat-ho-so";

/**
 * THE COMMUNE'S PRIVACY POLICY — the fidelity locks that guarded the commune interface's data flows inside the old
 * shared policy, MOVED here with the text (owner 11/10/2026: two apps, two policies), not dropped:
 *
 *   body the code builds  ⇄  the declared field table  ⇄  each declaration VERBATIM in this policy
 *
 * for the own app's login, the QR-path bridge, the location exchange, the SDK failure report and the scene-photo
 * upload; and key ⇄ phrase for the petition body, the rating body and the on-phone draft. Reword a declaration
 * without rewording the policy — or add a key the policy does not name — and a case here is red.
 */

const VALUES: CommuneTermsValues = {
  displayName: "Xã Thử",
  province: "Tỉnh Thử",
  introductionUrl: "https://xa-thu.example.gov.vn/gioi-thieu",
};

const policy = () => communePrivacy(VALUES);
const section = (ma: string) => {
  const m = policy().sections.find((s) => s.ma === ma);
  expect(m, `the commune policy lost its section ${ma}`).toBeDefined();
  return m!.doan.join("\n");
};
const whole = () => [policy().opening, ...policy().sections.flatMap((s) => [s.tieu_de, ...s.doan])].join("\n");
const keysOf = (json: string) => Object.keys(JSON.parse(json) as Record<string, unknown>).sort();

describe("chính sách của ứng dụng xã — đứng tên Ủy ban nhân dân xã, không tên doanh nghiệp nào", () => {
  it("the controller is the commune's People's Committee, with the commune's own contact page (decision 6)", () => {
    const t = section("ben-chiu-trach-nhiem");
    expect(t).toContain("Ủy ban nhân dân xã Thử là bên chịu trách nhiệm về dữ liệu cá nhân");
    expect(t).toMatch(/tiếp nhận và xử lý phản ánh bạn gửi, và trả lời các yêu cầu xem, sửa hoặc xoá dữ liệu của bạn/);
    expect(t).toContain(VALUES.introductionUrl);
    expect(section("quyen-cua-ban")).toContain(VALUES.introductionUrl);
    expect(policy().appLine).toBe("Mini App Xã Thử, Tỉnh Thử");
    expect(policy().opening).toMatch(/^Chính sách này áp dụng cho ứng dụng Zalo Mini App của xã Thử, Tỉnh Thử/);
  });

  it("names no company and no brand of the platform — the platform's servers are named neutrally (decision 1)", () => {
    const t = whole();
    expect(t).not.toMatch(/ViHAT|Vihat|vihat|Tập đoàn|VihatSoftware/);
    expect(t).toContain(PLATFORM_SERVER);
    expect(PLATFORM_SERVER).toBe("máy chủ của đơn vị cung cấp nền tảng kỹ thuật của ứng dụng");
  });

  it("no CHỜ DUYỆT, no open decision, no placeholder", () => {
    expect(whole()).not.toMatch(/CHỜ DUYỆT|CẦN CHỦ DỰ ÁN QUYẾT|GIÁ TRỊ TẠM|CHƯA CHỐT|[<>]/);
  });

  it("retention per decision 6, verbatim — petition data by archival law, session data until deletion", () => {
    expect(PETITION_RETENTION).toBe("theo thời hạn lưu trữ hồ sơ mà pháp luật về lưu trữ quy định cho loại hồ sơ ấy");
    expect(SESSION_RETENTION).toBe("cho tới khi bạn yêu cầu xoá");
    const t = section("thoi-gian-luu");
    expect(t).toContain(`Phản ánh và những gì đi kèm — nội dung, họ tên, số điện thoại, toạ độ và ảnh hiện trường — được lưu ${PETITION_RETENTION}.`);
    expect(t).toContain(`Số điện thoại đã xác thực, mã tài khoản Zalo, địa chỉ IP và nhật ký phiên làm việc được lưu ${SESSION_RETENTION}.`);
    expect(section("phien-lam-viec")).toContain(`Những dữ liệu này được lưu ${SESSION_RETENTION}.`);
    expect(section("anh-hien-truong")).toContain(PETITION_RETENTION);
    // The old provisional "24 tháng sau khi đóng" must not come back as if decided.
    expect(section("anh-hien-truong")).not.toMatch(/24 tháng/);
  });

  it("the scene-photo store's operator per decision 6", () => {
    expect(section("anh-hien-truong")).toContain(
      "Kho ấy do đơn vị cung cấp nền tảng kỹ thuật của ứng dụng vận hành, theo thoả thuận với Ủy ban nhân dân xã.",
    );
  });

  it("fails closed on a missing or placeholder value — a legal text in a commune's name never prints a guess", () => {
    expect(() => communePrivacy({ ...VALUES, displayName: "" })).toThrow();
    expect(() => communePrivacy({ ...VALUES, displayName: "Thử" })).toThrow();
    expect(() => communePrivacy({ ...VALUES, introductionUrl: "<trang-gioi-thieu>" })).toThrow();
    expect(() => communePrivacy({ ...VALUES, introductionUrl: "http://xa-thu.example.gov.vn" })).toThrow();
  });

  it("version 1.0, dated the day the content became true; the dossier prints it with its source", () => {
    expect(COMMUNE_PRIVACY_VERSION).toBe("1.0");
    expect(COMMUNE_PRIVACY_EFFECTIVE_DATE).toBe("11/10/2026");
    const printed = ketXuatVanBan(communePrivacyDocument(VALUES, "nguon-thu"));
    expect(printed).toContain("CHÍNH SÁCH QUYỀN RIÊNG TƯ");
    expect(printed).toContain("Phiên bản 1.0 — Hiệu lực từ ngày 11/10/2026");
    expect(printed).toContain("nguon-thu");
    for (const s of policy().sections) expect(printed).toContain(s.tieu_de.toUpperCase());
  });

  it("the commune's terms point at THIS policy, in the same authority's name", () => {
    const privacy = communeTerms(VALUES).sections.find((s) => s.ma === "quyen-rieng-tu")!.doan.join("\n");
    expect(privacy).toContain("Chính sách quyền riêng tư của ứng dụng, do Ủy ban nhân dân xã Thử công bố cùng Điều khoản này");
  });

  it("every section has its own key and no section is empty", () => {
    const keys = policy().sections.map((s) => s.ma);
    expect(new Set(keys).size).toBe(keys.length);
    for (const s of policy().sections) expect(s.doan.length, s.ma).toBeGreaterThan(0);
  });
});

describe("phiên làm việc với xã — thân gửi đi, bảng khai và văn bản khoá nhau", () => {
  it("own app: the login body sends exactly the declared keys (with and without the phone), each declared VERBATIM", () => {
    // Was a pinned GAP in the shared policy ("câu chính sách cho `appId` CÒN NỢ"); the commune policy now declares it,
    // so the case became the two-way lock it promised.
    const withPhone = keysOf(communeAppSessionBody({ ma_truy_cap: "m", ma_so_dien_thoai: "p", app_id: "1" }));
    expect(withPhone).toEqual(COMMUNE_APP_SESSION_FIELDS.map((t) => t.khoa).sort());
    expect(withPhone).not.toContain("communeHostHint");
    const phoneless = keysOf(communeAppSessionBody({ ma_truy_cap: "m", ma_so_dien_thoai: "", app_id: "1" }));
    for (const k of phoneless) expect(withPhone).toContain(k);
    const t = section("phien-lam-viec");
    for (const f of COMMUNE_APP_SESSION_FIELDS) expect(t, `not declared: ${f.khoa}`).toContain(f.trong_chinh_sach);
    expect(t).toMatch(/TRONG ỨNG DỤNG CỦA XÃ, khi bạn đồng ý, ứng dụng gửi thẳng tới hệ thống của xã/);
  });

  it("QR path: the bridge body sends exactly the declared keys, the phone code included, each declared VERBATIM", () => {
    const withPhone = keysOf(
      bridgeBodyWithPhone({ ma_truy_cap: "m", ten_mien_xa: "xa-vi-du.vigov.example", ma_so_dien_thoai: "p" }),
    );
    expect(withPhone).toEqual(BRIDGE_FIELDS_WITH_PHONE.map((t) => t.khoa).sort());
    expect(withPhone).toContain("phoneToken");
    const confirmOnly = keysOf(thanYeuCauCauViGov({ ma_truy_cap: "m", ten_mien_xa: "xa-vi-du.vigov.example" }));
    expect(confirmOnly).toEqual(TRUONG_GUI_DI_CAU_VIGOV.map((t) => t.khoa).sort());
    const t = section("phien-lam-viec");
    for (const f of BRIDGE_FIELDS_WITH_PHONE) expect(t, `not declared: ${f.khoa}`).toContain(f.trong_chinh_sach);
    expect(t).toMatch(new RegExp(`phiên làm việc được mở qua ${PLATFORM_SERVER}`));
    expect(t).toMatch(/và không gì khác/);
  });

  it("says what the platform server forwards, that it keeps no number, and what the commune system keeps", () => {
    const t = section("phien-lam-viec");
    for (const thing of [
      "mã tài khoản Zalo của bạn đối với riêng ứng dụng này",
      "số điện thoại Zalo của bạn",
      "địa chỉ IP và thông tin trình duyệt của máy bạn",
      "mã số của ứng dụng",
      "tên miền của xã",
    ]) {
      expect(t, `does not say the platform server forwards: ${thing}`).toContain(thing);
    }
    expect(t).toMatch(/KHÔNG lưu số điện thoại của bạn và không tạo tài khoản nào ở phía mình/);
    expect(t).toMatch(/HỆ THỐNG CỦA XÃ LƯU/);
  });
});

describe("phản ánh — thân gửi đi và câu khai khoá nhau", () => {
  /** Every key the petition body can carry ⇄ the phrase naming it in section `phan-anh`. */
  const PHRASE: Readonly<Record<string, string>> = {
    content: "nội dung phản ánh bạn viết",
    address: "địa chỉ nơi xảy ra sự việc bạn điền",
    reporter_name: "họ tên và số điện thoại bạn điền trong biểu mẫu",
    reporter_phone: "họ tên và số điện thoại bạn điền trong biểu mẫu",
    anonymous: "việc bạn có chọn gửi ẩn danh hay không",
    field: "lĩnh vực bạn chọn, nếu bạn chọn",
    residential_unit_id: "thôn, tổ dân phố bạn chọn, nếu bạn chọn",
    lat: "toạ độ nơi xảy ra sự việc, nếu ứng dụng đã lấy được vị trí với sự đồng ý của bạn",
    lng: "toạ độ nơi xảy ra sự việc, nếu ứng dụng đã lấy được vị trí với sự đồng ý của bạn",
    location_citizen_adjusted: "khi có toạ độ, việc bạn có tự chỉnh vị trí trên bản đồ hay không",
  };

  it("the WIDEST body sends exactly the keys the policy names — no more, no fewer", () => {
    const keys = keysOf(
      thanGuiPhanAnh({
        noi_dung: "x",
        dia_chi: "y",
        ho_ten: "z",
        dien_thoai: "0900000000",
        an_danh: false,
        scene_location: { lat: 16, lng: 108 },
        location_citizen_adjusted: true,
        field: "f",
        residential_unit_id: "u",
      }),
    );
    expect(keys).toEqual(Object.keys(PHRASE).sort());
    const t = section("phan-anh");
    for (const k of keys) expect(t, `the petition section does not declare "${k}"`).toContain(PHRASE[k]!);
    expect(t).toMatch(/đúng những thứ sau, và không gì khác/);
  });

  it("the accountless body is the form plus the domain — no coordinates, no session (ADR 0083)", () => {
    const keys = keysOf(
      accountlessReportBody(
        { noi_dung: "x", dia_chi: "y", ho_ten: "z", dien_thoai: "0900000000", an_danh: false, field: "f" },
        "xa-thu.example.gov.vn",
      ),
    );
    expect(keys).not.toContain("lat");
    expect(keys).toContain("host");
    const t = section("phan-anh");
    expect(t).toMatch(/KHÔNG CẦN TÀI KHOẢN/);
    expect(t).toMatch(/trừ ảnh và toạ độ/);
    expect(t).toMatch(/KHÔNG kèm phiếu phiên, mã phiên Zalo hay mã người dùng nào/);
  });

  it("anonymous: name and number not sent, and the text says honestly the system still knows whose it is", () => {
    const body = JSON.parse(
      thanGuiPhanAnh({ noi_dung: "x", dia_chi: "y", ho_ten: "z", dien_thoai: "0900000000", an_danh: true }),
    ) as Record<string, unknown>;
    expect(body["reporter_name"]).toBe("");
    expect(body["reporter_phone"]).toBe("");
    const t = section("phan-anh");
    expect(t).toMatch(/KHÔNG gửi họ tên và số điện thoại bạn đã điền/);
    expect(t, "hides that the citizen id is still stored (ADR 0008)").toMatch(/ẩn danh không có nghĩa là hệ thống không biết/);
    expect(t).toMatch(/che bớt trên màn hình của cán bộ, trừ cán bộ được giao quyền xem đầy đủ/);
  });

  it("location exchange: the body sends exactly the declared keys (+ appId in the own app), each declared VERBATIM", () => {
    expect(keysOf(locationBody({ access_token: "m", location_token: "v" }))).toEqual(LOCATION_FIELDS.map((t) => t.khoa).sort());
    expect(keysOf(locationBody({ access_token: "m", location_token: "v" }, "1"))).toEqual(
      COMMUNE_APP_LOCATION_FIELDS.map((t) => t.khoa).sort(),
    );
    const t = section("phan-anh");
    for (const f of COMMUNE_APP_LOCATION_FIELDS) expect(t, `not declared: ${f.khoa}`).toContain(f.trong_chinh_sach);
    expect(t).toMatch(new RegExp(`ứng dụng gửi tới ${PLATFORM_SERVER} đúng những thứ sau, và không gì khác`));
    // When it is asked — the commune row of `getLocation` (owner 11/10/2026: also on choosing a field).
    expect(t).toMatch(/khi bạn chọn lĩnh vực phản ánh, khi bạn tiếp tục một phản ánh đang soạn chưa ghi địa chỉ, hoặc khi bạn bấm 'Lấy vị trí hiện tại'/);
  });

  it("rating: the body sends stars and, when written, the comment — and the text names both", () => {
    expect(keysOf(ratingBody(4, "tốt"))).toEqual(["comment", "stars"]);
    expect(keysOf(ratingBody(4, " "))).toEqual(["stars"]);
    const t = section("phan-anh");
    expect(t).toMatch(/số sao bạn chọn \(từ 1 đến 5\)/);
    expect(t).toMatch(/lời nhận xét \(tối đa 1000 ký tự\)/);
  });
});

describe("bản nháp và ảnh hiện trường", () => {
  it("draft: names exactly the six keys the real store writes; no expiry, no photo, no coordinates", () => {
    const written = new Map<string, string>();
    const store = createFeedbackDraftStore(
      () => ({
        getItem: (k) => written.get(k) ?? null,
        setItem: (k, v) => void written.set(k, v),
        removeItem: (k) => void written.delete(k),
      }),
      "k",
    );
    store.save({ linh_vuc: "f", noi_dung: "x", dia_chi: "y", ho_ten: "z", dien_thoai: "0900000000", an_danh: true });
    const PHRASE: Readonly<Record<string, string>> = {
      linh_vuc: "lĩnh vực bạn chọn",
      noi_dung: "nội dung phản ánh",
      dia_chi: "địa chỉ",
      ho_ten: "họ tên",
      dien_thoai: "số điện thoại bạn đã điền",
      an_danh: "việc bạn có chọn gửi ẩn danh hay không",
    };
    const keys = keysOf(written.get("k")!);
    expect(keys).toEqual(Object.keys(PHRASE).sort());
    const t = section("ban-nhap");
    expect(t).toMatch(/Bản nháp gồm đúng sáu thứ/);
    for (const k of keys) expect(t, `the draft section does not name "${k}"`).toContain(PHRASE[k]!);
    expect(t).toMatch(/Ảnh và toạ độ KHÔNG nằm trong bản nháp/);
    expect(t).toMatch(/KHÔNG tự hết hạn/);
    expect(t).toMatch(/'Bỏ nháp'/);
    expect(t).toMatch(/'Huỷ bỏ'/);
    expect(t).toMatch(/không bao giờ được gửi đi đâu/);
  });

  it("scene photos: the multipart parts are exactly the declared ones, in order, each declared VERBATIM", () => {
    const parts = [...photoUploadForm(new Blob([new Uint8Array([0xff, 0xd8, 0xff])]), "image/jpeg").keys()];
    expect(parts).toEqual([...PHOTO_UPLOAD_PARTS]);
    expect(SCENE_PHOTO_UPLOAD_FIELDS.map((t) => t.khoa).filter((k) => k !== "maTraCuu")).toEqual(parts);
    const t = section("anh-hien-truong");
    for (const f of SCENE_PHOTO_UPLOAD_FIELDS) expect(t, `not declared: ${f.khoa}`).toContain(f.trong_chinh_sach);
  });

  it("scene photos: optional · at most 5 · only on a tap · after the petition is recorded · cleaned · private · 15 min", () => {
    const t = section("anh-hien-truong");
    expect(t).toMatch(/KHÔNG BẮT BUỘC/);
    expect(t).toMatch(/tối đa 5 ảnh hiện trường/);
    expect(t).toMatch(/JPEG, PNG hoặc WebP, không nhận video/);
    expect(t).toMatch(/'Chụp ảnh'[^\n]*'Chọn ảnh có sẵn'/);
    expect(t).toMatch(/chỉ khi chính bạn bấm/);
    expect(t).toMatch(/SAU KHI phản ánh của bạn đã được ghi nhận/);
    expect(t).toMatch(/Zalo không tải ảnh lên hộ/);
    expect(t).toMatch(/Tên tệp ảnh trên máy bạn không được gửi đi/);
    expect(t).toMatch(/quét mã độc/);
    expect(t).toMatch(/BỎ TOÀN BỘ thông tin kèm theo ảnh \(EXIF\), kể cả vị trí chụp/);
    expect(t).toMatch(/không bao giờ được công khai/);
    expect(t).toMatch(/tối đa 15 phút/);
    expect(t).toMatch(/cán bộ xem ảnh đều được ghi lại/);
    expect(t).toMatch(/không nằm trong bản nháp phản ánh/);
  });
});

describe("quyền, nơi nhận và báo lỗi — khoá với bảng khai lời gọi và danh sách đích", () => {
  it("every call the commune interface makes is named in the permission section", () => {
    const calls = communeAppCalls(KHAI_BAO_LOI_GOI).map((r) => r.api);
    expect(calls.length, "no commune call — this case would pass on nothing").toBeGreaterThan(5);
    const t = section("cac-quyen");
    for (const api of calls) expect(t, `the commune policy does not name ${api}`).toContain(`${api}`);
  });

  it("the outside destinations are the commune interface's, counted from `COMMUNE_DESTINATIONS`", () => {
    const t = section("noi-nhan");
    expect(t).toContain(cauKhaiDichRaNgoai(COMMUNE_DESTINATIONS));
    for (const d of DICH_MO_RA_NGOAI.filter((x) => x.app !== "commune")) {
      expect(t, `an intro destination is in the commune text: ${d.ma}`).not.toContain(d.trong_chinh_sach);
    }
    expect(t).toContain(
      "Liên kết trong bài viết của xã mở ra ngoài ứng dụng, trong trình duyệt của Zalo, sau khi bạn xác nhận; ứng dụng không theo dõi việc bạn bấm liên kết nào hay xem gì ở trang ấy.",
    );
  });

  it("the SDK failure report: the full body is exactly the declared keys, each declared VERBATIM", () => {
    const keys = keysOf(clientErrorBody({ capability: "phone", code: -1, message: "m", app_id: "1", host: "xa.example" }));
    expect(keys).toEqual(CLIENT_ERROR_FIELDS.map((f) => f.khoa).sort());
    const t = section("noi-nhan");
    for (const f of CLIENT_ERROR_FIELDS) expect(t, `not declared: ${f.khoa}`).toContain(f.trong_chinh_sach);
  });

  it("the public reads at open and the pictures are said, with the IP the image host sees", () => {
    const t = section("khi-mo-ung-dung");
    expect(t).toMatch(/NGAY KHI ỨNG DỤNG MỞ RA, trước khi bạn bấm gì/);
    expect(t).toMatch(/ngay khi chúng hiện lên màn hình, không cần bạn bấm/);
    expect(t).toMatch(/địa chỉ IP và thông tin trình duyệt/);
    expect(t).toMatch(/ảnh đại diện Zalo của bạn hiện ở màn Cá nhân; ảnh không được gửi đi đâu/);
  });
});
