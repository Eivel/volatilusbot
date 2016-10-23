FROM golang:1.6

RUN mkdir /app

ADD volatilusbot /app/volatilusbot
ADD .env /app/.env

WORKDIR /app

ENTRYPOINT /app/volatilusbot
