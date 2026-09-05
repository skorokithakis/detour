FROM golang:1.24 AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 go build -o /detour .

FROM gcr.io/distroless/static

COPY --from=build /detour /detour

# Do not add an EXPOSE directive. Dokku publishes a Dockerfile app on whatever port it
# finds in EXPOSE, so "EXPOSE 8080" would serve this app on port 8080 instead of port 80.
# With no EXPOSE, Dokku maps host port 80 to container port 5000 and sets PORT=5000,
# which detour reads. Under plain docker, pass -p and PORT yourself.
CMD ["/detour"]
