module github.com/ns1labs/webhook-cert-manager-ns1

go 1.16

require (
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0 // indirect
	github.com/jetstack/cert-manager v1.2.1-0.20210324111646-720428406370
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	go.uber.org/zap v1.15.0 // indirect
	google.golang.org/genproto v0.0.0-20200604104852-0b0486081ffb // indirect
	google.golang.org/grpc v1.30.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/ns1/ns1-go.v2 v2.7.1
	k8s.io/apiextensions-apiserver v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0 // indirect
)

replace google.golang.org/grpc => google.golang.org/grpc v1.29.0
