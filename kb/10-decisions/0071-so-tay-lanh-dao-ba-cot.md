---
id: 0071-so-tay-lanh-dao-ba-cot
tier: T1
source: CURATED
owner: architecture
derived_from_commit: c97f0cd8
expires: null
owns_facts:
  - "Sổ tay lãnh đạo — cột Việc quá hạn = việc còn mở đã quá hạn, đúng điều kiện metric=overdue của Tổng quan và /nhiem-vu (không tính tạm dừng, không tính việc đã xong trễ), toàn xã, gồm nhiệm vụ con (chốt 04/10/2026)"
  - "Sổ tay lãnh đạo — cột Chờ tôi duyệt = hai nhóm tách riêng: (1) Duyệt hoàn thành: mọi nhiệm vụ cho-duyet của xã, chỉ hiện cho người giữ task.approve; (2) Duyệt lùi hạn: đề nghị lùi hạn có tôi là lãnh đạo giao việc (approver=me, ADR 0038). Bỏ vế 'task.approve với bộ phận đó' của đặc tả 03:42 (chốt 04/10/2026)"
  - "Sổ tay lãnh đạo — cột Việc tôi đã giao = nguoi_tao_ma = tôi HOẶC lanh_dao_giao_viec_ma = tôi, mọi trạng thái trừ hoan-thanh, mới nhất trước (chốt 04/10/2026)"
  - "Sổ tay lãnh đạo — mọi số đếm tính ở máy chủ bằng đúng điều kiện của danh sách /nhiem-vu; không đếm trên trình duyệt (bản thử vigov-require đếm trên 300 dòng mới nhất — thiếu khi xã có hơn 300 việc) (chốt 04/10/2026)"
---

# 0071. Sổ tay lãnh đạo — định nghĩa ba cột

**Trạng thái:** đã chốt · **Ngày:** 2026-10-04 · **Người quyết:** chủ dự án, 04/10/2026, sau khi đối
chiếu kho yêu cầu (`kb/50-doi-chieu/2026-10-04-feat-m8-multitenant-foundation-so-tay.md`) · **Thay**
nghĩa của `docs/ui-ux/03-so-tay-lanh-dao.md:41-43` ở ba chỗ dưới đây (bản đặc tả là bản sao yêu cầu,
không sửa — ADR này là câu trả lời khi hai bên lệch).

## Bối cảnh

Ba nguồn định nghĩa mỗi cột một kiểu: đặc tả v2 (`03:41-43`), bản thử của kho yêu cầu
(`LeaderNotebook.tsx:48-54`) và mã v2 (`store/nhiem_vu.go:543`, `task_summary.go:62`). Đặc tả còn đòi
"quyền `task.approve` với bộ phận đó" — quyền ở kho này là khoá phẳng theo xã, không có chiều bộ phận
(luật 5 bất biến 3).

## Quyết định

| Cột | Điều kiện | Khác đặc tả 03 |
|---|---|---|
| Việc quá hạn | Việc còn mở, `han_xu_ly < now()` — **đúng** `metric=overdue` (trạng thái mở, không gồm `tam-dung`, không gồm việc đã xong trễ); toàn xã; gồm nhiệm vụ con; trễ nhiều nhất trước | Đặc tả `∉ {hoan-thanh}` gồm cả tạm dừng |
| Chờ tôi duyệt | Nhóm **Duyệt hoàn thành**: mọi `cho-duyet` của xã, chỉ người giữ `task.approve` thấy nhóm này (đúng người bấm duyệt được — ADR 0065 trả lời #2). Nhóm **Duyệt lùi hạn**: đề nghị lùi hạn có tôi là `lanh_dao_giao_viec_ma` (ADR 0038). Hai nhóm tách riêng, số đếm riêng | Bỏ "theo bộ phận"; bỏ vế `lanh_dao_giao_viec = me` cho phần duyệt hoàn thành |
| Việc tôi đã giao | `nguoi_tao_ma = tôi` HOẶC `lanh_dao_giao_viec_ma = tôi`; mọi trạng thái trừ `hoan-thanh`; mới nhất trước | Định nghĩa rõ "chưa đóng" = ≠ `hoan-thanh` |

Mọi số đếm tính **ở máy chủ** bằng cùng điều kiện với danh sách, để Sổ tay, `/nhiem-vu` và Tổng quan
khớp nhau. Phó Chủ tịch thấy toàn xã như Chủ tịch (chưa có dữ liệu "lĩnh vực phụ trách"). Việc tự chuyển
lãnh đạo vào Sổ tay sau đăng nhập **chưa làm** trong lượt này.

**Vì sao duyệt hoàn thành theo quyền chứ không theo người giao:** người giao việc không giữ
`task.approve` sẽ thấy việc mình không duyệt được; việc chưa ghi lãnh đạo giao thì không ai thấy.
**Vì sao người tạo cũng tính là "đã giao":** văn thư nhập hộ phần lớn nhiệm vụ (ADR 0038:25-28) — đổi
lại, cột của văn thư đầy việc mình đã nhập.

## Hệ quả

- `service-petitions` thêm phạm vi lọc "tôi tạo hoặc tôi giao" + bộ lọc "chưa hoàn thành", dùng chung
  điều kiện cho danh sách, số đếm (`/task-counts`) và xuất sổ; thêm số đếm cho hàng chờ lùi hạn.
- Test so số liệu Sổ tay với `/nhiem-vu` theo từng cột (đặc tả 03:82).
