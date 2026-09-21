/// <reference types="vite/client" />
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

/**
 * MÃ NGUỒN của màn chủ, đọc thô.
 *
 * Gần như mọi ca ở tệp này hỏi "người đọc thấy gì", và dựng ra là cách đúng để hỏi điều đó. Đúng
 * MỘT câu hỏi không trả lời được bằng cách dựng: "con số này được TÍNH RA hay được gõ vào?" — hai
 * đằng vẽ ra y hệt nhau. Câu ấy chỉ đọc mã nguồn mới trả lời được.
 */
import MA_NGUON_MAN_CHU from "./HomeScreen.tsx?raw";

import { App } from "../../App";
import { TabBar } from "../../components/TabBar";
import {
  BRAND_STATEMENTS,
  CAU_SAN_PHAM,
  CERTIFICATES,
  COMPANY,
  COMPANY_STATS,
  CONTACT,
  MEMBER_UNITS,
  MOC_LICH_SU,
  NHAN_SO_NAM,
  nhanLoaiHinhVaNam,
  OFFICES,
  SLOGAN_HERO,
  soNamHoatDong,
  SOLUTIONS,
  TECH_KEYWORDS,
} from "../../content/company-profile";
import { NGAY_CHUP_TIN, ngayDoc, TIN_VIHAT } from "../../content/tin-tuc";
import { ManDanhThiep } from "../tinh-nang/ManDanhThiep";
import { KHAI_BAO_LOI_GOI } from "../tinh-nang/zalo-api";
import { AboutScreen } from "./AboutScreen";
import { ChiTietGiaiPhap } from "./ChiTietGiaiPhap";
import { QuanLyQuyenScreen } from "./QuanLyQuyenScreen";
import { ContactScreen } from "./ContactScreen";
import { MOC_CHANG_DUONG, MOC_QUAN_LY_QUYEN } from "./dieu-huong";
import { HomeScreen } from "./HomeScreen";
import { MUC_MENU_NHANH } from "./MenuNhanh";
import { DEFAULT_SCREEN_ID, findScreen, SCREENS, TABS, tabDangSang } from "./screens";
import { SolutionsScreen } from "./SolutionsScreen";

/**
 * WHY PINNING THE CONTENT FILE IS NOT ENOUGH:
 *
 *   company-profile.test.ts proves the sourced facts are stored correctly. It cannot prove any
 *   of them reaches a screen. Deleting a figure, an office or a member unit from a screen leaves
 *   every one of those tests green while the app publishes less — or, the way this suite was
 *   first written, publishes a parent's figures with no owner named. Only rendering can check
 *   what a reader actually gets.
 *
 * ⚠ THE THREE ATTRIBUTION NOTES WENT AWAY ON 2026-09-21, WITH THEIR REASON. They said "these
 *   belong to the parent company", which was the truth exactly while a subsidiary published the
 *   app. The group publishes it now. The cases that pinned those notes became cases pinning
 *   that no such sentence is left on any screen and that no empty node was left where one was.
 *
 * WHY NOT A SNAPSHOT:
 *
 *   A whole-screen snapshot goes red on every wording change, and a suite that goes red for
 *   harmless reasons gets updated without being read. Each assertion below names one fact that
 *   must be visible, so a failure says what was lost.
 */

const render = (element: Parameters<typeof renderToStaticMarkup>[0]) =>
  renderToStaticMarkup(element);

/** Text content only — assertions are about what a citizen reads, not about markup. */
const ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&#x27;": "'",
  "&#39;": "'",
};

/**
 * Entities are decoded because the assertions are about what a CITIZEN reads. "Messaging &
 * Voice" leaves the renderer as `Messaging &amp; Voice`; comparing that against the published
 * sentence fails for a reason that has nothing to do with the sentence, and a test that is red
 * for a false reason is a test somebody relaxes.
 */
const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, (entity) => ENTITIES[entity] ?? entity)
    .replace(/\s+/g, " ");

/**
 * The `<li>` elements of a rendered screen, each cut at its OWN closing tag.
 *
 * ⚠ THE CUT IS THE WHOLE POINT, AND IT WAS MISSING UNTIL 2026-09-21. Without it every item ran
 * to the end of the document, so "this `<li>` contains nothing but the certificate name" really
 * asserted "nothing follows this `<li>` anywhere on the screen". That passed for as long as the
 * certificate list happened to be the last thing on the About screen, and went red the moment an
 * offices section was appended below it — for a reason that had nothing to do with certificates.
 * A check that is red for a false reason is a check somebody relaxes.
 */
const listItems = (markup: string) =>
  markup
    .split("<li")
    .slice(1)
    .map((item) => `<li${item.split("</li>")[0]!}</li>`);

