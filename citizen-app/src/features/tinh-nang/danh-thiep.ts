/**
 * BỘ BÓC TÁCH NỘI DUNG MÃ QR — phần logic thật duy nhất của ứng dụng này.
 *
 * ⚠ HÀM THUẦN, VÀ ĐÓ LÀ MỘT YÊU CẦU VỀ DỮ LIỆU CÁ NHÂN, KHÔNG PHẢI VỀ KIẾN TRÚC:
 *
 *   Nội dung một tấm danh thiếp là dữ liệu cá nhân của NGƯỜI KHÁC — tên, số điện thoại, email,
 *   nơi làm việc của họ. Luật 3 (Nghị định 13/2023/NĐ-CP) nói dữ liệu ấy không được vào log,
 *   không vào tên tệp, không vào URL, không vào bất kỳ chỗ lưu nào. Một bộ bóc tách THUẦN —
 *   chuỗi vào, đối tượng ra, không tác dụng phụ — làm câu ấy đúng **bằng cấu trúc**: tệp này
 *   không có chỗ nào để rò rỉ, vì nó không gọi ra ngoài lấy một lần nào.
 *
 *   Nó cũng là lý do bộ này kiểm được đầy đủ mà không cần một chiếc điện thoại: mọi ca trong
 *   `tinh-nang.test.tsx` đưa vào một chuỗi và đọc ra một đối tượng.
 *
 * VÌ SAO PHẢI BÓC TÁCH CHỨ KHÔNG HIỆN NGUYÊN CỤC CHỮ: một vCard thô là mười dòng chữ hoa có dấu
 * hai chấm. Người vừa quét danh thiếp của đối tác cần một cái tên đọc được và một nút gọi, chứ
 * không phải một khối văn bản để tự dò.
 */

/** Một trường của vCard sau khi đã gỡ gập dòng và tách tham số. */
type DongVCard = {
  /** Tên trường, đã viết hoa và đã bỏ tiền tố nhóm kiểu `item1.`. */
  ten: string;
  /** Phần sau dấu hai chấm đầu tiên, đã gỡ ký tự thoát. */
  gia_tri: string;
};

export type DanhThiep = {
  loai: "danh-thiep";
  /** `FN` nếu có; nếu không thì dựng lại từ `N`. Có thể rỗng — một vCard được phép thiếu tên. */
  ho_ten: string;
  to_chuc: string;
  chuc_danh: string;
  /** Nhiều `TEL`/`EMAIL`/`URL` là chuyện bình thường trên một tấm danh thiếp. */
  dien_thoai: readonly string[];
  email: readonly string[];
  trang_web: readonly string[];
};

export type KetQuaDoc =
  | DanhThiep
  | { loai: "lien-ket"; duong_dan: string }
  | { loai: "van-ban"; noi_dung: string }
  | { loai: "rong" };

/**
 * GỠ GẬP DÒNG (RFC 6350 §3.2).
 *
 * Một vCard dài được cắt thành nhiều dòng, và dòng tiếp theo bắt đầu bằng **một khoảng trắng**
 * hoặc một tab. Không nối lại thì một địa chỉ email dài bị cắt làm đôi và nửa sau trở thành một
 * trường tên rác — tức app hiện sai email của một người thật, và người dùng gửi thư đi đâu đó.
 */
function goGapDong(noi_dung: string): string[] {
  const dong: string[] = [];
  for (const mot of noi_dung.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n")) {
    if ((mot.startsWith(" ") || mot.startsWith("\t")) && dong.length > 0) {
      dong[dong.length - 1] += mot.slice(1);
    } else {
      dong.push(mot);
    }
  }
  return dong;
}

/**
 * Gỡ ký tự thoát của vCard: `\n` là xuống dòng, `\,` `\;` `\\` là chính ký tự ấy.
 *
 * Bỏ qua bước này thì một tên công ty có dấu phẩy hiện ra kèm dấu gạch chéo ngược — sai chính
 * tên của một pháp nhân, ngay trên màn hình.
 */
function goThoat(gia_tri: string): string {
  return gia_tri.replace(/\\([nN,;\\])/g, (_, ky_tu: string) =>
    ky_tu === "n" || ky_tu === "N" ? "\n" : ky_tu,
  );
}

/**
 * Tách một dòng thành tên trường và giá trị.
 *
 * Phần tên có thể mang tham số (`TEL;TYPE=CELL,VOICE:…`) và có thể mang tiền tố nhóm của Apple
 * (`item1.TEL:…`). Cả hai đều bị bỏ đi: cái app cần là "đây là một số điện thoại", không phải
 * loại máy. Cắt ở dấu hai chấm ĐẦU TIÊN, vì giá trị có quyền chứa dấu hai chấm (`URL:https://…`).
 */
function tachDong(dong: string): DongVCard | null {
  const cho_hai_cham = dong.indexOf(":");
  if (cho_hai_cham < 0) return null;

  const phan_ten = dong.slice(0, cho_hai_cham);
  const gia_tri = goThoat(dong.slice(cho_hai_cham + 1).trim());

  const ten_co_nhom = phan_ten.split(";")[0] ?? "";
  const ten = (ten_co_nhom.includes(".") ? ten_co_nhom.slice(ten_co_nhom.indexOf(".") + 1) : ten_co_nhom)
    .trim()
    .toUpperCase();

  if (ten === "") return null;
  return { ten, gia_tri };
}

