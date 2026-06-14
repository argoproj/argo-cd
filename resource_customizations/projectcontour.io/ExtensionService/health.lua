-- Status reporting information detailed here
-- ExtensionService status conditions api information here: https://projectcontour.io/docs/v1.18.0/config/api/#projectcontour.io/v1alpha1.ExtensionServiceStatus

hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status then
  if obj.status.conditions then
    for _, cond in ipairs(obj.status.conditions) do
      if obj.metadata.generation == cond.observedGeneration then -- This must match so that we don't report a resource as healthy even though its status is stale
        if cond.type == "Valid" and cond.status == "True" then -- Contour will update a single condition, Valid, that is in normal-true polarity.
          hs.status = "Healthy"
          hs.message = cond.message
          return hs
        elseif cond.type == "Valid" and cond.status == "False" then
          hs.status = "Degraded"
          hs.message = cond.message
          return hs
        end
      end
    end
  end
end

return hs