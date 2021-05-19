FROM golang:1.16 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o azdo-proxy .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/azdo-proxy /app/
WORKDIR /app
USER nonroot:nonroot
ENTRYPOINT ["./azdo-proxy"]
