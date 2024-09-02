local hs = {}
local healthy = false
local degraded = false
local suspended = false
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
        hs.message = condition.message
        suspended = true
      end
    end
  end
end
if degraded == true then
  hs.status = "Degraded"
  return hs
elseif healthy == true and suspended == false then
  hs.status = "Healthy"
  return hs 
elseif healthy == true and suspended == true then
  hs.status = "Suspended"
  return hs
end
hs.status = "Progressing"
hs.message = "Creating HorizontalPodAutoscaler Object"
return hs