# The web Demo Room: the same program as dist/demo/karots-suite-demo-room, listening for the reverse proxy.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* && useradd -m -u 1000 demo
COPY karots-suite-demo-room /usr/local/bin/karots-demo-room
USER 1000:1000
ENV HOME=/home/demo XDG_CACHE_HOME=/home/demo/.cache TZ=Asia/Colombo LANG=C.UTF-8
CMD ["karots-demo-room", "-host", "0.0.0.0", "-port", "3200"]
