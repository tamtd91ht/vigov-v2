"use client";

import { useEffect, useMemo, useState } from "react";

import type { KetQua } from "@/lib/api/goi";
import { layMaTranQuyen } from "@/lib/api/phan-quyen";
import type {
  identity_maTranQuyenRa,
  identity_nhomQuyenRa,
  identity_vaiTroCotRa,
} from "@/lib/api/schema.gen";

import { oDaCap, trangThaiMaTran, type BangDaCap } from "./ma-tran-quyen";
import { NHAN_LANH_DAO, nhanChuaCauHinh, nhanO, nhanSoNguoiGiuVaiTro } from "./nhan-ma-tran";

/**
 * Ma trận phân quyền — `docs/ui-ux/14-cau-hinh.md §4`, tab "Phân quyền".
 * **Hàng = quyền** (gom theo nhóm) · **cột = vai trò**.
 *
 * ═════════════════════════════════════════════════════════════════════════════════════════
 * MÀN HÌNH NÀY CHỈ ĐỌC. KHÔNG CÓ NÚT `LƯU`, VÀ KHÔNG CÓ Ô TÍCH BẤM ĐƯỢC.
 *
 * ĐÂY LÀ CHỖ NGƯỜI SAU SẼ ĐỊNH THÊM NÚT `Lưu` — đặc tả vẽ nó ở đầu mỗi cột vai trò (§4), và
 * §12.5 còn nói rõ "lưu theo từng cột, không lưu toàn ma trận". Lý do nó chưa có nằm ở đây, chứ
 * không nằm trong một thông điệp commit:
 *
 *   1. HỢP ĐỒNG KHÔNG CÓ TUYẾN GHI NÀO. `GET /api/v1/role-permissions` là tuyến duy nhất đứng
 *      sau màn hình này (`kb/20-contracts/openapi.json`).
 *   2. Tuyến ghi ấy nằm trên HAI câu khách chưa chốt (`kb/00-foundation/open-questions.json`):
 *      #13 — có chặn thao tác làm xã mất người quản trị CUỐI CÙNG không. Gỡ `admin.user` khỏi
 *           vai trò cuối cùng còn giữ nó là đúng một lần bấm trên chính bảng này, và sau đó
 *           không ai trong xã mở lại được; nhà cung cấp cũng không được phép chạm vào (ADR 0003).
 *           Đây là BẾ TẮC THỦ TỤC, không phải lỗi có đường vá nóng.
 *      #14 — người giữ `admin.user` có được thao tác lên CHÍNH MÌNH không.
 *   3. Vì vậy ô ở đây là DẤU HIỆU, không phải `<input type="checkbox">` và không phải nút. Một ô
 *      tích bấm được mà không lưu được là một lời hứa suông: cán bộ bấm, thấy dấu tích đổi, đóng
 *      màn hình, và tin rằng quyền đã đổi. Một ô chỉ đọc không nói dối câu nào.
 *
 * Ngày cả #13 và #14 được chốt: tuyến ghi mọc ở `service-identity` trước (kèm nhật ký thao tác
 * trong cùng giao dịch — luật 6), rồi mới tới nút ở đây.
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
export function MaTranPhanQuyen() {
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

  return (
    <section className="tab-phan-quyen" aria-labelledby="tieu-de-phan-quyen">
      <h2 id="tieu-de-phan-quyen">Phân quyền</h2>

      {/* Nói ngay ở đầu màn hình rằng đây là bảng chỉ xem, trước khi cán bộ thử bấm vào một ô.
          Câu thứ hai nói cái gì mở khoá việc sửa — "chưa làm được" mà không nói vì sao thì lần
          sau vẫn có người hỏi lại. */}
      <p className="ghi-chu">
        Bảng chỉ để xem. Việc cấp hoặc thu hồi quyền chưa mở vì đơn vị chưa chốt hai quy định: có
        chặn thao tác làm đơn vị mất người quản trị cuối cùng hay không, và người giữ quyền phân
        quyền có được tự đổi quyền của chính mình hay không.
      </p>

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

      {trangThai.pha === "coDuLieu" && (
        <BangMaTran nhom={trangThai.nhom} vaiTro={trangThai.vaiTro} daCap={trangThai.daCap} />
      )}
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
function BangMaTran({
  nhom,
  vaiTro,
  daCap,
}: {
  nhom: readonly identity_nhomQuyenRa[];
  vaiTro: readonly identity_vaiTroCotRa[];
  daCap: BangDaCap;
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
          Ma trận phân quyền của đơn vị: mỗi hàng là một quyền, mỗi cột là một vai trò. Bảng chỉ để
          xem, không sửa được trên màn hình này.
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
                {/*
                  HAI SỐ ĐẾM, và số sau là tập con của số trước — câu chữ và lý do ở `nhan-ma-tran.ts`.

                  ĐÂY CŨNG LÀ CHỖ ĐẶC TẢ ĐẶT NÚT `Lưu` (§4, §12.5). Nó không có ở đây; lý do đầy đủ
                  nằm ở đầu tệp này, và nó không phải chuyện quên.
                */}
                <span className="dong-phu">{nhanSoNguoiGiuVaiTro(vt)}</span>
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
                {vaiTro.map((vt) => (
                  <ODaCap key={vt.id} daCap={oDaCap(daCap, vt.id, q.code)} />
                ))}
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}

/**
 * Một ô của ma trận. **Không phải điều khiển** — không `<input>`, không `<button>`, không sự kiện
 * bấm, và CSS cố ý không cho nó con trỏ chuột hay hiệu ứng rê chuột nào: ô này không mời bấm.
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
