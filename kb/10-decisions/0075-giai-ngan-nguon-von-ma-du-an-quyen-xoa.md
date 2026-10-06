---
id: 0075-giai-ngan-nguon-von-ma-du-an-quyen-xoa
tier: T1
source: CURATED
owner: domain
derived_from_commit: f181bb76
expires: null
owns_facts:
  - "nguồn vốn là DANH MỤC CHUNG của xã qua các năm; vốn được giao ghi THEO TỪNG NĂM — bác phương án một số tổng không theo năm của prototype (chốt 06/10/2026)"
  - "tên nguồn vốn duy nhất trong xã; nguồn vốn không xoá, không đổi tên; thêm nguồn và ghi vốn giao năm bằng budget.update (chốt 06/10/2026)"
  - "luật phân bổ nguồn vốn của dự án: vượt kế hoạch vốn năm thì từ chối, thiếu thì cho; một nguồn không lặp trong một dự án; sửa phân bổ được sau khi tạo nhưng không gỡ nguồn đã có chứng từ (chốt 06/10/2026)"
  - "luật nguồn của chứng từ giải ngân: dự án đã gắn nguồn thì chứng từ BẮT BUỘC chọn nguồn và chỉ trong các nguồn ấy; dự án chưa gắn nguồn thì chứng từ không gắn nguồn (chốt 06/10/2026)"
  - "mã dự án đầu tư tự sinh DA01, DA02… một dãy theo xã (không theo năm), không bao giờ cấp lại kể cả mã của dự án đã rút (chốt 06/10/2026)"
  - "xoá (rút) dự án đầu tư dùng budget.update, lý do không bắt buộc, để trống thì ghi 'Rút khỏi danh sách dự án' (chốt 06/10/2026)"
  - "nút Hạng mục ở màn Giải ngân hiện với budget.update, nhưng ghi danh mục hạng mục vẫn cần admin.lookup — đúng như prototype (chốt 06/10/2026)"
  - "vướng mắc → nhiệm vụ theo dõi (petitions) và nhắc tên → thông báo (comms) đi qua SỰ KIỆN, nhất quán cuối (hướng chốt 06/10/2026, chưa dựng)"
---

# 0075. Giải ngân theo prototype — nguồn vốn theo năm, luật phân bổ, mã dự án tự sinh, quyền xoá dự án

**Trạng thái:** đã chốt · **Ngày:** 2026-10-06 · **Người quyết:** chủ dự án, 06/10/2026, trong phiên
chính · **Bổ sung** ADR 0068 *Sửa đổi lần 5* cho riêng menu Giải ngân — xem §*Quan hệ với ADR 0068*.

## Bối cảnh

ADR 0068 lần 5 đặt yêu cầu "giống prototype nhất" (`../vigov-require/apps/admin`) nhưng giới hạn ở
front-end. Ở Giải ngân, phần lớn thứ prototype có cần **tuyến, bảng và quyền** chưa tồn tại: danh mục
nguồn vốn, phân bổ, mã tự sinh, vướng mắc, trao đổi. Đồng thời đặc tả `docs/ui-ux/06-giai-ngan.md` và
migration 0007 để lại các câu chưa ai trả lời: tên nguồn có duy nhất không, một dự án rút hai lần
từ một nguồn là một dòng hay hai, và mã dự án theo khuôn nào (§7.2 ví dụ `DA-2026-<slug>`, §9 ghi
"dãy DA01, DA02…" — hai khuôn mâu thuẫn).

Chủ dự án chốt từng điểm ngày 06/10/2026. Lượt này đổi **hợp đồng REST, lược đồ và quyền**, nên cần
ghi lại vì sao.

## Quyết định

