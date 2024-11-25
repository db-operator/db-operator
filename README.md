# DB Operator

The DB Operator eases the pain of managing PostgreSQL and MySQL instances for applications running in Kubernetes. The Operator creates databases and make them available in the cluster via Custom Resource. It is designed to support the on demand creation of test environments in CI/CD pipelines.

## Documentations
* [Creating Instances](docs/creatinginstances.md) - make database instances available for the operator
* [Creating Databases](docs/creatingdatabases.md) - creating databases on instances
* [Creating Users](docs/creatingusers.md) - creating users in databases

## Quickstart

We are distributing the operator and `DbInstances` CR as helm charts.

### To install DB Operator with helm:

```SHELL
$ helm repo add db-operator https://db-operator.github.io/charts/
$ helm install --name my-release db-operator/db-operator
```

To see more options of helm values, [see chart repo]([https://github.com/db-operator/charts/tree/main/charts/db-operator])

## Development

#### Prerequisites
* go 1.23.0+
* make
* kubectl
* helm
* a kubernetes cluster

### Developing locally

As we are using **Kubebuilder** for the code generation, we assume that a developer should be aware of how it works. Though **kubebuilder** knowledge is required mostly for changing **CRDs**.

We tend not to release breaking changes to the API, and it means that once there is something that needs to be changed in the API and that would produce a breaking change, we are upgrading the API version

#### After code changes

rebuild CRD manifests
```SHELL
$ make manifests
$ make generate
```

rebuild local docker image
```SHELL
$ make build
```

if you don't use docker, you can set the `CONTAINER_TOOL` environment variable to use another tool, for example:

```SHELL
$ export CONTAINER_TOOL=nerdctl
```

### Deploy

```
helm repo add db-operator https://db-operator.github.io/charts
helm repo update
helm upgrade my-release db-operator/db-operator --set image.repository=my-db-operator --set image.tag=1.0.0-dev --set image.pullPolicy=IfNotPresent --install
```

### Run test locally

to run unit tests, use:
```SHELL
$ make unit
```

integration test for database require accessible databases which can be used by the operator for creating resources. They will be started by docker-compose if you run this command:

```SHELL
$ make test
```

if you don't use docker, you can set the `COMPOSE_TOOL` environment variable, for example

```SHELL
$ export COMPOSE_TOOL="nerdctl compose"
```
