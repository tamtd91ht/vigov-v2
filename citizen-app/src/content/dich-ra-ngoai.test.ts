/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import {
  cauKhaiDichRaNgoai,
  DICH_MO_RA_NGOAI,
  DUONG_DAN_CHAT_OA,
  OA_NEN_TANG_ID,
  OA_NEN_TANG_NGUON,
  soChu,
} from "./dich-ra-ngoai";
import { MUC_CHINH_SACH } from "./chinh-sach-rieng-tu";

/**
 * SỐ CHỖ ỨNG DỤNG MỞ MỘT TRANG BÊN NGOÀI — KHAI TRONG VĂN BẢN PHÁP LÝ, ĐẾM TỪ MÃ NGUỒN.
 *
 * ⚠ LỚP KHUYẾT TẬT TỆP NÀY TỒN TẠI ĐỂ CHẶN, và nó đã xảy ra bốn lần trong hai ngày:
 *
 *   Chính sách quyền riêng tư khai *"Có <N> chỗ ứng dụng mở một trang bên ngoài"* rồi liệt kê đủ
 *   N chỗ. Ai đó thêm một nút ở một tệp khác — một liên kết tin tức, một nút chat — và câu ấy
 *   đứng yên. Không màn hình nào vỡ, không test nào đỏ, bản dựng vẫn xanh: chỉ có một VĂN BẢN
 *   PHÁP LÝ SẮP NỘP đang khai thiếu một nơi dữ liệu người dùng có thể đi tới. Đó là đúng thứ Nghị
 *   định 13/2023/NĐ-CP nhắm tới, và là thứ không sửa lại được sau khi công bố.
 *
 *   Cả bốn lần trước đều do một NGƯỜI đọc lại văn bản mà phát hiện. Tệp này thay cho người ấy.
 *
 * CƠ CHẾ HAI VẾ, VÀ CẢ HAI PHẢI ĐỨNG THÌ MỚI KHOÁ ĐƯỢC:
 *
 *   1. Câu trong chính sách được **DỰNG RA** từ `DICH_MO_RA_NGOAI` (`cauKhaiDichRaNgoai`), nên số
 *      đếm và danh sách là cùng một mảng đọc hai lần — chúng không lệch nhau được.
 *   2. Mã nguồn **không có đường nào** mở một trang ngoài mà không gọi tên một `ma` trong danh
 *      sách ấy: `moRaNgoai` là cửa duy nhất, và mọi hình dạng đi vòng bị cấm ở đây.
 *
 *   Thiếu vế 2 thì vế 1 chỉ là một cách viết đẹp: người ta vẫn thêm được một `<a target="_blank">`
 *   ở một màn nào đó và văn bản vẫn nói "năm".
 */

