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
