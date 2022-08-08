health_status = {}
if obj.status ~= nil then
  if obj.status.status == "SUCCESS" then
    health_status.status = "Healthy"
    health_status.message = "Fetched ExternalSecret."
  elseif obj.status.status:find('^ERROR') ~= nil then
    health_status.status = "Degraded"
    health_status.message = obj.status.status:gsub("ERROR, ", "")
  else
    health_status.status = "Progressing"
    health_status.message = "Waiting for ExternalSecret."
  end
  return health_status
end
health_status.status = "Progressing"
health_status.message = "Waiting for ExternalSecret."
return health_status