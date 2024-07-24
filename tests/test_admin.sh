#!/usr/bin/env bash

set -euo pipefail
source ./testing.sh

echo "====================================================================================="
echo "When an admin user is created, it should have access to all resources in the cluster"
echo "======================================================================================"

kubectl apply -f tests/admin_user.yaml
assert "$(kubectl get user user1 -o jsonpath='{.metadata.name}')" "user1"
assert "$(kubectl get user user1 -o jsonpath='{.spec.enabled}')" "true"

kubectl get kubeconfig user1 -o json | jq .spec > kubeconfig
kubectl get all --kubeconfig kubeconfig --namespace default
kubectl get all --kubeconfig kubeconfig --namespace kube-system
kubectl get crd --kubeconfig kubeconfig

rm kubeconfig
kubectl delete -f tests/admin_user.yaml
echo "======================================================================================"
