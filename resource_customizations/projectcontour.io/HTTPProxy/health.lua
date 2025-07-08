-- Status reporting information detailed here
-- https://projectcontour.io/docs/main/config/fundamentals/#status-reporting
-- More HTTPProxy status conditions api information here: https://projectcontour.io/docs/v1.9.0/api/#projectcontour.io/v1.HTTPProxyStatus

hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status then
  if obj.status.conditions then
    for _, cond in ipairs(obj.status.conditions) do
      if obj.metadata.generation == cond.observedGeneration then -- This must match so that we don't report a resource as healthy even though its status is stale
        if cond.type == "Valid" and cond.status == "True" then -- Contour will update a single condition, Valid, that is in normal-true polarity. That is, when currentStatus is valid, the Valid condition will be status: true, and vice versa.
          hs.status = "Healthy"
          hs.message = obj.status.description
          return hs
        elseif cond.type == "Valid" and cond.status == "False" then
          hs.status = "Degraded"
          hs.message = obj.status.description
          return hs
        end
      end
    end
  elseif obj.status.currentStatus then -- Covers any state where conditions are absent (thus no observedGeneration) but currentStatus is present such as NotReconciled or future similar cases.
    hs.message = obj.status.description
  end
end

return hs