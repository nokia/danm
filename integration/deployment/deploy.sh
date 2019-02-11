#!/bin/sh -e
DANMKUBECONFIG=/etc/kubernetes/danm.kubeconfig
cp /opt/bin/* /target/cni-bin/
cat >/target/netd/00-danm.conf <<-EOF
{
  "name": "meta_cni",
  "type": "danm",
  "kubeconfig": "$DANMKUBECONFIG"
}
EOF
NAME=kubernetes
HOST=$KUBERNETES_SERVICE_HOST
PORT=$KUBERNETES_SERVICE_PORT
NS=default
ctx_name="${NAME}-${NS}"
TOKEN=`cat /run/secrets/kubernetes.io/serviceaccount/token`
cat >/target/$DANMKUBECONFIG <<-EOF
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://$HOST:$PORT/
  name: $NAME
contexts:
- context:
    cluster: $NAME
    namespace: $NS
    user: $SERVICEACCOUNTNAME
  name: $ctx_name
current-context: $ctx_name
kind: Config
preferences: {}
users:
- name: $SERVICEACCOUNTNAME
  user:
    token: $TOKEN
EOF


