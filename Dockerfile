FROM golang:1.15-alpine

COPY . .

RUN go build -o server . 

CMD ["./server"]