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

-- Health check for the Velero Schedule CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the Schedule status.phase to an Argo CD health status:
--   Healthy     - the schedule is enabled.
--   Degraded    - the schedule failed validation (e.g. invalid cron expression).
--   Progressing - the schedule is new, or the controller has not populated status yet.
local hs = { status = "Progressing", message = "Waiting for schedule to be processed" }
if obj.status ~= nil and obj.status.phase ~= nil then
  local phase = obj.status.phase
  if phase == "Enabled" then
    hs.status = "Healthy"
    hs.message = "Schedule is enabled"
  elseif phase == "FailedValidation" then
    hs.status = "Degraded"
    hs.message = "Schedule failed validation"
  else
    hs.status = "Progressing"
    hs.message = phase
  end
end
return hs
