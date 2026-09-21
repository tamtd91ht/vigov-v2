module github.com/vihat/vigov/tools

go 1.26.0

// `schema-smoke` áp migration CỦA TỪNG DỊCH VỤ, nên nó phải nhập đúng gói `migrations` mà mỗi
// dịch vụ nhúng vào binary của mình. Không có cách nào khác giữ được tính chất quan trọng nhất:
// thứ được kiểm ở đây LÀ thứ dịch vụ mang theo lúc khởi động, không phải một bản sao trên đĩa.
//
// Mọi `replace` đều trỏ thẳng vào thư mục trong kho, cùng lý do `core` được trỏ thẳng ở tám
// go.mod kia: một bản `core` duy nhất cho cả kho, không có chuyện công cụ kiểm một phiên bản
// `core/migrate` khác với phiên bản dịch vụ đang chạy.
require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/vihat/vigov/core v0.0.0
	github.com/vihat/vigov/service-comms v0.0.0
	github.com/vihat/vigov/service-documents v0.0.0
	github.com/vihat/vigov/service-finance v0.0.0
	github.com/vihat/vigov/service-identity v0.0.0
	github.com/vihat/vigov/service-petitions v0.0.0
	github.com/vihat/vigov/service-platform v0.0.0
	github.com/vihat/vigov/service-reporting v0.0.0
	// `tools/ingress` đọc manifest Service trong deploy/base/ và ĐỌC LẠI tệp Ingress nó vừa
	// sinh — bằng một bộ đọc YAML thật, không dùng chung mã với bộ kết xuất. Phép đối chiếu
	// vì thế chứng minh hai đoạn mã độc lập đồng ý về nội dung, chứ không phải một đoạn mã
	// đồng ý với chính nó.
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/vihat/vigov/core => ../core

replace github.com/vihat/vigov/service-comms => ../service-comms

replace github.com/vihat/vigov/service-documents => ../service-documents

replace github.com/vihat/vigov/service-finance => ../service-finance

replace github.com/vihat/vigov/service-identity => ../service-identity

replace github.com/vihat/vigov/service-petitions => ../service-petitions

replace github.com/vihat/vigov/service-platform => ../service-platform

replace github.com/vihat/vigov/service-reporting => ../service-reporting
