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

local hs = {
  status = "Progressing",
  message = "Update in progress"
}
if obj.status ~= nil then
  -- ConfigConnector and ConfigConnectorContext use status.healthy instead of conditions
  if (obj.kind == "ConfigConnector" or obj.kind == "ConfigConnectorContext") and obj.status.healthy ~= nil then
    -- Check if status is stale
    if obj.status.observedGeneration == nil or obj.status.observedGeneration == obj.metadata.generation then
      if obj.status.healthy == true then
        hs.status = "Healthy"
        hs.message = obj.kind .. " is healthy"
        return hs
      else
        hs.status = "Degraded"
        hs.message = obj.kind .. " is not healthy"
        return hs
      end
    end
  end

  if obj.status.conditions ~= nil then
    
    -- Progressing health while the resource status is stale, skip if observedGeneration is not set
    if obj.status.observedGeneration == nil or obj.status.observedGeneration == obj.metadata.generation then
      for i, condition in ipairs(obj.status.conditions) do

        -- Up To Date
        if condition.reason == "UpToDate" and condition.status == "True" then
          hs.status = "Healthy"
          hs.message = condition.message
          return hs
        end

        -- Update Failed
        if condition.reason == "UpdateFailed" then
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end

        -- Dependency Not Found
        if condition.reason == "DependencyNotFound" then
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end

        -- Dependency Not Ready
        if condition.reason == "DependencyNotReady" then
          hs.status = "Suspended"
          hs.message = condition.message
          return hs
        end
      end
    end
  end
end
return hs