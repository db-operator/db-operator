---
icon: lucide/user
---
# Database Users

**DbUsers** can be used to manage users on a database server.

When you create a `Database`, db-operator will create a main user, that full access to the created database. You should use this user for you workloads.
If you happen to need more users on your database, you can use `DbUser` Custom Resource

## How to create DbUsers

Let's have a look at the manifest:
```
apiVersion: kinda.rocks/v1beta1
kind: DbUser
metadata:
  name: mysql-readwrite
  namespace: default
spec:
  secretName: mysql-readwrite-secret
  accessType: readWrite
  databaseRef: mysql-db
```

### Secret Name

The `.secretName` we have already seen in the **Databases**, and here it's used almost in the same way. The only difference is that dbuser controller will reuse the **ConfigMap** that was created by the database controller, and will only create a **Secret**.


### Access Types

`DbUser` supports two access types:

- readWrite (SELECT, INSERT, UPDATE, DELETE)
- readOnly (SELECT)

### Database Reference

**DbUsers** are always attached to **Databases**. Once you have a **Database** that is ready, you can create a **DbUser** to grant additional access to it. Another important thing to consider, is that **DbUser** is a namespace resource, and it must live in the same namespace as a **Database**

### Credentials Templates

You can also use [templates](templates.md) to generate connection strings for users, but since it's reusing the database **ConfigMaps**, you are only allowed to write templated values to **Secrets**, so the `.secret` in the template should always be `true`

### Extra Labels and Annotations

With `credentials.metadata.extraLabels` and `credentials.metadata.extraAnnotations` you can add custom metadata to the **Secret** that stores the generated user credentials. These values are merged with the metadata created by the operator. Keys defined in `extraLabels` or `extraAnnotations` overwrite existing values with the same key.

Example:

```yaml
spec:
  credentials:
    metadata:
      extraLabels:
        environment: development
      extraAnnotations:
        reflector.v1.k8s.emberstack.com/reflection-allowed: "true"
        reflector.v1.k8s.emberstack.com/reflection-auto-enabled: "true"
        reflector.v1.k8s.emberstack.com/reflection-auto-namespaces: target-namespace
```

This metadata can be used by external controllers that watch annotations or require specific labels to enable Secret synchronization or reflection across namespaces.

### Using an existing user

Since version **2.22.0** it's possible to make DB Operator grant access to a database to an existing user instead of generating a new one. To do so add `.spec.existingUser: ${USERNAME}` to your manifest.

```yaml
kind: DbUser
metadata: {}
spec:
  existingUser: my-db-user
```

When using an existing user, DB Operator is not aware of the password anymore, hence the **password** field in the credentials Secret will be empty. Users must handle the authentication on their own.

It's also important to understand, that when you are switching **from/to** existing user **to/from** a generated one, you need to remove the Secret from the cluster, so it's recreated, otherwise it's either going to be empty for a generated user, or filled for an existing one. It will most probably be fixed in future versions.

### PostgreSQL deletion failures

Deleting a generated `DbUser` must remove its PostgreSQL role. If PostgreSQL cannot confirm whether the role exists, or refuses to drop it, the operator reports the error and retains the finalizer for another attempt. A role confirmed absent needs no further cleanup or role-deletion privileges.

DbUser permission cleanup can run [`DROP OWNED`](https://www.postgresql.org/docs/current/sql-drop-owned.html) in the managed database. That can delete objects owned there and revoke grants on shared objects, including other databases. `spec.existingUser` preserves the role, not its owned objects or grants. Completed cleanup is not rolled back if a later step fails.

The operator does not run object cleanup in other databases. Owned objects or grants on local objects there can still block role deletion with a dependency error (`2BP01`). Older versions treated that error as success, which could leave the role behind after removing the Kubernetes resource.

To unblock deletion:

1. Read the affected resource's Warning events and the operator's controller logs. Record its `.status.user` to identify the role. For a `DbUser`, follow `.spec.databaseRef` to the `Database` in the same namespace; that Database's `.status.database` records the PostgreSQL database name.
2. Ask the database administrator to restore any missing access or arrange owner-approved dependency cleanup. Follow PostgreSQL's [role-removal guidance](https://www.postgresql.org/docs/current/role-removal.html) to reassign objects that must survive before removing dependencies. Do not remove the finalizer as a substitute for cleanup.
3. Let the operator retry, then verify that the Kubernetes resource disappears. Confirm that a generated role is absent from `pg_roles`. For `spec.existingUser`, confirm that the role remains and its managed access has been revoked.

## Experimental features

Experimental features are added via annotations, the following features are available for `DbUsers`

### Grant To Admin on Delete

On instances where the admin is not a super user, it might not be able to drop owned by user, so we need to grant the user to the admin.

But it's not possible on the AWS instances with the `rds_iam` roles, because then admins are not able to log in with a password anymore and the db-operator will not be able to do anything on databases.

That's where this workaround might be used. It will only grant the role to the admin when a user is being removed.
```yaml
kinda.rocks/grant-to-admin-on-delete: "true"
```

### Allow Existing User

By default the operator is trying to make sure, that the user that it's trying to maintain is also created by it. Because otherwise it would be possible to make operator believe that a **Database** user that is created by the database controller should become a **readOnly** one.

So if a user already exists on a server, when a **CR** is created, operator will not try to manage it.

If you need the operator to be able to managed existing users, you need to set the following annotation:

```yaml
kinda.rocks/allow-existing-user: "true"
```