/**
 * MỌI màn của sổ màn hình, kể cả tab tính năng.
 *
 * Tab "Danh thiếp" nằm ở đây chứ không được miễn: những gì mọi màn khác nợ người đọc — một
 * `<h1>`, glyph không bị đọc ra, không ô nhập, không dữ liệu cá nhân viết cứng — nó cũng nợ.
 */
const SCREEN_MARKUP = [
  { id: "home", markup: render(<HomeScreen />) },
  { id: "solutions", markup: render(<SolutionsScreen />) },
  { id: "danh-thiep", markup: render(<ManDanhThiep />) },
  { id: "about", markup: render(<AboutScreen />) },
  { id: "contact", markup: render(<ContactScreen />) },
  /**
   * HAI MÀN CON — và chúng ở đây chính vì chúng KHÔNG nằm trong sổ màn hình.
   *
   *   `ChiTietGiaiPhap` và `QuanLyQuyenScreen` chỉ vẽ ra sau một lần bấm, nên chúng không có mặt
   *   trong bản dựng tĩnh của màn cha. Nghĩa là nếu chỉ liệt kê năm màn của sổ, mọi bất biến bên
   *   dưới — đúng một `<h1>`, không `<svg>` nào bị đọc ra, không nút rỗng, không ô nhập, không dữ
   *   liệu cá nhân viết cứng — **không hề chạm tới hai màn mới**, và không có gì báo điều đó.
   *
   *   Một màn hình người dùng thật sự đọc mà không phép kiểm nào nhìn tới là đúng chỗ khuyết tật
   *   đi vào mà không ai thấy. Nên chúng được dựng ở đây bằng chính tham số mà màn cha truyền.
   */
  {
    id: "chi-tiet-giai-phap",
    markup: render(<ChiTietGiaiPhap giai_phap={SOLUTIONS[0]!} onQuayLai={() => {}} />),
  },
  {
    // Dòng giải pháp KHÔNG có tên sản phẩm và KHÔNG có câu phụ — nhánh vẽ khác hẳn nhánh trên,
    // và là nhánh dễ để lại một `<p>` rỗng nhất.
    id: "chi-tiet-giai-phap-khong-ten",
    markup: render(
      <ChiTietGiaiPhap giai_phap={SOLUTIONS.find((mot) => !mot.product)!} onQuayLai={() => {}} />,
    ),
  },
  { id: "quan-ly-quyen", markup: render(<QuanLyQuyenScreen onQuayLai={() => {}} />) },
] as const;

