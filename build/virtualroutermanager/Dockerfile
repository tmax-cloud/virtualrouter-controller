FROM frolvlad/alpine-glibc:alpine-3.7_glibc-2.26

ADD virtualrouter-controller /virtualrouter-controller

RUN chmod a+x /virtualrouter-controller

ENTRYPOINT ["/virtualrouter-controller"]