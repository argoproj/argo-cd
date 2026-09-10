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

-- Health check for the cert-manager ACME Order CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the ACME Order status.state to an Argo CD health status:
--   Healthy     - the order is valid (the certificate has been issued).
--   Degraded    - the order is invalid, expired, or errored.
--   Progressing - the order is pending, ready, or processing, or status is
--                 not populated yet.
local hs = { status = "Progressing", message = "Waiting for order" }
if obj.status ~= nil and obj.status.state ~= nil and obj.status.state ~= "" then
  local state = obj.status.state
  if state == "valid" then
    hs.status = "Healthy"
    hs.message = "Order is valid"
  elseif state == "invalid" or state == "expired" or state == "errored" then
    hs.status = "Degraded"
    hs.message = obj.status.reason or ("Order is " .. state)
  else
    hs.status = "Progressing"
    hs.message = "Order is " .. state
  end
end
return hs
