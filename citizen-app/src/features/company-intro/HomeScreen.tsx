import {
  CAU_SAN_PHAM,
  COMPANY,
  COMPANY_STATS,
  NHAN_SO_NAM,
  nhanLoaiHinhVaNam,
  SLOGAN_HERO,
  soNamHoatDong,
  SOLUTIONS,
} from "../../content/company-profile";

import { DaiChienDich } from "./DaiChienDich";
import { MOC_CHANG_DUONG, type ThamSoMan } from "./dieu-huong";
import { GridGlyph, HandshakeGlyph } from "./icons";
import { KhoiTin } from "./KhoiTin";
import { MenuNhanh } from "./MenuNhanh";

/**
 * Screen 1 — who publishes this app, in one screen, above the fold.
 *
 * The figures no longer carry an owner note: they are the publisher's own since ownership moved
 * to the group (2026-09-21). Nothing was reworded to keep a note alive — the note existed to
 * stop a subsidiary appearing to claim its parent's scale, and there is no subsidiary here now.
 *
 * ⚠ BA CÂU TRONG HERO, HAI NGUỒN KHÁC NHAU, VÀ APP KHÔNG TRỘN CHÚNG LẠI:
 *
 *   `SLOGAN_HERO.cau` và `CAU_SAN_PHAM.cau` là chữ của **bản mẫu PM** (chủ dự án duyệt dùng);
 *   `COMPANY.positioning` là câu hero **đã công bố trên vihatgroup.com**. Câu thứ ba GIỮ NGUYÊN
 *   CHỖ của nó ở đây — nó không bị hai câu kia thay. Nguồn của từng câu ghi tại chỗ khai báo
 *   trong `company-profile.ts`, không ghi lên màn hình: người dùng không cần biết, người sửa mã
 *   thì phải biết.
 *
 * ⚠ `COMPANY.description` KHÔNG CÒN TRÊN MÀN NÀY — 21/09/2026 (khuya), theo bản mẫu.
 *
 *   Bản mẫu bày hero là: huy hiệu · tên · slogan lớn · câu sản phẩm · chỉ số. Đoạn mô tả bốn dòng
 *   không có chỗ trong đó, và nhồi nó vào làm thẻ menu nhanh — thứ phải ĐÈ LÊN đáy hero — bị đẩy
 *   xuống dưới màn hình đầu tiên.
 *
 *   NÓ KHÔNG BỊ XOÁ KHỎI APP: đoạn ấy vẫn in đủ trên màn Giải pháp (`SolutionsScreen`), nơi nó
 *   được in nguyên văn như một trích dẫn đã công bố. Một câu đã công bố bị gỡ khỏi MỌI màn là một
 *   lượt sửa văn bản của pháp nhân; gỡ khỏi MỘT màn thì không.
 *
 * ⚠ DÒNG "Tập đoàn công nghệ · từ 2013" LẤY NĂM TỪ `NGAY_THANH_LAP`, không gõ vào JSX. Bản mẫu
 * giao hàng ghi 2012 — sai đúng ở chỗ này — và một con số gõ thẳng vào một component thì mọi phép
 * kiểm đọc tệp nội dung đều không nhìn thấy. Xem `nhanLoaiHinhVaNam`.
 *
 * Everything decorative here is drawn, not photographed: no image file enters the uploaded
 * package (see icons.tsx).
 */
