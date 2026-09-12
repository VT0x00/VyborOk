package pb

//go:generate protoc --proto_path=. --proto_path=/home/vladimir/go/pkg/mod/go.unistack.org/micro-proto/v3@v3.4.6 --go_out=paths=source_relative:. --go_micro_out=paths=source_relative,components=micro|http:. main.proto
