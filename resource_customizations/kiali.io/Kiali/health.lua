local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      health_status.message = condition.message
      if condition.type == "Successful" and condition.status == "True" then
        health_status.status = "Healthy"
        return health_status
      end
      if condition.type == "Failure" and condition.status == "True" then
        health_status.status = "Degraded"
        return health_status
      end
      if condition.type == "Running" and condition.reason == "Running" then
        health_status.status = "Progressing"
        return health_status
      end
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Kiali"
return health_status
