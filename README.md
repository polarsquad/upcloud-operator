# upcloud-operator

Kubernetes operator for UpCloud managed services. It exposes namespaced CRDs
for Managed Databases, Managed Object Storage, SDN Networks and Routers,
Network Gateways and Managed Load Balancers, and keeps every UpCloud resource
in sync with its Custom Resource.

## Status

v0.1 in progress, see issue #1. This repository currently contains the
scaffold, CI, credential wiring and documentation; the kinds land in later
phases.

## Install

Apply the release manifests:

```sh
kubectl apply -f https://github.com/polarsquad/upcloud-operator/releases/latest/download/install.yaml
```

Then create the credentials Secret in the operator namespace:

```sh
kubectl create secret generic upcloud-credentials \
  -n upcloud-operator-system \
  --from-literal=UPCLOUD_TOKEN=***
```

`UPCLOUD_USERNAME` and `UPCLOUD_PASSWORD` are accepted instead of
`UPCLOUD_TOKEN`. The manager exits at startup when no credential is set.

## Example

```yaml
apiVersion: network.upcloud.polarsquad.com/v1alpha1
kind: Network
metadata:
  name: app-network
  namespace: default
spec:
  name: app-network
  zone: fi-hel1
  family: IPv4
  ipNetwork: 10.100.0.0/16
```

## Docs

See `docs/resources.md` for every kind, its UpCloud counterpart and the
Secrets the operator produces.

## License

Apache-2.0
