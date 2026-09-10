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

-- Health check for the Prometheus Operator ThanosRuler CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- This reads the ThanosRuler "Available" status condition (set by the
-- Prometheus Operator) and maps it to an Argo CD health status:
--   Healthy     - the Available condition is True (all instances available).
--   Progressing - Available is not True because some pods are not ready yet,
--                 or the operator has not populated status yet.
--   Degraded    - Available is not True for any other reason.
local hs={ status = "Progressing", message = "Waiting for initialization" }

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do

      if condition.type == "Available" and condition.status ~= "True" then
        if condition.reason == "SomePodsNotReady" then
          hs.status = "Progressing"
        else
          hs.status = "Degraded"
        end
        hs.message = condition.message or condition.reason
      end
      if condition.type == "Available" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "All instances are available"
      end
    end
  end
end

return hs