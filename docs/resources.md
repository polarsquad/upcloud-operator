# Resources

The table lists the kinds this operator manages, the UpCloud product each
kind maps to, where the UpCloud identity lands in status, and any Secrets
the operator produces.

| Kind | UpCloud product | Identity in status | Produced Secrets | Notes |
|---|---|---|---|---|
| Network | SDN private network | `status.uuid` | none | Attach with `routerRef` (a Router CR) or `routerUUID` (an existing UpCloud router). Zone is immutable. |
| Router | SDN router | `status.uuid` | none | `status.attachedNetworks` lists the networks attached in UpCloud. Deletion waits while networks remain attached. |
| ManagedDatabase | Managed Database (pg, mysql, valkey, opensearch) | `status.uuid` | `<name>-connection` with keys `uri`, `host`, `port`, `user`, `password`, `dbname`, `sslmode` | `spec.type`, `spec.zone` immutable; `spec.properties` is free-form JSON validated server-side; `spec.powered=false` shuts the service down. |
| ManagedDatabaseUser | Managed Database user | `status.username` | `<name>-credentials` with keys `username`, `password`, `host`, `port`, `uri` | Identity is `(serviceUUID, username)`; no label adoption. `spec.passwordSecretRef` sets the password (the API generates one otherwise). Deleting the primary user is refused. |
| ManagedDatabaseLogicalDatabase | Managed Database logical database | `status.name` | none | Identity is `(serviceUUID, name)`; `lcCollate`/`lcCtype` are create only, so a collation change requires recreation. |
