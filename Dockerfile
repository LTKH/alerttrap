ARG GOLANG_IMAGE="golang:1.20.3"
ARG ALPINE_IMAGE="alpine"

FROM ${GOLANG_IMAGE} AS builder

COPY . /src/
WORKDIR /src/
RUN go build -o /bin/alerttrap app/alerttrap/alerttrap.go

#FROM ${ALPINE_IMAGE}

EXPOSE 8081

ENV USER_ID=1000
ENV GROUP_ID=1000
ENV USER_NAME=alerttrap
ENV GROUP_NAME=alerttrap

RUN mkdir /data && chmod 755 /data && \
    groupadd --gid $GROUP_ID $GROUP_NAME && \
    useradd -M --uid $USER_ID --gid $GROUP_ID --home /data $USER_NAME && \
    chown -R $USER_NAME:$GROUP_NAME /data

#COPY --from=builder /bin/alerttrap /bin/alerttrap
COPY config/config.yml /etc/alerttrap.yml
COPY web /data/web

VOLUME ["/data"]

USER $USER_NAME

ENTRYPOINT ["/bin/alerttrap"]
CMD ["-web.dir=/data/web","-config.file=/etc/alerttrap.yml"]
