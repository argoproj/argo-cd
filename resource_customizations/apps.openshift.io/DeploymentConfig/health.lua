health_check = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    numTrue = 0
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "Available" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "Progressing" and condition.status == "True" then
        numTrue = numTrue + 1
        health_check.message = condition.message
      elseif condition.type == "Progressing" and condition.status == "Unknown" then
        health_check.message = condition.message
      end
    end
    if numTrue == 2 then
      health_check.status = "Healthy"
      return health_check
    elseif numTrue == 1 then
      health_check.status = "Progressing"
      return health_check
    else
      health_check.status = "Degraded"
      health_check.message = "Deployment config is degraded"
      return health_check
    end
  end
end