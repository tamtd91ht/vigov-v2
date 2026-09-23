---
name: doi-chieu-require
description: Dùng khi đọc thay đổi từ kho yêu cầu ../vigov-require, khi viết hoặc đọc một ghi chú đối chiếu, hoặc khi rào require_sync_guard chặn một lần ghi. Kích hoạt bởi - vigov-require, doi chieu, đối chiếu, yêu cầu mới, BA, PM, prototype, neo, sha_tu, sha_den, anh_huong, tiếp nhận, require-watcher, ghi chú đối chiếu, branch bên kia.
---

# Đọc thay đổi từ `../vigov-require`

Kỹ năng này là phần **dài** của cơ chế đối chiếu — nạp khi cần, không thường trực. Ba tệp kia
giữ ba thứ khác:

| Hỏi gì | Đọc đâu |
|---|---|
| Tầng này là gì, "đã tiếp nhận" nghĩa là gì | `kb/50-doi-chieu/README.md` |
| Chạy đối chiếu theo bước nào | `.claude/commands/doi-chieu-require.md` |
| Viết ghi chú theo luật nào | `.claude/agents/require-watcher.md` |
| **Đọc diff thế nào cho ra thứ dùng được** | tệp này |

---

## 1. HAI KHO CÙNG SẢN PHẨM, KIẾN TRÚC LỆCH TOÀN PHẦN

Đây là điều phải nắm trước khi mở commit đầu tiên, nếu không mọi diff đều trông như việc phải
làm.

| | `../vigov-require` | kho này |
|---|---|---|
| Backend | FastAPI · Python, monolith 16 module | Go, 7 vi dịch vụ |
| Cách ly xã | RLS trong Postgres | kho có phạm vi ở tầng Go |
| Danh tính công dân | header `X-Citizen-Id` | phiên do máy chủ phát |
| Nhận xã | `X-Tenant-Code` hoặc tên miền con | `Host` ở rìa |

**Nghiệp vụ thì gần như không lệch — kiến trúc thì lệch toàn phần, và lệch có chủ ý.** Đo đầy
đủ: `kb/90-ephemeral/doi-chieu-vigov-require.md`.

Hệ quả cho việc đọc diff: **phần lớn diff mã bên kia KHÔNG phải việc phải làm.** Một commit
sửa `apps/api/app/modules/tasks/service.py` thường không sinh ra việc gì ở kho này.

---

## 2. PHÉP LỌC — ba câu, theo thứ tự

Với mỗi commit, hỏi lần lượt. Dừng ở câu đầu tiên trả lời được.

### Câu 1 — nó đổi LUẬT NGHIỆP VỤ hay đổi CÁCH CÀI ĐẶT?

| Đổi luật → **giữ** | Đổi cài đặt → **bỏ** |
|---|---|
| thêm/bớt một trạng thái | đổi tên biến, tách hàm |
| đổi cách tính hạn, đổi SLA | sửa lỗi Python, nâng thư viện |
| thêm trường người dùng nhập | đổi truy vấn cho nhanh hơn |
| đổi ai được làm gì | sửa CSS, sửa bố cục |
| thêm màn hình, thêm luồng | đổi cấu hình Railway |

### Câu 2 — nó chạm vùng CỐ Ý LỆCH không?

Có → **bỏ, và ghi vào mục "Không ảnh hưởng"** kèm một dòng lý do. Bốn vùng đã đo và đã chốt là
lệch có chủ ý:

- kiến trúc (Python↔Go, monolith↔vi dịch vụ)
- đặt tên tài nguyên URL (`feedbacks`↔`citizen-reports`, `users`↔`staff`, …)
- danh tính công dân và cách nhận xã
- hồ sơ công dân — khách chốt **ngoài phạm vi hợp đồng** 20/09/2026 (ADR 0001)

Bỏ **không** phải vì nó không quan trọng, mà vì quyết định đã có. Ghi lại để lần sau khỏi đọc
lại cùng commit để kết luận y hệt.

### Câu 3 — nó mâu thuẫn với thứ KHÁCH ĐÃ CHỐT ở kho này?

Tra `kb/00-foundation/open-questions.json` và `kb/10-decisions/`.

Có → **ĐIỀU KIỆN DỪNG.** Không ghi như một mục thường. Xem §5.

---

## 3. ĐỌC Ở ĐÂU CHO RẺ

Thứ tự này ra kết luận nhanh nhất với ít ngữ cảnh nhất.

```sh
# 1. Toàn cảnh trước — đừng mở tệp nào vội
git -C ../vigov-require log --oneline <sha_tu>..<branch>
git -C ../vigov-require diff --stat <sha_tu>..<branch>

# 2. Chỉ phần tài liệu — đây là nơi BA/PM viết
git -C ../vigov-require diff <sha_tu>..<branch> -- docs/

# 3. Lược đồ đổi = nghiệp vụ đổi, gần như luôn đúng
git -C ../vigov-require diff --stat <sha_tu>..<branch> -- apps/api/migrations/

# 4. API mới = bề mặt mới
git -C ../vigov-require diff <sha_tu>..<branch> -- 'apps/api/app/modules/*/router.py'

# 5. Màn hình mới
git -C ../vigov-require diff --stat <sha_tu>..<branch> -- apps/admin/src/app/
```

