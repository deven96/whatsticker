FROM golang:1.17
WORKDIR /project
RUN apt-get update -q && apt-get -y install curl ffmpeg
RUN curl -o libweb.tar.gz -L https://storage.googleapis.com/downloads.webmproject.org/releases/webp/libwebp-0.4.3-rc1-linux-x86-64.tar.gz
RUN tar -xf libweb.tar.gz libwebp-0.4.3-rc1-linux-x86-64/bin/cwebp
RUN tar -xf libweb.tar.gz libwebp-0.4.3-rc1-linux-x86-64/bin/webpmux
RUN cp libwebp-0.4.3-rc1-linux-x86-64/bin/cwebp /usr/bin
RUN cp libwebp-0.4.3-rc1-linux-x86-64/bin/webpmux /usr/bin
RUN rm -rf libwebp-0.4.3-rc1-linux-x86-64/ libweb.tar.gz
# Add docker-compose-wait tool -------------------
ENV WAIT_VERSION 2.7.2
ADD https://github.com/ufoscout/docker-compose-wait/releases/download/$WAIT_VERSION/wait /wait
RUN chmod +x /wait
COPY go.mod go.sum ./
COPY worker/ ./worker
COPY utils/ ./utils
RUN go mod tidy
ENTRYPOINT ["go", "run", "worker/main.go"]