describe("what the content file stores reaches the screen", () => {
  it("names the publishing entity on the screen that opens the app", () => {
    const text = textOf(render(<HomeScreen />));
    expect(text).toContain(COMPANY.name);
  });

  it("prints every figure it stores — a dropped one is a changed claim", () => {
    const text = textOf(render(<HomeScreen />));
    for (const stat of COMPANY_STATS) {
      expect(text, `figure missing from the home screen: ${stat.label}`).toContain(stat.value);
      expect(text).toContain(stat.label);
    }
  });

  it("names the entity on the screen that carries its statements", () => {
    const text = textOf(render(<AboutScreen />));
    expect(text).toContain(COMPANY.name);
  });

  /**
   * NO SCREEN CLAIMS A PARENT OR A SUBSIDIARY ANY MORE, AND NONE MAY GET ONE BACK BY HAND.
   *
   *   Two of these sentences lived in the content file and one was typed straight into
   *   AboutScreen's JSX — which is exactly why this case renders instead of reading the content
   *   module: a disclaimer written into a component is invisible to `company-profile.test.ts`.
   */
  it("leaves no parent/subsidiary sentence on any screen", () => {
    for (const { id, markup } of SCREEN_MARKUP) {
      expect(textOf(markup), `${id} still claims a parent/subsidiary relation`).not.toMatch(
        /công ty mẹ|công ty con/,
      );
    }
  });

  /**
   * REMOVING A SENTENCE MUST NOT LEAVE THE ELEMENT THAT HELD IT.
   *
   *   An empty `<p class="banner__lead">` renders as padding a reader cannot explain, and a
   *   screen reader still walks into it. This case is the difference between "the note was
   *   deleted" and "the note was blanked".
   */
  it("leaves no empty paragraph or list item behind on any screen", () => {
    for (const { id, markup } of SCREEN_MARKUP) {
      expect(markup, `${id} renders an empty <p>`).not.toMatch(/<p[^>]*>\s*<\/p>/);
      expect(markup, `${id} renders an empty <li>`).not.toMatch(/<li[^>]*>\s*<\/li>/);
      expect(markup, `${id} renders an empty heading`).not.toMatch(/<h[1-6][^>]*>\s*<\/h[1-6]>/);
    }
  });

  it("lists all six member units", () => {
    const text = textOf(render(<SolutionsScreen />));
    for (const unit of MEMBER_UNITS) {
      expect(text, `member unit missing: ${unit.name}`).toContain(unit.name);
    }
  });

  it("prints every ecosystem line, with the product name the source gives it", () => {
    const text = textOf(render(<SolutionsScreen />));
    for (const solution of SOLUTIONS) {
      expect(text, `solution line missing: ${solution.id}`).toContain(solution.headline);
      if (solution.product) expect(text).toContain(solution.product);
      if (solution.note) expect(text).toContain(solution.note);
    }
  });

  it("puts the ecosystem heading ABOVE the first solution line", () => {
    // The heading is what tells a reader the list that follows is an ecosystem rather than a
    // list of this app's own features. It used to be a heading plus an owner note; the note went
    // with the parent/subsidiary split, the heading is what still has to arrive first.
    const text = textOf(render(<SolutionsScreen />));
    const headingAt = text.indexOf("Hệ sinh thái giải pháp");
    const firstLineAt = text.indexOf(SOLUTIONS[0]!.headline);
    expect(headingAt).toBeGreaterThanOrEqual(0);
    expect(firstLineAt).toBeGreaterThanOrEqual(0);
    expect(headingAt, "the solutions are listed before the heading that frames them").toBeLessThan(
      firstLineAt,
    );
  });

  it("prints every technology chip as WORDS", () => {
    // The chips exist to say "cloud, AI, CRM" at a glance. A glyph carries none of that to a
    // screen reader — the glyphs are aria-hidden precisely because the word next to them is the
    // content, and a chip reduced to its icon would be an empty pill.
    const text = textOf(render(<SolutionsScreen />));
    for (const keyword of TECH_KEYWORDS) {
      expect(text, `keyword chip missing: ${keyword.id}`).toContain(keyword.label);
    }
  });

  it("prints the three brand statements in full", () => {
    const text = textOf(render(<AboutScreen />));
    for (const statement of BRAND_STATEMENTS) {
      expect(text, `statement missing: ${statement.title}`).toContain(statement.title);
      expect(text, `statement body truncated: ${statement.title}`).toContain(statement.body);
    }
  });

  it("prints each certificate, and no scope for the one published without one", () => {
    const items = listItems(render(<AboutScreen />));
    for (const certificate of CERTIFICATES) {
      const item = items.find((candidate) => candidate.includes(certificate.name));
      expect(item, `certificate missing: ${certificate.name}`).toBeDefined();
    }
    const zalo = items.find((item) => item.includes("Chứng nhận đại lý chính thức của Zalo"));
    // Anything after the name inside that list item would be an invented scope.
    expect(textOf(zalo ?? "").trim()).toBe("Chứng nhận đại lý chính thức của Zalo");
  });

  it("shows every office address", () => {
    const text = textOf(render(<ContactScreen />));
    for (const office of OFFICES) {
      expect(text, `office missing: ${office.name}`).toContain(office.address);
    }
  });

  it("repeats the offices on the About screen, addresses in full", () => {
    // A reviewer reads the entity's addresses on the "about" screen, not next to a directions
    // button. Truncating one here would publish an incomplete address for a real legal entity.
    const text = textOf(render(<AboutScreen />));
    for (const office of OFFICES) {
      expect(text, `office missing from the About screen: ${office.name}`).toContain(office.address);
    }
  });

  it("prints every history milestone, year and event, in source order", () => {
    const text = textOf(render(<AboutScreen />));
    let at = -1;
    for (const moc of MOC_LICH_SU) {
      expect(text, `milestone event missing: ${moc.viec}`).toContain(moc.viec);
      const here = text.indexOf(moc.viec);
      expect(here, `the history strip is rendered out of order at ${moc.nam}`).toBeGreaterThan(at);
      at = here;
    }
    // The years are content, not decoration — a strip of sentences with no years is not a history.
    for (const moc of MOC_LICH_SU) {
      expect(text, `milestone year missing: ${moc.nam}`).toContain(moc.nam);
    }
  });

  it("renders no year before the founding year anywhere on the About screen", () => {
    // The prototype's 2012 again, this time at the render layer: a year typed straight into JSX is
    // invisible to `company-profile.test.ts`, which only ever reads the content module.
    expect(textOf(render(<AboutScreen />)), "a year before the entity existed is on screen").not.toContain(
      "2012",
    );
  });
});

