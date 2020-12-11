# Copyright 2018-2020 Hewlett Packard Enterprise Development LP

# Dockerfile for building hms-reds.

FROM dtr.dev.cray.com/baseos/golang:1.14-alpine3.12 AS build-base

RUN set -ex \
    && apk update \
    && apk add build-base

FROM build-base AS base

# Copy all the necessary files to the image.
COPY cmd $GOPATH/src/stash.us.cray.com/HMS/hms-reds/cmd
COPY internal $GOPATH/src/stash.us.cray.com/HMS/hms-reds/internal
COPY vendor $GOPATH/src/stash.us.cray.com/HMS/hms-reds/vendor

### Build Stage ###

FROM base AS builder

# Ensure the config file directory exists
RUN mkdir -p /etc/reds

# Now build
RUN set -ex \
    && go build -v -i stash.us.cray.com/HMS/hms-reds/cmd/reds \
    && go build -v -i stash.us.cray.com/HMS/hms-reds/cmd/vault_loader

### Final Stage ###

FROM dtr.dev.cray.com/baseos/alpine:3.12
LABEL maintainer="Cray, Inc."
EXPOSE 8269 162/udp
STOPSIGNAL SIGTERM

# Setup environment variables.
ENV HSM_URL=http://cray-smd/hsm/v1

# Set the default for this Docker image to be to use local storage...just makes it easier for local development.
ENV DATASTORE_URL="mem:"

ENV VAULT_ADDR="http://cray-vault.vault:8200"
ENV VAULT_SKIP_VERIFY="true"

ENV REDS_OPTS="--insecure"

ENV COLUMBIA_ENABLE="false"
ENV SLS_ADDR="http://cray-sls"

# Include curl, net-snmp and the git client in the final image.
RUN set -ex \
    && apk update \
    && apk add --no-cache \
        curl \
        net-snmp \
        git \
    && echo -e "createUser testuser MD5 testpass1 DES testpass2\nauthUser log,execute,net testuser" > /etc/snmp/snmptrapd.conf

# Get reds and reds loader from the builder stage.
COPY --from=builder /go/reds /usr/local/bin
COPY --from=builder /go/vault_loader /usr/local/bin

# Set up the command to start the service, the run the init script.
#CMD snmptrapd -f -Lo -c /etc/snmp/snmptrapd.conf -F '%B %#v\n' -OnQt | reds $REDS_OPTS $( [ -n "$HSM_URL" ] && echo --hsm=$HSM_URL ) --datastore=$DATASTORE_URL
CMD reds $REDS_OPTS $( [ -n "$HSM_URL" ] && echo --hsm=$HSM_URL ) --datastore=$DATASTORE_URL
