-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

-- Health check for the CloudNativePG Backup CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the Backup status.phase to an Argo CD health status:
--   Healthy     - the backup completed successfully.
--   Degraded    - the backup failed, WAL archiving is failing, or the backup
--                 definition is invalid.
--   Progressing - the backup is pending, started, running, or finalizing, or
--                 status is not populated yet.
local hs = { status = "Progressing", message = "Waiting for backup to be processed" }
if obj.status ~= nil and obj.status.phase ~= nil and obj.status.phase ~= "" then
  local phase = obj.status.phase
  if phase == "completed" then
    hs.status = "Healthy"
    hs.message = "Backup completed"
  elseif phase == "failed" or phase == "walArchivingFailing" or phase == "invalid backup definition" then
    hs.status = "Degraded"
    hs.message = obj.status.error or phase
  else
    hs.status = "Progressing"
    hs.message = phase
  end
end
return hs
