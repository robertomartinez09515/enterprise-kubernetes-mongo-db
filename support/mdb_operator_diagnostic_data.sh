#!/usr/bin/env bash

#
# mdb_operator_diagnostic_data.sh
#
# Use this script to gather data about your MongoDB Enterprise Kubernetes Operator
# and the MongoDB Resources deployed with it.
#

#
# shellcheck disable=SC2119
# shellcheck disable=SC2039
#

usage() {
    local script_name
    script_name=$(basename "${0}")
    echo "Usage:"
    echo "${script_name} <namespace> <mdb_resource_name> [<operator_name>] [--private]"
}

contains() {
    local e match=$1
    shift
    for e; do [[ "$e" == "$match" ]] && return 0; done
    return 1
}

if [ -z "${2}" ]; then
    usage
    exit 1
fi

namespace="${1}"
mdb_resource="${2}"
operator_name="${3:-mongodb-enterprise-operator}"
current_date="$(date +%Y-%m-%d_%H_%M)"

contains "--private" "$@"
private_mode=$?

log_dir="logs_${current_date}"
mkdir -p "${log_dir}" &> /dev/null


if ! kubectl get "namespace/${namespace}" &> /dev/null; then
    echo "Error fetching namespace. Make sure name ${namespace} for Namespace is correct."
    exit 1
fi

if ! kubectl -n "${namespace}" get "mdb/${mdb_resource}" &> /dev/null; then
    echo "Error fetching the MongoDB resource. Make sure the ${mdb_resource} is correct."
    exit 1
fi

if ! kubectl -n "${namespace}" get "deployment/${operator_name}" &> /dev/null; then
    echo "Error fetching the MongoDB Operator Deployment. Make sure the deployment/${operator_name} exist and it is running."
    exit 1
fi

if [ ${private_mode} == 0 ]; then
    echo "+ Running on private mode. Make sure you don't share the results of this run outside your organization."
fi

echo "++ Versions"
mdb_operator_pod=$(kubectl -n "${namespace}" get pods -l "app=${operator_name}" -o name | cut -d'/' -f 2)
echo "+ Operator Pod: pod/${mdb_operator_pod}"

mdb_operator_filename="operator.yaml"
echo "+ Saving Operator Deployment into ${mdb_operator_filename}"
kubectl -n "${namespace}" get "deployment/${operator_name}" -o yaml > "${log_dir}/${mdb_operator_filename}"

echo "+ Kubernetes Version Reported by kubectl"
kubectl version

if type oc &> /dev/null; then
    echo "+ Kubernetes Version Reported by oc"
    oc version
fi

operator_logs_filename="${operator_name}_${current_date}.logs"
echo "+ Saving Operator logs to file ${operator_logs_filename}"
kubectl -n "${namespace}" logs "deployment/${operator_name}" > "${log_dir}/${operator_logs_filename}"

