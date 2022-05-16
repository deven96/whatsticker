FROM golang:1.17
WORKDIR /project
RUN apt-get update -q && apt-get -y install curl
RUN curl -o libweb.tar.gz -L https://storage.googleapis.com/downloads.webmproject.org/releases/webp/libwebp-0.4.3-rc1-linux-x86-64.tar.gz
RUN tar -xf libweb.tar.gz libwebp-0.4.3-rc1-linux-x86-64/bin/cwebp
RUN cp libwebp-0.4.3-rc1-linux-x86-64/bin/cwebp /usr/bin
RUN rm -rf libwebp-0.4.3-rc1-linux-x86-64/ libweb.tar.gz
COPY go.mod main.go examplestore.db ./
RUN go mod tidy
ENTRYPOINT ["go", "run", "main.go"]
