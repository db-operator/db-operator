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

## Deleting a DbUser

Deleting a `DbUser` removes its database user, or revokes access when it manages an existing user. The referenced `Database` retains a `dbuser.<name>` finalizer until this cleanup succeeds.

Once the backend cleanup calls succeed, the operator releases its labels on the credential Secret if it still owns that Secret. It then records the Database UID and name in `status.cleanupDatabaseUID` and `status.cleanupDatabaseName`. This checkpoint does not confirm that credential template entries were removed.

On the next reconciliation, the operator removes the Database dependency finalizer before removing the DbUser finalizer. If an update fails or the operator restarts, it resumes from the checkpoint without accessing the credential Secret or database server, even if the Database has disappeared or `spec.databaseRef` or `spec.secretName` has changed.

Install the updated DbUser CRD before upgrading the controller. An older CRD can discard the checkpoint fields, so the controller keeps both finalizers until it can read back the saved checkpoint. Follow the [upgrade and rollback checks](upgrades.md) to verify the installed schema and avoid downgrading during an unfinished deletion.

If the Database is missing and no checkpoint exists, deletion stops because the operator cannot confirm that the external user was cleaned up. This also applies to resources left behind by older operator versions. Do not set the checkpoint or remove finalizers to bypass this check without verifying external cleanup.

Before a checkpoint exists, deletion also stops if the stored user identity is missing and the credential Secret has been released or belongs to another resource. The operator cannot safely use that Secret to choose a database user to delete. For legacy resources in this state, verify the original database and user identity before repairing their status; don't copy credentials from the reused Secret.

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
