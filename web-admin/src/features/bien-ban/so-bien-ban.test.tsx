import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { danhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type {
  identity_canBoChonNguoiRa,
  petitions_bienBanRa,
  petitions_ketLuanRa,
  petitions_loaiNhiemVuRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";

import {
  CANH_BAO_BI_MAT,
  CAU_XAC_NHAN_KY,
  CREATE_MEETING_DESCRIPTION,
  CREATE_MEETING_TITLE,
  CHUA_TACH_NHIEM_VU,
  NHAN_NUT_BO_DAU,
  NHAN_NUT_BO_SUNG,
  NHAN_NUT_DANH_DAU,
  NHAN_NUT_GHI_THONG_BAO,
  NHAN_NUT_KY,
  NHAN_NUT_LUU_SUA,
  NHAN_NUT_SUA_BIEN_BAN,
  NHAN_NUT_XAC_NHAN_KY,
  NHAN_NUT_XOA_BIEN_BAN,
  VI_SAO_BIEN_BAN_CON_NHIEM_VU,
  VI_SAO_KET_LUAN_KHOA,
  NGUON_GIAO_KHOA,
  NHAN_NUT_LUU,
  NHAN_NUT_TACH,
  NHAN_NUT_THEM_KET_LUAN,
  PLACEHOLDER_KET_LUAN,
  SO_RONG,
  SPLIT_DIALOG_TITLE,
  SPLIT_SUBMIT_LABEL,
} from "./nhan-bien-ban";
import {
  ChiTietBienBan,
  DanhSachBienBan,
  DongKetLuan,
  FormNhapBienBan,
  HangThemKetLuan,
  TheBienBan,
  type CheBieuMau,
  type PhepTach,
  type PhepVongDoi,
} from "./so-bien-ban";
import { BANG_NHAN_MAC_DINH, TASK_TYPE_MISSING, TASK_TYPE_PLACEHOLDER } from "@/features/nhiem-vu/nhan-nhiem-vu";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **SỐ THỨ TỰ KẾT LUẬN**, và nó là nhóm không ai nhìn
 * thấy trong lúc phát triển: dữ liệu mẫu bao giờ cũng liên tục 1-2-3, nên `{i + 1}` và
 * `{kl.ordinal}` cho ra cùng một trang. Điều phải đúng là chuyện chỉ xảy ra sau một lần xoá mềm —
 * biên bản có các số ①③④, và màn hình phải vẽ đúng ba con số ấy, kèm một khoảng trống ở ②.
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ `renderToStaticMarkup` thoát `"` thành `&quot;` và `&` thành `&amp;`, nên một phép
 * `not.toContain` với chuỗi thô sẽ XANH kể cả khi chữ ấy đang nằm chình ình trên trang — tức là
 * canh đúng con số không. Đã đo ở `features/thu-chi/bang-thu-chi.test.tsx`.
 */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/**
 * The ordinal tile of a conclusion, as it sits in the HTML: a `<span>` whose classes include the
 * stable hook `conclusion-ordinal`, holding exactly the number.
 *
 * PRESENTATIONAL PIN, re-pinned for ADR 0068: it was `<span class="chip">N</span>` before the
 * restyle. What it guards is unchanged — WHICH numbers are drawn — so the regex matches the hook
 * class only, not the utility classes beside it, which are look.
 */
function ordinalTile(n: number): RegExp {
  return new RegExp(`<span class="(?:[^"]* )?conclusion-ordinal(?: [^"]*)?">${n}</span>`);
}

/**
 * The class list of the status pill whose word is `word` (the `<span>` that directly holds an icon
 * and then that word). PRESENTATIONAL PIN for ADR 0068: was `<span class="chip chip-cham">…`.
 */
function pillClassOf(html: string, word: string): string {
  const end = html.indexOf(`</svg>${word}</span>`);
  if (end < 0) return "";
  const open = '<span class="';
  const start = html.lastIndexOf(open, end) + open.length;
  return html.slice(start, html.indexOf('"', start));
}

function ketLuan(sua: Partial<petitions_ketLuanRa> = {}): petitions_ketLuanRa {
  return {
    id: "01JKL1",
    ordinal: 1,
    content: "Giao bộ phận Địa chính rà soát tiến độ tuyến đường Hà Lam – Bình Trị.",
    task_count: 0,
    task_done_count: 0,
    status: "chua-giao",
    no_task: false,
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

function bienBan(sua: Partial<petitions_bienBanRa> = {}): petitions_bienBanRa {
  return {
    id: "01JBB1",
    title: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
    held_on: "2026-08-05",
    reference_no: "31/BB-UBND",
    location: "Phòng họp UBND xã",
    chaired_by: "",
    conclusions: [ketLuan()],
    task_count: 3,
    task_done_count: 1,
    status: "du-thao",
    conclusion_count: 1,
    conclusion_done_count: 0,
    created_by: "CB-2026-7K3M9Q",
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

/**
 * Luồng Tách ở trạng thái nghỉ: không hộp nào mở, không lỗi, không câu báo xong.
 *
 * Danh mục để RỖNG cố ý — nó không phải thứ nhóm nào dưới đây canh, và một danh mục giả đầy đủ chỉ
 * làm các ca khó đọc hơn.
 */
function phepTach(sua: Partial<PhepTach> = {}): PhepTach {
  return {
    danhMuc: { loai: [], mucUuTien: [], khoi: [], boPhan: [] },
    danhBa: null,
    danhBaLanhDao: null,
    dangGui: false,
    moOKetLuan: null,
    loi: null,
    mo: () => {},
    dong: () => {},
    gui: () => {},
    ...sua,
  };
}

/**
 * Vòng đời ở trạng thái nghỉ: KHÔNG có quyền ký, không hộp nào mở, không lỗi, danh bạ chưa đọc.
 * Mặc định KHÔNG quyền ký cố ý — ca "thấy nút Ký" phải tự khai quyền, nên không ca nào xanh nhờ
 * một mặc định rộng tay.
 */
function vongDoi(sua: Partial<PhepVongDoi> = {}): PhepVongDoi {
  return {
    dangGui: false,
    coQuyenKy: false,
    danhBa: null,
    nhanTT: BANG_NHAN_MAC_DINH,
    hop: null,
    loi: null,
    chiTiet: null,
    nhiemVuKL: new Map(),
    moHop: () => {},
    dongHop: () => {},
    dongChiTiet: () => {},
    moSua: () => {},
    moBoSung: () => {},
    ky: () => {},
    ghiThongBao: () => {},
    xoa: () => {},
    suaKL: () => {},
    goKL: () => {},
    danhDau: () => {},
    boDau: () => {},
    batNhiemVuKL: () => {},
    ...sua,
  };
}

function veThe(
  bb: petitions_bienBanRa,
  tach: PhepTach = phepTach(),
  vd: PhepVongDoi = vongDoi(),
): string {
  return renderToStaticMarkup(
    <TheBienBan
      bienBan={bb}
      lanGhiXong={0}
      dangGui={false}
      loiKetLuan={null}
      guiKetLuan={() => {}}
      tach={tach}
      vongDoi={vd}
    />,
  );
}

function veDong(
  kl: petitions_ketLuanRa,
  tach: PhepTach = phepTach(),
  vd: PhepVongDoi = vongDoi(),
  bb: petitions_bienBanRa = bienBan(),
): string {
  return renderToStaticMarkup(
    <DongKetLuan bienBan={bb} ketLuan={kl} tach={tach} vongDoi={vd} />,
  );
}

describe("thẻ biên bản §2", () => {
  it("header mang tiêu đề, dòng meta và badge của máy chủ", () => {
    const html = veThe(bienBan());

    expect(html).toContain("Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026");
    expect(html).toContain("5/8/2026 · 31/BB-UBND · Phòng họp UBND xã");
    // Quyết định chủ dự án 09/10/2026: MỘT badge, không in đậm phần nào, kiểu shadcn
    // `bg-canvas text-ink border-line`, không icon — cả bốn số đọc từ máy chủ.
    expect(html).toMatch(
      /<span class="[^"]*bg-canvas[^"]*">0\/1 kết luận xong · 1\/3 nhiệm vụ xong<\/span>/,
    );
    expect(html).not.toContain("<strong>");
    const pill = html.slice(html.lastIndexOf("<span", html.indexOf("0/1 kết luận xong")));
    const tag = pill.slice(0, pill.indexOf(">"));
    expect(tag).toContain("text-ink ");
    expect(tag).toContain("border-line");
    expect(tag).not.toContain("font-bold");
  });

  it("prototype header: ô icon navy/8, tiêu đề là h2 navy 14.5px đậm, Dự thảo là badge cam", () => {
    const html = veThe(bienBan());
    expect(html).toContain("bg-navy/8 text-navy grid size-9 place-items-center rounded-[9px]");
    expect(html).toMatch(/<h2 class="[^"]*text-\[14\.5px\][^"]*font-bold[^"]*">Giao ban Uỷ ban/);
    expect(html).not.toContain("<h3");
    expect(pillClassOf(html, "Dự thảo")).toContain("text-tangerine");
  });

  it("thiếu số hiệu và địa điểm thì dòng meta chỉ còn ngày — không có dấu chấm bơ vơ", () => {
    const html = veThe(bienBan({ reference_no: "", location: "" }));

    expect(html).toContain("5/8/2026");
    expect(html).not.toContain("· ·");
  });

  it("biên bản chưa có kết luận nào VẪN hiện, và nói ra trạng thái ấy (§7.3)", () => {
    const html = veThe(
      bienBan({ conclusions: [], task_count: 0, task_done_count: 0, conclusion_count: 0 }),
    );

    // Prototype: không câu nào cho "chưa có kết luận" — thẻ chỉ còn hàng thêm kết luận.
    expect(html).not.toContain("chưa ghi kết luận nào");
    expect(html).not.toContain("<ol");
    expect(html).toContain("0/0 kết luận xong · 0/0 nhiệm vụ xong");
    // Hàng thêm kết luận vẫn có: "nhập nháp trước, bổ sung sau" là cả điểm của trạng thái này.
    expect(html).toContain(PLACEHOLDER_KET_LUAN);
  });

  it("hàng thêm kết luận luôn ở cuối thẻ", () => {
    expect(veThe(bienBan())).toContain(nhuTrongHTML(NHAN_NUT_THEM_KET_LUAN));
  });

  it("câu 403 của máy chủ ra thẳng màn hình, nguyên văn", () => {
    const html = renderToStaticMarkup(
      <TheBienBan
        bienBan={bienBan()}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan="Bạn không có quyền tạo nhiệm vụ."
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );

    expect(html).toContain("Bạn không có quyền tạo nhiệm vụ.");
    expect(html).toContain('role="alert"');
  });
});

describe("SỐ THỨ TỰ KẾT LUẬN — nối tiếp số đã cấp, không đếm lại theo vị trí", () => {
  const CO_KHOANG_TRONG = bienBan({
    conclusions: [
      ketLuan({ id: "k1", ordinal: 1, content: "Kết luận thứ nhất." }),
      // ② đã bị gỡ. Số 2 KHÔNG BAO GIỜ được cấp lại (luật 7, bất biến 3).
      ketLuan({ id: "k3", ordinal: 3, content: "Kết luận thứ ba." }),
      ketLuan({ id: "k4", ordinal: 4, content: "Kết luận thứ tư." }),
    ],
  });

  it("vẽ ①③④ đúng như máy chủ trả, KHÔNG vẽ ①②③", () => {
    const html = veThe(CO_KHOANG_TRONG);

    // Số nằm trong ô tròn `conclusion-ordinal`, nên so cả thẻ bao quanh: một phép `toContain("3")`
    // trần sẽ xanh nhờ bất kỳ con số nào khác trên thẻ — kể cả `1/3 nhiệm vụ xong`.
    expect(html).toMatch(ordinalTile(1));
    expect(html).toMatch(ordinalTile(3));
    expect(html).toMatch(ordinalTile(4));
    expect(html).not.toMatch(ordinalTile(2));
  });

  it("nội dung đi CÙNG đúng con số của nó, không lệch một dòng", () => {
    const html = veDong(ketLuan({ ordinal: 4 }));

    expect(html).toMatch(ordinalTile(4));
    expect(html).not.toMatch(ordinalTile(1));
  });

  it("dòng phụ nói `Chưa tách thành nhiệm vụ nào` khi chưa có nhiệm vụ nào", () => {
    expect(veDong(ketLuan({ task_count: 0 }))).toContain(CHUA_TACH_NHIEM_VU);
  });

  it("có nhiệm vụ rồi thì là hai con số, không còn câu `Chưa tách…`", () => {
    const html = veDong(ketLuan({ task_count: 1, task_done_count: 1 }));

    expect(html).toContain("1/1 nhiệm vụ đã hoàn thành");
    expect(html).not.toContain(CHUA_TACH_NHIEM_VU);
  });

  /* ⚠ CA QUAN TRỌNG NHẤT CỦA CẢ TỆP. Con số trong tên đọc được của nút và con số đi trên đường dẫn
   * `{stt}` cùng đọc `ketLuan.ordinal`; lời gọi thì không kiểm được bằng `renderToStaticMarkup`,
   * nên tên nút là chỗ DUY NHẤT con số ấy quan sát được ở đây. Vẽ sai nó là dấu hiệu gửi sai nó. */
  it("TÊN ĐỌC ĐƯỢC CỦA NÚT TÁCH mang số ĐÃ CẤP, không mang vị trí trong mảng", () => {
    const html = veThe(CO_KHOANG_TRONG);

    expect(html).toContain('aria-label="Tách thành nhiệm vụ — kết luận số 1"');
    expect(html).toContain('aria-label="Tách thành nhiệm vụ — kết luận số 3"');
    expect(html).toContain('aria-label="Tách thành nhiệm vụ — kết luận số 4"');
    // Kết luận ở VỊ TRÍ THỨ HAI mang số 3. Một nút mang "số 2" nghĩa là màn đang đếm lại theo vị
    // trí — và con số ấy đi thẳng lên đường dẫn, tách nhầm kết luận.
    expect(html).not.toContain('aria-label="Tách thành nhiệm vụ — kết luận số 2"');
  });
});

/**
 * §3 — luồng Tách.
 *
 * NHÓM NÀY THAY CHO NHÓM CŨ "chỗ ấy là một dòng chữ, KHÔNG phải một nút". Ca ấy đúng cho tới
 * 24/09/2026: biểu mẫu "Giao việc mới" chưa có nên chỗ này là một dòng chữ nói rõ màn chưa dựng.
 * Biểu mẫu nay đã có và được dùng lại nguyên bản, nên hành vi đổi và ca kiểm đổi theo — KHÔNG gỡ
 * đi: mỗi điều ca cũ canh đều có một ca mới canh điều tương ứng ở hành vi mới.
 */
describe("nút `Tách thành nhiệm vụ` §3", () => {
  it("chỗ ấy NAY LÀ MỘT NÚT THẬT, mang đúng nhãn đặc tả vẽ", () => {
    const html = veDong(ketLuan());

    expect(html).toContain("<button");
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_TACH));
  });

  it("hộp Tách chỉ hiện khi chính kết luận NÀY đang mở", () => {
    const dong = ketLuan({ id: "k7" });

    // Chưa mở: không có hộp nào trên dòng.
    expect(veDong(dong)).not.toContain(SPLIT_DIALOG_TITLE);
    // Mở ở một kết luận KHÁC: dòng này vẫn không có hộp. Một hộp một lúc trên cả màn.
    expect(veDong(dong, phepTach({ moOKetLuan: "k9" }))).not.toContain(SPLIT_DIALOG_TITLE);
    // Mở ở chính nó.
    expect(veDong(dong, phepTach({ moOKetLuan: "k7" }))).toContain(SPLIT_DIALOG_TITLE);
  });

  it("prototype: hộp Tách là một DIALOG 500px — tiêu đề, câu kết luận dưới tiêu đề, `Huỷ` rồi `Tạo nhiệm vụ`", () => {
    const html = veDong(
      ketLuan({ id: "k7", content: "Giao Tài chính đối chiếu số liệu." }),
      phepTach({ moOKetLuan: "k7" }),
    );
    const dialog = html.slice(html.indexOf("<dialog"));
    expect(dialog).toMatch(/^<dialog aria-labelledby="tieu-de-giao-viec-moi" aria-modal="true" class="[^"]*max-w-\[500px\]/);
    expect(dialog).toContain(`>${SPLIT_DIALOG_TITLE}</h2>`);
    // Presentation pin (ADR 0068 §5): description colour = shadcn `text-muted-foreground` (spec 00 §5, lần 6).
    expect(dialog).toContain('<p class="m-0 text-sm text-muted-foreground">Giao Tài chính đối chiếu số liệu.</p>');
    expect(dialog.indexOf(">Huỷ</button>")).toBeLessThan(dialog.indexOf(`>${SPLIT_SUBMIT_LABEL}</button>`));
    expect(dialog).not.toContain(">Giao việc</button>");
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026: this pinned "nút tắt" with no type chosen. The button no longer
  // greys out for missing input (a press shows the error under the field), and the type now starts
  // on the commune's active default row — the same `FormGiaoViec` rule as the Nhiệm vụ screen.
  const DIRECTORY_READ = { ok: true as const, duLieu: { items: [] } };
  const typeRow = (code: string, label: string, is_default: boolean, active = true) =>
    ({ id: `01J${code}`, code, label, is_default, active, order: 1, source: "he-thong", tier: 1 }) as petitions_loaiNhiemVuRa;
  const submitTag = (html: string) => {
    const submit = html.slice(html.lastIndexOf("<button", html.indexOf(`>${SPLIT_SUBMIT_LABEL}</button>`)));
    return submit.slice(0, submit.indexOf(">"));
  };

  it("hộp Tách, xã CHƯA có loại mặc định: `— Chọn loại —` (không lấy dòng đầu, NV-01), nút vẫn bấm được", () => {
    const html = veDong(
      ketLuan({ id: "k7" }),
      phepTach({
        moOKetLuan: "k7",
        danhBa: DIRECTORY_READ,
        danhBaLanhDao: DIRECTORY_READ,
        danhMuc: { loai: [typeRow("co-ban", "Nhiệm vụ cơ bản", false)], mucUuTien: [], khoi: [], boPhan: [] },
      }),
    );
    expect(html).toContain('id="giao-loai"');
    expect(html).toContain(`<option value="" selected="">${TASK_TYPE_PLACEHOLDER}</option>`);
    expect(html).not.toMatch(/<option value="co-ban" selected="">/);
    expect(submitTag(html)).not.toContain("disabled");
    expect(html).not.toContain(TASK_TYPE_MISSING);
  });

  it("hộp Tách, xã CÓ loại mặc định đang dùng: chọn sẵn đúng dòng ấy", () => {
    const html = veDong(
      ketLuan({ id: "k7" }),
      phepTach({
        moOKetLuan: "k7",
        danhBa: DIRECTORY_READ,
        danhBaLanhDao: DIRECTORY_READ,
        danhMuc: {
          loai: [typeRow("theo-van-ban", "Theo văn bản", false), typeRow("co-ban", "Nhiệm vụ cơ bản", true)],
          mucUuTien: [],
          khoi: [],
          boPhan: [],
        },
      }),
    );
    expect(html).toContain('<option value="co-ban" selected="">Nhiệm vụ cơ bản</option>');
    expect(html).not.toContain(TASK_TYPE_PLACEHOLDER);
  });

  it("hộp Tách vẫn rộng 500px khi loại mặc định là `Theo văn bản` — không có ba danh sách văn bản để nới", () => {
    const loai = {
      id: "01JLOAI1",
      code: "theo-van-ban",
      label: "Theo văn bản",
      is_default: true,
      active: true,
      order: 1,
    } as petitions_loaiNhiemVuRa;
    const html = veDong(
      ketLuan({ id: "k7" }),
      phepTach({ moOKetLuan: "k7", danhMuc: { loai: [loai], mucUuTien: [], khoi: [], boPhan: [] } }),
    );
    expect(html).toMatch(/<dialog [^>]*class="[^"]*max-w-\[500px\]/);
    expect(html).not.toContain("Văn bản cấp trên giao");
  });

  it("hộp mở thì dùng LẠI biểu mẫu Giao việc của `02-nhiem-vu.md` §7, không dựng bản thứ hai", () => {
    const html = veDong(ketLuan({ id: "k7" }), phepTach({ moOKetLuan: "k7" }));

    // Ba ô chỉ có ở biểu mẫu ấy. Một bản chép tay ở `features/bien-ban/` sẽ trôi khỏi bản gốc, và
    // ngày một bên thêm một trường thì bên kia vẫn xanh (luật 9, cấm #2).
    expect(html).toContain("giao-tieu-de");
    expect(html).toContain("giao-tu-sinh-ma");
    expect(html).toContain("giao-han");
  });

  it("hộp mở thì kết luận gốc Ở LẠI dù ô đã điền sẵn, và nguồn giao là câu KHOÁ", () => {
    const html = veDong(
      ketLuan({ id: "k7", ordinal: 3, content: "Giao Tài chính đối chiếu số liệu." }),
      phepTach({ moOKetLuan: "k7" }),
    );

    // KẾT LUẬN GỐC PHẢI CÒN, và lý do đã ĐỔI kể từ 24/09/2026. Trước: ô "Nội dung nhiệm vụ" chưa
    // điền sẵn được nên đây là chỗ chép từ. Nay `tieuDeCoSan` đã điền sẵn — nhưng ô ấy là thứ cán
    // bộ SẼ SỬA thành một câu giao việc đọc được, nên câu gốc phải còn để đối chiếu. Xoá nó đi thì
    // sau lần sửa đầu tiên không còn chỗ nào trên màn nói kết luận ban đầu viết gì.
    expect(html).toContain("Giao Tài chính đối chiếu số liệu.");
    expect(html).toContain(nhuTrongHTML(NGUON_GIAO_KHOA));
    // Ô ĐÃ ĐIỀN SẴN: nội dung kết luận có mặt TRONG chính thuộc tính `value` của ô nhập, không chỉ
    // ở câu nhắc phía trên. So cả `id="giao-tieu-de"` để phép so không xanh nhờ câu nhắc ấy —
    // cùng chuỗi, hai chỗ, và chỉ một trong hai là thứ ca này canh.
    // Cắt từ `id="giao-tieu-de"` tới dấu đóng thẻ rồi mới so, thay vì ghim nguyên một chuỗi thẻ:
    // thứ tự thuộc tính do React quyết và nó đổi được mà không ai đụng vào màn này — một ca ghim
    // thứ tự sẽ đỏ vì lý do sai, và lần đỏ vì lý do sai đầu tiên là lần người ta bắt đầu bỏ qua nó.
    const o = html.slice(html.indexOf('id="giao-tieu-de"'));
    expect(o.slice(0, o.indexOf("/>"))).toContain(
      'value="Giao Tài chính đối chiếu số liệu."',
    );
  });

  it("câu từ chối của máy chủ hiện NGUYÊN VĂN, và chỉ trên dòng kết luận bị từ chối", () => {
    const tach = phepTach({
      moOKetLuan: "k7",
      loi: { ketLuanID: "k7", thongBao: "Bạn không có quyền tạo nhiệm vụ." },
    });

    expect(veDong(ketLuan({ id: "k7" }), tach)).toContain("Bạn không có quyền tạo nhiệm vụ.");
    // Kết luận khác: câu ấy không nói về nó.
    expect(veDong(ketLuan({ id: "k9" }), tach)).not.toContain(
      "Bạn không có quyền tạo nhiệm vụ.",
    );
  });

  it("đang gửi thì nút khoá — bấm đóng giữa chừng là huỷ khoá chống trùng đang bay", () => {
    expect(veDong(ketLuan(), phepTach({ dangGui: true }))).toContain("disabled");
  });

  /**
   * Phần §3 còn thiếu (hạn gợi ý) hiện ĐÚNG CHỖ nó sẽ ở — trong khung biểu mẫu Giao việc mở từ Tách —
   * dưới dạng dòng gợi ý vô hiệu mang dấu "?" (ADR 0068 §14), không còn trong khối gập đầu màn.
   */
  it("hộp Tách mở thì có dòng HẠN GỢI Ý vô hiệu kèm dấu “?”; hộp đóng thì không", () => {
    const mo = veDong(ketLuan({ id: "k7" }), phepTach({ moOKetLuan: "k7" }));
    expect(mo).toContain("Hạn gợi ý từ ngày nêu trong kết luận");
    expect(mo).toContain('aria-disabled="true"');
    expect(mo).toContain("data-pending-marker");
    expect(mo).toContain("tính năng đang phát triển. Bấm để xem mô tả");

    expect(veDong(ketLuan({ id: "k7" }), phepTach())).not.toContain("Hạn gợi ý từ ngày nêu");
  });

  it("khối gập “N phần của bản thiết kế chưa dựng được” không còn trên thẻ", () => {
    expect(veThe(bienBan())).not.toContain("phần của bản thiết kế chưa dựng được");
  });
});

describe("danh sách thẻ", () => {
  it("sổ rỗng thì nói ra, không để trang trắng", () => {
    const html = renderToStaticMarkup(
      <DanhSachBienBan
        bienBan={[]}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan={null}
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );

    expect(html).toContain(SO_RONG);
  });

  it("câu lỗi của MỘT thẻ không hiện trên thẻ khác", () => {
    // Lỗi mang theo id của biên bản đã từ chối. Hiện nó ở mọi thẻ sẽ nói với cán bộ rằng cả
    // quyển sổ vừa hỏng.
    const html = renderToStaticMarkup(
      <DanhSachBienBan
        bienBan={[bienBan(), bienBan({ id: "01JBB2", title: "Giao ban tháng 9" })]}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan={{ bienBanID: "01JBB2", thongBao: "Thiếu nội dung kết luận." }}
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );

    expect(html.split("Thiếu nội dung kết luận.").length - 1).toBe(1);
  });
});

/** ADR 0068 lần 5 — the composition of `apps/admin` `MeetingMinutes.tsx`. */
describe("prototype composition", () => {
  it("empty register: the prototype's dashed box and sentence", () => {
    const html = renderToStaticMarkup(
      <DanhSachBienBan
        bienBan={[]}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan={null}
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );
    expect(SO_RONG).toBe("Chưa có biên bản nào được nhập.");
    expect(html).toMatch(/^<p class="[^"]*border-dashed[^"]*">Chưa có biên bản nào được nhập\.<\/p>$/);
  });

  it("card: an `article`; header = title · meta · status · count pill; then the conclusions, then the add row", () => {
    const html = veThe(bienBan());
    expect(html.startsWith("<article")).toBe(true);
    const at = (s: string) => html.indexOf(s);
    expect(at("Giao ban Uỷ ban nhân dân xã")).toBeLessThan(at("5/8/2026 · 31/BB-UBND"));
    expect(at("5/8/2026 · 31/BB-UBND")).toBeLessThan(at(">Dự thảo<"));
    expect(at(">Dự thảo<")).toBeLessThan(at("0/1 kết luận xong · 1/3 nhiệm vụ xong"));
    expect(at("1/3 nhiệm vụ xong")).toBeLessThan(at("<ol"));
    expect(at("<ol")).toBeLessThan(at(nhuTrongHTML(PLACEHOLDER_KET_LUAN)));
  });

  it("conclusion row: a bordered box; ordinal · sentence · sub-line, `Tách thành nhiệm vụ` after them", () => {
    const html = veThe(bienBan());
    expect(html).toContain('<li class="mb-2.5 rounded-[10px] border border-line px-4 py-3">');
    const row = veDong(ketLuan());
    const at = (s: string) => row.indexOf(s);
    expect(at("conclusion-ordinal")).toBeLessThan(at("Giao bộ phận Địa chính"));
    expect(at("Giao bộ phận Địa chính")).toBeLessThan(at(CHUA_TACH_NHIEM_VU));
    expect(at(CHUA_TACH_NHIEM_VU)).toBeLessThan(at('aria-label="Tách thành nhiệm vụ — kết luận số 1"'));
    expect(row).toContain('aria-haspopup="dialog"');
  });

  it("add row: two-line box with the prototype placeholder, `Thêm kết luận` beside it, label for AT only", () => {
    const html = renderToStaticMarkup(
      <HangThemKetLuan bienBanID="01JBB1" dangGui={false} adding={false} gui={() => {}} />,
    );
    expect(html).toContain('rows="2"');
    expect(html).toContain('class="an-thi-giac">Thêm một kết luận</label>');
    expect(html.indexOf("<textarea")).toBeLessThan(html.indexOf(nhuTrongHTML(NHAN_NUT_THEM_KET_LUAN)));
    expect(html).toMatch(/^<form class="[^"]*mt-3[^"]*flex[^"]*gap-2/);
    expect(html).not.toContain("animate-spin");
  });

  it("add row: while THIS card's add is in flight the Plus becomes a spinning loader (prototype)", () => {
    const html = renderToStaticMarkup(
      <HangThemKetLuan bienBanID="01JBB1" dangGui adding gui={() => {}} />,
    );
    const button = html.slice(html.indexOf('<button type="submit"'));
    expect(button).toContain("animate-spin");
    expect(button).not.toContain("lucide-plus");
  });

  it("`Nhập biên bản` is a 500px DIALOG with the prototype's title, line, placeholders and field order", () => {
    const html = renderToStaticMarkup(
      <FormNhapBienBan
        che={{ loai: "tao", boSungCho: null }}
        danhBa={DANH_BA}
        loiDanhBa={null}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
        sua={() => {}}
      />,
    );
    expect(html).toMatch(/^<dialog aria-labelledby="tieu-de-nhap-bien-ban" aria-modal="true" class="[^"]*max-w-\[500px\]/);
    expect(html).toContain(`<h2 id="tieu-de-nhap-bien-ban" tabindex="-1" class="m-0 font-heading outline-none text-base leading-none font-medium text-popover-foreground">${CREATE_MEETING_TITLE}</h2>`);
    expect(html).toContain(CREATE_MEETING_DESCRIPTION);
    expect(html).toContain('placeholder="Giao ban tuần 34 năm 2026"');
    expect(html).toContain('placeholder="12/BB-UBND"');
    expect(html).toContain('placeholder="Dán nội dung biên bản vào đây…"');
    const at = (s: string) => html.indexOf(s);
    const order = [
      'id="ten-cuoc-hop"',
      'id="ngay-hop"',
      'id="so-hieu-bien-ban"',
      'id="dia-diem-hop"',
      'id="noi-dung-bien-ban"',
      'id="chu-tri-bien-ban"',
      'id="thu-ky-bien-ban"',
      'id="chon-thanh-phan"',
      'id="cac-ket-luan"',
      'id="tep-dinh-kem-bien-ban"',
      ">Huỷ</button>",
      // Not the words: `Lưu biên bản` is also the start of the prototype's line under the title.
      '<button type="submit"',
    ];
    for (let i = 1; i < order.length; i++) expect(at(order[i - 1]!)).toBeLessThan(at(order[i]!));
  });
});

describe("hàng thêm kết luận", () => {
  it("nút gửi TẮT khi ô còn trống — một kết luận rỗng là một dòng không ai thi hành được", () => {
    const html = renderToStaticMarkup(
      <HangThemKetLuan bienBanID="01JBB1" dangGui={false} adding={false} gui={() => {}} />,
    );

    expect(html).toContain("disabled");
    expect(html).toContain(PLACEHOLDER_KET_LUAN);
  });

  it("id của ô nhập mang id biên bản — hai thẻ trên một trang là hai `label` khác nhau", () => {
    const html = renderToStaticMarkup(
      <HangThemKetLuan bienBanID="01JBB9" dangGui={false} adding={false} gui={() => {}} />,
    );

    expect(html).toContain("them-ket-luan-01JBB9");
  });
});

const DANH_BA: identity_canBoChonNguoiRa[] = [
  { code: "CB-2026-7K3M9Q", full_name: "Nguyễn Văn An", position: "Chủ tịch UBND", department_id: "", email_masked: null },
  { code: "CB-2026-1A2B3C", full_name: "Trần Thị Bình", position: "Văn phòng", department_id: "", email_masked: null },
];

describe("biểu mẫu nhập biên bản §4", () => {
  function veForm(
    loi: string | null = null,
    che: CheBieuMau = { loai: "tao", boSungCho: null },
  ): string {
    return renderToStaticMarkup(
      <FormNhapBienBan
        che={che}
        danhBa={DANH_BA}
        loiDanhBa={null}
        dangGui={false}
        loi={loi}
        huy={() => {}}
        luu={() => {}}
        sua={() => {}}
      />,
    );
  }

  it("có đủ các ô dựng được, và chỉ hai ô là bắt buộc", () => {
    const html = veForm();

    // Labels are `Field`'s (spec 04: navy 13px semibold, the `*` a red span after the word).
    const SAO = '<span class="ml-1 text-danger">*</span>';
    expect(html).toContain(`Tên cuộc họp${SAO}`);
    expect(html).toContain(`Ngày họp${SAO}`);
    expect(html.split(SAO).length - 1).toBe(2);
    expect(html).toContain("Số hiệu biên bản");
    expect(html).toContain("Địa điểm");
    expect(html).toContain("Thành phần tham dự");
    expect(html).toContain("Nội dung biên bản");
    expect(html).toContain("Các kết luận");
    // Số hiệu KHÔNG mang dấu sao: nó tuỳ chọn, và nó không phải mã định danh.
    expect(html).not.toContain("Số hiệu biên bản *");
    expect(html).not.toContain("Chủ trì *");
    expect(html).not.toContain("Thư ký *");
  });

  it("prototype: không còn hai dòng gợi ý dưới Số hiệu và dưới Nội dung biên bản", () => {
    const html = veForm();
    expect(html).not.toContain("không phải mã tra cứu");
    expect(html).not.toContain("Toàn văn. Đọc lại");
  });

  it('ngày họp là ô `type="date"` — ngày lịch, không mốc thời gian', () => {
    expect(veForm()).toContain('type="date"');
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 09/10/2026 (ADR 0068 lần 6 #4): the create button no longer greys out for
  // missing input — a press shows the error under the field (flow test `so-bien-ban.flow.test.tsx`).
  it("nút Lưu BẤM ĐƯỢC khi chưa gõ tên và ngày — lỗi hiện dưới ô khi bấm", () => {
    const html = veForm();
    const form = html.slice(0, html.lastIndexOf("</form>"));
    const nut = form.slice(form.lastIndexOf("<button"), form.lastIndexOf("</button>"));
    expect(nut).toContain(NHAN_NUT_LUU);
    // The ATTRIBUTE, not the word: the class list always carries Tailwind's `disabled:` variants.
    expect(nut).not.toMatch(/\sdisabled(=|\s|>)/);
  });

  it("câu từ chối của máy chủ hiện nguyên văn", () => {
    const html = veForm("biên bản họp: thiếu ngày họp");

    expect(html).toContain("biên bản họp: thiếu ngày họp");
    expect(html).toContain('role="alert"');
  });

  it("cảnh báo bí mật nhà nước có mặt — đã quyết: không có cờ “mật” nào", () => {
    const html = veForm();
    expect(html).toContain(CANH_BAO_BI_MAT);
    expect(html).not.toMatch(/>\s*Mật\s*</);
  });

  it("Chủ trì và Thư ký là ô CHỌN từ danh bạ: giá trị là MÃ, chữ hiện là họ tên · chức vụ", () => {
    const html = veForm();
    expect(html).toContain(">Chủ trì<");
    expect(html).toContain(">Thư ký<");
    // Giá trị gửi đi là MÃ nghiệp vụ; họ tên chỉ là chữ hiện.
    expect(html).toContain('value="CB-2026-7K3M9Q"');
    expect(html).toContain("Nguyễn Văn An · Chủ tịch UBND");
    expect(html).not.toContain('value="Nguyễn Văn An"');
  });

  it("thành phần: ô chọn cán bộ VÀ ô chữ tự do — khách mời không có tài khoản vẫn ghi được", () => {
    const html = veForm();
    expect(html).toContain('id="chon-thanh-phan"');
    expect(html).toContain('id="thanh-phan-khac"');
  });

  /**
   * Ô tệp ở CUỐI biểu mẫu (§4) là chỗ giữ: vô hiệu, mang dấu "?", và KHÔNG có `name` — nên nó không
   * bao giờ đi lên cùng thân yêu cầu, kể cả khi ai đó gỡ `disabled`.
   */
  it("ô Tệp đính kèm là ô tệp VÔ HIỆU, không `name`, có dấu “?”, đứng sau ô cuối cùng dựng được", () => {
    const html = veForm();
    const tep = /<input[^>]*type="file"[^>]*>/.exec(html)?.[0] ?? "";

    expect(tep).toContain('id="tep-dinh-kem-bien-ban"');
    expect(tep).toContain('disabled=""');
    expect(tep).not.toContain("name=");
    expect(html).toContain("Tệp đính kèm — bản scan biên bản — tính năng đang phát triển");
    expect(html.indexOf('id="tep-dinh-kem-bien-ban"')).toBeGreaterThan(html.indexOf('id="cac-ket-luan"'));
    // Bản sửa cũng có ô ấy: đặc tả §4 dùng một biểu mẫu cho cả hai.
    expect(veForm(null, { loai: "sua", ban: bienBan() })).toContain('id="tep-dinh-kem-bien-ban"');
  });

  it("biểu mẫu BỔ SUNG nói nó bổ sung cho biên bản đã ký nào", () => {
    const goc = bienBan({ status: "da-ky", title: "Giao ban tháng 7" });
    const html = veForm(null, { loai: "tao", boSungCho: goc });
    expect(html).toContain("Lập biên bản bổ sung");
    expect(html).toContain("Giao ban tháng 7");
  });

  it("biểu mẫu SỬA điền sẵn, KHÔNG có ô “Các kết luận”, nút Lưu tắt khi chưa đổi gì", () => {
    const ban = bienBan({
      chaired_by: "CB-2026-7K3M9Q",
      minutes_taker: "CB-2026-1A2B3C",
      content: "Toàn văn đã lưu.",
      attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"],
    });
    const html = veForm(null, { loai: "sua", ban });
    expect(html).toContain("Sửa biên bản (dự thảo)");
    expect(html).toContain("Toàn văn đã lưu.");
    expect(html).toContain("Đại diện thôn Hà Lam");
    expect(html).not.toContain('id="cac-ket-luan"');
    // The form's last button — inside `</form>`; the dialog's own ✕ (ADR 0068 lần 6) comes after it.
    const form = html.slice(0, html.lastIndexOf("</form>"));
    const nut = form.slice(form.lastIndexOf("<button"), form.lastIndexOf("</button>"));
    expect(nut).toContain("disabled");
    expect(nut).toContain(NHAN_NUT_LUU_SUA);
  });
});

describe("vòng đời trên thẻ — dự thảo và đã ký", () => {
  const DA_KY = bienBan({
    status: "da-ky",
    signed_by: "CB-2026-7K3M9Q",
    signed_at: "2026-08-06T03:00:00Z",
  });

  it("chip trạng thái: “Dự thảo” và “Đã ký”", () => {
    expect(veThe(bienBan())).toContain(">Dự thảo<");
    expect(veThe(DA_KY)).toContain(">Đã ký<");
  });

  it("dự thảo: Sửa · Gỡ · Thêm kết luận có; Ghi Thông báo · Lập bổ sung KHÔNG", () => {
    const html = veThe(bienBan({ task_count: 0 }));
    expect(html).toContain(NHAN_NUT_SUA_BIEN_BAN);
    expect(html).toContain(NHAN_NUT_XOA_BIEN_BAN);
    expect(html).toContain(PLACEHOLDER_KET_LUAN);
    expect(html).not.toContain(NHAN_NUT_GHI_THONG_BAO);
    expect(html).not.toContain(NHAN_NUT_BO_SUNG);
  });

  it("đã ký: KHÔNG Sửa · Gỡ · Ký · Thêm kết luận; CÓ Lập bổ sung và Ghi Thông báo (khi chưa có)", () => {
    const html = veThe(DA_KY, phepTach(), vongDoi({ coQuyenKy: true }));
    expect(html).not.toContain(NHAN_NUT_SUA_BIEN_BAN);
    expect(html).not.toContain(NHAN_NUT_XOA_BIEN_BAN);
    expect(html).not.toContain(`>${NHAN_NUT_KY}<`);
    expect(html).not.toContain(PLACEHOLDER_KET_LUAN);
    expect(html).toContain(NHAN_NUT_BO_SUNG);
    expect(html).toContain(NHAN_NUT_GHI_THONG_BAO);
  });

  it("đã ký VÀ đã có Thông báo: nút Ghi Thông báo biến mất — máy chủ chỉ nhận một lần", () => {
    const html = veThe(
      bienBan({
        status: "da-ky",
        notice: { reference_no: "12/TB-UBND", issued_on: "2026-08-07" },
      }),
    );
    expect(html).not.toContain(NHAN_NUT_GHI_THONG_BAO);
  });

  it("nút Ký CHỈ hiện khi phiên có `task.approve` — ca BỊ TỪ CHỐI trước", () => {
    expect(veThe(bienBan(), phepTach(), vongDoi({ coQuyenKy: false }))).not.toContain(
      `>${NHAN_NUT_KY}<`,
    );
    expect(veThe(bienBan(), phepTach(), vongDoi({ coQuyenKy: true }))).toContain(
      `>${NHAN_NUT_KY}<`,
    );
  });

  it("hộp Ký nói rõ hệ quả và có hai ô Thông báo tuỳ chọn", () => {
    const html = veThe(
      bienBan(),
      phepTach(),
      vongDoi({ coQuyenKy: true, hop: { dich: "01JBB1", loai: "ky" } }),
    );
    expect(html).toContain(CAU_XAC_NHAN_KY);
    expect(html).toContain(NHAN_NUT_XAC_NHAN_KY);
    expect(html).toContain("Số, ký hiệu Thông báo kết luận");
  });

  it("Gỡ biên bản TẮT kèm lý do khi còn nhiệm vụ trỏ về", () => {
    const html = veThe(bienBan({ task_count: 2 }));
    expect(html).toContain(VI_SAO_BIEN_BAN_CON_NHIEM_VU);
    const dau = html.lastIndexOf("<button", html.indexOf('aria-describedby="ly-do-xoa-01JBB1"'));
    const nut = html.slice(dau, html.indexOf("</button>", dau));
    expect(nut).toContain("disabled");
    expect(nut).toContain(NHAN_NUT_XOA_BIEN_BAN);
    // Spec 05 §B: the reason is ALSO the disabled button's `title`, and sits under the row.
    expect(nut).toContain(`title="${VI_SAO_BIEN_BAN_CON_NHIEM_VU}"`);
    expect(html).toContain(
      `<p class="m-0 -mt-1 mb-3 text-[11.5px] text-ink-muted" id="ly-do-xoa-01JBB1">${VI_SAO_BIEN_BAN_CON_NHIEM_VU}</p>`,
    );
  });

  it("nút Gỡ biên bản: viền (outline) chữ đỏ — không nền đỏ", () => {
    const html = veThe(bienBan({ task_count: 0 }));
    const dau = html.lastIndexOf("<button", html.indexOf(`${NHAN_NUT_XOA_BIEN_BAN}</button>`));
    const tag = html.slice(dau, html.indexOf(">", dau));
    expect(tag).toContain("text-danger");
    expect(tag).toContain("border-border");
    expect(tag).not.toContain("bg-destructive");
    expect(tag).not.toContain("nut-xoa");
  });

  it("hộp Gỡ biên bản đòi lý do — nút tắt khi chưa gõ", () => {
    const html = veThe(
      bienBan({ task_count: 0 }),
      phepTach(),
      vongDoi({ hop: { dich: "01JBB1", loai: "xoa" } }),
    );
    expect(html).toContain("Lý do gỡ biên bản *");
    // Spec 05 §B: Gỡ opens a DIALOG; the reason field (rule 7) is inside it.
    expect(html).toContain("<dialog");
    expect(html.slice(html.indexOf("<dialog"))).toContain("Lý do gỡ biên bản *");
    // PRESENTATIONAL PIN re-pinned for ADR 0068 (was `<button type="submit" class="nut-xoa"
    // disabled="">`): the submit is still the red removal button, and still disabled.
    const submitStart = html.indexOf('<button type="submit"');
    const submitTag = html.slice(submitStart, html.indexOf(">", submitStart));
    expect(submitTag).toContain("nut-xoa");
    expect(submitTag).toContain('disabled=""');
  });

  it("câu lỗi vòng đời của MỘT biên bản chỉ hiện trên thẻ ấy", () => {
    const vd = vongDoi({ loi: { dich: "01JBB2", thongBao: "biên bản họp đã ký — …" } });
    expect(veThe(bienBan(), phepTach(), vd)).not.toContain("biên bản họp đã ký — …");
    expect(veThe(bienBan({ id: "01JBB2" }), phepTach(), vd)).toContain("biên bản họp đã ký — …");
  });

  it("biên bản bổ sung có liên kết về biên bản gốc", () => {
    expect(veThe(bienBan({ supplements_id: "01JBBGOC" }))).toContain('href="#bien-ban-01JBBGOC"');
  });

  it("“Xem biên bản” là neo tới chính thẻ — không mang dữ liệu nào ngoài id", () => {
    expect(veThe(bienBan())).toContain('href="#bien-ban-01JBB1"');
  });
});

describe("Xem biên bản — toàn văn, người ký, Thông báo, bổ sung hai chiều", () => {
  const CHI_TIET = bienBan({
    status: "da-ky",
    chaired_by: "CB-2026-7K3M9Q",
    minutes_taker: "CB-2026-1A2B3C",
    signed_by: "CB-2026-7K3M9Q",
    signed_at: "2026-08-06T03:00:00Z",
    notice: { reference_no: "12/TB-UBND", issued_on: "2026-08-07" },
    supplements_id: "01JBBGOC",
    supplemented_by: ["01JBBBS1"],
    content: "Dòng một.\nDòng hai.",
    attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"],
  });

  function veChiTiet(bb: petitions_bienBanRa = CHI_TIET): string {
    return renderToStaticMarkup(
      <ChiTietBienBan
        tai={{ pha: "xong", duLieu: bb }}
        danhBa={danhBaTheoMa(DANH_BA)}
        dong={() => {}}
      />,
    );
  }

  it("hiện chủ trì, thư ký bằng HỌ TÊN tra từ danh bạ", () => {
    const html = veChiTiet();
    expect(html).toContain("Nguyễn Văn An · Chủ tịch UBND");
    expect(html).toContain("Trần Thị Bình · Văn phòng");
  });

  it("thành phần: dòng chữ tự do ra nguyên văn", () => {
    expect(veChiTiet()).toContain("Đại diện thôn Hà Lam");
  });

  it("toàn văn giữ xuống dòng", () => {
    expect(veChiTiet()).toContain("Dòng một.<br/>");
  });

  it("Thông báo kết luận: số và NGÀY LỊCH d/M/yyyy", () => {
    expect(veChiTiet()).toContain("Số 12/TB-UBND, ngày 7/8/2026");
  });

  it("người ký và ngày ký", () => {
    expect(veChiTiet()).toContain("Nguyễn Văn An · Chủ tịch UBND · ngày 6/8/2026");
  });

  it("bổ sung HAI CHIỀU: về biên bản gốc và tới biên bản bổ sung", () => {
    const html = veChiTiet();
    expect(html).toContain('href="#bien-ban-01JBBGOC"');
    expect(html).toContain('href="#bien-ban-01JBBBS1"');
  });

  it("chủ trì không còn trong danh bạ vẫn hiện MÃ kèm câu trung tính, không một ô trống", () => {
    expect(veChiTiet(bienBan({ chaired_by: "CB-DA-NGHI" }))).toContain(
      "CB-DA-NGHI (không có trong danh bạ cán bộ đang hoạt động)",
    );
  });

  it("đọc hỏng thì câu máy chủ ra nguyên văn", () => {
    const html = renderToStaticMarkup(
      <ChiTietBienBan
        tai={{ pha: "loi", thongBao: "Không tìm thấy biên bản." }}
        danhBa={null}
        dong={() => {}}
      />,
    );
    expect(html).toContain("Không tìm thấy biên bản.");
  });
});

describe("dòng kết luận — chip trạng thái từ MÁY CHỦ và các nút theo trạng thái", () => {
  it("chip đọc đúng mã máy chủ; quá hạn là chip ĐỎ", () => {
    expect(veDong(ketLuan({ status: "chua-giao" }))).toContain(">Chưa giao<");
    expect(veDong(ketLuan({ status: "dang-thuc-hien", task_count: 1 }))).toContain(
      ">Đang thực hiện<",
    );
    expect(
      veDong(ketLuan({ status: "hoan-thanh", task_count: 1, task_done_count: 1 })),
    ).toContain(">Hoàn thành<");
    // Quá hạn là huy hiệu ĐỎ (tông `danger`) — và chữ đã nói rõ, màu không là tín hiệu duy nhất.
    // Presentation pin (ADR 0068 §5): the danger chip is `text-danger` since spec 00 §4 (lần 6).
    const overdue = veDong(ketLuan({ status: "qua-han", task_count: 1 }));
    expect(pillClassOf(overdue, "Quá hạn")).toContain("text-danger");
    expect(pillClassOf(veDong(ketLuan({ status: "chua-giao" })), "Chưa giao")).not.toContain(
      "text-danger",
    );
  });

  it("KHÔNG tự suy trạng thái từ bộ đếm: 1/1 xong mà máy chủ nói “qua-han” thì vẽ Quá hạn", () => {
    const html = veDong(ketLuan({ status: "qua-han", task_count: 1, task_done_count: 1 }));
    expect(html).toContain(">Quá hạn<");
    expect(html).not.toContain(">Hoàn thành<");
  });

  it("đánh dấu “không phát sinh”: chip nói đúng lý do, nút Tách ẨN, còn Bỏ dấu", () => {
    const html = veDong(ketLuan({ no_task: true, status: "hoan-thanh" }));
    expect(html).toContain(">Không phát sinh<");
    expect(pillClassOf(html, "Không phát sinh")).toContain("bg-ink-muted/10");
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_TACH));
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_BO_DAU));
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
  });

  it("dự thảo, chưa có nhiệm vụ: Sửa · Gỡ bấm được, Đánh dấu có", () => {
    const html = veDong(ketLuan({ ordinal: 3, task_count: 0 }));
    expect(html).toContain('aria-label="Sửa kết luận số 3"');
    expect(html).toContain('aria-label="Gỡ kết luận số 3"');
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
    expect(html).not.toContain(VI_SAO_KET_LUAN_KHOA);
  });

  it("dự thảo, ĐÃ có nhiệm vụ: Sửa · Gỡ TẮT kèm lý do, Đánh dấu ẨN", () => {
    const html = veDong(ketLuan({ ordinal: 3, task_count: 2, status: "dang-thuc-hien" }));
    for (const nhan of ["Sửa kết luận số 3", "Gỡ kết luận số 3"]) {
      const dau = html.lastIndexOf("<button", html.indexOf(`aria-label="${nhan}"`));
      const nut = html.slice(dau, html.indexOf("</button>", dau));
      expect(nut).toContain("disabled");
    }
    expect(html).toContain(VI_SAO_KET_LUAN_KHOA);
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
  });

  it("câu khoá kết luận nằm MỘT lần trong DOM; Sửa và Gỡ cùng trỏ vào nó, và mang nó ở `title`", () => {
    const html = veDong(ketLuan({ id: "k3", ordinal: 3, task_count: 2, status: "dang-thuc-hien" }));
    expect(html.split(VI_SAO_KET_LUAN_KHOA).length - 1).toBe(3); // the <p> + two `title`s
    expect(html.split(`>${VI_SAO_KET_LUAN_KHOA}</p>`).length - 1).toBe(1);
    expect(html).toContain('<p class="m-0 mt-1 text-[11px] text-ink-muted" id="ly-do-kl-k3">');
    expect(html.split('aria-describedby="ly-do-kl-k3"').length - 1).toBe(2);
    expect(html).not.toContain("an-thi-giac");
  });

  it("hàng thao tác kết luận: ghost sm h-7 px-2 11.5px; Gỡ chữ đỏ không nền đỏ", () => {
    const html = veDong(ketLuan({ ordinal: 3, task_count: 0 }));
    expect(html).toContain('<div class="mt-2 flex flex-wrap items-center gap-1">');
    const dau = html.lastIndexOf("<button", html.indexOf('aria-label="Gỡ kết luận số 3"'));
    const tag = html.slice(dau, html.indexOf(">", dau));
    expect(tag).toContain("h-7");
    expect(tag).toContain("px-2 ");
    expect(tag).toContain("text-[11.5px]");
    expect(tag).toContain("text-danger");
    expect(tag).toContain("hover:bg-danger/10");
    expect(tag).not.toContain("bg-destructive");
  });

  it("dòng kết luận: nội dung là <p> 12.8px; tiến độ và chip cùng một dòng; chip h-5 10.5px", () => {
    const html = veDong(ketLuan({ status: "chua-giao" }));
    expect(html).toContain(`<p class="m-0 text-[12.8px] break-words">${ketLuan().content}</p>`);
    expect(html).toContain('<div class="mt-1 flex flex-wrap items-center gap-2">');
    const cls = pillClassOf(html, "Chưa giao");
    expect(cls).toContain("h-5");
    expect(cls).toContain("text-[10.5px]");
    expect(cls).toContain("bg-canvas");
    expect(cls).toContain("text-ink-muted");
    expect(pillClassOf(veDong(ketLuan({ status: "dang-thuc-hien", task_count: 1 })), "Đang thực hiện")).toContain(
      "text-brand",
    );
    expect(pillClassOf(veDong(ketLuan({ status: "hoan-thanh", task_count: 1 })), "Hoàn thành")).toContain(
      "text-leaf",
    );
    expect(html).toMatch(/class="conclusion-ordinal [^"]*bg-brand\/12[^"]*"/);
  });

  it("Sửa kết luận: câu đổi thành ô nhập TẠI CHỖ, không còn <p> nội dung", () => {
    const kl = ketLuan({ id: "k3", ordinal: 3 });
    const html = veDong(kl, phepTach(), vongDoi({ hop: { dich: "k3", loai: "sua-kl" } }));
    expect(html).toContain('id="sua-ket-luan-k3"');
    expect(html).not.toContain(`<p class="m-0 text-[12.8px] break-words">${kl.content}</p>`);
    expect(html.indexOf("conclusion-ordinal")).toBeLessThan(html.indexOf('id="sua-ket-luan-k3"'));
    expect(html.indexOf('id="sua-ket-luan-k3"')).toBeLessThan(html.indexOf(CHUA_TACH_NHIEM_VU));
  });

  it("Gỡ kết luận mở DIALOG có ô lý do bắt buộc", () => {
    const html = veDong(
      ketLuan({ id: "k3", ordinal: 3 }),
      phepTach(),
      vongDoi({ hop: { dich: "k3", loai: "go-kl" } }),
    );
    expect(html).toContain("<dialog");
    expect(html.slice(html.indexOf("<dialog"))).toContain("Lý do gỡ kết luận số 3 *");
  });

  it("biên bản ĐÃ KÝ: Sửa · Gỡ · Đánh dấu · Bỏ dấu ẨN, còn Tách", () => {
    const daKy = bienBan({ status: "da-ky" });
    const html = veDong(ketLuan({ ordinal: 3 }), phepTach(), vongDoi(), daKy);
    expect(html).not.toContain("Sửa kết luận số 3");
    expect(html).not.toContain("Gỡ kết luận số 3");
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_TACH));
  });

  it("có nhiệm vụ thì có nút mở danh sách; mở ra thì mã · tên · trạng thái · hạn", () => {
    const kl = ketLuan({ id: "k3", ordinal: 3, task_count: 1, status: "dang-thuc-hien" });
    expect(veDong(kl)).toContain("Xem 1 nhiệm vụ đã tách");
    const nv = {
      code: "NV12",
      title: "Đối chiếu số liệu giải ngân",
      status: "dang-thuc-hien",
      due_at: "2026-08-20T16:59:59Z",
    } as petitions_nhiemVuRa;
    const html = veDong(
      kl,
      phepTach(),
      vongDoi({ nhiemVuKL: new Map([["k3", { pha: "xong", duLieu: [nv] }]]) }),
    );
    // The title IS the link to the task (spec 05 §B); the code no longer has a span of its own.
    expect(html).toContain(
      '<a href="/nhiem-vu?task=NV12" class="text-[12px] font-medium text-navy hover:underline">Đối chiếu số liệu giải ngân</a>',
    );
    expect(html).not.toContain(">NV12<");
    expect(html).toContain("Đang thực hiện");
    expect(html).toContain(
      '<span class="text-[11px] whitespace-nowrap text-ink-muted tabular-nums">Hạn 20/8/2026</span>',
    );
    expect(html).toContain(
      'class="m-0 mt-2 list-none space-y-1.5 rounded-[8px] border border-line bg-canvas px-3 py-2"',
    );
  });

  it("câu lỗi vòng đời của MỘT kết luận chỉ hiện trên dòng ấy", () => {
    const vd = vongDoi({ loi: { dich: "k7", thongBao: "kết luận đã được tách — …" } });
    expect(veDong(ketLuan({ id: "k7" }), phepTach(), vd)).toContain("kết luận đã được tách");
    expect(veDong(ketLuan({ id: "k9" }), phepTach(), vd)).not.toContain("kết luận đã được tách");
  });
});
