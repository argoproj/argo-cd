if obj.status == nil or obj.status.conditions == nil then
  -- no status info available yet
  return {
    status = "Progressing",
    message = "Waiting for Keycloak status conditions to exist",
  }
end

-- Sort conditions by lastTransitionTime, from old to new.
table.sort(obj.status.conditions, function(a, b)
  return a.lastTransitionTime < b.lastTransitionTime
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