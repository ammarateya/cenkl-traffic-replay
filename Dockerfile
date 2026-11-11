FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/cenkl-replay ./cmd/cenkl-replay && \
    CGO_ENABLED=0 go build -o /out/cenkl-server ./cmd/cenkl-server

FROM scratch
COPY --from=build /out/cenkl-replay /cenkl-replay
COPY --from=build /out/cenkl-server /cenkl-server
