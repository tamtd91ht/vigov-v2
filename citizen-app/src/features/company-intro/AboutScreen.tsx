import {
  BRAND_STATEMENTS,
  CERTIFICATES,
  COMPANY,
  MOC_LICH_SU,
  OFFICES,
} from "../../content/company-profile";
import {
  CompassGlyph,
  HandshakeGlyph,
  NetworkBackdrop,
  PinGlyph,
  ShieldGlyph,
  TargetGlyph,
} from "./icons";
import { MOC_CHANG_DUONG } from "./dieu-huong";

/**
 * Screen 3 — philosophy, vision, mission and certificates.
 *
 * All four belong to ViHAT Group, which is also who publishes this app, so the screen no longer
 * carries a line explaining whose they are: the title names the entity and everything under it
 * is that entity's. A certificate attributed to the wrong entity is a claim about an audit that
 * entity never passed — which is why the attribution line was removed, not reworded, when
 * ownership moved (2026-09-21).
 *
 * WHY THE GLYPHS ARE POSITIONAL AND NOT PART OF THE CONTENT FILE:
 *
 *   The three statements are published in a fixed order (company-profile.test.ts pins it), so
 *   the glyph is chosen by position. It is decoration: nothing on this screen is readable only
 *   through a picture, and the statement titles say in words what each card is.
 *
 * The certificates stay LAST on this screen. The Zalo accreditation is published without a
 * scope, and this app prints none — screens.test.tsx checks that its list item carries the name
 * and nothing else, which only holds while nothing further is appended after it.
 */
const STATEMENT_GLYPHS = [HandshakeGlyph, CompassGlyph, TargetGlyph];

export function AboutScreen() {
  return (
    <>
      <section className="banner">
        <NetworkBackdrop className="banner__backdrop" />
        <div className="banner__content">
          <h1 className="banner__title">Về {COMPANY.name}</h1>
        </div>
      </section>

      {BRAND_STATEMENTS.map((statement, index) => {
        const Glyph = STATEMENT_GLYPHS[index] ?? HandshakeGlyph;
        return (
          <section className="card card--statement" key={statement.title}>
            <span className="tile tile--soft" aria-hidden="true">
              <Glyph className="tile__glyph" />
            </span>
            <h2 className="card__title">{statement.title}</h2>
            <p className="card__body">{statement.body}</p>
          </section>
        );
      })}

      {/* DẢI LỊCH SỬ.
          ⚠ MỐC SỚM NHẤT LÀ 2013, KHÔNG PHẢI 2012 — bản mẫu vẽ một dải bắt đầu từ 2012, mà nguồn
          ghi ngày thành lập là 06/12/2013. Một dải lịch sử bắt đầu trước khi pháp nhân tồn tại là
          một khẳng định sai về một pháp nhân có thật, nên ở đây làm theo nguồn. Xem khối chú thích
          của `MOC_LICH_SU` trong company-profile.ts, kể cả mức tin cậy của nó.
          NĂM ĐỌC THÀNH CHỮ, KHÔNG PHẢI MỘT CHẤM TRÒN TRÊN MỘT ĐƯỜNG KẺ: một dải thời gian vẽ bằng
          hình thì trình đọc màn hình không đọc được thứ tự, và mắt kém thì không thấy chấm. */}
      {/* `id` LÀ MỘT MỎ NEO CÓ NGƯỜI DÙNG, không phải một thuộc tính thừa: mục "Chặng đường" trên
          màn chủ cuộn thẳng tới đây. Hằng nằm ở `screens.ts` để hai bên đọc cùng một chuỗi — gõ
          tay hai lần thì ngày một bên đổi, nút bên kia im lặng không đi đâu cả. */}
      <h2 className="section-title" id={MOC_CHANG_DUONG}>
        <span className="section-title__mark" aria-hidden="true">
          <CompassGlyph />
        </span>
        Chặng đường phát triển
      </h2>
      <ol className="lich-su">
        {MOC_LICH_SU.map((moc) => (
          <li className="moc" key={`${moc.nam}-${moc.viec}`}>
            <span className="moc__nam">{moc.nam}</span>
            <span className="moc__viec">{moc.viec}</span>
          </li>
        ))}
      </ol>

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <ShieldGlyph />
        </span>
        Chứng chỉ và chứng nhận
      </h2>
      <ul className="cert-grid">
        {CERTIFICATES.map((certificate) => (
          <li className="cert" key={certificate.name}>
            <ShieldGlyph className="cert__glyph" />
            <strong className="cert__name">{certificate.name}</strong>
            {certificate.scope ? <span className="cert__scope">{certificate.scope}</span> : null}
          </li>
        ))}
      </ul>

      {/* BA VĂN PHÒNG, ĐỌC ĐƯỢC, KHÔNG BẤM ĐƯỢC — và đó là chủ đích.
          Nút "Chỉ đường" sống trên màn Liên hệ, nơi nó đứng cạnh tính năng tìm văn phòng và nơi
          người đang muốn đi tới sẽ tìm. Đặt thêm một nút chỉ đường ở đây là hai chỗ làm một việc,
          và hai chỗ thì một trong hai sẽ lệch. Ở đây địa chỉ là một THÔNG TIN VỀ PHÁP NHÂN — thứ
          người duyệt hồ sơ đọc — nên nó là chữ đọc được và chọn sao chép được. */}
      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <PinGlyph />
        </span>
        Văn phòng
      </h2>
      <ul className="office-list">
        {OFFICES.map((office) => (
          <li className="office" key={office.name}>
            <PinGlyph className="office__glyph" />
            <span className="office__text">
              <strong className="office__name">{office.name}</strong>
              <span className="office__address">{office.address}</span>
            </span>
          </li>
        ))}
      </ul>
    </>
  );
}
