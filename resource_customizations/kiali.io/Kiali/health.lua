health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      health_status.message = condition.message
      if condition.reason == "Successful" then
        health_status.status = "Healthy"
      elseif condition.reason == "Running" then
        health_status.status = "Progressing"
      else
        health_status.status = "Degraded"
      end
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Kiali"
return health_status