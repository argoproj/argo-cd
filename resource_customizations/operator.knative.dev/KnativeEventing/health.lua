health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    numTrue = 0
    numFalse = 0
    msg = ""
    for i, condition in pairs(obj.status.conditions) do
      msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
      if condition.type == "Ready" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "InstallSucceeded" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.status == "Unknown" then
        numFalse = numFalse + 1
      end
    end
    if(numFalse > 0) then
      health_status.message = msg
      health_status.status = "Progressing"
      return health_status
    elseif(numTrue == 2) then
      health_status.message = "KnativeEventing is healthy."
      health_status.status = "Healthy"
      return health_status
    else
      health_status.message = msg
      health_status.status = "Degraded"
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for KnativeEventing"
return health_status