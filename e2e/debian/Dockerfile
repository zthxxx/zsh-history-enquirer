# Debian-slim image used to exercise the picker against a real zsh
# under a real pty. The Go binary is mounted at /usr/local/bin and the
# plugin file at /opt/zsh-history-enquirer/plugin.zsh; nothing inside
# the image needs to know the host paths.
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    zsh expect ca-certificates locales \
 && rm -rf /var/lib/apt/lists/* \
 && sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
 && locale-gen

ENV LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8 \
    TERM=xterm-256color

RUN useradd -m -s /bin/zsh tester
WORKDIR /home/tester

# We bind-mount the user's binary, plugin, scenarios, and runner at
# `task ci:e2e:run`. No COPY needed.
ENTRYPOINT ["/run.sh"]