**Bước 2 là bước quan trọng nhất.** `docs/spec/` bên kia sinh từ chính mô hình và lược đồ
OpenAPI của họ, nên nó **đã là bản tóm tắt** của phần lớn diff mã. Đọc nó rẻ hơn đọc mã và nói
đúng hơn.

**Mở mã chỉ khi:** `docs/` không đổi mà `migrations/` hoặc `router.py` có đổi — tức nghiệp vụ
đi trước tài liệu. Đó đúng là ca `docs/` một mình sẽ nói dối là *"không có gì mới"*.

## 4. ÁNH XẠ TÊN — bên kia gọi khác

Một mục nói *"đổi bảng `feedbacks`"* mà không dịch thì người đọc ở kho này đi tìm một bảng
không tồn tại.

| Bên kia | Kho này | Module |
|---|---|---|
| `feedbacks` | `phieu_phan_anh` · URL `citizen-reports` | `service-petitions` |
| `tasks` | nhiệm vụ | `service-petitions` |
| `documents` · `petitions` | `van_ban_den` · đơn thư | `service-documents` |
| `budget_items` · `disbursements` | `du_an` · `chung_tu_giai_ngan` | `service-finance` |
| `users` | `nguoi_dung` · URL `staff` | `service-identity` |
| `org_units` | `bo_phan` · URL `org-units` | `service-identity` |
| `hamlets` | `thon_to_dan_pho` · URL `residential-units` | `service-identity` |
| `sla_rules` · `holidays` | `sla` · `ngay_nghi_le` | `service-identity` |
| `assets` | tài nguyên bản đồ | `service-comms` |
| `announcements` · `content_*` | thông báo · nội dung Mini App | `service-comms` |
| `activity_logs` | `audit_log` | `core/audit` |
| `apps/admin/**` | `web-admin` | `web-admin` |
| `apps/miniapp/**` | `citizen-app` | `citizen-app` |
| `apps/platform/**` | `platform-admin` | `platform-admin` |

Bảng đầy đủ kèm lý do từng cái tên: `kb/00-foundation/ubiquitous-language.md` §Tên tài nguyên
trên URL.

**Cột "Module" chính là thứ điền vào `anh_huong`** — và `anh_huong` quyết định rào chặn ai, nên
điền sai là chặn nhầm người.

## 5. KHI MỘT COMMIT MÂU THUẪN VỚI QUYẾT ĐỊNH ĐÃ CHỐT

Ca đắt nhất, và ca dễ ghi sai nhất.

**Không ghi nó như một mục phải làm.** Ghi vào mục riêng *"Mâu thuẫn với quyết định đã chốt"*,
nêu đủ **ba** thứ:

1. bên kia nói gì — `<sha>` + đường dẫn
2. kho này đã chốt gì — ADR số mấy, hoặc câu hỏi mở số mấy, ngày nào
3. **ai phải quyết** — không tự đề xuất bên nào thắng

Rồi **báo người dùng ngay trong lần chạy ấy**, đừng để nằm im trong ghi chú.

**Vì sao đắt:** ghi nó như mục thường thì kho này lặng lẽ đi theo bản mới và bỏ quyết định
khách đã ký. Nếu quyết định ấy đã chạm dữ liệu, luật 7 gọi đó là **sửa hồ sơ lưu trữ** — và
những phiếu nhận trước lúc đổi vẫn mang thang cũ, vĩnh viễn.

Ví dụ đã có thật: chín trạng thái phiếu phản ánh. Bên kia dùng chuỗi tiếng Anh (`received`,
`screening`, …); kho này dùng tiếng Việt không dấu và **khách đã duyệt nguyên văn từng ký tự
ngày 20/09/2026** (ADR 0027). Bên kia đổi một chuỗi ấy **không phải** việc kho này làm theo —
đó là mâu thuẫn phải có người quyết.

## 6. KHI RÀO CHẶN BẠN

`require_sync_guard` chặn một lần ghi vào `<module>/` nghĩa là: **bản yêu cầu của module ấy đã
đổi và chưa ai mở việc cho thay đổi đó.**

Hai bước thoát, đúng thứ tự:

1. Đọc ghi chú mà câu chặn nêu tên.
2. Mở việc vào `kb/90-ephemeral/tien-do/<module>.json`, và trong `bang_chung` **trích tên tệp
   ghi chú**. Đó chính là thứ rào đọc để biết đã tiếp nhận. Thủ tục: `/progress`.

**Ghi chú hoá ra không liên quan tới module ấy** → sửa `anh_huong` ở frontmatter của ghi chú.
**Đừng mở một việc giả để rào im** — việc giả nằm lại trong sổ, và lần sau có người đọc nó như
một việc thật.

**Đừng đi đường vòng.** Rào này chặn đúng khoảnh khắc ngón tay sắp gõ, và đó là lớp duy nhất
không bị lướt qua — banner đầu phiên và tin nhắn tới agent đều có thể bỏ lỡ.

## 7. NGHIỆP VỤ BÊN KIA ĐÃ TẢ KỸ MÀ KHO NÀY CHƯA CÓ

Khi review chạm một trong các vùng dưới đây, bên kia **đã có bản thiết kế đầy đủ** — đọc trước
rẻ hơn tự nghĩ lại. Danh sách và trạng thái: `kb/90-ephemeral/doi-chieu-vigov-require.md` §Sổ việc,
nhóm `N`.

Đây **không** phải việc phải làm ngay. Chỉ là: tới lượt phân hệ ấy thì có chỗ để đọc.
