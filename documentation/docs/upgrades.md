# Upgrading DbUser deletion checkpoints

The controller needs `status.cleanupDatabaseUID` and `status.cleanupDatabaseName` in the installed DbUser CRD to resume deletion after a Database disappears. Apply that schema before rolling out the controller. See [DbUser deletion](dbuser.md#deleting-a-dbuser) for the checkpoint's meaning and limits.

## Rollout

Use a shell with `kubectl` configured for the target cluster and access to update the DbUser CRD, roll out the operator Deployment, and read DbUsers across namespaces. Run the file-based commands below from a checkout of the operator version you intend to deploy. Retain the current deployment configuration and back up the CRD and DbUser resources through your normal backup process.

Pause new DbUser deletion requests during the rollout. If Helm or GitOps manages the CRD, update that deployment source too, so it won't restore an older schema. The chart [renders CRDs through an ordinary template](https://github.com/db-operator/charts/blob/main/charts/db-operator/templates/crds.yaml), not Helm's special `crds/` directory. Changing only the controller image does not update the schema bundled in the chart.

Preview and apply the DbUser CRD:

```sh
kubectl diff -f config/crd/bases/kinda.rocks_dbusers.yaml
kubectl apply -f config/crd/bases/kinda.rocks_dbusers.yaml
```

`kubectl diff` exits with status 1 when it finds changes; review those changes before applying. Then check the installed schema:

```sh
kubectl wait --for=condition=Established crd/dbusers.kinda.rocks --timeout=60s
kubectl explain dbusers.status.cleanupDatabaseUID --api-version=kinda.rocks/v1beta1
kubectl explain dbusers.status.cleanupDatabaseName --api-version=kinda.rocks/v1beta1
```

Both `explain` commands must show a string field and its description. If either field is missing, stop and correct the CRD in the deployment source before upgrading the controller.

Roll out the controller through your existing Helm or deployment workflow. Set the namespace and Deployment name below to match your installation; the example names match the repository's source deployment:

```sh
OPERATOR_NAMESPACE=db-operator-system
OPERATOR_DEPLOYMENT=db-operator-controller-manager
kubectl -n "$OPERATOR_NAMESPACE" rollout status deployment/"$OPERATOR_DEPLOYMENT" --timeout=120s
kubectl -n "$OPERATOR_NAMESPACE" logs deployment/"$OPERATOR_DEPLOYMENT" --tail=100
```

Expect a successful rollout and no checkpoint-persistence errors in the controller logs. Repeat the two `explain` checks after the rollout, since Helm or GitOps may have applied its own CRD manifest. If the controller reports `cleanup checkpoint was not persisted`, fix the installed schema; don't remove finalizers to bypass the error.

## Rollback

Keep new DbUser deletion requests paused and inspect pending deletions:

```sh
kubectl get dbusers -A -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,DELETING:.metadata.deletionTimestamp'
```

Do not downgrade while any row has a deletion timestamp, even if its cleanup UID is empty. Let the current controller finish all pending deletions first; otherwise one can save a checkpoint after this check. An older controller does not understand the checkpoint and can leave a DbUser stuck once its Database has disappeared.

After all pending deletions finish, restore the previous controller image and configuration while retaining the updated CRD schema in your deployment source. Keep new deletion requests paused until the rollback finishes. Avoid a chart rollback that also restores an older CRD, and repeat the installed-schema checks afterward. Rolling back the controller cannot restore database users or permissions already removed by deletion.
