/**
 * Câu chữ của tab "Thời hạn xử lý và lịch làm việc" (`docs/ui-ux/14-cau-hinh.md §8`). Hàm thuần:
 * không gọi mạng, không dựng DOM.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * NGƯỜI ĐỌC MÀN HÌNH NÀY LÀ CÁN BỘ VĂN PHÒNG HOẶC CHỦ TỊCH XÃ, mở đúng một lần khi mới nhận hệ
 * thống rồi thỉnh thoảng sửa. Không ai trong số đó quen chữ "SLA", nên trên màn hình không có chữ
 * ấy: "Thời hạn xử lý", "Giờ làm việc", "Ngày nghỉ lễ", "Ngày làm bù".
 *
 * ĐƠN VỊ "GIỜ LÀM VIỆC" là sự thật nặng nhất về những con số này (ADR 0007, luật 10 bất biến 4): 8
 * giờ làm việc là hơn một ngày làm, không phải một buổi tối. Since ADR 0079 (spec 08) the cells say
 * "{n} giờ" and the unit is stated ONCE, in bold, in the banner directly above the table
 * (`SLA_BANNER_UNIT`) — so the banner sentence is what must never be dropped from this tab.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG MỘT PHÉP TÍNH HẠN NÀO Ở ĐÂY, và không được thêm vào. `identity` sở hữu bảng thời hạn, ba
 * bảng lịch, và phép cộng giờ làm việc. Tệp này chỉ ĐẶT TÊN cho những gì máy chủ đã trả về.
 */

/* ---- khối "đơn vị chưa khai xong" ---------------------------------------------------------- */

/**
 * Tiêu đề khối cảnh báo. Nói "chưa khai xong" chứ không "thiếu dữ liệu": thiếu dữ liệu nghe như
 * một bảng hiện chưa đẹp, còn chưa khai xong nói đúng rằng có một việc đang chờ người làm.
 */
export const KHOI_CHUA_KHAI_TIEU_DE = "Đơn vị chưa khai xong phần bắt buộc";

/**
 * CÂU QUAN TRỌNG NHẤT TRÊN CẢ MÀN HÌNH — nó nói HẬU QUẢ, không nói tình trạng.
 *
 * Một câu kiểu "chưa có dữ liệu thời hạn" đọc ra là một việc để hôm khác làm. Sự thật là xã đang
 * KHÔNG vào sổ được văn bản đến và KHÔNG nhận được phản ánh nào, và cán bộ sẽ gặp lỗi ấy ở một màn
 * hình khác hẳn màn hình sửa được nó. Câu này là thứ thay cho một bước hướng dẫn ban đầu mà hệ
 * thống không có.
 */
export const KHOI_CHUA_KHAI_HAU_QUA =
  "Cho tới khi khai xong, phần mềm chưa vào sổ được văn bản đến và chưa nhận được phản ánh của " +
  "người dân: mỗi lần tiếp nhận đều bị từ chối vì không tính được thời hạn xử lý. Khai xong là " +
  "tiếp nhận được ngay, không phải chờ ai duyệt.";

export const KHOI_CHUA_KHAI_THIEU_THOI_HAN =
  "Bảng Thời hạn xử lý đang trống — chưa có dòng nào nói mỗi loại việc phải xong trong bao nhiêu " +
  "giờ làm việc.";

export const KHOI_CHUA_KHAI_THIEU_LICH_TUAN =
  "Giờ làm việc trong tuần đang trống — chưa có ca nào, nên không đếm được một giờ làm việc nào.";

/**
 * Nhãn nút gieo thời hạn. **PHẢI ĐÚNG CHUỖI NÀY**: câu `problems` của máy chủ gọi đích danh nó —
 * *"hãy bấm “Gieo thời hạn mặc định”"* (`service-identity/internal/http/sla.go`). Đặt tên khác đi
 * là để máy chủ chỉ vào một cái nút không tồn tại trên màn hình.
 */
export const NUT_GIEO_THOI_HAN = "Gieo thời hạn mặc định";

export const NUT_GIEO_TUAN = "Gieo giờ làm việc mặc định";

export const NUT_GIEO_NGAY_LE = "Gieo ngày nghỉ lễ theo dương lịch";

