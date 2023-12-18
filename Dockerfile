FROM alpine

COPY ./kube-expose /usr/local/bin/kube-expose

ENTRYPOINT [ "/usr/local/bin/kube-expose" ]