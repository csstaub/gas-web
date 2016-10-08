# Our base image is Alpine Linux 3.4.
FROM alpine:3.4

# Set up the environment for building the application.
ENV GOROOT=/usr/lib/go \
    GOPATH=/go \
    PATH=$PATH:$GOROOT/bin:$GOPATH

# Establish a working directory and copy our application
# files into it.
WORKDIR /go
ADD . /go/src/github.com/csstaub/gas-web

# Build your application.
RUN \
	# Upgrade old packages.
	apk --update upgrade && \
	# Ensure we have ca-certs installed.
	apk add --no-cache ca-certificates && \
	# Install go for building.
	apk add -U go gcc g++ make && \
	# Compile our app
	go install github.com/csstaub/gas-web && \
	# Delete go after build.
	apk del go

# Run the application.
ENTRYPOINT ["/go/bin/gas-web"]

# You can test this Docker image locally by running:
#
#    $ docker build -t gas-web .
#    $ docker run --rm -it --expose 8081 -p 8081:8081 -e PORT=8081 gas-web
#
# and then visiting http://localhost:8081/ in your browser.