describe("màn Quản lý quyền vẽ từ bảng khai, không từ một danh sách chép tay", () => {
  const markup = render(<QuanLyQuyenScreen onQuayLai={() => {}} />);
  const text = textOf(markup);

  /**
   * ⚠ ĐÂY LÀ NỬA CÒN LẠI CỦA RÀNG BUỘC 3c. `ranh-gioi-hai-nua.test.ts` chứng minh bảng khai khớp
   * với MÃ NGUỒN; ca dưới chứng minh MÀN HÌNH khớp với bảng khai. Thiếu vế nào thì vế kia cũng
   * không nói được gì: một bảng khai đúng mà màn hình vẽ từ chỗ khác là đúng thứ bảng khai sinh ra
   * để chặn.
   */
  it("in đủ mọi lời gọi có trong bảng, không thiếu một dòng nào", () => {
    expect(KHAI_BAO_LOI_GOI.length).toBeGreaterThanOrEqual(10);
    for (const quyen of KHAI_BAO_LOI_GOI) {
      expect(text, `màn quyền thiếu tên kỹ thuật: ${quyen.api}`).toContain(quyen.api);
      expect(text, `màn quyền thiếu câu "để làm gì" của ${quyen.api}`).toContain(quyen.de_lam_gi);
      expect(text, `màn quyền thiếu tính năng của ${quyen.api}`).toContain(quyen.tinh_nang);
    }
  });

  it("nói trạng thái bằng CHỮ, không bằng màu", () => {
    // README §Non-negotiables #6. Hai trạng thái "Zalo có hỏi bạn không" phân biệt bằng hai câu
    // khác nhau; một chấm xanh và một chấm xám thì người cần màn này nhất không phân biệt được.
    const co_hoi = KHAI_BAO_LOI_GOI.filter((q) => q.hoi_nguoi_dung).length;
    const khong_hoi = KHAI_BAO_LOI_GOI.length - co_hoi;
    expect(co_hoi, "không lời gọi nào được khai là có hỏi người dùng").toBeGreaterThan(0);
    expect(khong_hoi, "không lời gọi nào được khai là không hỏi").toBeGreaterThan(0);
    expect(text).toContain("Zalo sẽ hỏi bạn trước khi chức năng này chạy.");
    expect(text).toContain("Zalo không hỏi lại");
  });

  it("nói ra thứ rời khỏi máy, và nói ra cả khi không có gì rời đi", () => {
    // Một ô để trống trông giống "chưa ai điền". Câu "Không có gì rời khỏi máy bạn" là một KHẲNG
    // ĐỊNH, và nó là câu người đọc chính sách đang tìm.
    expect(text).toContain("Không có gì rời khỏi máy bạn.");
    for (const quyen of KHAI_BAO_LOI_GOI.filter((q) => q.roi_khoi_may !== "")) {
      expect(text, `màn quyền không nói ${quyen.api} gửi gì đi`).toContain(quyen.roi_khoi_may);
    }
  });

  it("nói thẳng rằng nó không bật/tắt được quyền nào", () => {
    // Quyền do Zalo giữ. Một màn tên "Quản lý quyền" mà không nói điều đó là một màn hứa một việc
    // nó không làm được, và người dùng đi tìm cái công tắc không có.
    expect(text).toContain("Ứng dụng này không bật hay tắt được quyền nào");
  });
});

describe("contact actions do what they say", () => {
  const markup = render(<ContactScreen />);

  it("dials the number it displays", () => {
    // The displayed label and the tel: target are two separate fields. Editing one and not the
    // other sends a citizen's call to a number the screen never showed — and nobody notices
    // until a call lands somewhere it should not.
    const href = /href="tel:([^"]+)"/.exec(markup)?.[1];
    expect(href, "no tel: link on the contact screen").toBeDefined();
    expect(href).toBe(CONTACT.hotlineLabel.replace(/\D/g, ""));
    expect(textOf(markup)).toContain(CONTACT.hotlineLabel);
  });

  it("mails the address it displays", () => {
    const href = /href="mailto:([^"]+)"/.exec(markup)?.[1];
    expect(href).toBe(CONTACT.email);
    expect(textOf(markup)).toContain(CONTACT.email);
  });

  /**
   * ⚠ CA NÀY ĐÃ ĐỔI HÌNH DẠNG NGÀY 21/09/2026 VÌ HÀNH VI ĐỔI, VÀ NÓ ĐỔI THEO HƯỚNG GẮT HƠN.
   *
   *   Trước: website là một neo `<a href="…" target="_blank" rel="noopener noreferrer">`, và ca
   *   này canh hai thuộc tính ấy. Nay nó là một NÚT đi qua `openWebview` của nền tảng — bên trong
   *   Zalo, một liên kết mở bằng thẻ `a` không có đường quay lại Mini App (xem `NutWebsite.tsx`).
   *
   *   Nên ca này không còn canh `rel` của một thẻ không còn tồn tại; nó canh điều MẠNH HƠN và
   *   đúng cho MỌI màn: **không màn nào còn mở một tab mới**, và địa chỉ vẫn phải đọc được trước
   *   khi bấm. Ba dòng dưới thay cho hai thuộc tính cũ, và chúng là thứ dây bẫy
   *   `content/dich-ra-ngoai.test.ts` canh ở tầng mã nguồn.
   */
  it("mở website bằng nút của nền tảng, và KHÔNG màn nào còn mở một tab mới", () => {
    for (const { id, markup: man } of SCREEN_MARKUP) {
      expect(man, `${id} còn một neo mở tab mới — trong Zalo nó không có đường quay lại`).not.toContain(
        'target="_blank"',
      );
      expect(man, `${id} có một neo href rỗng — nó nạp lại chính Mini App`).not.toMatch(/href=""/);
    }

    if (COMPANY.website === undefined) {
      expect(textOf(markup), "màn Liên hệ vẽ nút Website trong khi không có địa chỉ nào").not.toContain(
        "Website",
      );
      return;
    }
    // Địa chỉ hiện ra NGUYÊN VĂN: người dùng phải đọc được mình sắp đi đâu trước khi bấm.
    expect(textOf(markup)).toContain(COMPANY.website);
  });

  it("offers no form, no field and no upload — phase 1 collects nothing", () => {
    for (const { id, markup: screenMarkup } of SCREEN_MARKUP) {
      expect(screenMarkup, `${id} renders an input control`).not.toMatch(
        /<(form|input|textarea|select)\b/,
      );
    }
  });
});

