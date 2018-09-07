FROM openshift/origin-release:golang-1.10 as build
WORKDIR /go/src/github.com/coredns/coredns
ENV GOPATH=/go
COPY . .
RUN go build -o coredns .

FROM centos:7
COPY --from=build /go/src/github.com/coredns/coredns/coredns /usr/bin/coredns

ENTRYPOINT ["/usr/bin/coredns"]

LABEL io.k8s.display-name="CoreDNS" \
      io.k8s.description="CoreDNS delivers the DNS and Discovery Service for a Kubernetes cluster." \
      maintainer="dev@lists.openshift.redhat.com"
