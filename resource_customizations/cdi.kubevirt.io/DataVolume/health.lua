hs = { status="Progressing", message="No status available"}
if obj.status ~= nil then
  if obj.status.phase ~= nil then
    hs.message = obj.status.phase
    if hs.message == "Succeeded" then
      hs.status = "Healthy"
      return hs
    elseif hs.message == "Failed" or hs.message == "Unknown" then
      hs.status = "Degraded"
    elseif hs.message == "Paused" then
      hs.status = "Suspended"
      return hs
    end
  end
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Running" and condition.status == "False" and condition.reason == "Error" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
  end
end
return hs
