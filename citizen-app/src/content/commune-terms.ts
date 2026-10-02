/**
 * TERMS OF USE OF A COMMUNE'S OWN MINI APP — one template, filled per commune when the Zalo dossier is generated.
 *
 * WHY A SECOND TERMS FILE, AND NOT AN EDIT OF `dieu-khoan.ts`:
 *
 *   `dieu-khoan.ts` is the terms of the SHARED ViHAT Group app (company intro, business-card tools, offices) and
 *   stays exactly that (ADR 0044 §"Đã quyết 02/10/2026", row "App chung ViHAT"). A commune's own app has none of
 *   those features and, by the owner's decision of 02/10/2026, a different party standing behind the text: ONLY
 *   "Ủy ban nhân dân <xã>". Repurposing the shared file would leave the shared dossier describing a commune app,
 *   or the commune dossier naming a company — both are a legal text saying something false.
 *
 * TWO HALVES KEPT APART, AS IN `dieu-khoan.ts`:
 *
 *   | Half | This file |
 *   |---|---|
 *   | WHO PROVIDES THE APP / WHO ANSWERS FOR IT | the commune's People's Committee, and nobody else |
 *   | WHERE DATA GOES | NOT stated here — the privacy policy owns it (open question #28) and stays shared |
 *
 *   Writing a data recipient here would either copy the privacy policy (two copies drift) or contradict it. The
 *   "Quyền riêng tư" section points to it instead. `ket-xuat-ho-so.test.ts` pins that no "máy chủ của …" sentence
 *   and no company name appears in the rendered text.
 *
 * WHY THE PER-COMMUNE VALUES ARE PARAMETERS, NOT IMPORTS:
 *
 *   They live in `COMMUNE_TERMS_BY_DOMAIN`, in the deploy-target file under `scripts/`, next to the domain → App ID
 *   table. No production file under `src/` may import that file — nor even name it, which is how the check in
 *   `scripts/` is written — because a per-commune value
 *   reachable from `src/` is one import away from being baked into the one bundle every commune shares (rule 1,
 *   invariant 10). Nothing in the app imports this module either; only the dossier generator does.
 *
 * WHY A MISSING VALUE THROWS: this is a legal text published in the commune's name. A default or a printed
 * placeholder ("<TÊN XÃ>") would publish a wrong party, or none, and nothing would turn red. Refusing stops the
 * dossier from being written at all (`scripts/ho-so-zalo.mjs`).
 */

import type { MucChinhSach } from "./chinh-sach-rieng-tu";

/** The three values the owner fixed per commune (02/10/2026). Nothing else in the text varies by commune. */
export type CommuneTermsValues = {
  /** As the commune is named on screen, unit type first: "Xã Thăng Bình". */
  displayName: string;
  /** "Thành phố Đà Nẵng". */
  province: string;
  /** The commune's own introduction page, the ONLY contact route the text gives (no phone, no e-mail). */
  introductionUrl: string;
};

/**
 * Version and effective date — same reasoning as `PHIEN_BAN_DIEU_KHOAN`: a text with no version cannot prove what
 * it said at the moment a citizen accepted it.
 *
 * `1.0` because no citizen has read any version: the commune app has not been released on Zalo (ledger
 * `nop-zalo-duyet` still open). Numbering drafts would make a reader believe there were earlier publications.
 *
 * `02/10/2026` because that is the day the content became true: the owner decided that day that only the commune's
 * People's Committee stands behind this text (ADR 0044), and scene photos — which the petition section speaks
 * about — reached the app the same day. A legal text dated before the facts it states misstates its own date.
 *
 * From the first release on, every change to a citizen's rights or obligations takes a new number.
 */
export const COMMUNE_TERMS_VERSION = "1.0";
export const COMMUNE_TERMS_EFFECTIVE_DATE = "02/10/2026";

export const COMMUNE_TERMS_TITLE = "Điều khoản sử dụng";

