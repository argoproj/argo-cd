hs = { status="Progressing", message="No status available"}
if obj.status ~= nil then
  if obj.status.phase ~= nil then
    hs.message = obj.status.phase
    if hs.message == "Failed" then
      hs.status = "Degraded"
      return hs
    elseif hs.message == "Pending" or hs.message == "Scheduling" or hs.message == "Scheduled" then
      return hs
    elseif hs.message == "Succeeded" then
      hs.status = "Suspended"
      return hs
    elseif hs.message == "Unknown" then
      hs.status = "Unknown"
      return hs
    end
  end
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = "Running"
        else
          hs.status = "Degraded"
          hs.message = condition.message
        end
      elseif condition.type == "Paused" and condition.status == "True" then
        hs.status = "Suspended"
        hs.message = condition.message
        return hs
      end
    end
  end
end
return hs
