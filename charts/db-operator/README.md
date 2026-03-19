# db-operator

![Version: 3.1.0](https://img.shields.io/badge/Version-3.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 2.24.0](https://img.shields.io/badge/AppVersion-2.24.0-informational?style=flat-square)

This operator lets you manage databases in a Kubernetes native way, even if they are not deployed to Kubernetes

**Homepage:** <https://github.com/db-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Nikolai Rodionov | <iam@allanger.xyz> | <https://badhouseplants.net> |

## Source Code

* <https://github.com/db-operator/db-operator>

## Requirements

Kubernetes: `>= 1.32-prerelease`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nameOverride | string | `""` |  |
| clusterDomain | string | `"cluster.local"` |  |
| image.registry | string | `"ghcr.io"` |  |
| image.repository | string | `"db-operator/db-operator"` |  |
| image.tag | string | `""` |  |
| image.pullPolicy | string | `"Always"` |  |
| metadata.labels | object | `{}` |  |
| metadata.annotations | object | `{}` |  |
| crds.install | bool | `true` |  |
| crds.keep | bool | `true` |  |
| crds.annotations | object | `{}` |  |
| controller.logLevel | string | `"info"` |  |
| controller.extraArgs | list | `[]` |  |
| controller.service.annotations | object | `{}` |  |
| controller.service.type | string | `"ClusterIP"` |  |
| controller.service.port | int | `8080` |  |
| controller.serviceAccount.name | string | `""` |  |
| controller.serviceAccount.create | bool | `true` |  |
| controller.rbac.create | bool | `true` |  |
| controller.args.reconcileInterval | string | `"60"` |  |
| controller.args.watchNamespace | string | `""` |  |
| controller.args.checkForChanges | bool | `false` |  |
| controller.serviceMonitor.enabled | bool | `false` |  |
| controller.config.instance.google.proxy.nodeSelector | object | `{}` |  |
| controller.config.instance.google.proxy.image | string | `"ghcr.io/db-operator/db-auth-gateway:v0.1.10"` |  |
| controller.config.instance.google.proxy.metricsPort | int | `9090` |  |
| controller.config.instance.generic | object | `{}` |  |
| controller.config.instance.percona.proxy.image | string | `"severalnines/proxysql:2.0"` |  |
| controller.config.instance.percona.proxy.metricsPort | int | `9090` |  |
| controller.config.backup.activeDeadlineSeconds | int | `600` |  |
| controller.config.backup.nodeSelector | object | `{}` |  |
| controller.config.backup.postgres.image | string | `"kloeckneri/pgdump-gcs:latest"` |  |
| controller.config.backup.mysql.image | string | `"kloeckneri/mydump-gcs:latest"` |  |
| controller.config.backup.resources.requests.memory | string | `"64Mi"` |  |
| controller.config.backup.resources.requests.cpu | float | `0.2` |  |
| controller.config.monitoring.promPushGateway | string | `""` |  |
| controller.config.monitoring.nodeSelector | object | `{}` |  |
| controller.config.monitoring.postgres.image | string | `"wrouesnel/postgres_exporter:latest"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.query | string | `"SELECT userid, pgss.dbid, pgdb.datname, queryid, query, calls, total_time, mean_time, rows FROM pg_stat_statements pgss LEFT JOIN (select oid as dbid, datname from pg_database) as pgdb on pgdb.dbid = pgss.dbid WHERE not queryid isnull ORDER BY mean_time desc limit 20"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[0].userid.usage | string | `"LABEL"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[0].userid.description | string | `"User ID"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[1].dbid.usage | string | `"LABEL"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[1].dbid.description | string | `"database ID"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[2].datname.usage | string | `"LABEL"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[2].datname.description | string | `"database NAME"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[3].queryid.usage | string | `"LABEL"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[3].queryid.description | string | `"Query unique Hash Code"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[4].query.usage | string | `"LABEL"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[4].query.description | string | `"Query class"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[5].calls.usage | string | `"COUNTER"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[5].calls.description | string | `"Number of times executed"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[6].total_time.usage | string | `"COUNTER"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[6].total_time.description | string | `"Total time spent in the statement, in milliseconds"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[7].mean_time.usage | string | `"GAUGE"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[7].mean_time.description | string | `"Mean time spent in the statement, in milliseconds"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[8].rows.usage | string | `"COUNTER"` |  |
| controller.config.monitoring.postgres.queries.pg_stat_statements.metrics[8].rows.description | string | `"Total number of rows retrieved or affected by the statement"` |  |
| webhook.enabled | bool | `true` |  |
| webhook.logLevel | string | `"info"` |  |
| webhook.extraArgs | list | `[]` |  |
| webhook.podLabels | object | `{}` |  |
| webhook.rbac.create | bool | `true` |  |
| webhook.serviceAccount.name | string | `""` |  |
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

