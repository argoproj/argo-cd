local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numTrue = 0
    local numFalse = 0
    local msg = ""
    for i, condition in pairs(obj.status.conditions) do
      msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
      if condition.type == "Ready" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "InstallSucceeded" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "DependenciesInstalled" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "DeploymentsAvailable" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "Ready" and condition.status == "False" then
        numFalse = numFalse + 1
      elseif condition.type == "DeploymentsAvailable" and condition.status == "False" then
        numFalse = numFalse + 1
      elseif condition.status == "Unknown" then
        numFalse = numFalse + 1
      end
    end
    if(numFalse > 0) then
      health_status.message = msg
      health_status.status = "Progressing"
      return health_status
    elseif(numTrue == 4) then
      health_status.message = "KnativeServing is healthy."
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
health_status.message = "Waiting for KnativeServing"
return health_status