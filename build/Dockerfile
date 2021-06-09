FROM frolvlad/alpine-glibc:alpine-3.7_glibc-2.26

ADD virtualrouter-controller /virtual-router-controller

RUN chmod a+x /virtual-router-controller

ENTRYPOINT ["/virtual-router-controller"]