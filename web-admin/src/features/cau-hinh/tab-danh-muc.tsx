"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

import {
  suaMuc,
  themMuc,
  xoaMuc,
  type MoTaDanhMucGhi,
  type MucDanhMucGhi,
} from "@/lib/api/danh-muc";
import { docDanhMucNghiepVu, type BayDanhMuc, type MucDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";
import { QUYEN_QUAN_LY_DANH_MUC, quyetDinhTheoKhoa } from "@/lib/quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";

import {
  CANH_BAO_XOA,
  CAU_THIEU_QUYEN_GHI,
  GHI_CHU_BA_TANG,
  GHI_CHU_NHOM_CHI_XEM,
  GIAI_THICH_DA_TAT,
  GIAI_THICH_O_MA,
  GIAI_THICH_O_NHAN,
  GIAI_THICH_THANG_BAC,
  NUT_BAT_LAI,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA,
  NUT_TAT,
  NUT_THEM,
  NUT_XAC_NHAN_XOA,
  NUT_XOA,
  O_LY_DO_XOA,
  O_MA,
  O_MAC_DINH,
  O_NHAN,
  O_THU_TU,
  daLuu,
  daThem,
  daXoa,
  giaiThichKhongThaoTac,
  lopTrangThaiMuc,
  nhanMacDinh,
  nhanNguon,
  nhanNhomRong,
  nhanNutCuaDong,
  nhanSoMuc,
  nhanTrangThaiMuc,
  tieuDeSua,
  tieuDeThem,
  tieuDeXoa,
  type LoiRaCuaNhomRong,
} from "./nhan-danh-muc";
import { nhomDanhMuc, type NhomDanhMuc } from "./nhom-danh-muc";
import { NhomTrangThaiNhiemVu } from "./nhom-trang-thai-nhiem-vu";
import { choBatLai, kiemLyDoXoa, laMucGhi, thaoTacCuaMuc } from "./tang-danh-muc";

/**
 * Tab "Danh mục" — `docs/ui-ux/14-cau-hinh.md §5`, bảy danh mục nghiệp vụ của đơn vị, cả bảy sửa
 * được từ màn hình.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ MÀN HÌNH ĐẦU TIÊN CỦA WEB QUẢN TRỊ CÓ ĐƯỜNG GHI. Ba điều dưới đây quyết định nó đúng
 * hay sai, và không điều nào nhìn thấy được bằng mắt trên một màn hình chạy tốt:
 *
 * 1. NÚT VẼ THEO TẦNG, VÀ TẦNG ĐẾN TỪ MÁY CHỦ. Tầng 3 không có `Tắt` và không có `Xoá`; tầng 2
 *    không có `Xoá`. Quy tắc nằm ở `tang-danh-muc.ts` (hàm thuần, có bài test) chứ không rải
 *    trong JSX, vì một nhánh `if` giữa hai thẻ `<td>` là nhánh không bài test nào chạm tới. Cái
 *    CHẶN thật là trigger CSDL (ADR 0024); vẽ đúng nút chỉ để cán bộ không bấm vào một 409.
 *
 * 2. `source` VÀ `tier` KHÔNG BAO GIỜ ĐI LÊN. Máy chủ trả 400 nếu thân yêu cầu nhắc tới chúng,
 *    kể cả với giá trị đúng. Lối sai tự nhiên nhất — đọc một dòng rồi gửi lại chính nó — bị
 *    chặn ở `lib/api/danh-muc.ts`, nơi thân yêu cầu được dựng từng trường.
 *
 * 3. ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP. Ba tuyến ghi khai `RequirePermission
 *    ("admin.lookup")` ở máy chủ và kiểm trên TỪNG yêu cầu; ẩn nút chỉ để cán bộ không bấm vào
 *    một thứ chắc chắn trả 403 (luật 5, cấm #1).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * VÌ SAO BẢNG VẪN HIỆN CHO MỌI TÀI KHOẢN, CHỈ NÚT MỚI ẨN — và đây là chỗ tab này khác hẳn tab
 * Người dùng. Bảy tuyến ĐỌC khai `any-authenticated`, có lý do ghi ngay trên tuyến: nhãn danh
 * mục xuất hiện ở ô chọn và bộ lọc của gần như mọi màn hình. Dựng một cổng quyền quanh cả tab
 * sẽ là để GIAO DIỆN từ chối điều máy chủ đang phục vụ bình thường, và hiện ra một câu SAI —
 * "bạn không có quyền xem danh mục" — với người đang đọc đúng danh mục ấy trên năm màn hình
 * khác. Cổng quyền vì vậy đặt đúng chỗ máy chủ đặt nó: quanh ba thao tác GHI.
 *
 * HAI NHÓM CỦA ĐẶC TẢ KHÔNG CÓ Ở ĐÂY: `Lĩnh vực phản ánh` và `Loại đơn thư` chưa có tuyến nào
 * trong hợp đồng REST. `Trạng thái nhiệm vụ` CÓ, là nhóm thứ tám, nhưng KHÔNG cùng khuôn thêm ·
 * sửa · xoá mềm: câu hỏi #21 đã chốt là đơn vị chỉ đổi nhãn và thứ tự của một bộ mã cố định, nên
 * nó là một thành phần riêng — `nhom-trang-thai-nhiem-vu.tsx`.
 *
 * NÚT `⬆ Nhập từ Excel` CỦA ĐẶC TẢ CŨNG KHÔNG CÓ: không có tuyến nào phía sau nó. Một nút bấm
 * vào không có gì xảy ra còn tệ hơn không có nút — cán bộ sẽ tin là mình thao tác sai.
 */
export function TabDanhMuc() {
  /** `null` là chưa đọc xong. Bảy danh mục về trong MỘT lượt nên cả tab có đúng một pha tải. */
  const [bay, datBay] = useState<BayDanhMuc | null>(null);
  /**
   * Tăng lên sau mỗi lần ghi thành công để đọc lại từ máy chủ.
   *
   * ĐỌC LẠI, KHÔNG VÁ TẠI CHỖ. Vá mảng trong state bằng dòng máy chủ vừa trả về là nhanh hơn,
   * và nó sai ở đúng chỗ khó thấy: đặt một mục làm mặc định sẽ GỠ mặc định của mục khác trong
   * cùng danh mục (`moc_mac_dinh` chỉ nhận một dòng), nên bảng sẽ hiện hai mục cùng "Mặc định"
   * cho tới lần mở màn hình sau. Một lượt đọc lại thì không thể lệch.
   */
  const [lanDoc, datLanDoc] = useState(0);

  const [dangMo, datDangMo] = useState<DangMo>(null);
  /**
   * Mã trạng thái nhiệm vụ đang mở biểu mẫu sửa ở nhóm thứ tám, hoặc `null`. Giữ Ở TAB, cạnh
   * `dangMo`, để luật "một biểu mẫu cho cả tab" phủ cả nhóm ấy: mở bên này thì đóng bên kia.
   */
  const [maTrangThaiDangSua, datMaTrangThaiDangSua] = useState<string | null>(null);
  const [ban, datBan] = useState<BanNhap>(BAN_TRONG);
  /** Lỗi do chính màn hình phát hiện trước khi gửi. Khác hẳn câu của máy chủ — xem `loiMayChu`. */
  const [loiTaiCho, datLoiTaiCho] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  const phien = usePhien();
  /**
   * BA TRẠNG THÁI, KHÔNG HAI: chưa đọc xong phiên thì chưa vẽ nút ghi nào. "Chưa biết" không
   * được hành xử như "có quyền" — và cũng không được hành xử như "thiếu quyền", vì câu giải
   * thích thiếu quyền hiện ra trong lúc còn đang đọc là nói một điều chưa biết đúng hay sai.
   */
  const quyetDinhGhi = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_DANH_MUC);
  const coQuyenGhi = quyetDinhGhi !== null && quyetDinhGhi.hien;

  useEffect(() => {
    let bo = false;
    docDanhMucNghiepVu().then((d) => {
      if (!bo) datBay(d);
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  const nhom = useMemo<readonly NhomDanhMuc[]>(() => (bay === null ? [] : nhomDanhMuc(bay)), [bay]);

  const coMucNao = nhom.some((n) => n.trangThai.pha === "coMuc");

  /** Mở một biểu mẫu: dọn sạch mọi thông báo của lần trước, và nạp giá trị đang có vào bản nháp. */
  const mo = useCallback((m: DangMo, banDau: BanNhap) => {
    datMaTrangThaiDangSua(null);
    datDangMo(m);
    datBan(banDau);
    datLoiTaiCho("");
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  const dong = useCallback(() => {
    datDangMo(null);
    datBan(BAN_TRONG);
    datLoiTaiCho("");
    datLoiMayChu("");
  }, []);

  /** Nhóm thứ tám mở biểu mẫu: đóng biểu mẫu của bảy nhóm kia trước (một biểu mẫu cho cả tab). */
  const moSuaTrangThai = useCallback(
    (ma: string | null) => {
      if (ma !== null) dong();
      datMaTrangThaiDangSua(ma);
    },
    [dong],
  );

  /** Sau một lần ghi thành công: đóng biểu mẫu, nói ra đã làm gì, và đọc lại từ máy chủ. */
  const xong = useCallback((cau: string) => {
    datDangMo(null);
    datBan(BAN_TRONG);
    datLoiTaiCho("");
    datLoiMayChu("");
    datCauDaXong(cau);
    datLanDoc((n) => n + 1);
  }, []);

  const thaoTac = useMemo<ThaoTacNhom>(
    () => ({
      them: (mo_, nhanNhom) =>
        mo(
          {
            kieu: "them",
            ghi: mo_,
            nhanNhom,
            // KHOÁ CHỐNG TRÙNG SINH KHI MỞ BIỂU MẪU, KHÔNG SINH LÚC GỬI. Bấm `Lưu` lần thứ hai
            // sau một lỗi mạng phải mang ĐÚNG khoá của lần đầu: lần đầu có thể đã tới máy chủ,
            // và một khoá mới biến lần thử lại thành một mục thứ hai trong danh mục.
            khoaChongTrung: khoaChongTrungMoi(),
          },
          BAN_TRONG,
        ),
      sua: (mo_, nhanNhom, m) =>
        mo({ kieu: "sua", ghi: mo_, nhanNhom, muc: m }, banTuMuc(m)),
      xoa: (mo_, nhanNhom, m) => mo({ kieu: "xoa", ghi: mo_, nhanNhom, muc: m }, BAN_TRONG),
      datTrangThai: (mo_, nhanNhom, m, dung) => {
        datLoiMayChu("");
        datCauDaXong("");
        datDangGui(true);
        void suaMuc(mo_, m.id, { active: dung }).then((kq) => {
          datDangGui(false);
          if (kq.ok) xong(daLuu(nhanNhom));
          else datLoiMayChu(kq.thongBao);
        });
      },
    }),
    [mo, xong],
  );

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    if (dangMo.kieu === "xoa") {
      // PHÉP KIỂM DUY NHẤT MÀN HÌNH TỰ LÀM — xem `tang-danh-muc.ts`. Không gửi gì khi lý do rỗng.
      const kiem = kiemLyDoXoa(ban.lyDo);
      if (!kiem.ok) {
        datLoiTaiCho(kiem.loi);
        return;
      }
      datLoiTaiCho("");
      datLoiMayChu("");
      datDangGui(true);
      void xoaMuc(dangMo.ghi, dangMo.muc.id, kiem.giaTri).then((kq) => {
        datDangGui(false);
        if (kq.ok) xong(daXoa(dangMo.nhanNhom));
        else datLoiMayChu(kq.thongBao);
      });
      return;
    }

    datLoiTaiCho("");
    datLoiMayChu("");
    datDangGui(true);

    // KHÔNG KIỂM KHUÔN MÃ, ĐỘ DÀI NHÃN HAY KHOẢNG THỨ TỰ Ở ĐÂY. Máy chủ đã kiểm cả ba, mỗi thứ
    // kèm một câu tiếng Việt nói rõ phải sửa gì; chép chúng xuống client là dựng bản sao thứ hai
    // của một bộ quy tắc nghiệp vụ, và bản sao ấy trôi mà không bài test nào đỏ (luật 9).
    if (dangMo.kieu === "them") {
      void themMuc(
        dangMo.ghi,
        { code: ban.ma, label: ban.nhan, order: soThuTu(ban.thuTu), is_default: ban.macDinh },
        dangMo.khoaChongTrung,
      ).then((kq) => {
        datDangGui(false);
        if (kq.ok) xong(daThem(dangMo.nhanNhom));
        else datLoiMayChu(kq.thongBao);
      });
      return;
    }

    void suaMuc(dangMo.ghi, dangMo.muc.id, {
      label: ban.nhan,
      order: soThuTu(ban.thuTu),
      is_default: ban.macDinh,
    }).then((kq) => {
      datDangGui(false);
      if (kq.ok) xong(daLuu(dangMo.nhanNhom));
      else datLoiMayChu(kq.thongBao);
    });
  }, [ban, dangGui, dangMo, xong]);

  return (
    <section className="tab-danh-muc" aria-labelledby="tieu-de-danh-muc">
      <h2 id="tieu-de-danh-muc">Danh mục</h2>
      <p className="ghi-chu">{GHI_CHU_BA_TANG}</p>

      {/* MỘT dòng `role="status"` cho cả tab, không phải bảy. Bảy vùng thông báo cùng đọc
          "Đang tải…" là bảy lần trình đọc màn hình ngắt lời người dùng về cùng một chuyện. */}
      {bay === null && <p role="status">Đang tải danh mục của đơn vị…</p>}

      {/* Câu xác nhận sau khi ghi. `role="status"` chứ không `alert`: không có gì hỏng. */}
      {cauDaXong !== "" && <p role="status">{cauDaXong}</p>}

      {/* Không đọc được quyền (phiên hết hạn, mạng hỏng) thì nói đúng câu của máy chủ, và không
          vẽ nút ghi nào. Đóng khi không chắc. */}
      {quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-doc-duoc" && (
        <p className="thong-bao-loi" role="alert">
          {quyetDinhGhi.thongBao}
        </p>
      )}
      {quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-du-quyen" && (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_GHI}</p>
      )}

      {coMucNao && <p className="ghi-chu">{GIAI_THICH_DA_TAT}</p>}

      {nhom.map((n) => (
        <NhomMuc
          key={n.khoa}
          nhom={n}
          coQuyenGhi={coQuyenGhi}
          thaoTac={thaoTac}
          form={
            dangMo !== null && dangMo.ghi.khoa === n.ghi?.khoa ? (
              <BieuMauGhi
                dangMo={dangMo}
                ban={ban}
                datBan={datBan}
                loiTaiCho={loiTaiCho}
                loiMayChu={loiMayChu}
                dangGui={dangGui}
                onGui={guiBieuMau}
                onHuy={dong}
              />
            ) : null
          }
        />
      ))}

      {/* NHÓM THỨ TÁM, KHUÔN RIÊNG (#21): chỉ đổi nhãn và thứ tự, không thêm · tắt · xoá. Nút ghi
          chỉ vẽ khi đã biết có quyền — `coQuyenGhi` là `false` khi phiên còn đang đọc. */}
      <NhomTrangThaiNhiemVu
        coQuyenGhi={coQuyenGhi}
        maDangSua={maTrangThaiDangSua}
        moSua={moSuaTrangThai}
      />
    </section>
  );
}

/* ---- trạng thái của các biểu mẫu ghi ------------------------------------------------------ */

/**
 * Biểu mẫu nào đang mở, của nhóm nào, trên mục nào.
 *
 * MỘT BIỂU MẪU MỞ TẠI MỘT THỜI ĐIỂM, CHO CẢ TAB. Bảy nhóm mở được bảy biểu mẫu cùng lúc là bảy
 * bản nháp mà cán bộ không thấy hết trên một màn hình 320px, và bản nháp không nhìn thấy là bản
 * nháp bị gửi nhầm.
 */
type DangMo =
  | { kieu: "them"; ghi: MoTaDanhMucGhi; nhanNhom: string; khoaChongTrung: string }
  | { kieu: "sua"; ghi: MoTaDanhMucGhi; nhanNhom: string; muc: MucDanhMucGhi }
  | { kieu: "xoa"; ghi: MoTaDanhMucGhi; nhanNhom: string; muc: MucDanhMucGhi }
  | null;

/** Bản nháp đang gõ. Chuỗi hết, kể cả thứ tự — ô nhập của trình duyệt trả về chuỗi. */
type BanNhap = { ma: string; nhan: string; thuTu: string; lyDo: string; macDinh: boolean };

const BAN_TRONG: BanNhap = { ma: "", nhan: "", thuTu: "", lyDo: "", macDinh: false };

function banTuMuc(m: MucDanhMucGhi): BanNhap {
  return { ma: m.code, nhan: m.label, thuTu: String(m.order), lyDo: "", macDinh: m.is_default };
}

/**
 * Ô `Thứ tự` rỗng nghĩa là KHÔNG ĐỔI, không phải số 0.
 *
 * `undefined` bị `JSON.stringify` bỏ khỏi thân, nên máy chủ giữ nguyên thứ tự đang có. Trả 0 ở
 * đây thì một cán bộ chỉ định sửa nhãn sẽ vô tình đẩy mục lên đầu danh mục — và ở nhóm mức ưu
 * tiên, thứ tự ấy LÀ thang bậc của đơn vị.
 *
 * KHÔNG `parseInt`: `parseInt("3 chữ")` trả 3, tức là nuốt một lỗi gõ thành một con số. `Number`
 * trả `NaN`, và `NaN` được trả về `undefined` để máy chủ không nhận một thân JSON có `null`.
 */
function soThuTu(oNhap: string): number | undefined {
  const sach = oNhap.trim();
  if (sach === "") return undefined;
  const n = Number(sach);
  return Number.isInteger(n) ? n : undefined;
}

/**
 * Khoá chống trùng cho một lần thêm mục.
 *
 * `crypto.randomUUID` có sẵn trong mọi trình duyệt chạy được ứng dụng này và không cần thư viện.
 * Nó chỉ tồn tại trong ngữ cảnh an toàn (HTTPS hoặc localhost) — đúng ngữ cảnh mà cookie phiên
 * `Secure` cũng đòi, nên không có môi trường nào ứng dụng đăng nhập được mà hàm này lại vắng.
 */
function khoaChongTrungMoi(): string {
  return crypto.randomUUID();
}

/** Ba thao tác ghi mà một dòng hoặc một tiêu đề nhóm có thể yêu cầu. */
export type ThaoTacNhom = {
  readonly them: (ghi: MoTaDanhMucGhi, nhanNhom: string) => void;
  readonly sua: (ghi: MoTaDanhMucGhi, nhanNhom: string, m: MucDanhMucGhi) => void;
  readonly xoa: (ghi: MoTaDanhMucGhi, nhanNhom: string, m: MucDanhMucGhi) => void;
  readonly datTrangThai: (
    ghi: MoTaDanhMucGhi,
    nhanNhom: string,
    m: MucDanhMucGhi,
    dung: boolean,
  ) => void;
};

/** Một nhóm danh mục: tiêu đề, rồi đúng một trong ba trạng thái. Không trạng thái nào là ô trống.
 *
 * XUẤT RA để `tab-danh-muc.test.tsx` kết xuất được nó bằng `react-dom/server`. Đây là thành phần
 * thuần: nhận một trạng thái đã tính sẵn và không gọi gì. Trước khi nó được xuất, phép đột biến
 * bôi trắng câu báo danh mục rỗng ở dòng dưới KHÔNG làm ca test nào đỏ — mọi ca đều canh quyết
 * định trong module thuần, không ca nào canh việc quyết định ấy có ra tới trang hay không.
 */
export function NhomMuc({
  nhom,
  coQuyenGhi,
  thaoTac,
  form,
}: {
  nhom: NhomDanhMuc;
  coQuyenGhi: boolean;
  thaoTac: ThaoTacNhom;
  /** Biểu mẫu đang mở của CHÍNH nhóm này, hoặc `null`. Do tab dựng, xem `TabDanhMuc`. */
  form: ReactNode;
}) {
  const maTieuDe = `nhom-danh-muc-${nhom.khoa}`;
  // Rút ra một `const` để phép thu hẹp kiểu còn sống bên trong các hàm xử lý sự kiện phía dưới.
  // Đọc thẳng `nhom.ghi` trong một closure thì TypeScript phải giả định nó đã đổi, và cách "sửa"
  // gần nhất là một phép ép kiểu — tức là mã tự khẳng định điều trình biên dịch vừa từ chối.
  const ghi = nhom.ghi;
  const veDuocNutGhi = ghi !== null && coQuyenGhi;

  return (
    <section className="nhom-danh-muc" aria-labelledby={maTieuDe}>
      <h3 id={maTieuDe}>
        {nhom.nhan}
        {nhom.trangThai.pha === "coMuc" && (
          <span className="dem-muc">{nhanSoMuc(nhom.trangThai.muc.length)}</span>
        )}
      </h3>

      {/* Nhóm chưa có tuyến ghi thì NÓI RA, một lần, ngay dưới tiêu đề của chính nó. Không vẽ
          nút mờ để dành chỗ: một nút bấm vào không có gì xảy ra khiến cán bộ tin mình bấm sai. */}
      {ghi === null && <p className="ghi-chu">{GHI_CHU_NHOM_CHI_XEM}</p>}

      {veDuocNutGhi && ghi !== null && (
        <p>
          <button type="button" className="nut-phu" onClick={() => thaoTac.them(ghi, nhom.nhan)}>
            {NUT_THEM}
          </button>
        </p>
      )}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải, không rẽ nhánh theo `code`, không
          hiện `trace_id` (`lib/api/goi.ts`). Năm dịch vụ đứng sau bảy nhóm, nên một nhóm hỏng
          trong khi sáu nhóm còn lại hiện bình thường là ca có thật. */}
      {nhom.trangThai.pha === "khongDocDuoc" && (
        <p className="thong-bao-loi" role="alert">
          {nhom.trangThai.thongBao}
        </p>
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI — và hôm nay đây vẫn là đường THÔNG THƯỜNG của
          hầu hết đơn vị. Câu chữ và lý do đầy đủ ở `nhan-danh-muc.ts`. */}
      {nhom.trangThai.pha === "chuaCoMuc" && (
        <p className="trang-thai-rong">{nhanNhomRong(nhom.nhan, loiRaCuaNhom(nhom, coQuyenGhi))}</p>
      )}

      {nhom.trangThai.pha === "coMuc" && (
        <>
          {nhom.thuTuLaThangBac && <p className="ghi-chu">{GIAI_THICH_THANG_BAC}</p>}
          <BangMuc
            nhan={nhom.nhan}
            muc={nhom.trangThai.muc}
            ghi={ghi}
            veDuocNutGhi={veDuocNutGhi}
            thaoTac={thaoTac}
          />
        </>
      )}

      {form}
    </section>
  );
}

/** Ba lý do một nhóm rỗng không thêm được mục — ba câu khác nhau, xem `nhanNhomRong`. */
function loiRaCuaNhom(nhom: NhomDanhMuc, coQuyenGhi: boolean): LoiRaCuaNhomRong {
  if (nhom.ghi === null) return "khongCoTuyen";
  return coQuyenGhi ? "themDuoc" : "thieuQuyen";
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
 *
 * HAI CỘT CUỐI CHỈ MỌC KHI NHÓM CÓ ĐƯỜNG GHI. Một nhóm không có tuyến ghi thì cột hành động
 * không có hành động nào để chứa, và cột `Nguồn` chỉ có nghĩa cạnh quy tắc ba tầng của đường ghi.
 */
function BangMuc({
  nhan,
  muc,
  ghi,
  veDuocNutGhi,
  thaoTac,
}: {
  nhan: string;
  muc: readonly MucDanhMuc[];
  ghi: MoTaDanhMucGhi | null;
  veDuocNutGhi: boolean;
  thaoTac: ThaoTacNhom;
}) {
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
            {ghi !== null && <th scope="col">Nguồn</th>}
            <th scope="col">Trạng thái</th>
            {ghi !== null && (
              <th scope="col">
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {muc.map((m, i) => (
            <tr key={m.id}>
              {/* `code` font mono theo đặc tả §5: nó là một slug được gõ lại và đọc qua điện
                  thoại, nên `l`/`1` và `O`/`0` phải phân biệt được. */}
              <td className="ma-muc">{m.code}</td>
              <td>{m.label}</td>
              {/* THỨ TỰ LÀ TRƯỜNG CỦA HỢP ĐỒNG KHI CÓ, VÀ LÀ VỊ TRÍ TRONG MẢNG KHI KHÔNG. Cả bảy
                  danh mục đều phát ra `order`, và cột này PHẢI hiện đúng con số ấy: biểu mẫu sửa
                  đổi chính nó, nên một cột hiện vị trí trong mảng sẽ nói "3" sau khi cán bộ vừa
                  đặt thứ tự 7. Vị trí trong `items` chỉ còn là đường lui cho một dòng méo hình
                  dạng (hợp đồng trôi) — `laMucGhi` trả `false` cho nó. */}
              <td>{laMucGhi(m) ? m.order : i + 1}</td>
              <td>{nhanMacDinh(m.is_default)}</td>
              {ghi !== null && <td>{laMucGhi(m) ? nhanNguon(m.source) : ""}</td>}
              <td>
                <span className={lopTrangThaiMuc(m.active)}>{nhanTrangThaiMuc(m.active)}</span>
              </td>
              {ghi !== null && (
                <td className="o-thao-tac">
                  <NutCuaDong
                    ghi={ghi}
                    nhanNhom={nhan}
                    m={m}
                    veDuocNutGhi={veDuocNutGhi}
                    thaoTac={thaoTac}
                  />
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Những nút của MỘT dòng — và đây là chỗ ba tầng hiện ra thành giao diện.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHÉP QUYẾT ĐỊNH KHÔNG NẰM Ở ĐÂY, NÓ Ở `tang-danh-muc.ts`. Hàm này chỉ đọc kết quả. Viết
 * `m.tier !== 3 && <button>Tắt</button>` ngay tại đây thì con số 3 nằm giữa hai thẻ JSX, không
 * bài test nào chạm tới nó, và bản sao thứ hai của quy tắc ba tầng bắt đầu từ đúng dòng ấy.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * DÒNG KHÔNG CÓ THAO TÁC NÀO THÌ NÓI RA, không để ô trống. Một ô trống đọc bằng trình đọc màn
 * hình thành im lặng, và người xem bằng mắt thì không phân biệt được "mục này không xoá được"
 * với "màn hình chưa dựng xong".
 */
function NutCuaDong({
  ghi,
  nhanNhom,
  m,
  veDuocNutGhi,
  thaoTac,
}: {
  ghi: MoTaDanhMucGhi;
  nhanNhom: string;
  m: MucDanhMuc;
  veDuocNutGhi: boolean;
  thaoTac: ThaoTacNhom;
}) {
  if (!veDuocNutGhi) return null;
  if (!laMucGhi(m)) return <span className="ghi-chu">{giaiThichKhongThaoTac(null)}</span>;

  const cho = thaoTacCuaMuc(m);
  const batLaiDuoc = choBatLai(m);

  // Tầng 3 đang dùng: không `Tắt`, không `Xoá`, chỉ còn `Sửa`. Tầng 2: không `Xoá`.
  if (!cho.doiNhan && !cho.tat && !cho.xoa && !batLaiDuoc) {
    return <span className="ghi-chu">{giaiThichKhongThaoTac(m.tier)}</span>;
  }

  return (
    <span className="cum-nut">
      {cho.doiNhan && (
        <button
          type="button"
          className="nut-phu"
          aria-label={nhanNutCuaDong(NUT_SUA, m.label)}
          onClick={() => thaoTac.sua(ghi, nhanNhom, m)}
        >
          {NUT_SUA}
        </button>
      )}

      {/* `Tắt` THEO TẦNG, `Bật lại` THÌ KHÔNG. Trigger chỉ từ chối chiều bật → tắt ở tầng 3;
          chiều ngược lại luôn được phép, và phải được phép, nếu không một dòng tầng 3 lỡ tắt
          sẽ không còn đường quay lại (`tang-danh-muc.ts`, `choBatLai`). */}
      {cho.tat && m.active && (
        <button
          type="button"
          className="nut-phu"
          aria-label={nhanNutCuaDong(NUT_TAT, m.label)}
          onClick={() => thaoTac.datTrangThai(ghi, nhanNhom, m, false)}
        >
          {NUT_TAT}
        </button>
      )}
      {batLaiDuoc && (
        <button
          type="button"
          className="nut-phu"
          aria-label={nhanNutCuaDong(NUT_BAT_LAI, m.label)}
          onClick={() => thaoTac.datTrangThai(ghi, nhanNhom, m, true)}
        >
          {NUT_BAT_LAI}
        </button>
      )}

      {cho.xoa && (
        <button
          type="button"
          className="nut-phu nut-xoa"
          aria-label={nhanNutCuaDong(NUT_XOA, m.label)}
          onClick={() => thaoTac.xoa(ghi, nhanNhom, m)}
        >
          {NUT_XOA}
        </button>
      )}

      {/* Tầng 2 và 3 vẫn phải nói vì sao thiếu nút, kể cả khi còn nút `Sửa` đứng bên cạnh. */}
      {!cho.xoa && <span className="ghi-chu">{giaiThichKhongThaoTac(m.tier)}</span>}
    </span>
  );
}

/* ---- ba biểu mẫu ghi ---------------------------------------------------------------------- */

/**
 * Biểu mẫu đang mở — thêm, sửa, hoặc xoá.
 *
 * THUẦN TRÌNH BÀY: mọi giá trị đi vào qua `ban`, mọi thay đổi đi ra qua `datBan`, và phép kiểm
 * nằm ở chỗ gọi. Tách như vậy để hai nhánh KHÔNG ai nhìn thấy trong lúc phát triển — "lý do xoá
 * còn trống" và "máy chủ vừa từ chối" — kết xuất được bằng `react-dom/server` mà không cần một
 * trình duyệt giả lập (cùng lý lẽ với `KhungQuyen` ở `features/quyen/cong-quyen.tsx`).
 */
export function BieuMauGhi({
  dangMo,
  ban,
  datBan,
  loiTaiCho,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: NonNullable<DangMo>;
  ban: BanNhap;
  datBan: (b: BanNhap) => void;
  loiTaiCho: string;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe =
    dangMo.kieu === "them"
      ? tieuDeThem(dangMo.nhanNhom)
      : dangMo.kieu === "sua"
        ? tieuDeSua(dangMo.muc.label)
        : tieuDeXoa(dangMo.muc.label);

  return (
    <form
      className="form-danh-muc"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4>{tieuDe}</h4>

      {dangMo.kieu === "them" && (
        <div className="o-nhap">
          <label htmlFor="o-ma-muc">{O_MA}</label>
          <input
            id="o-ma-muc"
            name="ma"
            value={ban.ma}
            onChange={(e) => datBan({ ...ban, ma: e.target.value })}
            aria-describedby="giai-thich-ma"
          />
          <p className="ghi-chu" id="giai-thich-ma">
            {GIAI_THICH_O_MA}
          </p>
        </div>
      )}

      {dangMo.kieu !== "xoa" && (
        <>
          <div className="o-nhap">
            <label htmlFor="o-nhan-muc">{O_NHAN}</label>
            <input
              id="o-nhan-muc"
              name="nhan"
              value={ban.nhan}
              onChange={(e) => datBan({ ...ban, nhan: e.target.value })}
              aria-describedby="giai-thich-nhan"
            />
            <p className="ghi-chu" id="giai-thich-nhan">
              {GIAI_THICH_O_NHAN}
            </p>
          </div>

          <div className="o-nhap">
            <label htmlFor="o-thu-tu-muc">{O_THU_TU}</label>
            {/* `inputMode="numeric"` chứ không `type="number"`: ô số của trình duyệt có nút tăng
                giảm bé xíu và cuộn chuột đổi giá trị mà người dùng không biết. */}
            <input
              id="o-thu-tu-muc"
              name="thuTu"
              inputMode="numeric"
              value={ban.thuTu}
              onChange={(e) => datBan({ ...ban, thuTu: e.target.value })}
            />
          </div>

          <div className="o-nhap o-chon">
            <input
              id="o-mac-dinh-muc"
              name="macDinh"
              type="checkbox"
              checked={ban.macDinh}
              onChange={(e) => datBan({ ...ban, macDinh: e.target.checked })}
            />
            <label htmlFor="o-mac-dinh-muc">{O_MAC_DINH}</label>
          </div>
        </>
      )}

      {dangMo.kieu === "xoa" && (
        <>
          <p className="canh-bao-pham-vi">{CANH_BAO_XOA}</p>
          <div className="o-nhap">
            <label htmlFor="o-ly-do-xoa">{O_LY_DO_XOA}</label>
            {/* `required` là lớp nhắc của trình duyệt, KHÔNG phải phép kiểm: nó không bắt được
                một ô toàn dấu cách, và tắt được. Phép kiểm thật là `kiemLyDoXoa`, chạy trước khi
                gửi và có bài test riêng. */}
            <input
              id="o-ly-do-xoa"
              name="lyDo"
              required
              value={ban.lyDo}
              onChange={(e) => datBan({ ...ban, lyDo: e.target.value })}
              aria-invalid={loiTaiCho !== ""}
              aria-describedby={loiTaiCho !== "" ? "loi-ly-do-xoa" : undefined}
            />
          </div>
        </>
      )}

      {/* HAI VÙNG LỖI RIÊNG, KHÔNG GỘP. Lỗi tại chỗ nói "bạn còn thiếu một ô"; lỗi máy chủ nói
          "yêu cầu vừa rồi bị từ chối" — trong đó có cả câu 409 giải thích quy tắc ba tầng. Gộp
          chúng vào một dòng thì câu sau đè mất câu trước ở đúng lúc cần đọc cả hai. */}
      {loiTaiCho !== "" && (
        <p className="thong-bao-loi" id="loi-ly-do-xoa" role="alert">
          {loiTaiCho}
        </p>
      )}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {dangMo.kieu === "xoa" ? NUT_XAC_NHAN_XOA : NUT_LUU}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}
