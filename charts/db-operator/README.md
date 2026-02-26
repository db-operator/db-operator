# db-operator

![Version: 2.6.0](https://img.shields.io/badge/Version-2.6.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 2.22.0](https://img.shields.io/badge/AppVersion-2.22.0-informational?style=flat-square)

The DB Operator creates databases and make them available in the cluster via Custom Resource.

## Source Code

* <https://github.com/db-operator/db-operator>
* <https://github.com/db-operator/cloudish-sql>
* <https://github.com/db-operator/pgdump-gcs>
* <https://github.com/db-operator/mydump-gcs>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Nikolai Rodionov | <iam@allanger.xyz> | <https://badhouseplants.net> |

## Requirements

Kubernetes: `>= 1.30-prerelease`

## Installing the Chart

```console
$ helm repo add db-operator https://db-operator.github.io/charts
$ helm repo update
$ helm install db-operator/db-operator
```

If you want to manage `CRDs` yourself, you need to set `.Values.crds.install` to `false`. In that case they won't be a part of templated manifests.

We don't support this way of `CRD` management officially though, so we can't promise to answer your questions if you choose to go this way. If you want to know more about it, keep reading, we'll talk about it later.

## Upgarding the Chart

If there are breaking changes, they should be described in this README's latest item and in helm-chart notes. So if you run `helm upgrade db-operator db-operator/db-operator --dry-run`, you will see if there are breaking changes there.

## Uninstalling the Chart

To uninstall the operator only, run

```console
$ helm uninstall db-operator
```

If you have CRDs installed by the chart, you will also have to run something like:

```console
# !!! This command will remove all kinda.rocks CRDs from the cluster,
#     so please make sure that you really want to run it
$ for crd in $(kubectl get crds | awk '{print $1}' | grep 'kinda.rocks'); do kubectl delete crd "${crd}"; done
```

## Why are we packaging CRDs in templates?

There are several reasons for that. The main one is that we are using webhooks and they require certificates to be created. If you have a look at the `CRDs` templates, you'll see something like that:

```mustache
  annotations:
    controller-gen.kubebuilder.io/version: v0.12.1
    {{- if .Values.webhook.certificate.create }}
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/{{ .Values.webhook.certificate.name}}
    {{ else }}
    cert-manager.io/inject-ca-from-secret: {{ .Release.Namespace }}/{{ .Values.webhook.certificate.secretName}}
    {{- end }}
    {{- if .Values.crds.keep }}
    helm.sh/resource-policy: keep
    {{- end }}
    {{- with .Values.crds.annotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
```

So, as you can see they require some information about the release, so they can't be static. Since the webhook is an important part of what we're distributing, we don't want to have it disabled by default.

Also, we do not have `CRs` in the package, so there is no chicken-egg problem here. You don't need to have `CRDs` installed to install the chart. Since `CRDs` are a part of the operator and you can't have one without another, having them packaged and distributed together makes more sense to us.

But it's still possible for users to manage `CRDs` on their own. You need to set `.Values.crds.install` to false and go to the [db-operator rgit repository](https://github.com/db-operator/db-operator). There, in the `config/crd/bases` directory, you'll find `CRD` manifests. But if you decided to go this way and you have problems with `CRDs`, you're most probably on your own, you still can open issues though.

## Why do we need cert-manager?

At the moment, `db-operator` chart requires `cert-manager` to be present in the cluster. It's because webhook related resources are using it to create certificates. It should change one day, since we would like this chart to be self-sufficient, but currently that's what we've got.

## What is `.Values.tests`?

We use helm tests feature for testing the chart in CI. Though we know that this feature should be used by users to test their intsallations, we do not think that it can be very helpful in this particular case. Test resources are creating a powerful `ClusterRoles`, so **please make sure you don't have them enabled**, unless you really know what you're doing and why.

## Chart Values
| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nameOverride | string | `""` |  |
| image.repository | string | `"ghcr.io/db-operator/db-operator"` |  |
| image.pullPolicy | string | `"Always"` |  |
| controller.logLevel | string | `"info"` |  |
| controller.extraArgs | list | `[]` |  |
| reconcileInterval | string | `"60"` |  |
| watchNamespace | string | `""` |  |
| checkForChanges | bool | `false` | ---------------------------------------------------------- |
| rbac.create | bool | `true` |  |
| serviceAccount.create | bool | `true` |  |
| crds.install | bool | `true` |  |
| crds.keep | bool | `true` |  |
| crds.annotations | object | `{}` |  |
| webhook.extraArgs | list | `[]` |  |
| webhook.enabled | bool | `true` |  |
| webhook.logLevel | string | `"info"` |  |
| webhook.podLabels | object | `{}` |  |
| webhook.serviceAccount.create | bool | `true` |  |
| webhook.names.mutating | string | `"db-operator-mutating-webhook-configuration"` |  |
| webhook.names.validating | string | `"db-operator-validating-webhook-configuration"` |  |
| webhook.certificate.create | bool | `true` | ------------------------------------------ |
| webhook.certificate.name | string | `"db-operator-webhook"` |  |
| webhook.certificate.secretName | string | `"db-operator-webhook-cert"` |  |
| webhook.certificate.issuer.create | bool | `true` |  |
| webhook.certificate.issuer.name | string | `"db-operator-issuer"` |  |
| webhook.certificate.issuer.kind | string | `"Issuer"` | --------------------------------------- |
| podSecurityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Configure the security context for the operator pods |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Configure the security context for the operator container |
| hostUsers | bool | `true` |  |
| resources | object | `{}` |  |
| nodeSelector | object | `{}` |  |
| annotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| affinity | object | `{}` |  |
| tolerations | list | `[]` |  |
| config.instance.google.proxy.nodeSelector | object | `{}` |  |
| config.instance.google.proxy.image | string | `"ghcr.io/db-operator/db-auth-gateway:v0.1.10"` |  |
| config.instance.google.proxy.metricsPort | int | `9090` |  |
| config.instance.generic | object | `{}` |  |
| config.instance.percona.proxy.image | string | `"severalnines/proxysql:2.0"` |  |
| config.instance.percona.proxy.metricsPort | int | `9090` |  |
| config.backup.activeDeadlineSeconds | int | `600` |  |
| config.backup.nodeSelector | object | `{}` |  |
| config.backup.postgres.image | string | `"kloeckneri/pgdump-gcs:latest"` |  |
| config.backup.mysql.image | string | `"kloeckneri/mydump-gcs:latest"` |  |
| config.backup.resources.requests.memory | string | `"64Mi"` |  |
| config.backup.resources.requests.cpu | float | `0.2` |  |
| config.monitoring.promPushGateway | string | `""` |  |
| config.monitoring.nodeSelector | object | `{}` |  |
| config.monitoring.postgres.image | string | `"wrouesnel/postgres_exporter:latest"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.query | string | `"SELECT userid, pgss.dbid, pgdb.datname, queryid, query, calls, total_time, mean_time, rows FROM pg_stat_statements pgss LEFT JOIN (select oid as dbid, datname from pg_database) as pgdb on pgdb.dbid = pgss.dbid WHERE not queryid isnull ORDER BY mean_time desc limit 20"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[0].userid.usage | string | `"LABEL"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[0].userid.description | string | `"User ID"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[1].dbid.usage | string | `"LABEL"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[1].dbid.description | string | `"database ID"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[2].datname.usage | string | `"LABEL"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[2].datname.description | string | `"database NAME"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[3].queryid.usage | string | `"LABEL"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[3].queryid.description | string | `"Query unique Hash Code"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[4].query.usage | string | `"LABEL"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[4].query.description | string | `"Query class"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[5].calls.usage | string | `"COUNTER"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[5].calls.description | string | `"Number of times executed"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[6].total_time.usage | string | `"COUNTER"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[6].total_time.description | string | `"Total time spent in the statement, in milliseconds"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[7].mean_time.usage | string | `"GAUGE"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[7].mean_time.description | string | `"Mean time spent in the statement, in milliseconds"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[8].rows.usage | string | `"COUNTER"` |  |
| config.monitoring.postgres.queries.pg_stat_statements.metrics[8].rows.description | string | `"Total number of rows retrieved or affected by the statement"` |  |
| secrets.gsql | object | `{}` |  |
| serviceMonitor.enabled | bool | `false` |  |
| service.annotations | object | `{}` |  |
| service.type | string | `"ClusterIP"` |  |
| service.port | int | `8080` |  |
## Dev Notes

After changing default `Values`, please execute `make gen_docs` to update the `README.md` file. Readme file is generated by the `helm-docs` tool, so make sure not to edit it manually.

## Upgrading

If there is an breaking change, or something that might make the upgrade complicated, it should be described here

<details>
  <summary>To <code>v2.0.0</code></summary>
From `v2.0.0` onwards, the `v` prefix will not be automatically added to the image tag. If you are overriding the image tag through `.Values.image.tag` and are relying on this behaviour, please add the `v` prefix manually to the Helm value. Otherwise, no action is required from your side!
</details>

<details>
  <summary>To <code>v1.11.0</code></summary>
Additional selectors were added to the default templates in an attempt to follow the same labelling scheme everywhere, but since selectors are immutable, the upgrade will require removing of the db-operator deployment.

```bash
$ kubectl get deploy
NAME          READY   UP-TO-DATE   AVAILABLE   AGE
db-operator   1/1     1            1           22s
$ kubectl delete deploy db-operator
deployment.apps "db-operator" deleted
$ helm upgrade db-operator db-operator/db-operator --version 1.11.0
```

</details>

<details>
  <summary>To <code>v1.10.0</code></summary>

CRDs are moved to the `templates` folder, so now they are part of the release. It means that after the upgrade, you will get errors about resource ownerships. Thow errors will contain messages about missing `labels` and `annotations`, and the easiest way to fix it, will be just to add the `metadata` that helm can't find. So you can follow those messages one by one and when all the `CRDs` are patched, you'll be able to install the release.

For example:

```BASH
$ helm upgrade my-release .
Error: UPGRADE FAILED: rendered manifests contain a resource that already exists. Unable to continue with update: CustomResourceDefinition "databases.kinda.rocks" in namespace "" exists and cannot be imported into the current release: invalid ownership metadata; label validation error: missing key "app.kubernetes.io/managed-by": must be set to "Helm"; annotation validation error: missing key "meta.helm.sh/release-name": must be set to "my-release"; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "default"
```

So you should add following metadata:
```YAML
metadata:
  labels:
    "app.kubernetes.io/managed-by": Helm
  annotations:
    "meta.helm.sh/release-name": my-release
    "meta.helm.sh/release-namespace": default
```
</details>
