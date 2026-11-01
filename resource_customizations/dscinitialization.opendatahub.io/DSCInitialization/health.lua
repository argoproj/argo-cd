local health_status = {}
if obj.metadata ~= nil and obj.metadata.generation ~= nil and obj.status ~= nil and obj.status.observedGeneration ~= nil and obj.metadata.generation ~= obj.status.observedGeneration then
  health_status.status = "Progressing"
  health_status.message = "Waiting for DSCInitialization spec update to be observed"
  return health_status
end
if obj.status ~= nil and obj.status.phase ~= nil then
  if obj.status.phase == "Ready" then
    health_status.status = "Healthy"
    health_status.message = "DSCInitialization is ready"
    return health_status
  end
  if obj.status.phase == "Error" then
    health_status.status = "Degraded"
    health_status.message = obj.status.errorMessage or "DSCInitialization encountered an error"
    return health_status
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for DSCInitialization to become ready"
return health_status
