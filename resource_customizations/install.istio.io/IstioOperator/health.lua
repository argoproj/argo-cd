local health_status = {}
if obj.status ~= nil then
  if obj.status.status ~= nil then
    if obj.status.status == 0 or obj.status.status == "NONE" then
      health_status.status = "Unknown"
      health_status.message = "Component is not present."
      return health_status
    end
    if obj.status.status == 1 or obj.status.status == "UPDATING" then
      health_status.status = "Progressing"
      health_status.message = "Component is being updated to a different version."
      return health_status
    end
    if obj.status.status == 2 or obj.status.status == "RECONCILING" then
      health_status.status = "Progressing"
      health_status.message = "Controller has started but not yet completed reconciliation loop for the component."
      return health_status
    end
    if obj.status.status == 3 or obj.status.status == "HEALTHY" then
      health_status.status = "Healthy"
      health_status.message = "Component is healthy."
      return health_status
    end
    if obj.status.status == 4 or obj.status.status == "ERROR" then
      health_status.status = "Degraded"
      health_status.message = "Component is in an error state."
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Istio Control Plane"
return health_status