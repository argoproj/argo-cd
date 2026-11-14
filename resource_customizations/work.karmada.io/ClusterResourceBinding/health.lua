local health_status = {}
if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Current resource status is insufficient"
  return health_status
end

if obj.spec.clusters == nil or #obj.spec.clusters == 0 then
  health_status.status = "Progressing"
  health_status.message = "Current resource status is insufficient"
  return health_status
end

if obj.status.aggregatedStatus == nil or #obj.spec.clusters ~= #obj.status.aggregatedStatus then
  health_status.status = "Progressing"
  health_status.message = "Current resource status is insufficient"
  return health_status
end

for i, status in ipairs(obj.status.aggregatedStatus) do
  if status.health == "Unhealthy" then
    health_status.status = "Degraded"
    health_status.message = "Current resource status is unhealthy"
    return health_status
  end

  if status.health == "Unknown" then
    if status.applied ~= true then
      health_status.status = "Degraded"
      health_status.message = "Current resource status is unhealthy"
      return health_status
    end
  end
end

health_status.status = "Healthy"
return health_status
