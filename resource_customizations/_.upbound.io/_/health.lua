-- Health check for Crossplane resources, adapted from
-- https://github.com/crossplane/docs/blob/9fe744889fc150ca71e5298d90b4133f79ea20f2/content/master/guides/crossplane-with-argo-cd.md

health_status = {
  status = "Progressing",
  message = "Provisioning ..."
}

local function contains(list, val)
  for _, v in ipairs(list) do
    if v == val then
      return true
    end
  end
  return false
end

-- Kinds that never get status.conditions, so would sit at Progressing forever.
-- Only consulted when there are none, so a kind that gains them can stay.
local has_no_conditions = {
  -- v1 and v2
  "ProviderConfig",
  "ProviderConfigUsage",

  -- v1 only
  "StoreConfig",

  -- v2 only
  "ClusterProviderConfig",
  "ClusterProviderConfigUsage"
}

if obj.status == nil or obj.status.conditions == nil or next(obj.status.conditions) == nil then
  if contains(has_no_conditions, obj.kind) then
    health_status.status = "Healthy"
    -- status.users affects the message, not the health status.
    if obj.status ~= nil and obj.status.users ~= nil then
      health_status.message = "Resource is in use."
    else
      health_status.message = "Resource is up-to-date."
    end
  end
  return health_status
end

for _, condition in ipairs(obj.status.conditions) do
  if contains({"LastAsyncOperation", "Synced"}, condition.type) and condition.status == "False" then
    health_status.status = "Degraded"
    health_status.message = condition.message
    return health_status
  end

  if condition.type == "Ready" and condition.status == "True" then
    health_status.status = "Healthy"
    health_status.message = "Resource is up-to-date."
  end
end

return health_status
