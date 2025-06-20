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