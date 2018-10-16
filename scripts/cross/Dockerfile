FROM ubuntu:xenial

RUN apt-get update -y && apt-get install -yq  \
		gcc \
		make \
		wget \
		tar \
		gzip \
		vim \
		unzip \
		zip \
		git \
		freetds-bin \
	&& rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.10.3

RUN set -eux; \
	\
# this "case" statement is generated via "update.sh"
	url="https://golang.org/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz"; \
	wget -O go.tar.gz "$url"; \
	gunzip go.tar.gz; \
	tar -C /usr/local -xf go.tar; \
	rm go.tar; \
	export PATH="/usr/local/go/bin:$PATH"; \
	go version

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" "$GOPATH/src/github.com/hashicorp/vault-plugin-database-sybase" && chmod -R 777 "$GOPATH"
WORKDIR $GOPATH/src/github.com/hashicorp/vault-plugin-database-sybase

CMD make bootstrap bin
