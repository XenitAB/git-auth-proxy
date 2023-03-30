FROM golang:1.20 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o git-auth-proxy .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/git-auth-proxy /app/
WORKDIR /app
USER nonroot:nonroot
ENTRYPOINT ["./git-auth-proxy"]
