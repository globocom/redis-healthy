FROM golang
ADD . /go
RUN go get -u github.com/kardianos/govendor
ENV $GOPATH=GOPATH:/go/vendor
RUN export GOPATH="$GOPATH:$GOPATH/vendor"
# RUN echo $GOPATH
# RUN pwd
# RUN ls vendor
# RUN govendor add +external