/**
 * Dựng lại họ tên từ trường `N` — `N:Họ;Tên;Đệm;Tiền tố;Hậu tố`.
 *
 * THỨ TỰ LÀ THỨ TỰ TIẾNG VIỆT: họ trước, đệm, rồi tên. Xếp theo lối Anh ngữ ("An Nguyễn") là
 * gọi sai tên một người ngay ở dòng to nhất màn hình — và đây là app của một công ty Việt Nam,
 * dùng ở Việt Nam. `FN` khi có mặt luôn thắng, vì đó là tên chủ thẻ tự viết ra.
 */
function hoTenTuN(gia_tri: string): string {
  const phan = gia_tri.split(";").map((mot) => mot.trim());
  const [ho = "", ten = "", dem = "", tien_to = "", hau_to = ""] = phan;
  return [tien_to, ho, dem, ten, hau_to].filter((mot) => mot !== "").join(" ");
}

const LA_LIEN_KET = /^https?:\/\//i;

/**
 * Đọc nội dung một mã QR và nói ra nó là thứ gì.
 *
 * Ba nhánh, theo đúng thứ tự chắc chắn giảm dần: vCard (có dấu hiệu mở đầu rõ ràng) → liên kết
 * (có giao thức) → văn bản thuần (mọi thứ còn lại). Không đoán thêm nhánh nào nữa: đoán sai một
 * mã quét được nghĩa là giấu mất nội dung thật của nó khỏi người vừa quét.
 */
export function docMaQR(noi_dung: string): KetQuaDoc {
  const goc = noi_dung.trim();
  if (goc === "") return { loai: "rong" };

  if (/^BEGIN:VCARD/i.test(goc)) return docVCard(goc);
  if (LA_LIEN_KET.test(goc)) return { loai: "lien-ket", duong_dan: goc };
  return { loai: "van-ban", noi_dung: goc };
}

/**
 * Bóc một vCard thành các trường ứng dụng hiện ra.
 *
 * THIẾU TRƯỜNG LÀ CHUYỆN BÌNH THƯỜNG, KHÔNG PHẢI LỖI: rất nhiều danh thiếp không có chức danh,
 * không có website, có khi không có cả tên. Trả về chuỗi rỗng và mảng rỗng để chỗ vẽ chỉ việc bỏ
 * qua — ném lỗi ở đây là biến một tấm thiếp thiếu thông tin thành một màn hình hỏng.
 */
function docVCard(goc: string): DanhThiep {
  let ho_ten = "";
  let ho_ten_tu_n = "";
  let to_chuc = "";
  let chuc_danh = "";
  const dien_thoai: string[] = [];
  const email: string[] = [];
  const trang_web: string[] = [];

  for (const dong of goGapDong(goc)) {
    const truong = tachDong(dong);
    if (truong === null || truong.gia_tri === "") continue;

    switch (truong.ten) {
      case "FN":
        if (ho_ten === "") ho_ten = truong.gia_tri;
        break;
      case "N":
        if (ho_ten_tu_n === "") ho_ten_tu_n = hoTenTuN(truong.gia_tri);
        break;
      case "ORG":
        // `ORG` phân cấp bằng dấu chấm phẩy (công ty;phòng ban). Nối lại bằng dấu phẩy để đọc
        // được, thay vì hiện ra một dấu chấm phẩy giữa tên công ty.
        if (to_chuc === "")
          to_chuc = truong.gia_tri
            .split(";")
            .map((mot) => mot.trim())
            .filter((mot) => mot !== "")
            .join(", ");
        break;
      case "TITLE":
      case "ROLE":
        if (chuc_danh === "") chuc_danh = truong.gia_tri;
        break;
      case "TEL":
        dien_thoai.push(truong.gia_tri);
        break;
      case "EMAIL":
        email.push(truong.gia_tri);
        break;
      case "URL":
        trang_web.push(truong.gia_tri);
        break;
      default:
        // Mọi trường khác (`VERSION`, `ADR`, `NOTE`, `PHOTO`, `BEGIN`, `END`…) không được hiện.
        // Hiện tất cả là hiện lại đúng cục chữ thô mà màn hình này sinh ra để thay thế.
        break;
    }
  }

  return {
    loai: "danh-thiep",
    ho_ten: ho_ten !== "" ? ho_ten : ho_ten_tu_n,
    to_chuc,
    chuc_danh,
    dien_thoai,
    email,
    trang_web,
  };
}

/**
 * Một tấm thiếp có gì để hiện hay không.
 *
 * `BEGIN:VCARD` rồi `END:VCARD` mà không có trường nào là một mã hợp lệ nhưng rỗng ruột. Chỗ vẽ
 * đọc hàm này để nói ra điều đó bằng một câu, thay vì vẽ một tấm thiếp trắng.
 */
export function thiepCoNoiDung(thiep: DanhThiep): boolean {
  return (
    thiep.ho_ten !== "" ||
    thiep.to_chuc !== "" ||
    thiep.chuc_danh !== "" ||
    thiep.dien_thoai.length > 0 ||
    thiep.email.length > 0 ||
    thiep.trang_web.length > 0
  );
}
