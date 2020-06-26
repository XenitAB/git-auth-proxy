FROM golang:alpine as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/main /app/
WORKDIR /app
USER nonroot:nonroot
ENTRYPOINT ["./main"]
