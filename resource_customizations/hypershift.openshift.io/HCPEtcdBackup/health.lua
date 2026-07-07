-- HCPEtcdBackup represents a single etcd backup operation for a HostedControlPlane.
-- It is a short-lived, ephemeral resource; completed backups may be deleted by retention policies.
-- The backup URL is also persisted on the HostedCluster status for durability.
--
-- Documentation:
--   Etcd backup guide: https://hypershift-docs.netlify.app/how-to/etcd-backup/
--
-- Condition types and reasons are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/etcdbackup_types.go
--
-- Single condition type:
--   BackupCompleted - tracks the lifecycle of the backup operation
--     Reasons:
--       BackupSucceeded  - backup snapshot uploaded successfully  => True
--       BackupInProgress - snapshot upload is in progress         => False
--       BackupRejected   - backup request was rejected            => False
--       EtcdUnhealthy    - etcd is unhealthy, cannot snapshot     => False
--       BackupFailed     - upload or snapshot failed permanently  => False
--
-- ArgoCD health mapping:
--   BackupCompleted=True                   => Healthy
--   BackupCompleted=False, reason=BackupFailed => Degraded
--   BackupCompleted=False, other reasons   => Progressing
--   No conditions                          => Progressing
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for HCPEtcdBackup status"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "BackupCompleted" then
    if condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
    if condition.reason == "BackupFailed" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    -- BackupInProgress, BackupRejected, EtcdUnhealthy
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for backup to start"
return hs
