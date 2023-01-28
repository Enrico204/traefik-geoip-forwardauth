FROM golang:1.19.4 AS builder
WORKDIR /src/
COPY . .
RUN go build -mod=vendor -ldflags "-extldflags \"-static\"" -a -installsuffix cgo -o /app/traefik-geoip-forwardauth .

FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
WORKDIR /app/
COPY --from=builder /app/* ./
USER appuser
ENTRYPOINT ["/app/traefik-geoip-forwardauth"]
