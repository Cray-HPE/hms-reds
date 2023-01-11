# MIT License
#
# (C) Copyright [2018-2022] Hewlett Packard Enterprise Development LP
#
# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the "Software"),
# to deal in the Software without restriction, including without limitation
# the rights to use, copy, modify, merge, publish, distribute, sublicense,
# and/or sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included
# in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
# THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
# OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
# OTHER DEALINGS IN THE SOFTWARE.

# Dockerfile for building hms-reds.

### Build Base Stage ###

FROM artifactory.algol60.net/docker.io/library/golang:1.16-alpine AS build-base

RUN set -ex \
    && apk -U upgrade \
    && apk add build-base

### Base Stage ###

FROM build-base AS base

RUN go env -w GO111MODULE=auto

# Copy all the necessary files to the image.
COPY cmd $GOPATH/src/github.com/Cray-HPE/hms-reds/cmd
COPY internal $GOPATH/src/github.com/Cray-HPE/hms-reds/internal
COPY vendor $GOPATH/src/github.com/Cray-HPE/hms-reds/vendor

### Build Stage ###

FROM base AS builder

# Ensure the config file directory exists
RUN mkdir -p /etc/reds

# Now build
RUN set -ex \
    && go build -v -i github.com/Cray-HPE/hms-reds/cmd/reds \
    && go build -v -i github.com/Cray-HPE/hms-reds/cmd/vault_loader

### Final Stage ###

FROM artifactory.algol60.net/docker.io/alpine:3.15
LABEL maintainer="Hewlett Packard Enterprise"
EXPOSE 8269 162/udp
STOPSIGNAL SIGTERM

# Setup environment variables.
ENV HSM_URL=http://cray-smd/hsm/v2

ENV VAULT_ADDR="http://cray-vault.vault:8200"
ENV VAULT_SKIP_VERIFY="true"

ENV REDS_OPTS="--insecure"

ENV SLS_ADDR="http://cray-sls"

# Include curl, net-snmp and the git client in the final image.
RUN set -ex \
    && apk -U upgrade \
    && apk add --no-cache \
        curl \
        net-snmp

# Get reds and reds loader from the builder stage.
COPY --from=builder /go/reds /usr/local/bin
COPY --from=builder /go/vault_loader /usr/local/bin

COPY configs configs


# nobody 65534:65534
USER 65534:65534

# Set up the command to start the service, the run the init script.
#CMD snmptrapd -f -Lo -c /etc/snmp/snmptrapd.conf -F '%B %#v\n' -OnQt | reds $REDS_OPTS $( [ -n "$HSM_URL" ] && echo --hsm=$HSM_URL ) --datastore=$DATASTORE_URL
CMD reds $REDS_OPTS $( [ -n "$HSM_URL" ] && echo --hsm=$HSM_URL ) --sls=$SLS_ADDR
