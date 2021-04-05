FROM golang:1.14.3-alpine
WORKDIR /go/src/app
RUN apk --no-cache add curl make sqlite gcc musl-dev git

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . .
RUN dep ensure

RUN make db
EXPOSE 3000
CMD ["make", "run"]