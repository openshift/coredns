FROM centos:7

RUN yum install -y golang

ENV GOPATH /go
RUN mkdir $GOPATH

COPY . $GOPATH/src/github.com/coredns/coredns

RUN cd $GOPATH/src/github.com/coredns/coredns \
    && go build -o coredns . \
    && cp $GOPATH/src/github.com/coredns/coredns/coredns /usr/bin/

RUN yum remove -y golang

ENTRYPOINT ["/usr/bin/coredns"]

USER 1001

LABEL io.k8s.display-name="CoreDNS" \
      io.k8s.description="CoreDNS is a DNS server that chains plugins" \
      maintainer="coredns-discuss@googlegroups.com"
