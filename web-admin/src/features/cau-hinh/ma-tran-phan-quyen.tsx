"use client";

import { useEffect, useMemo, useState } from "react";

import type { KetQua } from "@/lib/api/goi";
import { layMaTranQuyen, luuPhanQuyenVaiTro } from "@/lib/api/phan-quyen";
import type {
  identity_maTranQuyenRa,
  identity_nhomQuyenRa,
  identity_vaiTroCotRa,
} from "@/lib/api/schema.gen";

import { oDaCap, trangThaiMaTran, type BangDaCap } from "./ma-tran-quyen";
import {
  CHU_THICH_BANG_SUA,
  CHU_THICH_BANG_XEM,
  GHI_CHU_CHI_XEM,
  HUONG_DAN_SUA,
  LY_DO_KHONG_TU_SUA,
  NHAN_LANH_DAO,
  NUT_DANG_LUU,
  NUT_HUY,
  NUT_LUU,
  nhanChuaCauHinh,
  nhanNutHuy,
  nhanNutLuu,
  nhanO,
  nhanOBatTat,
  nhanSoNguoiGiuVaiTro,
} from "./nhan-ma-tran";
import {
  banSuaMoi,
  batDauLuu,
  batTatO,
  bangHienThi,
  cotDaSua,
  guiCot,
  huyCot,
  ketThucLuu,
  type BanSua,
} from "./sua-phan-quyen";

/**
 * Ma trận phân quyền — `docs/ui-ux/14-cau-hinh.md §4`, tab "Phân quyền".
 * **Hàng = quyền** (gom theo nhóm) · **cột = vai trò**.
 *
 * ═════════════════════════════════════════════════════════════════════════════════════════
 * LƯU THEO TỪNG CỘT, KHÔNG LƯU TOÀN MA TRẬN (§12.5) — `PUT /api/v1/roles/{id}/permissions`.
 *
 * Mỗi cột có trạng thái sửa riêng (`sua-phan-quyen.ts`): nút `Lưu` chỉ bật khi cột ấy đã khác bản
 * máy chủ, bấm thì gửi đúng tập của cột ấy, và các cột khác — đã sửa hay chưa — không đi kèm. 200
 * thay cột bằng tập máy chủ trả; lỗi giữ nguyên phần cán bộ đã tick và hiện nguyên câu máy chủ
 * viết cạnh cột. Không tự thử lại.
 *
 * Ô chỉ thành `<input type="checkbox">` khi tài khoản giữ `admin.role` (`choSua`). Đó là tiện dụng:
 * ba ràng buộc thật — #13 người giữ cuối cùng, #14 không tự sửa vai trò mình, không cấp/gỡ khoá
 * mình không giữ — nằm ở máy chủ, và màn hình không dựng bản sao nào của chúng ngoài một chỗ: cột
 * của chính vai trò người đang đăng nhập được khoá sẵn kèm lý do (#14), vì phiên có phát mã vai trò
 * (`quyen-tab.ts`, `maVaiTroCuaToi`). Khoá ấy sai thì máy chủ vẫn trả 403.
 * ═════════════════════════════════════════════════════════════════════════════════════════
 *
 * KHÔNG HIỆN CON SỐ TỔNG NÀO — không "33 quyền", không "10 nhóm", không "8 vai trò". Danh mục
 * quyền nằm trong CSDL và khách còn có thể chốt thêm khoá; một con số cứng trên màn hình sẽ sai
 * vào đúng ngày ấy, lặng lẽ, vì không ai kiểm lại nó. (Tiêu đề đặc tả §4.2 ghi "43 quyền, 11
 * nhóm" trong khi bảng liệt kê ngay dưới nó có 33 khoá / 10 nhóm và CSDL khớp bảng — đúng kiểu
 * hỏng mà một con số chép tay gây ra.)
 *
 * KHÔNG DỮ LIỆU CÁ NHÂN (luật 3): đầu cột chỉ có hai số đếm, và màn hình này không gọi thêm tuyến
 * nào để đổi số đếm thành danh sách tên.
 */