| # | Điểm | Chốt | Phương án bị bác |
|---|---|---|---|
| 1 | Hình dạng nguồn vốn | **Danh mục chung của xã** qua các năm; **vốn được giao ghi theo từng năm** (bảng riêng, một dòng mỗi xã · nguồn · năm). Tên nguồn **duy nhất trong xã**. Nguồn **không xoá, không đổi tên**. Thêm nguồn và ghi vốn giao bằng `budget.update` | Prototype: một số tổng **không theo năm**. Chủ dự án đã cân nhắc và bác, vì số tổng là mẫu số của thanh tiến độ mọi năm — sửa nó là viết lại báo cáo năm cũ đã gửi |
| 2a | Phân bổ của dự án | Tổng phân bổ **vượt** kế hoạch vốn năm → **từ chối**; **thiếu** → cho. Một nguồn **không lặp** trong một dự án. Sửa phân bổ được sau khi tạo (thay cả tập); **không gỡ** nguồn đã có chứng từ trên dự án ấy. Chip "Thiếu X" khi phân bổ chưa đủ | Đọc §9 cũ "chỉ cảnh báo" |
| 2b | Nguồn của chứng từ | Dự án **đã gắn nguồn** → chứng từ **bắt buộc** chọn nguồn, và chỉ trong các nguồn đã phân bổ. Dự án **chưa gắn nguồn** → chứng từ **không** gắn nguồn | §13 quy tắc 6: gắn nguồn là tuỳ chọn ở mọi dự án |
| 3 | Mã dự án | Tự sinh **DA01, DA02…**, **một dãy theo xã** (không theo năm), **không cấp lại** — bước qua mọi mã đã có, kể cả của dự án đã rút. Mã tự gõ vẫn được, vẫn phải chưa từng dùng | Khuôn `DA-<năm>-<slug>` của §7.2 |
| 4a | Quyền xoá dự án | `budget.update`, **lý do không bắt buộc**; để trống thì máy chủ ghi câu cố định *"Rút khỏi danh sách dự án"* vào `delete_reason`. Vẫn là xoá mềm, có vết. Dự án còn chứng từ thì từ chối | `budget.confirm` + lý do bắt buộc (lựa chọn của phiên làm việc trước đó, chưa từng có người dùng chốt) |
| 4b | Nút Hạng mục | Hiện với `budget.update`; **ghi** hạng mục vẫn cần `admin.lookup` — đúng như prototype, vì tuyến danh mục của prototype cũng đòi quyền quản lý danh mục | Nới tuyến danh mục xuống `budget.update` |
| 5 | Vướng mắc và trao đổi | Lượt này chỉ dựng phần **trong finance**. Vướng mắc của dự án có cán bộ phụ trách → nhiệm vụ theo dõi (`petitions`); nhắc tên `@` → thông báo (`comms`): đi qua **sự kiện**, nhất quán cuối. Đây là **hướng**, chưa dựng — xem §*Việc còn lại* | Gọi gRPC đồng bộ từ finance trong giao dịch ghi |

**Vì sao #1 đi ngược prototype mà #2–#4 theo prototype:** #2–#4 là hành vi màn hình và luật nhập liệu,
đổi về sau được bằng một bản phát hành. #1 là **mẫu số của báo cáo đã gửi lên**. Số tổng chung cho mọi
năm thì xã sửa số giao năm 2027 là thanh tiến độ năm 2026 đổi theo, không ai thấy. Cùng lý do luật 10
bất biến 2 bắt **lưu** hạn.

