#!/bin/sh
# MIT License
#
# (C) Copyright [2021] Hewlett Packard Enterprise Development LP
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

GATEWAY_IP=""

echo "Looking up Istio gateway IP..."
ISTIO_API_GW_IP=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.loadBalancerIP}')
if [[ ! -z ${ISTIO_API_GW_IP} ]]; then
    echo "Found Istio gateway at: $ISTIO_API_GW_IP"
    GATEWAY_IP="$ISTIO_API_GW_IP"
else
    echo "Didn't find Istio, looking up Kong's API gateway IP address..."

    if KONG_API_GW_IP=$(kubectl get service api-gateway -o jsonpath='{.spec.loadBalancerIP}' -n default); then
        echo "Found Kong Gateway at: $KONG_API_GW_IP"
        GATEWAY_IP="$KONG_API_GW_IP"
    else
        echo "Did not find Kong. Looks like we're on Virtual Shasta. Using Dummy Kong API GW."
        GATEWAY_IP="cray-reds-dummy-kong-apigw-tls"
    fi
fi

if [[ -z ${GATEWAY_IP} ]]; then
    echo "Unable to determine gateway IP!!!"
    exit 1
fi

echo "Creating/updating cray-reds-init-config with gateway IP: ${GATEWAY_IP}"

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: reds-init-configmap
  namespace: services
data:
  GATEWAY_IP: ${GATEWAY_IP}
EOF
