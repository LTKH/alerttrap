FROM golang:1.17 AS builder

COPY . /src/
WORKDIR /src/
RUN go build -o /bin/alerttrap alerttrap.go

FROM alpine:3.15.0

RUN apk update && apk add --no-cache bash

EXPOSE 8100
WORKDIR /data
VOLUME ["/data"]

COPY --from=builder /bin/alerttrap /bin/alerttrap
COPY config/config.yml /etc/alerttrap.yml

ENTRYPOINT ["/bin/alerttrap"]
CMD ["-config=/etc/alerttrap.yml"]