export function MaTranPhanQuyen({
  choSua,
  maVaiTroCuaToi,
}: {
  /** Tài khoản giữ `admin.role` — ô thành ô bấm. Tiện dụng, máy chủ vẫn kiểm. */
  choSua: boolean;
  /** `role.code` của phiên, để khoá sẵn cột của chính mình (#14). `null` = không biết, không khoá. */
  maVaiTroCuaToi: string | null;
}) {
  /** `null` là CHƯA ĐỌC XONG, khác hẳn "đọc xong và hỏng". Ba pha, không hai (`goi.ts`). */
  const [phanHoi, datPhanHoi] = useState<KetQua<identity_maTranQuyenRa> | null>(null);

  /**
   * MỘT LỜI GỌI, MỘT LẦN, CHO CẢ MÀN HÌNH — `[]` ở cuối effect là phần quan trọng nhất của khối
   * này. Hàng, cột và ô đã cấp về trong cùng một phản hồi vì ma trận chỉ đúng khi ba thứ ấy được
   * đọc ở cùng một thời điểm (`lib/api/phan-quyen.ts`).
   *
   * `bo` chặn một phản hồi đến muộn ghi vào một component đã rời màn hình.
   *
   * KHÔNG CÓ BỘ ĐỆM NÀO SỐNG QUA LẦN MỞ MÀN HÌNH: ma trận là của MỘT xã. Một biến ở mức module
   * giữ nó lại, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng của một lần bảng
   * phân quyền xã này hiện trên màn hình xã khác (luật 1, cấm #1).
   */
  useEffect(() => {
    let bo = false;
    layMaTranQuyen().then((kq) => {
      if (!bo) datPhanHoi(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  const trangThai = useMemo(() => trangThaiMaTran(phanHoi), [phanHoi]);

  /**
   * Bản sửa, dựng lại MỖI LẦN có phản hồi đọc mới. `null` khi chưa có ma trận để sửa. Cặp
   * `[daCapGoc, banSua]` dựng theo khuôn "điều chỉnh state khi prop đổi" của React: so với bảng gốc
   * của lần trước ngay trong lúc vẽ, không cần một effect chạy sau.
   */
  const daCapGoc = trangThai.pha === "coDuLieu" ? trangThai.daCap : null;
  const [banSua, datBanSua] = useState<BanSua | null>(null);
  const [gocDaDung, datGocDaDung] = useState<BangDaCap | null>(null);
  if (daCapGoc !== gocDaDung) {
    datGocDaDung(daCapGoc);
    datBanSua(daCapGoc === null ? null : banSuaMoi(daCapGoc));
  }

  async function luuCot(vaiTroId: string) {
    const b = banSua;
    if (b === null || !cotDaSua(b, vaiTroId) || b.dangLuu.has(vaiTroId)) return;
    datBanSua((x) => (x === null ? x : batDauLuu(x, vaiTroId)));
    // Thân dựng từ bản sửa TẠI LÚC BẤM, và chỉ cho cột này — `guiCot` gọi mạng đúng một lần.
    const kq = await guiCot(b, vaiTroId, luuPhanQuyenVaiTro);
    datBanSua((x) => (x === null ? x : ketThucLuu(x, vaiTroId, kq)));
  }

  return (
    <section className="tab-phan-quyen" aria-labelledby="tieu-de-phan-quyen">
      <h2 id="tieu-de-phan-quyen">Phân quyền</h2>

      {/* Câu hướng dẫn của đặc tả §4 khi sửa được; câu chỉ-xem khi không. */}
      <p className="ghi-chu">{choSua ? HUONG_DAN_SUA : GHI_CHU_CHI_XEM}</p>

      {trangThai.pha === "dangDoc" && <p role="status">Đang tải ma trận phân quyền…</p>}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải, không rẽ nhánh theo `code`, không
          hiện `trace_id` (`lib/api/goi.ts`). 403 ở đây là ca thật: quyền `admin.role` có thể vừa
          bị gỡ giữa lúc màn hình đang mở. */}
      {trangThai.pha === "khongDocDuoc" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI — và không bao giờ là một bảng trống không có
          lời giải thích nào. Xem `nhan-ma-tran.ts`. */}
      {trangThai.pha === "chuaCauHinh" && (
        <p className="trang-thai-rong">{nhanChuaCauHinh(trangThai.thieu)}</p>
      )}

      {trangThai.pha === "coDuLieu" &&
        (choSua && banSua !== null ? (
          <BangMaTran
            nhom={trangThai.nhom}
            vaiTro={trangThai.vaiTro}
            daCap={bangHienThi(banSua)}
            chinhSua={{
              banSua,
              maVaiTroCuaToi,
              batTat: (vaiTroId, khoa) =>
                datBanSua((x) => (x === null ? x : batTatO(x, vaiTroId, khoa))),
              luu: (vaiTroId) => void luuCot(vaiTroId),
              huy: (vaiTroId) => datBanSua((x) => (x === null ? x : huyCot(x, vaiTroId))),
            }}
          />
        ) : (
          <BangMaTran nhom={trangThai.nhom} vaiTro={trangThai.vaiTro} daCap={trangThai.daCap} />
        ))}
    </section>
  );
}

/**
 * Bảng ma trận.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỀ RỘNG NHỎ NHẤT ĐƯỢC HỖ TRỢ LÀ 320px, và ở đó một bảng 8 cột vai trò không vừa. Ba điều giữ
 * cho nó vẫn đọc được, cả ba nằm trong CSS đi kèm (`globals.css`, khối `.bang-phan-quyen`):
 *
 *   1. Cột đầu (tên quyền) GHIM TRÁI khi cuộn ngang — không có nó thì cuộn sang cột thứ tư là
 *      không còn biết mình đang ở hàng nào, và một dấu tích đọc nhầm hàng là đọc sai quyền.
 *   2. Hàng đầu (tên vai trò) GHIM TRÊN khi cuộn dọc — 33 hàng thì đầu cột trôi khỏi màn hình
 *      ngay ở nhóm thứ hai. Ghim được là vì vùng cuộn có TRẦN CHIỀU CAO; không có trần thì
 *      `sticky` không có gì để ghim vào (xem `.bang-cuon-ma-tran`).
 *   3. Mỗi nhóm quyền có một DẢI TIÊU ĐỀ riêng chạy ngang bảng, nên hàng nào cũng biết mình
 *      thuộc nhóm nào mà không phải cuộn ngược lên.
 *
 * Bảng CUỘN NGANG chứ không đổi thành thẻ: đổi `display` của `table`/`tr`/`td` làm mất ngữ nghĩa
 * bảng với trình đọc màn hình, mà đây đúng là dữ liệu dạng bảng — và là loại dữ liệu bảng cần
 * quan hệ hàng–cột nhất.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
// EXPORTED FOR ONE REASON, and it is worth the widened surface: this is the component that turns
// the server's answer into the rows a person reads, and until 2026-09-22 NOTHING rendered it in a
// test. `ma-tran-quyen.test.ts` proves the module shapes the data correctly; it cannot see whether
// this JSX uses that data or a list typed by hand. Replacing `n.permissions.map` with a hardcoded
// array stayed green — measured, not assumed.
//
// The parent reads the API inside `useEffect`, which `renderToStaticMarkup` never runs, so the
// property can only be pinned by rendering this half directly with data the test controls.
/** Phần sửa của bảng. Vắng mặt = bảng chỉ xem, không một ô bấm nào. */
export type ChinhSuaMaTran = {
  banSua: BanSua;
  maVaiTroCuaToi: string | null;
  batTat: (vaiTroId: string, khoa: string) => void;
  luu: (vaiTroId: string) => void;
  huy: (vaiTroId: string) => void;
};

export function BangMaTran({
  nhom,
  vaiTro,
  daCap,
  chinhSua,
}: {
  nhom: readonly identity_nhomQuyenRa[];
  vaiTro: readonly identity_vaiTroCotRa[];
  /** Thứ bảng HIỆN RA ở từng ô — ở chế độ sửa, bên gọi truyền `bangHienThi(banSua)`. */
  daCap: BangDaCap;
  chinhSua?: ChinhSuaMaTran;
}) {
  return (
    // `role="region"` + `tabIndex` để vùng cuộn tới được bằng bàn phím — vùng này cuộn CẢ HAI
    // chiều, nên không tới được bằng bàn phím là 33 hàng đọc được bằng chuột thôi.
    <div
      className="bang-cuon bang-cuon-ma-tran"
      role="region"
      aria-label="Ma trận phân quyền"
      tabIndex={0}
    >
      <table className="bang-phan-quyen">
        <caption className="an-thi-giac">
          {chinhSua === undefined ? CHU_THICH_BANG_XEM : CHU_THICH_BANG_SUA}
        </caption>
        <thead>
          <tr>
            <th scope="col" className="cot-quyen">
              Quyền
            </th>
            {vaiTro.map((vt) => (
              <th scope="col" key={vt.id} className="cot-vai-tro">
                <span className="ten-vai-tro">{vt.name}</span>
                {/*
                  `is_leader` CHỈ SINH RA NHÃN NÀY VÀ KHÔNG QUYẾT ĐỊNH GÌ KHÁC — không ẩn/hiện cột,
                  không đổi thứ tự, không đổi cách đọc một ô. Dùng nó để rẽ nhánh là mở một hệ
                  phân quyền THỨ HAI không đi qua `(tenant_id, vai trò, quyền)`, tức là đúng thứ
                  luật 5 cấm ở #2 và #3. Cấp bậc lãnh đạo là thông tin tổ chức, không phải thang
                  quyền (`service-identity/internal/domain/quyen.go`).
                */}
                {vt.is_leader && <span className="chip chip-lanh-dao">{NHAN_LANH_DAO}</span>}
                {/* HAI SỐ ĐẾM, và số sau là tập con của số trước — câu chữ và lý do ở `nhan-ma-tran.ts`. */}
                <span className="dong-phu">{nhanSoNguoiGiuVaiTro(vt)}</span>
                {chinhSua !== undefined && <DauCotSua vt={vt} chinhSua={chinhSua} />}
              </th>
            ))}
          </tr>
        </thead>

        {/*
          MỖI NHÓM MỘT `<tbody>`, và dải tiêu đề nhóm là một hàng trong chính nhóm ấy — nên quan hệ
          "quyền này thuộc nhóm kia" có thật trong cấu trúc bảng chứ không chỉ có trên hình.

          `key` theo tên nhóm: máy chủ gom nhóm bằng một bảng vị trí nên không phát ra hai dải cùng
          tên (`gomTheoNhom`). Tên nhóm hiện NGUYÊN VĂN thứ máy chủ trả — danh mục quyền là danh
          mục dùng chung, và dịch lại nhãn ở đây là dựng một nguồn sự thật thứ hai cho cùng một chữ.
        */}
        {nhom.map((n) => (
          <tbody key={n.name}>
            <tr className="dai-nhom">
              <th scope="rowgroup" colSpan={vaiTro.length + 1}>
                {/* Chữ nằm trong một `<span>` GHIM TRÁI, không nằm thẳng trong `<th>`: dải nhóm
                    rộng bằng cả bảng, nên khi cuộn ngang thì chính cái dải ấy đứng yên còn chữ
                    trôi ra khỏi khung nhìn. Ghim chữ thì cuộn tới cột vai trò cuối cùng vẫn biết
                    mình đang ở nhóm nào. */}
                <span className="nhan-nhom">{n.name}</span>
              </th>
            </tr>
            {n.permissions.map((q) => (
              <tr key={q.code}>
                <th scope="row" className="cot-quyen">
                  <span className="nhan-quyen">{q.label}</span>
                  {/* Khoá quyền hiện ngay dưới nhãn: nó là MỘT khoá phẳng `<nhóm>.<việc>` (luật 5,
                      bất biến 3b), là chuỗi mà máy chủ kiểm trên từng tuyến, và là thứ cán bộ đối
                      chiếu được khi hỏi "vì sao màn hình kia báo không có quyền". */}
                  <span className="dong-phu">{q.code}</span>
                </th>
                {vaiTro.map((vt) =>
                  chinhSua === undefined ? (
                    <ODaCap key={vt.id} daCap={oDaCap(daCap, vt.id, q.code)} />
                  ) : (
                    <OBatTat
                      key={vt.id}
                      daCap={oDaCap(daCap, vt.id, q.code)}
                      daDoi={
                        oDaCap(daCap, vt.id, q.code) !== oDaCap(chinhSua.banSua.goc, vt.id, q.code)
                      }
                      khoa={khoaCot(chinhSua, vt)}
                      nhan={nhanOBatTat(q.label, vt.name)}
                      batTat={() => chinhSua.batTat(vt.id, q.code)}
                    />
                  ),
                )}
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}

/** Cột không bấm được: đang lưu, hoặc là vai trò của chính người đang đăng nhập (#14). */
function khoaCot(c: ChinhSuaMaTran, vt: identity_vaiTroCotRa): boolean {
  return c.banSua.dangLuu.has(vt.id) || laCotCuaToi(c, vt);
}

function laCotCuaToi(c: ChinhSuaMaTran, vt: identity_vaiTroCotRa): boolean {
  return c.maVaiTroCuaToi !== null && c.maVaiTroCuaToi === vt.code;
}

/**
 * Phần sửa ở đầu một cột: `Lưu` · `Huỷ` · câu lỗi của máy chủ · lý do cột bị khoá.
 *
 * `Lưu` HIỆN Ở MỌI CỘT (đặc tả §4 vẽ nó ở mọi đầu cột) nhưng chỉ BẬT khi cột đã sửa. `Huỷ` chỉ hiện
 * khi có gì để huỷ. Câu lỗi mang `role="alert"` và nằm ngay dưới nút của CHÍNH cột ấy — một câu
 * lỗi chung ở đầu bảng không cho biết cột nào chưa lưu được.
 */
function DauCotSua({ vt, chinhSua }: { vt: identity_vaiTroCotRa; chinhSua: ChinhSuaMaTran }) {
  const { banSua } = chinhSua;
  const daSua = cotDaSua(banSua, vt.id);
  const dangLuu = banSua.dangLuu.has(vt.id);
  const cuaToi = laCotCuaToi(chinhSua, vt);
  const loi = banSua.loi.get(vt.id);

  return (
    <>
      <span className="nut-cot">
        <button
          type="button"
          className="nut-phu"
          disabled={!daSua || dangLuu || cuaToi}
          aria-label={nhanNutLuu(vt.name)}
          onClick={() => chinhSua.luu(vt.id)}
        >
          {dangLuu ? NUT_DANG_LUU : NUT_LUU}
        </button>
        {daSua && (
          <button
            type="button"
            className="nut-phu"
            disabled={dangLuu}
            aria-label={nhanNutHuy(vt.name)}
            onClick={() => chinhSua.huy(vt.id)}
          >
            {NUT_HUY}
          </button>
        )}
      </span>
      {cuaToi && <span className="dong-phu">{LY_DO_KHONG_TU_SUA}</span>}
      {loi !== undefined && (
        <span className="thong-bao-loi loi-cot" role="alert">
          {loi}
        </span>
      )}
    </>
  );
}

/**
 * Một ô của ma trận ở chế độ XEM. **Không phải điều khiển** — không `<input>`, không `<button>`,
 * không sự kiện bấm, và CSS cố ý không cho nó con trỏ chuột hay hiệu ứng rê chuột nào.
 *
 * HAI TÍN HIỆU, KHÔNG PHẢI MÀU: dấu `✓` và `–` khác nhau về HÌNH DẠNG, nên người không phân biệt
 * được màu vẫn đọc ra, và người dùng trình đọc màn hình nghe được câu đầy đủ ("Đã cấp" / "Chưa
 * cấp") thay vì một ký tự — `aria-hidden` trên dấu, câu chữ trong lớp chỉ-đọc-màn-hình.
 */
function ODaCap({ daCap }: { daCap: boolean }) {
  return (
    <td className={daCap ? "o-da-cap" : "o-chua-cap"}>
      <span aria-hidden="true">{daCap ? "✓" : "–"}</span>
      <span className="an-thi-giac">{nhanO(daCap)}</span>
    </td>
  );
}

/**
 * Một ô ở chế độ SỬA: một ô tích thật, tên đọc được "{nhãn quyền} — {tên vai trò}".
 *
 * Ô đã khác bản máy chủ mang thêm lớp `o-da-doi` — cán bộ thấy được mình đã đổi những ô nào trước
 * khi bấm Lưu, thay vì phải nhớ. Màu không phải tín hiệu duy nhất: trạng thái của ô là chính dấu
 * tích, lớp ấy chỉ là phần nhắc thêm.
 */
function OBatTat({
  daCap,
  daDoi,
  khoa,
  nhan,
  batTat,
}: {
  daCap: boolean;
  daDoi: boolean;
  khoa: boolean;
  nhan: string;
  batTat: () => void;
}) {
  return (
    <td className={`${daCap ? "o-da-cap" : "o-chua-cap"}${daDoi ? " o-da-doi" : ""}`}>
      <input
        type="checkbox"
        className="o-bat-tat"
        checked={daCap}
        disabled={khoa}
        aria-label={nhan}
        onChange={batTat}
      />
    </td>
  );
}
