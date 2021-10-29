FROM frolvlad/alpine-glibc:alpine-3.7_glibc-2.26

RUN apk update && apk add iproute2

ADD daemon /daemon

RUN chmod a+x /daemon

ENTRYPOINT ["/daemon"]