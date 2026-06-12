-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

if obj.status == nil or obj.status.conditions == nil then
  -- no status info available yet
  return {
    status = "Progressing",
    message = "Waiting for Keycloak status conditions to exist",
  }
end

-- Sort conditions by lastTransitionTime, from old to new.
-- Ensure that conditions with nil lastTransitionTime are always sorted after those with non-nil values.
table.sort(obj.status.conditions, function(a, b)
  -- Nil values are considered "less than" non-nil values.
  -- This means that conditions with nil lastTransitionTime will be sorted to the end.
  if a.lastTransitionTime == nil then
    return false
  elseif b.lastTransitionTime == nil then
    return true
  else
    -- If both have non-nil lastTransitionTime, compare them normally.
    return a.lastTransitionTime < b.lastTransitionTime
  end
end)

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Ready" and condition.status == "True" then
    return {
      status = "Healthy",
      message = "",
    }
  elseif condition.type == "HasErrors" and condition.status == "True" then
    return {
      status = "Degraded",
      message = "Has Errors: " .. condition.message,
    }
  end
end

-- We couldn't find matching conditions yet, so assume progressing
return {
  status = "Progressing",
  message = "",
}