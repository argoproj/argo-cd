-- Health check copied from here: https://github.com/crossplane/docs/blob/bd701357e9d5eecf529a0b42f23a78850a6d1d87/content/master/guides/crossplane-with-argo-cd.md

health_status = {
  status = "Progressing",
  message = "Provisioning ..."
}

local function contains (table, val)
  for i, v in ipairs(table) do
    if v == val then
      return true
    end
  end
  return false
end

local has_no_status = {
  "Composition",
  "CompositionRevision",
  "DeploymentRuntimeConfig",
  "ControllerConfig",
  "ProviderConfig",
  "ProviderConfigUsage"
}
if obj.status == nil or next(obj.status) == nil and contains(has_no_status, obj.kind) then
    health_status.status = "Healthy"
    health_status.message = "Resource is up-to-date."
  return health_status
end

if obj.status == nil or next(obj.status) == nil or obj.status.conditions == nil then
  if obj.kind == "ProviderConfig" and obj.status.users ~= nil then
    health_status.status = "Healthy"
    health_status.message = "Resource is in use."
    return health_status
  end
  return health_status
end

for i, condition in ipairs(obj.status.conditions) do
  if condition.type == "LastAsyncOperation" then
    if condition.status == "False" then
      health_status.status = "Degraded"
      health_status.message = condition.message
      return health_status
    end
  end

  if condition.type == "Synced" then
    if condition.status == "False" then
      health_status.status = "Degraded"
      health_status.message = condition.message
      return health_status
    end
  end

  if contains({"Ready", "Healthy", "Offered", "Established"}, condition.type) then
    if condition.status == "True" then
      health_status.status = "Healthy"
      health_status.message = "Resource is up-to-date."
      return health_status
    end
  end
end

return health_status
