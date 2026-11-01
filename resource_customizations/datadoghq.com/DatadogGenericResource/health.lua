-- Reference CRD can be found here:
-- https://github.com/DataDog/helm-charts/blob/main/charts/datadog-crds/templates/datadoghq.com_datadoggenericresources_v1.yaml

hs = {}
if obj.status == nil or obj.status.conditions == nil or #obj.status.conditions == 0 then
  hs.status = "Progressing"
  hs.message = "Waiting for DatadogGenericResource to be synced"
  return hs
end
for i, condition in ipairs(obj.status.conditions) do
  if condition.type == "Error" and condition.status == "True" then
    hs.status = "Degraded"
    hs.message = (condition.reason or "Error") .. ": " .. (condition.message or "DatadogGenericResource reported an error")
    return hs
  end
end
hs.status = "Healthy"
hs.message = "DatadogGenericResource is synced"
return hs