/**
 * Gieo xong KHÔNG phải là xong, và câu này đứng ngay dưới hai nút để không ai đọc "đã gieo" thành
 * "đã đúng". Bộ số gieo là điểm khởi đầu của phần mềm; quy định thật là của đơn vị.
 */
export const GHI_CHU_SAU_KHI_GIEO =
  "Gieo xong vẫn phải rà lại: đây là bộ số khởi điểm của phần mềm, không phải quy định của đơn " +
  "vị. Bấm lại lần nữa không ghi đè con số đơn vị đã sửa.";

/* ---- bảng thời hạn xử lý -------------------------------------------------------------------- */

/**
 * THE BANNER ABOVE THE TABLE (spec 08 "Hộp giải thích"), split where the spec sets words in bold.
 *
 * Every clause was checked against the code before it was kept (08/10/2026):
 *
 * - `SLA_BANNER_UNIT` / `_HOLIDAYS`: `identity.AdvanceWorkingHours` counts working hours only (ADR 0007).
 * - `SLA_BANNER_APPLIES`: the spec's "Thay đổi chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu" is
 *   NOT true here and was reworded. A deadline is fixed at the act that fixes it and stored (rule 10
 *   invariant 2), and for a citizen's petition `han_xu_ly_xong` is fixed when staff settle the field,
 *   not on receipt (ADR 0028 decision E) — so a petition received BEFORE the save but classified after
 *   it takes the new figure. What is true for every record: a deadline already set never moves.
 * - `SLA_BANNER_DUE_SOON_*`: the three uses are real — the reminder digest (`slaReminders` →
 *   `ResolveDueSoonCutoff`, `service-petitions/internal/app/automation_jobs.go`), the task register's
 *   `soon=true` filter (`service-petitions/internal/http/nhiem_vu.go`), and the bell's "Sắp đến hạn"
 *   notice. The spec's closing "Mặc định 72 giờ, tức ba ngày" is DROPPED: an SLA figure written into
 *   source, false for most rows (rule 10 forbidden #3, ADR 0079).
 */
export const SLA_BANNER_LEAD = "Thời hạn tính theo ";
export const SLA_BANNER_UNIT = "giờ làm việc";
export const SLA_BANNER_HOLIDAYS = ", không tính ngày nghỉ và ngày lễ. ";
export const SLA_BANNER_APPLIES =
  "Thay đổi chỉ áp dụng cho hạn đặt sau thời điểm lưu; hạn đã đặt cho hồ sơ giữ nguyên.";
export const SLA_BANNER_DUE_SOON_LEAD = "Cột ";
export const SLA_BANNER_DUE_SOON_COLUMN = "Sắp đến hạn khi còn";
export const SLA_BANNER_DUE_SOON_REST =
  " quyết định cả ba: lúc nào gửi lời nhắc, ô lọc “Sắp đến hạn” trên màn nhiệm vụ lấy ra việc nào, " +
  "và con số trong thông báo ở chuông.";

/*
 * NO NOTE UNDER THE TABLE (owner, 08/10/2026: "Bỏ hết, đúng prototype"). The former note explained
 * what the reporting columns count from and that an empty sixth box means "không báo"; the prototype
 * has no such line, so the tab says neither. (Its old closing claim, "CHƯA tự gửi lời báo nào", had
 * also become false: escalation and the unassigned-hold check run in `AutomationRunner`, ADR 0058.)
 */

/** Column heading of the sixth figure — the owner's name for it (ADR 0079 decision 3). */
export const UNASSIGNED_HOLD_LABEL = "Giữ chưa phân công";

/**
 * Accessible text of an unset sixth figure, behind the visible "—". NULL is the commune's choice,
 * never replaced by a number, and "—" alone would read as "no data".
 */
export const UNASSIGNED_HOLD_OFF = "Không báo";

/**
 * Local refusal mirroring the server's ordering rule (`domain.ErrChairmanBeforeUnitHead`). The
 * server still checks and its sentence still wins if the two ever disagree.
 */
export const PRESIDENT_BELOW_LEADER_ERROR =
  "Số giờ Báo Chủ tịch không được nhỏ hơn số giờ Báo lãnh đạo trực tiếp: Chủ tịch được báo cùng " +
  "lúc hoặc sau lãnh đạo trực tiếp, không bao giờ trước.";

