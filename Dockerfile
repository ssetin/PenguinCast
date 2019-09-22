FROM alpine:3.9
RUN wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub && \
    wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.28-r0/glibc-2.28-r0.apk && \
    apk add glibc-2.28-r0.apk && \
    rm glibc-2.28-r0.apk
COPY build/penguin /ice/penguin
COPY templates /ice/templates
COPY html /ice/html
COPY config.yaml /ice/config.yaml
WORKDIR /ice
RUN mkdir log
RUN mkdir dump
EXPOSE 8008
ENTRYPOINT ["/ice/penguin"]
