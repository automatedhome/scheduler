FROM arm32v7/golang:stretch

COPY qemu-arm-static /usr/bin/
WORKDIR /go/src/github.com/automatedhome/scheduler
COPY . .
RUN make build

FROM arm32v7/busybox:1.30-glibc

COPY site.html /usr/share/site.tmpl
COPY config.yaml /usr/share/config.yaml
COPY --from=0 /go/src/github.com/automatedhome/scheduler/scheduler /usr/bin/scheduler

HEALTHCHECK --timeout=5s --start-period=1m \
  CMD wget --quiet --tries=1 --spider http://localhost:7009/health || exit 1

EXPOSE 7009
ENTRYPOINT [ "/usr/bin/scheduler", "-template", "/usr/share/site.tmpl" ]
CMD ["-config", "/usr/share/config.yaml"]
