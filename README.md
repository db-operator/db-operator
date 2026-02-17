# DB Operator

This operator lets you manage databases in a Kubernetes native way, even if they are not deployed to Kubernetes

## Features

DB Operator provides following features:

* Management of **MySQL** and **PostgreSQL** databases in the same way
* Create/Delete databases on the database server running outside/inside Kubernetes by creating `Database` custom resource;
* Create/Delete users on the database server running outside/inside Kubernetes by creating `DbUser` custom resource;
* Creating of custom connection strings using **GO templates**

## Documentation
* [Get Started](https://db-operator.github.io/documentation/)
* [Configure instances](https://db-operator.github.io/documentation/dbinstance/)
* [Manage databases](https://db-operator.github.io/documentation/database/)
* [Manage users](https://db-operator.github.io/documentation/dbuser/)
* [A deeper look at templates](https://db-operator.github.io/documentation/templates/)

## Quickstart

### To install DB Operator with helm:

```
$ helm repo add db-operator https://db-operator.github.io/charts/
$ helm install --name my-release db-operator/db-operator
```

To see more options of helm values, [see the chart repo]([https://github.com/db-operator/charts/tree/main/charts/db-operator])