/**
 * MÀN CHỦ SAU LƯỢT 21/09/2026 (tối) — menu nhanh, giải pháp nổi bật, khối tin, khối quyền tóm tắt.
 *
 * ⚠ ĐIỀU DUY NHẤT CÁC CA DƯỚI ĐÂY CANH, VÀ NÓ ĐÁNG GIÁ HƠN MỌI CA "CÓ VẼ RA KHÔNG": **không nút
 * nào dẫn tới hư không**. Một nút không dẫn đi đâu là thứ người duyệt của Zalo bấm vào đầu tiên,
 * và là thứ người lớn tuổi bấm rồi kết luận rằng app hỏng. Nên mỗi mốc điều hướng được đối chiếu
 * với chính màn nó dẫn tới: mốc phải là một `id` CÓ THẬT ở đó, hoặc phải mở được một màn con.
 */
describe("màn chủ dẫn đi đâu, và có dẫn tới nơi thật không", () => {
  const home = textOf(render(<HomeScreen />));

  it("sáu mục menu, mỗi mục có một đích có thật", () => {
    expect(MUC_MENU_NHANH).toHaveLength(6);
    for (const muc of MUC_MENU_NHANH) {
      expect(home, `menu thiếu nhãn: ${muc.nhan}`).toContain(muc.nhan);
      expect(home, `mục ${muc.ma} không nói nó dẫn đi đâu`).toContain(muc.phu);
      if (muc.kieu === "man") {
        expect(() => findScreen(muc.di.man), `mục ${muc.ma} trỏ vào một màn không có`).not.toThrow();
      } else {
        // "Tư vấn" là một cuộc gọi tới ĐÚNG số hotline đã công bố, không phải một biểu mẫu.
        expect(muc.so).toBe(CONTACT.hotlineDialable);
      }
    }
  });

  it("mỗi mốc là một `id` CÓ THẬT trên màn nó dẫn tới, hoặc mở được một màn con", () => {
    // ĐÂY LÀ CA ĐẮT NHẤT CỦA CẢ KHỐI. Một mốc gõ sai một ký tự làm nút vẫn đổi tab nhưng không
    // cuộn đi đâu — người bấm thấy đầu màn và không có gì đỏ lên, vì cả hai bên đều "đúng" khi
    // đọc riêng.
    for (const muc of MUC_MENU_NHANH) {
      if (muc.kieu !== "man" || muc.di.moc === undefined) continue;
      const Man = findScreen(muc.di.man).component;
      const ve_ra = render(<Man moc={muc.di.moc} />);

      if (muc.di.moc === MOC_QUAN_LY_QUYEN) {
        // Mốc dạng thứ hai: tên một màn CON, không phải một `id`. Nó phải MỞ được màn ấy.
        expect(textOf(ve_ra), "mốc quản lý quyền không mở ra màn quyền").toContain("Quản lý quyền");
        expect(textOf(ve_ra)).toContain(KHAI_BAO_LOI_GOI[0]!.api);
      } else {
        expect(ve_ra, `mốc "${muc.di.moc}" không có trên màn ${muc.di.man}`).toContain(
          `id="${muc.di.moc}"`,
        );
      }
    }
  });

  it("mốc 'Chặng đường' có thật trên màn Về ViHAT", () => {
    expect(render(<AboutScreen />)).toContain(`id="${MOC_CHANG_DUONG}"`);
  });

  it("màn Liên hệ KHÔNG mở màn quyền khi không ai yêu cầu", () => {
    // Mặt kia của ca trên. Không có nó thì một lỗi "luôn mở màn con" vẫn xanh, và tab Liên hệ mất
    // hotline, email, đăng nhập và chính sách quyền riêng tư — tức mất phần người duyệt đọc.
    expect(textOf(render(<ContactScreen />))).toContain(CONTACT.hotlineLabel);
    expect(textOf(render(<ContactScreen />))).not.toContain("Quay lại Liên hệ");
  });

  it("KHÔNG dựng ba mục của bản mẫu không có đích nào: Brochure · 24/7 · Đặt lịch", () => {
    // Chúng bị BỎ, không phải bị vẽ mờ đi. Một nút chết là thứ người duyệt bấm trước tiên.
    expect(home, "một nút Brochure đã được dựng mà không có gì sau nó").not.toMatch(/Brochure/i);
    // "24/7" là một CAM KẾT VỀ GIỜ PHỤC VỤ đứng tên một pháp nhân có thật, và không trang nào của
    // ViHAT mà kho này đã đọc công bố nó. Người phát hiện ra là người gọi lúc 11 giờ đêm.
    expect(home, "một lời hứa trực 24/7 chưa ai công bố đã lên màn chủ").not.toContain("24/7");
    // "Đặt lịch tư vấn" là một biểu mẫu thu dữ liệu, và chưa ai quyết dữ liệu ấy gửi đi đâu.
    expect(home, "mục đặt lịch tư vấn — một biểu mẫu — đã được dựng").not.toMatch(/Đặt lịch/i);
  });

  it("khối Quản lý quyền tóm tắt đọc số từ BẢNG KHAI, không gõ vào", () => {
    expect(home).toContain("Quản lý quyền");
    expect(home, "số chức năng trên màn chủ không khớp bảng khai").toContain(
      `${KHAI_BAO_LOI_GOI.length} chức năng`,
    );
  });

  it("hero mang HAI câu bản mẫu, câu định vị đã công bố, và năm THÀNH LẬP", () => {
    expect(home).toContain(SLOGAN_HERO.cau);
    expect(home, "câu sản phẩm của bản mẫu chưa lên hero").toContain(CAU_SAN_PHAM.cau);
    expect(home, "câu định vị đã công bố bị hai câu bản mẫu thay mất").toContain(
      COMPANY.positioning!,
    );
    expect(home).toContain(nhanLoaiHinhVaNam());
    expect(home, "năm 2012 của bản mẫu đã lên màn chủ").not.toContain("2012");
  });

  /**
   * HUY HIỆU HAI CHỮ CẮT RA TỪ TÊN PHÁP NHÂN — không phải một chuỗi "Vi" gõ vào JSX.
   *
   * Gõ vào thì đó là chỗ thứ hai giữ tên công ty, và ngày cái tên đổi thì huy hiệu là chỗ không
   * ai nhớ sửa: màn chủ mở ra với hai chữ cái của một pháp nhân đã đổi tên.
   *
   * ⚠ HAI CA, VÀ CA THỨ HAI MỚI LÀ CA THẬT. Dựng ra rồi so chuỗi thì một chữ "Vi" gõ cứng vẽ ra
   * y hệt một chữ "Vi" cắt ra — phép kiểm xanh, và nó xanh vì lý do sai. Chỉ có đọc MÃ NGUỒN mới
   * phân biệt được hai thứ ấy, nên ca dưới đọc chính tệp màn chủ.
   */
  it("huy hiệu trong hero hiện ra đúng hai chữ đầu của tên pháp nhân", () => {
    expect(render(<HomeScreen />)).toContain(`>${COMPANY.name.slice(0, 2)}<`);
  });

  it("và hai chữ ấy được CẮT RA lúc chạy, không gõ thẳng vào JSX", () => {
    const o = /className="hero__huy-hieu"[\s\S]{0,120}?>\s*\{([^}]+)\}/.exec(MA_NGUON_MAN_CHU);
    expect(o, "huy hiệu không còn là một biểu thức — hai chữ đã bị gõ thẳng vào JSX").not.toBeNull();
    expect(o![1], "huy hiệu không còn đọc từ `COMPANY.name`").toContain("COMPANY.name");
  });

  /**
   * NĂM CHỈ SỐ, VÀ Ô THỨ NĂM LÀ CON SỐ TÍNH RA — bản mẫu chỉ có bốn.
   *
   * Ca này canh đúng một chuyện: ai đó dọn hero cho khớp bức ảnh bằng cách bỏ ô thứ năm. Ô ấy là
   * con số DUY NHẤT suy ra từ `NGAY_THANH_LAP`; bỏ nó rồi viết "12 năm" lại vào một chỗ nào đó là
   * dựng lại đúng cái sai mà `soNamHoatDong` sinh ra để chặn.
   */
  it("hero giữ con số năm hoạt động TÍNH RA, không phải một chuỗi ghi cứng", () => {
    expect(home).toContain(NHAN_SO_NAM);
    expect(home).toContain(`${soNamHoatDong()} năm`);
  });

  /**
   * BỐN CÁI TÊN LỚP MÀ `accessibility.test.ts` ĐO HÌNH HỌC CỦA — và chúng phải thật sự được vẽ ra.
   *
   *   Phép đo chồng lớp ở tệp kia đọc CSS: lề âm của `.menu-the` so với đệm đáy của `.hero`, bề
   *   rộng cột của `.stat-grid` và `.menu-nhanh`. Nó không biết gì về JSX. Đổi tên lớp trong
   *   component — hoặc bỏ mất cái thẻ bọc — thì luật CSS vẫn nằm đó, mọi phép đo vẫn xanh, và màn
   *   chủ thì đã vỡ bố cục. Đó là một phép kiểm chết mà không có gì báo.
   */
  it("những lớp mà phép đo chồng lớp dựa vào đều có mặt trên màn chủ", () => {
    const ma = render(<HomeScreen />);
    for (const lop of ["hero", "menu-the", "menu-nhanh", "stat-grid", "hang-tieu-de"]) {
      // TÌM ĐÚNG MỘT TÊN LỚP TRỌN VẸN TRONG THUỘC TÍNH, không theo tiền tố và KHÔNG dùng `\b`:
      // `class="menu-the-moi"` chứa chuỗi `class="menu-the`, và `\b` cũng khớp ngay trước dấu
      // gạch nối — cả hai cách viết ấy đều xanh với đúng cái lỗi chúng phải bắt. Nên ranh giới ở
      // đây là khoảng trắng hoặc dấu nháy, tức là ranh giới thật của một tên lớp.
      expect(
        ma,
        `màn chủ không còn vẽ .${lop} — phép đo 320px của nó thành phép đo vô chủ`,
      ).toMatch(new RegExp(`class="(?:[^"]*\\s)?${lop}(?:\\s[^"]*)?"`));
    }
  });

  it("một nút 'Xem tất cả' dẫn sang màn Giải pháp, và chỉ một", () => {
    // Bản mẫu đặt nó cạnh đầu mục. Hai đường tới cùng một nơi là hai chỗ sẽ lệch nhau.
    expect(home).toContain("Xem tất cả");
    expect(
      (render(<HomeScreen />).match(/Xem tất cả/g) ?? []).length,
      "màn chủ có hơn một đường sang màn Giải pháp",
    ).toBe(1);
  });

  it("khối tin in đủ hai bài, kèm NGÀY ĐĂNG và lời khai đây là bản chụp", () => {
    // Ngày là nội dung, không phải trang trí: không có nó thì người đọc không biết mình đang xem
    // một thứ đã cũ — và cả khối là một BẢN CHỤP, không phải tin trực tiếp.
    for (const bai of TIN_VIHAT) {
      expect(home, `khối tin thiếu bài: ${bai.ma}`).toContain(bai.tieu_de);
      expect(home, `bài ${bai.ma} không hiện ngày đăng`).toContain(ngayDoc(bai.ngay));
      expect(home, `bài ${bai.ma} không có đoạn trích`).toContain(bai.trich);
    }
    expect(home, "không nói ra rằng đây là bản chụp ngày nào").toContain(NGAY_CHUP_TIN);
  });
});

