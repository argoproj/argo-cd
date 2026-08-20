-- Health check for the Velero Restore CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the Restore status.phase to an Argo CD health status:
--   Healthy     - the restore completed successfully.
--   Degraded    - the restore failed, partially failed, or failed validation.
--   Progressing - the restore is new, in progress, or finalizing,
--                 or the controller has not populated status yet.
local hs = { status = "Progressing", message = "Waiting for restore to be processed" }
if obj.status ~= nil and obj.status.phase ~= nil then
  local phase = obj.status.phase
  if phase == "Completed" then
    hs.status = "Healthy"
    hs.message = "Restore completed"
  elseif phase == "FailedValidation" or phase == "Failed" or phase == "PartiallyFailed" or phase == "WaitingForPluginOperationsPartiallyFailed" or phase == "FinalizingPartiallyFailed" then
    hs.status = "Degraded"
    hs.message = obj.status.failureReason or phase
  else
    hs.status = "Progressing"
    hs.message = phase
  end
end
return hs
