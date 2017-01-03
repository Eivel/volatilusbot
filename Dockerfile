FROM golang:1.6

RUN mkdir /app

ADD volatilusbot /app/volatilusbot
ADD .env /app/.env
ADD .permissions.json /app/.permissions.json

WORKDIR /app

ENTRYPOINT /app/volatilusbot
