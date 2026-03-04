# Getting Started

DB Operator is a Kubernetes operator for managing **MySQL** and **PostgreSQL** databases through **CRDs**.

This operator doesn't launch database servers, instead it should be connected to the running ones, that's why it doesn't nececeraly require that the database is running in Kubernetes, and can be used with any server that can be accessed from inside the cluster.

After it is connected to a server, you can start managing databases and users through CRDs. When a user or a database is created, db-operator will add a `Secret` and a `ConfigMap` to the namespace where CR is deployed, they can be used by a pod to establish a connection with a database.

### Example

Let's imagine, you have an application that requires a connection to a Postgres DB, it receives credentials from the environment variable `POSTGRES_DATASOURCE`, and it needs to be in a following format: `postgresql://${USER}:${PASSWORD}@${HOSTNAME}:${PORT}/${DATABASE}?search_path=myapp`

We will talk about how to install the operator and connect it to a server later, now let's focus on the main logic. We need to create a `Database` resource:
```yaml
apiVersion: kinda.rocks/v1beta1
kind: Database
metadata:
  name: my-app
  namespace: my-namespace
spec:
  backup:
    enable: false
  credentials:
    templates:
    # - This template will be used by the operator to add POSTGRES_DATASOURCE to the secret.
    - name: POSTGRES_DATASOURCE
      secret: true
      template: '{{ .Protocol }}://{{ .Username }}:{{ .Password }}@{{ .Hostname }}:{{ .Port }}/{{ .Database }}?search_path=my-app'
  deletionProtected: true
  instance: some-postgres
  postgres:
    dropPublicSchema: true
    schemas:
    - my-app
  secretName: my-app-db-creds
```

[Read more about templates here](templates.md)


> (@allanger): This logic might be changed when the `Database` resource will be upgrade to the version `v1`
After the reconciliation you should be able to find a `ConfigMap` and a `Secret` in the namespace `my-namespace`. They both will be called `my-app-db-creds`, and you can use them to connect your application to the database. No manual interactions are needed, everything is managed within a Kubernetes cluster.

## Install db-operator

We distribute DB Operator as a `helm` chart. You don't have to use it, but if you want to be able to get support, it's would be easier for us if you use the chart.

The charts are released as s simple helm repo as well as a an OCI artifact.

To use the repo, run the following:

```sh
$ helm repo add db-operator https://db-operator.github.io/charts
$ helm search repo db-operator
```

OCI artifacts are available under `ghcr.io/db-operator/charts/`

To install the chart, run the following:

```sh
$ helm install db-operator/db-operator
# -- Or OCI
$ helm install ghcr.io/db-operator/charts/db-operator:${CHART_VERSION}
```

More info about the db-operator chart can be found in the [README.md](https://github.com/db-operator/charts/tree/main/charts/db-operator)

## Create a DbInstance

After the operator is installed, you need to connect it to a database. For this the `db-instances` chart can be used.

Create a following `values.yaml` file:
```yaml
# -- values.yaml
dbinstances:
  instance1:
    engine: postgres
    monitoring:
      enabled: false
    secrets:
      adminUser: admin # Root username
      adminPassword: password # Root password
    generic:
      host: postgres.databases.svc.cluster.local # Host
      port: 5432
```

And install a helm chart like this:

```sh
$ helm install db-operator/db-instances -f ./values.yaml
```

To check the DbInstance status, run:
```sh
kubectl get dbinstance instance1

NAME              PHASE          STATUS
instance1         Running        true
```

If `.status.Status` is `true`, it means that you can create Databases on this instance.

You can read more about DbInstances [here](dbinstance.md)

## Create a Database

After the instancei is ready, you can start managing databases with the operator. Databases are not packaged in any helm chart, because they are supposed to be parts of the end applications, like an `Ingress` or a `PVC`, - something that your pod will need to run. Let's create a database:

```yaml
# - db.yaml
apiVersion: kinda.rocks/v1beta1
kind: Database
metadata:
  name: database-1
spec:
  backup:
    enable: false
  deletionProtected: false
  instance: instance1
  secretName: database-1-creds
```

```sh
$ kubectl apply -f db.yaml
$ kubectl get db
NAME         STATUS   PROTECTED   DBINSTANCE         OPERATORVERSION   AGE
database-1   true     false       instance1          2.19.0            31s
```

When `.status.Status` is `true`, it means that you can use your database.

You can read more about databases [here](database.md)

## F.A.Q.

### How to add ownerReferences to Secrets and ConfigMaps created by the operator?

DB Operator is designed in a way, that an application should be able to connect to a database, even if the operator was accidentally removed with all the CRDs. That's why by default Secrets and ConfigMaps are just created in the cluster without owner references. But if you would like these resources to be cleaned up after a databases is removed, or you need to see connection between them in ArgoCD, you can set the `.spec.cleanup` to `true`

If ArgoCD is used to manage Databases and the `cleanup` is set to `true`, please make sure that the `PrunePropagationPolicy` is not set to `foreground`, because db-operator is using secrets to understand which Database must be removed, and with the `foreground` policy the secret is removed before the Database, that makes it impossible for the operator to finish the reconciliation.

### How to connect the operator to an existing Database?

DB Operator is reading the secret using the `spec.secretName` entry. If this secret doesn't exist, operator will create it and read data out of it. But if a secret is found, it will try to get items that are required to connect to a database from there.

There are the keys, they a secret must contain:

```
# for PosrgreSQL
POSTGRES_DB: $DATABASE_NAME
POSTGRES_PASSWORD: $PASSWORD
POSTGRES_USER: $USERNAME
# and for MySQL
DB: $DATABASE_NAME
PASSWORD: $PASSWORD
USER: $USER
```

### How to rotate passwords?

To rotate a db password with a help of DB Operator, it's enough to remove/update the secret that is used by a database.
