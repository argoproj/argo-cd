-- Health check for the Knative Eventing Trigger CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Knative resources expose an aggregate "Ready" status condition (knative duck):
--   Healthy     - Ready is True.
--   Degraded    - Ready is False.
--   Progressing - Ready is Unknown, or status is not populated yet.
local hs = { status = "Progressing", message = "Waiting for status update." }
if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message or "Trigger is ready"
      elseif condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message or condition.reason or "Trigger is not ready"
      else
        hs.status = "Progressing"
        hs.message = condition.message or condition.reason or "Waiting for status update."
      end
      return hs
    end
  end
end
return hs