/**
 * Mã loại việc → tên đọc được. Bốn mã là đúng bốn giá trị ràng buộc CHECK của CSDL nhận
 * (`sla_loai_viec_hop_le`; `don-thu` từ migration 0016 của `service-identity`).
 *
 * MÃ LẠ THÌ HIỆN NGUYÊN MÃ, không hiện một tên đoán ra: một loại việc lạ trong bảng nghĩa là CSDL
 * không còn đúng lược đồ mà mã này được dựng theo, và che nó bằng một cái tên đẹp là xoá đúng dấu
 * vết người vận hành cần.
 */
export function nhanLoaiViec(ma: string): string {
  switch (ma) {
    case "van-ban-den":
      return "Văn bản đến";
    case "don-thu":
      return "Đơn thư";
    case "phan-anh":
      return "Phản ánh của người dân";
    case "nhiem-vu":
      return "Nhiệm vụ";
    default:
      return ma;
  }
}

/**
 * Cột Lĩnh vực: dòng mặc định nói rõ nó là mặc định; dòng có lĩnh vực hiện **nhãn của xã**, lấy từ
 * `labels` — bảng `petitions` trả về (`GET /api/v1/citizen-report-fields`, đọc một lần cho cả màn).
 *
 * KHÔNG CÓ BẢNG TRA GÕ TAY Ở ĐÂY: nhãn là của danh mục xã, một bản chép trong tệp này trôi mà không
 * bài test nào đỏ (luật 9, cấm #2). Mã không có trong `labels` — đọc hỏng, tài khoản thiếu
 * `admin.lookup`, hay một mã danh mục không còn biết — thì hiện NGUYÊN MÃ, không đoán một tên.
 */
export function nhanLinhVuc(
  linhVuc: string,
  laMacDinh: boolean,
  labels: ReadonlyMap<string, string> = new Map(),
): string {
  if (laMacDinh) return "Mặc định cho mọi lĩnh vực";
  const label = labels.get(linhVuc);
  return label !== undefined && label !== "" ? label : linhVuc;
}

/**
 * A deadline cell: "{n} giờ" (spec 08). The unit "giờ làm việc" is stated in bold in the banner
 * directly above the table, not repeated per cell — see the file header.
 */
export function hoursCell(hours: number): string {
  return `${hours} giờ`;
}

/**
 * A reporting-threshold cell: "sau {n} giờ" — it counts from a moment (a missed deadline, a hold),
 * not from receipt. `null` (only the sixth figure can be unset) has no sentence here: the cell draws
 * "—" with `UNASSIGNED_HOLD_OFF` for a screen reader.
 */
export function afterHoursCell(hours: number): string {
  return `sau ${hours} giờ`;
}

/* ---- kết quả các lượt gieo ------------------------------------------------------------------ */

/**
 * Câu báo sau khi gieo bảng thời hạn. `kept` được nói ra chứ không nuốt: nó là lời cam đoan rằng
 * con số đơn vị đã sửa không bị đụng tới, và đó là điều người bấm nút lần thứ hai đang lo.
 */
export function cauGieoThoiHan(daGieo: number, giuNguyen: number): string {
  return (
    `Đã thêm ${daGieo} dòng thời hạn xử lý. Giữ nguyên ${giuNguyen} dòng đơn vị đã có, kể cả con ` +
    "số đơn vị đã sửa."
  );
}

/** Câu báo sau khi gieo giờ làm việc. `skipped` chỉ nói ra khi có — xem `cauBoQua`. */
export function cauGieoTuan(daGieo: number, giuNguyen: number, boQua: number): string {
  return (
    `Đã thêm ${daGieo} ca làm việc. Giữ nguyên ${giuNguyen} ca đơn vị đã có.` + cauBoQua(boQua)
  );
}

export function cauGieoNgayLe(nam: number, daGieo: number, giuNguyen: number, boQua: number): string {
  return (
    `Năm ${nam}: đã thêm ${daGieo} ngày nghỉ lễ theo dương lịch. Giữ nguyên ${giuNguyen} ngày đơn ` +
    `vị đã có.` + cauBoQua(boQua)
  );
}

