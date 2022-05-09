FROM golang:1.18 as builder
 
WORKDIR /go/src/github.com/automatedhome/scheduler
COPY . .
RUN CGO_ENABLED=0 go build -o scheduler cmd/main.go

FROM busybox:glibc

COPY --from=builder /go/src/github.com/automatedhome/scheduler/site.html /usr/share/site.tmpl
COPY --from=builder /go/src/github.com/automatedhome/scheduler/config.yaml /usr/share/config.yaml
COPY --from=builder /go/src/github.com/automatedhome/scheduler/scheduler /usr/bin/scheduler

HEALTHCHECK --timeout=5s --start-period=1m \
  CMD wget --quiet --tries=1 --spider http://localhost:7009/health || exit 1

EXPOSE 7009
ENTRYPOINT [ "/usr/bin/scheduler", "-template", "/usr/share/site.tmpl" ]
CMD ["-config", "/usr/share/config.yaml"]
