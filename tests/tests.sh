#!/usr/bin/env bash

set -euo pipefail
source ./tests/testing.sh

echo "Waiting for klum to start"
eventually kubectl get crd users.klum.cattle.io

echo "======================================================================================"
echo "When an admin user is created, it should have access to all resources in the cluster"
echo "======================================================================================"

kubectl apply -f tests/admin_user.yaml
assert "$(kubectl get user user1 -o jsonpath='{.metadata.name}')" "user1"
assert "$(kubectl get user user1 -o jsonpath='{.spec.enabled}')" "true"

eventually kubectl get kubeconfig user1

kubectl get kubeconfig user1 -o json | jq .spec > kubeconfig
kubectl get all --kubeconfig kubeconfig --namespace default
kubectl get all --kubeconfig kubeconfig --namespace kube-system
kubectl get crd --kubeconfig kubeconfig

rm kubeconfig
kubectl delete -f tests/admin_user.yaml

echo "======================================================================================"
echo "When an user is created disabled, it should have no access and no kubeconfig is created"
echo "======================================================================================"

kubectl apply -f tests/admin_user_disabled.yaml
assert "$(kubectl get user user1 -o jsonpath='{.metadata.name}')" "user1"
assert "$(kubectl get user user1 -o jsonpath='{.spec.enabled}')" "false"

assert_fail kubectl get kubeconfig user1

kubectl delete -f tests/admin_user_disabled.yaml
echo "======================================================================================"

echo "======================================================================================"
echo "When an user has a role, it has access only to the resources in the role"
echo "======================================================================================"

kubectl apply -f tests/user_with_role.yaml
assert "$(kubectl get user user2 -o jsonpath='{.metadata.name}')" "user2"
assert "$(kubectl get user user2 -o jsonpath='{.spec.roles[0]}')" "{\"clusterRole\":\"cluster-admin\",\"namespace\":\"default\"}"

eventually kubectl get kubeconfig user2

kubectl get kubeconfig user2 -o json | jq .spec > kubeconfig
kubectl get all --kubeconfig kubeconfig --namespace default
assert_fail kubectl get all --kubeconfig kubeconfig --namespace kube-system

kubectl delete -f tests/user_with_role.yaml
echo "======================================================================================"