describe("what every screen owes the reader", () => {
  it("gives each screen exactly one first-level heading", () => {
    for (const { id, markup } of SCREEN_MARKUP) {
      const headings = markup.match(/<h1\b/g) ?? [];
      expect(headings.length, `${id} has ${headings.length} <h1> — a screen reader needs one`).toBe(
        1,
      );
    }
  });

  it("hides every decorative glyph from the screen reader", () => {
    // A drawn icon that is announced is noise for the citizen who depends on the announcement
    // most, and an icon that is NOT decoration would mean information carried by a picture alone
    // (README §Non-negotiables #6). Either way the answer is the same: the words carry it.
    for (const { id, markup } of SCREEN_MARKUP) {
      const svgs = markup.match(/<svg[\s>][^>]*>/g) ?? [];
      expect(svgs.length, `${id} renders no glyph at all`).toBeGreaterThan(0);
      for (const svg of svgs) {
        expect(svg, `${id} renders an <svg> a screen reader will announce`).toContain(
          'aria-hidden="true"',
        );
      }
      // No <title>/<desc> either: both are announced, and neither is content this app owns.
      expect(markup, `${id} puts text inside a decorative glyph`).not.toMatch(/<(title|desc)[\s>]/);
    }
  });

  it("renders no personal data, even hardcoded straight into a screen", () => {
    // The content-file sweep cannot see a number typed directly into JSX. This one can.
    for (const { id, markup } of SCREEN_MARKUP) {
      const digits = textOf(markup).replace(/\s/g, "");
      expect(digits, `${id} renders a Vietnamese mobile number`).not.toMatch(
        /(^|\D)0[35789]\d{8}(\D|$)/,
      );
      expect(digits, `${id} renders a 12-digit identity number`).not.toMatch(/(^|\D)\d{12}(\D|$)/);
    }
  });

  it("names the publisher and the current screen in the header", () => {
    // The header slot phase 2 fills with the COMMUNE name (README non-negotiable #2). If it can
    // go empty in phase 1 it can go empty in phase 2, and then a citizen submits to a commune
    // they were never shown.
    const text = textOf(render(<App />));
    expect(text).toContain(COMPANY.name);
    expect(text).toContain(findScreen(DEFAULT_SCREEN_ID).headerTitle);
  });
});