/**
 * The commune-level unit types a display name may start with. The name of the People's Committee is derived from
 * it ("Xã Thăng Bình" → "Ủy ban nhân dân xã Thăng Bình", `skills/administrative-language`: lower-case unit after
 * "Ủy ban nhân dân"). A display name without one of them ("Thăng Bình") would print "Ủy ban nhân dân Thăng Bình"
 * — a body that does not exist — so it is refused rather than guessed.
 */
const UNIT_TYPES = ["Xã", "Phường", "Đặc khu"] as const;

/**
 * The checks, in one place, returning every problem at once — the person filling the table should fix all of them
 * in one pass, not discover them one run at a time.
 */
export function communeTermsProblems(values: Partial<Record<keyof CommuneTermsValues, unknown>>): string[] {
  const problems: string[] = [];
  const text = (key: keyof CommuneTermsValues): string | null => {
    const v = values[key];
    if (typeof v !== "string" || v.trim() === "") {
      problems.push(`${key}: chưa điền.`);
      return null;
    }
    if (v !== v.trim()) problems.push(`${key}: có khoảng trắng thừa ở đầu hoặc cuối.`);
    // `<…>` is this repository's placeholder spelling (the App ID table under `scripts/`). It must never be
    // printed into a legal text.
    if (/[<>]/.test(v)) problems.push(`${key}: còn là chỗ giữ chỗ "${v}", chưa phải giá trị thật.`);
    return v;
  };

  const displayName = text("displayName");
  if (displayName !== null && !UNIT_TYPES.some((unit) => displayName.startsWith(`${unit} `) && displayName.length > unit.length + 1)) {
    problems.push(`displayName: phải bắt đầu bằng ${UNIT_TYPES.join(" / ")}, rồi một dấu cách và tên xã.`);
  }
  text("province");
  const url = text("introductionUrl");
  if (url !== null) {
    let parsed: URL | null = null;
    try {
      parsed = new URL(url);
    } catch {
      parsed = null;
    }
    if (parsed === null || parsed.protocol !== "https:" || parsed.hostname === "") {
      problems.push(`introductionUrl: phải là một địa chỉ https đầy đủ, ví dụ "https://<tên-miền-của-xã>/gioi-thieu".`);
    }
  }
  return problems;
}

/** "Xã Thăng Bình" → "Ủy ban nhân dân xã Thăng Bình". Only the first letter changes case. */
export function communeAuthority(displayName: string): string {
  return `Ủy ban nhân dân ${lowerFirst(displayName)}`;
}

function lowerFirst(displayName: string): string {
  return `${displayName.charAt(0).toLocaleLowerCase("vi-VN")}${displayName.slice(1)}`;
}

/**
 * Look up the values of one commune in the per-commune table, refusing when the commune is absent or incomplete.
 * The table is passed in (it lives under `scripts/`, see the header); the error names the table and the key so the
 * next step is obvious.
 */
export function communeTermsValuesFor(
  table: Readonly<Record<string, Partial<Record<keyof CommuneTermsValues, unknown>>>>,
  domain: string,
): CommuneTermsValues {
  // The caller (`scripts/ho-so-zalo.mjs`) adds the file path: this module may not name that file (see the header).
  const where = `COMMUNE_TERMS_BY_DOMAIN["${domain}"]`;
  const row = Object.prototype.hasOwnProperty.call(table, domain) ? table[domain] : undefined;
  if (row === undefined) {
    throw new Error(
      `Chưa có ${where}. Điền tên xã, tỉnh/thành và địa chỉ trang Giới thiệu của xã do chủ dự án giao, rồi chạy lại.`,
    );
  }
  const problems = communeTermsProblems(row);
  if (problems.length > 0) {
    throw new Error(`${where} chưa dùng được:\n${problems.map((p) => `  - ${p}`).join("\n")}\nSửa rồi chạy lại.`);
  }
  return row as CommuneTermsValues;
}

