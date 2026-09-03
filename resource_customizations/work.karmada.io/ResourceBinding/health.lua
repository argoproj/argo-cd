local health_status = {}
if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Current resource status is insufficient"
  return health_status
end

-- spec.clusters is nil for dependency-propagated ResourceBindings (propagateDeps=true).
-- In that case, fall through and evaluate aggregatedStatus directly.
if obj.spec.clusters ~= nil and #obj.spec.clusters == 0 then
  health_status.status = "Progressing"
  health_status.message = "Current resource status is insufficient"
  return health_status
end

if obj.status.aggregatedStatus == nil or #obj.status.aggregatedStatus == 0 then
  health_status.status = "Progressing"
  health_status.message = "Current resource status is insufficient"
  return health_status
end

-- When spec.clusters is set, all clusters must have reported back before we evaluate.
if obj.spec.clusters ~= nil and #obj.spec.clusters ~= #obj.status.aggregatedStatus then
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
