module github.com/temporalio/features/features

go 1.18

replace (
	go.temporal.io/api => github.com/tdeebswihart/temporal-api-go v0.0.0-20231016220718-646941139bf7
	go.temporal.io/sdk => github.com/tdeebswihart/temporal-sdk-go v0.0.0-20231017162805-b4c4dbb2a35a
)

require (
	github.com/google/uuid v1.3.0
	github.com/temporalio/features/harness/go v1.0.0
	github.com/uber-go/tally/v4 v4.1.1
	go.temporal.io/api v1.24.0
	go.temporal.io/sdk v1.25.0
	go.temporal.io/sdk/contrib/tally v0.1.0
	golang.org/x/mod v0.11.0
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	github.com/twmb/murmur3 v1.1.6 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/genproto v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/temporalio/features/harness/go => ../harness/go
