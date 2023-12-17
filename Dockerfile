# STEP1: CLONE THE CODE
FROM alpine:latest as builder_prepare
WORKDIR /home/filestash
COPY . .

# STEP2: BUILD THE FRONTEND
FROM node:18-alpine AS builder_frontend
WORKDIR /home/
COPY --from=builder_prepare /home/filestash/ ./
RUN apk add make git gzip brotli && \
    npm install --legacy-peer-deps && \
    make build_frontend && \
    cd public && make compress

# STEP3: BUILD THE BACKEND
FROM golang:1.21-bookworm AS builder_backend
WORKDIR /home/
COPY --from=builder_frontend /home/ ./
RUN apt-get update > /dev/null && \
    apt-get install -y libvips-dev curl make > /dev/null 2>&1 && \
    apt-get install -y libjpeg-dev libtiff-dev libpng-dev libwebp-dev libraw-dev libheif-dev libgif-dev && \
    make build_init && \
    make build_backend && \
    mkdir -p ./dist/data/state/config/ && \
    cp config/config.json ./dist/data/state/config/config.json

# STEP4: Create the prod image from the build
FROM debian:stable-slim
MAINTAINER mickael@kerjean.me
COPY --from=builder_backend /home/dist/ /app/
WORKDIR "/app"
RUN apt-get update > /dev/null && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get install -y curl emacs-nox ffmpeg zip poppler-utils > /dev/null && \
    # org-mode: html export
    curl https://raw.githubusercontent.com/mickael-kerjean/filestash/master/server/.assets/emacs/htmlize.el > /usr/share/emacs/site-lisp/htmlize.el && \
    # org-mode: markdown export
    curl https://raw.githubusercontent.com/mickael-kerjean/filestash/master/server/.assets/emacs/ox-gfm.el > /usr/share/emacs/site-lisp/ox-gfm.el && \
    apt-get purge -y --auto-remove perl wget && \
    # Cleanup
    find /usr/share/ -name 'doc' | xargs rm -rf && \
    find /usr/share/emacs -name '*.pbm' | xargs rm -f && \
    find /usr/share/emacs -name '*.png' | xargs rm -f && \
     find /usr/share/emacs -name '*.xpm' | xargs rm -f

RUN useradd filestash && \
    chown -R filestash:filestash /app/ && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/*

USER filestash
CMD ["/app/filestash"]
EXPOSE 8334
