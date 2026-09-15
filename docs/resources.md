# Resources

The table lists the kinds this operator manages, the UpCloud product each
kind maps to, where the UpCloud identity lands in status, and any Secrets
the operator produces.

| Kind | UpCloud product | Identity in status | Produced Secrets | Notes |
|---|---|---|---|---|
| Network | SDN private network | `status.uuid` | none | Attach with `routerRef` (a Router CR) or `routerUUID` (an existing UpCloud router). Zone is immutable. |
| Router | SDN router | `status.uuid` | none | `status.attachedNetworks` lists the networks attached in UpCloud. Deletion waits while networks remain attached. |
