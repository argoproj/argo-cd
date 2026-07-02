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

local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "InvalidSpec" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Progressing" and condition.reason == "RolloutAborted" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Progressing" and condition.reason == "ProgressDeadlineExceeded" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Paused" and condition.status == "True" then
    hs.status = "Suspended"
    hs.message = "Rollout is paused"
    return hs
  end
end

if obj.status.phase == "Progressing" then
  hs.status = "Progressing"
  hs.message = "Waiting for rollout to finish steps"
  return hs
end

hs.status = "Healthy"
hs.message = ""
return hs



