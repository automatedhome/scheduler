FROM arm32v7/golang:stretch

COPY qemu-arm-static /usr/bin/
WORKDIR /go/src/github.com/automatedhome/scheduler
COPY . .
RUN go build -o scheduler cmd/main.go

FROM arm32v7/busybox:1.30-glibc

COPY site.html /usr/share/site.tmpl
COPY --from=0 /go/src/github.com/automatedhome/scheduler/scheduler /usr/bin/scheduler

ENTRYPOINT /usr/bin/scheduler
