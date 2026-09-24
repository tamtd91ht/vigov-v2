import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BieuMauGhiCanBo, type DangMoGhi, type MucChon } from "./bieu-mau-ghi-can-bo";
import { BAN_TRONG, banTuCanBo, type BanNhapCanBo } from "./nhan-ghi-danh-ba";

/**
 * BỐN QUY TẮC NGHIỆP VỤ ĐI QUA ĐÚNG TỆP NÀY, và cả bốn đều là loại "phá được bằng một dòng, xanh
 * cả bộ test" nếu không ai kết xuất biểu mẫu ra chuỗi mà đọc lại:
 *
 *   #15  form THÊM không có ô Mã — mã do hệ thống cấp, và mã đã cấp thì không bao giờ cấp lại
 *        (luật 7, bất biến 3). Ô Mã trên form là đường để một mã do người gõ chỉ vào hồ sơ lưu
 *        trữ của người khác.
 *   #16  form SỬA có HAI ô điện thoại, hai nhãn nói rõ loại. Máy bàn cơ quan là thông tin công
 *        vụ; di động cá nhân là dữ liệu cá nhân theo Nghị định 13.
 *   #10  không có nút Xoá ở bất kỳ biểu mẫu nào.
 *   #13  câu từ chối "người quản trị cuối cùng" phải RA TỚI TRANG, nguyên văn, đọc được — không
 *        phải một mã lỗi, không phải một số hiệu HTTP.
 *   #14  hai ràng buộc khác nhau thì HAI câu khác nhau, và cả hai đều tới được người đọc.
 *
 * Ba câu của #13 và #14 do MÁY CHỦ viết (`service-identity/internal/http/can_bo_ghi.go`); các ca
 * dưới đây dán đúng chuỗi ấy vào `loiMayChu` và đòi nó xuất hiện. Đó là phép kiểm đúng chỗ: nếu
 * một ngày ai đó bọc lỗi lại bằng một câu tự viết, hoặc rẽ nhánh theo `code`, ca này đỏ.
 */

const MAY_BAN = "02350000000";
const DI_DONG = "0900000000";

const CAN_BO: identity_canBoTomTat = {
  id: "01J000000000000000000001",
  code: "CB001",
  full_name: "Huỳnh Văn A",
  email: "demo@thangbinh.test",
  position: "Chuyên viên",
  department_id: "01J0000000000000000BOPHAN",
  role_id: "01J00000000000000000VAITRO",
  phone: MAY_BAN,
  mobile: DI_DONG,
  has_account: true,
  active: true,
  last_login_at: null,
  created_at: "2026-09-22T08:00:00Z",
  has_zalo: false,
  published: false,
  display_order: null,
  consent_recorded_at: null,
};

const BO_PHAN: readonly MucChon[] = [{ id: "01J0000000000000000BOPHAN", name: "Văn phòng" }];
const VAI_TRO: readonly MucChon[] = [{ id: "01J00000000000000000VAITRO", name: "Chuyên viên" }];

function ve(
  dangMo: DangMoGhi,
  {
    ban = BAN_TRONG,
    vaiTroID = "",
    loiMayChu = "",
    boPhan = BO_PHAN,
  }: { ban?: BanNhapCanBo; vaiTroID?: string; loiMayChu?: string; boPhan?: readonly MucChon[] } = {},
): string {
  return renderToStaticMarkup(
    <BieuMauGhiCanBo
      dangMo={dangMo}
      ban={ban}
      datBan={() => {}}
      vaiTroID={vaiTroID}
      datVaiTroID={() => {}}
      boPhan={boPhan}
      vaiTro={VAI_TRO}
      loiMayChu={loiMayChu}
      dangGui={false}
      onGui={() => {}}
      onHuy={() => {}}
    />,
  );
}

