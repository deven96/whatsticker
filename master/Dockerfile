FROM golang:1.17
WORKDIR /project
# Add docker-compose-wait tool -------------------
ENV WAIT_VERSION 2.7.2
ADD https://github.com/ufoscout/docker-compose-wait/releases/download/$WAIT_VERSION/wait /wait
RUN chmod +x /wait
COPY go.mod main.go ./
ADD handler ./handler
ADD task ./task
RUN go mod tidy
ENTRYPOINT ["go", "run", "main.go", "-log-level", "DEBUG"]
