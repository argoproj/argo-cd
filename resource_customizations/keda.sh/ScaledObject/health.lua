local hs = {}
local healthy = false
local degraded = false
local pausedMessage = nil
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "False" and condition.type == "Ready" then
        hs.message = condition.message
        degraded = true
      end
      if condition.status == "True" and condition.type == "Ready" then
        hs.message = condition.message
        healthy = true
      end
      if condition.status == "True" and condition.type == "Paused" then
        pausedMessage = condition.message
      end
      if condition.status == "True" and condition.type == "Fallback" then
        hs.message = condition.message
        degraded = true
      end
    end
  end
end
if degraded == true then
  hs.status = "Degraded"
  return hs
elseif healthy == true then
  -- Pausing a ScaledObject is a declared state, not a failure, so a paused
  -- ScaledObject that is Ready is Healthy. Reporting it as Suspended would keep
  -- a sync that pauses it waiting for a healthy state that never comes.
  hs.status = "Healthy"
  if pausedMessage ~= nil then
    hs.message = pausedMessage
  end
  return hs
end
hs.status = "Progressing"
hs.message = "Creating HorizontalPodAutoscaler Object"
return hs
