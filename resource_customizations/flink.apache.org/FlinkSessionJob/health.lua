local health_status = {}

if obj.status ~= nil and obj.status.jobStatus ~= nil then
  if obj.status.jobStatus.state == "RUNNING" or obj.status.jobStatus.state == "FINISHED" then
    health_status.status = "Healthy"
    health_status.message = obj.status.jobStatus.state
    return health_status
  end

  if obj.status.jobStatus.state == "RECONCILING" or obj.status.jobStatus.state == "CREATED" then
    health_status.status = "Progressing"
    health_status.message = obj.status.jobStatus.state
    return health_status
  end

  if obj.status.jobStatus.state == "SUSPENDED" or obj.status.jobStatus.state == "CANCELED" then
    health_status.status = "Suspended"
    health_status.message = obj.status.jobStatus.state
    return health_status
  end

  if obj.status.jobStatus.state == "FAILED" then
    health_status.status = "Degraded"
    if obj.status.error ~= nil then
      health_status.message = obj.status.error
    else
      health_status.message = "FlinkSessionJob failed"
    end
    return health_status
  end
end

if obj.status ~= nil and obj.status.error ~= nil then
  health_status.status = "Degraded"
  health_status.message = obj.status.error
  return health_status
end

health_status.status = "Progressing"
health_status.message = "Waiting for FlinkSessionJob to be reconciled"
return health_status
