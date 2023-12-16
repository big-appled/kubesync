# kubesync

Sync Kubernetes resources between clusters and namespaces

## Description

This is a k8s resources synchronization tool that synchronizes resources between clusters and namespaces using k8s dynamic client and informers. The intention is to keep different clusters or namespaces in sync when disaster recovery.

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

For use with this tool, you need to install the resources in deploy folder.

```sh
kubectl apply -f deploy
```

default namespace is `kubesync-system`

save cluster kubeconfig into a secret in k8s cluster.

Then you can apply the example cr

```sh
kubectl apply -f examples
```

The example will create a cr `test` in `qiming-migration` namespace.

```yaml
apiVersion: jibutech.com/v1
kind: KubeSync
metadata:
  name: test
  namespace: qiming-migration
spec:
  srcClusterSecret: cluster2 # source cluster kubeconfig secret name
  dstClusterSecret: cluster1 # destination cluster kubeconfig secret name
  syncResources:
    - gvr:
        group: migration.yinhestor.com
        resource: droperationrequests
        version: v1
      namespaces:
        - qiming-migration
  nsMap: # optional
    qiming-migration: ys1000
  pause: false
```

### Running on the cluster

1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/kubesync:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/kubesync:tag
```

### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller

UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing

Feel free to contribute and create pull requests.

### How it works

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out

1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
