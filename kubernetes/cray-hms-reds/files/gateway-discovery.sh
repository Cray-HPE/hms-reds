#!/bin/sh
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
