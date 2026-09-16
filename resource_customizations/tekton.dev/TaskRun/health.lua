-- Health check for the Tekton TaskRun CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Tekton reports run status via the standard "Succeeded" condition:
--   Healthy     - Succeeded is True (the run completed successfully).
--   Degraded    - Succeeded is False (the run failed, timed out, or was cancelled).
--   Progressing - Succeeded is Unknown (the run is pending or running), or status
--                 is not populated yet.
local hs = { status = "Progressing", message = "Waiting for run to start" }
if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Succeeded" then
      if condition.status == "True" then
        hs.status = "Healthy"
      elseif condition.status == "False" then
        hs.status = "Degraded"
      else
        hs.status = "Progressing"
      end
      hs.message = condition.message or condition.reason
      return hs
    end
  end
end
return hs
