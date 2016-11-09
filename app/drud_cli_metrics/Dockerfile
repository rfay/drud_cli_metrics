FROM alpine:3.1
MAINTAINER Randy Fay <rfay@newmediadenver.com>
ADD drud_cli_metrics /usr/bin/drud_cli_metrics
RUN mkdir -p /var/lib/sqlite3 && chmod 777 /var/lib/sqlite3
ENTRYPOINT ["drud_cli_metrics", "/var/lib/sqlite3/drud_cli_metrics.db"]
