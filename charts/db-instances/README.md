# db-instances

![Version: 2.7.1](https://img.shields.io/badge/Version-2.7.1-informational?style=flat-square) ![AppVersion: 1.0](https://img.shields.io/badge/AppVersion-1.0-informational?style=flat-square)

Database Instances for db operator

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| dbinstances | object | `{}` |  |
| nodeSelector | object | `{}` |  |
| exporter.postgres.image | string | `"prometheuscommunity/postgres-exporter:v0.15.0"` |  |
| mysql.enabled | bool | `false` |  |
| postgresql.enabled | bool | `false` |  |
| tests | object | `{"monitoring":{"enabled":false}}` | ------------------------------------------------------------------- |

