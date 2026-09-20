"use client";

import { useEffect, useMemo, useState } from "react";

import { docDanhMucNghiepVu, type BayDanhMuc, type MucDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";

import {
  GHI_CHU_CHI_XEM,
  GIAI_THICH_DA_TAT,
  GIAI_THICH_THANG_BAC,
  lopTrangThaiMuc,
  nhanMacDinh,
  nhanNhomRong,
  nhanSoMuc,
  nhanTrangThaiMuc,
} from "./nhan-danh-muc";
import { nhomDanhMuc, type NhomDanhMuc } from "./nhom-danh-muc";

/**
 * Tab "Danh mục" — `docs/ui-ux/14-cau-hinh.md §5`, bảy danh mục nghiệp vụ của đơn vị.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TAB NÀY KHÔNG CÓ CỔNG QUYỀN, VÀ ĐÓ LÀ MỘT KHÁC BIỆT CÓ CHỦ Ý SO VỚI HAI TAB BÊN CẠNH — đọc
 * hết đoạn này trước khi "cho nó giống `tab-nguoi-dung.tsx`".
 *
 * Đặc tả §12.8 gán tab này cho quyền `admin.lookup`. Nhưng BẢY TUYẾN phía sau nó đều khai
 * `any-authenticated` ở máy chủ, và lý do được ghi ngay trên từng tuyến: nhãn danh mục xuất hiện
 * ở ô chọn và bộ lọc của gần như mọi màn hình, nên đòi một quyền cấu hình sẽ làm rỗng những ô đó
 * cho mọi tài khoản không phải quản trị (`tasks/web/done/*.json`, `permission_reason`).
 *
 * Nên nếu ẩn tab này theo `admin.lookup` thì:
 *
 *   1. Giao diện trở thành thứ DUY NHẤT quyết định — chính hình dạng luật 5 cấm #1 nói tới. Ở tab
 *      Người dùng, ẩn tab chỉ là tiện dụng vì `GET /api/v1/staff` vẫn trả 403 cho người thiếu
 *      quyền. Ở đây máy chủ trả 200 cho mọi tài khoản đã đăng nhập, nên cái "cổng" này không canh
 *      gì cả: nó chỉ trông như đang canh.
 *   2. Màn hình sẽ hiện một câu SAI — "tài khoản của bạn không có quyền xem danh mục" — với người
 *      mà máy chủ vẫn đang phục vụ đúng danh mục ấy trên năm màn hình khác.
 *
 * Mâu thuẫn đặc tả ↔ hợp đồng này đã được báo cho người dùng. Ngày khách chốt rằng danh mục phải
 * đòi `admin.lookup`, chỗ sửa là KHAI BÁO TUYẾN ở máy chủ trước, rồi mới tới một cổng ở đây —
 * không bao giờ chỉ ở đây.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ MỘT BỀ MẶT GHI NÀO, VÀ CŨNG KHÔNG CÓ MỘT NÚT MỜ ĐỂ DÀNH CHỖ. Đặc tả §5 vẽ
 * `+ Thêm mục`, `⬆ Nhập từ Excel`, `✎`, `Tắt`, `🗑` và `Đặt mặc định`; không thao tác nào trong số
 * đó có tuyến phía sau, và đó không phải thiếu sót của hợp đồng: câu hỏi mở #21 — đơn vị được sửa
 * cả DANH SÁCH MÃ hay chỉ được sửa nhãn và thứ tự — chưa được khách trả lời, và một bề mặt ghi
 * chính là hình dạng trả lời câu ấy (`kb/00-foundation/open-questions.json` #21; chi phí đảo
 * ngược ghi rõ là "CAO VÀ BẤT ĐỐI XỨNG"). Một nút `+ Thêm mục` bấm vào không có gì xảy ra còn tệ
 * hơn không có nút: nó khiến cán bộ tin rằng mình đã thao tác sai.
 *
 * BA NHÓM CỦA ĐẶC TẢ KHÔNG CÓ Ở ĐÂY (`Lĩnh vực phản ánh`, `Loại đơn thư`, `Trạng thái nhiệm vụ`)
 * — chúng chưa có tuyến nào trong hợp đồng REST. Xem `nhom-danh-muc.ts`.
 *
 * CỘT `Nguồn` CỦA ĐẶC TẢ (`Hệ thống` / `Đơn vị`) KHÔNG CÓ Ở ĐÂY: hợp đồng không phát ra trường
 * nào cho nó (`schema.gen.ts` — một mục có đúng `id`, `code`, `label`, `is_default`, `active`).
 * Đoán nguồn từ một trường khác là dựng ra một câu trả lời thứ hai cho câu hỏi mà chính câu hỏi
 * mở #21 đang treo.
 */
export function TabDanhMuc() {
  /** `null` là chưa đọc xong. Bảy danh mục về trong MỘT lượt nên cả tab có đúng một pha tải. */
  const [bay, datBay] = useState<BayDanhMuc | null>(null);

  /**
   * BẢY DANH MỤC, ĐỌC ĐÚNG MỘT LƯỢT KHI MỞ MÀN HÌNH — `[]` ở cuối effect là phần quan trọng nhất
   * của khối này. Không đọc lại theo từng nhóm, và tuyệt đối không đọc theo từng dòng
   * (`skills/load-data-once`, dạng 1).
   *
   * `bo` chặn một phản hồi về sau khi component đã rời màn hình ghi vào state đã chết.
   *
   * KHÔNG CÓ BỘ ĐỆM NÀO SỐNG QUA LẦN MỞ MÀN HÌNH: dữ liệu nằm trong state của component. Một biến
   * ở mức module giữ bảy danh mục lại là đúng hình dạng của một lần danh mục đơn vị này hiện trên
   * màn hình đơn vị khác — rò dữ liệu giữa hai cơ quan nhà nước, không phải lỗi hiển thị (luật 1,
   * cấm #1; lý lẽ đầy đủ ở `lib/api/danh-muc-nghiep-vu.ts`).
   */
  useEffect(() => {
    let bo = false;
    docDanhMucNghiepVu().then((d) => {
      if (!bo) datBay(d);
    });
    return () => {
      bo = true;
    };
  }, []);

  const nhom = useMemo<readonly NhomDanhMuc[]>(
    () => (bay === null ? [] : nhomDanhMuc(bay)),
    [bay],
  );

  // Câu giải thích "vì sao mục đã tắt vẫn nằm trong bảng" chỉ hiện khi CÓ bảng để mà giải thích.
  const coMucNao = nhom.some((n) => n.trangThai.pha === "coMuc");

  return (
    <section className="tab-danh-muc" aria-labelledby="tieu-de-danh-muc">
      <h2 id="tieu-de-danh-muc">Danh mục</h2>
      <p className="ghi-chu">{GHI_CHU_CHI_XEM}</p>

      {/* MỘT dòng `role="status"` cho cả tab, không phải bảy. Bảy vùng thông báo cùng đọc
          "Đang tải…" là bảy lần trình đọc màn hình ngắt lời người dùng về cùng một chuyện. */}
      {bay === null && <p role="status">Đang tải danh mục của đơn vị…</p>}

      {coMucNao && <p className="ghi-chu">{GIAI_THICH_DA_TAT}</p>}

      {nhom.map((n) => (
        <NhomMuc key={n.khoa} nhom={n} />
      ))}
    </section>
  );
}

/** Một nhóm danh mục: tiêu đề, rồi đúng một trong ba trạng thái. Không trạng thái nào là ô trống.
 *
 * XUẤT RA để `tab-danh-muc.test.tsx` kết xuất được nó bằng `react-dom/server`. Đây là thành phần
 * thuần: nhận một trạng thái đã tính sẵn và không gọi gì. Trước khi nó được xuất, phép đột biến
 * bôi trắng câu báo danh mục rỗng ở dòng dưới KHÔNG làm ca test nào đỏ — mọi ca đều canh quyết
 * định trong module thuần, không ca nào canh việc quyết định ấy có ra tới trang hay không.
 */
export function NhomMuc({ nhom }: { nhom: NhomDanhMuc }) {
  const maTieuDe = `nhom-danh-muc-${nhom.khoa}`;

  return (
    <section className="nhom-danh-muc" aria-labelledby={maTieuDe}>
      <h3 id={maTieuDe}>
        {nhom.nhan}
        {nhom.trangThai.pha === "coMuc" && (
          <span className="dem-muc">{nhanSoMuc(nhom.trangThai.muc.length)}</span>
        )}
      </h3>

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải, không rẽ nhánh theo `code`, không
          hiện `trace_id` (`lib/api/goi.ts`). Năm dịch vụ đứng sau bảy nhóm, nên một nhóm hỏng
          trong khi sáu nhóm còn lại hiện bình thường là ca có thật. */}
      {nhom.trangThai.pha === "khongDocDuoc" && (
        <p className="thong-bao-loi" role="alert">
          {nhom.trangThai.thongBao}
        </p>
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI — và hôm nay đây là đường THÔNG THƯỜNG của mọi
          đơn vị, không phải ca biên. Câu chữ và lý do đầy đủ ở `nhan-danh-muc.ts`. */}
      {nhom.trangThai.pha === "chuaCoMuc" && (
        <p className="trang-thai-rong">{nhanNhomRong(nhom.nhan)}</p>
      )}

      {nhom.trangThai.pha === "coMuc" && (
        <>
          {nhom.thuTuLaThangBac && <p className="ghi-chu">{GIAI_THICH_THANG_BAC}</p>}
          <BangMuc nhan={nhom.nhan} muc={nhom.trangThai.muc} />
        </>
      )}
    </section>
  );
}

/**
 * Bảng các mục của một nhóm.
 *
 * `muc` ĐƯỢC DỰNG THEO ĐÚNG THỨ TỰ NHẬN ĐƯỢC. Không `sort`, không `filter` — ở nhóm mức ưu tiên
 * thứ tự ấy LÀ thang bậc của đơn vị, và sắp lại nó không phải đổi cách trình bày mà là đổi mức
 * việc đơn vị coi là gấp nhất, với màn hình vẫn trông bình thường (`nhom-danh-muc.ts`).
 *
 * MỤC ĐÃ TẮT VẪN HIỆN, KHÔNG LỌC BỚT: một hồ sơ đã lập theo mã đã tắt vẫn phải tra ra được nhãn
 * của nó. Máy chủ cũng trả về chúng đúng vì lý do này ("Returned rather than filtered
 * server-side, so the list screen can show it while a picker filters it out").
 */
function BangMuc({ nhan, muc }: { nhan: string; muc: readonly MucDanhMuc[] }) {
  return (
    // `role="region"` + `tabIndex` để vùng cuộn ngang tới được bằng bàn phím — ở 320px bảng cuộn
    // ngang chứ không đổi thành thẻ, vì đổi `display` của phần tử bảng làm mất ngữ nghĩa bảng với
    // trình đọc màn hình (cùng lý lẽ với `bang-can-bo`).
    <div className="bang-cuon" role="region" aria-label={`Danh mục ${nhan}`} tabIndex={0}>
      <table className="bang-danh-muc">
        <caption className="an-thi-giac">
          Các mục của danh mục {nhan}, theo đúng thứ tự đơn vị đã sắp
        </caption>
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Nhãn hiển thị</th>
            <th scope="col">Thứ tự</th>
            <th scope="col">Mặc định</th>
            <th scope="col">Trạng thái</th>
          </tr>
        </thead>
        <tbody>
          {muc.map((m, i) => (
            <tr key={m.id}>
              {/* `code` font mono theo đặc tả §5: nó là một slug được gõ lại và đọc qua điện
                  thoại, nên `l`/`1` và `O`/`0` phải phân biệt được. */}
              <td className="ma-muc">{m.code}</td>
              <td>{m.label}</td>
              {/* THỨ TỰ LÀ VỊ TRÍ TRONG MẢNG, TÍNH LÚC DỰNG — không phải một trường của hợp đồng.
                  Hợp đồng cố ý KHÔNG phát ra số thứ tự: vị trí trong `items` đã là thứ tự, và một
                  con số đặt cạnh mảng là bản sao thứ hai — bản sao mà một client giữ lại sau khi
                  sắp lại mảng chính là bản nói dối về bậc nào trên bậc nào
                  (`service-petitions/internal/http/muc_uu_tien_nhiem_vu.go`). Tính từ chỉ số thì
                  nó không thể lệch với thứ tự đang hiện ngay bên cạnh. */}
              <td>{i + 1}</td>
              <td>{nhanMacDinh(m.is_default)}</td>
              <td>
                <span className={lopTrangThaiMuc(m.active)}>{nhanTrangThaiMuc(m.active)}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