/** Tên của mọi ô nhập chữ trong markup — thứ quyết định trường nào đi lên được máy chủ. */
function tenONhap(html: string): string[] {
  return [...html.matchAll(/<input[^>]*\sname="([^"]+)"/g)].map((m) => m[1] ?? "");
}

const FORM_THEM: DangMoGhi = { kieu: "them", khoaChongTrung: "k-co-dinh" };
const FORM_SUA: DangMoGhi = { kieu: "sua", canBo: CAN_BO };
const FORM_VAI_TRO: DangMoGhi = { kieu: "vaiTro", canBo: CAN_BO };
const FORM_KHOA: DangMoGhi = { kieu: "khoa", canBo: CAN_BO, khoa: true };
const BON_BIEU_MAU: readonly DangMoGhi[] = [FORM_THEM, FORM_SUA, FORM_VAI_TRO, FORM_KHOA];

describe("ô 'Có Zalo' — chỉ ở biểu mẫu SỬA, dùng chung cho cả hai màn", () => {
  it("biểu mẫu sửa có ô tick, nạp đúng giá trị đang có", () => {
    const chua = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) });
    const oChua = /<input[^>]*id="o-co-zalo-can-bo"[^>]*>/.exec(chua)?.[0] ?? "";
    expect(oChua).toContain('type="checkbox"');
    expect(oChua).not.toContain("checked");

    const co = ve(FORM_SUA, { ban: banTuCanBo({ ...CAN_BO, has_zalo: true }) });
    expect(/<input[^>]*id="o-co-zalo-can-bo"[^>]*>/.exec(co)?.[0]).toContain('checked=""');
  });

  it("nói rõ ô ấy KHÔNG đưa số lên Mini App", () => {
    expect(ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) })).toContain("không đưa số lên Zalo Mini App");
  });

  it("biểu mẫu thêm, đổi vai trò, khoá: KHÔNG có ô ấy — hợp đồng tạo mới không có trường", () => {
    for (const dm of [FORM_THEM, FORM_VAI_TRO, FORM_KHOA]) {
      expect(ve(dm, { ban: banTuCanBo(CAN_BO) })).not.toContain("o-co-zalo-can-bo");
    }
  });
});

describe("#15 — form thêm KHÔNG có ô Mã", () => {
  it("năm ô nhập, và không ô nào là Mã", () => {
    const ten = tenONhap(ve(FORM_THEM));

    // SO CẢ TẬP, KHÔNG CHỈ "không chứa chữ mã". Một ô Mã tên `o-so-hieu`, `o-code` hay `o-ma-nv`
    // đều lọt qua một phép kiểm tìm chuỗi; so đúng tập thì MỌI ô thứ sáu đều làm ca này đỏ.
    expect(ten).toEqual([
      "o-ho-ten-can-bo",
      "o-chuc-danh-can-bo",
      "o-email-can-bo",
      "o-may-ban-can-bo",
      "o-di-dong-can-bo",
    ]);
  });

  it("không có NHÃN Ô nào tên Mã trên form thêm", () => {
    const html = ve(FORM_THEM);

    // Kiểm `<label>` chứ không kiểm chuỗi "Mã cán bộ": chữ ấy CÓ mặt trên form, trong câu giải
    // thích vì sao không có ô nhập mã. Một phép kiểm cấm cả chữ ấy sẽ buộc người sau xoá đúng câu
    // giải thích để làm test xanh — tức làm màn hình tệ đi để chiều một bài test.
    expect(html).not.toMatch(/<label[^>]*>\s*Mã/);
    expect(html).not.toMatch(/<(input|select)[^>]*\sid="[^"]*-ma-/);
  });

  it("nói ra vì sao không có ô Mã, và rằng thêm danh bạ chưa phải cấp tài khoản", () => {
    // Không nói thì người quen nhập hồ sơ nhân sự đi tìm ô Mã, và xã tưởng đã tạo xong người dùng
    // rồi ngồi chờ người ấy đăng nhập (#15 và #9 là hai chuyện khác nhau, cùng gây hiểu nhầm ở đây).
    const html = ve(FORM_THEM);

    expect(html).toContain("Mã cán bộ do hệ thống cấp sau khi lưu");
    expect(html).toContain("chưa phải là cấp tài khoản đăng nhập");
  });

  it("form SỬA hiện mã, nhưng là CHỮ chứ không phải ô nhập", () => {
    const html = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) });

    expect(html).toContain("Mã cán bộ: CB001");
    // Vế chịu lực: mã có mặt để đối chiếu hồ sơ giấy, và KHÔNG có đường nào cho nó vào thân yêu cầu.
    expect(tenONhap(html)).not.toContain("o-ma-can-bo");
    expect(html).not.toMatch(/<input[^>]*value="CB001"/);
  });
});

