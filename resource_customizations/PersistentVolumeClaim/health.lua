local hs = {}

local phase = ""
if obj.status ~= nil then
  phase = obj.status.phase or ""
end

if phase == "Lost" then
  hs.status = "Degraded"
elseif phase == "Pending" then
  hs.status = "Progressing"
elseif phase == "Bound" then
  hs.status = "Healthy"
else
  hs.status = "Unknown"
end

return hs
