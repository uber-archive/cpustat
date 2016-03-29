FROM debian:sid

COPY cpustat /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/cpustat"]
