FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.24-openshift-4.21 as builder
WORKDIR /go/src/github.com/openshift/coredns-ocp-dnsnameresolver
COPY . .

RUN make build-coredns

FROM registry.ci.openshift.org/ocp/4.21:base
COPY --from=builder /go/src/github.com/openshift/coredns-ocp-dnsnameresolver/coredns /usr/bin/

ENTRYPOINT ["/usr/bin/coredns"]

LABEL io.k8s.display-name="CoreDNS" \
      io.k8s.description="CoreDNS delivers the DNS and Discovery Service for a Kubernetes cluster." \
      maintainer="dev@lists.openshift.redhat.com"
