-- Reference CRD can be found here:
-- https://github.com/DataDog/helm-charts/blob/main/charts/datadog-crds/templates/datadoghq.com_datadogmetrics_v1.yaml

hs = {}
if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do
    -- Check for the "Error: True" condition first
    if condition.type == "Error" and condition.status == "True" then
      hs.status = "Degraded"
      local reason = condition.reason or ""
      local message = condition.message or "DatadogMetric reported an error"
      if reason ~= "" then
        hs.message = reason .. ": " .. message
      else
        hs.message = message
      end
      return hs
    end
  end
  for i, condition in ipairs(obj.status.conditions) do
    -- Check for the "Valid: False" condition
    if condition.type == "Valid" and condition.status == "False" then
      hs.status = "Degraded"
      hs.message = condition.message or "DatadogMetric is not valid"
      return hs
    end
  end
end
-- If no "Degraded" conditions are found, default to Healthy
hs.status = "Healthy"
hs.message = "DatadogMetric is healthy"
return hs
