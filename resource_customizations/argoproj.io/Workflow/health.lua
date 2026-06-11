local hs = {}

local phase = ""
local message = ""
if obj.status ~= nil then
  phase = obj.status.phase or ""
  message = obj.status.message or ""
end

if phase == "" or phase == "Pending" or phase == "Running" then
  hs.status = "Progressing"
  hs.message = message
  return hs
end
if phase == "Succeeded" then
  hs.status = "Healthy"
  hs.message = message
  return hs
end
if phase == "Failed" or phase == "Error" then
  hs.status = "Degraded"
  hs.message = message
  return hs
end

hs.status = "Unknown"
hs.message = message
return hs