/**
 * `skipped` KHÁC `kept`, và hai chữ ấy nợ người đọc hai câu khác nhau: `kept` là "đơn vị đã có
 * rồi", `skipped` là "chúng tôi KHÔNG ghi, vì ghi vào sẽ làm hỏng lịch — hãy đi xem vì sao". Gộp
 * hai con số lại là giấu đúng con số cần người xem.
 *
 * KHÔNG CÓ CÂU NÀO KHI `skipped = 0`: một dòng "bỏ qua 0" là một dòng báo động giả, và báo động
 * giả lặp lại làm hỏng mọi báo động thật trên cùng màn hình.
 */
export function cauBoQua(boQua: number): string {
  if (boQua <= 0) return "";
  return (
    ` Bỏ qua ${boQua} dòng vì ghi vào sẽ làm lịch mâu thuẫn — hãy xem lại các dòng đang có trước ` +
    "khi gieo lại."
  );
}

/**
 * ⚠ CÂU NÀY LÀ ĐIỀU KIỆN ĐỂ NÚT GIEO NGÀY LỄ KHÔNG NÓI DỐI, và nó phải hiện ngay cạnh kết quả.
 *
 * Tuyến chỉ gieo BỐN ngày nghỉ cố định theo dương lịch, và `seeded: 4` đứng một mình đọc ra là
 * "xong". Điều 112 Bộ luật Lao động 2019 có mười một ngày: Tết Nguyên đán (5 ngày) và Giỗ Tổ Hùng
 * Vương theo ÂM LỊCH, còn ngày liền kề 02/9 do Thủ tướng chọn từng năm giữa 01/9 và 03/9. Máy chủ
 * cố ý không gieo ba nhóm ấy — một phép quy đổi âm lịch tự viết sai một ngày là một hạn đếm xuyên
 * qua ngày trụ sở đóng cửa (`service-identity/internal/domain/lich_gieo.go`).
 *
 * Xã tin là đã đủ ngày lễ thì mọi thời hạn rơi vào dịp Tết bị tính sai — mà không gì báo lỗi.
 */
export const CON_THIEU_NGAY_LE =
  "Mới gieo 4 ngày nghỉ cố định theo dương lịch. Tết Nguyên đán, Giỗ Tổ Hùng Vương và ngày liền " +
  "kề 02/9 CHƯA có: hai ngày đầu tính theo âm lịch nên đổi ngày dương mỗi năm, còn ngày liền kề " +
  "02/9 do Thủ tướng chọn từng năm. Đơn vị phải tự thêm ba nhóm ngày này theo thông báo nghỉ lễ " +
  "hằng năm — thiếu chúng thì mọi thời hạn rơi vào dịp Tết đều bị tính sai.";

/* ---- lịch làm việc: nhãn ô nhập và cảnh báo ------------------------------------------------- */

export const O_THU = "Thứ trong tuần";
export const O_GIO_BAT_DAU = "Giờ bắt đầu";
export const O_GIO_KET_THUC = "Giờ kết thúc";
export const O_GHI_CHU_CA = "Ghi chú (buổi sáng, buổi chiều…)";
export const O_NGAY = "Ngày";
export const O_TEN_NGAY_NGHI = "Tên ngày nghỉ";
export const O_TEN_NGAY_LAM_BU = "Theo thông báo nào";
export const O_LY_DO_XOA = "Lý do xoá";

/**
 * Nghỉ trưa KHÔNG phải một ô đánh dấu, nó là khoảng hở giữa hai ca. Cán bộ khai một ca 07:30–17:00
 * là khai rằng xã làm việc liên tục qua trưa, và mọi hạn đi qua buổi trưa sẽ dài thêm một tiếng
 * rưỡi so với thực tế.
 */
export const GIAI_THICH_CA =
  "Mỗi dòng là MỘT ca. Nghỉ trưa là khoảng hở giữa hai ca cùng một thứ, không phải một ô đánh " +
  "dấu — một ngày làm cả sáng lẫn chiều là hai dòng.";

