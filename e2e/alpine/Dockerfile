# Alpine image — exercises the same picker on musl libc. Combined with
# debian/bookworm (glibc) this gives us the two libc surfaces our
# users actually run on.
FROM alpine:3.20

RUN apk add --no-cache zsh expect ca-certificates ncurses

ENV TERM=xterm-256color

RUN adduser -D -s /bin/zsh tester
WORKDIR /home/tester

ENTRYPOINT ["/run.sh"]