describe("#16 — form sửa có HAI ô điện thoại, nhãn nói rõ loại", () => {
  it("hai nhãn khác nhau, không một nhãn trung tính", () => {
    const html = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) });

    expect(html).toContain("Máy bàn cơ quan");
    expect(html).toContain("Di động cá nhân");

    // Nhãn trung tính "Điện thoại" là đúng hình dạng ô gộp — người đang gõ không biết mình đặt
    // một số di động cá nhân vào cột công vụ hay ngược lại.
    expect(html).not.toMatch(/<label[^>]*>Điện thoại<\/label>/);
  });

  it("hai ô RIÊNG, mỗi ô giữ đúng số của nó", () => {
    const html = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) });

    expect(html).toContain(`name="o-may-ban-can-bo" value="${MAY_BAN}"`);
    expect(html).toContain(`name="o-di-dong-can-bo" value="${DI_DONG}"`);
  });

  it("nói ra địa vị pháp lý của từng số, ngay cạnh ô", () => {
    // Đây là chỗ duy nhất trên đường GHI mà người dùng được nhắc rằng hai ô này khác nhau về LUẬT
    // chứ không chỉ khác nhau về chỗ để gọi.
    const html = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) });

    expect(html).toContain("thông tin công vụ");
    expect(html).toContain("Nghị định 13/2023/NĐ-CP");
  });
});

describe("#10 — không biểu mẫu nào có nút Xoá", () => {
  it.each(BON_BIEU_MAU.map((m) => [m.kieu, m] as const))("biểu mẫu %s", (_ten, dangMo) => {
    const html = ve(dangMo, { ban: banTuCanBo(CAN_BO) });

    expect(html).not.toContain("🗑");
    expect(html).not.toMatch(/Xo[áa] kh[ỏo]i danh b[ạa]/i);
    expect(html).not.toMatch(/>\s*Xoá\s*</);
  });

  it("biểu mẫu khoá nói ra hậu quả, và nói rõ người đó VẪN CÒN trong danh bạ", () => {
    const html = ve(FORM_KHOA);

    expect(html).toContain("không đăng nhập được nữa");
    // Nửa câu hay bị hiểu sai nhất: người bấm đang nghĩ mình xoá một người khỏi hệ thống.
    expect(html).toContain("VẪN CÒN trong danh bạ");
    expect(html).toContain("Xác nhận khoá");
  });

  it("biểu mẫu mở khoá nói câu khác hẳn, và nút cũng khác", () => {
    const html = ve({ kieu: "khoa", canBo: CAN_BO, khoa: false });

    expect(html).toContain("đăng nhập lại được");
    expect(html).toContain("Xác nhận mở khoá");
    expect(html).not.toContain("không đăng nhập được nữa");
  });
});