export function HomeScreen({ onDi }: ThamSoMan) {
  /**
   * Vỏ app (`App.tsx`) LUÔN truyền `onDi`. Giá trị lui này chỉ phục vụ những chỗ dựng màn hình
   * một mình — `renderToStaticMarkup` trong bộ test — và nó không bao giờ chạy trong app thật.
   * Có nó thì kiểu của sổ màn hình giữ được `onDi` là tuỳ chọn, thay vì bắt bốn màn còn lại khai
   * một tham số chúng không dùng.
   */
  const di = onDi ?? (() => {});

  return (
    <>
      {/*
        HERO TRÀN NGANG, NỀN NAVY PHẲNG (12/10/2026), CHỮ TRẮNG, CHỈ SỐ NẰM NGAY TRONG NÓ — bản mẫu của PM.

        ⚠ HUY HIỆU "Vi" CẮT RA TỪ CHÍNH TÊN PHÁP NHÂN, KHÔNG GÕ VÀO. Gõ "Vi" vào JSX là dựng chỗ
        thứ hai giữ tên công ty, và ngày cái tên đổi thì huy hiệu là chỗ không ai nhớ sửa. Nó
        `aria-hidden` vì tên đầy đủ đứng ngay cạnh: đọc lên là đọc hai lần.

        ⚠ `<h1>` VẪN LÀ TÊN PHÁP NHÂN, DÙ BẢN MẪU VẼ SLOGAN TO NHẤT. Cỡ chữ là việc của CSS; cấu
        trúc tiêu đề là việc của trình đọc màn hình, và chủ đề của màn này là pháp nhân phát hành
        ứng dụng, không phải một câu quảng cáo. Hai thứ ấy được tách ra chứ không đánh đổi.
      */}
      <section className="hero hien-len">
        <div className="hero__content">
          <div className="hero__dau">
            <span className="hero__huy-hieu" aria-hidden="true">
              {COMPANY.name.slice(0, 2)}
            </span>
            {/* `<div>`, không phải `<span>`: một `<h1>` bên trong nội dung nội tuyến là HTML
                không hợp lệ, và trình duyệt sửa lại bằng cách tách thẻ ra — làm vỡ đúng bố cục
                hàng ngang này. */}
            <div className="hero__ten">
              <h1 className="hero__title">{COMPANY.name}</h1>
              <p className="hero__kicker">{nhanLoaiHinhVaNam()}</p>
            </div>
          </div>

          <p className="hero__slogan">{SLOGAN_HERO.cau}</p>
          {/* CÂU THỨ HAI CỦA BẢN MẪU. Nguồn ghi tại chỗ khai báo, không ghi lên màn hình — xem
              `CAU_SAN_PHAM` trong company-profile.ts, và khối chú thích đầu tệp này. */}
          <p className="hero__san-pham">{CAU_SAN_PHAM.cau}</p>
          {COMPANY.positioning ? <p className="hero__lead">{COMPANY.positioning}</p> : null}

          {/*
            NĂM CHỈ SỐ TRONG HERO — bốn của bản mẫu, cộng con số TÍNH RA.

            Bản mẫu bày đúng bốn ô trên một hàng. Ô thứ năm — số năm hoạt động — nằm nguyên một
            hàng bên dưới, và nó ở lại vì nó là con số DUY NHẤT trong app suy ra từ `NGAY_THANH_LAP`
            thay vì chép tay: "12 năm" viết thẳng vào mã thì đúng hôm nay, sai từ 06/12/2026, và
            sai theo kiểu không có gì đỏ lên — app chỉ lặng lẽ khai ít tuổi hơn pháp nhân nó đứng
            tên. Bỏ nó đi cho khớp một bức ảnh là bỏ đúng cái chống trôi ấy.
          */}
          <ul className="stat-grid">
            {COMPANY_STATS.map((stat) => (
              <li className="stat" key={stat.label}>
                <span className="stat__value">{stat.value}</span>
                <span className="stat__label">{stat.label}</span>
              </li>
            ))}
            <li className="stat stat--wide">
              <span className="stat__value">{soNamHoatDong()} năm</span>
              <span className="stat__label">{NHAN_SO_NAM}</span>
            </li>
          </ul>
        </div>
      </section>

      {/* SÁU LỐI ĐI NGẮN, TRÊN MỘT THẺ TRẮNG ĐÈ LÊN ĐÁY HERO. Sáu chứ không tám — ba mục của bản
          mẫu không có đích nào để dẫn tới, và mục thứ sáu là hotline. Xem `MenuNhanh`.

          DẢI CHIẾN DỊCH ĐI VÀO THẺ ẤY, KHÔNG ĐỨNG GIỮA HERO VÀ THẺ: lề âm của thẻ và đệm đáy của
          hero là một cặp đã đo, và một khối chen vào giữa làm thẻ trắng trùm mất chính dải ấy mà
          không phép kiểm nào thấy. Lý do đầy đủ ở tham số `dau` của `MenuNhanh`. */}
      <MenuNhanh onDi={di} dau={<DaiChienDich />} />

      {/*
        ĐƯỜNG VÀO MÀN "TƯ VẤN VÀ BÁO GIÁ" — MỘT NÚT, DƯỚI THẺ MENU, KHÔNG PHẢI MỘT Ô THỨ BẢY.

        Sáu ô là một phép đo ở 320px (lưới 3x2, xem `MenuNhanh`); ô thứ bảy để lại một hàng ba có
        một ô lẻ. Và nút này KHÔNG đứng giữa hero và thẻ menu: lề âm của thẻ ăn khớp với đệm đáy
        hero, và một khối chen vào giữa làm thẻ trắng trùm lên nó mà không phép kiểm nào thấy.

        Nó nói ra TRẠNG THÁI CỦA VIỆC, không chỉ tên việc: "Gửi yêu cầu, chúng tôi gọi lại" cho
        biết sau cú bấm là gì. Ô menu quay số mang tên "Hotline" (14/10/2026) chứ không còn là "Tư
        vấn": hai nút cùng một chữ mà một quay số, một mở biểu mẫu là hai thứ người dùng phải đoán.
      */}
      <button type="button" className="action action--primary action--mo" onClick={() => di({ man: "tu-van" })}>
        <span className="tile" aria-hidden="true">
          <HandshakeGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gửi yêu cầu, chúng tôi liên hệ lại</span>
          <span className="action__value">Tư vấn và báo giá</span>
        </span>
      </button>

      {/*
        GIẢI PHÁP NỔI BẬT — MỖI THẺ BẤM ĐƯỢC, MỞ TRANG CHI TIẾT CỦA ĐÚNG GIẢI PHÁP ẤY (chủ dự án
        14/10/2026; bản mẫu của PM vẽ chúng là thẻ bấm được). Người đọc thấy một giải pháp mình quan
        tâm thì bấm ngay vào nó, không phải sang tab Giải pháp rồi tìm lại.

        KHÔNG CÓ TRANG CHI TIẾT THỨ HAI: thẻ đi `{ man: "solutions", moc: <mã giải pháp> }`, và
        `SolutionsScreen` mở `ChiTietGiaiPhap` của mã ấy — cùng một màn, cùng một nút "Tất cả giải
        pháp" để quay về. Danh sách đọc từ cùng `SOLUTIONS`, nên hai nơi không lệch nhau được.
        Mỗi thẻ là một `<button>` thật: tên đọc lên là tên sản phẩm cộng câu giới thiệu, chiều cao
        tối thiểu do lớp `.gp-noi-bat__nut` đặt.
      */}
      {/* ĐẦU MỤC VÀ NÚT "Xem tất cả" TRÊN CÙNG MỘT HÀNG (bản mẫu). Nút vẫn là một `<button>` thật
          với chiều cao `--tap-min`, không phải một dòng chữ xanh bấm được: một đích chạm 20px cao
          nằm sát mép phải màn hình là đích chạm trượt nhiều nhất trên cả màn này. */}
      <div className="hang-tieu-de">
        {/* `.section-title` GIỮ NGUYÊN lớp của nó — cùng một đầu mục như trên bốn màn kia. Chỉ có
            cái hàng bọc ngoài là mới; đổi lớp dùng chung để nhét thêm một nút vào là đổi đầu mục
            của cả app cho một chỗ. */}
        <h2 className="section-title">
          <span className="section-title__mark" aria-hidden="true">
            <GridGlyph />
          </span>
          Giải pháp nổi bật
        </h2>
        <button type="button" className="hang-tieu-de__them" onClick={() => di({ man: "solutions" })}>
          Xem tất cả
        </button>
      </div>
      <ul className="gp-noi-bat">
        {SOLUTIONS.map((giai_phap) => (
          <li className="gp-noi-bat__mot" key={giai_phap.id}>
            <button
              type="button"
              className="gp-noi-bat__nut"
              onClick={() => di({ man: "solutions", moc: giai_phap.id })}
            >
              <span className="gp-noi-bat__chu">
                {giai_phap.product ? (
                  <strong className="gp-noi-bat__ten">{giai_phap.product}</strong>
                ) : null}
                <span className="gp-noi-bat__cau">{giai_phap.headline}</span>
              </span>
              <span className="solution__mui" aria-hidden="true">
                ›
              </span>
            </button>
          </li>
        ))}
      </ul>

      <KhoiTin />

      {/* "Nhận ưu đãi qua SMS" stood here 07/10 → 12/10/2026; it moved into the "Kết nối với ViHAT" card on Liên hệ
          (owner, Zalo resubmission), so every ask of the phone number sits in one place with its purpose. */}

      {/* "Quản lý quyền" is no longer a block on this screen (owner 11/10/2026); it stays reachable from Liên hệ. */}

      {/* HAI ĐƯỜNG SANG MÀN VỀ ViHAT. Một dẫn tới đầu màn, một dẫn thẳng tới dải lịch sử — đó là
          hai việc khác nhau, nên là hai nút chứ không phải một. */}
      <div className="tn__doi-nut">
        <button
          type="button"
          className="tn-hanh-dong tn-hanh-dong--rong"
          onClick={() => di({ man: "about" })}
        >
          Về tập đoàn
        </button>
        <button
          type="button"
          className="tn-hanh-dong tn-hanh-dong--rong"
          onClick={() => di({ man: "about", moc: MOC_CHANG_DUONG })}
        >
          Chặng đường
        </button>
      </div>
    </>
  );
}
