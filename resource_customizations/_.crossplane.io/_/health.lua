-- Health check copied from here: https://github.com/crossplane/docs/blob/9fe744889fc150ca71e5298d90b4133f79ea20f2/content/master/guides/crossplane-with-argo-cd.md

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
  "ClusterProviderConfig",
  "ProviderConfig",
  "ProviderConfigUsage",
  "ControllerConfig" -- Added to ensure that healthcheck is backwards-compatible with Crossplane v1
}
if obj.status == nil or next(obj.status) == nil and contains(has_no_status, obj.kind) then
    health_status.status = "Healthy"
    health_status.message = "Resource is up-to-date."
  return health_status
end

if obj.status == nil or next(obj.status) == nil or obj.status.conditions == nil then
  if (obj.kind == "ProviderConfig" or obj.kind == "ClusterProviderConfig") and obj.status.users ~= nil then
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

  if contains({"Ready", "Healthy", "Offered", "Established", "ValidPipeline", "RevisionHealthy"}, condition.type) then
    if condition.status == "True" then
      health_status.status = "Healthy"
      health_status.message = "Resource is up-to-date."
    end
  end
end

return health_status