describe("tab bar", () => {
  /**
   * ⚠ "MỌI MÀN", KHÔNG PHẢI "MỌI TAB" — VÀ ĐÓ LÀ CẢ Ý NGHĨA CỦA CA NÀY TỪ 21/09/2026 (khuya).
   *
   *   Sổ màn hình có NĂM màn, thanh tab vẽ BỐN ô. Màn "Danh thiếp" không có ô riêng, và nếu
   *   `TabBar` so thẳng `props.current` với `screen.id` thì đứng ở màn ấy thanh tab không sáng ô
   *   nào: công dân mất dấu vị trí của mình trong app, và không có gì đỏ lên — vì chạy hết bốn ô
   *   thì "đúng một ô sáng" vẫn đúng cho bốn màn kia.
   */
  it("marks exactly one tab as current, for every screen — kể cả màn KHÔNG có tab", () => {
    for (const screen of SCREENS) {
      const markup = render(<TabBar current={screen.id} onSelect={() => {}} />);
      const current = markup.match(/aria-current="page"/g) ?? [];
      expect(current.length, `${screen.id}: ${current.length} tabs marked current`).toBe(1);
    }
  });

  it("đứng ở màn ngoài tab thì ô CHA của nó sáng, không phải một ô bất kỳ", () => {
    const ngoai_tab = SCREENS.filter((man) => man.cho.kieu === "ngoai-tab");
    expect(ngoai_tab.length, "không còn màn nào ngoài tab — ca này xanh vì lý do sai").toBeGreaterThan(0);
    for (const man of ngoai_tab) {
      const cha = tabDangSang(man.id);
      expect(cha, `${man.id} trỏ về chính nó`).not.toBe(man.id);
      const o_cha = findScreen(cha);
      expect(o_cha.cho.kieu, `${man.id} trỏ về ${cha}, một màn cũng không có tab`).toBe("tab");
      const markup = render(<TabBar current={man.id} onSelect={() => {}} />);
      const dang_sang = markup.split("<button").find((item) => item.includes('aria-current="page"'));
      expect(textOf(dang_sang ?? "")).toContain(
        o_cha.cho.kieu === "tab" ? o_cha.cho.nhan : "",
      );
    }
  });

  it("offers a button for every tab and nothing else", () => {
    const markup = render(<TabBar current={DEFAULT_SCREEN_ID} onSelect={() => {}} />);
    const buttons = markup.match(/<button\b/g) ?? [];
    expect(TABS.length, "thanh tab rỗng").toBe(4);
    expect(buttons).toHaveLength(TABS.length);
    for (const screen of TABS) {
      expect(textOf(markup)).toContain(screen.cho.nhan);
    }
  });

  /**
   * HÌNH KHÔNG BAO GIỜ ĐỨNG MỘT MÌNH TRÊN MỘT TAB.
   *
   *   Bản mẫu vẽ một biểu tượng trên mỗi nhãn. Một thanh tab CHỈ có hình là một thanh tab phải
   *   học thuộc mới dùng được — và nó là "trạng thái báo bằng hình ảnh đơn thuần", đúng thứ
   *   README cấm ở yêu cầu #6. Ca này giữ cả hai nửa: có hình, và hình bị trình đọc màn hình bỏ
   *   qua, vì nhãn chữ bên dưới đã nói hết.
   */
  it("mỗi tab có một hình, và không hình nào bị đọc lên", () => {
    const markup = render(<TabBar current={DEFAULT_SCREEN_ID} onSelect={() => {}} />);
    const svgs = markup.match(/<svg[\s>][^>]*>/g) ?? [];
    expect(svgs).toHaveLength(TABS.length);
    for (const svg of svgs) {
      expect(svg, "thanh tab vẽ một <svg> trình đọc màn hình sẽ đọc lên").toContain(
        'aria-hidden="true"',
      );
    }
    for (const screen of TABS) {
      expect(textOf(markup), `tab ${screen.id} mất nhãn chữ, chỉ còn hình`).toContain(
        screen.cho.nhan,
      );
    }
  });

  it("announces the current tab through aria-current, not through colour", () => {
    const markup = render(<TabBar current="contact" onSelect={() => {}} />);
    const currentButton = markup.split("<button").find((item) => item.includes('aria-current="page"'));
    const lien_he = findScreen("contact");
    expect(textOf(currentButton ?? "")).toContain(lien_he.cho.kieu === "tab" ? lien_he.cho.nhan : "");
  });
});
