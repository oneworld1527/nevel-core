FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN sed -i 's/^go 1.23/go 1.22/' go.mod && \
    sed -i 's/^go 1.23/go 1.22/' internal/pebble/go.mod && \
    go build -o neveld ./cmd/neveld/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/neveld .
RUN mkdir -p /var/nevel
EXPOSE 8332 8333
CMD ["sh", "-c", "if [ ! -f /var/nevel/restored ]; then echo 'Downloading chain backup...' && wget -q --header='Authorization: token '\"$GITHUB_TOKEN\"'' 'https://raw.githubusercontent.com/oneworld1527/NEVELBLOCKCHAIN/main/nevel_20percent_final.tar.gz' -O /tmp/chain.tar.gz && tar -xzf /tmp/chain.tar.gz -C / && touch /var/nevel/restored && echo 'Chain restored'; fi && ./neveld start --network mainnet --datadir /var/nevel --rpc 0.0.0.0:${PORT:-8332} --rpctoken ${NEVEL_RPC_TOKEN}"]
