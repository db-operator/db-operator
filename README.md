## Charts for deploying DB Operator

## Migration to the db-operator v3

Version `3.0.0` contains breaking changes for ones using the google cloudsql instances. It doesn't mean that the operator will not work with them anymore, but users will have to use **generic** DbInsances and prepare the infrasturture using other tools.

Other changes that might require migration:

- Pod names were changed, now instead of `db-operator-` and `db-operator-webhook`, we have `db-opetator-controller` and `db-operator-webhook`
- RBAC separation: the controller and webhook now use different ClusterRoles. The configuration has been moved to .controller.rbac and .webhook.rbac accordingly.
- DB Operator configuration is now set under `controller.config`

## Get started with the helm chart

Docs: <https://db-operator.github.io/documentation/install_helm>

Add the repo:

```shell
$ helm repo add db-operator https://db-operator.github.io/charts
$ helm search repo db-operator
```

Install DB Operator:

```shell
$ helm install db-operator/db-operator
# -- Or OCI
$ helm install ghcr.io/db-operator/charts/db-operator:${CHART_VERSION}
```

- More info about the db-operator chart here: [README.md](https://github.com/db-operator/charts/tree/main/charts/db-operator)
- More info about the db-instances chart here: [README.md](https://github.com/db-operator/charts/tree/main/charts/db-instances)

## Development

Docs: <https://db-operator.github.io/documentation/development/helm-charts>

### Pre Commit

It's not required to use **pre-commit hook**, but it will make the development easier. Pre commit hooks against all files are execited during CI.

## Releasing new chart version

The new chart version release is executed automatically with Github actions.
For triggering it, change the version of Chart.yaml in the chart directory and merge to `main` branch.
