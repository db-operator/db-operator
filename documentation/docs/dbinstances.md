---
icon: lucide/server
---
# Database Instances

DbInstance is a resource that connects the operator to a Database server and defines some rules for Database/DbUser management.

## How to configure a DbInstance

You need to have a **PostgreSQL** or a **MySQL** server running, and it has to be accessible by the operator. You also need a user with sufficient permissions, if it's fine in your environment, I would suggest to use an admin user.

A lot of DbInstance values are using the internal `ValueSource` type, so once you know how to use it, you can configure a DbInstance resource.

### ValueSource

Value source allows user to either set a value directly or read a value from any ConfigMap or Secret in the cluster. For example, let's have a look at the admin username:

This will read a key "user" from a secret named "my-secret" from the "my-namespace" namespace:

```yaml
   username:
      valueFrom:
        secret:
          key: user
          name: my-secret
          namespace: my-namespace
```

This will do the same, but with a configmap instead of a secret:

```yaml
   username:
      valueFrom:
        configMap:
          key: user
          name: my-secret
          namespace: my-namespace
```

And this one will simple set a value directly from the DbInstance manifest:

```yaml
   username:
      value: my-username
```

Value can only be a string, so if you define a port as a value, you should use quotation marks: `"5432"`

If you have a db-operator webhook enabled, it will only allow reading password from a secret.

### Creating a DbInstance

Now let's get started:

```yaml
apiVersion: kinda.rocks/v1
kind: DbInstance
metadata:
  name: cloudnative-pg
spec:
  auth:
    username:
      value: admin
    password:
      valueFrom:
        secret:
          namespace: databases
          name: cloudnative-admin-creds
          key: password
  endpoint:
    host:
      value: cloudnative-pg.databases.svc.cluster.local
    port:
      value: "5432"
  engine: postgres
```

## Automatic Reconciliation on Resource Changes

The `DbInstance` controller automatically reconciles when referenced `Secrets` or `ConfigMaps` change. The controller adds the label `kinda.rocks/dbinstance-name: <dbinstance-name>` to all Secrets or ConfigMaps that are referenced by a particular `DbInstance`.

**Note:** Since the operator uses a label to track the relationship, there is a **one-to-one relationship** between the `DbInstance` and its referenced resources. Sharing a Secret or ConfigMap between multiple `DbInstance` objects is discouraged, as the labeling won't be consistent.

## Additional configurations

### NamespaceFilters

DbInstance can only allow the operator to create Databases only from specified namespaces:

```yaml
kind: DbInstance
spec:
  namespaceFilters:
    - test-*
```

Would only allow Databases that match the `test-*` regex

### GrantRules

It's possible to set rules to automatically add extra grants to Databases. It can be used, if for example developers connect to a database server with a "developer" role, and you would like all dev Databases to be available for them:

```yaml
kind: DbInstance
spec:
  grantRules:
    - namespace: dev-*
      role: developer
      accessLevel: readWrite
```

Currently grants are automatically created, but automatically removing them is not yet supported.

### Allowed Privileges

To use the `.extraPrivileges` feature of `DbUsers`, you also need to enabled the privileges on the instance level. Extra privileges is a list of roles that can be granted to `DbUsers`. For example:

```yaml
spec:
  generic:
    allowedPrivileges:
      - namespace: dev-*
        role: readOnlyAdmin
      - namespace: '*'
        role: rds-iam
```

Then you will be able to assigned these roles to DbUsers. The roles are not managed by the operator, they must be already present on a server when a user is created.

### Instance Vars

It may happen that you need to share the same variable in the Database/DbUser [templates](templates.md). Let's say you have a **RW** and **RO** urls, and your application need two env variables to connect: one - for actibely writing, and another - only for reading.

The **RW** URL will be available anyway, but how to set a **RO** one.

Without the instance vars you could just create a template with a hardcoded string in it:

```yaml
templates:
  - name: PG_READONLYHOST
    secret: false
    template: "my-read-only-postgres-url.test"
```

Or you can use the instance variables.

```yaml
kind: DbInstance
spec:
  instanceVars:
    PG_READONLYHOST: my-read-only-postgres-url.test
```

And then later use it in a template like that:
```yaml
templates:
  - name: PG_READONLYHOST
    secret: false
    template: '{{ .instanceVar "PG_READONLYHOST" }}'
```

If a value of a variable is changed on the instance, it will be also synced for each Database and User.

### UsingSSLconnection

By default, db-operator use non ssl connection to database instances.
In case you are using public connection, you can enable ssl connection.
To use ssl connection, set `sslConnection.enabled` to `true` in `DbInstance` spec.

#### No SSL

* postgres: disable
* mysql: disabled

```YAML
apiVersion: kinda.rocks/v1beta1
kind: DbInstance
metadata:
  name: example-generic
spec:
  endpoint:
    host: {}
    port: {}
    sslConnection:
      enabled: false
      skip-verify: false
```

#### Always SSL (skip verification)

* postgres: require
* mysql: required

```YAML
apiVersion: kinda.rocks/v1beta1
kind: DbInstance
metadata:
  name: example-generic
spec:
  endpoint:
    host: {}
    port: {}
    sslConnection:
      enabled: true
      skip-verify: true
```

#### Always SSL (verify that the certificate presented by the server was signed by a trusted CA)

* postgres: verify-ca
* mysql: verify_ca

```YAML
apiVersion: kinda.rocks/v1beta1
kind: DbInstance
metadata:
  name: example-generic
spec:
  endpoint:
    host: {}
    port: {}
    sslConnection:
      enabled: true
      skip-verify: false
```

### DbInstance Status

DbInstance status contains some information about Database server:

- Database version: For example: 17.6 (Debian 17.6-2.pgdg11+1)
- serverStatus.Databases: It's a list of all databases found on a server
- serverStatus.Users: It's a list of all users found on a server
- serverStatus.DatabaseCount: An amount of all databases on a server
- serverStatus.ManagedDatabaseCount: An amount of all databases on a server that are managed by the operator

Lists of users and databases can be disabled by setting the `databaseAwareness` to `false` in the operator config.

Database version is not checked on each reconciliation, but only once the TTL exceeded. TTL can be set in the configas `serverVersionTTL`, default value is 1 hour.
