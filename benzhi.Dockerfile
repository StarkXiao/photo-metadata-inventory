FROM golang:1.26

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build ./... && go test ./...

CMD ["bash"]
