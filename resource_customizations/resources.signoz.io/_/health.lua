-- Health check for SigNoz operator resources (https://github.com/SigNoz/signoz-operator).
-- The status is not kstatus: there are no Reconciling or Stalled conditions. Every content kind
-- (Dashboard, Rule, SavedView, ...) shares one condition vocabulary: Ready, Synced, Terminal,
-- Recoverable and Suspended. ProviderConfig and ClusterProviderConfig report only Ready.
-- Condition semantics: https://github.com/SigNoz/signoz-operator/blob/main/docs/core-status.md

local hs = {
  status = "Progressing",
  message = "Waiting for SigNoz to accept the resource"
}

if obj.status == nil or obj.status.observedGeneration ~= obj.metadata.generation then
  return hs
end

-- Only conditions set for the current generation describe the current spec.
local current = {}
for _, condition in ipairs(obj.status.conditions or {}) do
  if condition.observedGeneration == nil or condition.observedGeneration == obj.metadata.generation then
    current[condition.type] = condition
  end
end

-- SigNoz rejected the spec (HTTP 400 or 409); the operator stops retrying until the spec changes.
if current.Terminal ~= nil and current.Terminal.status == "True" then
  hs.status = "Degraded"
  hs.message = current.Terminal.message or "SigNoz rejected the resource"
  return hs
end

if current.Suspended ~= nil and current.Suspended.status == "True" then
  hs.status = "Suspended"
  hs.message = current.Suspended.message or "Reconciliation is suspended"
  return hs
end

local ready = current.Ready
if ready == nil then
  return hs
end
hs.message = ready.message or hs.message

if ready.status == "True" then
  hs.status = "Healthy"
elseif ready.status == "False" and (obj.kind == "ProviderConfig" or obj.kind == "ClusterProviderConfig") then
  -- A provider config is not ready when its endpoint or credential failed to resolve.
  hs.status = "Degraded"
end
-- A content kind with Ready Unknown and Recoverable True is retrying a transient failure.
return hs
