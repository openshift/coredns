module github.com/coredns/coredns

go 1.15

require (
	github.com/Azure/azure-sdk-for-go v32.6.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.9
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.1
	github.com/aws/aws-sdk-go v1.25.48
	github.com/caddyserver/caddy v1.0.4
	github.com/dnstap/golang-dnstap v0.0.0-20170829151710-2cf77a2b5e11
	github.com/farsightsec/golang-framestream v0.0.0-20181102145529-8a0cb8ba8710
	github.com/golang/protobuf v1.4.3
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/infobloxopen/go-trees v0.0.0-20190313150506-2af4e13f9062
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/dns v1.1.25
	github.com/openshift/coredns-mdns v0.0.0-20200617110758-8a8fcb1926a0
	github.com/opentracing/opentracing-go v1.1.0
	github.com/openzipkin-contrib/zipkin-go-opentracing v0.3.5
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.7.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200306183522-221f0cc107cb
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/sys v0.0.0-20201112073958-5cba982894dd
	google.golang.org/api v0.20.0
	google.golang.org/grpc v1.27.1
	gopkg.in/DataDog/dd-trace-go.v1 v1.19.0
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/klog v0.4.0
)

replace github.com/miekg/dns v1.1.3 => github.com/miekg/dns v1.1.22
