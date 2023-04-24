FROM golang:1.19-alpine

RUN mkdir /app
ADD . /app
WORKDIR /app
RUN go build -o npmjack .
RUN chmod a+x ./npmjack
ENTRYPOINT ["/app/npmjack"]