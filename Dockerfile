# https://betterprogramming.pub/why-i-will-never-use-alpine-linux-ever-again-a324fd0cbfd6
# switch to debian
FROM --platform=linux/amd64 golang:1.20.2-bullseye AS builder
LABEL maintainer="Eugene Romanenko <fessae@gmail.com>"

COPY ./ /app/
WORKDIR /app
RUN go build -o pullcsv cmd/pullcsv/main.go && chmod +x pullcsv


FROM --platform=linux/amd64 debian:bullseye-20230502
WORKDIR /app
COPY --from=builder /app/pullcsv /app/
RUN apt update -yy && apt install -qy rsync coreutils
ENV TZ Europe/Moscow

CMD ["/app/pullcsv"]