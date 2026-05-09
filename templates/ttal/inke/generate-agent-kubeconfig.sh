#!/bin/bash
# Generates ~/.ttal/kubeconfig from the agent ServiceAccount token.
# Requires: kubectl + admin kubeconfig + agent-rbac namespace existing.
set -euo pipefail

KUBECONFIG_PATH="$HOME/.ttal/kubeconfig"
CLUSTER=$(kubectl config view --minify -o jsonpath='{.clusters[0].name}')
SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
INSECURE=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.insecure-skip-tls-verify}')

TOKEN=$(kubectl create token agent -n agent-rbac --duration=720h)

mkdir -p "$(dirname "$KUBECONFIG_PATH")"

if [ "$INSECURE" = "true" ]; then
  # Tunnel/proxy mode — no CA data, skip TLS verify
  cat > "$KUBECONFIG_PATH" <<KUBECFG
apiVersion: v1
kind: Config
current-context: agent
contexts:
- name: agent
  context:
    cluster: ${CLUSTER}
    user: agent
clusters:
- name: ${CLUSTER}
  cluster:
    server: ${SERVER}
    insecure-skip-tls-verify: true
users:
- name: agent
  user:
    token: ${TOKEN}
KUBECFG
else
  # Standard mode — include CA data from admin config
  CA_DATA=$(kubectl config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')
  cat > "$KUBECONFIG_PATH" <<KUBECFG
apiVersion: v1
kind: Config
current-context: agent
contexts:
- name: agent
  context:
    cluster: ${CLUSTER}
    user: agent
clusters:
- name: ${CLUSTER}
  cluster:
    server: ${SERVER}
    certificate-authority-data: ${CA_DATA}
users:
- name: agent
  user:
    token: ${TOKEN}
KUBECFG
fi

chmod 600 "$KUBECONFIG_PATH"
echo "✅ Kubeconfig written to $KUBECONFIG_PATH"
echo "   Token expires: $(date -v+30d '+%Y-%m-%d' 2>/dev/null || date -d '+30 days' '+%Y-%m-%d')"
