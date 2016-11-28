FROM golang
ENV APP_PATH /go/src/github.com/globocom/redis-healthy
ADD . $APP_PATH
WORKDIR $APP_PATH
RUN go get -u github.com/Masterminds/glide
RUN glide install