/** What the renderer needs besides the version constants: the app line under the title, the opening, the sections. */
export type CommuneTerms = {
  appLine: string;
  opening: string;
  sections: readonly MucChinhSach[];
};

/**
 * The emergency sentence shown on the send screens (`cong-dan/man/noi-dung.ts` `KHAN_CAP`). Written out here, not
 * imported: `content/` and `cong-dan/` are two halves that may not import each other (`ranh-gioi-hai-nua.test.ts`).
 * `ket-xuat-ho-so.test.ts` pins the two strings equal, so they cannot drift apart.
 */
export const COMMUNE_TERMS_EMERGENCY =
  "Việc khẩn cấp, cần giúp ngay: gọi 113 (Công an), 114 (Cứu hỏa), 115 (Cấp cứu). Phản ánh trên ứng dụng không được xử lý ngay lập tức.";

/**
 * The text. Validates first and throws on any problem, so no caller can render a half-filled template.
 *
 * Section order and numbering follow `MUC_DIEU_KHOAN`: a flat list, numbered by the renderer, no sentence refers to
 * "section N". Every feature sentence describes what the commune app actually does today
 * (`cong-dan/man/noi-dung.ts`); features still being finished are named only as "đang được hoàn thiện".
 */
export function communeTerms(values: CommuneTermsValues): CommuneTerms {
  const problems = communeTermsProblems(values);
  if (problems.length > 0) {
    throw new Error(`Không dựng được điều khoản của app xã:\n${problems.map((p) => `  - ${p}`).join("\n")}`);
  }
  const { displayName, province, introductionUrl } = values;
  const authority = communeAuthority(displayName);

  const sections: MucChinhSach[] = [
    {
      ma: "ung-dung-la-gi",
      tieu_de: "Ứng dụng này là gì",
      doan: [
        `Đây là kênh thông tin và tương tác giữa ${authority} và người dân, chạy trên nền tảng Zalo.`,
        "Trên ứng dụng, bạn có thể đọc tin tức, sự kiện, thông báo của xã; nghe bản tin truyền thanh; xem video tuyên truyền; xem danh bạ cán bộ xã; gửi phản ánh, kiến nghị tới xã; theo dõi phản ánh của mình và đánh giá kết quả xử lý.",
        "Một số mục đang được hoàn thiện. Khi một mục chưa dùng được, ứng dụng nói rõ ngay trên màn hình và chỉ cách liên hệ xã.",
        `Ứng dụng là một kênh thêm, không thay cho các kênh hiện có. Bạn vẫn có thể đến trụ sở ${authority} hoặc liên hệ xã như trước.`,
      ],
    },
    {
      ma: "ai-dung-duoc",
      tieu_de: "Ai được dùng, và xác nhận tài khoản thế nào",
      doan: [
        "Mọi người dùng Zalo đều mở được ứng dụng. Đọc tin, nghe truyền thanh, xem video và xem danh bạ không cần xác nhận tài khoản.",
        "Gửi phản ánh, xem phản ánh của tôi, tra cứu phiếu và đánh giá kết quả xử lý cần xác nhận số điện thoại Zalo của bạn. Việc xác nhận là một lần chạm: Zalo hỏi bạn có đồng ý chia sẻ số điện thoại không. Bạn không phải gõ số điện thoại, mật khẩu hay mã xác thực nào.",
        "Bạn chỉ xác nhận bằng tài khoản Zalo và số điện thoại của chính mình. Không dùng tài khoản Zalo của người khác để gửi phản ánh dưới tên họ.",
        "Zalo chỉ hỏi một quyền (số điện thoại, họ tên, vị trí, máy ảnh, ảnh trong máy) sau khi bạn bấm vào tính năng cần quyền ấy. Bạn có thể từ chối. Khi đó chỉ tính năng cần quyền ấy không dùng được; phần còn lại của ứng dụng vẫn dùng bình thường.",
        "Ứng dụng không thu phí. Không có gói cước, không có thanh toán, không có giao dịch tài chính nào trong ứng dụng.",
      ],
    },
    {
      ma: "thong-tin-cua-xa",
      tieu_de: "Thông tin xã đăng trên ứng dụng",
      doan: [
        `Tin tức, sự kiện, thông báo, bản tin truyền thanh, video và ảnh trên trang chủ do ${authority} đăng. Đó là thông tin chính thức của xã tại thời điểm đăng.`,
        "Tin lấy từ Cổng thông tin điện tử được giữ nguyên dòng ghi nguồn của tin gốc.",
        `${authority} có thể cập nhật hoặc gỡ một tin khi cần. Với văn bản có giá trị pháp lý, bạn căn cứ vào bản chính thức của cơ quan ban hành, không căn cứ vào bản tin trên ứng dụng.`,
      ],
    },
    {
      ma: "gui-phan-anh",
      tieu_de: "Gửi phản ánh, kiến nghị",
      doan: [
        `Mục "Gửi phản ánh" để bạn báo với ${authority} những sự việc trên địa bàn xã, hoặc nêu kiến nghị, thuộc các lĩnh vực xã đang nhận qua ứng dụng. Danh sách lĩnh vực hiện ở bước đầu khi bạn gửi.`,
        "Trước khi gửi, ứng dụng hiện tên xã sẽ nhận phản ánh. Chỉ xã này nhận và xử lý phản ánh. Nếu sự việc xảy ra ở xã khác, bạn liên hệ Ủy ban nhân dân xã nơi xảy ra sự việc.",
        "Bạn viết đúng sự thật, nói rõ sự việc, nơi xảy ra và thời điểm xảy ra.",
        "Không gửi phản ánh sai sự thật hoặc bịa đặt. Không xúc phạm, đe doạ hay bôi nhọ cá nhân, tổ chức nào. Không gửi quảng cáo, nội dung không liên quan, và không gửi lặp lại cùng một việc nhiều lần.",
        "Ảnh hiện trường không bắt buộc. Ảnh gửi kèm phải do bạn chụp hoặc bạn có quyền sử dụng, và phải là ảnh của đúng sự việc. Không chụp rõ mặt người, giấy tờ, biển số xe hay bên trong nhà riêng của người khác, trừ khi thật cần để làm rõ sự việc.",
        `Bạn có thể chọn "Gửi ẩn danh". Ứng dụng nói rõ ngay tại đó những thông tin nào của bạn khi ấy không hiện cho cán bộ xử lý.`,
        `${authority} tiếp nhận và xử lý phản ánh theo quy trình và thời hạn của xã. Phản ánh chỉ được xem là đã gửi khi ứng dụng hiện mã phiếu. Cùng với mã phiếu, ứng dụng hiện mốc thời gian cán bộ xã sẽ xem phiếu; khi xã đã xác định lĩnh vực, phiếu hiện thêm thời hạn dự kiến xử lý xong.`,
        // Not a string starting "Xã …": `kham-pha.test.tsx` reads any such literal as a commune name baked into code.
        `${authority} có thể không tiếp nhận một phản ánh, hoặc chuyển phản ánh tới cơ quan có thẩm quyền. Khi đó phiếu ghi rõ lý do, hoặc tên cơ quan nhận.`,
        COMMUNE_TERMS_EMERGENCY,
        `Phản ánh, kiến nghị gửi qua ứng dụng không phải là đơn khiếu nại hay đơn tố cáo, và không thay thế các thủ tục, hồ sơ hành chính theo quy định. Để khiếu nại, tố cáo hoặc làm thủ tục hành chính, bạn liên hệ trực tiếp ${authority} để được hướng dẫn.`,
      ],
    },
    {
      ma: "phan-anh-cua-toi",
      tieu_de: "Phản ánh của tôi, mã phiếu và đánh giá",
      doan: [
        `Mục "Phản ánh của tôi" chỉ hiện những phản ánh chính bạn đã gửi tới xã này sau khi xác nhận số điện thoại. Bạn không xem được phản ánh của người khác, và người khác không xem được phản ánh của bạn qua ứng dụng.`,
        "Mỗi phản ánh có một mã phiếu. Bạn giữ mã này để tra cứu và để nói với xã khi hỏi về phản ánh ấy. Không đưa mã phiếu cho người không liên quan.",
        "Ứng dụng hiện tình trạng và kết quả xử lý phản ánh của bạn. Ghi chú nội bộ và quá trình luân chuyển trong xã không hiện trên ứng dụng.",
        "Khi ứng dụng mời bạn đánh giá kết quả xử lý, bạn có thể chấm từ 1 đến 5 sao và viết thêm nhận xét. Đánh giá giúp xã biết việc đã được giải quyết đúng mong muốn chưa. Nhận xét cũng phải đúng sự thật và không xúc phạm ai.",
      ],
    },
    {
      ma: "danh-ba",
      tieu_de: "Danh bạ cán bộ xã",
      doan: [
        "Danh bạ hiện thông tin liên hệ công vụ mà xã chọn công khai, như họ tên, chức vụ, bộ phận và số điện thoại.",
        "Bạn dùng danh bạ để liên hệ việc công. Không dùng để quảng cáo, gọi điện hay nhắn tin quấy rối, và không thu thập các số điện thoại ấy để chia sẻ hay bán lại.",
        "Cuộc gọi từ danh bạ do điện thoại và nhà mạng của bạn thực hiện, cước gọi tính theo nhà mạng.",
      ],
    },
    {
      ma: "lien-ket-ra-ngoai",
      tieu_de: "Liên kết mở ra ngoài ứng dụng",
      doan: [
        "Một số bài viết và ảnh trên trang chủ có liên kết tới trang web khác. Trước khi mở, ứng dụng hỏi lại bạn và nêu tên trang sẽ mở. Bạn bấm \"Mở trang\" thì trang mới mở; bấm \"Ở lại ứng dụng\" thì không có gì xảy ra.",
        "Nút \"Xem video\" mở video trong trình duyệt của Zalo.",
        "Trang bên ngoài không thuộc ứng dụng của xã. Xã không chịu trách nhiệm về nội dung, độ an toàn hay cách các trang ấy xử lý thông tin của bạn.",
      ],
    },
    {
      ma: "khong-duoc-lam",
      tieu_de: "Những việc bạn không được làm",
      doan: [
        "Không mạo danh người khác, cán bộ hay cơ quan nhà nước.",
        "Không dùng ứng dụng để gửi nội dung vi phạm pháp luật, sai sự thật, hoặc xúc phạm danh dự, nhân phẩm của người khác.",
        "Không thu thập, sử dụng thông tin của người khác trái với quy định về bảo vệ dữ liệu cá nhân tại Nghị định 13/2023/NĐ-CP.",
        "Không can thiệp, dịch ngược, dò quét hay tấn công ứng dụng và hệ thống của xã. Không dùng công cụ tự động để gửi yêu cầu hàng loạt.",
        "Không dùng tên, biểu tượng hay nội dung của xã vào mục đích thương mại, hoặc theo cách làm người khác hiểu sai là thông tin của xã.",
        "Phản ánh vi phạm các điều trên có thể không được tiếp nhận. Người vi phạm chịu trách nhiệm theo quy định của pháp luật.",
      ],
    },
    {
      ma: "quyen-doi-voi-noi-dung",
      tieu_de: "Quyền đối với nội dung",
      doan: [
        `Tin, ảnh, bản tin truyền thanh, video và các nội dung khác do ${authority} đăng thuộc quyền của xã, hoặc của nguồn được ghi kèm nội dung ấy.`,
        "Không sửa đổi nội dung của xã rồi đưa ra như thông tin chính thức của xã.",
        "Nội dung phản ánh và ảnh bạn gửi là để xã tiếp nhận và xử lý phản ánh. Cách các thông tin ấy được xử lý và lưu giữ nằm trong Chính sách quyền riêng tư.",
      ],
    },
    {
      ma: "gioi-han-trach-nhiem",
      tieu_de: "Giới hạn trách nhiệm",
      doan: [
        "Ứng dụng được cung cấp miễn phí và theo hiện trạng. Xã không cam kết ứng dụng luôn chạy liên tục, không gián đoạn.",
        "Ứng dụng chạy trong Zalo. Nó phụ thuộc vào nền tảng Zalo, phiên bản Zalo, điện thoại, mạng và các quyền bạn đã cấp. Khi những thứ ấy gặp sự cố, một số tính năng có thể tạm thời không dùng được.",
        `Khi ứng dụng không dùng được, bạn vẫn có thể liên hệ trực tiếp ${authority}. Một phản ánh chưa có mã phiếu là phản ánh xã chưa nhận được.`,
        "Giới hạn này không loại trừ những trách nhiệm mà pháp luật Việt Nam không cho phép loại trừ.",
      ],
    },
    {
      ma: "quyen-rieng-tu",
      tieu_de: "Quyền riêng tư",
      doan: [
        // The policy is shared by every app and owns every data statement (#28). Not "đọc được trong Mini App": no
        // commune screen opens it yet (tmp/xin-quyen-zalo/README.md, "Việc còn phải làm"). When one does, say so here.
        "Ứng dụng xử lý thông tin nào của bạn, gửi tới đâu, lưu trong bao lâu và bạn có những quyền gì: tất cả nằm trong Chính sách quyền riêng tư của ứng dụng, công bố cùng Điều khoản này.",
        "Nội dung ấy không chép lại vào đây, để hai văn bản không bao giờ nói hai điều khác nhau về cùng một việc.",
      ],
    },
    {
      ma: "thay-doi-dieu-khoan",
      tieu_de: "Thay đổi điều khoản",
      doan: [
        `Điều khoản này có hiệu lực từ ngày ${COMMUNE_TERMS_EFFECTIVE_DATE}, phiên bản ${COMMUNE_TERMS_VERSION}.`,
        "Khi ứng dụng có thêm tính năng hoặc có thay đổi làm khác những gì viết ở trên, xã cập nhật điều khoản và công bố bản mới, ghi rõ phiên bản và ngày hiệu lực, trước khi thay đổi ấy bắt đầu áp dụng.",
      ],
    },
    {
      ma: "luat-va-lien-he",
      tieu_de: "Luật áp dụng và liên hệ",
      doan: [
        "Điều khoản này chịu sự điều chỉnh của pháp luật Việt Nam.",
        // The owner's sentence, verbatim (02/10/2026). No phone or e-mail: the commune keeps its own contact details
        // current on its introduction page; a copy here would go stale without anyone noticing. No full stop after
        // the URL, so a reader copying it does not copy the dot.
        `Mọi câu hỏi hoặc khiếu nại liên quan tới ứng dụng và nội dung của xã, xin liên hệ ${authority} theo thông tin tại trang Giới thiệu của xã: ${introductionUrl}`,
      ],
    },
  ];

  return {
    appLine: `Mini App ${displayName}, ${province}`,
    // Mid-sentence the unit is lower case ("của xã Thăng Bình"), as after "Ủy ban nhân dân"; the app line above is
    // a heading and keeps the on-screen spelling.
    opening: `Điều khoản này áp dụng cho ứng dụng Zalo Mini App của ${lowerFirst(displayName)}, ${province}. ${authority} cung cấp ứng dụng này cho người dân và chịu trách nhiệm về ứng dụng. Khi dùng ứng dụng, bạn đồng ý thực hiện các điều khoản dưới đây.`,
    sections,
  };
}
