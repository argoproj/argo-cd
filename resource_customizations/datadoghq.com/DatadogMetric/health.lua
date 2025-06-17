hs = {}
if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do
    -- Check for the "Valid: False" condition
    if condition.type == "Valid" and condition.status == "False" then
      hs.status = "Degraded"
      hs.message = condition.message or "DatadogMetric is not valid"
      return hs
    end
    -- Check for the "Error: True" condition
    if condition.type == "Error" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "DatadogMetric reported an error"
      return hs
    end
  end
end
-- If no "Degraded" conditions are found, default to Healthy
hs.status = "Healthy"
hs.message = "DatadogMetric is healthy"
return hs 