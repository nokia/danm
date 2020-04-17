#!/bin/sh -e

#
# Do some basic sanity testing around configuration parameters
#
for required_config in deploy_mode cni_dir cni_naming_scheme default_cni_type default_cni_network_id
do
  if [ -f "${required_config}" ]
  then
    echo "configuration item: \"${required_config}\" must be configured"
    exit 1
  fi
done

export DANM_DEPLOY_MODE=$(cat /config/deploy_mode)
if [ "${DANM_DEPLOY_MODE}" != "lightweight" ] && [ "${DANM_DEPLOY_MODE}" != "production" ]
then
  echo "\"deploy_mode\" must be either \"lightweight\" or \"production\""
  exit 1
fi

export CNI_DIR=$(cat /config/cni_dir)
export CNI_NAMING_SCHEME=$(cat /config/cni_naming_scheme)
export DEFAULT_CNI_TYPE=$(cat /config/default_cni_type)
export DEFAULT_CNI_NETWORK_ID=$(cat /config/default_cni_network_id)
export IMAGE_PULL_POLICY=$(cat /config/image_pull_policy)

if [ -f /config/image_registry_prefix ]
then
  export IMAGE_REGISTRY_PREFIX="$(cat /config/image_registry_prefix)"
  echo "Using configured image registry prefix: ${IMAGE_REGISTRY_PREFIX}"
else
  echo "Not using any image registry prefix"
fi

if [ -f /config/image_tag ]
then
  export IMAGE_TAG=":$(cat /config/image_tag)"
  echo "Using configured image tag: ${IMAGE_TAG}"
else
  echo "Not using any image tag"
fi

if [ -f /config/image_pull_secret ]
then
  export IMAGE_PULL_SECRET="$(cat /config/image_pull_secret)"
  echo "Using configured image pull secret: ${IMAGE_PULL_SECRET}"
else
  echo "Not using any image pull secret"
fi

if [ -f /config/default_cni_config_data ]
then
  export DEFAULT_CNI_CONFIG_DATA="$(cat /config/default_cni_config_data)"
  echo "Using supplied CNI configuration data"
fi

# We need to get the API server's CA certificate. That is easier said than done. We can glean the server's
# certificate by connecting to it. However, what we'd REALLY want to get, is the *ROOT* CA certificate, and
# not just the individual server certificate. The root CA certificate, however, is nowhere to be found
# programatically.
#
# We can retrieve that from a configmap, of course - but that requires the user to put it there,
# which puts some of the effort back on the user that we wanted to avoid. So, providing two options:
# 1) Read the root cert (or any cert) if provided via configmap, or else 2) read the server
# certificate and make do with that.
if [ -f /config/api_ca_cert ]
then
  echo ; echo "Using supplied CA certificate"
  export KUBERNETES_CA_CERTIFICATE="$(cat /config/api_ca_cert)"
else
  echo ; echo "Reading Kubernetes API server certificate"
  export KUBERNETES_CA_CERTIFICATE="$(openssl s_client -connect ${KUBERNETES_HOST_PORT} 2>&1 </dev/null | sed -ne '/BEGIN CERT/,/END CERT/p')"
fi

#
# Apply API extension CRDs
#
echo ; echo "Applying CRDs to extend Kubernetes API..."
kubectl apply -f /integration/crds/${DANM_DEPLOY_MODE}

#
# Create Service-Account user
#
# Try to create the account; don't be fussy if it already exists
#
echo ; echo "Creating Service Account"
kubectl create --namespace kube-system serviceaccount danm || true
kubectl apply -f /integration/cni_config/danm_rbac.yaml

#
# Create CSR and certificate (using existing script)
#
echo ; echo "Creating WebHook certificate..."
/integration/manifests/webhook/webhook-create-signed-cert.sh

#
# Render templated resources: CNI DaemonSet configuration, CNI DaemonSet,
#   default network, netwatcher
#
echo ; echo "Rendering configuration templates"

SECRET_NAME="$(kubectl get --namespace kube-system -o jsonpath='{.secrets[0].name}' serviceaccounts danm)"
export SERVICEACCOUNT_TOKEN="$(kubectl get --namespace kube-system secrets ${SECRET_NAME} -o jsonpath='{.data.token}' | base64 -d)"
export KUBERNETES_HOST_PORT="$(echo ${KUBERNETES_PORT} | cut -f 3 -d '/')"

mkdir -p /config-out/resources
# It seems confd really does not want to render templates with absolute paths - so we
# need to link in a few externals
ln -sf /integration/manifests/cni_plugins/cni_plugins_ds.yaml.tmpl /etc/confd/templates/cni_plugins_ds.yaml.tmpl
ln -sf /integration/manifests/netwatcher/netwatcher_ds.yaml.tmpl /etc/confd/templates/netwatcher_ds.yaml.tmpl
ln -sf /integration/manifests/webhook/webhook.yaml.tmpl /etc/confd/templates/webhook.yaml.tmpl
ln -sf /integration/manifests/svcwatcher/svcwatcher_ds.yaml.tmpl /etc/confd/templates/svcwatcher_ds.yaml.tmpl

confd -onetime

echo ; echo "Applying DANM CNI DS configuration and default network..."
kubectl apply -f /config-out/resources/

echo ; echo "Creating DANM CNI DaemonSet..."
kubectl apply -f /integration/manifests/cni_plugins

echo ; echo "Creating NetWatcher DaemonSet..."
kubectl apply -f /integration/manifests/netwatcher

echo ; echo "Creating Webhook..."
kubectl apply -f /integration/manifests/webhook

echo ; echo "Creating SvcWatcher..."
kubectl apply -f /integration/manifests/svcwatcher

echo ; echo "Done."
