FROM golang:1.19 as app-builder

ARG DESCRIBE=""
WORKDIR /go/src/app
COPY . .
RUN make clean linux MULTIARCH=true && ls -ali ./release/bolt-agent*linux*

FROM scratch

ENV AWS_DEFAULT_REGION us-east-1

COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=app-builder /go/src/app/release/bolt-agent*linux* /bolt-agent
VOLUME ["/tmp"]

USER 666
ENTRYPOINT ["/bolt-agent"]