**Vì sao không xoá, không đổi tên nguồn (#1):** khoá duy nhất `(tenant_id, ten)` đếm cả dòng xoá mềm
(`tools/check_khoa_duy_nhat.py` cấm khoá có điều kiện `deleted_at`). Không có đường xoá thì không có
dòng chết nào chặn một tên. Nếu sau này cần gỡ nguồn, thêm lại phải là **khôi phục** dòng cũ, không
nới khoá — để lịch sử của nguồn ở trên một id.

**Vì sao một dãy mã theo xã chứ không theo năm (#3):** theo prototype. Mã đã cấp là số trên hồ sơ lưu
trữ (luật 7 bất biến 3). Một dãy theo năm sẽ cấp `DA01` mỗi năm, và một trích dẫn "DA01" thiếu năm trỏ
vào nhiều dự án.

## Quan hệ với ADR 0068

ADR 0068 không bị sửa (ADR không bao giờ sửa). Với **riêng menu Giải ngân**, ADR này **thay** ba chỗ
của 0068 lần 5:

| 0068 lần 5 | Nay |
|---|---|
| #4 "không tự sinh mã dự án trước khi chốt định dạng" | Định dạng đã chốt (#3 trên) — tự sinh được |
| #4 "xoá mềm kèm lý do" (áp cho dự án đầu tư) | Vẫn xoá mềm; lý do **không bắt buộc**, có câu mặc định (#4a). Xoá nhiều dự án một lần là **N lần xoá mềm riêng**, mỗi lần một vết — không có tuyến xoá hàng loạt |
| #5 "chỉ front-end, không đổi API, proto, migration" | Giải ngân đổi hợp đồng REST, thêm migration 0013–0015 và đổi quyền một tuyến. Không thêm khoá quyền mới (`check_quyen` vẫn xanh) |

Các điểm khác của 0068 lần 5 #4 giữ nguyên ở Giải ngân: xã lấy từ `Host`, sửa chứng từ đã xác nhận →
về nháp (ADR 0036), không xoá cứng.

## Hệ quả

- Lược đồ: migration `0013` (danh mục nguồn + vốn giao theo năm + hai khoá duy nhất), `0014` (bộ đếm
  mã dự án theo xã), `0015` (vướng mắc, trao đổi; cột `tracking_task_id` để sẵn, chưa ai ghi). **Chưa
  chạy trên PostgreSQL thật** — máy làm việc không có `VIGOV_TEST_DSN`; các ca `*_pg_test.go` đang bỏ qua.
- `0013` **từ chối chạy** nếu `nguon_von` đã có dòng: đổi hình dạng khi có dữ liệu cần backfill và
  một quyết định riêng (luật 7 điều kiện dừng #2).
- Người giữ `budget.update` nay rút được dự án không cần người giữ `budget.confirm` — ranh giới "người
  nhập / người chịu trách nhiệm" (sổ `web-admin` → `giai-ngan-tuyen-ghi-len-man`) không còn áp cho
  việc xoá dự án; vẫn áp cho chứng từ (gỡ, xác nhận, khoá).
- Chứng từ cũ của dự án đã gắn nguồn mà nguồn còn trống vẫn sửa được số tiền/mô tả: luật 2b chỉ kiểm
  khi thân `PATCH` nêu `funding_source_id`.
- Commit: `fd03e88f` · `473df7c8` · `8245698b` · `db94b35c` · `a3fdcac2` · `9f0a0187` · `889d4598` ·
  `f181bb76` (máy chủ); `cc76427c` · `4338918a` · `2d4d4c55` · `56980d00` · `95d84ff0` · `2d01c386` ·
  `744582d7` · `b0a880e0` (web).

## Việc còn lại — hướng #5

| Việc | Ghi chú |
|---|---|
| finance phát sự kiện khi ghi vướng mắc / ý kiến có nhắc tên | Tên sự kiện mang phiên bản (luật 2 bất biến 4); khai mức nhất quán và bù trừ trong `transaction-boundaries` (luật 2 bất biến 6). Thân sự kiện không mang nội dung tự do (luật 3) |
| petitions tạo nhiệm vụ theo dõi, finance ghi `tracking_task_id` | Bên tiêu thụ luỹ đẳng (luật 2 bất biến 5) |
| comms gửi thông báo cho người được nhắc | — |

Lượt giao thiết kế sự kiện bị chặn quyền; chờ chủ dự án.

## Còn mở — chưa ai chốt, không tự chọn

| # | Câu | Mã hôm nay |
|---|---|---|
| 1 | Cột *Nguồn vốn* khi **nhập Excel** chứng từ có áp luật 2b không | Áp (`f181bb76`); prototype nhập với nguồn rỗng |
| 2 | Nhập **lại** cùng một tệp Excel | Ghi chứng từ lần nữa — không chặn, không cảnh báo |
| 3 | Cờ *"nguy cơ không giải ngân hết"* | Prototype là cờ đặt tay; **chưa dựng** |
| 4 | Vạch *thời gian đã qua* theo năm ngân sách hay theo khung khởi công → hoàn thành | Theo năm ngân sách. Đổi sang khung dự án là đổi luật chậm (luật 10 điều kiện dừng #1) |
| 5 | `assignee_id` của dự án giữ **mã cán bộ** hay id | Web gửi mã cán bộ (`744582d7`); đặc tả §11 ghi `uuid` |
| 6 | Kế hoạch vốn và thời hạn **theo hạng mục** | Suy ra từ các dự án (`a3fdcac2`); đặc tả §5 nói đặt trên hạng mục. ⚠ Câu mở #34 (đã chốt 22/09) ghi *"màn hình KHÔNG được hiện chúng"* — số suy ra đang hiện ở bảng Tiến độ theo hạng mục, cần chủ dự án xác nhận đây là cách đọc mới hay là lệch |

→ ADR 0036 (sửa chứng từ đã xác nhận) · ADR 0068 lần 5 · ADR 0035 (câu #31, #34)
→ Đặc tả: `docs/ui-ux/06-giai-ngan.md` §6, §7.2, §8, §9, §11, §13
→ Sổ tiến độ: `python tools/tien_do.py --menu giai-ngan`
