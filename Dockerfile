FROM alpine:latest
ENV TEMPLATE_PATHS=/templates/default.tmpl
RUN apk add --update ca-certificates

COPY ./default.tmpl /templates/default.tmpl
COPY ./alertmanager-bot/go_build_main_go_linux /usr/bin/alertmanager-bot

RUN chmod 777 /usr/bin/alertmanager-bot

USER nobody

ENTRYPOINT ["/usr/bin/alertmanager-bot"]
