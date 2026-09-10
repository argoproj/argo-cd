local sep = " --- "
local hs = {}
if obj.status ~= nil then
  local message = ""
  if tonumber(obj.status.canaryWeight) > 0 then            
    message = "Canary Weight: " .. obj.status.canaryWeight .. " %"
  end
  for i, condition in ipairs(obj.status.conditions) do
    if message ~= "" then
      message = message .. sep
    end
    message = message .. condition.message
  end          
  if obj.status.phase == "Failed" then
    hs.status = "Degraded"
  elseif ( obj.status.phase == "Progressing" or 
      obj.status.phase == "Finalising" or
      obj.status.phase == "Promoting" ) then             
    hs.status = "Progressing"
  elseif ( obj.status.phase == "Succeeded" or 
      obj.status.phase == "Initialized" ) then
    hs.status = "Healthy"
  else
    hs.status = "Unknown"
  end
  hs.message = obj.status.phase .. sep .. message 
  return hs
end        
hs.status = "Unknown"
hs.message = "No status"
return hs