local health_status = {}

if obj.status ~= nil and obj.status.reconciliationStatus ~= nil then
  if obj.status.reconciliationStatus.success or obj.status.reconciliationStatus.state == "DEPLOYED" then
    health_status.status = "Healthy"
    return health_status
  end 

  if obj.status.jobManagerDeploymentStatus == "DEPLOYED_NOT_READY" or obj.status.jobManagerDeploymentStatus == "DEPLOYING" then
    health_status.status = "Progressing"
    health_status.message = "Waiting for deploying"
    return health_status
  end

  if obj.status.jobManagerDeploymentStatus == "ERROR" then
    health_status.status = "Degraded"
    health_status.message = obj.status.reconciliationStatus.error
    return health_status
  end 
end

health_status.status = "Progressing"
health_status.message = "Waiting for Flink operator"
return health_status
