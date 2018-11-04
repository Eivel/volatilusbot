FROM golang:1.10

RUN mkdir /app

WORKDIR /app

ADD bot/main.go /app/main.go

RUN go get -t -d -v ./...

RUN go build -o bot main.go

ENTRYPOINT /app/bot
