```
This tool is a modification of https://github.com/jianz/k8s-reset-terminating-pv
USE IT WITH CAUTION AT YOUR OWN RISK
```

# k8s-reset-terminating-ns

Reset namespace  status from terminating back to active. 

## Purpose

When delete a kubernetes namespace by accident, it may stuck in the terminating status due to some finalizer prevent it from being deleted. You can use this tool to reset its status back to active.

## Installing

You can download the latest compiled binary from [here](https://github.com/gmeghnag/k8s-reset-terminating-ns/releases).

If you prefer to compile by yourself:

```shell
git clone git@github.com:gmeghnag/k8s-reset-terminating-ns.git
cd k8s-reset-terminating-ns
go build -o resetns
```

## Usage

For simplicity, you can name the etcd certificate ca.crt, etcd.crt, etcd.key, and put them in the same directory as the tool(resetns).

The tool by default connect to etcd using `localhost:2379`. You can forward the etcd port on the pod to the localhost:

```shell

kubectl port-forward pods/etcd-member-master0 2379:2379 -n etcd
```

`--k8s-key-prefix`: Default set to `registry` for the community version of kubernetes as it uses `/registry` as etcd key prefix, the key for namespace  mynamespace is `/registry/namespaces/mynamespace`. Set to `kubernetes.io` for OpenShift as it uses `/kubernetes.io` as prefix and the key for mynamespace is `/kubernetes.io/namespaces/mynamespace`.

## Example on OpenShift:
In the first terminal execute:
```shell
oc port-forward pods/$(oc get pods -n openshift-etcd -l app=etcd --field-selector="status.phase==Running" -o jsonpath="{.items[0].metadata.name}") 2379:2379 -n openshift-etcd
```
Then, in the second terminal execute:
```shell
oc get secret -n openshift-etcd etcd-client -o json | jq '.data."tls.crt"' -r | base64 -d > ca.crt
oc get secret -n openshift-etcd etcd-client -o json | jq '.data."tls.crt"' -r | base64 -d > etcd.crt
oc get secret -n openshift-etcd etcd-client -o json | jq '.data."tls.key"' -r | base64 -d > etcd.key
./resetns --k8s-key-prefix kubernetes.io <NAMESPACE_NAME>
```