database_container_pretty_name=$(kubectl -n "${namespace}" exec -it "${mdb_resource}-0" -- cat /etc/*release | grep "PRETTY_NAME" | cut -d'=' -f 2)
operator_container_pretty_name=$(kubectl -n "${namespace}" exec -it "${mdb_operator_pod}" -- cat /etc/*release | grep "PRETTY_NAME" | cut -d'=' -f 2)
echo "+ Operator is running on: ${operator_container_pretty_name}"
echo "+ Database is running on: ${database_container_pretty_name}"

echo "++ Kubernetes Cluster Ecosystem"
echo "+ Kubectl Cluster Information"
kubectl cluster-info

if [ ${private_mode} == 0 ]; then
    kubectl_cluster_info_filename="kubectl_cluster_info_${current_date}.logs"
    echo "+ Saving Cluster Info to file ${kubectl_cluster_info_filename} (this might take a few minutes)"
    kubectl cluster-info dump | gzip > "${log_dir}/${kubectl_cluster_info_filename}.gz"
else
    echo "= Skipping Kubectl cluster information dump, use --private to enable."
fi

kubectl_sc_dump_filename="kubectl_storage_class_${current_date}.yaml"
kubectl get storageclass -o yaml > "${log_dir}/${kubectl_sc_dump_filename}"

nodes_filename="nodes.yaml"
echo "+ Nodes"
kubectl get nodes

echo "+ Saving Nodes full state to ${nodes_filename}"
kubectl get nodes -o yaml > "${log_dir}/${nodes_filename}"

echo "++ MongoDB Resource Running Environment"
crd_filename="crd_mdb.yaml"
echo "+ Saving MDB Customer Resource Definition into ${crd_filename}"
kubectl -n "${namespace}" get crd/mongodb.mongodb.com -o yaml > "${crd_filename}"

project_filename="project.yaml"
mdb_resource_name="mdb/${mdb_resource}"
project_name=$(kubectl -n "${namespace}" get "${mdb_resource_name}" -o jsonpath='{.spec.project}')
credentials_name=$(kubectl -n "${namespace}" get "${mdb_resource_name}" -o jsonpath='{.spec.credentials}')

resource_filename="mdb_object_${mdb_resource}.yaml"
echo "+ MongoDB Resource Status"
kubectl -n  "${namespace}" get "${mdb_resource_name}" -o yaml > "${log_dir}/${resource_filename}"

echo "+ Saving Project YAML file to ${project_filename}"
kubectl -n "${namespace}" get "configmap/${project_name}" -o yaml > "${log_dir}/${project_filename}"

credentials_user=$(kubectl -n "${namespace}" get "secret/${credentials_name}" -o jsonpath='{.data.user}' | base64 --decode)
echo "+ User configured is (credentials.user): ${credentials_user}"

echo "= To get the Secret Public API Key use: kubectl -n ${namespace} get secret/${credentials_name} -o jsonpath='{.data.publicApiKey}' | base64 --decode)"

statefulset_filename="statefulset.yaml"
echo "+ Saving StatefulSet state to ${statefulset_filename}"
kubectl -n "${namespace}" get "sts/${mdb_resource}" -o yaml > "${log_dir}/${statefulset_filename}"

echo "+ Deployment Pods"
kubectl -n "${namespace}" get pods | grep -E "^${mdb_resource}-[0-9]+"

echo "+ Saving Pods state to ${mdb_resource}-N.logs"
pods_in_namespace=$(kubectl -n "${namespace}" get pods -o name | cut -d'/' -f 2 | grep -E "^${mdb_resource}-[0-9]+")
for pod in ${pods_in_namespace}; do
    kubectl -n "${namespace}" logs "${pod}" > "${log_dir}/${pod}.log"
    kubectl -n "${namespace}" get event --field-selector "involvedObject.name=${pod}" > "${log_dir}/${pod}_events.log"
done

echo "+ Persistent Volumes"
kubectl -n "${namespace}" get pv

echo "+ Persistent Volume Claims"
kubectl -n "${namespace}" get pvc

pv_filename="persistent_volumes.yaml"
echo "+ Saving Persistent Volumes state to ${pv_filename}"
kubectl -n "${namespace}" get pv -o yaml > "${log_dir}/${pv_filename}"

pvc_filename="persistent_volume_claims.yaml"
echo "+ Saving Persistent Volumes Claims state to ${pvc_filename}"
kubectl -n "${namespace}" get pvc -o yaml > "${log_dir}/${pvc_filename}"

services_filename="services.yaml"
echo "+ Services"
kubectl -n "${namespace}" get services

echo "+ Saving Services state to ${services_filename}"
kubectl -n "${namespace}" get services -o yaml > "${log_dir}/${services_filename}"

echo "+ Saving Events for the Namespace"
kubectl -n "${namespace}" get events > "${log_dir}/events.log"

echo "+ Certificates (no private keys are captured)"
csr_filename="csr.text"
kubectl get csr | grep "${namespace}"
echo "+ Saving Certificate state into ${csr_filename}"
kubectl describe "$(kubectl get csr -o name | grep ${namespace})"

echo "++ MongoDBUser Resource Status"
mdbusers_filename="mdbu.yaml"
kubectl -n "${namespace}" get mdbu
echo "+ Saving MongoDBUsers to ${mdbusers_filename}"
kubectl -n "${namespace}" get mdbu > "${log_dir}/${mdbusers_filename}"

crdu_filename="crd_mdbu.yaml"
echo "+ Saving MongoDBUser Customer Resource Definition into ${crdu_filename}"
kubectl -n "${namespace}" get crd/mongodbusers.mongodb.com -o yaml > "${log_dir}/${crdu_filename}"


echo "++ Compressing files"
compressed_logs_filename="${namespace}__${mdb_resource}__${current_date}.tar.gz"
tar -czf "${compressed_logs_filename}" -C "${log_dir}" .

echo "- All logs have been captured and compressed into the file ${compressed_logs_filename}."
echo "- If support is needed, please attach this file to an email to provide you with a better support experience."
echo "- If there are additional logs that your organization is capturing, they should be made available in case of a support request."
