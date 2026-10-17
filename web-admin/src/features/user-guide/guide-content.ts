import { createHref } from "@/lib/create-link";
import {
  CITIZEN_UPDATE_PERMISSION,
  PETITION_CREATE_PERMISSION,
  QUYEN_GHI_NGAN_SACH,
  QUYEN_QUAN_LY_NGUOI_DUNG,
  QUYEN_SOAN_THONG_BAO,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XEM_NOI_DUNG,
} from "@/lib/quyen";
import { CONTENT_UPDATE_PERMISSION } from "@/features/noi-dung/nhan-noi-dung";
import { PETITION_INTAKE_PERMISSION } from "@/features/phan-anh/nhan-phieu";
import type { MaTabCauHinh, SystemTabId } from "@/features/cau-hinh/thanh-tab-cau-hinh";

/**
 * The "Hướng dẫn sử dụng" page's content (`/huong-dan`) — the end-user guide, as typed data so the page
 * can drop what a reader cannot open (owner 17/10/2026: sections and buttons for menus the account cannot
 * reach are HIDDEN, not greyed).
 *
 * WRITTEN FROM TODAY'S SCREENS, NOT FROM THE PROTOTYPE'S TEXT. The prototype guide (16/10/2026) described
 * several things that are not true of the code: greyed menu items for missing permissions, a working
 * global search, mail sent with every announcement, a fixed Kanban chain. Each sentence below was checked
 * against the screen's own words file; where a behaviour could not be checked, it is left out rather than
 * guessed — a guide that promises what the screen does not do is read as the commune's word.
 *
 * NO COMMUNE VALUE HERE (rule 1, invariant 10): the commune's name is drawn by the page from the runtime
 * configuration. NO PERMISSION KEY IS SHOWN to the reader: keys appear only in `permission` fields, which
 * decide visibility and are never rendered.
 *
 * TEXT FORMAT: plain strings; `**…**` marks a bold run (a button or tab name). Parsed into React nodes by
 * `RichText` — never injected as HTML (rule 13, forbidden #3).
 */

/**
 * The page title — the menu item's label. Here, not in the `"use client"` component: the server page reads
 * it for the tab title, and a constant exported from a client file reaches the server as a client
 * reference, not a string (`lib/cau-hinh-xa-hien-thi.ts` records the day that broke a page).
 */
export const GUIDE_TITLE = "Hướng dẫn sử dụng";

/** One step, bullet or cell. `**word**` = bold. */
export type GuideText = string;

export type GuideBlock =
  | { readonly kind: "steps"; readonly title?: string; readonly items: readonly GuideText[] }
  | { readonly kind: "bullets"; readonly title?: string; readonly items: readonly GuideText[] }
  | { readonly kind: "paragraph"; readonly text: GuideText }
  | { readonly kind: "note"; readonly tone: "info" | "warn"; readonly text: GuideText }
  | {
      readonly kind: "table";
      readonly head: readonly [string, string];
      readonly rows: readonly (readonly [GuideText, GuideText])[];
    };

/**
 * A button into the real feature. Shown only when the reader can use it:
 * - `route` — the MENU route that must be visible to this account (`locMenu`), the door to the screen;
 *   `null` for a page every signed-in account opens (`/ca-nhan`, `/doi-mat-khau`);
 * - `permission` — an extra key the screen itself checks before drawing that control (the create button);
 * - `configTab` / `systemTab` — a tab of `/cau-hinh` / `/he-thong` whose own gate must open.
 * Hiding is convenience: the screen and the server still check (rule 5, forbidden #1).
 */
export type GuideAction = {
  readonly label: string;
  readonly href: string;
  /** `null` = needs no menu item (the person's own pages, open to every signed-in account). */
  readonly route: string | null;
  readonly permission?: string;
  readonly configTab?: MaTabCauHinh;
  readonly systemTab?: SystemTabId;
  /** The section's main act — drawn as the solid button. */
  readonly primary?: boolean;
};

export type GuideSection = {
  /** Anchor id — part of the address bar (`/huong-dan#nhiem-vu`), so it reads as the screen's name. */
  readonly id: string;
  readonly title: string;
  /**
   * Menu routes this section explains. `null` = not tied to a menu (always shown). Several = shown when
   * ANY of them is visible ("Người dùng & Phân quyền" covers two menu items).
   */
  readonly routes: readonly string[] | null;
  readonly lede: GuideText;
  readonly actions: readonly GuideAction[];
  readonly blocks: readonly GuideBlock[];
};

const CONFIG = "/cau-hinh";
const SYSTEM = "/he-thong";

function configTab(label: string, tab: MaTabCauHinh): GuideAction {
  return { label, href: `${CONFIG}?tab=${tab}`, route: CONFIG, configTab: tab };
}

function systemTab(label: string, tab: SystemTabId): GuideAction {
  return { label, href: `${SYSTEM}?tab=${tab}`, route: SYSTEM, systemTab: tab };
}

