-- Health check for the Velero Backup CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the Backup status.phase to an Argo CD health status:
--   Healthy     - the backup completed successfully.
--   Degraded    - the backup failed, partially failed, or failed validation.
--   Progressing - the backup is new, queued, in progress, finalizing, or deleting,
--                 or the controller has not populated status yet.
local hs = { status = "Progressing", message = "Waiting for backup to be processed" }
if obj.status ~= nil and obj.status.phase ~= nil then
  local phase = obj.status.phase
  if phase == "Completed" then
    hs.status = "Healthy"
    hs.message = "Backup completed"
  elseif phase == "FailedValidation" or phase == "Failed" or phase == "PartiallyFailed" or phase == "WaitingForPluginOperationsPartiallyFailed" or phase == "FinalizingPartiallyFailed" then
    hs.status = "Degraded"
    hs.message = obj.status.failureReason or phase
  else
    hs.status = "Progressing"
    hs.message = phase
  end
end
return hs
