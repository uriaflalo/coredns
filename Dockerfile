FROM debian:stable-slim

RUN apt-get update && apt-get -uy upgrade
RUN apt-get -y install ca-certificates && update-ca-certificates

FROM scratch

COPY --from=0 /etc/ssl/certs /etc/ssl/certs
ADD coredns /coredns
ADD licenses /THIRD_PARTY_NOTICES/
COPY sources/ /sources/

EXPOSE 54 53/udp
ENTRYPOINT ["/coredns"]
