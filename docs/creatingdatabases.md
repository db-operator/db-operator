# Creating Databases

## Before start

First, in order to create a **Database** resource, a **DbInstance** resource is necessary. This defines the target server where the database must be created. **Database** resources require **DbInstance**. One or more **DbInstance(s)** are necessary for creating **Database(s)**

Check if DbInstance exists on cluster and running.
```
$ kubectl get dbin
```

The result should be like below.
```
NAME              PHASE      STATUS
example-generic   Running   false
```

If you get `No resources found.`, go to [how to create DbInstance](creatinginstances.md)

For more details about how it works check [here](howitworks.md)

## Next
- [Creating Databases](#creating-databases)
  - [Before start](#before-start)
  - [Next](#next)
    - [CreatingDatabases](#creatingdatabases)
    - [ConnectingToTheDatabase](#connectingtothedatabase)
    - [CheckingDatabaseStatus](#checkingdatabasestatus)
    - [PostgreSQL](#postgresql)

### CreatingDatabases

Create Database custom resource

```YAML
apiVersion: "kinda.rocks/v1beta1"
kind: "Database"
metadata:
  name: "example-db"
spec:
  secretName: example-db-credentials # DB Operator will create secret with this name. it contains db name, user, password
  instance: example-gsql # This has to be match with DbInstance name
  deletionProtected: false # Protection to not delete database when custom resource is deleted
  backup:
    enable: false # turn it to true when you want to use back up feature. currently only support postgres
    cron: "0 0 * * *"
  credentials: 
    templates:
      - name: USER_PASSWORD
        template: "{{ .Username }}-{{ .Password }}"
        secret: true
  secretsTemplates:
    CONNECTION_STRING: "jdbc:{{ .Protocol }}://{{ .UserName }}:{{ .Password }}@{{ .DatabaseHost }}:{{ .DatabasePort }}/{{ .DatabaseName }}" 
    PASSWORD_USER: "{{ .Password }}_{{ .UserName }}"
```
With `credentials.templates` you can add new entries to database ConfigMap and Secret. This feature uses go templates, so you can build custom string using either predefined helper functions:

- Protocol: Depends on the db engine. Possible values are mysql/postgresql
- Hostname: The same value as for db host in the connection configmap 
- Port: The same value as for db port in the connection configmap 
- Database: The same value as for db name in the creds secret
- Username: The same value as for database user in the creds secret
- Password: The same value as for password in the creds secret

Or getting data directly from a data source, possible options are.

- Secret: Query data from the Secret
- ConfigMap: Query data from the ConfigMap 
- Query: Get data directly from the database

When using `Secret` and `ConfigMap` you can query the previously created secret to template a new one, e.g.:

```yaml
spec:
  credentials:
    templates:
      - name: TMPL_1
        template: "test"
        secret: false
      - name: TMPL_2
        template: "{{ .ConfigMap \"TMPL_1\"}}"
        secret: true
      - name: TMPL_3
        template: "{{ .Secret \"TMPL_2\"}}"
        secret: false
```

When using `Query` you need to make sure that you query returns only one value. For example:

```yaml
...
    templates:
      - name: POSTGRES_VERSION
        secret: false
        template: "{{ .Query \"SHOW server_version;\" }}"

```


Make sure to set `.templates[].secret` to `true` when templating sensitive data, db-operator will not detect it automatically. By default, secret is set to `false`, so new entry will be added to the ConfigMap

> `secretsTemplates` are deprecated and will be completely replaced by `credentials.templates` in the `v1beta2`, so please, make sure to migrate, or let the webhook take care of it later. You can't use both: secretsTemplates and credentials.templates at the same time, please choose only one option

With `secretsTemplates` you can add fields to the database secret that are composed by any string and by any of the following templated values: 
```YAML
- Protocol: Depending on db engine. Possible values are mysql/postgresql
- UserName: The same value as for database user in the creds secret
- Password: The same value as for password in the creds secret
- DatabaseHost: The same value as for db host in the connection configmap 
- DatabasePort: The same value as for db port in the connection configmap 
- DatabaseName: The same value as for db host in the creds secret
```

If no `credentials.templates` and `secretsTemplates` are specified, a default connection string example will be added to the secret: 
```YAML
CONNECTION_STRING: "jdbc:{{ .Protocol }}://{{ .UserName }}:{{ .Password }}@{{ .DatabaseHost }}:{{ .DatabasePort }}/{{ .DatabaseName }}" 
```

For `postgres` it's also possible to drop the `Public` schema after the database creation, or to create additional schemas. To do that, you need to provide these fields: 
```YAML
postgres:
  dropPublicSchema: true # Do not set it, or set to false if you don't want to drop the public schema
  schemas: # The user that's going to be created by db-operator, will be granted all privileges on these schemas
    - schema_1
    - schema_2
```

If you initialize a database with `dropPublicSchema: false` and then later change it to `true`, or add schemas with the `schemas` field and later try to remove them by updating the manifest, you may be unable to do that. Because `db-operator` won't use `DROP CASCADE` for removing schemas, and if there are objects depending on a schema, someone with admin access will have to remove these objects manually. 

There is a support for [Postgres Database Templates](https://www.postgresql.org/docs/current/manage-ag-templatedbs.html). To create a database from template, you need to set `.spec.postgres.template`. It's referencing to a database on the Postgres server, but not to the k8s Database resource that is created by operator, so there is no validation on the db-operator side that a template exists.

After successful `Database` creation, you must be able to get a secret named like `example-db-credentials`.

```
$ kubectl get secret example-db-credentials
```
It contains credentials to connect to the database generated by operator.

For postgres,
```YAML
apiVersion: v1
kind: Secret
metadata:
  labels:
    created-by: db-operator
  name: example-db-credentials
type: Opaque
data:
  POSTGRES_DB: << base64 encoded database name (generated by db operator) >>
  POSTGRES_PASSWORD: << base64 encoded password (generated by db operator) >>
  POSTGRES_USER: << base64 encoded user name (generated by db operator) >>
  CONNECTION_STRING: << base64 encoded database connection string >>
```

For mysql,
```YAML
apiVersion: v1
kind: Secret
metadata:
  labels:
    created-by: db-operator
  name: example-db-credentials
type: Opaque
data:
  DB: << base64 encoded database name (generated by db operator) >>
  PASSWORD: << base64 encoded password (generated by db operator) >>
  USER: << base64 encoded user name (generated by db operator) >>
  CONNECTION_STRING: << base64 encoded database connection string >>
```

You should be able to get configmap with same name as secret like `example-db-credentials`.
```
$ kubectl get configmap example-db-credentials
```

It contains connection information for database server access.

```YAML
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    created-by: db-operator
  name: example-db-credentials
data:
  DB_CONN: << database server address >>
  DB_PORT: << database server port >>
  DB_PUBLIC_IP: << database server public ip >>
  ...
```

By default, ConfigMap and Secret are created without an Owner Reference, so they won't be removed if the `Database` resource is removed. If you want it to be deleted too, you need to turn on the cleanup function.
```YAML
apiVersion: "kinda.rocks/v1beta1"
kind: "Database"
metadata:
  name: "example-db"
spec:
  cleanup: true
```

If this feature is enabled, then `Database` becomes an owner of Secrets and ConfigMaps, and by removing a database, you'll also remove them. 
### ConnectingToTheDatabase

By using the secret and the configmap created by operator after database creation, pods in Kubernetes can connect to the database.
The following deployment is an example of how application pods can connect to the database.

```YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: example
  template:
    metadata:
      labels:
        app: example
    spec:
      containers:
      - name: example-app
        image: "appimage:latest"
        imagePullPolicy: Always
        env:
        - name: POSTGRES_PASSWORD_FILE
          value: /run/secrets/postgres/POSTGRES_PASSWORD
        - name: POSTGRES_USERNAME
          valueFrom:
            secretKeyRef:
              key: POSTGRES_USER
              name: example-db-credentials # has to be same with spec.secretName of Database custom resource
        - name: POSTGRES_DB
          valueFrom:
            secretKeyRef:
              key: POSTGRES_DB
              name: example-db-credentials # has to be same with spec.secretName of Database custom resource
        - name: POSTGRES_HOST
          valueFrom:
            configMapKeyRef:
              key: DB_CONN
              name: example-db-credentials # has to be same with spec.secretName of Database custom resource
        volumeMounts:
        - mountPath: /run/secrets/postgres/
          name: db-secret
          readOnly: true
      volumes:
      - name: db-secret
        secret:
          defaultMode: 420
          secretName: example-db-credentials # has to be same with spec.secretName of Database custom resource
```


### CheckingDatabaseStatus

To check **Database** status
```
kubectl get db example-db
```

The output should be like
```
NAME          PHASE   STATUS   PROTECTED   DBINSTANCE         AGE
example-db    Ready   true     false       example-generic    4h39m
```

Possible phases and meanings
| Phase                 | Description                           |
|-------------------    |-----------------------                |
| `Creating`            | On going creation of database in the database server |
| `InfoConfigMapCreating` | Generating and building configmap data with database server information |
| `InstanceAccessSecretCreating`  | When instance type is `google`, it's creating access secret in the namespace where `Database` exists.  |
| `BackupJobCreating`   | Creating backup `Cronjob` when backup is enabled in the `spec` |
| `Finishing`           | Setting status of `Database` to true |
| `Ready`               | `Database` is created and all the configs are applied. Healthy status. |
| `Deleting`            | `Database` is being deleted. |

### PostgreSQL

PostgreSQL extensions listed under `spec.postgres.extensions` will be enabled by DB Operator.
DB Operator execute `CREATE EXTENSION IF NOT EXISTS` on the target database.

```YAML
apiVersion: "kinda.rocks/v1beta1"
kind: "Database"
metadata:
  name: "example-db"
spec:
  secretName: example-db-credentials
  instance: example-gsql
  deletionProtected: false
  postgres:
    extensions:
      - pgcrypto
      - uuid-ossp
      - plpgsql
```
When monitoring is enabled on DbInstance spec, `pg_stat_statements` extension will be enabled.
If below error occurs during database creation, the module must be loaded by adding pg_stat_statements to shared_preload_libraries in postgresql.conf on the server side.
```
ERROR: pg_stat_statements must be loaded via shared_preload_libraries
```