export const GUIDE_SECTIONS: readonly GuideSection[] = [
  {
    id: "dang-nhap",
    title: "Đăng nhập và làm quen giao diện",
    routes: null,
    lede:
      "Mỗi cán bộ một tài khoản riêng, đăng nhập bằng thư điện tử công vụ. Không dùng chung tài khoản: mọi thao tác trong hệ thống đều ghi tên người làm.",
    actions: [],
    blocks: [
      {
        kind: "steps",
        items: [
          "Mở địa chỉ phần mềm của xã trên trình duyệt.",
          "Nhập **thư điện tử công vụ** và **mật khẩu** được cấp, rồi bấm **Đăng nhập**.",
          "Lần đầu đăng nhập bằng mật khẩu tạm, hệ thống mở màn **Đổi mật khẩu** trước; đổi xong mới dùng được các màn khác.",
          "Sau khi vào: tài khoản giữ vai trò lãnh đạo mở thẳng **Sổ tay lãnh đạo**, các tài khoản khác mở **Tổng quan**.",
        ],
      },
      {
        kind: "bullets",
        title: "Bố cục chung của mọi màn hình",
        items: [
          "**Thanh bên trái** chia hai nhóm **Điều hành** và **Quản trị**. Thanh bên chỉ hiện những mục tài khoản của bạn được dùng; mục ngoài quyền không hiện. Nút thu gọn nằm ở đầu thanh bên.",
          "Mục có chữ **mờ** trên thanh bên là màn chưa được dựng, chưa bấm được.",
          "**Thanh trên cùng**: tên đơn vị, ô tìm kiếm chung, chuông thông báo và tên tài khoản đang đăng nhập. Ô tìm kiếm chung hiện chưa dùng được (có dấu **?**); mỗi sổ có bộ lọc riêng trên màn của nó.",
          "Bấm vào **tên của mình** ở góc phải để mở **Hồ sơ cá nhân**, **Đổi mật khẩu** hoặc **Đăng xuất**.",
          "Trên điện thoại hoặc màn hình hẹp, thanh bên được thu vào nút menu trên thanh trên cùng.",
          "Nút hoặc ô có dấu **?** là phần chưa dùng được; di chuột hoặc bấm vào dấu **?** để đọc lý do.",
        ],
      },
      {
        kind: "note",
        tone: "info",
        text: "**Quên mật khẩu?** Nhờ quản trị viên của xã đặt lại mật khẩu ở màn Người dùng. Không chia sẻ mật khẩu qua tin nhắn.",
      },
    ],
  },
  {
    id: "tong-quan",
    title: "Tổng quan",
    routes: ["/tong-quan"],
    lede:
      "Nhiệm vụ, văn bản, giải ngân, thu - chi, phản ánh và kinh tế nằm trên một trang, không phải mở từng phân hệ để cộng lại.",
    actions: [{ label: "Mở Tổng quan", href: "/tong-quan", route: "/tong-quan", primary: true }],
    blocks: [
      {
        kind: "bullets",
        items: [
          "**Chọn kỳ**: Tuần này, Tháng này, Quý này, Năm nay. Dòng dưới tiêu đề ghi rõ kỳ đang xem và thời điểm tính số liệu.",
          "**Mỗi khối** có con số chính kèm dòng “Kỳ trước” để so sánh.",
          "**Giải ngân** và **Thu - Chi** tính luỹ kế theo năm ngân sách, không theo kỳ đang xem.",
          "**Cần xử lý ngay** (khối cuối): các việc cần lãnh đạo can thiệp. Bấm một dòng nhiệm vụ để mở chi tiết.",
          "Xuất **PDF**, **XLSX**, **PPTX**, hoặc bấm **Trình chiếu** để chiếu trong phòng họp.",
          "Khối nào thuộc phân hệ tài khoản của bạn không được xem thì không hiện.",
        ],
      },
      {
        kind: "note",
        tone: "info",
        text: "Ô có dấu **—** là chưa có số liệu, không phải bằng 0.",
      },
    ],
  },
  {
    id: "nhiem-vu",
    title: "Nhiệm vụ",
    routes: ["/nhiem-vu"],
    lede:
      "Giao việc, theo dõi tiến độ và đôn đốc. Mỗi nhiệm vụ có mã, người hoặc bộ phận thực hiện, hạn xử lý và trạng thái.",
    actions: [
      { label: "Giao việc mới", href: createHref("/nhiem-vu"), route: "/nhiem-vu", permission: QUYEN_TAO_NHIEM_VU, primary: true },
      { label: "Mở danh sách nhiệm vụ", href: "/nhiem-vu", route: "/nhiem-vu" },
    ],
    blocks: [
      {
        kind: "steps",
        title: "Giao việc mới",
        items: [
          "Bấm **Giao việc mới** (góc phải trên).",
          "Nhập tên nhiệm vụ, chọn bộ phận hoặc người nhận việc, loại nhiệm vụ, mức ưu tiên, nguồn giao và hạn xử lý.",
          "Lưu. Người được giao thấy việc trên bảng và trong phạm vi **Giao cho tôi**.",
        ],
      },
      { kind: "paragraph", text: "Có nhiều việc một lúc thì dùng **Nhập từ Excel**." },
      {
        kind: "bullets",
        title: "Theo dõi và lọc",
        items: [
          "Ba phạm vi: **Toàn xã**, **Giao cho tôi**, **Liên quan đến tôi**.",
          "Lọc theo bộ phận, người thực hiện, mức ưu tiên, loại nhiệm vụ, nguồn giao; bật nhanh **Chỉ việc quá hạn** hoặc **Sắp đến hạn**.",
          "Ba cách xem: **Kanban**, **Danh sách**, **Sổ theo dõi**.",
          "Tên các cột trạng thái (mặc định bắt đầu bằng **Mới giao**) do xã đặt ở **Cấu hình › Danh mục**, nên có thể khác giữa các xã.",
        ],
      },
      {
        kind: "bullets",
        title: "Trong chi tiết một nhiệm vụ",
        items: [
          "**Đề nghị lùi hạn**: người làm ghi hạn mới và lý do rồi gửi; chỉ lãnh đạo giao việc ghi trên nhiệm vụ mới **Duyệt lùi hạn** hoặc **Từ chối**. Hạn ban đầu vẫn được giữ trong **Lịch sử gia hạn**.",
          "**Thêm việc con**: tách một phần việc giao cho người khác, dùng cùng biểu mẫu với Giao việc mới.",
          "**Nhật ký & Trao đổi**: ghi lại mọi lần chuyển trạng thái, giao lại, và trao đổi giữa người giao với người làm.",
          "Cán bộ báo xong thì việc chuyển sang chờ duyệt; lãnh đạo duyệt hoàn thành hoặc trả lại.",
        ],
      },
    ],
  },
  {
    id: "so-tay",
    title: "Sổ tay lãnh đạo",
    routes: ["/nhiem-vu/so-tay"],
    lede: "Ba việc lãnh đạo cần biết ngay trên một màn hình: việc trễ, việc chờ mình duyệt và việc mình đã giao.",
    actions: [{ label: "Mở Sổ tay", href: "/nhiem-vu/so-tay", route: "/nhiem-vu/so-tay", primary: true }],
    blocks: [
      {
        kind: "bullets",
        items: [
          "**Việc quá hạn**: các việc đã qua hạn mà chưa xong.",
          "**Chờ tôi duyệt**: gồm **Duyệt hoàn thành** (cán bộ báo xong, chờ xác nhận) và **Duyệt lùi hạn** (cán bộ xin gia hạn).",
          "**Việc tôi đã giao**: các việc lãnh đạo đã giao, kèm người làm và hạn. Bấm một dòng để mở chi tiết.",
        ],
      },
    ],
  },
  {
    id: "bien-ban",
    title: "Biên bản họp",
    routes: ["/nhiem-vu/bien-ban"],
    lede:
      "Nhập một biên bản, tách các kết luận thành nhiệm vụ. Mỗi nhiệm vụ giữ liên kết về kết luận gốc để truy vết về sau.",
    actions: [
      // The screen draws `Nhập biên bản` with no permission gate of its own: the menu item is the door.
      { label: "Nhập biên bản", href: createHref("/nhiem-vu/bien-ban"), route: "/nhiem-vu/bien-ban", primary: true },
    ],
    blocks: [
      {
        kind: "steps",
        items: [
          "Bấm **Nhập biên bản**, điền **Tên cuộc họp** và **Ngày họp** (bắt buộc), **Số hiệu biên bản**, địa điểm, nội dung và cán bộ tham dự.",
          "Nhập từng kết luận vào ô “Nhập một kết luận của cuộc họp…” rồi bấm **Thêm kết luận**.",
          "Ở mỗi kết luận, bấm **Tách thành nhiệm vụ** để giao việc. Kết luận không cần giao việc thì **Đánh dấu “không phát sinh nhiệm vụ”**.",
          "Người có thẩm quyền bấm **Ký biên bản**. Sau khi ký, có thể **Ghi số Thông báo kết luận** (một lần) hoặc **Lập biên bản bổ sung** khi cần đính chính.",
        ],
      },
      {
        kind: "note",
        tone: "warn",
        text: "**Lưu ý:** kết luận đã tách thành nhiệm vụ bị khoá nội dung, không sửa hay gỡ được; biên bản còn nhiệm vụ trỏ về cũng không gỡ được. Sau khi ký, nội dung, kết luận, chủ trì và thư ký bị khoá. Không nhập nội dung thuộc bí mật nhà nước vào hệ thống.",
      },
    ],
  },
  {
    id: "van-ban",
    title: "Văn bản & Đơn thư",
    routes: ["/van-ban"],
    lede:
      "Vào sổ, phân công xử lý và theo dõi hạn giải quyết đơn thư của công dân, cùng hai sổ văn bản đến và văn bản đi.",
    actions: [
      { label: "Vào sổ đơn thư", href: createHref("/van-ban"), route: "/van-ban", permission: PETITION_CREATE_PERMISSION, primary: true },
      { label: "Mở Văn bản & Đơn thư", href: "/van-ban", route: "/van-ban" },
    ],
    blocks: [
      {
        kind: "steps",
        title: "Đơn thư công dân",
        items: [
          "Bấm **Vào sổ đơn thư** để nhập từng đơn, hoặc **Nhập từ Excel** khi có sẵn danh sách.",
          "Chọn đơn vị và cán bộ xử lý; hạn giải quyết tính theo cấu hình thời hạn của xã.",
          "Theo dõi bằng bộ lọc ngày, đơn vị, cán bộ và trạng thái. Thẻ **Báo cáo** tổng hợp số liệu đơn thư.",
        ],
      },
      {
        kind: "note",
        tone: "info",
        text: "Hệ thống **cảnh báo khi một người gửi lại đơn có nội dung tương tự**, để tránh thụ lý trùng.",
      },
      {
        kind: "bullets",
        title: "Văn bản đến và văn bản đi",
        items: [
          "Màn mở ở thẻ **Đơn thư công dân**; chuyển sang thẻ **Văn bản đến** hoặc **Văn bản đi** để làm việc với sổ văn bản hành chính.",
          "Thẻ **Văn bản đến**: bấm **Vào sổ văn bản đến**, sau đó **Chuyển xử lý** cho bộ phận chịu trách nhiệm.",
          "Thẻ **Văn bản đi**: bấm **Cấp số văn bản đi** khi phát hành văn bản.",
        ],
      },
    ],
  },
  {
    id: "giai-ngan",
    title: "Giải ngân",
    routes: ["/giai-ngan"],
    lede: "Tiến độ giải ngân theo dự án, chứng từ và vướng mắc cần tháo gỡ.",
    actions: [
      { label: "Thêm dự án", href: createHref("/giai-ngan"), route: "/giai-ngan", permission: QUYEN_GHI_NGAN_SACH, primary: true },
      { label: "Mở Giải ngân", href: "/giai-ngan", route: "/giai-ngan" },
    ],
    blocks: [
      {
        kind: "steps",
        items: [
          "Chọn **Năm ngân sách**.",
          "Khai báo **Hạng mục** và **Nguồn vốn** (lần đầu).",
          "Bấm **Thêm dự án**, sau đó cập nhật từng đợt bằng **Nhập giải ngân**.",
        ],
      },
      {
        kind: "bullets",
        items: [
          "Biểu đồ luỹ kế giải ngân so với kế hoạch, tiến độ theo hạng mục và theo nguồn vốn.",
          "Dự án tụt sau thời gian đã trôi qua quá ngưỡng cảnh báo của xã được tính là **dự án chậm**. Lọc nhanh **Chỉ dự án chậm**, hoặc **Gộp theo hạng mục**.",
        ],
      },
      {
        kind: "bullets",
        title: "Chứng từ của một dự án",
        items: [
          "Chứng từ đi qua ba bước: nhập → **Xác nhận** → **Khoá**.",
          "Sửa một chứng từ đã xác nhận sẽ đưa nó về trạng thái nhập và xoá dấu người xác nhận.",
          "Chứng từ đã khoá không sửa, không gỡ được. Muốn sửa phải **Mở khoá**, ghi lý do, và phải là **một cán bộ khác** — không phải chính người vừa khoá.",
        ],
      },
      {
        kind: "note",
        tone: "warn",
        text: "Hệ thống là công cụ theo dõi và điều hành, **không phải phần mềm kế toán**. Số liệu phục vụ chỉ đạo, không thay sổ sách kế toán và không đối chiếu với Kho bạc.",
      },
    ],
  },
  {
    id: "thu-chi",
    title: "Thu - Chi ngân sách",
    routes: ["/giai-ngan/thu-chi"],
    lede:
      "Nạp thẳng tệp Excel của bộ phận tài chính. Một tệp hai sheet nạp được cả thu lẫn chi trong một lần; khoản mục và cột sinh ra theo đúng tệp.",
    actions: [{ label: "Mở Thu - Chi", href: "/giai-ngan/thu-chi", route: "/giai-ngan/thu-chi", primary: true }],
    blocks: [
      {
        kind: "steps",
        items: [
          "Chọn **Năm ngân sách**, rồi bấm **Nạp từ Excel** (hoặc **Lập bảng** để tạo tay).",
          "Chuyển giữa thẻ **Chi ngân sách** và **Thu ngân sách** để xem từng bảng.",
          "Khối **Chỉ số ngân sách** cho biết thu, chi đạt bao nhiêu so với dự toán và chênh lệch thu – chi luỹ kế.",
        ],
      },
      {
        kind: "paragraph",
        text: "**Chốt kỳ**: chọn kỳ cần chốt rồi bấm **Chốt kỳ…**. Chốt theo tháng khoá việc thêm, gỡ đợt thu chi của tháng đó; chốt cả năm khoá luôn việc sửa bảng, khoản mục và số liệu của năm.",
      },
    ],
  },
  {
    id: "thong-bao",
    title: "Thông báo nội bộ",
    routes: ["/thong-bao"],
    lede:
      "Thông báo gửi tới cán bộ trong phần mềm. Người nhận thấy trên trang này và ở chuông thông báo. Đây không phải tin gửi cho người dân — tin cho người dân dùng màn Gửi tin ZNS / SMS.",
    actions: [
      { label: "Soạn thông báo", href: createHref("/thong-bao"), route: "/thong-bao", permission: QUYEN_SOAN_THONG_BAO, primary: true },
    ],
    blocks: [
      {
        kind: "steps",
        items: [
          "Bấm **Soạn thông báo**, nhập tiêu đề và nội dung.",
          "Chọn người nhận theo **họ tên** ở ô **Chọn cán bộ nhận thông báo**. Ô chọn theo bộ phận chưa dùng được (có dấu **?**).",
          "Bấm **Phát hành**.",
          "Xem lại ở **Cả sổ thông báo**. Lối lọc **Gửi cho tôi** chưa dùng được.",
        ],
      },
      {
        kind: "note",
        tone: "warn",
        text: "Ô **Gửi thư điện tử** hiện chỉ ghi lại yêu cầu; thông báo **chưa được gửi qua thư điện tử**.",
      },
    ],
  },
  {
    id: "nguoi-dan",
    title: "Danh bạ người dân",
    routes: ["/nguoi-dan"],
    lede:
      "Số điện thoại để xã liên lạc với bà con, phân theo nhóm và gắn nhãn. Cột Zalo cho biết số đã đăng nhập Mini App của xã. Đây là nguồn người nhận của màn Gửi tin ZNS / SMS.",
    actions: [
      { label: "Thêm người dân", href: createHref("/nguoi-dan"), route: "/nguoi-dan", permission: CITIZEN_UPDATE_PERMISSION, primary: true },
      { label: "Mở Danh bạ người dân", href: "/nguoi-dan", route: "/nguoi-dan" },
    ],
    blocks: [
      {
        kind: "bullets",
        title: "Tìm và lọc",
        items: [
          "Ô tìm theo tên, số điện thoại, email; lọc theo **nhóm**, theo **nhãn** đang dùng, hoặc bật **Chỉ người có Zalo**.",
          "Nút **Bộ lọc** mở thêm điều kiện, ví dụ ngày tạo.",
          "Số điện thoại hiện dạng che một phần. Bấm **Xem số đầy đủ** khi thật cần.",
        ],
      },
      {
        kind: "steps",
        title: "Thêm người dân",
        items: [
          "Bấm **Thêm người dân**. Bắt buộc **Họ và tên** và **Số điện thoại**; các ô còn lại điền được tới đâu hay tới đó.",
          "Chọn thôn, tổ dân phố, gắn nhãn (cách nhau bằng dấu phẩy) và chọn nhóm.",
          "Điền các thuộc tính riêng của xã nếu có (xã tự khai ở **Cấu hình › Thuộc tính danh bạ**), rồi bấm **Thêm vào danh bạ**.",
        ],
      },
      {
        kind: "bullets",
        items: [
          "Nhiều người một lúc: **Nhập từ Excel**, tải tệp mẫu ngay trong hộp nhập.",
          "Thẻ **Nhóm**: lập nhóm mới hoặc gỡ nhóm. Gỡ nhóm không xoá người trong nhóm khỏi danh bạ.",
          "Xoá một người phải ghi **lý do gỡ**; dòng được giữ lại kèm lý do, không xoá hẳn.",
        ],
      },
      {
        kind: "note",
        tone: "warn",
        text: "Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị.",
      },
    ],
  },
  {
    id: "gui-tin",
    title: "Gửi tin ZNS / SMS",
    routes: ["/gui-thong-bao"],
    lede:
      "Gửi tin tới người dân hoặc cán bộ qua Zalo (ZNS, từ OA của xã) hoặc SMS. Tin ở đây tốn phí và không rút lại được, nên luôn phải xem trước khi gửi.",
    actions: [{ label: "Soạn tin", href: "/gui-thong-bao", route: "/gui-thong-bao", primary: true }],
    blocks: [
      {
        kind: "steps",
        title: "Thẻ Soạn tin",
        items: [
          "**Gửi cho ai**: chọn toàn bộ người dân, người dân theo nhóm hoặc theo nhãn, toàn bộ cán bộ hoặc cán bộ theo bộ phận. Có thể **Nhập số điện thoại** (cách nhau bằng dấu phẩy) hoặc **Nhập tệp** danh sách thay cho danh bạ.",
          "**Mẫu tin**: chọn kênh và mẫu. Mẫu ZNS xếp theo từng OA của xã; mẫu mới duyệt trên Zalo thì bấm **Nạp mẫu từ OA**. Mẫu của OA cần uỷ quyền lại chưa gửi được.",
          "**Nội dung**: với mỗi tham số của mẫu, chọn **Giá trị chung** (mọi người nhận cùng một giá trị) hoặc **Theo thuộc tính** (mỗi người nhận giá trị của chính mình trong danh bạ).",
          "Bấm **Xem trước** để thấy số người nhận, số bị bỏ qua, nội dung, đơn giá và chi phí dự kiến.",
          "Kiểm tra lại rồi bấm **Gửi thông báo**. Đợt đã gửi không rút lại được.",
        ],
      },
      {
        kind: "paragraph",
        text: "Thẻ **Đã gửi**: danh sách các đợt đã gửi; bấm một đợt để xem từng người nhận.",
      },
      {
        kind: "note",
        tone: "info",
        text: "Chưa có OA hoặc tài khoản SMS nào thì phải kết nối trước ở **Cấu hình › Kênh Zalo** hoặc **Cấu hình › Kênh SMS**.",
      },
    ],
  },
  {
    id: "phan-anh",
    title: "Phản ánh người dân",
    routes: ["/phan-anh"],
    lede:
      "Tiếp nhận phản ánh từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh trước và sau khi xử lý.",
    actions: [
      { label: "Nhập hộ phản ánh", href: createHref("/phan-anh"), route: "/phan-anh", permission: PETITION_INTAKE_PERMISSION, primary: true },
      { label: "Mở danh sách phản ánh", href: "/phan-anh", route: "/phan-anh" },
    ],
    blocks: [
      {
        kind: "bullets",
        items: [
          "Bốn chỉ số đầu trang: **Tổng phản ánh 90 ngày**, **Đúng hạn / trễ hạn**, **Điểm hài lòng trung bình**, **Chờ kiểm duyệt**.",
          "Ba thẻ: **Danh sách**, **Bản đồ nhiệt** (phản ánh tập trung ở đâu), **Báo cáo**.",
          "Lọc theo trạng thái, lĩnh vực, địa bàn; bật **Chỉ phiếu trễ hạn** hoặc **Bị đánh giá thấp**.",
          "Người dân báo qua điện thoại hoặc đến trực tiếp: bấm **Nhập hộ phản ánh**, rồi đọc hoặc đưa mã tra cứu cho người dân giữ lại.",
        ],
      },
      {
        kind: "steps",
        title: "Quy trình xử lý một phiếu",
        items: [
          "Phiếu mới vào ở trạng thái **Đã tiếp nhận**.",
          "Phân loại lĩnh vực: phiếu sang **Đang phân loại** và hạn xử lý xong được ấn định từ lúc này.",
          "Chuyển cho bộ phận hoặc cán bộ xử lý: phiếu sang **Đã chuyển xử lý**.",
          "Xử lý xong, tải **ảnh sau xử lý** để đối chiếu với ảnh người dân gửi.",
          "Đóng phiếu: bắt buộc có ảnh sau xử lý và ghi **Kết quả xử lý** — người dân đọc được câu này khi tra cứu.",
        ],
      },
      {
        kind: "note",
        tone: "info",
        text: "Người dân chấm 1–2 sao trên Mini App thì phiếu **tự mở lại** để xử lý tiếp. Đánh giá do cán bộ ghi hộ (qua điện thoại, tại quầy) chỉ để theo dõi, không mở lại phiếu.",
      },
    ],
  },
  {
    id: "ban-do",
    title: "Bản đồ kinh tế số",
    routes: ["/ban-do"],
    lede: "Bản đồ doanh nghiệp, hộ kinh doanh và tài nguyên trên địa bàn xã, kèm lớp bản đồ nhiệt phản ánh.",
    actions: [{ label: "Mở bản đồ", href: "/ban-do", route: "/ban-do", primary: true }],
    blocks: [
      {
        kind: "steps",
        title: "Thiết lập lần đầu",
        items: [
          "Bấm **Nạp 11 nhóm mặc định** để có sẵn nhóm tài nguyên bản đồ.",
          "Ở **Đặt khung bản đồ của xã**, nhập vĩ độ tâm, kinh độ tâm và bán kính, rồi bấm **Lưu khung bản đồ**.",
          "Thêm từng địa điểm trên bản đồ. Nút **Mẫu Excel** và **Nhập Excel** chưa dùng được.",
        ],
      },
      {
        kind: "bullets",
        items: [
          "Chuyển giữa chế độ **Bản đồ** và **Sổ địa điểm**; bấm **Trình chiếu** để chiếu toàn màn hình.",
          "Bật, tắt từng lớp bản đồ; lọc theo tên, mã số thuế, ngành nghề, thôn, tổ dân phố. Bộ lọc **Quy mô lao động** chưa dùng được.",
          "Chấm nhạt là điểm **chưa xác minh**.",
        ],
      },
    ],
  },
  {
    id: "mini-app",
    title: "Nội dung Mini App",
    routes: ["/mini-app"],
    lede: "Tin tức, sự kiện, thông báo, bản tin truyền thanh và video hiển thị cho bà con trên Zalo Mini App của xã.",
    actions: [
      {
        label: "Thêm nội dung",
        href: createHref("/mini-app?tab=noi-dung"),
        route: "/mini-app",
        permission: CONTENT_UPDATE_PERMISSION,
        primary: true,
      },
      // Each tab of `/mini-app` keeps its own read key (`mini-app-tabs.ts`); the link follows the tab's key.
      { label: "Cảnh báo thiên tai", href: "/mini-app?tab=canh-bao", route: "/mini-app", permission: QUYEN_XEM_NOI_DUNG },
      { label: "Danh bạ cán bộ", href: "/mini-app?tab=danh-ba", route: "/mini-app", permission: QUYEN_QUAN_LY_NGUOI_DUNG },
    ],
    blocks: [
      {
        kind: "steps",
        items: [
          "Chọn loại: **Tin tức**, **Sự kiện**, **Thông báo**, **Truyền thanh**, **Video** hoặc **Banner**.",
          "Bấm **Thêm nội dung**, nhập tiêu đề, chuyên mục, ảnh hoặc tệp đính kèm rồi đăng.",
          "Quản lý chuyên mục bằng nút **Danh mục** của loại đang xem.",
        ],
      },
      {
        kind: "bullets",
        items: [
          "**Đồng bộ tin từ Cổng thông tin điện tử**: bấm **Nối Cổng thông tin** để khỏi phải đăng tin hai lần.",
          "Thẻ **Cảnh báo thiên tai**: đăng cảnh báo khẩn tới người dân qua Mini App.",
          "Thẻ **Danh bạ cán bộ**: chọn cán bộ có số điện thoại hiện cho bà con trên Mini App.",
        ],
      },
    ],
  },
  {
    id: "bao-cao",
    title: "Báo cáo",
    routes: ["/bao-cao"],
    lede: "Báo cáo điều hành tổng hợp các chỉ số, có so sánh với kỳ trước.",
    actions: [{ label: "Mở Báo cáo", href: "/bao-cao", route: "/bao-cao", primary: true }],
    blocks: [
      {
        kind: "steps",
        items: [
          "Chọn kỳ: Tuần này, Tháng này, Quý này, Năm nay hoặc **Tuỳ chọn** khoảng ngày.",
          "Kiểm tra số liệu từng khối; dòng chữ nhỏ dưới mỗi số cho biết kỳ trước và thời điểm tính.",
          "Xuất **PDF**, **XLSX** hoặc **PPTX** để gửi hoặc trình chiếu.",
        ],
      },
    ],
  },
  {
    id: "nguoi-dung",
    title: "Người dùng & Phân quyền",
    routes: ["/nguoi-dung", "/nguoi-dung/phan-quyen"],
    lede: "Tài khoản cán bộ của đơn vị: bộ phận công tác, vai trò được gán và trạng thái hoạt động.",
    actions: [
      { label: "Thêm cán bộ", href: createHref("/nguoi-dung"), route: "/nguoi-dung", permission: QUYEN_QUAN_LY_NGUOI_DUNG, primary: true },
      { label: "Phân quyền", href: "/nguoi-dung/phan-quyen", route: "/nguoi-dung/phan-quyen" },
    ],
    blocks: [
      {
        kind: "steps",
        title: "Người dùng",
        items: [
          "Bấm **Thêm cán bộ** (hoặc **Nhập từ Excel** cho nhiều người).",
          "Nhập họ tên, thư điện tử công vụ, chức danh, bộ phận, số điện thoại và tích các vai trò. Một cán bộ giữ được **nhiều vai trò**; quyền của người đó là tổng quyền của các vai trò.",
          "Ở menu **⋯** cuối dòng: **Cấp tài khoản**, **Đặt lại mật khẩu**, **Khoá** hoặc **Mở khoá** tài khoản.",
        ],
      },
      {
        kind: "note",
        tone: "warn",
        text: "Mật khẩu tạm **chỉ hiện một lần**. Hãy chép và chuyển cho cán bộ ngay; để mất thì phải đặt lại mật khẩu, lần đặt lại sinh một mật khẩu khác.",
      },
      {
        kind: "bullets",
        title: "Phân quyền",
        items: [
          "Mỗi cột là một vai trò, mỗi dòng là một quyền. Bấm vào ô để bật hoặc tắt quyền, rồi bấm **Lưu** ở đầu cột của vai trò đó.",
          "**Thêm vai trò mẫu** để có sẵn các vai trò thường gặp, hoặc **Thêm vai trò** để tự đặt. **Đổi tên** và **Xoá vai trò** ở đầu cột.",
          "Nếu cán bộ chưa thấy quyền mới, nhờ họ đăng xuất rồi đăng nhập lại.",
        ],
      },
    ],
  },
  {
    id: "cau-hinh",
    title: "Cấu hình",
    routes: ["/cau-hinh"],
    lede:
      "Sơ đồ tổ chức, địa bàn dân cư, danh mục nghiệp vụ, thời hạn xử lý và các kênh liên lạc của đơn vị. Mỗi thẻ hiện theo quyền của tài khoản.",
    actions: [
      configTab("Sơ đồ tổ chức", "so-do-to-chuc"),
      configTab("Thôn / Tổ dân phố", "thon-to-dan-pho"),
      configTab("Danh mục", "danh-muc"),
      configTab("Trường bản đồ", "truong-ban-do"),
      configTab("Lời hệ thống", "loi-he-thong"),
      configTab("Thời hạn xử lý", "thoi-han-xu-ly"),
      configTab("Tự động hoá", "tu-dong-hoa"),
      configTab("Máy chủ thư", "may-chu-thu"),
      configTab("Kênh Zalo", "kenh-zalo"),
      configTab("Kênh SMS", "kenh-sms"),
      configTab("Thuộc tính danh bạ", "thuoc-tinh-danh-ba"),
    ],
    blocks: [
      {
        kind: "table",
        head: ["Thẻ", "Dùng để"],
        rows: [
          ["Sơ đồ tổ chức", "Khai báo bộ phận theo cây, có thể nhập từ Excel."],
          ["Thôn / Tổ dân phố", "Danh sách địa bàn, dùng cho phản ánh, bản đồ, danh bạ và lọc số liệu."],
          ["Danh mục", "Lĩnh vực, loại nhiệm vụ, tên các trạng thái nhiệm vụ, hạng mục kế hoạch vốn…"],
          ["Trường bản đồ", "Các trường thông tin của điểm trên bản đồ kinh tế số."],
          ["Lời hệ thống", "Nội dung tin nhắn, thư hệ thống gửi đi."],
          ["Thời hạn xử lý", "Số giờ xử lý cho từng loại việc, đơn thư, phản ánh. Có nút gieo bộ thời hạn khởi điểm; gieo xong vẫn phải rà lại."],
          ["Tự động hoá", "Các việc hệ thống tự chạy theo lịch."],
          ["Máy chủ thư", "Cấu hình gửi thư điện tử."],
          ["Kênh Zalo", "OA Zalo của xã, mẫu tin, nhắc việc qua Zalo và mã QR mở Mini App của xã (chi tiết dưới)."],
          ["Kênh SMS", "Tài khoản eSMS của xã và mẫu tin SMS (chi tiết dưới)."],
          ["Thuộc tính danh bạ", "Các ô riêng của xã trên biểu mẫu Thêm người dân (chi tiết dưới)."],
        ],
      },
      {
        kind: "bullets",
        title: "Kênh Zalo",
        items: [
          "**OA Zalo của xã**: mỗi OA một thẻ. **Kết nối thêm OA**, **Uỷ quyền lại** khi OA báo hết hạn, **Kiểm tra kết nối**, **Ngắt OA**.",
          "**Mẫu tin ZBS**: bấm **Nạp mẫu từ OA** sau khi mẫu được Zalo duyệt.",
          "**Nhắc việc qua Zalo** và **Con bot của xã**: chọn việc gì thì nhắn qua Zalo, giờ không nhắn; mục **Ai đã ghép nối** cho biết cán bộ nào đã ghép Zalo của mình. Có thể gắn **Nhóm Zalo** để bot nhắn việc của một bộ phận vào nhóm.",
          "**Mã QR mở Mini App của xã**: in hoặc chia sẻ để bà con mở Mini App.",
        ],
      },
      {
        kind: "bullets",
        title: "Kênh SMS",
        items: [
          "Xã khai được nhiều tài khoản eSMS: **Thêm tài khoản**, **Kiểm tra kết nối**, **Đặt làm mặc định**, **Ngừng dùng** (kèm lý do).",
          "Khoá truy cập đã lưu không bao giờ hiện lại; để trống ô khoá khi sửa là giữ khoá cũ.",
          "Mẫu tin SMS: bấm **Nạp mẫu từ eSMS** sau khi mẫu được nhà mạng duyệt.",
        ],
      },
      {
        kind: "bullets",
        title: "Thuộc tính danh bạ",
        items: [
          "Xem dạng **Biểu mẫu** (đúng như cán bộ thấy khi thêm người dân) hoặc **Danh sách**.",
          "Bấm **Thêm thuộc tính** hoặc dấu **+** ở chỗ muốn thêm. Kéo thả để đổi vị trí; mỗi thuộc tính chọn **Nửa hàng** hoặc **Cả hàng**.",
          "Mã thuộc tính dùng để ghép biến khi gửi tin ZNS/SMS và không đổi được sau khi tạo. Xoá thuộc tính phải ghi lý do; lựa chọn đã dùng thì **Ngừng dùng** thay vì xoá.",
        ],
      },
    ],
  },
  {
    id: "he-thong",
    title: "Hệ thống",
    routes: ["/he-thong"],
    lede: "Lịch làm việc và ngày nghỉ, nhật ký hệ thống, nhận diện của xã.",
    actions: [
      systemTab("Lịch làm việc", "lich-lam-viec"),
      systemTab("Nhật ký hệ thống", "nhat-ky-he-thong"),
      systemTab("Nhận diện xã", "nhan-dien-xa"),
    ],
    blocks: [
      {
        kind: "bullets",
        items: [
          "**Lịch làm việc** quyết định cách đếm thời hạn: hạn không chạy ngoài giờ làm, không chạy ngày nghỉ lễ, và có chạy trong ngày làm bù.",
          "Mỗi dòng là **một ca**; một ngày làm cả sáng lẫn chiều là hai dòng. Bấm **Gieo giờ làm việc mặc định** hoặc **Gieo ngày nghỉ lễ theo dương lịch** để có bộ khởi điểm, rồi rà lại cho đúng lịch của xã.",
          "**Nhật ký hệ thống**: ai đã làm gì, lúc nào.",
          "**Nhận diện xã**: **Logo xã** (hiện ở thanh bên, màn đăng nhập và Zalo Mini App) và **Banner trang quản trị**.",
        ],
      },
    ],
  },
  {
    id: "tai-khoan",
    title: "Tài khoản cá nhân",
    routes: null,
    lede: "Cài đặt của riêng tài khoản đang đăng nhập. Mở bằng cách bấm vào tên của mình ở góc phải thanh trên cùng.",
    actions: [
      { label: "Hồ sơ cá nhân", href: "/ca-nhan", route: null, primary: true },
      { label: "Đổi mật khẩu", href: "/doi-mat-khau", route: null },
    ],
    blocks: [
      {
        kind: "steps",
        title: "Nhận nhắc việc qua Zalo",
        items: [
          "Mở **Hồ sơ cá nhân**, lấy mã ghép nối.",
          "Bấm **Mở Zalo** để mở cuộc trò chuyện với bot của xã, gửi mã cho bot.",
          "Chờ vài giây, trang tự cập nhật khi ghép nối xong. Từ đó bạn nhận nhắc việc sắp đến hạn, quá hạn qua Zalo, bên cạnh chuông thông báo.",
        ],
      },
      {
        kind: "paragraph",
        text: "Xã chưa bật kênh nhắc việc qua Zalo thì chưa ghép nối được; chuông thông báo trong phần mềm vẫn hoạt động đủ.",
      },
    ],
  },
  {
    id: "hoi-dap",
    title: "Hỏi nhanh — đáp nhanh",
    routes: null,
    lede: "Những câu cán bộ hay hỏi khi mới dùng.",
    actions: [],
    blocks: [
      {
        kind: "table",
        head: ["Câu hỏi", "Trả lời"],
        rows: [
          [
            "Vì sao tôi không thấy một mục mà đồng nghiệp thấy?",
            "Mục ngoài quyền của tài khoản không hiện trên thanh bên. Nhờ quản trị viên của xã kiểm tra vai trò của bạn.",
          ],
          ["Vì sao một mục trên thanh bên bị mờ?", "Màn đó chưa được dựng, chưa bấm được."],
          ["Ô hiện “—” là bằng 0 à?", "Không. “—” nghĩa là chưa có số liệu."],
          ["Không sửa được kết luận họp?", "Kết luận đã tách thành nhiệm vụ bị khoá. Xử lý nhiệm vụ đó trước."],
          [
            "Hạn xử lý tính thế nào?",
            "Theo giờ làm việc ở Hệ thống › Lịch làm việc, cộng số giờ ở Cấu hình › Thời hạn xử lý.",
          ],
          ["Số giải ngân có thay sổ kế toán được không?", "Không. Phần mềm chỉ phục vụ theo dõi, điều hành; không đối chiếu với Kho bạc."],
          ["Đổi quyền rồi mà cán bộ chưa thấy?", "Nhờ cán bộ đăng xuất rồi đăng nhập lại."],
          [
            "Muốn tìm nhanh một việc, một văn bản?",
            "Ô tìm kiếm chung trên thanh trên cùng chưa dùng được. Mở màn tương ứng và dùng bộ lọc của sổ đó.",
          ],
        ],
      },
    ],
  },
];
