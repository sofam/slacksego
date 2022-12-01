FROM golang:1.19.3-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /slackse

FROM gcr.io/distroless/base-debian10



WORKDIR /

COPY --from=build /slackse /slackse
COPY megahal /megahal
USER nonroot:nonroot

CMD [ "/slackse" ]