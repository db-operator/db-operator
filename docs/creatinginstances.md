# Creating DbInstances

## What are DbInstances?

**DbInstance** is a **CRD** that is supposed to make the operator aware of database servers it should be connected to. It's nothing more than a Kubernetes resource, and db-operator doesn't have any logic for bootstrapping database installations. **DbInstance** is necessary to create databases.

## Let's get started
You can use an existing database server or create/use Google Cloud SQL instance to create a **DbInstance**.

* [Using existing database server](#GenericDbInstance)
* [Checking DbInstance status](#CheckingStatus)
* [Using SSL connection](#UsingSSLconnection)

### DbInstance

#### Prerequisite
* running database server accessible by ip or hostname

Create a new secret containing admin username and password of an instance. This user will be used by the operator for managing resource on a database, so it has to have sufficient permissions.

```SHELL
$ kubectl create secret generic example-generic-admin-secret --from-literal=user=<admin user name> --from-literal=password='<admin user password>'
```

Or use existing secret created by stable mysql/postgres helm chart.

Create **DbInstance** custom resource.
```YAML
apiVersion: kinda.rocks/v1beta1
kind: DbInstance
metadata:
  name: example-generic
spec:
  adminSecretRef:
    Name: example-generic-admin-secret
    Namespace: <namespace of secret existing>
  engine: <postgres or mysql>
  generic:
    host: <host address to connect database server>
    port: <port to connect database server>
```

### CheckingStatus

Check **DbInstance** status
```
kubectl get dbin example-generic
```

The output should be like
```
NAME              PHASE      STATUS
example-generic   Creating   false
```

Possible phases and meanings

| Phase                 | Description                           |
|-------------------    |-----------------------                |
| `Validating`          | Validate all the necessary fields provided in the resource spec |
| `Creating`            | Create (only google type) or check if the database server is reachable |
| `Broadcasting`        | Trigger `Database` phase cycle if there was an update on `DbInstance` |
| `Running`             | Backend database server connection checked and ready for database creation |


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
  sslConnection:
    enabled: false
    skip-verify: false
...
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
  sslConnection:
    enabled: true
    skip-verify: true
...
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
  sslConnection:
    enabled: true
    skip-verify: false
...
```

> * Self-signed certificates with verify option is currently not supported.
