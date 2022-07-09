FROM golang:1.17
WORKDIR /project
# Add docker-compose-wait tool -------------------
ENV WAIT_VERSION 2.7.2
ADD https://github.com/ufoscout/docker-compose-wait/releases/download/$WAIT_VERSION/wait /wait
RUN chmod +x /wait
COPY go.mod go.sum ./
COPY utils ./utils
COPY master ./master
RUN mkdir -p ./master/db
RUN go mod tidy
ENTRYPOINT ["go", "run", "master/main.go", "-log-level", "DEBUG"]



