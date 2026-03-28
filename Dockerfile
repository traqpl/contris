FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o server/web/game.wasm ./game/
RUN cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" server/web/wasm_exec.js

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/contris ./server/

FROM scratch

WORKDIR /app

COPY --from=build /out/contris /app/contris

ENV PORT=8072
ENV DB_PATH=/data/contris_scores.db

EXPOSE 8072
VOLUME ["/data"]

ENTRYPOINT ["/app/contris"]
