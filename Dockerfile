FROM golang:1.17 AS builder

COPY . /src/
WORKDIR /src/
RUN go build -o /bin/alerttrap alerttrap.go

FROM alpine:3.15.0

ADD https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.34-r0/glibc-2.34-r0.apk /tmp
RUN apk update && \
    apk add --no-cache bash curl && \
    apk add --allow-untrusted /tmp/*.apk && rm -f /tmp/*.apk

EXPOSE 8000
WORKDIR /data
VOLUME ["/data"]

COPY --from=builder /bin/alerttrap /bin/alerttrap
COPY config/config.yml /etc/alerttrap.yml
COPY web /data/web

ENTRYPOINT ["/bin/alerttrap"]
CMD ["-web.dir=/data/web","-config.file=/etc/alerttrap.yml"]
