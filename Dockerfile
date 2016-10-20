FROM golang:1.6

RUN mkdir /app

ADD vollybot /app/vollybot

WORKDIR /app

ENTRYPOINT /app/vollybot