const RAW_SOURCES = import.meta.glob("../**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/**
 * Bỏ chú thích trước khi quét — cùng lý do và cùng biểu thức với hai dây bẫy kia: chính những tệp
 * này GIẢI THÍCH bằng văn xuôi thứ chúng cấm (`NutWebsite.tsx` kể lại vì sao nó thôi dùng
 * `target="_blank"`), và quét văn bản thô thì lời giải thích vi phạm đúng luật nó mô tả.
 */
function boChuThich(ma: string): string {
  return ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
}

type TepNguon = { path: string; code: string };

const TEP_SAN_XUAT: readonly TepNguon[] = Object.entries(RAW_SOURCES)
  .filter(([path]) => !path.includes(".test."))
  .map(([path, code]) => ({ path: path.replace(/^\.\.\//, "./"), code: boChuThich(code) }));

/**
 * HAI TỆP ĐƯỢC PHÉP NHẮC `moTrangWeb` — tệp KHAI nó, và tệp BỌC nó.
 *
 * Miễn theo TỆP chứ không theo thư mục, cùng kỷ luật với `TEP_GOI_MANG` trong
 * `phase1-collects-nothing.test.ts`: một ngoại lệ theo thư mục thì lời gọi thứ hai, thứ ba mọc
 * lên bên cạnh mà không có gì đỏ lên.
 */
const TEP_DUOC_GOI_MO_TRANG = [
  "./features/tinh-nang/zalo-api.ts",
  "./features/tinh-nang/mo-ra-ngoai.ts",
];

/** Mọi hình dạng đưa người dùng ra khỏi app. Mỗi dòng là một đường đi vòng đã đóng. */
const LOI_RA: readonly { ten: string; mau: RegExp; mien?: readonly string[] }[] = [
  {
    ten: "`moTrangWeb(` gọi thẳng, không qua cửa khai báo",
    mau: /\bmoTrangWeb\s*\(/,
    mien: TEP_DUOC_GOI_MO_TRANG,
  },
  // `target="_blank"` mở một trang mà bên trong Zalo KHÔNG có đường quay lại Mini App — và nó là
  // hình dạng mà `moRaNgoai` không nhìn thấy, tức một lối ra không đếm được.
  { ten: 'một neo `target="_blank"`', mau: /target\s*=\s*[{"']?\s*["']_blank/ },
  { ten: "`window.open(`", mau: /\bwindow\s*\.\s*open\s*\(/ },
  {
    ten: "gán `location.href` / `location.assign` / `location.replace`",
    mau: /\blocation\s*\.\s*(?:href\s*=|assign\s*\(|replace\s*\()/,
  },
];

function viPham(loi_ra: (typeof LOI_RA)[number], tep: readonly TepNguon[]): string[] {
  return tep
    .filter((f) => loi_ra.mau.test(f.code))
    .filter((f) => !(loi_ra.mien ?? []).includes(f.path))
    .map((f) => f.path);
}

/** Mã đích đến gọi ra trong mã nguồn: `moRaNgoai("ban-do", …)`. */
function maDaGoiTrongMa(tep: readonly TepNguon[]): string[] {
  const ra = new Set<string>();
  for (const f of tep) {
    for (const khop of f.code.matchAll(/\bmoRaNgoai\s*\(\s*["'`]([^"'`]+)["'`]/g)) {
      ra.add(khop[1]!);
    }
  }
  return [...ra].sort();
}

describe("mọi lối ra ngoài đều đi qua đúng một cửa, và cửa ấy khai tên", () => {
  it("quét cây mã THẬT — một lượt quét rỗng sẽ xanh vì lý do sai", () => {
    const duong_dan = TEP_SAN_XUAT.map((f) => f.path);
    expect(duong_dan).toContain("./features/tinh-nang/mo-ra-ngoai.ts");
    expect(duong_dan).toContain("./features/tinh-nang/zalo-api.ts");
    expect(duong_dan).toContain("./features/company-intro/NutWebsite.tsx");
    expect(duong_dan.length).toBeGreaterThanOrEqual(20);
  });

  for (const loi_ra of LOI_RA) {
    it(`không tệp nào còn ${loi_ra.ten}`, () => {
      expect(
        viPham(loi_ra, TEP_SAN_XUAT),
        `${loi_ra.ten}.\nMọi lối ra ngoài đi qua \`moRaNgoai(<mã đích>, …)\` — xem ` +
          "`features/tinh-nang/mo-ra-ngoai.ts`. Lý do không phải kiến trúc cho đẹp: chính sách " +
          "quyền riêng tư ĐẾM số chỗ ứng dụng mở trang ngoài, và một lối ra mà cửa ấy không thấy " +
          "là một dòng thiếu trong một văn bản pháp lý sắp nộp.",
      ).toEqual([]);
    });
  }

  it("mọi mã đích gọi trong mã nguồn đều ĐÃ KHAI, và mọi mã đã khai đều CÓ NGƯỜI GỌI", () => {
    const trong_ma = maDaGoiTrongMa(TEP_SAN_XUAT);
    // Kiểu `string`, không phải `MaDichRaNgoai`: hai mảng dưới đây được so với nhau theo CẢ HAI
    // chiều, và chiều "mã gọi trong mã nguồn" đọc ra từ văn bản nên nó là `string` — kể cả một mã
    // chưa ai khai, thứ chính ca này tồn tại để bắt.
    const da_khai = DICH_MO_RA_NGOAI.map((d): string => d.ma).sort();

    expect(
      trong_ma.filter((ma) => !da_khai.includes(ma)),
      "một đích được mở mà không có dòng khai — chính sách đang đếm thiếu",
    ).toEqual([]);
    expect(
      da_khai.filter((ma) => !trong_ma.includes(ma)),
      "một đích được khai mà không nút nào mở tới — chính sách đang khai một nơi app không đưa " +
        "người dùng tới, và người duyệt đối chiếu được điều đó",
    ).toEqual([]);
    expect(new Set(da_khai).size, "hai dòng khai cùng một mã đích").toBe(da_khai.length);
  });

  /* ------------------------------------------------------------------------------------------
     THỬ ĐỘT BIẾN DỰNG SẴN — phần trả lời câu "phép kiểm trên còn sống không".

     Cây mã hôm nay sạch ở cả bốn hình dạng, nên bốn ca trên đều xanh, và một dây bẫy xanh không
     nói lên gì. Ca dưới cho từng lệnh cấm ăn một vi phạm dựng sẵn.
     ------------------------------------------------------------------------------------------ */
  it("bắt được cả bốn hình dạng, ở một tệp không được miễn", () => {
    const VI_PHAM: readonly TepNguon[] = [
      { path: "./features/company-intro/HomeScreen.tsx", code: 'await moTrangWeb("https://vidu.vn");' },
      { path: "./App.tsx", code: '<a href="https://vidu.vn" target="_blank">Mở</a>' },
      { path: "./features/company-intro/KhoiTin.tsx", code: "<a target={'_blank'} href={u} />" },
      { path: "./components/TabBar.tsx", code: 'window.open("https://vidu.vn");' },
      { path: "./App.tsx", code: 'location.href = "https://vidu.vn";' },
      { path: "./lib/cuon-toi.ts", code: 'window.location.assign("https://vidu.vn");' },
      { path: "./main.tsx", code: 'location.replace("https://vidu.vn");' },
    ];
    for (const tep of VI_PHAM) {
      const bat = LOI_RA.some((loi_ra) => viPham(loi_ra, [tep]).length === 1);
      expect(bat, `lối ra không bị bắt: ${tep.path} — ${tep.code}`).toBe(true);
    }

    // Và KHÔNG kêu oan ở hai tệp được miễn, cũng không kêu oan ở những thứ chỉ TRÔNG giống. Một
    // dây bẫy kêu sai chỗ bị tắt nhanh y như một dây bẫy câm.
    for (const tep of [
      { path: "./features/tinh-nang/mo-ra-ngoai.ts", code: "const kq = await moTrangWeb(duong_dan);" },
      { path: "./features/tinh-nang/zalo-api.ts", code: "export function moTrangWeb(d: string) {}" },
      { path: "./App.tsx", code: '<a href="tel:0287…">Gọi</a>' },
      { path: "./App.tsx", code: "const o = document.getElementById(moc);" },
      { path: "./features/tinh-nang/SoHoaThiepGiay.tsx", code: "await chonAnhTuMay();" },
    ]) {
      for (const loi_ra of LOI_RA) {
        expect(viPham(loi_ra, [tep]), `kêu oan ở: ${tep.code}`).toEqual([]);
      }
    }
  });

  it("bắt được một mã đích CHƯA KHAI, kể cả khi nó đi qua đúng cửa", () => {
    // Kiểu `MaDichRaNgoai` chặn ở tầng biên dịch, nhưng nó chặn được chừng nào không ai nới kiểu
    // ấy hay ép kiểu tại chỗ gọi. Ca này là lớp thứ hai, và nó đọc mã nguồn chứ đọc kiểu.
    const trom = [
      { path: "./features/company-intro/HomeScreen.tsx", code: 'moRaNgoai("facebook", dia_chi);' },
    ];
    expect(maDaGoiTrongMa(trom)).toEqual(["facebook"]);
    expect(DICH_MO_RA_NGOAI.map((d) => d.ma)).not.toContain("facebook");
  });
});

/**
 * CÂU KHAI TRONG CHÍNH SÁCH — dựng từ danh sách, và ca dưới đối chiếu CẢ HAI CHIỀU.
 */
describe("câu khai của chính sách khớp với danh sách đích", () => {
  const benThuBa = () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "ben-thu-ba");
    expect(muc, "chính sách không còn mục nào về chuyển dữ liệu cho bên thứ ba").toBeDefined();
    return muc!.doan;
  };

  it("nói ĐÚNG số chỗ, bằng chữ, và số ấy bằng số dòng đã khai", () => {
    const chu = benThuBa().join("\n");
    expect(chu, "văn bản không còn câu đếm số chỗ mở trang ngoài").toContain(
      `Có ${soChu(DICH_MO_RA_NGOAI.length)} chỗ ứng dụng mở một trang bên ngoài`,
    );
  });

  it("liệt kê ĐỦ từng đích, nguyên văn cụm từ đã khai", () => {
    const chu = benThuBa().join("\n");
    for (const dich of DICH_MO_RA_NGOAI) {
      expect(chu, `văn bản không kể tới đích "${dich.ma}"`).toContain(dich.trong_chinh_sach);
    }
  });

  it("câu ấy xuất hiện ĐÚNG MỘT LẦN trong cả văn bản", () => {
    // Hai câu đếm trong một văn bản là hai câu sẽ nói hai con số khác nhau.
    const toan_van = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    const so_lan = toan_van.split("chỗ ứng dụng mở một trang bên ngoài").length - 1;
    expect(so_lan, `câu đếm xuất hiện ${so_lan} lần`).toBe(1);
  });

  it("đổi danh sách thì câu đổi theo — con số không thể đứng yên", () => {
    // ĐÂY LÀ CA CHỨNG MINH CƠ CHẾ CÒN SỐNG, không phải một ca về nội dung. Nếu ai đó gỡ
    // `cauKhaiDichRaNgoai()` khỏi chính sách và gõ lại một câu tay, ba ca trên vẫn xanh chừng nào
    // câu gõ tay còn khớp hôm nay — và nó sẽ đứng yên vào lần thêm đích tiếp theo.
    const bot_mot = DICH_MO_RA_NGOAI.slice(0, DICH_MO_RA_NGOAI.length - 1);
    const cau_bot = cauKhaiDichRaNgoai(bot_mot);
    expect(cau_bot).toContain(`Có ${soChu(bot_mot.length)} chỗ`);
    expect(cau_bot, "câu vẫn kể một đích đã bị gỡ khỏi danh sách").not.toContain(
      DICH_MO_RA_NGOAI[DICH_MO_RA_NGOAI.length - 1]!.trong_chinh_sach,
    );

    // Danh sách một dòng: phần liệt kê phải là ĐÚNG cụm từ ấy rồi hết câu — không còn một "và"
    // treo lủng lẳng, thứ một hàm ghép chuỗi hay để lại ở trường hợp biên.
    const mot_minh = cauKhaiDichRaNgoai([DICH_MO_RA_NGOAI[0]!]);
    expect(mot_minh, "danh sách một dòng để lại một liên từ treo").toContain(
      `bấm: ${DICH_MO_RA_NGOAI[0]!.trong_chinh_sach}. Ứng dụng`,
    );
  });

  it("đọc số thành chữ, và không im lặng khi vượt bảng", () => {
    expect(soChu(5)).toBe("năm");
    expect(soChu(10)).toBe("mười");
    // Vượt bảng thì in chữ số: một câu đọc hơi cứng vẫn ĐÚNG, còn một ngoại lệ ném ra lúc dựng
    // văn bản là hồ sơ không sinh được.
    expect(soChu(11)).toBe("11");
  });
});

/**
 * ĐỊA CHỈ OFFICIAL ACCOUNT — một giá trị CHƯA XÁC MINH nằm trong một bản sắp nộp.
 *
 * Ca dưới không kiểm được ID có đúng OA hay không (chỉ Developer Console trả lời được). Thứ nó
 * kiểm được, và là thứ giữ cho lời cảnh báo không bị tách khỏi giá trị: **lời khai nguồn phải còn
 * ở đó và phải nói rằng nó chưa được đối chiếu**.
 */
describe("địa chỉ Official Account — chưa xác minh, và nói ra điều đó", () => {
  it("là một chuỗi chữ số, ghép thành đúng một đường dẫn zalo.me", () => {
    expect(OA_NEN_TANG_ID).toMatch(/^\d+$/);
    expect(DUONG_DAN_CHAT_OA).toBe(`https://zalo.me/${OA_NEN_TANG_ID}`);
  });

  it("mang theo lời khai nguồn, và lời khai ấy nói CHƯA đối chiếu Console", () => {
    expect(OA_NEN_TANG_NGUON).toMatch(/[Bb]ản mẫu/);
    expect(OA_NEN_TANG_NGUON, "lời cảnh báo 'chưa đối chiếu' đã bị gỡ").toMatch(/CHƯA/);
    expect(OA_NEN_TANG_NGUON).toMatch(/Console/);
  });

  it("đích chat có mặt trong danh sách khai, và câu chính sách gọi tên Official Account", () => {
    const chat = DICH_MO_RA_NGOAI.find((d) => d.ma === "chat-oa");
    expect(chat, "nút chat mở một nơi mà chính sách không khai").toBeDefined();
    expect(chat!.trong_chinh_sach).toMatch(/Official Account/);
  });
});
