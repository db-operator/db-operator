bootstrap: cert-manager db-operator db-instances

restart:
	kubectl -n db-operator rollout restart deploy db-operator

cert-manager:
	helmfile -l name=cert-manager apply

db-operator: 
	helmfile -l name=db-operator apply

db-instances:
	helmfile -l kind=database apply && \
		helmfile -l name=db-instances apply

refresh_dbs: delete_dbs db-instances

delete_dbs:
	helmfile -l kind=database destroy && \
		kubectl --namespace db-operator delete pvc data-mysql-instance-0 && \
		kubectl --namespace db-operator delete pvc data-postgres-instance-postgresql-0 \

mysql_connect: 
	kubectl exec -it mysql-instance-0 -- mysql --host=localhost --user=root --password=123123!!

postgres_connect:
	kubectl exec -it postgres-instance-postgresql-0 -- bash -c 'PGPASSWORD=123123!! psql -h localhost -U postgres -d postgres'

