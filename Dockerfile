FROM golang:alpine AS build
WORKDIR /
RUN apk add -U ca-certificates git
ADD go.mod go.sum ./
RUN go mod download
ADD . .
ENV CGO_ENABLED=0 
ENV GOOS=linux
RUN sh ./test.sh
RUN go build -a -installsuffix cgo -o app .

FROM scratch
COPY --from=build /app /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]