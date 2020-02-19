module github.com/coredns/coredns

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v32.6.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.3
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.1
	github.com/aws/aws-sdk-go v1.25.48
	github.com/caddyserver/caddy v1.0.4
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/coredns/federation v0.0.0-20190818181423-e032b096babe
	github.com/dnstap/golang-dnstap v0.0.0-20170829151710-2cf77a2b5e11
	github.com/farsightsec/golang-framestream v0.0.0-20181102145529-8a0cb8ba8710
	github.com/golang/protobuf v1.3.2
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/infobloxopen/go-trees v0.0.0-20190313150506-2af4e13f9062
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/dns v1.1.25
	github.com/openshift/coredns-mdns v0.0.0-20200122115902-259b209eea6a
	github.com/opentracing/opentracing-go v1.1.0
	github.com/openzipkin-contrib/zipkin-go-opentracing v0.3.5
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.7.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20190917205325-a14579fbfb1a
	golang.org/x/crypto v0.0.0-20191206172530-e9b2fee46413
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449
	google.golang.org/api v0.14.0
	google.golang.org/grpc v1.25.1
	gopkg.in/DataDog/dd-trace-go.v1 v1.19.0
	k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/klog v0.4.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.0.0+incompatible
	github.com/miekg/dns v1.1.3 => github.com/miekg/dns v1.1.22
)