/**
 * ⚠ CẢNH BÁO TRƯỚC KHI XOÁ MỘT CA, và nó phải đứng ngay trong biểu mẫu xoá.
 *
 * `UNIQUE (tenant_id, thu, bat_dau)` tính cả dòng đã xoá mềm, có chủ ý: một khoá duy nhất có điều
 * kiện là thứ cho phép cấp lại một giá trị đã cấp, điều luật 7 bất biến 3 cấm. Hệ quả trên màn
 * hình thì rất cụ thể — xoá ca 07:30 thứ Hai xong là vĩnh viễn không khai lại được ca bắt đầu
 * 07:30 thứ Hai. Máy chủ trả 409 kèm đúng câu ấy, nhưng lúc đó thì đã muộn.
 */
export const CANH_BAO_XOA_CA =
  "Xoá một ca thì giờ bắt đầu của ca đó bị giữ lại vĩnh viễn: sau này không khai lại được một ca " +
  "bắt đầu đúng giờ đó trong đúng thứ đó. Muốn đổi giờ làm thì SỬA ca đang có, đừng xoá rồi thêm " +
  "lại.";

export const CANH_BAO_XOA_NGAY_NGHI =
  "Xoá một ngày nghỉ thì ngày đó bị giữ lại vĩnh viễn: sau này không khai lại được đúng ngày đó. " +
  "Muốn đổi thì SỬA dòng đang có.";

export const CANH_BAO_XOA_NGAY_LAM_BU =
  "Xoá một ca làm bù thì giờ bắt đầu của ca đó trong ngày đó bị giữ lại vĩnh viễn. Muốn đổi thì " +
  "SỬA dòng đang có.";

/**
 * Lý do xoá là BẮT BUỘC và đi vào vết lưu của hồ sơ (luật 7, bất biến 1). Không phải một ô cho
 * đủ thủ tục: khi đoàn kiểm tra hỏi vì sao một hạn năm ngoái đếm qua ngày thứ Bảy, câu trả lời
 * nằm đúng ở đây.
 */
export const GIAI_THICH_LY_DO_XOA =
  "Lý do được lưu lại cùng dòng đã xoá, và là câu trả lời khi có người hỏi vì sao lịch của đơn vị " +
  "đổi.";

/* ---- nút và câu xác nhận dùng chung --------------------------------------------------------- */

export const NUT_THEM_CA = "Thêm ca làm việc";
export const NUT_THEM_NGAY_NGHI = "Thêm ngày nghỉ lễ";
export const NUT_THEM_NGAY_LAM_BU = "Thêm ca làm bù";
export const NUT_SUA = "Sửa";
export const NUT_XOA = "Xoá";
export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";
export const NUT_XAC_NHAN_XOA = "Xác nhận xoá";

/** Toast after an SLA row is saved (spec 08). */
export const DA_LUU_THOI_HAN = "Đã lưu thời hạn mới.";
export const DA_LUU_LICH = "Đã lưu lịch làm việc của đơn vị.";
export const DA_XOA_LICH = "Đã xoá dòng lịch. Dòng cũ vẫn được giữ lại trong sổ của hệ thống.";

/**
 * Lỗi màn hình tự phát hiện trước khi gửi — ĐÚNG MỘT phép kiểm, và chỉ vì gửi lên cũng vô nghĩa.
 *
 * Mọi phép kiểm còn lại (khuôn giờ, thứ hợp lệ, ca chồng nhau, ngày vừa nghỉ vừa làm bù) đều do
 * máy chủ làm, mỗi thứ kèm một câu tiếng Việt nói rõ phải sửa gì. Chép chúng xuống client là dựng
 * bản sao thứ hai của một bộ quy tắc nghiệp vụ, và bản sao ấy trôi mà không bài test nào đỏ.
 */
export const LOI_THIEU_LY_DO = "Hãy nêu lý do xoá.";

/** Lỗi tại chỗ của biểu mẫu sửa thời hạn: không đổi gì thì không có gì để gửi. */
export const LOI_KHONG_DOI_GI = "Chưa có con số nào được sửa.";

/** Con số phải là số nguyên dương — máy chủ từ chối, nhưng ô trống thì không đáng gửi đi. */
export const LOI_SO_GIO_LA = "Số giờ phải là một số nguyên lớn hơn 0.";

/** Spec 08's own sentence for "Xử lý xong" typed as zero or below — the column a citizen is promised. */
export const RESOLVE_HOURS_ERROR = "Thời hạn xử lý phải lớn hơn 0 giờ.";
