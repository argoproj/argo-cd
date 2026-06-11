local hs = {}

local failed = false
local failMsg = ""
local complete = false
local message = ""
local isSuspended = false

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Failed" then
      failed = true
      complete = true
      failMsg = condition.message or ""
    elseif condition.type == "Complete" then
      complete = true
      message = condition.message or ""
    elseif condition.type == "Suspended" then
      complete = true
      message = condition.message or ""
      if condition.status == "True" then
        isSuspended = true
      end
    end
  end
end

if not complete then
  hs.status = "Progressing"
  hs.message = message
  return hs
end
if failed then
  hs.status = "Degraded"
  hs.message = failMsg
  return hs
end
if isSuspended then
  hs.status = "Suspended"
  hs.message = failMsg
  return hs
end

hs.status = "Healthy"
hs.message = message
return hs
