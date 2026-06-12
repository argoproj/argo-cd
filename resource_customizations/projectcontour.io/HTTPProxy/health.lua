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
       elseif obj.spec.includes ~= nil and cond.status == "False" then
          hs.status = "Healthy"
          hs.message = "HTTPProxy inclusions cannot be health checked" -- Parent/child pairs depend on each other circularly. This means that, without this check here, we block deployments. Either we flag orphans as valid/unknown, risking a successful deploy followed by subsequent failures once adopted, or we mark proxies with inclusions (parents) as healthy.
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