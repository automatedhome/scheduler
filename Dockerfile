FROM arm32v7/golang:stretch

COPY qemu-arm-static /usr/bin/
WORKDIR /go/src/mqtt-scheduler
COPY . .
RUN go get -u github.com/eclipse/paho.mqtt.golang
RUN go build main.go

FROM arm32v7/busybox:1.30-glibc

COPY site.html /usr/share/site.tmpl
COPY --from=0 /go/src/mqtt-scheduler/main /usr/bin/scheduler

WORKDIR /usr/bin
ENTRYPOINT /usr/bin/scheduler
