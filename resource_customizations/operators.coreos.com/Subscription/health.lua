local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numDegraded = 0
    local numPending = 0
    local msg = ""
    for i, condition in pairs(obj.status.conditions) do
      msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
      if condition.type == "InstallPlanPending" and condition.status == "True" then
        numPending = numPending + 1
      elseif (condition.type == "InstallPlanMissing" and condition.reason ~= "ReferencedInstallPlanNotFound") then
        numDegraded = numDegraded + 1
      elseif (condition.type == "CatalogSourcesUnhealthy" or condition.type == "InstallPlanFailed" or condition.type == "ResolutionFailed") and condition.status == "True" then
        numDegraded = numDegraded + 1
      end
    end
    if numDegraded == 0 and numPending == 0 then
      health_status.status = "Healthy"
      health_status.message = msg
      return health_status
    elseif numPending > 0 and numDegraded == 0 then
      health_status.status = "Progressing"
      health_status.message = "An install plan for a subscription is pending installation"
      return health_status
    else
      health_status.status = "Degraded"
      health_status.message = msg
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "An install plan for a subscription is pending installation"
return health_status