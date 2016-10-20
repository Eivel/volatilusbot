FROM golang:1.6

RUN mkdir /app

ADD vollybot /app/vollybot
ADD .env /app/.env

WORKDIR /app

ENTRYPOINT /app/vollybot
