ARG ARCH="amd64"
ARG OS="linux"
FROM 192.168.1.202/linuxamd64/alpine-java8:20201125
LABEL maintainer="hunterfox<butteflywangqq.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/amtool       /bin/amtool
COPY .build/${OS}-${ARCH}/alertmanager /bin/alertmanager
COPY examples/ha/alertmanager.yml       /etc/alertmanager/alertmanager.yml
COPY scripts/conf/nacos.yml             /etc/conf/nacos.yml

RUN mkdir -p /alertmanager && \
    chown -R nobody:nogroup /etc/alertmanager /alertmanager /etc/conf
USER       nobody
EXPOSE     9093
VOLUME     [ "/alertmanager","/etc/alertmanager" ]
WORKDIR    /alertmanager
ENTRYPOINT [ "/bin/alertmanager" ]
CMD        [ "--config.file=/etc/alertmanager/alertmanager.yml","--nacos=/etc/conf", \
             "--storage.path=/alertmanager" ]
