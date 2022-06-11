health_status = {}
if obj.status ~= nil then
  if obj.status.phase == "Running" then
    health_status.status = "Healthy"
    health_status.message = "Jaeger is Running"
    return health_status
  end
  if obj.status.phase == "Failed" then
    health_status.status = "Degraded"
    health_status.message = "Jaeger Failed For Some Reason"
    return health_status
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Jaeger"
return health_status