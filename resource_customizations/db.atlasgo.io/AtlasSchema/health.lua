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

hs = {}

local function readyCond(obj)
  if obj.status ~= nil and obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        return condition
      end
    end
  end
  return nil
end

local ready = readyCond(obj)

if ready == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for Atlas Operator"
  return hs
end

if ready.status == "True" then
  hs.status = "Healthy"
  hs.message = ready.reason
  return hs
end

if ready.message == "Reconciling" or ready.message == "GettingDevDB" then
  hs.status = "Progressing"
else
  hs.status = "Degraded"
end

hs.message = ready.reason

return hs

