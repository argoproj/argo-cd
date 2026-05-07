-- Health check for VictoriaMetrics operator CRDs.
-- Status field reference:
--   https://github.com/VictoriaMetrics/operator/blob/master/api/operator/v1beta1/vmextra_types.go
-- UpdateStatus values: "expanding", "operational", "failed", "paused".
-- Request: https://github.com/VictoriaMetrics/operator/issues/1181

local hs = { status = "Progressing", message = "Waiting for the operator to reconcile" }

-- Status not yet reported by the operator.
if obj.status == nil then
  return hs
end

-- The operator has not observed the latest spec yet.
if obj.metadata ~= nil and obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil
    and obj.status.observedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for the operator to observe the latest generation"
  return hs
end

local updateStatus = obj.status.updateStatus
local reason = obj.status.reason or ""

-- operational => Healthy
if updateStatus == "operational" then
  hs.status = "Healthy"
  hs.message = "All components are operational"
-- expanding => rollout in progress
elseif updateStatus == "expanding" then
  hs.status = "Progressing"
  hs.message = reason ~= "" and reason or "Rollout is in progress"
-- paused => reconciliation intentionally paused by the user
elseif updateStatus == "paused" then
  hs.status = "Suspended"
  hs.message = reason ~= "" and reason or "Reconciliation is paused"
-- failed => reconciliation error
elseif updateStatus == "failed" then
  hs.status = "Degraded"
  hs.message = reason ~= "" and reason or "Reconciliation failed"
else
  hs.status = "Progressing"
  hs.message = "Waiting for the operator to report status"
end

return hs
