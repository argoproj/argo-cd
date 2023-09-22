local hs = {
  status = "Progressing",
  message = "Update in progress"
}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
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
return hs