describe("#13 và #14 — câu từ chối của máy chủ ra tới trang, nguyên văn", () => {
  /** Đúng ba chuỗi `service-identity/internal/http/can_bo_ghi.go` phát ra. */
  const CAU_QUAN_TRI_CUOI =
    "Xã phải luôn còn ít nhất một người quản trị. Hãy cấp quyền quản trị cho một cán bộ " +
    "khác trước, rồi thực hiện lại thao tác này.";
  const CAU_TU_THAO_TAC =
    "Không thao tác được lên chính tài khoản của mình. " +
    "Hãy nhờ một người quản trị khác của xã thực hiện.";
  const CAU_VUOT_QUYEN =
    "Vai trò này mang quyền mà tài khoản của bạn không có, nên bạn không gán được: " +
    "admin.user. Hãy nhờ người có đủ quyền thực hiện.";

  it("#13 — câu 'quản trị viên cuối cùng' hiện nguyên văn, trong một vùng báo động", () => {
    const html = ve(FORM_KHOA, { loiMayChu: CAU_QUAN_TRI_CUOI });

    expect(html).toContain(CAU_QUAN_TRI_CUOI);
    expect(html).toContain('role="alert"');

    // KHÔNG mã lỗi, KHÔNG số hiệu HTTP, KHÔNG `trace_id`. Ba thứ ấy là thứ cán bộ đọc lại qua
    // điện thoại cho người hỗ trợ, thay cho câu nói rõ phải làm gì tiếp theo.
    expect(html).not.toContain("last_admin");
    expect(html).not.toContain("409");
  });

  it("#14 — hai ràng buộc cho ra HAI câu khác nhau, không một câu chung", () => {
    const tuThaoTac = ve(FORM_VAI_TRO, { loiMayChu: CAU_TU_THAO_TAC });
    const vuotQuyen = ve(FORM_VAI_TRO, { loiMayChu: CAU_VUOT_QUYEN });

    expect(tuThaoTac).toContain(CAU_TU_THAO_TAC);
    expect(vuotQuyen).toContain(CAU_VUOT_QUYEN);

    // VẾ CHỊU LỰC: hai câu không được trộn. "Tự thao tác lên chính mình" KHÔNG sửa được bằng cách
    // xin thêm quyền, nên câu của nó cố ý không nhắc tới màn Phân quyền; câu còn lại thì có.
    expect(tuThaoTac).not.toContain("không gán được");
    expect(vuotQuyen).not.toContain("chính tài khoản của mình");
  });

  it("không có lỗi thì không dựng sẵn một vùng báo động rỗng", () => {
    // Một chỗ trống dành sẵn cho thông báo lỗi là chỗ chữ nhảy vào giữa lúc người dùng đang gõ.
    const html = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO) });

    expect(html).not.toContain('role="alert"');
    expect(html).not.toContain("thong-bao-loi");
  });
});

describe("ô chọn danh mục giữ được giá trị mà danh mục không tra ra", () => {
  it("bộ phận đã bị xoá mềm vẫn còn là giá trị đang chọn, kèm một mục nói rõ", () => {
    // Ca có thật: danh mục lọc `deleted_at IS NULL`, dòng danh bạ vẫn trỏ tới bộ phận cũ. Không có
    // mục bù này thì `value` không khớp option nào, ô chọn hiện TRỐNG, người dùng đọc ra "chưa
    // phân bộ phận" — rồi bấm Lưu và ghi đè một liên kết họ không định đụng.
    const html = ve(FORM_SUA, { ban: banTuCanBo(CAN_BO), boPhan: [] });

    expect(html).toContain("không còn trong danh mục");
    expect(html).toContain(`value="${CAN_BO.department_id}"`);
  });

  it("mục 'không giữ vai trò nào' LUÔN có mặt — gỡ vai trò phải làm được từ màn hình", () => {
    // `role_id: ""` là một đích đến hợp lệ của hợp đồng (`nguoi_dung.vai_tro_id` cho NULL). Không
    // có mục này thì việc gỡ vai trò của một người không có đường nào thực hiện.
    const html = ve(FORM_VAI_TRO, { vaiTroID: CAN_BO.role_id });

    expect(html).toContain("Không giữ vai trò nào");
    expect(html).toContain('<option value="">');
  });
});
