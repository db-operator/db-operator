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


## Next
- [Creating Databases](#creating-databases)
  - [Before start](#before-start)
  - [Next](#next)
    - [CreatingDatabases](#creatingdatabases)
    - [ConnectingToTheDatabase](#connectingtothedatabase)
    - [CheckingDatabaseStatus](#checkingdatabasestatus)
    - [PostgreSQL](#postgresql)

### Creating Databases

Create Database custom resource

```YAML
apiVersion: "kinda.rocks/v1beta1"
kind: "Database"
metadata:
  name: "example-db"
spec:
  instance: example-gsql # This has to be match with DbInstance name
  deletionProtected: false # Protection to not delete database when custom resource is deleted
  credentials:
    secretName: example-db-credentials # DB Operator will create secret with this name. it contains db name, user, password
    setOwnerReference: false # Should secret be removed when a database is removed
    templates:
      - name: USER_PASSWORD
        template: "{{ .Username }}-{{ .Password }}"
```
With `credentials.templates` you can add new entries to database ConfigMap and Secret. This feature uses go templates, so you can build custom string using either predefined helper functions:

- Protocol: Depends on the db engine. Possible values are mysql/postgresql
- Hostname: The same value as for db host in the connection configmap
- Port: The same value as for db port in the connection configmap
- Database: The same value as for db name in the creds secret
- Username: The same value as for database user in the creds secret
- Password: The same value as for password in the creds secret

... or getting data directly from a data source, possible options are.

- Secret: Query data from the Secret
- Query: Get data directly from the database

When using `Secret`, you can query the previously created secret to template a new one, e.g.:

```yaml
spec:
  credentials:
    templates:
      - name: TMPL_1
        template: "test"
      - name: TMPL_2
        template: "{{ .Secret \"TMPL_1\"}}"
      - name: TMPL_3
        template: "{{ .Secret \"TMPL_2\"}}"
```

When using `Query` you need to make sure that you query returns only one value. For example:

```yaml
...
    templates:
      - name: POSTGRES_VERSION
        template: "{{ .Query \"SHOW server_version;\" }}"

```
All the values will be appended to the secret which name was set in `spec.credentials.secretName`


If no `credentials.templates` are specified, a default connection string example will be added to the secret:
```YAML
CONNECTION_STRING: "jdbc:{{ .Protocol }}://{{ .UserName }}:{{ .Password }}@{{ .DatabaseHost }}:{{ .DatabasePort }}/{{ .DatabaseName }}"
```

#### Postgres specific options

##### Extensions

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

##### Schemas

It's possible to drop the `Public` schema after the database creation, or/and to create additional schemas:

```YAML
spec:
  postgres:
    dropPublicSchema: true # Do not set it, or set to false if you don't want to drop the public schema
    schemas: # The user that's going to be created by db-operator, will be granted all privileges on these schemas
      - schema_1
      - schema_2
```

If you initialize a database with `dropPublicSchema: false` and then later change it to `true`, or add schemas with the `schemas` field and later try to remove them by updating the manifest, you may be unable to do that. Because `db-operator` won't use `DROP CASCADE` for removing schemas, and if there are objects depending on a schema, someone with admin access will have to remove these objects manually.

##### Options

Database creation options are partially supported by the operator as well (currently it's only one option, but it's about to change: https://github.com/db-operator/db-operator/issues/17)


- [Postgres Database Templates](https://www.postgresql.org/docs/current/manage-ag-templatedbs.html): to create a database from template, you need to set `.spec.postgres.template`. It's referencing to a database on the Postgres server, but not to the k8s Database resource that is created by operator, so there is no validation on the db-operator side that a template exists.

### ConnectingToTheDatabase

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
  DB_CONN: << base64 encoded ddatabase server address >>
  DB_PORT: << base64 encoded ddatabase server port >>
  DB_PUBLIC_IP: << base64 encoded ddatabase server public ip >>
  POSTGRES_USER: << base64 encoded user name (generated by db operator) >>
  CONNECTION_STRING: << base64 encoded database connection string (a templated value) >>
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
  DB_CONN: << base64 encoded ddatabase server address >>
  DB_PORT: << base64 encoded ddatabase server port >>
  DB_PUBLIC_IP: << base64 encoded ddatabase server public ip >>
  CONNECTION_STRING: << base64 encoded database connection string (a templated value) >>
```

Why are we putting non-sensitive values to a secret?

We used to have a separation between sensitive and non-sensitive values before, and were putting "public" values to a ConfigMap, but we've removed ConfigMaps support because of several reasons

- A ConfigMap name was never specified explicitly. We could have also added an additional field to the `CRD`, but it would add complexity (even though it would be a minor one) to the `CR` and hence it would put it on users
- It was making the code more complex
- Credentials templates were making it easy to expose secret via ConfigMap.

So after all, we've decided that it wasn't worth it, but we're open for suggestions.

By default, Secrets are created without Owner References, so they won't be removed when a `Database` resource is gone. If you want them to get deleted too, you need to make db-operator add owner references to them.
```YAML
apiVersion: "kinda.rocks/v1beta1"
kind: "Database"
metadata:
  name: "example-db"
spec:
  credentials:
    setOwnerReference: true

```

If ArgoCD is used to manage Databases and the `cleanup` is set to `true`, please make sure that the `PrunePropagationPolicy` is not set to `foreground`, because db-operator is using secrets to understand which Database must be removed, and with the `foreground` policy the secret is removed before the Database, that makes it impossible for the operator to finish the reconciliation.

If this feature is enabled, then `Database` becomes an owner of Secrets and ConfigMaps, and by removing a database, you'll also remove them.

### ConnectingToTheDatabase

By using the secret created by operator after database creation, pods in Kubernetes can connect to the database.
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
            secretKeyRef:
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
| `Finishing`           | Setting status of `Database` to true |
| `Ready`               | `Database` is created and all the configs are applied. Healthy status. |
| `Deleting`            | `Database` is being deleted. |
