module github.com/vihat/vigov/service-comms

go 1.26.0

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/vihat/vigov/core v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

// `core` la ma dung chung TRONG CUNG KHO, nen no duoc tro thang vao thu muc thay vi tai qua
// mot phien ban da phat hanh.
//
// Y nghia cua dong nay dang noi ro, vi no quyet dinh mot rui ro that: chung nao con `replace`,
// ca tam dich vu dung CHUNG MOT ban `core` — khong co chuyen tam dich vu chay tam phien ban
// khac nhau cua `core/authz` va `core/tenant`, tuc tam ban cai dat khac nhau cua mo hinh cach
// ly (luat 1). Ngay ai do bo dong nay de ghim phien ban rieng, rui ro ay xuat hien, va CI phai
// co kiem san phien ban truoc khi dieu do xay ra — xem ADR 0015.
//
// `replace` trong mot module MAIN khong anh huong ai khac: dich vu khong bao gio bi import.
replace github.com/vihat/vigov/core => ../core
